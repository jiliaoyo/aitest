package practice

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"

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

// RegisterRoutes 挂载学习端练习路由（mux 外层已带登录鉴权）。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/practice/availability", h.availability)
	mux.HandleFunc("GET /api/v1/practice/sources", h.sources)
	mux.HandleFunc("POST /api/v1/practice-sessions", h.create)
	mux.HandleFunc("GET /api/v1/practice-sessions", h.list)
	mux.HandleFunc("GET /api/v1/practice-sessions/{id}", h.get)
	mux.HandleFunc("PUT /api/v1/practice-sessions/{id}/answers/{itemId}", h.saveAnswer)
	mux.HandleFunc("POST /api/v1/practice-sessions/{id}/submit", h.submit)
	mux.HandleFunc("GET /api/v1/practice-sessions/{id}/result", h.result)
	mux.HandleFunc("POST /api/v1/practice-sessions/{id}/analysis/retry", h.retryAnalysis)
}

func (h *Handler) sources(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sources, err := h.service.PracticeSources(r.Context(), q.Get("levelId"), q.Get("subjectId"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"sources": sources})
}

func (h *Handler) availability(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := CreateRequest{
		LevelID:         q.Get("levelId"),
		SubjectID:       q.Get("subjectId"),
		Mode:            q.Get("mode"),
		SelectionOrder:  q.Get("selectionOrder"),
		SourceID:        q.Get("sourceId"),
		SourceSectionID: q.Get("sourceSectionId"),
		Count:           10,
	}
	req.KnowledgePointIDs = parseIDList(q.Get("knowledgePointIds"))
	n, err := h.service.Availability(r.Context(), ctxkeys.UserID(r.Context()), req)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]int{"available": n})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := httpapi.DecodeJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	session, err := h.service.CreateSession(r.Context(), ctxkeys.UserID(r.Context()), req)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, session)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := atoiDefault(q.Get("limit"), 20)
	items, cursor, err := h.service.ListSessions(r.Context(), ctxkeys.UserID(r.Context()), q.Get("status"), q.Get("cursor"), limit)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"sessions": items, "nextCursor": cursor})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	session, err := h.service.GetPreSubmit(r.Context(), ctxkeys.UserID(r.Context()), r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, session)
}

func (h *Handler) saveAnswer(w http.ResponseWriter, r *http.Request) {
	var req SaveAnswerRequest
	if err := httpapi.DecodeJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	savedAt, err := h.service.SaveAnswer(r.Context(), ctxkeys.UserID(r.Context()),
		r.PathValue("id"), r.PathValue("itemId"), req)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"savedAt": savedAt})
}

// submit 要求携带全部最终答案与 Idempotency-Key；请求体哈希用于幂等冲突检测。
func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	raw, err := httpapi.ReadBody(w, r)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	var req SubmitRequest
	if err := httpapi.DecodeRaw(raw, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	sum := sha256.Sum256(raw)
	bodyHash := hex.EncodeToString(sum[:])
	_, err = h.service.Submit(r.Context(), ctxkeys.UserID(r.Context()),
		r.PathValue("id"), r.Header.Get("Idempotency-Key"), bodyHash, req)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	result, err := h.service.GetResult(r.Context(), ctxkeys.UserID(r.Context()), r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) result(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.GetResult(r.Context(), ctxkeys.UserID(r.Context()), r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) retryAnalysis(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.RetryAnalysis(r.Context(), ctxkeys.UserID(r.Context()), r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, result)
}

func parseIDList(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	cur := ""
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		cur += string(s[i])
	}
	return out
}

func atoiDefault(s string, fallback int) int {
	n := 0
	if s == "" {
		return fallback
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return fallback
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return fallback
	}
	return n
}
