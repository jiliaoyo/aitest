package admin

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

type Store struct{ db store.DBTx }

func NewStore(db store.DBTx) *Store { return &Store{db: db} }

type usageRow struct {
	PracticeSessions         int
	CompletedSessions        int
	AnalysisFailedSessions   int
	GenerationFailedSessions int
	ActiveSessions           int
	SubmittedSessions        int
	PracticeItems            int
	AnsweredItems            int
	AIGenerationRequests     int
	AIGeneratedQuestions     int
	AICalls                  int
	AISuccessfulCalls        int
	AIFailedCalls            int
	AIGenerationCalls        int
	AIPromptTokens           int64
	AICompletionTokens       int64
	AIDurationMs             int64
	AICostedCalls            int
	AIEstimatedCostUSD       float64
	LoginCount               int
	ActiveAuthSessions       int
	LastLoginAt              *string
	ActiveDays               int
	LastActiveAt             *string
}

func (r usageRow) usage() UserUsage {
	return UserUsage{
		ActiveDays: r.ActiveDays, LastActiveAt: r.LastActiveAt,
		PracticeSessions: r.PracticeSessions, CompletedSessions: r.CompletedSessions,
		AnalysisFailedSessions: r.AnalysisFailedSessions, GenerationFailedSessions: r.GenerationFailedSessions,
		ActiveSessions: r.ActiveSessions, SubmittedSessions: r.SubmittedSessions,
		PracticeItems: r.PracticeItems, AnsweredItems: r.AnsweredItems,
		AIGenerationRequests: r.AIGenerationRequests, AIGeneratedQuestions: r.AIGeneratedQuestions,
		AI: aiUsage(r.AICalls, r.AISuccessfulCalls, r.AIFailedCalls, r.AIGenerationCalls,
			r.AIPromptTokens, r.AICompletionTokens, r.AIDurationMs, r.AICostedCalls, r.AIEstimatedCostUSD),
		LoginCount: r.LoginCount, ActiveAuthSessions: r.ActiveAuthSessions, LastLoginAt: r.LastLoginAt,
	}
}

func dateClause(column string) string {
	return "($1::date IS NULL OR " + column + " >= $1::date) AND ($2::date IS NULL OR " + column + " < ($2::date + INTERVAL '1 day'))"
}

func userClause(column string) string { return "($3 = '' OR " + column + "::text = $3)" }

