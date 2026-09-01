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
	"github.com/aishuati/backend/internal/practice"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	gradePromptVersion   = "practice_grade.v1"
	explainPromptVersion = "practice_explain.v1"
)

//go:embed prompts/practice_grade.v1.md
var gradePrompt string

//go:embed prompts/practice_explain.v1.md
var explainPrompt string

type Service struct {
	pool    *pgxpool.Pool
	client  *Client
	logger  *slog.Logger
}

func NewService(pool *pgxpool.Pool, client *Client, logger *slog.Logger) *Service {
	return &Service{pool: pool, client: client, logger: logger}
}

// Handlers 返回 AI 相关任务处理器，供 worker 注册。
func (s *Service) Handlers() map[string]jobs.Handler {
	return map[string]jobs.Handler{
		"grade_practice_item_ai":   s.handleGrade,
		"explain_practice_item_ai": s.handleExplain,
	}
}

type itemContext struct {
	ItemID      string
	SessionID   string
	Type        string
	Stem        string
	Options     *string
	Material    *string
	UserValue   *string
	KeyValue    *string
	KeyAuthority *string
	Explanation *string
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
