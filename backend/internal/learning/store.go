package learning

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ db store.DBTx }

func NewStore(db store.DBTx) *Store { return &Store{db: db} }

// With 返回绑定到事务的视图（统计重算在 job 事务中执行）。
func (s *Store) With(db store.DBTx) *Store { return &Store{db: db} }

// ---------- 知识点（学习端视角） ----------

type kpStatRow struct {
	ID                string
	Name              string
	LevelID           string
	LevelCode         string
	SubjectID         string
	SubjectName       string
	SubjectSortOrder  int
	ParentID          *string
	QuestionCount     int
	ConfirmedAnswered int
	ConfirmedCorrect  int
	RecentAnswered    int
	RecentCorrect     int
	AIAnswered        int
	AICorrect         int
	ConsecutiveWrong  int
	LastPracticedAt   *string
	StatsFound        bool
}

func (r kpStatRow) toItem() KnowledgePointItem {
	item := KnowledgePointItem{
		ID: r.ID, Name: r.Name, LevelID: r.LevelID, LevelCode: r.LevelCode,
		SubjectID: r.SubjectID, SubjectName: r.SubjectName, ParentID: r.ParentID,
		QuestionCount: r.QuestionCount,
	}
	if r.StatsFound {
		item.Stats = &KPStats{
			ConfirmedAnswered: r.ConfirmedAnswered, ConfirmedCorrect: r.ConfirmedCorrect,
			RecentAnswered: r.RecentAnswered, RecentCorrect: r.RecentCorrect,
			AIAnswered: r.AIAnswered, AICorrect: r.AICorrect,
			ConsecutiveWrong: r.ConsecutiveWrong, LastPracticedAt: r.LastPracticedAt,
		}
	}
	return item
}

const kpStatsColumns = `kp.id::text, kp.name, kp.level_id::text, l.code, kp.subject_id::text, s.name, s.sort_order,
 kp.parent_id,
	 (SELECT count(*) FROM question_version_knowledge_points qvkp
	  JOIN question_versions v ON v.id = qvkp.question_version_id
	  JOIN questions q ON q.id = v.question_id
	  LEFT JOIN source_sections ss ON ss.id = v.source_section_id
	  LEFT JOIN sources src ON src.id = ss.source_id
	  WHERE qvkp.knowledge_point_id = kp.id AND q.published_version_id = v.id
	    AND coalesce(src.kind, '') <> 'ai_generated') AS question_count,
 coalesce(st.confirmed_answered, 0), coalesce(st.confirmed_correct, 0),
 coalesce(st.recent_answered, 0), coalesce(st.recent_correct, 0),
 coalesce(st.ai_answered, 0), coalesce(st.ai_correct, 0),
 coalesce(st.consecutive_wrong, 0), st.last_practiced_at::text,
 (st.user_id IS NOT NULL) AS stats_found`

const kpStatsJoins = `FROM knowledge_points kp
 JOIN exam_levels l ON l.id = kp.level_id
 JOIN subjects s ON s.id = kp.subject_id
 LEFT JOIN user_knowledge_stats st ON st.knowledge_point_id = kp.id AND st.user_id = $1`