func (s *Store) Usage(ctx context.Context, dateRange DateRange, userID string) (UserUsage, error) {
	q := `
WITH practice AS (
  SELECT
    (count(DISTINCT ps.id))::int AS practice_sessions,
    (count(DISTINCT ps.id) FILTER (WHERE ps.status = 'completed'))::int AS completed_sessions,
    (count(DISTINCT ps.id) FILTER (WHERE ps.status = 'analysis_failed'))::int AS analysis_failed_sessions,
    (count(DISTINCT ps.id) FILTER (WHERE ps.status = 'generation_failed'))::int AS generation_failed_sessions,
    (count(DISTINCT ps.id) FILTER (WHERE ps.status IN ('active', 'grading', 'generating')))::int AS active_sessions,
    (count(DISTINCT ps.id) FILTER (WHERE ps.submitted_at IS NOT NULL))::int AS submitted_sessions,
    (count(pi.id))::int AS practice_items,
    (count(ua.item_id) FILTER (WHERE ua.value IS NOT NULL))::int AS answered_items,
    (count(DISTINCT ps.id) FILTER (WHERE ps.scope->>'mode' = 'ai_generated'))::int AS ai_generation_requests,
    (count(pi.id) FILTER (WHERE ps.scope->>'mode' = 'ai_generated'))::int AS ai_generated_questions
  FROM practice_sessions ps
  LEFT JOIN practice_items pi ON pi.session_id = ps.id
  LEFT JOIN user_answers ua ON ua.item_id = pi.id
  WHERE ` + dateClause("ps.created_at") + ` AND ` + userClause("ps.user_id") + `
),
ai_stats AS (
  SELECT
    count(*)::int AS ai_calls,
    (count(*) FILTER (WHERE ar.error = ''))::int AS ai_successful_calls,
    (count(*) FILTER (WHERE ar.error <> ''))::int AS ai_failed_calls,
    (count(*) FILTER (WHERE ar.kind = 'practice_question_generation'))::int AS ai_generation_calls,
    coalesce(sum(ar.prompt_tokens), 0)::bigint AS ai_prompt_tokens,
    coalesce(sum(ar.completion_tokens), 0)::bigint AS ai_completion_tokens,
    coalesce(sum(ar.duration_ms), 0)::bigint AS ai_duration_ms,
    (count(*) FILTER (WHERE ar.estimated_cost_usd IS NOT NULL))::int AS ai_costed_calls,
    coalesce(sum(ar.estimated_cost_usd), 0)::float8 AS ai_estimated_cost_usd
  FROM ai_runs ar
  WHERE ` + dateClause("ar.created_at") + ` AND ar.user_id IS NOT NULL AND ` + userClause("ar.user_id") + `
),
session_stats AS (
  SELECT
    (count(*) FILTER (WHERE ` + dateClause("s.created_at") + `))::int AS login_count,
    (count(*) FILTER (WHERE s.revoked_at IS NULL AND s.expires_at > now()))::int AS active_auth_sessions,
    max(s.created_at) FILTER (WHERE ` + dateClause("s.created_at") + `)::text AS last_login_at
  FROM auth_sessions s
  WHERE ` + userClause("s.user_id") + `
),
active_events AS (
  SELECT ps.user_id, ps.created_at AS event_at
  FROM practice_sessions ps
  WHERE ` + dateClause("ps.created_at") + ` AND ` + userClause("ps.user_id") + `
  UNION ALL
  SELECT ar.user_id, ar.created_at
  FROM ai_runs ar
  WHERE ` + dateClause("ar.created_at") + ` AND ar.user_id IS NOT NULL AND ` + userClause("ar.user_id") + `
  UNION ALL
  SELECT ua.user_id, ua.updated_at
  FROM user_answers ua
  WHERE ` + dateClause("ua.updated_at") + ` AND ` + userClause("ua.user_id") + `
  UNION ALL
  SELECT s.user_id, s.created_at
  FROM auth_sessions s
  WHERE ` + dateClause("s.created_at") + ` AND ` + userClause("s.user_id") + `
), active_days AS (
  SELECT count(*)::int AS active_days, max(day_at)::text AS last_active_at
  FROM (
    SELECT event_at::date AS day, max(event_at) AS day_at
    FROM active_events
    GROUP BY event_at::date
  ) days
)
SELECT p.practice_sessions, p.completed_sessions, p.analysis_failed_sessions, p.generation_failed_sessions,
       p.active_sessions, p.submitted_sessions, p.practice_items, p.answered_items,
       p.ai_generation_requests, p.ai_generated_questions,
       a.ai_calls, a.ai_successful_calls, a.ai_failed_calls, a.ai_generation_calls,
       a.ai_prompt_tokens, a.ai_completion_tokens, a.ai_duration_ms, a.ai_costed_calls, a.ai_estimated_cost_usd,
       s.login_count, s.active_auth_sessions, s.last_login_at,
       d.active_days, d.last_active_at
FROM practice p CROSS JOIN ai_stats a CROSS JOIN session_stats s CROSS JOIN active_days d`
	args := append(dateRange.args(), userID)
	row, err := store.CollectOneRow[usageRow](ctx, s.db, q, args...)
	if err != nil {
		return UserUsage{}, err
	}
	return row.usage(), nil
}

type userListRow struct {
	ID                       string
	Email                    string
	Role                     string
	DefaultLevelID           *string
	DefaultLevelCode         string
	DefaultLevelName         string
	CreatedAt                string
	LastActiveAt             *string
	LastLoginAt              *string
	PracticeSessions         int
	CompletedSessions        int
	AnalysisFailedSessions   int
	GenerationFailedSessions int
	ActiveSessions           int
	SubmittedSessions        int
	PracticeItems            int
	AnsweredItems            int
	AIGenerationRequests     int
	AIGeneratedQuestions     int
	AICalls                  int
	AISuccessfulCalls        int
	AIFailedCalls            int
	AIGenerationCalls        int
	AIPromptTokens           int64
	AICompletionTokens       int64
	AIDurationMs             int64
	AICostedCalls            int
	AIEstimatedCostUSD       float64
	LoginCount               int
	ActiveAuthSessions       int
	ActiveDays               int
}

