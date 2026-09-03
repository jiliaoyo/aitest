package learning

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/httpapi/ctxkeys"
	"github.com/aishuati/backend/internal/jobs"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	pool   *pgxpool.Pool
	store  *Store
	logger *slog.Logger
}

func NewHandler(pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{pool: pool, store: NewStore(pool), logger: logger}
}

// Handlers 返回统计重算任务处理器，供 worker 注册。
func (h *Handler) Handlers() map[string]jobs.Handler {
	return map[string]jobs.Handler{
		"rebuild_user_knowledge_stats": h.handleRebuildStats,
	}
}

func (h *Handler) handleRebuildStats(ctx context.Context, attempts, maxAttempts int, payload json.RawMessage) error {
	var req struct {
		UserID string `json:"userId"`
	}
	if err := json.Unmarshal(payload, &req); err != nil || req.UserID == "" {
		return fmt.Errorf("rebuild stats payload 不合法")
	}
	return h.store.RebuildUserStats(ctx, h.pool, req.UserID)
}

// RegisterRoutes 挂载学习端档案路由与举报管理路由；mux/adminMux 传 nil 表示不挂载。
func (h *Handler) RegisterRoutes(mux *http.ServeMux, adminMux *http.ServeMux) {
	if mux != nil {
		mux.HandleFunc("GET /api/v1/knowledge-points", h.listKnowledgePoints)
		mux.HandleFunc("GET /api/v1/knowledge-points/{id}", h.knowledgePointDetail)
		mux.HandleFunc("GET /api/v1/dashboard", h.dashboard)
		mux.HandleFunc("GET /api/v1/learning-memory", h.memory)
		mux.HandleFunc("DELETE /api/v1/learning-memory", h.deleteMemory)
		mux.HandleFunc("GET /api/v1/wrong-items", h.wrongItems)
		mux.HandleFunc("DELETE /api/v1/wrong-items/{id}", h.deleteWrongItem)
		mux.HandleFunc("POST /api/v1/issue-reports", h.createIssueReport)
	}
	if adminMux != nil {
		adminMux.HandleFunc("GET /api/v1/admin/issue-reports", h.listIssueReports)
		adminMux.HandleFunc("PATCH /api/v1/admin/issue-reports/{id}", h.resolveIssueReport)
	}
}

func (h *Handler) listKnowledgePoints(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, err := h.store.KnowledgePoints(r.Context(), ctxkeys.UserID(r.Context()),
		q.Get("levelId"), q.Get("subjectId"), q.Get("q"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"knowledgePoints": items})
}

func (h *Handler) knowledgePointDetail(w http.ResponseWriter, r *http.Request) {
	detail, err := h.store.KnowledgePointDetailForUser(r.Context(),
		ctxkeys.UserID(r.Context()), r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, detail)
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := ctxkeys.UserID(ctx)
	d := Dashboard{RecentSessions: []RecentSession{}, Recommendations: []Recommendation{}}

	active, err := h.store.ActiveSession(ctx, userID)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	d.ActiveSession = active

	recent, err := h.store.RecentSessions(ctx, userID, 5)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	d.RecentSessions = recent

	memory, err := h.store.MemoryForUser(ctx, userID)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	d.Memory = memory

	weak, err := h.store.WeakKnowledgePoints(ctx, userID, 3)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	// 推荐原因中的数字全部来自后端统计，不交给模型或前端计算
	for _, w := range weak {
		rec := Recommendation{
			Type: "knowledge", KnowledgePointID: &w.ID, Name: w.Name,
			RecentAnswered:    w.RecentAnswered,
			RecentWrongCount:  w.RecentAnswered - w.RecentCorrect,
			ConsecutiveWrong:  w.ConsecutiveWrong,
			SuggestedCount:    10,
			KnowledgePointIDs: []string{w.ID},
		}
		if w.RecentAnswered > 0 {
			acc := float64(w.RecentCorrect) / float64(w.RecentAnswered)
			rec.Accuracy = &acc
		}
		rec.Reason = fmt.Sprintf("最近 30 天该知识点已确认作答 %d 题、错了 %d 题，连续错误 %d 次，建议专项练习 10 题。",
			w.RecentAnswered, rec.RecentWrongCount, w.ConsecutiveWrong)
		d.Recommendations = append(d.Recommendations, rec)
	}
	if len(d.Recommendations) == 0 {
		d.StatsEmpty = true
		d.Comprehensive = &Recommendation{
			Type: "comprehensive", Name: "综合练习",
			SuggestedCount: 20,
			Reason:         "数据不足，暂时无法定位薄弱知识点，建议先做一组综合练习积累数据。",
		}
	}
	httpapi.WriteJSON(w, http.StatusOK, d)
}

func (h *Handler) memory(w http.ResponseWriter, r *http.Request) {
	memory, err := h.store.MemoryForUser(r.Context(), ctxkeys.UserID(r.Context()))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, memory)
}

