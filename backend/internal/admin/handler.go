package admin

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/aishuati/backend/internal/httpapi"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) RegisterRoutes(adminMux *http.ServeMux) {
	adminMux.HandleFunc("GET /api/v1/admin/users", h.listUsers)
	adminMux.HandleFunc("GET /api/v1/admin/users/{id}", h.userDetail)
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	dateRange, err := ParseDateRange(q.Get("from"), q.Get("to"))
	if err != nil {
		httpapi.WriteError(w, r, httpapi.ValidationError(map[string]string{"date": err.Error()}))
		return
	}
	limit := 20
	if raw := q.Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			httpapi.WriteError(w, r, httpapi.ValidationError(map[string]string{"limit": "limit 必须是数字"}))
			return
		}
	}
	page, err := h.service.ListUsers(r.Context(), UserListFilter{
		Query: q.Get("q"), Role: q.Get("role"), Cursor: q.Get("cursor"), Limit: limit, DateRange: dateRange,
	})
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	h.logger.Debug("admin_users_listed", "count", len(page.Users))
	httpapi.WriteJSON(w, http.StatusOK, page)
}

func (h *Handler) userDetail(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	dateRange, err := ParseDateRange(q.Get("from"), q.Get("to"))
	if err != nil {
		httpapi.WriteError(w, r, httpapi.ValidationError(map[string]string{"date": err.Error()}))
		return
	}
	detail, err := h.service.UserDetail(r.Context(), r.PathValue("id"), dateRange)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, detail)
}
