package ai

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aishuati/backend/internal/jobs"
	"github.com/aishuati/backend/internal/learning"
	"github.com/aishuati/backend/internal/practice"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	gradePromptVersion         = "practice_grade.v1"
	explainPromptVersion       = "practice_explain.v1"
	batchAnalysisPromptVersion = "practice_batch_analysis.v1"
)

//go:embed prompts/practice_grade.v1.md
var gradePrompt string

//go:embed prompts/practice_explain.v1.md
var explainPrompt string

//go:embed prompts/practice_batch_analysis.v1.md
var batchAnalysisPrompt string

type Service struct {
	pool   *pgxpool.Pool
	client *Client
	logger *slog.Logger
}

func NewService(pool *pgxpool.Pool, client *Client, logger *slog.Logger) *Service {
	return &Service{pool: pool, client: client, logger: logger}
}

// Handlers 返回 AI 相关任务处理器，供 worker 注册。
func (s *Service) Handlers() map[string]jobs.Handler {
	return map[string]jobs.Handler{
		"grade_practice_item_ai":       s.handleGrade,
		"explain_practice_item_ai":     s.handleExplain,
		"analyze_practice_session_ai":  s.handleBatchAnalysis,
		"generate_ai_practice_session": s.handleGenerate,
	}
}

type itemContext struct {
	ItemID       string
	SessionID    string
	Type         string
	Stem         string
	Options      *string
	Material     *string
	UserValue    *string
	KeyValue     *string
	KeyAuthority *string
	Explanation  *string
}

type batchAnalysisRow struct {
	ItemID               string
	Position             int
	QuestionVersionID    string
	Type                 string
	Stem                 string
	SourceSection        *string
	Options              *string
	MaterialID           *string
	Material             *string
	UserValue            *string
	StandardAnswer       *string
	AnswerAuthority      *string
	GeneratedAnswer      *string
	GeneratedExplanation *string
	GradingSource        string
	GradingStatus        string
	CorrectValue         *string
	ExplanationSource    *string
	CachedExplanation    *string
	CachedPromptVersion  *string
}

func (s *Service) loadItem(ctx context.Context, db store.DBTx, sessionID, itemID string) (itemContext, error) {
	const q = `SELECT pi.id::text, pi.session_id::text, v.type, v.stem, v.options::text,
	        mv.content, ua.value::text, ak.value::text, ak.authority, ak.explanation
	 FROM practice_items pi
	 JOIN question_versions v ON v.id = pi.question_version_id
	 JOIN practice_sessions ps ON ps.id = pi.session_id
	 LEFT JOIN material_versions mv ON mv.id = v.material_version_id
	 LEFT JOIN user_answers ua ON ua.item_id = pi.id
	 LEFT JOIN answer_keys ak ON ak.question_version_id = v.id
	 WHERE pi.id = $1 AND pi.session_id = $2`
	return store.CollectOneRow[itemContext](ctx, db, q, itemID, sessionID)
}

func (s *Service) loadBatch(ctx context.Context, sessionID string) ([]batchAnalysisRow, error) {
	return store.CollectRows[batchAnalysisRow](ctx, s.pool,
		`SELECT pi.id::text, pi.position, v.id::text, v.type, v.stem, ss.name, v.options::text,
		        mv.material_id::text, mv.content, ua.value::text,
		        ak.value::text, ak.authority, aga.value::text, aga.explanation,
		        gr.source, gr.status, gr.correct_value::text, gr.explanation_source,
		        qae.explanation, qae.prompt_version
		 FROM practice_items pi
		 JOIN question_versions v ON v.id = pi.question_version_id
		 LEFT JOIN source_sections ss ON ss.id = v.source_section_id
		 LEFT JOIN material_versions mv ON mv.id = v.material_version_id
		 LEFT JOIN user_answers ua ON ua.item_id = pi.id
		 JOIN grading_results gr ON gr.item_id = pi.id AND gr.session_id = pi.session_id
		 LEFT JOIN answer_keys ak ON ak.question_version_id = v.id
		 LEFT JOIN ai_generated_question_answers aga ON aga.question_version_id = v.id
		 LEFT JOIN question_ai_explanations qae ON qae.question_version_id = v.id
		 WHERE pi.session_id = $1
		 ORDER BY pi.position`, sessionID)
}