func (h *Handler) deleteMemory(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteMemory(r.Context(), h.pool, ctxkeys.UserID(r.Context())); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) wrongItems(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.WrongItems(r.Context(), ctxkeys.UserID(r.Context()),
		r.URL.Query().Get("knowledgePointId"), 50)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	items := map[string]*wrongItemDTO{}
	order := []string{}
	for _, row := range rows {
		dto, ok := items[row.ItemID]
		if !ok {
			dto = &wrongItemDTO{
				ItemID: row.ItemID, SessionID: row.SessionID, QuestionID: row.QuestionID,
				Position: row.Position, Type: row.Type, Stem: row.Stem,
				Options: json.RawMessage("null"), GradingStatus: row.Status,
				KnowledgePoints: []kpRef{},
			}
			if row.OptionsText != nil && *row.OptionsText != "null" {
				dto.Options = json.RawMessage(*row.OptionsText)
			}
			if row.MaterialID != nil {
				dto.Material = &materialDTO{ID: *row.MaterialID}
				if row.MaterialTitle != nil {
					dto.Material.Title = *row.MaterialTitle
				}
				if row.MaterialContent != nil {
					dto.Material.Content = *row.MaterialContent
				}
			}
			if row.UserValue != nil {
				dto.UserAnswer = json.RawMessage(*row.UserValue)
			}
			if row.CorrectValue != nil {
				dto.CorrectAnswer = json.RawMessage(*row.CorrectValue)
			}
			if row.Authority != nil {
				dto.AnswerAuthority = row.Authority
			}
			if row.Explanation != nil && *row.Explanation != "" && row.ExplanationSource != nil {
				dto.Explanation = &explanationDTO{Text: *row.Explanation, Source: *row.ExplanationSource}
			}
			items[row.ItemID] = dto
			order = append(order, row.ItemID)
		}
		if row.KPID != nil {
			dto.KnowledgePoints = append(dto.KnowledgePoints, kpRef{ID: *row.KPID, Name: *row.KPName})
		}
	}
	out := make([]*wrongItemDTO, 0, len(order))
	for _, id := range order {
		out = append(out, items[id])
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"wrongItems": out})
}

func (h *Handler) deleteWrongItem(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteWrongItem(r.Context(), ctxkeys.UserID(r.Context()), r.PathValue("id")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) createIssueReport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PracticeItemID string `json:"practiceItemId"`
		TargetType     string `json:"targetType"`
		Description    string `json:"description"`
	}
	if err := httpapi.DecodeJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	valid := map[string]bool{"stem": true, "answer": true, "explanation": true, "classification": true, "ai_grading": true}
	fields := map[string]string{}
	if !valid[req.TargetType] {
		fields["targetType"] = "问题类型不合法"
	}
	if len(req.Description) > 2000 {
		fields["description"] = "说明过长"
	}
	if len(fields) > 0 {
		httpapi.WriteError(w, r, httpapi.ValidationError(fields))
		return
	}
	report, err := h.store.CreateIssueReport(r.Context(), h.pool,
		ctxkeys.UserID(r.Context()), req.PracticeItemID, req.TargetType, req.Description)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, report)
}

func (h *Handler) listIssueReports(w http.ResponseWriter, r *http.Request) {
	limit := 50
	reports, err := h.store.ListIssueReports(r.Context(), r.URL.Query().Get("status"), limit)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"issueReports": reports})
}

func (h *Handler) resolveIssueReport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status         string `json:"status"`
		ResolutionNote string `json:"resolutionNote"`
	}
	if err := httpapi.DecodeJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if err := h.store.ResolveIssueReport(r.Context(), h.pool,
		ctxkeys.UserID(r.Context()), r.PathValue("id"), req.Status, req.ResolutionNote); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type wrongItemDTO struct {
	ItemID          string          `json:"itemId"`
	SessionID       string          `json:"sessionId"`
	QuestionID      string          `json:"questionId"`
	Position        int             `json:"position"`
	Type            string          `json:"type"`
	Stem            string          `json:"stem"`
	Options         json.RawMessage `json:"options"`
	Material        *materialDTO    `json:"material,omitempty"`
	KnowledgePoints []kpRef         `json:"knowledgePoints"`
	GradingStatus   string          `json:"gradingStatus"`
	AnswerAuthority *string         `json:"answerAuthority,omitempty"`
	UserAnswer      json.RawMessage `json:"userAnswer"`
	CorrectAnswer   json.RawMessage `json:"correctAnswer"`
	Explanation     *explanationDTO `json:"explanation,omitempty"`
}

type materialDTO struct {
	ID      string `json:"id"`
	Title   string `json:"title,omitempty"`
	Content string `json:"content,omitempty"`
}

type kpRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type explanationDTO struct {
	Text   string `json:"text"`
	Source string `json:"source"`
}