func (r userListRow) item() UserListItem {
	return UserListItem{
		ID: r.ID, Email: r.Email, Role: r.Role, DefaultLevelID: r.DefaultLevelID,
		DefaultLevelCode: r.DefaultLevelCode, DefaultLevelName: r.DefaultLevelName, CreatedAt: r.CreatedAt,
		LastActiveAt: r.LastActiveAt, LastLoginAt: r.LastLoginAt,
		Usage: usageRow{
			PracticeSessions: r.PracticeSessions, CompletedSessions: r.CompletedSessions,
			AnalysisFailedSessions: r.AnalysisFailedSessions, GenerationFailedSessions: r.GenerationFailedSessions,
			ActiveSessions: r.ActiveSessions, SubmittedSessions: r.SubmittedSessions,
			PracticeItems: r.PracticeItems, AnsweredItems: r.AnsweredItems,
			AIGenerationRequests: r.AIGenerationRequests, AIGeneratedQuestions: r.AIGeneratedQuestions,
			AICalls: r.AICalls, AISuccessfulCalls: r.AISuccessfulCalls, AIFailedCalls: r.AIFailedCalls,
			AIGenerationCalls: r.AIGenerationCalls, AIPromptTokens: r.AIPromptTokens,
			AICompletionTokens: r.AICompletionTokens, AIDurationMs: r.AIDurationMs,
			AICostedCalls: r.AICostedCalls, AIEstimatedCostUSD: r.AIEstimatedCostUSD,
			LoginCount: r.LoginCount, ActiveAuthSessions: r.ActiveAuthSessions,
			ActiveDays: r.ActiveDays, LastActiveAt: r.LastActiveAt, LastLoginAt: r.LastLoginAt,
		}.usage(),
	}
}