func (s *Service) handleGrade(ctx context.Context, attempts, maxAttempts int, payload json.RawMessage) error {
	var req struct {
		SessionID string `json:"sessionId"`
		ItemID    string `json:"itemId"`
	}
	if err := strictDecode(payload, &req); err != nil {
		return err
	}
	item, err := s.loadItem(ctx, s.pool, req.SessionID, req.ItemID)
	if err != nil {
		return fmt.Errorf("加载判分题目失败: %w", err)
	}
	payloadJSON, _ := json.Marshal(map[string]any{
		"type":       item.Type,
		"stem":       item.Stem,
		"options":    item.Options,
		"material":   item.Material,
		"userAnswer": item.UserValue,
	})
	out, err := s.client.RunPrompt(ctx, "practice_grade", gradePromptVersion, item.ItemID, gradePrompt, string(payloadJSON))
	if err != nil {
		if attempts >= maxAttempts {
			return s.failGrading(ctx, item.SessionID, item.ItemID, err)
		}
		return err
	}
	var resp struct {
		Correctness   string          `json:"correctness"`
		CorrectAnswer json.RawMessage `json:"correctAnswer"`
		Explanation   string          `json:"explanation"`
		Confidence    string          `json:"confidence"`
	}
	if err := strictDecode(out, &resp); err != nil {
		if attempts >= maxAttempts {
			return s.failGrading(ctx, item.SessionID, item.ItemID, err)
		}
		return err
	}
	explanation := strings.TrimSpace(resp.Explanation)
	if explanation == "" || len([]rune(explanation)) > 2000 {
		err := errors.New("AI 判分解析文本缺失或超长")
		if attempts >= maxAttempts {
			return s.failGrading(ctx, item.SessionID, item.ItemID, err)
		}
		return err
	}
	status := map[string]string{
		"correct":          practice.StatusCorrect,
		"incorrect":        practice.StatusIncorrect,
		"cannot_determine": practice.StatusFailed,
	}[resp.Correctness]
	if status == "" {
		err := fmt.Errorf("AI 判分结论不合法: %s", resp.Correctness)
		if attempts >= maxAttempts {
			return s.failGrading(ctx, item.SessionID, item.ItemID, err)
		}
		return err
	}
	if status == practice.StatusFailed {
		explanation = "AI 无法可靠判定本题，已留待人工处理。"
	}
	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`UPDATE grading_results
			 SET status = $3, correct_value = $4, explanation = $5, explanation_source = 'ai', updated_at = now()
			 WHERE item_id = $1 AND session_id = $2 AND source = 'ai' AND status = 'pending'`,
			item.ItemID, item.SessionID, status, nullableJSON(resp.CorrectAnswer), explanation)
		if err != nil {
			return err
		}
		var userID string
		if err := tx.QueryRow(ctx,
			`SELECT user_id::text FROM practice_sessions WHERE id = $1`, item.SessionID).Scan(&userID); err != nil {
			return err
		}
		if err := jobs.EnqueueTx(ctx, tx, "rebuild_user_knowledge_stats", map[string]string{"userId": userID}); err != nil {
			return err
		}
		return practice.NewStore(tx).CompleteIfDone(ctx, tx, item.SessionID)
	})
	if err != nil {
		return fmt.Errorf("写回 AI 判分失败: %w", err)
	}
	s.logger.Info("ai_grade_done", "item_id", item.ItemID, "status", status)
	return nil
}