func (s *Store) KnowledgePoints(ctx context.Context, userID, levelID, subjectID, search, cursor string, limit int) ([]KnowledgePointItem, string, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	args := []any{userID}
	where := "kp.status = 'published'"
	if levelID != "" {
		args = append(args, levelID)
		where += " AND kp.level_id::text = $" + strconv.Itoa(len(args))
	}
	if subjectID != "" {
		args = append(args, subjectID)
		where += " AND kp.subject_id::text = $" + strconv.Itoa(len(args))
	}
	if search != "" {
		args = append(args, "%"+search+"%")
		where += " AND kp.name ILIKE $" + strconv.Itoa(len(args))
	}
	if cursor != "" {
		parts := strings.Split(cursor, "\x00")
		if len(parts) != 3 {
			return nil, "", httpapi.ValidationError(map[string]string{"cursor": "游标无效"})
		}
		args = append(args, parts[0], parts[1], parts[2])
		n := len(args)
		where += " AND (s.sort_order, kp.name, kp.id) > ($" + strconv.Itoa(n-2) + ", $" + strconv.Itoa(n-1) + ", $" + strconv.Itoa(n) + "::uuid)"
	}
	args = append(args, limit)
	rows, err := store.CollectRows[kpStatRow](ctx, s.db,
		`SELECT `+kpStatsColumns+` `+kpStatsJoins+` WHERE `+where+` ORDER BY s.sort_order, kp.name, kp.id LIMIT $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, "", err
	}
	out := make([]KnowledgePointItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.toItem())
	}
	next := ""
	if len(rows) == limit {
		r := rows[len(rows)-1]
		next = strconv.Itoa(r.SubjectSortOrder) + "\x00" + r.Name + "\x00" + r.ID
	}
	return out, next, nil
}

func (s *Store) KnowledgePointDetailForUser(ctx context.Context, userID, id string) (KnowledgePointDetail, error) {
	var d KnowledgePointDetail
	var row kpStatRow
	var description, commonMistakes, examples, status string
	err := s.db.QueryRow(ctx,
		`SELECT `+kpStatsColumns+`,
		        kp.description, kp.common_mistakes, kp.examples, kp.status
		 FROM knowledge_points kp
		 JOIN exam_levels l ON l.id = kp.level_id
		 JOIN subjects s ON s.id = kp.subject_id
		 LEFT JOIN user_knowledge_stats st ON st.knowledge_point_id = kp.id AND st.user_id = $1
		 WHERE kp.id = $2 AND kp.status = 'published'`, userID, id,
	).Scan(&row.ID, &row.Name, &row.LevelID, &row.LevelCode, &row.SubjectID, &row.SubjectName, &row.SubjectSortOrder,
		&row.ParentID, &row.QuestionCount,
		&row.ConfirmedAnswered, &row.ConfirmedCorrect, &row.RecentAnswered, &row.RecentCorrect,
		&row.AIAnswered, &row.AICorrect, &row.ConsecutiveWrong, &row.LastPracticedAt, &row.StatsFound,
		&description, &commonMistakes, &examples, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, httpapi.ErrNotFound
	}
	if err != nil {
		return d, err
	}
	// 未发布正文对学习者不可见
	if status != "published" {
		description, commonMistakes, examples = "", "", ""
	}
	d.KnowledgePointItem = row.toItem()
	d.Description = description
	d.CommonMistakes = commonMistakes
	d.Examples = examples
	d.Status = status
	return d, nil
}

// MemoryForUser 返回账号级学习记忆。user_knowledge_stats 是可重算缓存，建议文本则由整批 AI 任务更新。
func (s *Store) MemoryForUser(ctx context.Context, userID string) (LearningMemory, error) {
	var memory LearningMemory
	var adviceStatus, adviceText string
	var adviceUpdatedAt, statsUpdatedAt *string
	err := s.db.QueryRow(ctx,
		`SELECT coalesce(ulm.ai_advice_status, 'not_requested'), coalesce(ulm.ai_advice, ''),
		        ulm.ai_advice_updated_at::text,
		        (SELECT max(updated_at)::text FROM user_knowledge_stats WHERE user_id = $1)
		 FROM users u LEFT JOIN user_learning_memory ulm ON ulm.user_id = u.id
		 WHERE u.id = $1`, userID,
	).Scan(&adviceStatus, &adviceText, &adviceUpdatedAt, &statsUpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return memory, httpapi.ErrNotFound
	}
	if err != nil {
		return memory, err
	}
	if err := s.db.QueryRow(ctx,
		`SELECT coalesce(sum(confirmed_answered), 0), coalesce(sum(confirmed_correct), 0),
		        coalesce(sum(ai_answered), 0), coalesce(sum(ai_correct), 0)
		 FROM user_knowledge_stats WHERE user_id = $1`, userID,
	).Scan(&memory.ConfirmedAnswered, &memory.ConfirmedCorrect, &memory.AIAnswered, &memory.AICorrect); err != nil {
		return memory, err
	}
	memory.StatsUpdatedAt = statsUpdatedAt
	memory.Advice = MemoryAdvice{Status: adviceStatus, Text: adviceText, UpdatedAt: adviceUpdatedAt}
	return memory, nil
}

// MemorySnapshotForAI 只返回可解释的统计事实，不把账号身份或原始答案发送给模型。
func (s *Store) MemorySnapshotForAI(ctx context.Context, userID string) (AIMemorySnapshot, error) {
	var snapshot AIMemorySnapshot
	if err := s.db.QueryRow(ctx,
		`SELECT coalesce(sum(confirmed_answered), 0), coalesce(sum(confirmed_correct), 0)
		 FROM user_knowledge_stats WHERE user_id = $1`, userID,
	).Scan(&snapshot.ConfirmedAnswered, &snapshot.ConfirmedCorrect); err != nil {
		return snapshot, err
	}
	rows, err := store.CollectRows[recommendationRow](ctx, s.db,
		`SELECT kp.id::text, kp.name, st.recent_answered, st.recent_correct,
		        st.consecutive_wrong, st.last_practiced_at::text
		 FROM user_knowledge_stats st
		 JOIN knowledge_points kp ON kp.id = st.knowledge_point_id
		 WHERE st.user_id = $1 AND st.recent_answered >= 5 AND kp.status = 'published'
		 ORDER BY (st.recent_correct::float / greatest(st.recent_answered, 1)) ASC,
		          st.consecutive_wrong DESC, st.last_practiced_at ASC NULLS LAST
		 LIMIT 5`, userID)
	if err != nil {
		return snapshot, err
	}
	snapshot.WeakPoints = make([]AIMemoryWeakPoint, 0, len(rows))
	for _, row := range rows {
		snapshot.WeakPoints = append(snapshot.WeakPoints, AIMemoryWeakPoint{
			Name: row.Name, RecentAnswered: row.RecentAnswered,
			RecentCorrect: row.RecentCorrect, ConsecutiveWrong: row.ConsecutiveWrong,
		})
	}
	return snapshot, nil
}

// GenerationMemoryForAI 返回当前级别的已审核知识点和账号统计，供生成题任务使用。
// memory 模式优先薄弱点，level 模式随机抽取更大的知识点样本，避免生成内容只围绕少数薄弱点。
func (s *Store) GenerationMemoryForAI(ctx context.Context, userID, levelID, subjectID string, knowledgePointIDs []string, generationMode string) (AIGenerationMemory, error) {
	var memory AIGenerationMemory
	if err := s.db.QueryRow(ctx,
		`SELECT coalesce(sum(confirmed_answered), 0), coalesce(sum(confirmed_correct), 0)
		 FROM user_knowledge_stats WHERE user_id = $1`, userID,
	).Scan(&memory.ConfirmedAnswered, &memory.ConfirmedCorrect); err != nil {
		return memory, err
	}
	if knowledgePointIDs == nil {
		knowledgePointIDs = []string{}
	}
	rows, err := store.CollectRows[AIGenerationKnowledgePoint](ctx, s.db,
		`SELECT kp.id::text, kp.name, kp.subject_id::text, kp.description, kp.common_mistakes, kp.examples,
		        coalesce(st.recent_answered, 0), coalesce(st.recent_correct, 0),
		        coalesce(st.consecutive_wrong, 0)
		 FROM knowledge_points kp
		 LEFT JOIN user_knowledge_stats st
		   ON st.knowledge_point_id = kp.id AND st.user_id = $1
		 WHERE kp.status = 'published'
		   AND kp.level_id::text = $2
		   AND ($3 = '' OR kp.subject_id::text = $3)
		   AND ($4::uuid[] = '{}' OR kp.id = ANY($4::uuid[]))
		 ORDER BY CASE WHEN $5 = 'level' THEN random() END,
		          CASE WHEN $5 = 'memory' AND coalesce(st.recent_answered, 0) >= 5 THEN 0 ELSE 1 END,
		          CASE WHEN $5 = 'memory' THEN coalesce(st.recent_correct, 0)::float / greatest(coalesce(st.recent_answered, 0), 1) END,
		          CASE WHEN $5 = 'memory' THEN coalesce(st.consecutive_wrong, 0)::float END DESC,
		          random()
		 LIMIT CASE WHEN $4::uuid[] = '{}' AND $5 = 'memory' THEN 5 ELSE 50 END`, userID, levelID, subjectID, knowledgePointIDs, generationMode)
	if err != nil {
		return memory, err
	}
	memory.KnowledgePoints = rows
	return memory, nil
}

// DeleteMemory 清除派生学习记忆并设置新的统计起点；练习历史和成绩仍保留。
func (s *Store) DeleteMemory(ctx context.Context, pool *pgxpool.Pool, userID string) error {
	return store.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO user_learning_memory
			   (user_id, reset_at, ai_advice, ai_advice_status, ai_advice_updated_at, updated_at)
			 VALUES ($1, now(), '', 'not_requested', NULL, now())
			 ON CONFLICT (user_id) DO UPDATE
			 SET reset_at = now(), ai_advice = '', ai_advice_status = 'not_requested',
			     ai_advice_updated_at = NULL, updated_at = now()`, userID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM user_knowledge_stats WHERE user_id = $1`, userID)
		return err
	})
}

// WriteAIAdviceTx 只在重置边界没有变化时写入，避免删除记忆后旧任务复活旧建议。
func (s *Store) WriteAIAdviceTx(ctx context.Context, tx pgx.Tx, userID string, resetAt any, status, text string) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO user_learning_memory
		   (user_id, reset_at, ai_advice, ai_advice_status, ai_advice_updated_at, updated_at)
		 VALUES ($1, $2, $3, $4, now(), now())
		 ON CONFLICT (user_id) DO UPDATE
		 SET ai_advice = EXCLUDED.ai_advice, ai_advice_status = EXCLUDED.ai_advice_status,
		     ai_advice_updated_at = now(), updated_at = now()
		 WHERE user_learning_memory.reset_at IS NOT DISTINCT FROM EXCLUDED.reset_at`,
		userID, resetAt, text, status)
	return err
}