// ponytail: 列表直接聚合原始事实表；数据量明显增长时再加按日用量汇总表。
func (s *Store) ListUsers(ctx context.Context, f UserListFilter) ([]UserListItem, string, error) {
	if f.Limit < 1 || f.Limit > 100 {
		f.Limit = 20
	}
	f.Query = normalizedQuery(f.Query)
	if len([]rune(f.Query)) > 120 {
		return nil, "", httpapi.ValidationError(map[string]string{"q": "搜索条件过长"})
	}
	if f.Role != "" && !validRole(f.Role) {
		return nil, "", httpapi.ValidationError(map[string]string{"role": "用户角色不合法"})
	}

	q := `
WITH practice AS (
  SELECT ps.user_id,
    (count(DISTINCT ps.id))::int AS practice_sessions,
    (count(DISTINCT ps.id) FILTER (WHERE ps.status = 'completed'))::int AS completed_sessions,
    (count(DISTINCT ps.id) FILTER (WHERE ps.status = 'analysis_failed'))::int AS analysis_failed_sessions,
    (count(DISTINCT ps.id) FILTER (WHERE ps.status = 'generation_failed'))::int AS generation_failed_sessions,
    (count(DISTINCT ps.id) FILTER (WHERE ps.status IN ('active', 'grading', 'generating')))::int AS active_sessions,
    (count(DISTINCT ps.id) FILTER (WHERE ps.submitted_at IS NOT NULL))::int AS submitted_sessions,
    (count(pi.id))::int AS practice_items,
    (count(ua.item_id) FILTER (WHERE ua.value IS NOT NULL))::int AS answered_items,
    (count(DISTINCT ps.id) FILTER (WHERE ps.scope->>'mode' = 'ai_generated'))::int AS ai_generation_requests,
    (count(pi.id) FILTER (WHERE ps.scope->>'mode' = 'ai_generated'))::int AS ai_generated_questions
  FROM practice_sessions ps
  LEFT JOIN practice_items pi ON pi.session_id = ps.id
  LEFT JOIN user_answers ua ON ua.item_id = pi.id
  WHERE ` + dateClause("ps.created_at") + `
  GROUP BY ps.user_id
),
ai_stats AS (
  SELECT ar.user_id,
    count(*)::int AS ai_calls,
    (count(*) FILTER (WHERE ar.error = ''))::int AS ai_successful_calls,
    (count(*) FILTER (WHERE ar.error <> ''))::int AS ai_failed_calls,
    (count(*) FILTER (WHERE ar.kind = 'practice_question_generation'))::int AS ai_generation_calls,
    coalesce(sum(ar.prompt_tokens), 0)::bigint AS ai_prompt_tokens,
    coalesce(sum(ar.completion_tokens), 0)::bigint AS ai_completion_tokens,
    coalesce(sum(ar.duration_ms), 0)::bigint AS ai_duration_ms,
    (count(*) FILTER (WHERE ar.estimated_cost_usd IS NOT NULL))::int AS ai_costed_calls,
    coalesce(sum(ar.estimated_cost_usd), 0)::float8 AS ai_estimated_cost_usd
  FROM ai_runs ar
  WHERE ` + dateClause("ar.created_at") + ` AND ar.user_id IS NOT NULL
  GROUP BY ar.user_id
),
session_stats AS (
  SELECT s.user_id,
    (count(*) FILTER (WHERE ` + dateClause("s.created_at") + `))::int AS login_count,
    (count(*) FILTER (WHERE s.revoked_at IS NULL AND s.expires_at > now()))::int AS active_auth_sessions,
    max(s.created_at) FILTER (WHERE ` + dateClause("s.created_at") + `)::text AS last_login_at
  FROM auth_sessions s
  GROUP BY s.user_id
),
active_events AS (
  SELECT ps.user_id, ps.created_at AS event_at
  FROM practice_sessions ps
  WHERE ` + dateClause("ps.created_at") + `
  UNION ALL
  SELECT ar.user_id, ar.created_at
  FROM ai_runs ar
  WHERE ` + dateClause("ar.created_at") + ` AND ar.user_id IS NOT NULL
  UNION ALL
  SELECT ua.user_id, ua.updated_at
  FROM user_answers ua
  WHERE ` + dateClause("ua.updated_at") + `
  UNION ALL
  SELECT s.user_id, s.created_at
  FROM auth_sessions s
  WHERE ` + dateClause("s.created_at") + `
), active_days AS (
  SELECT user_id, count(*)::int AS active_days, max(day_at)::text AS last_active_at
  FROM (
    SELECT user_id, event_at::date AS day, max(event_at) AS day_at
    FROM active_events
    GROUP BY user_id, event_at::date
  ) days
  GROUP BY user_id
)
SELECT u.id::text, u.email, u.role, u.default_level_id::text,
       coalesce(l.code, ''), coalesce(l.name, ''), u.created_at::text,
       d.last_active_at,
       s.last_login_at,
       coalesce(p.practice_sessions, 0), coalesce(p.completed_sessions, 0),
       coalesce(p.analysis_failed_sessions, 0), coalesce(p.generation_failed_sessions, 0),
       coalesce(p.active_sessions, 0), coalesce(p.submitted_sessions, 0),
       coalesce(p.practice_items, 0), coalesce(p.answered_items, 0),
       coalesce(p.ai_generation_requests, 0), coalesce(p.ai_generated_questions, 0),
       coalesce(a.ai_calls, 0), coalesce(a.ai_successful_calls, 0), coalesce(a.ai_failed_calls, 0),
       coalesce(a.ai_generation_calls, 0), coalesce(a.ai_prompt_tokens, 0), coalesce(a.ai_completion_tokens, 0),
       coalesce(a.ai_duration_ms, 0), coalesce(a.ai_costed_calls, 0), coalesce(a.ai_estimated_cost_usd, 0),
       coalesce(s.login_count, 0), coalesce(s.active_auth_sessions, 0),
       coalesce(d.active_days, 0)
FROM users u
LEFT JOIN exam_levels l ON l.id = u.default_level_id
LEFT JOIN practice p ON p.user_id = u.id
LEFT JOIN ai_stats a ON a.user_id = u.id
LEFT JOIN session_stats s ON s.user_id = u.id
LEFT JOIN active_days d ON d.user_id = u.id
WHERE true`
	args := append(f.DateRange.args(), []any{}...)
	conds := []string{}
	if f.Role != "" {
		args = append(args, f.Role)
		conds = append(conds, "u.role = $"+store.Itoa(len(args)))
	}
	if f.Query != "" {
		args = append(args, f.Query)
		conds = append(conds, "u.email ILIKE '%' || $"+store.Itoa(len(args))+" || '%'")
	}
	if f.Cursor != "" {
		createdAt, id, err := parseCursor(f.Cursor)
		if err != nil {
			return nil, "", httpapi.ValidationError(map[string]string{"cursor": "游标无效"})
		}
		args = append(args, createdAt, id)
		createdAtParam := store.Itoa(len(args) - 1)
		idParam := store.Itoa(len(args))
		conds = append(conds, "(u.created_at, u.id::text) < ($"+createdAtParam+"::timestamptz, $"+idParam+")")
	}
	if len(conds) > 0 {
		q += " AND " + strings.Join(conds, " AND ")
	}
	args = append(args, f.Limit)
	q += " ORDER BY u.created_at DESC, u.id DESC LIMIT $" + store.Itoa(len(args))
	rows, err := store.CollectRows[userListRow](ctx, s.db, q, args...)
	if err != nil {
		return nil, "", err
	}
	out := make([]UserListItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.item())
	}
	next := ""
	if len(rows) == f.Limit {
		last := rows[len(rows)-1]
		next = last.CreatedAt + "\x00" + last.ID
	}
	return out, next, nil
}