type batchAnalysisItem struct {
	ItemID               string `json:"itemId"`
	Position             int    `json:"position"`
	Type                 string `json:"type"`
	Stem                 string `json:"stem"`
	SourceSection        string `json:"sourceSection,omitempty"`
	Options              any    `json:"options"`
	MaterialID           string `json:"materialId,omitempty"`
	UserAnswer           any    `json:"userAnswer"`
	StandardAnswer       any    `json:"standardAnswer"`
	GeneratedAnswer      any    `json:"generatedAnswer,omitempty"`
	GeneratedExplanation string `json:"generatedExplanation,omitempty"`
	CorrectAnswer        any    `json:"correctAnswer"`
	AnswerAuthority      string `json:"answerAuthority,omitempty"`
	GradingSource        string `json:"gradingSource"`
	GradingStatus        string `json:"gradingStatus"`
	NeedsGrading         bool   `json:"needsGrading"`
	NeedsExplanation     bool   `json:"needsExplanation"`
}

type batchAnalysisInput struct {
	Materials      map[string]string         `json:"materials,omitempty"`
	Items          []batchAnalysisItem       `json:"items"`
	LearningMemory learning.AIMemorySnapshot `json:"learningMemory"`
}

func (s *Service) needsExplanation(row batchAnalysisRow) bool {
	if row.GradingSource != practice.SourceDeterministic || row.AnswerAuthority == nil {
		return false
	}
	if row.ExplanationSource != nil && (*row.ExplanationSource == "official" || *row.ExplanationSource == "human_verified") {
		return false
	}
	return row.CachedExplanation == nil || row.CachedPromptVersion == nil || *row.CachedPromptVersion != batchAnalysisPromptVersion
}

func (s *Service) cachedExplanation(row batchAnalysisRow) (string, bool) {
	if row.CachedExplanation == nil || row.CachedPromptVersion == nil || *row.CachedPromptVersion != batchAnalysisPromptVersion {
		return "", false
	}
	return strings.TrimSpace(*row.CachedExplanation), true
}