// ---------- 仪表盘 ----------

func (s *Store) ActiveSession(ctx context.Context, userID string) (*ActiveSession, error) {
	var a ActiveSession
	err := s.db.QueryRow(ctx,
		`SELECT ps.id::text, ps.status,
		        (SELECT count(*) FROM user_answers ua JOIN practice_items pi ON pi.id = ua.item_id
		         WHERE pi.session_id = ps.id AND ua.value IS NOT NULL) AS answered,
		        (SELECT count(*) FROM practice_items pi WHERE pi.session_id = ps.id) AS total
		 FROM practice_sessions ps
			 WHERE ps.user_id = $1 AND ps.deleted_at IS NULL AND ps.status IN ('active', 'generating')
		 ORDER BY ps.created_at DESC LIMIT 1`, userID,
	).Scan(&a.ID, &a.Status, &a.AnsweredCount, &a.TotalCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

type recentSessionRow struct {
	ID          string
	Status      string
	TotalCount  int
	CreatedAt   string
	SubmittedAt *string
}

func (s *Store) RecentSessions(ctx context.Context, userID string, limit int) ([]RecentSession, error) {
	rows, err := store.CollectRows[recentSessionRow](ctx, s.db,
		`SELECT ps.id::text, ps.status,
		        (SELECT count(*) FROM practice_items pi WHERE pi.session_id = ps.id),
		        ps.created_at::text, ps.submitted_at::text
			 FROM practice_sessions ps
			 WHERE ps.user_id = $1 AND ps.deleted_at IS NULL AND ps.status <> 'active'
		 ORDER BY ps.created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]RecentSession, 0, len(rows))
	for _, r := range rows {
		out = append(out, RecentSession{ID: r.ID, Status: r.Status, TotalCount: r.TotalCount,
			CreatedAt: r.CreatedAt, SubmittedAt: r.SubmittedAt})
	}
	return out, nil
}

type recommendationRow struct {
	ID               string
	Name             string
	RecentAnswered   int
	RecentCorrect    int
	ConsecutiveWrong int
	LastPracticedAt  *string
}

// WeakKnowledgePoints 按规范 §14.2 的稳定规则排序薄弱知识点：
// 近 30 天已确认作答 ≥ 5 题 → 近期正确率升序 → 连续错误降序 → 最近练习时间升序。
func (s *Store) WeakKnowledgePoints(ctx context.Context, userID string, limit int) ([]recommendationRow, error) {
	return store.CollectRows[recommendationRow](ctx, s.db,
		`SELECT kp.id::text, kp.name, st.recent_answered, st.recent_correct,
		        st.consecutive_wrong, st.last_practiced_at::text
		 FROM user_knowledge_stats st
			JOIN knowledge_points kp ON kp.id = st.knowledge_point_id
			WHERE st.user_id = $1 AND st.recent_answered >= 5 AND kp.status = 'published'
		 ORDER BY (st.recent_correct::float / greatest(st.recent_answered, 1)) ASC,
		          st.consecutive_wrong DESC, st.last_practiced_at ASC NULLS LAST
		 LIMIT $2`, userID, limit)
}

// ---------- 统计重算（缓存可重建；原始作答是唯一事实来源） ----------

// RebuildUserStats 从 grading_results 全量重算用户知识点统计并整体替换缓存。
// 正式统计只聚合 official / human_verified 的确定性判分；AI 判定单独计数。
func (s *Store) RebuildUserStats(ctx context.Context, pool *pgxpool.Pool, userID string) error {
	return store.WithTx(ctx, pool, func(tx pgx.Tx) error {
		return s.RebuildUserStatsTx(ctx, tx, userID)
	})
}

func (s *Store) RebuildUserStatsTx(ctx context.Context, tx pgx.Tx, userID string) error {
	if _, err := tx.Exec(ctx,
		`INSERT INTO user_learning_memory (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, userID); err != nil {
		return err
	}
	// 与删除记忆共用同一行锁，保证重算不会在删除提交后把旧统计写回来。
	if err := tx.QueryRow(ctx,
		`SELECT user_id::text FROM user_learning_memory WHERE user_id = $1 FOR UPDATE`, userID,
	).Scan(new(string)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM user_knowledge_stats WHERE user_id = $1`, userID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
WITH joined AS (
  SELECT qvkp.knowledge_point_id AS kp_id,
         gr.status, gr.source, gr.answer_authority,
         COALESCE(ps.submitted_at, ps.created_at) AS at_time
  FROM grading_results gr
  JOIN practice_items pi ON pi.id = gr.item_id
  JOIN practice_sessions ps ON ps.id = pi.session_id
  JOIN question_version_knowledge_points qvkp ON qvkp.question_version_id = pi.question_version_id
  LEFT JOIN user_learning_memory mem ON mem.user_id = $1
  WHERE ps.user_id = $1 AND ps.status IN ('grading', 'completed', 'analysis_failed')
    AND (mem.reset_at IS NULL OR COALESCE(ps.submitted_at, ps.created_at) > mem.reset_at)
),
confirmed AS (
  SELECT * FROM joined
  WHERE source = 'deterministic' AND answer_authority IS NOT NULL
    AND status IN ('correct', 'incorrect', 'unanswered')
),
ordered AS (
  SELECT kp_id, status, at_time,
         ROW_NUMBER() OVER (PARTITION BY kp_id ORDER BY at_time DESC) AS rn
  FROM confirmed
),
streak AS (
  SELECT kp_id,
         CASE WHEN MAX(CASE WHEN status = 'correct' THEN rn END) IS NULL
              THEN COUNT(*)
              ELSE COUNT(*) - MAX(CASE WHEN status = 'correct' THEN rn END)
         END AS consecutive_wrong
  FROM ordered GROUP BY kp_id
),
agg AS (
  SELECT kp_id,
         COUNT(*) AS confirmed_answered,
         COUNT(*) FILTER (WHERE status = 'correct') AS confirmed_correct,
         COUNT(*) FILTER (WHERE at_time >= now() - interval '30 days') AS recent_answered,
         COUNT(*) FILTER (WHERE status = 'correct' AND at_time >= now() - interval '30 days') AS recent_correct,
         MAX(at_time) AS last_practiced_at
  FROM confirmed GROUP BY kp_id
),
ai_agg AS (
  SELECT kp_id,
         COUNT(*) FILTER (WHERE status IN ('correct', 'incorrect')) AS ai_answered,
         COUNT(*) FILTER (WHERE status = 'correct') AS ai_correct
  FROM joined WHERE source = 'ai' GROUP BY kp_id
),
all_kp AS (
  SELECT kp_id FROM agg
  UNION
  SELECT kp_id FROM ai_agg
)
INSERT INTO user_knowledge_stats
  (user_id, knowledge_point_id, confirmed_answered, confirmed_correct,
   recent_answered, recent_correct, ai_answered, ai_correct, consecutive_wrong, last_practiced_at)
SELECT $1, all_kp.kp_id, coalesce(agg.confirmed_answered, 0), coalesce(agg.confirmed_correct, 0),
       coalesce(agg.recent_answered, 0), coalesce(agg.recent_correct, 0),
       coalesce(ai.ai_answered, 0), coalesce(ai.ai_correct, 0),
       coalesce(stk.consecutive_wrong, 0), agg.last_practiced_at
FROM all_kp
LEFT JOIN agg ON agg.kp_id = all_kp.kp_id
LEFT JOIN streak stk ON stk.kp_id = all_kp.kp_id
LEFT JOIN ai_agg ai ON ai.kp_id = all_kp.kp_id`, userID)
	return err
}

// ---------- 错题本 ----------

type wrongItemRow struct {
	ItemID            string
	SessionID         string
	QuestionID        string
	Position          int
	Type              string
	Stem              string
	OptionsText       *string
	MaterialID        *string
	MaterialTitle     *string
	MaterialContent   *string
	KPID              *string
	KPName            *string
	Status            string
	Authority         *string
	CorrectValue      *string
	UserValue         *string
	Explanation       *string
	ExplanationSource *string
	GradedAt          string
}

// WrongItems 返回每个错题（最近一次错误作答）及其解析，支持按知识点筛选。
func (s *Store) WrongItems(ctx context.Context, userID, knowledgePointID, cursor string, limit int) ([]wrongItemRow, string, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	args := []any{userID}
	where := ""
	if knowledgePointID != "" {
		args = append(args, knowledgePointID)
		where = ` AND EXISTS (SELECT 1 FROM question_version_knowledge_points qvkp2
		     WHERE qvkp2.question_version_id = pi.question_version_id
		       AND qvkp2.knowledge_point_id::text = $2)`
	}
	outerWhere := ""
	if cursor != "" {
		parts := strings.Split(cursor, "\x00")
		if len(parts) != 2 {
			return nil, "", httpapi.ValidationError(map[string]string{"cursor": "游标无效"})
		}
		args = append(args, parts[0], parts[1])
		n := len(args)
		outerWhere = " WHERE (w.graded_at::timestamptz, w.item_id::uuid) < ($" + strconv.Itoa(n-1) + "::timestamptz, $" + strconv.Itoa(n) + "::uuid)"
	}
	args = append(args, limit)
	limitPh := "$" + strconv.Itoa(len(args))
	rows, err := store.CollectRows[wrongItemRow](ctx, s.db,
		`SELECT w.* FROM (
		   SELECT DISTINCT ON (pi.question_id)
		          pi.id::text AS item_id, ps.id::text AS session_id, pi.question_id::text,
		          pi.position, v.type, v.stem, v.options::text,
		          mv.material_id::text, mv.title, mv.content,
		          kp.id::text, kp.name,
		          gr.status, gr.answer_authority, gr.correct_value::text, gr.user_value::text,
		          gr.explanation, gr.explanation_source, gr.updated_at::text AS graded_at
		   FROM grading_results gr
		   JOIN practice_items pi ON pi.id = gr.item_id
		   JOIN practice_sessions ps ON ps.id = pi.session_id
		   LEFT JOIN user_learning_memory mem ON mem.user_id = ps.user_id
		   JOIN question_versions v ON v.id = pi.question_version_id
		   LEFT JOIN material_versions mv ON mv.id = v.material_version_id
		   LEFT JOIN question_version_knowledge_points qvkp ON qvkp.question_version_id = v.id
		   LEFT JOIN knowledge_points kp ON kp.id = qvkp.knowledge_point_id
			   WHERE ps.user_id = $1 AND ps.deleted_at IS NULL AND pi.deleted_at IS NULL
		     AND (mem.reset_at IS NULL OR COALESCE(ps.submitted_at, ps.created_at) > mem.reset_at)
		     AND (
		     (gr.source = 'deterministic' AND gr.answer_authority IS NOT NULL AND gr.status IN ('incorrect', 'unanswered'))
		     OR (gr.source = 'ai' AND gr.status = 'incorrect'))`+where+`
		   ORDER BY pi.question_id, gr.updated_at DESC
		 ) w`+outerWhere+` ORDER BY w.graded_at DESC, w.item_id DESC LIMIT `+limitPh, args...)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(rows) == limit {
		r := rows[len(rows)-1]
		next = r.GradedAt + "\x00" + r.ItemID
	}
	return rows, next, nil
}

func (s *Store) DeleteWrongItem(ctx context.Context, userID, itemID string) error {
	commandTag, err := s.db.Exec(ctx,
		`UPDATE practice_items pi SET deleted_at = now()
		 FROM practice_sessions ps
		 WHERE pi.id = $1 AND pi.session_id = ps.id AND ps.user_id = $2
		   AND ps.deleted_at IS NULL AND pi.deleted_at IS NULL`, itemID, userID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return httpapi.ErrNotFound
	}
	return nil
}

// ---------- 举报 ----------

type IssueReport struct {
	ID                string          `json:"id"`
	UserID            string          `json:"userId"`
	QuestionID        string          `json:"questionId"`
	QuestionVersionID string          `json:"questionVersionId"`
	PracticeItemID    *string         `json:"practiceItemId,omitempty"`
	SessionID         *string         `json:"sessionId,omitempty"`
	TargetType        string          `json:"targetType"`
	Description       string          `json:"description"`
	Context           json.RawMessage `json:"context"`
	Status            string          `json:"status"`
	ResolutionNote    string          `json:"resolutionNote"`
	HandledBy         *string         `json:"handledBy,omitempty"`
	HandledAt         *string         `json:"handledAt,omitempty"`
	CreatedAt         string          `json:"createdAt"`
	UserEmail         string          `json:"userEmail,omitempty"`
	Stem              string          `json:"stem,omitempty"`
}

// CreateIssueReport 由学习端提交；上下文（题目版本、用户答案、判分状态）由后端补全。
func (s *Store) CreateIssueReport(ctx context.Context, pool *pgxpool.Pool, userID, practiceItemID, targetType, description string) (IssueReport, error) {
	var report IssueReport
	err := store.WithTx(ctx, pool, func(tx pgx.Tx) error {
		// 校验 item 属于当前用户
		var questionID, versionID, sessionID, contextJSON string
		err := tx.QueryRow(ctx,
			`SELECT pi.question_id::text, pi.question_version_id::text, ps.id::text,
			        coalesce(to_jsonb(jsonb_build_object(
			          'userAnswer', ua.value,
			          'gradingStatus', gr.status,
			          'gradingSource', gr.source,
			          'answerAuthority', gr.answer_authority
			        ))::text, '{}')
			 FROM practice_items pi
			 JOIN practice_sessions ps ON ps.id = pi.session_id
			 LEFT JOIN user_answers ua ON ua.item_id = pi.id
			 LEFT JOIN grading_results gr ON gr.item_id = pi.id
			 WHERE pi.id = $1 AND ps.user_id = $2`,
			practiceItemID, userID,
		).Scan(&questionID, &versionID, &sessionID, &contextJSON)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpapi.ErrNotFound
		}
		if err != nil {
			return err
		}
		return tx.QueryRow(ctx,
			`INSERT INTO issue_reports
			   (user_id, question_id, question_version_id, practice_item_id, session_id, target_type, description, context)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 RETURNING id::text, user_id::text, question_id::text, question_version_id::text,
			           practice_item_id::text, session_id::text, target_type, description, context::text,
			           status, resolution_note, handled_by::text, handled_at::text, created_at::text`,
			userID, questionID, versionID, practiceItemID, sessionID, targetType, description, contextJSON,
		).Scan(&report.ID, &report.UserID, &report.QuestionID, &report.QuestionVersionID,
			&report.PracticeItemID, &report.SessionID, &report.TargetType, &report.Description, &report.Context,
			&report.Status, &report.ResolutionNote, &report.HandledBy, &report.HandledAt, &report.CreatedAt)
	})
	return report, err
}

type issueListRow struct {
	ID             string
	TargetType     string
	Description    string
	Status         string
	ResolutionNote string
	HandledBy      *string
	HandledAt      *string
	CreatedAt      string
	UserEmail      string
	Stem           string
	QuestionID     string
}

func (s *Store) ListIssueReports(ctx context.Context, status, cursor string, limit int) ([]IssueReport, string, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	args := []any{}
	where := "true"
	if status != "" {
		args = append(args, status)
		where = "ir.status = $1"
	}
	if cursor != "" {
		parts := strings.Split(cursor, "\x00")
		if len(parts) != 2 {
			return nil, "", httpapi.ValidationError(map[string]string{"cursor": "游标无效"})
		}
		args = append(args, parts[0], parts[1])
		n := len(args)
		where += " AND (ir.created_at, ir.id) < ($" + strconv.Itoa(n-1) + "::timestamptz, $" + strconv.Itoa(n) + "::uuid)"
	}
	args = append(args, limit)
	rows, err := store.CollectRows[issueListRow](ctx, s.db,
		`SELECT ir.id::text, ir.target_type, ir.description, ir.status, ir.resolution_note,
		        ir.handled_by::text, ir.handled_at::text, ir.created_at::text,
		        u.email, v.stem, ir.question_id::text
		 FROM issue_reports ir
		 JOIN users u ON u.id = ir.user_id
		 JOIN question_versions v ON v.id = ir.question_version_id
		 WHERE `+where+`
		 ORDER BY ir.created_at DESC, ir.id DESC LIMIT `+"$"+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, "", err
	}
	out := make([]IssueReport, 0, len(rows))
	for _, r := range rows {
		out = append(out, IssueReport{
			ID: r.ID, TargetType: r.TargetType, Description: r.Description, Status: r.Status,
			ResolutionNote: r.ResolutionNote, HandledBy: r.HandledBy, HandledAt: r.HandledAt,
			CreatedAt: r.CreatedAt, UserEmail: r.UserEmail, Stem: r.Stem, QuestionID: r.QuestionID,
		})
	}
	next := ""
	if len(rows) == limit {
		r := rows[len(rows)-1]
		next = r.CreatedAt + "\x00" + r.ID
	}
	return out, next, nil
}

func (s *Store) ResolveIssueReport(ctx context.Context, pool *pgxpool.Pool, adminID, id, status, note string) error {
	if status != "resolved" && status != "dismissed" {
		return httpapi.ValidationError(map[string]string{"status": "处理结果只能是 resolved 或 dismissed"})
	}
	res, err := pool.Exec(ctx,
		`UPDATE issue_reports
		 SET status = $3, resolution_note = $4, handled_by = $5, handled_at = now(), updated_at = now()
		 WHERE id = $1 AND status = 'open'`, id, status, note, adminID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return httpapi.E(409, "issue_already_handled", "该举报已被处理")
	}
	return nil
}