func parseCursor(value string) (string, string, error) {
	parts := strings.Split(value, "\x00")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("invalid cursor")
	}
	valid := false
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999-07", "2006-01-02 15:04:05-07"} {
		if _, err := time.Parse(layout, parts[0]); err == nil {
			valid = true
			break
		}
	}
	if !valid {
		return "", "", errors.New("invalid cursor timestamp")
	}
	return parts[0], parts[1], nil
}

func (s *Store) Summary(ctx context.Context, dateRange DateRange) (UsersSummary, error) {
	var summary UsersSummary
	args := dateRange.args()
	err := s.db.QueryRow(ctx, `
WITH active_users AS (
  SELECT ps.user_id FROM practice_sessions ps WHERE `+dateClause("ps.created_at")+`
  UNION
  SELECT ua.user_id FROM user_answers ua WHERE `+dateClause("ua.updated_at")+`
  UNION
  SELECT ar.user_id FROM ai_runs ar WHERE `+dateClause("ar.created_at")+` AND ar.user_id IS NOT NULL
  UNION
  SELECT s.user_id FROM auth_sessions s WHERE `+dateClause("s.created_at")+`
)
SELECT count(*)::int,
       (count(*) FILTER (WHERE role = 'learner'))::int,
       (count(*) FILTER (WHERE role = 'admin'))::int,
       (count(*) FILTER (WHERE `+dateClause("created_at")+`))::int,
       (SELECT count(*)::int FROM active_users)
FROM users`, args...).Scan(&summary.TotalUsers, &summary.LearnerUsers, &summary.AdminUsers, &summary.NewUsers, &summary.ActiveUsers)
	if err != nil {
		return UsersSummary{}, err
	}
	usage, err := s.Usage(ctx, dateRange, "")
	if err != nil {
		return UsersSummary{}, err
	}
	summary.Usage = usage
	summary.AIByKind, err = s.AIByKind(ctx, dateRange, "")
	if err != nil {
		return UsersSummary{}, err
	}
	summary.AIByModel, err = s.AIByModel(ctx, dateRange, "")
	if err != nil {
		return UsersSummary{}, err
	}
	summary.AIDaily, err = s.AIDaily(ctx, dateRange, "")
	if err != nil {
		return UsersSummary{}, err
	}
	return summary, nil
}

type aiBreakdownRow struct {
	Key              string
	Calls            int
	SuccessfulCalls  int
	FailedCalls      int
	PromptTokens     int64
	CompletionTokens int64
	DurationMs       int64
	CostedCalls      int
	EstimatedCostUSD float64
}

