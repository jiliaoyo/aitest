package practice

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

type Store struct{ db store.DBTx }

func NewStore(db store.DBTx) *Store { return &Store{db: db} }

// With 返回绑定到事务的视图。
func (s *Store) With(db store.DBTx) *Store { return &Store{db: db} }

// ---------- 创建批次 ----------

func (s *Store) InsertSession(ctx context.Context, tx pgx.Tx, userID, levelID string, subjectID *string, scope []byte, requestedCount int) (string, error) {
	var id string
	err := tx.QueryRow(ctx,
		`INSERT INTO practice_sessions (user_id, level_id, subject_id, scope, requested_count)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
		userID, levelID, subjectID, scope, requestedCount).Scan(&id)
	return id, err
}

func (s *Store) InsertItems(ctx context.Context, tx pgx.Tx, sessionID string, items []ItemSeed) error {
	for i, it := range items {
		if _, err := tx.Exec(ctx,
			`INSERT INTO practice_items (session_id, question_id, question_version_id, position)
			 VALUES ($1, $2, $3, $4)`,
			sessionID, it.QuestionID, it.VersionID, i+1); err != nil {
			return err
		}
	}
	return nil
}

// LockUser 串行化同一账号的复习批次创建，避免并发请求选到同一组到期题。
func (s *Store) LockUser(ctx context.Context, tx pgx.Tx, userID string) error {
	var id string
	return tx.QueryRow(ctx, `SELECT id::text FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&id)
}

