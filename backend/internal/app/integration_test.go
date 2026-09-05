//go:build integration

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/aishuati/backend/internal/ai"
	"github.com/aishuati/backend/internal/config"
	"github.com/aishuati/backend/internal/learning"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type integrationData struct {
	pool            *pgxpool.Pool
	server          *httptest.Server
	admin           *http.Client
	learnerA        *http.Client
	learnerB        *http.Client
	adminID         string
	levelID         string
	subjectID       string
	knowledgePoint1 string
	knowledgePoint2 string
	sectionID       string
	keyQuestionID   string
	noAnswerID      string
}

type preSessionResponse struct {
	ID    string `json:"id"`
	Items []struct {
		ID   string `json:"id"`
		Stem string `json:"stem"`
	} `json:"items"`
}

type submittedAnswer struct {
	ItemID string          `json:"itemId"`
	Value  json.RawMessage `json:"value"`
}

type resultResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Summary struct {
		Confirmed struct {
			Correct int `json:"correct"`
			Total   int `json:"total"`
		} `json:"confirmed"`
		AI struct {
			Correct int `json:"correct"`
			Pending int `json:"pending"`
		} `json:"ai"`
	} `json:"summary"`
	Items []struct {
		ID            string `json:"id"`
		GradingSource string `json:"gradingSource"`
		GradingStatus string `json:"gradingStatus"`
	} `json:"items"`
}

func TestPracticeHTTPIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("设置 TEST_DATABASE_URL 后运行 PostgreSQL 集成测试")
	}

	ctx := context.Background()
	pool, err := store.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	resetIntegrationDatabase(t, pool)
	data := seedIntegrationData(t, pool)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := newHTTPHandler(ctx, config.Config{
		AppEnv: "dev", PublicOrigin: "http://localhost:5173", RunWorker: false,
		WorkerConcurrency: 1, SessionTTL: time.Hour, AITimeout: time.Second,
	}, pool, logger)
	server := httptest.NewServer(handler)
	data.pool, data.server = pool, server
	t.Cleanup(server.Close)
	data.admin = newIntegrationClient(t)
	data.learnerA = newIntegrationClient(t)
	data.learnerB = newIntegrationClient(t)
	loginIntegration(t, data.admin, server.URL, "admin@example.com", "admin-pass-123")
	loginIntegration(t, data.learnerA, server.URL, "learner-a@example.com", "learner-pass-123")
	loginIntegration(t, data.learnerB, server.URL, "learner-b@example.com", "learner-pass-123")

	t.Run("管理端用户用量接口", func(t *testing.T) {
		learnerID := dataUserID(t, pool, "learner-a@example.com")
		if _, err := pool.Exec(context.Background(), `
			INSERT INTO ai_runs (user_id, kind, prompt_version, model, input_ref, prompt_tokens, completion_tokens, duration_ms, error, estimated_cost_usd)
			VALUES ($1, 'practice_question_generation', 'test.v1', 'test-model', 'test-ref', 17, 3, 42, '', 0.0012)`, learnerID); err != nil {
			t.Fatal(err)
		}
		var page struct {
			Summary struct {
				TotalUsers int `json:"totalUsers"`
				Usage      struct {
					AI struct {
						Calls int `json:"calls"`
					} `json:"ai"`
				} `json:"usage"`
			} `json:"summary"`
			Users []struct {
				Email string `json:"email"`
				Usage struct {
					AI struct {
						Calls int `json:"calls"`
					} `json:"ai"`
				} `json:"usage"`
			} `json:"users"`
		}
		decodeResponse(t, jsonRequest(t, data.admin, server.URL, http.MethodGet, "/api/v1/admin/users", nil, ""), &page)
		if page.Summary.TotalUsers != 3 || page.Summary.Usage.AI.Calls != 1 || len(page.Users) != 3 {
			t.Fatalf("unexpected user page: %+v", page)
		}
		var detail struct {
			User struct {
				Email string `json:"email"`
			} `json:"user"`
			Usage struct {
				PracticeSessions int `json:"practiceSessions"`
				AI               struct {
					Calls            int      `json:"calls"`
					PromptTokens     int64    `json:"promptTokens"`
					EstimatedCostUSD *float64 `json:"estimatedCostUsd"`
				} `json:"ai"`
			} `json:"usage"`
			RecentAIRuns []struct {
				Kind string `json:"kind"`
			} `json:"recentAiRuns"`
		}
		decodeResponse(t, jsonRequest(t, data.admin, server.URL, http.MethodGet, "/api/v1/admin/users/"+learnerID, nil, ""), &detail)
		if detail.User.Email != "learner-a@example.com" || detail.Usage.PracticeSessions != 0 ||
			detail.Usage.AI.Calls != 1 || detail.Usage.AI.PromptTokens != 17 || detail.Usage.AI.EstimatedCostUSD == nil ||
			*detail.Usage.AI.EstimatedCostUSD != 0.0012 || len(detail.RecentAIRuns) != 1 || detail.RecentAIRuns[0].Kind != "practice_question_generation" {
			t.Fatalf("unexpected user detail: %+v", detail)
		}
		var emptyRange struct {
			Summary struct {
				Usage struct {
					AI struct {
						Calls int `json:"calls"`
					} `json:"ai"`
				} `json:"usage"`
			} `json:"summary"`
		}
		decodeResponse(t, jsonRequest(t, data.admin, server.URL, http.MethodGet,
			"/api/v1/admin/users?from=2099-01-01&to=2099-01-02", nil, ""), &emptyRange)
		if emptyRange.Summary.Usage.AI.Calls != 0 {
			t.Fatalf("future date range should have no AI calls: %+v", emptyRange)
		}
		var filtered struct {
			Users []struct {
				Email string `json:"email"`
			} `json:"users"`
		}
		decodeResponse(t, jsonRequest(t, data.admin, server.URL, http.MethodGet,
			"/api/v1/admin/users?role=learner&q=learner-a", nil, ""), &filtered)
		if len(filtered.Users) != 1 || filtered.Users[0].Email != "learner-a@example.com" {
			t.Fatalf("role and email filters should narrow users: %+v", filtered.Users)
		}
		var firstPage, secondPage struct {
			Users []struct {
				Email string `json:"email"`
			} `json:"users"`
			NextCursor string `json:"nextCursor"`
		}
		decodeResponse(t, jsonRequest(t, data.admin, server.URL, http.MethodGet, "/api/v1/admin/users?limit=1", nil, ""), &firstPage)
		if len(firstPage.Users) != 1 || firstPage.NextCursor == "" {
			t.Fatalf("first user page should include a cursor: %+v", firstPage)
		}
		decodeResponse(t, jsonRequest(t, data.admin, server.URL, http.MethodGet,
			"/api/v1/admin/users?limit=1&cursor="+url.QueryEscape(firstPage.NextCursor), nil, ""), &secondPage)
		if len(secondPage.Users) != 1 || secondPage.Users[0].Email == firstPage.Users[0].Email {
			t.Fatalf("cursor should advance user page: %+v", secondPage)
		}
		assertStatus(t, jsonRequest(t, data.learnerA, server.URL, http.MethodGet, "/api/v1/admin/users", nil, ""), http.StatusForbidden)
	})

	t.Run("管理概览质量入口与题目筛选", func(t *testing.T) {
		var overview struct {
			PublishedNoKnowledge int `json:"publishedNoKnowledge"`
			PublishedNoSource    int `json:"publishedNoSource"`
			PublishedNoAnswer    int `json:"publishedNoAnswer"`
		}
		decodeResponse(t, jsonRequest(t, data.admin, server.URL, http.MethodGet, "/api/v1/admin/overview", nil, ""), &overview)
		if overview.PublishedNoKnowledge != 0 || overview.PublishedNoSource != 0 || overview.PublishedNoAnswer != 1 {
			t.Fatalf("unexpected quality overview: %+v", overview)
		}
		var filtered struct {
			Questions []struct {
				ID string `json:"id"`
			} `json:"questions"`
		}
		decodeResponse(t, jsonRequest(t, data.admin, server.URL, http.MethodGet,
			"/api/v1/admin/questions?status=published&quality=no_answer", nil, ""), &filtered)
		if len(filtered.Questions) != 1 {
			t.Fatalf("quality filter should find the single no-answer question: %+v", filtered.Questions)
		}
	})

	t.Run("账号隔离、权限和答题前 DTO", func(t *testing.T) {
		pre := createIntegrationSession(t, data, data.learnerA)
		itemID := pre.Items[0].ID
		assertStatus(t, jsonRequest(t, data.learnerB, server.URL, http.MethodGet,
			"/api/v1/practice-sessions/"+pre.ID, nil, ""), http.StatusNotFound)
		assertStatus(t, jsonRequest(t, data.learnerB, server.URL, http.MethodPut,
			"/api/v1/practice-sessions/"+pre.ID+"/answers/"+itemID,
			map[string]any{"value": map[string]any{"optionIds": []string{"a"}}}, ""), http.StatusNotFound)
		assertStatus(t, jsonRequest(t, data.learnerB, server.URL, http.MethodGet,
			"/api/v1/practice-sessions/"+pre.ID+"/result", nil, ""), http.StatusNotFound)
		assertStatus(t, jsonRequest(t, data.learnerB, server.URL, http.MethodGet,
			"/api/v1/admin/questions", nil, ""), http.StatusForbidden)

		resp := jsonRequest(t, data.learnerA, server.URL, http.MethodGet,
			"/api/v1/practice-sessions/"+pre.ID, nil, "")
		body := readResponse(t, resp)
		for _, forbidden := range []string{"correctAnswer", "answerAuthority", "gradingStatus", "gradingSource", "explanation", "aiAnalysis"} {
			if bytes.Contains(body, []byte(forbidden)) {
				t.Fatalf("pre-submit response leaked %q: %s", forbidden, body)
			}
		}
	})

	t.Run("管理员创建单个和多个知识点题目", func(t *testing.T) {
		for _, knowledgePointIDs := range [][]string{{data.knowledgePoint1}, {data.knowledgePoint1, data.knowledgePoint2}} {
			resp := jsonRequest(t, data.admin, server.URL, http.MethodPost, "/api/v1/admin/questions", map[string]any{
				"type": "single_choice", "stem": "知识点保存测试题。", "options": []map[string]string{
					{"id": "a", "label": "A", "text": "甲"}, {"id": "b", "label": "B", "text": "乙"},
				}, "levelId": data.levelID, "subjectId": data.subjectID, "sourceSectionId": data.sectionID,
				"difficulty": 3, "knowledgePointIds": knowledgePointIDs,
				"answer": map[string]any{"value": map[string]any{"optionIds": []string{"a"}}, "authority": "official"},
			}, "")
			var created struct {
				Question struct {
					ID string `json:"id"`
				} `json:"question"`
			}
			decodeResponse(t, resp, &created)
			if countRows(t, pool, `SELECT count(*) FROM question_version_knowledge_points qvkp JOIN questions q ON q.current_version_id = qvkp.question_version_id WHERE q.id = $1`, created.Question.ID) != len(knowledgePointIDs) {
				t.Fatalf("expected %d knowledge point links", len(knowledgePointIDs))
			}
		}
	})

	t.Run("题目版本发布不影响旧批次", func(t *testing.T) {
		body := map[string]any{
			"type": "single_choice", "stem": "第 1 题新版本题干。", "options": []map[string]string{
				{"id": "a", "label": "A", "text": "甲"}, {"id": "b", "label": "B", "text": "乙"},
			}, "levelId": data.levelID, "subjectId": data.subjectID, "sourceSectionId": data.sectionID,
			"difficulty": 3, "knowledgePointIds": []string{data.knowledgePoint1},
			"answer": map[string]any{"value": map[string]any{"optionIds": []string{"b"}}, "authority": "human_verified", "explanation": "新版本解析"},
		}
		resp := jsonRequest(t, data.admin, server.URL, http.MethodPatch,
			"/api/v1/admin/questions/"+data.keyQuestionID, body, "")
		assertStatus(t, resp, http.StatusOK)
		assertStatus(t, jsonRequest(t, data.admin, server.URL, http.MethodPost,
			"/api/v1/admin/questions/"+data.keyQuestionID+"/publish", nil, ""), http.StatusOK)

		var old preSessionResponse
		// 之前的批次由前一个子测试创建，按数据库快照验证其题干仍是旧版本。
		row := pool.QueryRow(context.Background(),
			`SELECT ps.id::text FROM practice_sessions ps WHERE ps.user_id = (SELECT id FROM users WHERE email = 'learner-a@example.com') ORDER BY ps.created_at LIMIT 1`)
		var sessionID string
		if err := row.Scan(&sessionID); err != nil {
			t.Fatal(err)
		}
		resp = jsonRequest(t, data.learnerA, server.URL, http.MethodGet,
			"/api/v1/practice-sessions/"+sessionID, nil, "")
		decodeResponse(t, resp, &old)
		itemID := itemIDForQuestion(t, pool, sessionID, data.keyQuestionID)
		for _, item := range old.Items {
			if item.ID == itemID && item.Stem != "第 1 题旧题干。" {
				t.Fatalf("old session read a new question version: %q", item.Stem)
			}
		}
		if countRows(t, pool, `SELECT count(*) FROM question_versions WHERE question_id = $1`, data.keyQuestionID) != 2 {
			t.Fatal("publishing an edit should create a second immutable version")
		}
	})

	var learnerASession preSessionResponse
	t.Run("提交竞态、分层判分、整批 AI 任务和幂等", func(t *testing.T) {
		row := pool.QueryRow(context.Background(),
			`SELECT ps.id::text FROM practice_sessions ps WHERE ps.user_id = (SELECT id FROM users WHERE email = 'learner-a@example.com') ORDER BY ps.created_at LIMIT 1`)
		if err := row.Scan(&learnerASession.ID); err != nil {
			t.Fatal(err)
		}
		resp := jsonRequest(t, data.learnerA, server.URL, http.MethodGet,
			"/api/v1/practice-sessions/"+learnerASession.ID, nil, "")
		decodeResponse(t, resp, &learnerASession)
		answers := finalAnswers(t, pool, learnerASession.ID, data)
		raw := submitJSON(t, answers)
		resp = rawRequest(t, data.learnerA, server.URL, http.MethodPost,
			"/api/v1/practice-sessions/"+learnerASession.ID+"/submit", raw, map[string]string{"Idempotency-Key": "idem-a"})
		var result resultResponse
		decodeResponse(t, resp, &result)
		if result.Status != "grading" || result.Summary.Confirmed.Total != 9 || result.Summary.Confirmed.Correct != 9 || result.Summary.AI.Pending != 1 {
			t.Fatalf("unexpected layered result: %+v", result)
		}
		var keyItem, noAnswerItem struct {
			ID            string
			GradingSource string
			GradingStatus string
		}
		for _, item := range result.Items {
			if item.ID == itemIDForQuestion(t, pool, learnerASession.ID, data.keyQuestionID) {
				keyItem.ID, keyItem.GradingSource, keyItem.GradingStatus = item.ID, item.GradingSource, item.GradingStatus
			}
			if item.ID == itemIDForQuestion(t, pool, learnerASession.ID, data.noAnswerID) {
				noAnswerItem.ID, noAnswerItem.GradingSource, noAnswerItem.GradingStatus = item.ID, item.GradingSource, item.GradingStatus
			}
		}
		if keyItem.GradingSource != "deterministic" || keyItem.GradingStatus != "correct" || noAnswerItem.GradingSource != "ai" || noAnswerItem.GradingStatus != "pending" {
			t.Fatalf("unexpected grading sources: %+v %+v", keyItem, noAnswerItem)
		}
		if countRows(t, pool, `SELECT count(*) FROM jobs WHERE kind = 'analyze_practice_session_ai' AND payload->>'sessionId' = $1`, learnerASession.ID) != 1 {
			t.Fatal("a submitted batch should enqueue exactly one batch AI job")
		}
		if countRows(t, pool, `SELECT count(*) FROM jobs WHERE kind IN ('grade_practice_item_ai', 'explain_practice_item_ai') AND payload->>'sessionId' = $1`, learnerASession.ID) != 0 {
			t.Fatal("submitted batch must not enqueue per-item AI jobs")
		}

		decodeResponse(t, rawRequest(t, data.learnerA, server.URL, http.MethodPost,
			"/api/v1/practice-sessions/"+learnerASession.ID+"/submit", raw, map[string]string{"Idempotency-Key": "idem-a"}), &result)
		conflictAnswers := append([]submittedAnswer(nil), answers...)
		conflictAnswers[0].Value = json.RawMessage(`{"optionIds":["b"]}`)
		conflictBody := submitJSON(t, conflictAnswers)
		resp = rawRequest(t, data.learnerA, server.URL, http.MethodPost,
			"/api/v1/practice-sessions/"+learnerASession.ID+"/submit", conflictBody, map[string]string{"Idempotency-Key": "idem-a"})
		if resp.StatusCode != http.StatusConflict || !bytes.Contains(readResponse(t, resp), []byte("idempotency_conflict")) {
			t.Fatalf("expected idempotency conflict, got %d", resp.StatusCode)
		}
	})

	t.Run("失败的本批 AI 分析可重试且重复点击不重复入队", func(t *testing.T) {
		if _, err := pool.Exec(context.Background(),
			`UPDATE jobs SET status = 'failed' WHERE kind = 'analyze_practice_session_ai' AND payload->>'sessionId' = $1`, learnerASession.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`UPDATE grading_results SET status = 'failed', explanation = 'AI 失败', explanation_source = 'ai' WHERE session_id = $1 AND source = 'ai'`, learnerASession.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(),
			`UPDATE practice_sessions SET status = 'analysis_failed', ai_summary_status = 'failed' WHERE id = $1`, learnerASession.ID); err != nil {
			t.Fatal(err)
		}
		path := "/api/v1/practice-sessions/" + learnerASession.ID + "/analysis/retry"
		assertStatus(t, jsonRequest(t, data.learnerB, server.URL, http.MethodPost, path, nil, ""), http.StatusNotFound)
		var result resultResponse
		decodeResponse(t, jsonRequest(t, data.learnerA, server.URL, http.MethodPost, path, nil, ""), &result)
		if result.Status != "grading" || result.Summary.AI.Pending != 1 {
			t.Fatalf("retry should restore pending AI result: %+v", result)
		}
		decodeResponse(t, jsonRequest(t, data.learnerA, server.URL, http.MethodPost, path, nil, ""), &result)
		if countRows(t, pool, `SELECT count(*) FROM jobs WHERE kind = 'analyze_practice_session_ai' AND payload->>'sessionId' = $1`, learnerASession.ID) != 2 ||
			countRows(t, pool, `SELECT count(*) FROM jobs WHERE kind = 'analyze_practice_session_ai' AND status IN ('queued', 'running') AND payload->>'sessionId' = $1`, learnerASession.ID) != 1 {
			t.Fatal("repeated analysis retry should leave one active batch job")
		}
	})

	t.Run("统计重算、薄弱点推荐和知识点专项练习", func(t *testing.T) {
		pre := createIntegrationSession(t, data, data.learnerB)
		answers := finalAnswers(t, pool, pre.ID, data)
		// 首批 9 道有权威答案题中故意答错 5 道，留下 4 道正确和 1 道 AI 题。
		for _, index := range []int{1, 2, 3, 4, 5} {
			answers[index].Value = json.RawMessage(`{"optionIds":["b"]}`)
		}
		assertStatus(t, rawRequest(t, data.learnerB, server.URL, http.MethodPost,
			"/api/v1/practice-sessions/"+pre.ID+"/submit", submitJSON(t, answers), map[string]string{"Idempotency-Key": "idem-learning-first"}), http.StatusOK)

		var learnerBID string
		if err := pool.QueryRow(context.Background(), `SELECT id::text FROM users WHERE email = 'learner-b@example.com'`).Scan(&learnerBID); err != nil {
			t.Fatal(err)
		}
		learningStore := learning.NewStore(pool)
		if err := learningStore.RebuildUserStats(context.Background(), pool, learnerBID); err != nil {
			t.Fatal(err)
		}
		var retiredID string
		if err := pool.QueryRow(context.Background(), `INSERT INTO knowledge_points
			(exam_id, level_id, subject_id, name, status)
			SELECT l.exam_id, l.id, s.id, '历史退休知识点', 'retired'
			FROM exam_levels l JOIN subjects s ON s.exam_id = l.exam_id
			WHERE l.id = $1 AND s.id = $2 RETURNING id::text`, data.levelID, data.subjectID).Scan(&retiredID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(), `INSERT INTO user_knowledge_stats
			(user_id, knowledge_point_id, confirmed_answered, recent_answered, updated_at)
			VALUES ($1, $2, 5, 5, now())`, learnerBID, retiredID); err != nil {
			t.Fatal(err)
		}
		var dashboard struct {
			Recommendations []struct {
				KnowledgePointID *string  `json:"knowledgePointId"`
				RecentAnswered   int      `json:"recentAnswered"`
				RecentWrongCount int      `json:"recentWrongCount"`
				Accuracy         *float64 `json:"accuracy"`
			} `json:"recommendations"`
		}
		decodeResponse(t, jsonRequest(t, data.learnerB, server.URL, http.MethodGet, "/api/v1/dashboard", nil, ""), &dashboard)
		foundRecommendation := false
		for _, rec := range dashboard.Recommendations {
			if rec.KnowledgePointID != nil && *rec.KnowledgePointID == data.knowledgePoint1 {
				foundRecommendation = rec.RecentAnswered == 9 && rec.RecentWrongCount == 5 && rec.Accuracy != nil && *rec.Accuracy < 0.5
			}
		}
		if !foundRecommendation {
			t.Fatalf("low-accuracy knowledge point was not recommended: %+v", dashboard.Recommendations)
		}
		for _, rec := range dashboard.Recommendations {
			if rec.KnowledgePointID != nil && *rec.KnowledgePointID == retiredID {
				t.Fatal("dashboard must not recommend a retired knowledge point")
			}
		}
		var points struct {
			KnowledgePoints []struct {
				ID string `json:"id"`
			} `json:"knowledgePoints"`
		}
		decodeResponse(t, jsonRequest(t, data.learnerB, server.URL, http.MethodGet, "/api/v1/knowledge-points", nil, ""), &points)
		for _, point := range points.KnowledgePoints {
			if point.ID == retiredID {
				t.Fatal("knowledge point list must not expose retired knowledge point")
			}
		}
		assertStatus(t, jsonRequest(t, data.learnerB, server.URL, http.MethodGet,
			"/api/v1/knowledge-points/"+retiredID, nil, ""), http.StatusNotFound)
		assertStatus(t, jsonRequest(t, data.learnerB, server.URL, http.MethodPost, "/api/v1/practice-sessions", map[string]any{
			"levelId": data.levelID, "mode": "knowledge", "knowledgePointIds": []string{retiredID}, "count": 10,
		}, ""), http.StatusNotFound)

		special := jsonRequest(t, data.learnerB, server.URL, http.MethodPost, "/api/v1/practice-sessions", map[string]any{
			"levelId": data.levelID, "mode": "knowledge", "knowledgePointIds": []string{data.knowledgePoint1}, "count": 10,
		}, "")
		var specialSession preSessionResponse
		decodeResponse(t, special, &specialSession)
		if len(specialSession.Items) != 10 {
			t.Fatalf("expected 10 specialized items, got %d", len(specialSession.Items))
		}
		if countRows(t, pool, `SELECT count(*) FROM practice_items pi WHERE pi.session_id = $1 AND NOT EXISTS (
			SELECT 1 FROM question_version_knowledge_points m WHERE m.question_version_id = pi.question_version_id AND m.knowledge_point_id = $2
		)`, specialSession.ID, data.knowledgePoint1) != 0 {
			t.Fatal("specialized practice selected a question outside the requested knowledge point")
		}
		specialAnswers := finalAnswers(t, pool, specialSession.ID, data)
		assertStatus(t, rawRequest(t, data.learnerB, server.URL, http.MethodPost,
			"/api/v1/practice-sessions/"+specialSession.ID+"/submit", submitJSON(t, specialAnswers), map[string]string{"Idempotency-Key": "idem-learning-special"}), http.StatusOK)

		if _, err := pool.Exec(context.Background(), `UPDATE grading_results SET status = 'correct' WHERE session_id = $1 AND source = 'ai'`, pre.ID); err != nil {
			t.Fatal(err)
		}
		if err := learningStore.RebuildUserStats(context.Background(), pool, learnerBID); err != nil {
			t.Fatal(err)
		}
		var confirmedAnswered, confirmedCorrect, aiAnswered, aiCorrect int
		if err := pool.QueryRow(context.Background(), `SELECT confirmed_answered, confirmed_correct, ai_answered, ai_correct
			FROM user_knowledge_stats WHERE user_id = $1 AND knowledge_point_id = $2`, learnerBID, data.knowledgePoint1).
			Scan(&confirmedAnswered, &confirmedCorrect, &aiAnswered, &aiCorrect); err != nil {
			t.Fatal(err)
		}
		if confirmedAnswered != 19 || confirmedCorrect != 14 || aiAnswered != 0 || aiCorrect != 0 {
			t.Fatalf("unexpected confirmed stats after re-practice: %d/%d ai=%d/%d", confirmedAnswered, confirmedCorrect, aiAnswered, aiCorrect)
		}
		if err := pool.QueryRow(context.Background(), `SELECT ai_answered, ai_correct FROM user_knowledge_stats
			WHERE user_id = $1 AND knowledge_point_id = $2`, learnerBID, data.knowledgePoint2).Scan(&aiAnswered, &aiCorrect); err != nil {
			t.Fatalf("read AI-only stats for %s: %v", data.knowledgePoint2, err)
		}
		if aiAnswered != 1 || aiCorrect != 1 {
			t.Fatalf("AI result should remain separate from confirmed stats: %d/%d", aiAnswered, aiCorrect)
		}
		var memory learning.LearningMemory
		decodeResponse(t, jsonRequest(t, data.learnerB, server.URL, http.MethodGet, "/api/v1/learning-memory", nil, ""), &memory)
		if memory.EstimatedAccuracy == nil || *memory.EstimatedAccuracy != 0.75 {
			t.Fatalf("unexpected estimated accuracy: %+v", memory)
		}
		if countRows(t, pool, `SELECT count(*) FROM grading_results gr JOIN practice_sessions ps ON ps.id = gr.session_id WHERE ps.user_id = $1`, learnerBID) != 20 {
			t.Fatal("re-practice should preserve the first batch grading history")
		}
	})

	t.Run("自动保存与提交同时发生时最终提交胜出", func(t *testing.T) {
		pre := createIntegrationSession(t, data, data.learnerB)
		first := pre.Items[0].ID
		answers := finalAnswers(t, pool, pre.ID, data)
		raw := submitJSON(t, answers)
		saveRaw, _ := json.Marshal(map[string]any{"value": map[string]any{"optionIds": []string{"a"}}, "markedForReview": false})
		saveResult := make(chan requestResult, 1)
		go func() {
			resp, err := doRawRequest(data.learnerB, server.URL, http.MethodPut,
				"/api/v1/practice-sessions/"+pre.ID+"/answers/"+first, saveRaw, nil)
			saveResult <- requestResult{resp: resp, err: err}
		}()
		resp := rawRequest(t, data.learnerB, server.URL, http.MethodPost,
			"/api/v1/practice-sessions/"+pre.ID+"/submit", raw, map[string]string{"Idempotency-Key": "idem-race"})
		assertStatus(t, resp, http.StatusOK)
		save := <-saveResult
		if save.err != nil {
			t.Fatal(save.err)
		}
		saveResp := save.resp
		if saveResp.StatusCode != http.StatusOK && saveResp.StatusCode != http.StatusConflict {
			t.Fatalf("unexpected concurrent autosave status: %d", saveResp.StatusCode)
		}
		var value string
		if err := pool.QueryRow(context.Background(), `SELECT value::text FROM user_answers WHERE session_id = $1 AND item_id = $2`, pre.ID, first).Scan(&value); err != nil {
			t.Fatal(err)
		}
		var gotValue, wantValue any
		if err := json.Unmarshal([]byte(value), &gotValue); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(answersForItem(answers, first).Value, &wantValue); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(gotValue, wantValue) {
			t.Fatalf("final submit did not win over autosave: %s", value)
		}
	})

	t.Run("账号学习记忆可删除并从新进度重建", func(t *testing.T) {
		var before learning.LearningMemory
		decodeResponse(t, jsonRequest(t, data.learnerB, server.URL, http.MethodGet,
			"/api/v1/learning-memory", nil, ""), &before)
		if before.ConfirmedAnswered == 0 {
			t.Fatalf("expected account memory before deletion: %+v", before)
		}
		sessionCount := countRows(t, pool, `SELECT count(*) FROM practice_sessions WHERE user_id = (SELECT id FROM users WHERE email = 'learner-b@example.com')`)
		assertStatus(t, jsonRequest(t, data.learnerB, server.URL, http.MethodDelete,
			"/api/v1/learning-memory", nil, ""), http.StatusNoContent)

		var after learning.LearningMemory
		decodeResponse(t, jsonRequest(t, data.learnerB, server.URL, http.MethodGet,
			"/api/v1/learning-memory", nil, ""), &after)
		if after.ConfirmedAnswered != 0 || after.ConfirmedCorrect != 0 || after.Advice.Text != "" {
			t.Fatalf("memory deletion should clear derived data: %+v", after)
		}
		var wrongAfterDelete struct {
			WrongItems []json.RawMessage `json:"wrongItems"`
		}
		decodeResponse(t, jsonRequest(t, data.learnerB, server.URL, http.MethodGet,
			"/api/v1/wrong-items", nil, ""), &wrongAfterDelete)
		if len(wrongAfterDelete.WrongItems) != 0 {
			t.Fatalf("memory deletion should hide old wrong items: %d", len(wrongAfterDelete.WrongItems))
		}
		learningStore := learning.NewStore(pool)
		if err := learningStore.RebuildUserStats(context.Background(), pool, dataUserID(t, pool, "learner-b@example.com")); err != nil {
			t.Fatal(err)
		}
		decodeResponse(t, jsonRequest(t, data.learnerB, server.URL, http.MethodGet,
			"/api/v1/learning-memory", nil, ""), &after)
		if after.ConfirmedAnswered != 0 {
			t.Fatalf("rebuilding immediately after deletion must not restore old memory: %+v", after)
		}
		if countRows(t, pool, `SELECT count(*) FROM practice_sessions WHERE user_id = (SELECT id FROM users WHERE email = 'learner-b@example.com')`) != sessionCount {
			t.Fatal("deleting memory must preserve practice history")
		}

		pre := createIntegrationSession(t, data, data.learnerB)
		answers := finalAnswers(t, pool, pre.ID, data)
		assertStatus(t, rawRequest(t, data.learnerB, server.URL, http.MethodPost,
			"/api/v1/practice-sessions/"+pre.ID+"/submit", submitJSON(t, answers),
			map[string]string{"Idempotency-Key": "idem-memory-after-reset"}), http.StatusOK)
		if err := learningStore.RebuildUserStats(context.Background(), pool, dataUserID(t, pool, "learner-b@example.com")); err != nil {
			t.Fatal(err)
		}
		decodeResponse(t, jsonRequest(t, data.learnerB, server.URL, http.MethodGet,
			"/api/v1/learning-memory", nil, ""), &after)
		if after.ConfirmedAnswered == 0 {
			t.Fatalf("new progress should rebuild account memory: %+v", after)
		}
		if countRows(t, pool, `SELECT count(*) FROM jobs WHERE kind = 'analyze_practice_session_ai' AND payload->>'sessionId' = $1`, pre.ID) != 1 {
			t.Fatal("a new batch must keep exactly one batch AI job")
		}

		analysisCalls := 0
		fakeAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			analysisCalls++
			var request struct {
				Messages []struct {
					Content string `json:"content"`
				} `json:"messages"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Messages) < 2 {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			var input struct {
				RefreshMemoryAdvice bool `json:"refreshMemoryAdvice"`
				LearningMemory      struct {
					ConfirmedAnswered int `json:"confirmedAnswered"`
				} `json:"learningMemory"`
				Items []struct {
					ItemID           string `json:"itemId"`
					NeedsGrading     bool   `json:"needsGrading"`
					NeedsExplanation bool   `json:"needsExplanation"`
				} `json:"items"`
			}
			if err := json.Unmarshal([]byte(request.Messages[1].Content), &input); err != nil || input.LearningMemory.ConfirmedAnswered == 0 {
				http.Error(w, "missing learning memory", http.StatusBadRequest)
				return
			}
			if (analysisCalls == 1 && !input.RefreshMemoryAdvice) || (analysisCalls > 1 && input.RefreshMemoryAdvice) {
				http.Error(w, "unexpected memory advice refresh flag", http.StatusBadRequest)
				return
			}
			grades := []map[string]any{}
			explanations := []map[string]string{}
			for _, item := range input.Items {
				if item.NeedsGrading {
					grades = append(grades, map[string]any{
						"itemId": item.ItemID, "correctness": "cannot_determine",
						"correctAnswer": nil, "explanation": "无法可靠判定。",
					})
				}
				if item.NeedsExplanation {
					explanations = append(explanations, map[string]string{"itemId": item.ItemID, "text": "根据权威答案判断。"})
				}
			}
			memoryAdvice := ""
			if input.RefreshMemoryAdvice {
				memoryAdvice = "累计进度：继续复习已暴露的薄弱知识点。"
			}
			content, _ := json.Marshal(map[string]any{
				"summary": "本批表现：完成了当前练习。", "memoryAdvice": memoryAdvice, "grades": grades, "explanations": explanations,
			})
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{"message": map[string]string{"content": string(content)}}},
			})
		}))
		defer fakeAI.Close()
		aiClient := ai.NewClient(ai.Config{BaseURL: fakeAI.URL, APIKey: "test-key", Model: "test-model", Timeout: time.Second}, pool, logger)
		batchHandler := ai.NewService(pool, aiClient, logger).Handlers()["analyze_practice_session_ai"]
		if err := batchHandler(context.Background(), 1, 3, json.RawMessage(`{"sessionId":"`+pre.ID+`"}`)); err != nil {
			t.Fatalf("batch AI should update account advice: %v", err)
		}
		var batchSummary string
		if err := pool.QueryRow(context.Background(), `SELECT ai_summary FROM practice_sessions WHERE id = $1`, pre.ID).Scan(&batchSummary); err != nil {
			t.Fatal(err)
		}
		if batchSummary != "本批表现：完成了当前练习。" {
			t.Fatalf("batch result should keep batch summary: %q", batchSummary)
		}
		decodeResponse(t, jsonRequest(t, data.learnerB, server.URL, http.MethodGet,
			"/api/v1/learning-memory", nil, ""), &after)
		if after.Advice.Status != "completed" || after.Advice.Text != "累计进度：继续复习已暴露的薄弱知识点。" {
			t.Fatalf("successful batch AI should persist account advice: %+v", after)
		}
		if err := batchHandler(context.Background(), 1, 3, json.RawMessage(`{"sessionId":"`+pre.ID+`"}`)); err != nil {
			t.Fatalf("batch AI should succeed without refreshing recent account advice: %v", err)
		}
		decodeResponse(t, jsonRequest(t, data.learnerB, server.URL, http.MethodGet,
			"/api/v1/learning-memory", nil, ""), &after)
		if after.Advice.Text != "累计进度：继续复习已暴露的薄弱知识点。" {
			t.Fatalf("recent account advice should not be overwritten: %+v", after.Advice)
		}
	})

	t.Run("AI 根据全局记忆生成私有练习并保持 AI 判分分层", func(t *testing.T) {
		fakeAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var request struct {
				Messages []struct {
					Content string `json:"content"`
				} `json:"messages"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Messages) < 2 {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			var generation struct {
				Count          int `json:"count"`
				LearningMemory struct {
					KnowledgePoints []struct {
						ID                string  `json:"id"`
						ConfirmedAnswered int     `json:"confirmedAnswered"`
						ConfirmedCorrect  int     `json:"confirmedCorrect"`
						RecentWrongCount  int     `json:"recentWrongCount"`
						PriorityScore     float64 `json:"priorityScore"`
					} `json:"knowledgePoints"`
				} `json:"learningMemory"`
			}
			var batch struct {
				Items []struct {
					ItemID       string `json:"itemId"`
					NeedsGrading bool   `json:"needsGrading"`
				} `json:"items"`
			}
			_ = json.Unmarshal([]byte(request.Messages[1].Content), &generation)
			_ = json.Unmarshal([]byte(request.Messages[1].Content), &batch)
			var output any
			if generation.Count > 0 {
				if len(generation.LearningMemory.KnowledgePoints) == 0 {
					http.Error(w, "missing knowledge points", http.StatusBadRequest)
					return
				}
				if generation.LearningMemory.KnowledgePoints[0].ID != data.knowledgePoint1 ||
					generation.LearningMemory.KnowledgePoints[0].ConfirmedAnswered == 0 ||
					generation.LearningMemory.KnowledgePoints[0].RecentWrongCount == 0 ||
					generation.LearningMemory.KnowledgePoints[0].PriorityScore <= 0 {
					http.Error(w, "memory candidates are not ranked by weakness", http.StatusBadRequest)
					return
				}
				pointID := generation.LearningMemory.KnowledgePoints[0].ID
				questions := make([]map[string]any, generation.Count)
				for i := range questions {
					questions[i] = map[string]any{
						"type": "single_choice", "stem": fmt.Sprintf("AI 个性化测试题 %d。", i+1),
						"options": []map[string]string{
							{"id": "a", "label": "A", "text": "正解"}, {"id": "b", "label": "B", "text": "选项二"},
							{"id": "c", "label": "C", "text": "选项三"}, {"id": "d", "label": "D", "text": "选项四"},
						},
						"correctAnswer": map[string]any{"optionIds": []string{"a"}},
						"explanation":   "这是测试解析。", "knowledgePointIds": []string{pointID}, "difficulty": 3,
					}
				}
				output = map[string]any{"questions": questions}
			} else {
				grades := make([]map[string]any, 0)
				for _, item := range batch.Items {
					if item.NeedsGrading {
						grades = append(grades, map[string]any{
							"itemId": item.ItemID, "correctness": "correct",
							"correctAnswer": map[string]any{"optionIds": []string{"a"}}, "explanation": "根据生成时的答案判断。",
						})
					}
				}
				output = map[string]any{"summary": "本批表现：完成了 AI 个性化练习。", "memoryAdvice": "累计进度：继续保持练习。", "grades": grades, "explanations": []any{}}
			}
			content, _ := json.Marshal(output)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{"message": map[string]string{"content": string(content)}}},
			})
		}))
		defer fakeAI.Close()
		aiClient := ai.NewClient(ai.Config{BaseURL: fakeAI.URL, APIKey: "test-key", Model: "test-model", Timeout: time.Second}, pool, logger)
		aiService := ai.NewService(pool, aiClient, logger)
		generated, err := aiService.CreateGeneratedSession(context.Background(), dataUserID(t, pool, "learner-b@example.com"), ai.AIGenerateRequest{
			LevelID: data.levelID, SubjectID: data.subjectID, Count: 10,
		})
		if err != nil {
			t.Fatal(err)
		}
		if generated.Status != "generating" || countRows(t, pool, `SELECT count(*) FROM jobs WHERE kind = 'generate_ai_practice_session' AND payload->>'sessionId' = $1`, generated.ID) != 1 {
			t.Fatalf("AI generation should enqueue one generation job: %+v", generated)
		}
		if err := aiService.Handlers()["generate_ai_practice_session"](context.Background(), 1, 3, json.RawMessage(`{"sessionId":"`+generated.ID+`"}`)); err != nil {
			t.Fatalf("AI generation failed: %v", err)
		}
		if countRows(t, pool, `SELECT count(*) FROM practice_items WHERE session_id = $1`, generated.ID) != 10 ||
			countRows(t, pool, `SELECT count(*) FROM ai_generated_question_answers aga JOIN practice_items pi ON pi.question_version_id = aga.question_version_id WHERE pi.session_id = $1`, generated.ID) != 10 ||
			countRows(t, pool, `SELECT count(*) FROM practice_items pi JOIN answer_keys ak ON ak.question_version_id = pi.question_version_id WHERE pi.session_id = $1`, generated.ID) != 0 {
			t.Fatal("generated answers must remain private and separate from answer_keys")
		}
		pre := jsonRequest(t, data.learnerB, server.URL, http.MethodGet, "/api/v1/practice-sessions/"+generated.ID, nil, "")
		body := readResponse(t, pre)
		if pre.StatusCode != http.StatusOK || bytes.Contains(body, []byte("correctAnswer")) {
			t.Fatalf("generated pre-submit response leaked answer: %s", body)
		}
		var preGenerated preSessionResponse
		if err := json.Unmarshal(body, &preGenerated); err != nil || len(preGenerated.Items) != 10 {
			t.Fatalf("generated session should expose 10 answerable items: %s", body)
		}
		answers := make([]submittedAnswer, 0, len(preGenerated.Items))
		for _, item := range preGenerated.Items {
			answers = append(answers, submittedAnswer{ItemID: item.ID, Value: json.RawMessage(`{"optionIds":["a"]}`)})
		}
		var result resultResponse
		decodeResponse(t, rawRequest(t, data.learnerB, server.URL, http.MethodPost,
			"/api/v1/practice-sessions/"+generated.ID+"/submit", submitJSON(t, answers), map[string]string{"Idempotency-Key": "ai-generated-1"}), &result)
		if result.Summary.Confirmed.Total != 0 || result.Summary.AI.Pending != 10 || countRows(t, pool,
			`SELECT count(*) FROM jobs WHERE kind = 'analyze_practice_session_ai' AND payload->>'sessionId' = $1`, generated.ID) != 1 {
			t.Fatalf("generated batch should use one AI analysis and no confirmed score: %+v", result)
		}
		if err := aiService.Handlers()["analyze_practice_session_ai"](context.Background(), 1, 3, json.RawMessage(`{"sessionId":"`+generated.ID+`"}`)); err != nil {
			t.Fatalf("generated batch analysis failed: %v", err)
		}
		if _, err := pool.Exec(context.Background(),
			`UPDATE jobs SET status = 'succeeded' WHERE kind = 'analyze_practice_session_ai' AND payload->>'sessionId' = $1`, generated.ID); err != nil {
			t.Fatal(err)
		}
		decodeResponse(t, jsonRequest(t, data.learnerB, server.URL, http.MethodGet,
			"/api/v1/practice-sessions/"+generated.ID+"/result", nil, ""), &result)
		if result.Status != "completed" || result.Summary.Confirmed.Total != 0 || result.Summary.AI.Correct != 10 {
			t.Fatalf("AI-generated result should stay out of confirmed score: %+v", result)
		}
	})

	t.Run("重置密码与令牌消费原子完成并撤销旧会话", func(t *testing.T) {
		resetClient := newIntegrationClient(t)
		var reset struct {
			ResetToken string `json:"resetToken"`
		}
		decodeResponse(t, jsonRequest(t, resetClient, server.URL, http.MethodPost,
			"/api/v1/auth/password-reset/request", map[string]string{"email": "learner-a@example.com"}, ""), &reset)
		if reset.ResetToken == "" {
			t.Fatal("dev password reset should return a test token")
		}
		assertStatus(t, jsonRequest(t, resetClient, server.URL, http.MethodPost,
			"/api/v1/auth/password-reset/confirm", map[string]string{"token": reset.ResetToken, "password": "learner-new-pass-123"}, ""), http.StatusOK)
		assertStatus(t, jsonRequest(t, data.learnerA, server.URL, http.MethodGet, "/api/v1/me", nil, ""), http.StatusUnauthorized)
		loginIntegration(t, newIntegrationClient(t), server.URL, "learner-a@example.com", "learner-new-pass-123")
	})
}

func newIntegrationClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar}
}

func loginIntegration(t *testing.T, client *http.Client, baseURL, email, password string) {
	t.Helper()
	resp := jsonRequest(t, client, baseURL, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"email": email, "password": password}, "")
	assertStatus(t, resp, http.StatusOK)
}

func createIntegrationSession(t *testing.T, data integrationData, client *http.Client) preSessionResponse {
	t.Helper()
	resp := jsonRequest(t, client, data.server.URL, http.MethodPost, "/api/v1/practice-sessions", map[string]any{
		"levelId": data.levelID, "subjectId": data.subjectID, "mode": "comprehensive",
		"selectionOrder": "source_order", "count": 10,
	}, "")
	var out preSessionResponse
	decodeResponse(t, resp, &out)
	if len(out.Items) != 10 {
		t.Fatalf("expected 10 practice items, got %d", len(out.Items))
	}
	return out
}

func finalAnswers(t *testing.T, pool *pgxpool.Pool, sessionID string, data integrationData) []submittedAnswer {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT pi.id::text, pi.question_id::text, ak.value::text
		FROM practice_items pi
		JOIN question_versions v ON v.id = pi.question_version_id
		LEFT JOIN answer_keys ak ON ak.question_version_id = v.id
		WHERE pi.session_id = $1 ORDER BY pi.position`, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	answers := []submittedAnswer{}
	for rows.Next() {
		var itemID, questionID string
		var correctValue *string
		if err := rows.Scan(&itemID, &questionID, &correctValue); err != nil {
			t.Fatal(err)
		}
		value := json.RawMessage(`null`)
		if questionID == data.noAnswerID {
			value = json.RawMessage(`{"text":"わたしは勉強したいです。"}`)
		} else if correctValue != nil {
			value = json.RawMessage(*correctValue)
		}
		answers = append(answers, submittedAnswer{ItemID: itemID, Value: value})
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return answers
}

func submitJSON(t *testing.T, answers []submittedAnswer) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"answers": answers})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func answersForItem(answers []submittedAnswer, itemID string) submittedAnswer {
	for _, answer := range answers {
		if answer.ItemID == itemID {
			return answer
		}
	}
	return submittedAnswer{}
}

func itemIDForQuestion(t *testing.T, pool *pgxpool.Pool, sessionID, questionID string) string {
	t.Helper()
	var itemID string
	if err := pool.QueryRow(context.Background(), `SELECT id::text FROM practice_items WHERE session_id = $1 AND question_id = $2`, sessionID, questionID).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	return itemID
}

func jsonRequest(t *testing.T, client *http.Client, baseURL, method, path string, body any, idemKey string) *http.Response {
	t.Helper()
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	return rawRequest(t, client, baseURL, method, path, raw, headerIf(idemKey))
}

func rawRequest(t *testing.T, client *http.Client, baseURL, method, path string, body []byte, headers map[string]string) *http.Response {
	t.Helper()
	resp, err := doRawRequest(client, baseURL, method, path, body, headers)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

type requestResult struct {
	resp *http.Response
	err  error
}

func doRawRequest(client *http.Client, baseURL, method, path string, body []byte, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(method, baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return client.Do(req)
}

func headerIf(idemKey string) map[string]string {
	if idemKey == "" {
		return nil
	}
	return map[string]string{"Idempotency-Key": idemKey}
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("expected HTTP %d, got %d: %s", want, resp.StatusCode, readResponse(t, resp))
	}
	_ = readResponse(t, resp)
}

func decodeResponse(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	body := readResponse(t, resp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("unexpected HTTP %d: %s", resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, dst); err != nil {
		t.Fatalf("decode response: %v: %s", err, body)
	}
}

func readResponse(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func countRows(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func dataUserID(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `SELECT id::text FROM users WHERE email = $1`, email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func resetIntegrationDatabase(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `TRUNCATE
		audit_logs, ai_runs, jobs, import_items, import_jobs, issue_reports, user_knowledge_stats,
		grading_results, user_answers, practice_items, practice_sessions, password_reset_tokens,
		auth_sessions, rate_limit_counters, answer_keys, question_version_knowledge_points,
		questions, question_versions, material_versions, materials, source_sections, sources,
		knowledge_points, subjects, exam_levels, exams, users RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatal(err)
	}
}