func (s *Store) AIByKind(ctx context.Context, dateRange DateRange, userID string) ([]AIUsageBreakdown, error) {
	return s.aiBreakdown(ctx, dateRange, userID, "ar.kind", "kind")
}

func (s *Store) AIByModel(ctx context.Context, dateRange DateRange, userID string) ([]AIUsageBreakdown, error) {
	return s.aiBreakdown(ctx, dateRange, userID, "ar.model", "model")
}

func (s *Store) aiBreakdown(ctx context.Context, dateRange DateRange, userID, groupColumn, orderColumn string) ([]AIUsageBreakdown, error) {
	q := `SELECT ` + groupColumn + `, count(*)::int,
       (count(*) FILTER (WHERE ar.error = ''))::int,
       (count(*) FILTER (WHERE ar.error <> ''))::int,
       coalesce(sum(ar.prompt_tokens), 0)::bigint,
       coalesce(sum(ar.completion_tokens), 0)::bigint,
       coalesce(sum(ar.duration_ms), 0)::bigint,
       (count(*) FILTER (WHERE ar.estimated_cost_usd IS NOT NULL))::int,
       coalesce(sum(ar.estimated_cost_usd), 0)::float8
FROM ai_runs ar
WHERE ` + dateClause("ar.created_at") + ` AND ar.user_id IS NOT NULL AND ` + userClause("ar.user_id") + `
GROUP BY ` + groupColumn + `
ORDER BY count(*) DESC, ` + orderColumn
	rows, err := store.CollectRows[aiBreakdownRow](ctx, s.db, q, append(dateRange.args(), userID)...)
	if err != nil {
		return nil, err
	}
	out := make([]AIUsageBreakdown, 0, len(rows))
	for _, row := range rows {
		out = append(out, aiBreakdown(row.Key, row.Calls, row.SuccessfulCalls, row.FailedCalls,
			row.PromptTokens, row.CompletionTokens, row.DurationMs, row.CostedCalls, row.EstimatedCostUSD))
	}
	return out, nil
}

type aiDailyRow struct {
	Date             string
	Calls            int
	FailedCalls      int
	PromptTokens     int64
	CompletionTokens int64
	DurationMs       int64
	CostedCalls      int
	EstimatedCostUSD float64
}

func (s *Store) AIDaily(ctx context.Context, dateRange DateRange, userID string) ([]AIDailyUsage, error) {
	q := `SELECT ar.created_at::date::text, count(*)::int,
       (count(*) FILTER (WHERE ar.error <> ''))::int,
       coalesce(sum(ar.prompt_tokens), 0)::bigint,
       coalesce(sum(ar.completion_tokens), 0)::bigint,
       coalesce(sum(ar.duration_ms), 0)::bigint,
       (count(*) FILTER (WHERE ar.estimated_cost_usd IS NOT NULL))::int,
       coalesce(sum(ar.estimated_cost_usd), 0)::float8
FROM ai_runs ar
WHERE ` + dateClause("ar.created_at") + ` AND ar.user_id IS NOT NULL AND ` + userClause("ar.user_id") + `
GROUP BY ar.created_at::date
ORDER BY ar.created_at::date DESC
LIMIT $4`
	rows, err := store.CollectRows[aiDailyRow](ctx, s.db, q, append(append(dateRange.args(), userID), 366)...)
	if err != nil {
		return nil, err
	}
	out := make([]AIDailyUsage, 0, len(rows))
	for _, row := range rows {
		out = append(out, AIDailyUsage{
			Date: row.Date, Calls: row.Calls, FailedCalls: row.FailedCalls,
			PromptTokens: row.PromptTokens, CompletionTokens: row.CompletionTokens,
			TotalTokens: row.PromptTokens + row.CompletionTokens, DurationMs: row.DurationMs,
			CostedCalls:      row.CostedCalls,
			EstimatedCostUSD: costPointer(row.CostedCalls, row.EstimatedCostUSD),
		})
	}
	return out, nil
}

func costPointer(costed int, value float64) *float64 {
	if costed == 0 {
		return nil
	}
	return &value
}