func (s *Store) ActiveReviewSessionID(ctx context.Context, userID string) (string, error) {
	var id string
	err := s.db.QueryRow(ctx, `
		SELECT id::text
		FROM practice_sessions
		WHERE user_id = $1 AND status IN ('active', 'grading') AND deleted_at IS NULL
		  AND scope->>'mode' = 'review'
		ORDER BY created_at DESC
		LIMIT 1`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// DueReviewItems 复用触发复习计划的最后一次有效作答版本，避免发布新版本后错配旧计划。
func (s *Store) DueReviewItems(ctx context.Context, userID, levelID, subjectID string, limit int) ([]ItemSeed, error) {
	return store.CollectRows[ItemSeed](ctx, s.db, `
		WITH due AS (
		  SELECT r.question_id,
		         history_item.question_version_id AS version_id,
		         r.next_review_at,
		         CASE WHEN history_source.kind = 'ai_generated' OR last_result.source = 'ai' THEN 1 ELSE 0 END AS source_priority
		  FROM user_question_reviews r
		  JOIN grading_results last_result ON last_result.id = r.last_grading_result_id
		  JOIN practice_items history_item ON history_item.id = last_result.item_id
		  JOIN practice_sessions history_session
		    ON history_session.id = history_item.session_id AND history_session.user_id = r.user_id
		  JOIN question_versions history_version ON history_version.id = history_item.question_version_id
		  LEFT JOIN source_sections history_section ON history_section.id = history_version.source_section_id
		  LEFT JOIN sources history_source ON history_source.id = history_section.source_id
		  JOIN questions q ON q.id = r.question_id
		  WHERE r.user_id = $1 AND r.next_review_at <= now() AND q.retired_at IS NULL
		    AND (history_source.kind = 'ai_generated' OR q.published_version_id IS NOT NULL)
		)
		SELECT due.question_id::text, due.version_id::text
		FROM due
		JOIN question_versions v ON v.id = due.version_id
		WHERE v.level_id::text = $2 AND ($3 = '' OR v.subject_id::text = $3)
		ORDER BY due.next_review_at, due.source_priority, due.question_id
		LIMIT CASE WHEN $4 > 0 THEN $4 ELSE NULL END`, userID, levelID, subjectID, limit)
}

type ItemSeed struct {
	QuestionID string
	VersionID  string
}

// ---------- 查询 ----------

type SessionMeta struct {
	ID                   string
	UserID               string
	Status               string
	SubmitKey            *string
	SubmitHash           *string
	CreatedAt            string
	SubmittedAt          *string
	CompletedAt          *string
	AISummary            string
	AISummaryStatus      string
	GenerationCallsUsed  int
	GenerationCallBudget int
	GenerationLastError  string
}

// SessionMetaForUser 按 (id, user_id) 查询；不匹配一律当作不存在，防止越权探测。
func (s *Store) SessionMetaForUser(ctx context.Context, sessionID, userID string) (SessionMeta, error) {
	var m SessionMeta
	err := s.db.QueryRow(ctx,
		`SELECT id::text, user_id::text, status, submit_key, submit_hash,
		        created_at::text, submitted_at::text, completed_at::text,
		        ai_summary, ai_summary_status, ai_generation_calls_used,
		        ai_generation_call_budget, ai_generation_last_error
		 FROM practice_sessions WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`,
		sessionID, userID,
	).Scan(&m.ID, &m.UserID, &m.Status, &m.SubmitKey, &m.SubmitHash, &m.CreatedAt, &m.SubmittedAt, &m.CompletedAt, &m.AISummary, &m.AISummaryStatus, &m.GenerationCallsUsed, &m.GenerationCallBudget, &m.GenerationLastError)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionMeta{}, httpapi.ErrNotFound
	}
	return m, err
}

type preItemRow struct {
	ID                string
	Position          int
	Type              string
	Stem              string
	SourceSectionName *string
	Options           *string
	MaterialID        *string
	MaterialTItle     *string
	MaterialContent   *string
	SavedAnswer       *string
	Marked            *bool
	SavedAt           *string
}

func (s *Store) PreSubmitItems(ctx context.Context, sessionID string) ([]PreSubmitItem, int, error) {
	rows, err := store.CollectRows[preItemRow](ctx, s.db,
		`SELECT pi.id::text, pi.position, v.type, v.stem, ss.name, v.options::text,
		        mv.material_id::text, mv.title, mv.content,
		        ua.value::text, ua.marked_for_review, ua.saved_at::text
		 FROM practice_items pi
		 JOIN question_versions v ON v.id = pi.question_version_id
		 LEFT JOIN source_sections ss ON ss.id = v.source_section_id
		 LEFT JOIN material_versions mv ON mv.id = v.material_version_id
		 LEFT JOIN user_answers ua ON ua.item_id = pi.id
		 WHERE pi.session_id = $1
		 ORDER BY pi.position`, sessionID)
	if err != nil {
		return nil, 0, err
	}
	items := make([]PreSubmitItem, 0, len(rows))
	answered := 0
	for _, r := range rows {
		item := PreSubmitItem{
			ID:       r.ID,
			Position: r.Position,
			Type:     r.Type,
			Stem:     r.Stem,
			Options:  []PreSubmitOption{},
		}
		if r.SourceSectionName != nil {
			item.SourceSectionName = *r.SourceSectionName
		}
		if r.Options != nil && *r.Options != "null" {
			var opts []PreSubmitOption
			if err := jsonUnmarshal(*r.Options, &opts); err == nil && opts != nil {
				item.Options = opts
			}
		}
		if r.MaterialID != nil {
			item.Material = &PreSubmitMaterial{ID: *r.MaterialID}
			if r.MaterialTItle != nil {
				item.Material.Title = *r.MaterialTItle
			}
			if r.MaterialContent != nil {
				item.Material.Content = *r.MaterialContent
			}
		}
		if r.SavedAnswer != nil {
			item.SavedAnswer = jsonRaw(*r.SavedAnswer)
			if string(item.SavedAnswer) != "null" {
				answered++
			}
		}
		if r.Marked != nil && *r.Marked {
			item.MarkedForReview = true
		}
		item.SavedAt = r.SavedAt
		items = append(items, item)
	}
	return items, answered, nil
}

// UpsertAnswer 仅在批次 active 且 item 属于该批次时写入；返回是否写入与 savedAt。
func (s *Store) UpsertAnswer(ctx context.Context, tx pgx.Tx, sessionID, itemID, userID string, value []byte, marked bool) (bool, string, error) {
	var savedAt string
	err := tx.QueryRow(ctx,
		`WITH locked AS (
		   SELECT 1 FROM practice_sessions
		   WHERE id = $1 AND user_id = $3 AND status = 'active' FOR UPDATE
		 ), item AS (
		   SELECT 1 FROM practice_items WHERE id = $2 AND session_id = $1
		 ), upsert AS (
		   INSERT INTO user_answers (session_id, item_id, user_id, value, marked_for_review)
		   SELECT $1, $2, $3, $4, $5 FROM locked JOIN item ON true
		   ON CONFLICT (item_id) DO UPDATE
		     SET value = $4, marked_for_review = $5, saved_at = now(), updated_at = now()
		   RETURNING saved_at::text
		 )
		 SELECT coalesce((SELECT saved_at FROM upsert), '')`,
		sessionID, itemID, userID, value, marked,
	).Scan(&savedAt)
	if err != nil {
		return false, "", err
	}
	return savedAt != "", savedAt, nil
}

// ---------- 提交与判分 ----------

type gradeItemRow struct {
	ItemID               string
	Position             int
	Type                 string
	OptionsText          *string
	KeyValue             *string
	KeyAuthority         *string
	Explanation          *string
	GeneratedValue       *string
	GeneratedExplanation *string
}

// GradeSourceItems 加载批次内全部题目及标准答案（仅提交事务内使用）。
func (s *Store) GradeSourceItems(ctx context.Context, tx pgx.Tx, sessionID string) ([]gradeItemRow, error) {
	return store.CollectRows[gradeItemRow](ctx, tx,
		`SELECT pi.id::text, pi.position, v.type, v.options::text,
		        ak.value::text, ak.authority, ak.explanation,
		        aga.value::text, aga.explanation
		 FROM practice_items pi
		 JOIN question_versions v ON v.id = pi.question_version_id
		 LEFT JOIN answer_keys ak ON ak.question_version_id = v.id
		 LEFT JOIN ai_generated_question_answers aga ON aga.question_version_id = v.id
		 WHERE pi.session_id = $1
		 ORDER BY pi.position`, sessionID)
}

type recoverAIGeneratedGradeRow struct {
	ResultID    string
	Type        string
	UserValue   *string
	AnswerValue string
	Explanation string
}

// RecoverAIGeneratedObjectiveGrades 用生成时私下保存的答案恢复旧版失败的 AI 客观题判分。
func (s *Store) RecoverAIGeneratedObjectiveGrades(ctx context.Context, tx pgx.Tx, userID string) (int, error) {
	rows, err := store.CollectRows[recoverAIGeneratedGradeRow](ctx, tx, `
		SELECT gr.id::text, v.type, coalesce(ua.value, gr.user_value)::text, aga.value::text, aga.explanation
		FROM grading_results gr
		JOIN practice_items pi ON pi.id = gr.item_id
		JOIN practice_sessions ps ON ps.id = pi.session_id
		JOIN question_versions v ON v.id = pi.question_version_id
		JOIN ai_generated_question_answers aga ON aga.question_version_id = v.id
		LEFT JOIN user_answers ua ON ua.item_id = pi.id
		WHERE ps.user_id = $1 AND ps.status IN ('completed', 'analysis_failed')
		  AND gr.source = 'ai' AND gr.status = 'failed'
		  AND v.type IN ('single_choice', 'multiple_choice', 'fill_blank')`, userID)
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		var userValue json.RawMessage
		if row.UserValue != nil {
			userValue = jsonRaw(*row.UserValue)
		}
		outcome := Grade(row.Type, userValue, &StandardKey{Value: jsonRaw(row.AnswerValue)})
		if _, err := tx.Exec(ctx, `
			UPDATE grading_results
			SET status = $2, correct_value = $3, explanation = $4,
			    explanation_source = 'ai', updated_at = now()
			WHERE id = $1 AND source = 'ai' AND status = 'failed'`,
			row.ResultID, outcome.Status, outcome.CorrectValue, row.Explanation); err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}

func (s *Store) InsertGrading(ctx context.Context, tx pgx.Tx, g GradingInsert) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO grading_results
		   (session_id, item_id, source, status, answer_authority, correct_value, user_value, explanation, explanation_source)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		g.SessionID, g.ItemID, g.Source, g.Status, g.Authority, g.CorrectValue, g.UserValue, g.Explanation, g.ExplanationSource)
	return err
}

