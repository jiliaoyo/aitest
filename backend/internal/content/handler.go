package content

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/httpapi/ctxkeys"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// RegisterRoutes 挂载管理端内容路由（adminMux 已带管理员鉴权）。
func (h *Handler) RegisterRoutes(adminMux *http.ServeMux) {
	adminMux.HandleFunc("GET /api/v1/admin/overview", h.overview)

	adminMux.HandleFunc("GET /api/v1/admin/questions", h.listQuestions)
	adminMux.HandleFunc("POST /api/v1/admin/questions", h.createQuestion)
	adminMux.HandleFunc("GET /api/v1/admin/questions/{id}", h.getQuestion)
	adminMux.HandleFunc("PATCH /api/v1/admin/questions/{id}", h.updateQuestion)
	adminMux.HandleFunc("POST /api/v1/admin/questions/{id}/submit-review", h.submitReview)
	adminMux.HandleFunc("POST /api/v1/admin/questions/{id}/publish", h.publish)
	adminMux.HandleFunc("POST /api/v1/admin/questions/{id}/retire", h.retire)

	adminMux.HandleFunc("GET /api/v1/admin/sources", h.listSources)
	adminMux.HandleFunc("POST /api/v1/admin/sources", h.createSource)
	adminMux.HandleFunc("PATCH /api/v1/admin/sources/{id}", h.updateSource)
	adminMux.HandleFunc("POST /api/v1/admin/sources/{id}/sections", h.createSection)
}

func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	o, err := h.service.Overview(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, o)
}

func (h *Handler) listQuestions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit == 0 {
		limit = 20
	}
	items, cursor, err := h.service.ListQuestions(r.Context(), ListFilter{
		Status:    q.Get("status"),
		LevelID:   q.Get("levelId"),
		SubjectID: q.Get("subjectId"),
		Query:     q.Get("q"),
		HasAnswer: q.Get("hasAnswer"),
		Cursor:    q.Get("cursor"),
		Limit:     limit,
	})
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"questions": items, "nextCursor": cursor})
}

func (h *Handler) getQuestion(w http.ResponseWriter, r *http.Request) {
	q, err := h.service.GetQuestion(r.Context(), r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"question": q})
}

func (h *Handler) createQuestion(w http.ResponseWriter, r *http.Request) {
	var in QuestionInput
	if err := httpapi.DecodeJSON(w, r, &in); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	q, err := h.service.CreateQuestion(r.Context(), ctxkeys.UserID(r.Context()), in)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	h.logger.Info("admin_question_created", "admin_id", ctxkeys.UserID(r.Context()), "question_id", q.ID)
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"question": q})
}

func (h *Handler) updateQuestion(w http.ResponseWriter, r *http.Request) {
	var in QuestionInput
	if err := httpapi.DecodeJSON(w, r, &in); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	q, err := h.service.UpdateQuestion(r.Context(), ctxkeys.UserID(r.Context()), r.PathValue("id"), in)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"question": q})
}

func (h *Handler) submitReview(w http.ResponseWriter, r *http.Request) {
	q, err := h.service.SubmitReview(r.Context(), ctxkeys.UserID(r.Context()), r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"question": q})
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	q, err := h.service.Publish(r.Context(), ctxkeys.UserID(r.Context()), r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"question": q})
}

func (h *Handler) retire(w http.ResponseWriter, r *http.Request) {
	q, err := h.service.Retire(r.Context(), ctxkeys.UserID(r.Context()), r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"question": q})
}

func (h *Handler) listSources(w http.ResponseWriter, r *http.Request) {
	sources, err := h.service.ListSources(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"sources": sources})
}

func (h *Handler) createSource(w http.ResponseWriter, r *http.Request) {
	var src Source
	if err := httpapi.DecodeJSON(w, r, &src); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if err := h.service.CreateSource(r.Context(), ctxkeys.UserID(r.Context()), &src); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"source": src})
}

func (h *Handler) updateSource(w http.ResponseWriter, r *http.Request) {
	var fields map[string]any
	if err := httpapi.DecodeJSON(w, r, &fields); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	allowed := map[string]bool{"name": true, "kind": true, "author": true, "publisher": true, "year": true, "licenseNote": true, "internalNote": true}
	for k := range fields {
		if !allowed[k] {
			httpapi.WriteError(w, r, httpapi.E(http.StatusBadRequest, "bad_request", "包含不允许修改的字段: "+k))
			return
		}
	}
	if err := h.service.UpdateSource(r.Context(), r.PathValue("id"), fields); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) createSection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := httpapi.DecodeJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	sec, err := h.service.CreateSection(r.Context(), r.PathValue("id"), req.Name)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"section": sec})
}
