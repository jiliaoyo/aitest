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
	"os"
	"reflect"
	"testing"
	"time"

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

	t.Run("导入任务重试不重复发布", func(t *testing.T) {
		var jobID, itemID string
		if err := pool.QueryRow(context.Background(),
			`INSERT INTO import_jobs (created_by, file_name, status) VALUES ($1, 'retry.txt', 'failed') RETURNING id::text`, data.adminID).Scan(&jobID); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(context.Background(),
			`INSERT INTO import_items (import_job_id, position, raw_excerpt, review_status, published_question_id)
			 VALUES ($1, 1, '已有发布题目', 'published', $2) RETURNING id::text`, jobID, data.keyQuestionID).Scan(&itemID); err != nil {
			t.Fatal(err)
		}
		path := "/api/v1/admin/import-jobs/" + jobID + "/retry"
		assertStatus(t, jsonRequest(t, data.admin, server.URL, http.MethodPost, path, nil, ""), http.StatusOK)
		resp := jsonRequest(t, data.admin, server.URL, http.MethodPost, path, nil, "")
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("second import retry should conflict, got %d", resp.StatusCode)
		}
		if countRows(t, pool, `SELECT count(*) FROM import_items WHERE import_job_id = $1`, jobID) != 1 ||
			countRows(t, pool, `SELECT count(*) FROM import_items WHERE id = $1 AND published_question_id = $2`, itemID, data.keyQuestionID) != 1 ||
			countRows(t, pool, `SELECT count(*) FROM jobs WHERE payload->>'jobId' = $1`, jobID) != 1 {
			t.Fatal("import retry duplicated an item, published question, or job")
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