type GradingInsert struct {
	SessionID         string
	ItemID            string
	Source            string
	Status            string
	Authority         *string
	CorrectValue      []byte
	UserValue         []byte
	Explanation       *string
	ExplanationSource *string
}

// SubmitFinalAnswers 用提交请求中的最终答案覆盖自动保存（同一事务内）。
func (s *Store) SubmitFinalAnswers(ctx context.Context, tx pgx.Tx, sessionID, userID string, answers []SubmittedAnswer) error {
	for _, a := range answers {
		var value []byte
		if len(a.Value) > 0 && string(a.Value) != "null" {
			value = a.Value
		}
		if _, _, err := s.UpsertAnswer(ctx, tx, sessionID, a.ItemID, userID, value, a.MarkedForReview); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) MarkSubmitted(ctx context.Context, tx pgx.Tx, sessionID, submitKey, submitHash string) error {
	_, err := tx.Exec(ctx,
		`UPDATE practice_sessions
		 SET status = 'grading', ai_summary_status = 'pending', ai_summary = '',
		     submit_key = $2, submit_hash = $3, submitted_at = now(), updated_at = now()
		 WHERE id = $1`, sessionID, submitKey, submitHash)
	return err
}

func (s *Store) HasActiveBatchAnalysisJob(ctx context.Context, tx pgx.Tx, sessionID string) (bool, error) {
	return store.Exists(ctx, tx,
		`SELECT true FROM jobs
		 WHERE kind = 'analyze_practice_session_ai'
		   AND status IN ('queued', 'running')
		   AND payload->>'sessionId' = $1
		 LIMIT 1`, sessionID)
}

func (s *Store) ResetAIAnalysisForRetry(ctx context.Context, tx pgx.Tx, sessionID string) error {
	if _, err := tx.Exec(ctx,
		`UPDATE grading_results
		 SET status = 'pending', correct_value = NULL, explanation = NULL,
		     explanation_source = NULL, updated_at = now()
		 WHERE session_id = $1 AND source = 'ai' AND status = 'failed'`, sessionID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx,
		`UPDATE practice_sessions
		 SET status = 'grading', ai_summary_status = 'pending', ai_summary = '',
		     completed_at = NULL, updated_at = now()
		 WHERE id = $1`, sessionID)
	return err
}

// SetSessionStatus 仅当当前处于中间态时迁移，保证幂等。
func (s *Store) SetSessionStatus(ctx context.Context, tx pgx.Tx, sessionID, status string) error {
	_, err := tx.Exec(ctx,
		`UPDATE practice_sessions SET status = $2, completed_at = CASE WHEN $2 IN ('completed','analysis_failed') THEN now() ELSE completed_at END,
		 updated_at = now() WHERE id = $1`, sessionID, status)
	return err
}

// CompleteIfDone 当批次没有 pending 判分时结束批次；有 failed 则 analysis_failed。
func (s *Store) CompleteIfDone(ctx context.Context, tx pgx.Tx, sessionID string) error {
	_, err := tx.Exec(ctx,
		`UPDATE practice_sessions ps
		 SET status = CASE WHEN EXISTS (
		       SELECT 1 FROM grading_results g WHERE g.session_id = ps.id AND g.status = 'pending'
		     ) THEN ps.status
		     WHEN EXISTS (
		       SELECT 1 FROM grading_results g WHERE g.session_id = ps.id AND g.status = 'failed'
		     ) THEN 'analysis_failed'
		     ELSE 'completed' END,
		 completed_at = now(), updated_at = now()
		 WHERE ps.id = $1 AND ps.status = 'grading'
		   AND NOT EXISTS (SELECT 1 FROM grading_results g WHERE g.session_id = ps.id AND g.status = 'pending')`,
		sessionID)
	return err
}

// ---------- 结果与历史 ----------

type resultItemRow struct {
	ID                string
	Position          int
	Type              string
	Stem              string
	SourceSectionName *string
	OptionsText       *string
	MaterialID        *string
	MaterialTitle     *string
	MaterialContent   *string
	KPID              *string
	KPName            *string
	Source            string
	Status            string
	Authority         *string
	CorrectValue      *string
	UserValue         *string
	Explanation       *string
	ExplanationSource *string
}

func (s *Store) ResultRows(ctx context.Context, sessionID string) ([]resultItemRow, error) {
	return store.CollectRows[resultItemRow](ctx, s.db,
		`SELECT pi.id::text, pi.position, v.type, v.stem, ss.name, v.options::text,
		        mv.material_id::text, mv.title, mv.content,
		        kp.id::text, kp.name,
		        gr.source, gr.status, gr.answer_authority,
		        gr.correct_value::text, gr.user_value::text, gr.explanation, gr.explanation_source
		 FROM practice_items pi
		 JOIN question_versions v ON v.id = pi.question_version_id
		 LEFT JOIN source_sections ss ON ss.id = v.source_section_id
		 LEFT JOIN material_versions mv ON mv.id = v.material_version_id
		 LEFT JOIN question_version_knowledge_points qvkp ON qvkp.question_version_id = v.id
		 LEFT JOIN knowledge_points kp ON kp.id = qvkp.knowledge_point_id
		 JOIN grading_results gr ON gr.item_id = pi.id
		 WHERE pi.session_id = $1
		 ORDER BY pi.position`, sessionID)
}

type summaryRow struct {
	ConfirmedTotal   int
	ConfirmedCorrect int
	AiCompleted      int
	AiCorrect        int
	AiPending        int
	AiFailed         int
}

func (s *Store) Summary(ctx context.Context, sessionID string) (summaryRow, error) {
	var r summaryRow
	err := s.db.QueryRow(ctx,
		`SELECT
		   count(*) FILTER (WHERE source = 'deterministic' AND answer_authority IS NOT NULL AND status IN ('correct','incorrect','unanswered')),
		   count(*) FILTER (WHERE source = 'deterministic' AND answer_authority IS NOT NULL AND status = 'correct'),
		   count(*) FILTER (WHERE source = 'ai' AND status IN ('correct','incorrect','unanswered')),
		   count(*) FILTER (WHERE source = 'ai' AND status = 'correct'),
		   count(*) FILTER (WHERE source = 'ai' AND status = 'pending'),
		   count(*) FILTER (WHERE source = 'ai' AND status = 'failed')
		 FROM grading_results WHERE session_id = $1`, sessionID,
	).Scan(&r.ConfirmedTotal, &r.ConfirmedCorrect, &r.AiCompleted, &r.AiCorrect, &r.AiPending, &r.AiFailed)
	return r, err
}

func (s *Store) HasPendingAIJobs(ctx context.Context, sessionID string) (bool, error) {
	return store.Exists(ctx, s.db,
		`SELECT true FROM jobs
		 WHERE status IN ('queued', 'running')
		   AND kind IN ('grade_practice_item_ai', 'explain_practice_item_ai', 'analyze_practice_session_ai')
		   AND payload->>'sessionId' = $1
		 LIMIT 1`, sessionID)
}

func (s *Store) SetAISummary(ctx context.Context, sessionID, status, summary string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE practice_sessions
		 SET ai_summary_status = $2, ai_summary = $3, updated_at = now()
		 WHERE id = $1`, sessionID, status, summary)
	return err
}

type sessionListRow struct {
	ID          string
	Status      string
	Mode        string
	TotalCount  int
	CreatedAt   string
	SubmittedAt *string
}

func (s *Store) ListSessions(ctx context.Context, userID, status, mode, cursor string, limit int) ([]SessionListItem, string, error) {
	args := []any{userID}
	conds := []string{"ps.user_id = $1", "ps.deleted_at IS NULL"}
	if status != "" {
		args = append(args, status)
		conds = append(conds, "ps.status = $"+store.Itoa(len(args)))
	}
	if mode != "" {
		args = append(args, mode)
		conds = append(conds, "coalesce(ps.scope->>'mode', 'comprehensive') = $"+store.Itoa(len(args)))
	}
	if cursor != "" {
		args = append(args, cursor)
		conds = append(conds, "ps.created_at < $"+store.Itoa(len(args))+"::timestamptz")
	}
	args = append(args, limit)
	limitPh := "$" + store.Itoa(len(args))
	rows, err := store.CollectRows[sessionListRow](ctx, s.db,
		`SELECT ps.id::text, ps.status,
		        coalesce(ps.scope->>'mode', 'comprehensive'),
		        (SELECT count(*) FROM practice_items pi WHERE pi.session_id = ps.id) AS total_count,
		        ps.created_at::text, ps.submitted_at::text
		 FROM practice_sessions ps
		 WHERE `+joinConds(conds)+`
		 ORDER BY ps.created_at DESC
		 LIMIT `+limitPh, args...)
	if err != nil {
		return nil, "", err
	}
	out := make([]SessionListItem, 0, len(rows))
	next := ""
	for _, r := range rows {
		out = append(out, SessionListItem{
			ID: r.ID, Status: r.Status, Mode: r.Mode, TotalCount: r.TotalCount,
			CreatedAt: r.CreatedAt, SubmittedAt: r.SubmittedAt,
		})
		next = r.CreatedAt
	}
	if len(rows) < limit {
		next = ""
	}
	return out, next, nil
}

type WrongQuestionFilter struct {
	UserID            string
	LevelID           string
	SubjectID         string
	SourceID          string
	SourceSectionID   string
	KnowledgePointIDs []string
	FromDate          string
	ToDate            string
	Keyword           string
}

// WrongQuestionIDs 先应用当前可练范围，再返回最近一次可判定作答仍为错误的题目 ID。
func (s *Store) WrongQuestionIDs(ctx context.Context, f WrongQuestionFilter) ([]string, error) {
	args := []any{f.UserID, f.LevelID, f.SubjectID, f.SourceID, f.SourceSectionID}
	kpIDs := f.KnowledgePointIDs
	if kpIDs == nil {
		kpIDs = []string{}
	}
	args = append(args, kpIDs)
	where := ""
	if f.FromDate != "" {
		args = append(args, f.FromDate)
		where += " AND r.at_time >= $" + strconv.Itoa(len(args)) + "::date"
	}
	if f.ToDate != "" {
		args = append(args, f.ToDate)
		where += " AND r.at_time < ($" + strconv.Itoa(len(args)) + "::date + interval '1 day')"
	}
	if f.Keyword != "" {
		args = append(args, "%"+f.Keyword+"%")
		p := "$" + strconv.Itoa(len(args))
		where += ` AND (v.stem ILIKE ` + p + ` OR coalesce(v.options::text, '') ILIKE ` + p + `
		  OR coalesce(mv.content, '') ILIKE ` + p + ` OR EXISTS (
		    SELECT 1 FROM question_version_knowledge_points qvkp2
		    JOIN knowledge_points kp2 ON kp2.id = qvkp2.knowledge_point_id
		    WHERE qvkp2.question_version_id = v.id AND kp2.name ILIKE ` + p + `))`
	}
	rows, err := store.CollectRows[struct{ ID string }](ctx, s.db,
		`WITH eligible AS (
		   SELECT pi.question_id, pi.position, gr.id AS result_id, gr.status,
		          COALESCE(ps.submitted_at, ps.created_at) AS at_time
		   FROM grading_results gr
		   JOIN practice_items pi ON pi.id = gr.item_id
		   JOIN practice_sessions ps ON ps.id = pi.session_id
		   LEFT JOIN user_learning_memory mem ON mem.user_id = ps.user_id
		   WHERE ps.user_id = $1 AND ps.deleted_at IS NULL AND pi.deleted_at IS NULL
		     AND (mem.reset_at IS NULL OR COALESCE(ps.submitted_at, ps.created_at) > mem.reset_at)
		     AND ((gr.source = 'deterministic' AND gr.answer_authority IS NOT NULL
		           AND gr.status IN ('correct', 'incorrect', 'unanswered'))
		       OR (gr.source = 'ai' AND gr.status IN ('correct', 'incorrect')))
		 ), ranked AS (
		   SELECT eligible.*,
		          row_number() OVER (PARTITION BY question_id ORDER BY at_time DESC, position DESC, result_id DESC) AS rn
		   FROM eligible
		 )
		 SELECT r.question_id::text FROM ranked r
		 JOIN questions q ON q.id = r.question_id
		 JOIN question_versions v ON v.id = q.published_version_id
		 LEFT JOIN material_versions mv ON mv.id = v.material_version_id
		 LEFT JOIN source_sections ss ON ss.id = v.source_section_id
		 LEFT JOIN sources src ON src.id = ss.source_id
		 WHERE r.rn = 1 AND r.status IN ('incorrect', 'unanswered')
		   AND q.retired_at IS NULL AND coalesce(src.kind, '') <> 'ai_generated'
		   AND v.level_id::text = $2
		   AND ($3 = '' OR v.subject_id::text = $3)
		   AND ($4 = '' OR ss.source_id::text = $4)
		   AND ($5 = '' OR v.source_section_id::text = $5)
		   AND ($6::uuid[] = '{}' OR EXISTS (
		     SELECT 1 FROM question_version_knowledge_points qvkp
		     WHERE qvkp.question_version_id = v.id AND qvkp.knowledge_point_id = ANY($6::uuid[])))`+where+`
		 ORDER BY r.at_time DESC, r.question_id DESC`, args...)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

func joinConds(conds []string) string {
	out := ""
	for i, c := range conds {
		if i > 0 {
			out += " AND "
		}
		out += "(" + c + ")"
	}
	return out
}

func jsonUnmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }

func jsonRaw(s string) json.RawMessage { return json.RawMessage(s) }