func (s *Service) handleBatchAnalysis(ctx context.Context, attempts, maxAttempts int, payload json.RawMessage) error {
	var req struct {
		SessionID string `json:"sessionId"`
	}
	if err := strictDecode(payload, &req); err != nil {
		return err
	}
	userID, resetAt, err := s.sessionMemoryContext(ctx, s.pool, req.SessionID)
	if err != nil {
		err = fmt.Errorf("加载账号学习记忆失败: %w", err)
		if attempts >= maxAttempts {
			return s.failBatchAnalysis(ctx, req.SessionID, err)
		}
		return err
	}
	// 批次 AI 需要看到本批确定性成绩；重算仍是唯一统计写入路径。
	if err := learning.NewStore(s.pool).RebuildUserStats(ctx, s.pool, userID); err != nil {
		err = fmt.Errorf("更新账号学习记忆失败: %w", err)
		if attempts >= maxAttempts {
			return s.failBatchAnalysis(ctx, req.SessionID, err)
		}
		return err
	}
	memory, err := learning.NewStore(s.pool).MemorySnapshotForAI(ctx, userID)
	if err != nil {
		err = fmt.Errorf("读取账号学习记忆失败: %w", err)
		if attempts >= maxAttempts {
			return s.failBatchAnalysis(ctx, req.SessionID, err)
		}
		return err
	}
	rows, err := s.loadBatch(ctx, req.SessionID)
	if err != nil {
		err = fmt.Errorf("加载批次分析数据失败: %w", err)
		if attempts >= maxAttempts {
			return s.failBatchAnalysis(ctx, req.SessionID, err)
		}
		return err
	}
	input := batchAnalysisInput{Materials: map[string]string{}, Items: make([]batchAnalysisItem, 0, len(rows)), LearningMemory: memory}
	for _, row := range rows {
		section := ""
		if row.SourceSection != nil {
			section = *row.SourceSection
		}
		if row.MaterialID != nil && row.Material != nil {
			input.Materials[*row.MaterialID] = *row.Material
		}
		input.Items = append(input.Items, batchAnalysisItem{
			ItemID: row.ItemID, Position: row.Position, Type: row.Type, Stem: row.Stem,
			SourceSection: section, Options: jsonValue(row.Options),
			MaterialID: jsonValueString(row.MaterialID), UserAnswer: jsonValue(row.UserValue),
			StandardAnswer: jsonValue(row.StandardAnswer), GeneratedAnswer: jsonValue(row.GeneratedAnswer),
			GeneratedExplanation: jsonValueString(row.GeneratedExplanation), CorrectAnswer: jsonValue(row.CorrectValue),
			AnswerAuthority: jsonValueString(row.AnswerAuthority), GradingSource: row.GradingSource,
			GradingStatus:    row.GradingStatus,
			NeedsGrading:     row.GradingSource == practice.SourceAI && row.GradingStatus == practice.StatusPending,
			NeedsExplanation: s.needsExplanation(row),
		})
	}
	if len(input.Materials) == 0 {
		input.Materials = nil
	}
	inputJSON, _ := json.Marshal(input)
	out, err := s.client.RunPrompt(ctx, "practice_batch_analysis", batchAnalysisPromptVersion, req.SessionID, batchAnalysisPrompt, string(inputJSON))
	if err != nil {
		if attempts >= maxAttempts {
			return s.failBatchAnalysis(ctx, req.SessionID, err)
		}
		return err
	}
	var response struct {
		Summary string `json:"summary"`
		Grades  []struct {
			ItemID        string          `json:"itemId"`
			Correctness   string          `json:"correctness"`
			CorrectAnswer json.RawMessage `json:"correctAnswer"`
			Explanation   string          `json:"explanation"`
		} `json:"grades"`
		Explanations []struct {
			ItemID string `json:"itemId"`
			Text   string `json:"text"`
		} `json:"explanations"`
	}
	if err := strictDecode(out, &response); err != nil {
		if attempts >= maxAttempts {
			return s.failBatchAnalysis(ctx, req.SessionID, err)
		}
		return err
	}
	summary := strings.TrimSpace(response.Summary)
	if summary == "" || len([]rune(summary)) > 4000 {
		err := errors.New("AI 批次总结文本缺失或超长")
		if attempts >= maxAttempts {
			return s.failBatchAnalysis(ctx, req.SessionID, err)
		}
		return err
	}
	allowedGrades := map[string]bool{}
	allowedExplanations := map[string]bool{}
	versionIDs := map[string]string{}
	cachedExplanations := map[string]string{}
	for _, row := range rows {
		versionIDs[row.ItemID] = row.QuestionVersionID
		allowedGrades[row.ItemID] = row.GradingSource == practice.SourceAI && row.GradingStatus == practice.StatusPending
		allowedExplanations[row.ItemID] = s.needsExplanation(row)
		if explanation, ok := s.cachedExplanation(row); ok {
			cachedExplanations[row.ItemID] = explanation
		}
	}
	seenGrades := map[string]bool{}
	for _, grade := range response.Grades {
		if !allowedGrades[grade.ItemID] || seenGrades[grade.ItemID] || (grade.Correctness != "correct" && grade.Correctness != "incorrect" && grade.Correctness != "cannot_determine") || strings.TrimSpace(grade.Explanation) == "" || len([]rune(grade.Explanation)) > 2000 {
			err := errors.New("AI 批次判定包含无效题目或结论")
			if attempts >= maxAttempts {
				return s.failBatchAnalysis(ctx, req.SessionID, err)
			}
			return err
		}
		seenGrades[grade.ItemID] = true
	}
	for itemID, needed := range allowedGrades {
		if needed && !seenGrades[itemID] {
			err := errors.New("AI 批次判定缺少题目结果")
			if attempts >= maxAttempts {
				return s.failBatchAnalysis(ctx, req.SessionID, err)
			}
			return err
		}
	}
	seenExplanations := map[string]bool{}
	for _, explanation := range response.Explanations {
		text := strings.TrimSpace(explanation.Text)
		if !allowedExplanations[explanation.ItemID] || seenExplanations[explanation.ItemID] || text == "" || len([]rune(text)) > 2000 {
			err := errors.New("AI 批次解析包含无效题目或文本")
			if attempts >= maxAttempts {
				return s.failBatchAnalysis(ctx, req.SessionID, err)
			}
			return err
		}
		seenExplanations[explanation.ItemID] = true
	}
	for itemID, needed := range allowedExplanations {
		if needed && !seenExplanations[itemID] {
			err := errors.New("AI 批次解析缺少题目结果")
			if attempts >= maxAttempts {
				return s.failBatchAnalysis(ctx, req.SessionID, err)
			}
			return err
		}
	}

	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		for _, row := range rows {
			if explanation, ok := cachedExplanations[row.ItemID]; ok {
				if _, err := tx.Exec(ctx,
					`UPDATE grading_results
					 SET explanation = $3, explanation_source = 'ai', updated_at = now()
					 WHERE session_id = $1 AND item_id = $2 AND source = 'deterministic'
					   AND (explanation IS NULL OR explanation_source = 'ai')`,
					req.SessionID, row.ItemID, explanation); err != nil {
					return err
				}
			}
		}
		for _, grade := range response.Grades {
			status := practice.StatusFailed
			if grade.Correctness == "correct" {
				status = practice.StatusCorrect
			} else if grade.Correctness == "incorrect" {
				status = practice.StatusIncorrect
			}
			explanation := strings.TrimSpace(grade.Explanation)
			if status == practice.StatusFailed {
				explanation = "AI 无法可靠判定本题，已留待人工处理。"
			}
			if _, err := tx.Exec(ctx,
				`UPDATE grading_results
				 SET status = $3, correct_value = $4, explanation = $5, explanation_source = 'ai', updated_at = now()
				 WHERE session_id = $1 AND item_id = $2 AND source = 'ai' AND status = 'pending'`,
				req.SessionID, grade.ItemID, status, nullableJSON(grade.CorrectAnswer), explanation); err != nil {
				return err
			}
		}
		for _, explanation := range response.Explanations {
			if _, err := tx.Exec(ctx,
				`UPDATE grading_results
				 SET explanation = $3, explanation_source = 'ai', updated_at = now()
				 WHERE session_id = $1 AND item_id = $2 AND source = 'deterministic'
				   AND (explanation IS NULL OR explanation = '')`,
				req.SessionID, explanation.ItemID, strings.TrimSpace(explanation.Text)); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO question_ai_explanations (question_version_id, prompt_version, model, explanation)
				 VALUES ($1, $2, $3, $4)
				 ON CONFLICT (question_version_id) DO UPDATE
				 SET prompt_version = EXCLUDED.prompt_version, model = EXCLUDED.model,
				     explanation = EXCLUDED.explanation, updated_at = now()`,
				versionIDs[explanation.ItemID], batchAnalysisPromptVersion, s.client.cfg.Model, strings.TrimSpace(explanation.Text)); err != nil {
				return err
			}
		}
		if err := practice.NewStore(tx).CompleteIfDone(ctx, tx, req.SessionID); err != nil {
			return err
		}
		if err := practice.NewStore(tx).SetAISummary(ctx, req.SessionID, "completed", summary); err != nil {
			return err
		}
		return learning.NewStore(tx).WriteAIAdviceTx(ctx, tx, userID, resetAt, "completed", summary)
	})
	if err != nil {
		return fmt.Errorf("写回批次 AI 分析失败: %w", err)
	}
	s.logger.Info("ai_batch_analysis_done", "session_id", req.SessionID, "explanations", len(response.Explanations))
	return nil
}

func (s *Service) failBatchAnalysis(ctx context.Context, sessionID string, cause error) error {
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		message := "AI 批次分析失败，稍后可重试。" + shortError(cause)
		if _, err := tx.Exec(ctx,
			`UPDATE grading_results
			 SET status = 'failed', explanation = $2, explanation_source = 'ai', updated_at = now()
			 WHERE session_id = $1 AND source = 'ai' AND status = 'pending'`, sessionID, message); err != nil {
			return err
		}
		if err := practice.NewStore(tx).CompleteIfDone(ctx, tx, sessionID); err != nil {
			return err
		}
		if err := practice.NewStore(tx).SetAISummary(ctx, sessionID, "failed", message); err != nil {
			return err
		}
		userID, resetAt, err := s.sessionMemoryContext(ctx, tx, sessionID)
		if err != nil {
			return err
		}
		return learning.NewStore(tx).WriteAIAdviceTx(ctx, tx, userID, resetAt, "failed", "AI 建议暂时不可用，请稍后重试。")
	})
	if err != nil {
		return err
	}
	return cause
}

func (s *Service) sessionMemoryContext(ctx context.Context, db store.DBTx, sessionID string) (string, any, error) {
	var userID string
	var resetAt *string
	err := db.QueryRow(ctx,
		`SELECT ps.user_id::text, ulm.reset_at::text
		 FROM practice_sessions ps
		 LEFT JOIN user_learning_memory ulm ON ulm.user_id = ps.user_id
		 WHERE ps.id = $1`, sessionID,
	).Scan(&userID, &resetAt)
	if err != nil {
		return "", nil, err
	}
	var resetArg any
	if resetAt != nil {
		resetArg = *resetAt
	}
	return userID, resetArg, nil
}

func jsonValue(value *string) any {
	if value == nil || *value == "" || *value == "null" {
		return nil
	}
	return json.RawMessage(*value)
}

func jsonValueString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// failGrading 把 pending 的 AI 判定标为 failed 并推进批次状态；确定性成绩不受影响。
func (s *Service) failGrading(ctx context.Context, sessionID, itemID string, cause error) error {
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`UPDATE grading_results SET status = 'failed', explanation = $3, explanation_source = 'ai', updated_at = now()
			 WHERE item_id = $1 AND session_id = $2 AND source = 'ai' AND status = 'pending'`,
			itemID, sessionID, "AI 判定失败，稍后可重试。"+shortError(cause)); err != nil {
			return err
		}
		return practice.NewStore(tx).CompleteIfDone(ctx, tx, sessionID)
	})
	if err != nil {
		return err
	}
	return cause
}

func (s *Service) handleExplain(ctx context.Context, attempts, maxAttempts int, payload json.RawMessage) error {
	var req struct {
		SessionID string `json:"sessionId"`
		ItemID    string `json:"itemId"`
	}
	if err := strictDecode(payload, &req); err != nil {
		return err
	}
	item, err := s.loadItem(ctx, s.pool, req.SessionID, req.ItemID)
	if err != nil {
		return fmt.Errorf("加载解析题目失败: %w", err)
	}
	if item.KeyValue == nil {
		return errors.New("解析任务缺少标准答案")
	}
	payloadJSON, _ := json.Marshal(map[string]any{
		"type": item.Type, "stem": item.Stem,
		"options": item.Options, "material": item.Material,
		"standardAnswer": item.KeyValue, "userAnswer": item.UserValue,
	})
	out, err := s.client.RunPrompt(ctx, "practice_explain", explainPromptVersion, item.ItemID, explainPrompt, string(payloadJSON))
	if err != nil {
		return err // 解析失败不影响判分，直接按任务重试策略处理
	}
	var resp struct {
		Explanation string `json:"explanation"`
	}
	if err := strictDecode(out, &resp); err != nil {
		return err
	}
	explanation := strings.TrimSpace(resp.Explanation)
	if explanation == "" || len([]rune(explanation)) > 2000 {
		return errors.New("AI 解析文本缺失或超长")
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE grading_results
		 SET explanation = $3, explanation_source = 'ai', updated_at = now()
		 WHERE item_id = $1 AND session_id = $2 AND source = 'deterministic' AND (explanation IS NULL OR explanation = '')`,
		item.ItemID, item.SessionID, explanation)
	if err != nil {
		return fmt.Errorf("写回 AI 解析失败: %w", err)
	}
	s.logger.Info("ai_explain_done", "item_id", item.ItemID)
	return nil
}

func strictDecode(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func nullableJSON(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return raw
}

func shortError(err error) string {
	msg := err.Error()
	if len(msg) > 120 {
		msg = msg[:120]
	}
	return msg
}
