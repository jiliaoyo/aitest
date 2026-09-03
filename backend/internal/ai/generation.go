package ai

import (
	"context"
	cryptorand "crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/aishuati/backend/internal/content"
	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/httpapi/ctxkeys"
	"github.com/aishuati/backend/internal/jobs"
	"github.com/aishuati/backend/internal/learning"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

const questionGenerationPromptVersion = "practice_question_generation.v1"

//go:embed prompts/practice_question_generation.v1.md
var questionGenerationPrompt string

type AIGenerateRequest struct {
	LevelID           string   `json:"levelId"`
	SubjectID         string   `json:"subjectId"`
	KnowledgePointIDs []string `json:"knowledgePointIds"`
	Count             int      `json:"count"`
}

type AIGeneratedSession struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// RegisterRoutes 提供账号私有的 AI 个性化出题入口；生成结果仍通过普通练习接口答题。
func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/ai-practice-sessions", s.createGeneratedSession)
}

func (s *Service) createGeneratedSession(w http.ResponseWriter, r *http.Request) {
	var req AIGenerateRequest
	if err := httpapi.DecodeJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	session, err := s.CreateGeneratedSession(r.Context(), ctxkeys.UserID(r.Context()), req)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusAccepted, session)
}

func (s *Service) CreateGeneratedSession(ctx context.Context, userID string, req AIGenerateRequest) (AIGeneratedSession, error) {
	if !s.client.Configured() {
		return AIGeneratedSession{}, httpapi.E(http.StatusServiceUnavailable, "ai_unavailable", "AI 出题服务暂不可用")
	}
	if req.Count == 0 {
		req.Count = 10
	}
	if !validGeneratedCount(req.Count) {
		return AIGeneratedSession{}, httpapi.ValidationError(map[string]string{"count": "题量只能是 10、20 或 30"})
	}
	if len(req.KnowledgePointIDs) > 10 {
		return AIGeneratedSession{}, httpapi.ValidationError(map[string]string{"knowledgePointIds": "一次最多选择 10 个知识点"})
	}
	if req.LevelID == "" {
		if err := s.pool.QueryRow(ctx, `SELECT coalesce(default_level_id::text, '') FROM users WHERE id::text = $1`, userID).Scan(&req.LevelID); err != nil {
			return AIGeneratedSession{}, err
		}
	}
	if req.LevelID == "" {
		return AIGeneratedSession{}, httpapi.ValidationError(map[string]string{"levelId": "请选择级别"})
	}
	if err := s.validateGenerationScope(ctx, req); err != nil {
		return AIGeneratedSession{}, err
	}
	scope, _ := json.Marshal(map[string]any{
		"mode":              "ai_generated",
		"subjectId":         req.SubjectID,
		"knowledgePointIds": req.KnowledgePointIDs,
	})
	var out AIGeneratedSession
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var subjectID any
		if req.SubjectID != "" {
			subjectID = req.SubjectID
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO practice_sessions (user_id, status, level_id, subject_id, scope, requested_count)
			 VALUES ($1, 'generating', $2, $3, $4, $5) RETURNING id::text`,
			userID, req.LevelID, subjectID, scope, req.Count).Scan(&out.ID); err != nil {
			return err
		}
		out.Status = "generating"
		return jobs.EnqueueTx(ctx, tx, "generate_ai_practice_session", map[string]string{"sessionId": out.ID})
	})
	return out, err
}

func validGeneratedCount(count int) bool { return count == 10 || count == 20 || count == 30 }

func (s *Service) validateGenerationScope(ctx context.Context, req AIGenerateRequest) error {
	var levelExists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM exam_levels WHERE id::text = $1)`, req.LevelID).Scan(&levelExists); err != nil {
		return err
	}
	if !levelExists {
		return httpapi.ErrNotFound
	}
	if req.SubjectID != "" {
		var scopeExists bool
		if err := s.pool.QueryRow(ctx,
			`SELECT EXISTS(
			   SELECT 1 FROM exam_levels l JOIN subjects sub ON sub.exam_id = l.exam_id
			   WHERE l.id::text = $1 AND sub.id::text = $2)`, req.LevelID, req.SubjectID).Scan(&scopeExists); err != nil {
			return err
		}
		if !scopeExists {
			return httpapi.ErrNotFound
		}
	}
	if len(req.KnowledgePointIDs) > 0 {
		var count int
		if err := s.pool.QueryRow(ctx,
			`SELECT count(*) FROM knowledge_points
			 WHERE id::text = ANY($1::text[]) AND status = 'published'
			   AND level_id::text = $2 AND ($3 = '' OR subject_id::text = $3)`,
			req.KnowledgePointIDs, req.LevelID, req.SubjectID).Scan(&count); err != nil {
			return err
		}
		if count != len(uniqueStrings(req.KnowledgePointIDs)) {
			return httpapi.ErrNotFound
		}
		return nil
	}
	var count int
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM knowledge_points
		 WHERE status = 'published' AND level_id::text = $1 AND ($2 = '' OR subject_id::text = $2)`,
		req.LevelID, req.SubjectID).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return httpapi.E(http.StatusConflict, "no_knowledge_points", "当前级别暂无可用于 AI 出题的知识点")
	}
	return nil
}

func uniqueStrings(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

type generationJobRequest struct {
	SessionID string `json:"sessionId"`
}

type questionGenerationInput struct {
	Count          int                         `json:"count"`
	LevelID        string                      `json:"levelId"`
	SubjectID      string                      `json:"subjectId,omitempty"`
	RandomSeed     string                      `json:"randomSeed"`
	LearningMemory learning.AIGenerationMemory `json:"learningMemory"`
}

type generatedOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Text  string `json:"text"`
}

type generatedQuestion struct {
	Type              string            `json:"type"`
	Stem              string            `json:"stem"`
	Options           []generatedOption `json:"options"`
	CorrectAnswer     json.RawMessage   `json:"correctAnswer"`
	Explanation       string            `json:"explanation"`
	KnowledgePointIDs []string          `json:"knowledgePointIds"`
	Difficulty        int               `json:"difficulty"`
}

type generatedQuestionResponse struct {
	Questions []generatedQuestion `json:"questions"`
}

type generationSessionRow struct {
	UserID         string
	LevelID        string
	SubjectID      *string
	RequestedCount int
	Scope          string
	Status         string
}

func (s *Service) handleGenerate(ctx context.Context, attempts, maxAttempts int, payload json.RawMessage) error {
	var req generationJobRequest
	if err := strictDecode(payload, &req); err != nil {
		return err
	}
	var row generationSessionRow
	err := s.pool.QueryRow(ctx,
		`SELECT user_id::text, level_id::text, subject_id::text, requested_count, scope::text, status
		 FROM practice_sessions WHERE id = $1`, req.SessionID,
	).Scan(&row.UserID, &row.LevelID, &row.SubjectID, &row.RequestedCount, &row.Scope, &row.Status)
	if errors.Is(err, pgx.ErrNoRows) || row.Status == "active" {
		return nil
	}
	if err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
	}
	if row.Status != "generating" {
		return nil
	}
	var scope struct {
		Mode              string   `json:"mode"`
		SubjectID         string   `json:"subjectId"`
		KnowledgePointIDs []string `json:"knowledgePointIds"`
	}
	if err := strictDecode([]byte(row.Scope), &scope); err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, fmt.Errorf("解析 AI 出题范围失败: %w", err))
	}
	subjectID := ""
	if row.SubjectID != nil {
		subjectID = *row.SubjectID
	}
	memory, err := learning.NewStore(s.pool).GenerationMemoryForAI(ctx, row.UserID, row.LevelID, subjectID, scope.KnowledgePointIDs)
	if err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, fmt.Errorf("读取 AI 出题记忆失败: %w", err))
	}
	if len(memory.KnowledgePoints) == 0 {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, errors.New("没有可用于 AI 出题的已审核知识点"))
	}
	seed, err := randomSeed()
	if err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
	}
	inputJSON, _ := json.Marshal(questionGenerationInput{
		Count: row.RequestedCount, LevelID: row.LevelID, SubjectID: subjectID,
		RandomSeed: seed, LearningMemory: memory,
	})
	out, err := s.client.RunPromptWithTemperature(ctx, "practice_question_generation", questionGenerationPromptVersion,
		req.SessionID, questionGenerationPrompt, string(inputJSON), 0.8)
	if err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
	}
	var response generatedQuestionResponse
	if err := strictDecode(out, &response); err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, fmt.Errorf("AI 出题输出不合法: %w", err))
	}
	if err := validateGeneratedQuestions(response.Questions, row.RequestedCount, memory.KnowledgePoints); err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
	}
	if err := s.persistGeneratedQuestions(ctx, req.SessionID, row.UserID, row.LevelID, subjectID, memory.KnowledgePoints, response.Questions); err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, fmt.Errorf("保存 AI 题目失败: %w", err))
	}
	s.logger.Info("ai_generated_practice_done", "session_id", req.SessionID, "count", len(response.Questions))
	return nil
}

func randomSeed() (string, error) {
	var seed [16]byte
	if _, err := cryptorand.Read(seed[:]); err != nil {
		return "", fmt.Errorf("生成随机种子失败: %w", err)
	}
	return hex.EncodeToString(seed[:]), nil
}

func validateGeneratedQuestions(questions []generatedQuestion, expected int, points []learning.AIGenerationKnowledgePoint) error {
	if len(questions) != expected {
		return fmt.Errorf("AI 出题数量不正确：需要 %d 道，实际 %d 道", expected, len(questions))
	}
	allowed := make(map[string]bool, len(points))
	for _, point := range points {
		allowed[point.ID] = true
	}
	seenStems := map[string]bool{}
	for i, question := range questions {
		if question.Type != "single_choice" || len([]rune(strings.TrimSpace(question.Stem))) < 2 {
			return fmt.Errorf("AI 第 %d 题题型或题干不合法", i+1)
		}
		stem := strings.TrimSpace(question.Stem)
		if seenStems[stem] {
			return fmt.Errorf("AI 第 %d 题与其他题目重复", i+1)
		}
		seenStems[stem] = true
		if len(question.Options) != 4 {
			return fmt.Errorf("AI 第 %d 题必须有 4 个选项", i+1)
		}
		options := make([]content.Option, 0, len(question.Options))
		seenOptions := map[string]bool{}
		for _, option := range question.Options {
			if option.ID == "" || seenOptions[option.ID] || strings.TrimSpace(option.Text) == "" {
				return fmt.Errorf("AI 第 %d 题选项不合法", i+1)
			}
			seenOptions[option.ID] = true
			options = append(options, content.Option{ID: option.ID, Label: option.Label, Text: option.Text})
		}
		if err := content.ValidateAnswerValue(question.Type, options, question.CorrectAnswer); err != nil {
			return fmt.Errorf("AI 第 %d 题答案不合法: %w", i+1, err)
		}
		if question.Difficulty < 1 || question.Difficulty > 5 || strings.TrimSpace(question.Explanation) == "" || len([]rune(question.Explanation)) > 2000 {
			return fmt.Errorf("AI 第 %d 题难度或解析不合法", i+1)
		}
		if len(question.KnowledgePointIDs) == 0 {
			return fmt.Errorf("AI 第 %d 题缺少知识点", i+1)
		}
		for _, pointID := range question.KnowledgePointIDs {
			if !allowed[pointID] {
				return fmt.Errorf("AI 第 %d 题引用了未审核知识点", i+1)
			}
		}
	}
	return nil
}

func (s *Service) persistGeneratedQuestions(ctx context.Context, sessionID, userID, levelID, subjectID string, points []learning.AIGenerationKnowledgePoint, questions []generatedQuestion) error {
	return store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		pointSubjects := make(map[string]string, len(points))
		for _, point := range points {
			pointSubjects[point.ID] = point.SubjectID
		}
		var sourceID, sectionID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO sources (name, kind, author, internal_note, created_by)
			 VALUES ('AI 个性化练习', 'ai_generated', 'AI', '账号私有生成题目，未经人工审核，不进入普通题库。', $1)
			 RETURNING id::text`, userID).Scan(&sourceID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO source_sections (source_id, name, sort_order) VALUES ($1, '根据全局记忆生成', 1) RETURNING id::text`, sourceID).Scan(&sectionID); err != nil {
			return err
		}
		for i, question := range questions {
			optionsJSON, err := json.Marshal(question.Options)
			if err != nil {
				return err
			}
			var questionID, versionID string
			if err := tx.QueryRow(ctx,
				`INSERT INTO questions (status, has_answer, created_by)
				 VALUES ('draft', false, $1) RETURNING id::text`, userID).Scan(&questionID); err != nil {
				return err
			}
			questionSubjectID := subjectID
			if questionSubjectID == "" {
				for _, pointID := range question.KnowledgePointIDs {
					if pointSubject := pointSubjects[pointID]; pointSubject != "" {
						questionSubjectID = pointSubject
						break
					}
				}
			}
			if questionSubjectID == "" {
				return errors.New("AI 题目无法确定科目")
			}
			if err := tx.QueryRow(ctx,
				`INSERT INTO question_versions
				 (question_id, version_no, type, stem, options, level_id, subject_id, source_section_id, difficulty, source_order, created_by)
				 VALUES ($1, 1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
				 RETURNING id::text`, questionID, question.Type, strings.TrimSpace(question.Stem), optionsJSON,
				levelID, questionSubjectID, sectionID, question.Difficulty, i+1, userID).Scan(&versionID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`UPDATE questions SET current_version_id = $2, updated_at = now() WHERE id = $1`, questionID, versionID); err != nil {
				return err
			}
			for _, pointID := range question.KnowledgePointIDs {
				if _, err := tx.Exec(ctx,
					`INSERT INTO question_version_knowledge_points (question_version_id, knowledge_point_id) VALUES ($1, $2)`, versionID, pointID); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO ai_generated_question_answers (question_version_id, value, explanation, prompt_version, model)
				 VALUES ($1, $2, $3, $4, $5)`, versionID, question.CorrectAnswer, strings.TrimSpace(question.Explanation), questionGenerationPromptVersion, s.client.cfg.Model); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO practice_items (session_id, question_id, question_version_id, position) VALUES ($1, $2, $3, $4)`,
				sessionID, questionID, versionID, i+1); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx,
			`UPDATE practice_sessions SET status = 'active', updated_at = now() WHERE id = $1 AND status = 'generating'`, sessionID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO audit_logs (actor_user_id, action, object_type, object_id, detail)
			 VALUES ($1, 'ai_practice_generated', 'practice_session', $2, jsonb_build_object('count', $3::int))`, userID, sessionID, len(questions))
		return err
	})
}

func (s *Service) generationRetry(ctx context.Context, sessionID string, attempts, maxAttempts int, cause error) error {
	if attempts < maxAttempts {
		return cause
	}
	if err := s.markGenerationFailed(ctx, sessionID, cause); err != nil {
		return fmt.Errorf("标记 AI 出题失败失败: %v（原错误：%w）", err, cause)
	}
	return cause
}

func (s *Service) markGenerationFailed(ctx context.Context, sessionID string, cause error) error {
	message := "AI 出题失败，请重新开始。"
	if cause != nil {
		message += shortError(cause)
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE practice_sessions
		 SET status = 'generation_failed', ai_summary_status = 'failed', ai_summary = $2, updated_at = now()
		 WHERE id = $1 AND status = 'generating'`, sessionID, message)
	return err
}