func (s *Store) Profile(ctx context.Context, userID string) (UserProfile, error) {
	var profile UserProfile
	err := s.db.QueryRow(ctx, `
SELECT u.id::text, u.email, u.role, u.default_level_id::text,
       coalesce(l.code, ''), coalesce(l.name, ''), u.created_at::text
FROM users u
LEFT JOIN exam_levels l ON l.id = u.default_level_id
WHERE u.id::text = $1`, userID).Scan(
		&profile.ID, &profile.Email, &profile.Role, &profile.DefaultLevelID,
		&profile.DefaultLevelCode, &profile.DefaultLevelName, &profile.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserProfile{}, httpapi.ErrNotFound
	}
	if err != nil {
		return UserProfile{}, err
	}
	return profile, nil
}

type recentAIRunRow struct {
	ID               string
	Kind             string
	PromptVersion    string
	Model            string
	InputRef         string
	Status           string
	PromptTokens     int
	CompletionTokens int
	DurationMs       int
	EstimatedCostUSD *float64
	Error            string
	CreatedAt        string
}

func (s *Store) RecentAIRuns(ctx context.Context, dateRange DateRange, userID string) ([]RecentAIRun, error) {
	q := `SELECT ar.id::text, ar.kind, ar.prompt_version, ar.model, ar.input_ref,
       CASE WHEN ar.error = '' THEN 'succeeded' ELSE 'failed' END,
       ar.prompt_tokens, ar.completion_tokens, ar.duration_ms,
       ar.estimated_cost_usd::float8, ar.error, ar.created_at::text
FROM ai_runs ar
WHERE ` + dateClause("ar.created_at") + ` AND ar.user_id::text = $3
ORDER BY ar.created_at DESC
LIMIT $4`
	rows, err := store.CollectRows[recentAIRunRow](ctx, s.db, q, append(append(dateRange.args(), userID), 100)...)
	if err != nil {
		return nil, err
	}
	out := make([]RecentAIRun, 0, len(rows))
	for _, row := range rows {
		out = append(out, RecentAIRun{
			ID: row.ID, Kind: row.Kind, PromptVersion: row.PromptVersion, Model: row.Model,
			InputRef: row.InputRef, Status: row.Status, PromptTokens: row.PromptTokens,
			CompletionTokens: row.CompletionTokens, TotalTokens: row.PromptTokens + row.CompletionTokens,
			DurationMs: row.DurationMs, EstimatedCostUSD: row.EstimatedCostUSD, Error: row.Error, CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

type recentPracticeRow struct {
	ID              string
	Status          string
	Mode            string
	RequestedCount  int
	TotalCount      int
	AnsweredCount   int
	AISummaryStatus string
	CreatedAt       string
	SubmittedAt     *string
	DeletedAt       *string
}

func (s *Store) RecentPracticeSessions(ctx context.Context, dateRange DateRange, userID string) ([]RecentPracticeSession, error) {
	q := `SELECT ps.id::text, ps.status, coalesce(ps.scope->>'mode', 'comprehensive'), ps.requested_count,
       count(pi.id)::int, (count(ua.item_id) FILTER (WHERE ua.value IS NOT NULL))::int,
       ps.ai_summary_status, ps.created_at::text, ps.submitted_at::text, ps.deleted_at::text
FROM practice_sessions ps
LEFT JOIN practice_items pi ON pi.session_id = ps.id
LEFT JOIN user_answers ua ON ua.item_id = pi.id
WHERE ps.user_id::text = $3 AND ` + dateClause("ps.created_at") + `
GROUP BY ps.id
ORDER BY ps.created_at DESC
LIMIT $4`
	rows, err := store.CollectRows[recentPracticeRow](ctx, s.db, q, append(append(dateRange.args(), userID), 50)...)
	if err != nil {
		return nil, err
	}
	out := make([]RecentPracticeSession, 0, len(rows))
	for _, row := range rows {
		out = append(out, RecentPracticeSession{
			ID: row.ID, Status: row.Status, Mode: row.Mode, RequestedCount: row.RequestedCount,
			TotalCount: row.TotalCount, AnsweredCount: row.AnsweredCount, AISummaryStatus: row.AISummaryStatus,
			CreatedAt: row.CreatedAt, SubmittedAt: row.SubmittedAt, DeletedAt: row.DeletedAt,
		})
	}
	return out, nil
}