func seedIntegrationData(t *testing.T, pool *pgxpool.Pool) integrationData {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	data := integrationData{}
	if err := tx.QueryRow(ctx, `INSERT INTO exams (code, name) VALUES ('jlpt-integration', 'JLPT 集成测试') RETURNING id::text`).Scan(&data.adminID); err != nil {
		t.Fatal(err)
	}
	examID := data.adminID
	if err := tx.QueryRow(ctx, `INSERT INTO exam_levels (exam_id, code, name) VALUES ($1, 'n5', 'N5') RETURNING id::text`, examID).Scan(&data.levelID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO subjects (exam_id, code, name) VALUES ($1, 'grammar', '语法') RETURNING id::text`, examID).Scan(&data.subjectID); err != nil {
		t.Fatal(err)
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("learner-pass-123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	adminHash, err := bcrypt.GenerateFromPassword([]byte("admin-pass-123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO users (email, email_normalized, password_hash, role) VALUES ('admin@example.com', 'admin@example.com', $1, 'admin') RETURNING id::text`, string(adminHash)).Scan(&data.adminID); err != nil {
		t.Fatal(err)
	}
	for _, email := range []string{"learner-a@example.com", "learner-b@example.com"} {
		if _, err := tx.Exec(ctx, `INSERT INTO users (email, email_normalized, password_hash, role) VALUES ($1, $1, $2, 'learner')`, email, string(passwordHash)); err != nil {
			t.Fatal(err)
		}
	}

	var sourceID string
	if err := tx.QueryRow(ctx, `INSERT INTO sources (name, kind, license_note, created_by) VALUES ('集成测试来源', 'self_made', '测试内容', $1) RETURNING id::text`, data.adminID).Scan(&sourceID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO source_sections (source_id, name, sort_order) VALUES ($1, '第一章', 1) RETURNING id::text`, sourceID).Scan(&data.sectionID); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO knowledge_points (exam_id, level_id, subject_id, name, status) VALUES ($1, $2, $3, $4, 'published') RETURNING id::text`, examID, data.levelID, data.subjectID, fmt.Sprintf("测试知识点%d", i)).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			data.knowledgePoint1 = id
		} else {
			data.knowledgePoint2 = id
		}
	}

	var materialID, materialVersionID string
	if err := tx.QueryRow(ctx, `INSERT INTO materials (created_by) VALUES ($1) RETURNING id::text`, data.adminID).Scan(&materialID); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO material_versions (material_id, version_no, title, content, created_by) VALUES ($1, 1, '测试材料', 'これは共有材料です。', $2) RETURNING id::text`, materialID, data.adminID).Scan(&materialVersionID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE materials SET current_version_id = $2 WHERE id = $1`, materialID, materialVersionID); err != nil {
		t.Fatal(err)
	}

	options := []map[string]string{{"id": "a", "label": "A", "text": "甲"}, {"id": "b", "label": "B", "text": "乙"}}
	optionsJSON, _ := json.Marshal(options)
	for i := 0; i < 30; i++ {
		questionType := "single_choice"
		stem := fmt.Sprintf("第 %d 题旧题干。", i+1)
		var versionOptions any = optionsJSON
		if i == 1 {
			questionType, versionOptions, stem = "short_answer", nil, "请用日语写一句愿望。"
		}
		hasAnswer := i != 1
		var questionID, versionID string
		if err := tx.QueryRow(ctx, `INSERT INTO questions (status, has_answer, created_by) VALUES ('published', $1, $2) RETURNING id::text`, hasAnswer, data.adminID).Scan(&questionID); err != nil {
			t.Fatal(err)
		}
		var materialRef any
		if i == 0 {
			materialRef = materialVersionID
		}
		if err := tx.QueryRow(ctx, `INSERT INTO question_versions (question_id, version_no, type, stem, material_version_id, options, level_id, subject_id, source_section_id, difficulty, source_order, created_by)
			VALUES ($1, 1, $2, $3, $4, $5, $6, $7, $8, 3, $9, $10) RETURNING id::text`, questionID, questionType, stem, materialRef, versionOptions, data.levelID, data.subjectID, data.sectionID, i, data.adminID).Scan(&versionID); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE questions SET current_version_id = $2, published_version_id = $2, published_at = now() WHERE id = $1`, questionID, versionID); err != nil {
			t.Fatal(err)
		}
		kpID := data.knowledgePoint1
		if i == 1 {
			kpID = data.knowledgePoint2
		}
		if _, err := tx.Exec(ctx, `INSERT INTO question_version_knowledge_points (question_version_id, knowledge_point_id) VALUES ($1, $2)`, versionID, kpID); err != nil {
			t.Fatal(err)
		}
		if hasAnswer {
			answer := json.RawMessage(`{"optionIds":["a"]}`)
			if _, err := tx.Exec(ctx, `INSERT INTO answer_keys (question_version_id, value, authority, created_by) VALUES ($1, $2, 'official', $3)`, versionID, answer, data.adminID); err != nil {
				t.Fatal(err)
			}
		}
		if i == 0 {
			data.keyQuestionID = questionID
		}
		if i == 1 {
			data.noAnswerID = questionID
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return data
}
