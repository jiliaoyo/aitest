package imports

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

func (h *Handler) RegisterRoutes(adminMux *http.ServeMux) {
	adminMux.HandleFunc("POST /api/v1/admin/import-jobs", h.createJob)
	adminMux.HandleFunc("GET /api/v1/admin/import-jobs", h.listJobs)
	adminMux.HandleFunc("GET /api/v1/admin/import-jobs/{id}", h.getJob)
	adminMux.HandleFunc("GET /api/v1/admin/import-items/{id}", h.getItem)
	adminMux.HandleFunc("PATCH /api/v1/admin/import-items/{id}", h.updateItem)
	adminMux.HandleFunc("POST /api/v1/admin/import-items/{id}/approve", h.approveItem)
	adminMux.HandleFunc("POST /api/v1/admin/import-items/{id}/publish", h.publishItem)
}

func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.service.maxBytes+(1<<20))
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		httpapi.WriteError(w, r, httpapi.E(http.StatusBadRequest, "bad_multipart", "文件上传格式不正确"))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpapi.WriteError(w, r, httpapi.ValidationError(map[string]string{"file": "请选择要导入的文件"}))
		return
	}
	defer file.Close()
	job, err := h.service.Upload(r.Context(), ctxkeys.UserID(r.Context()), file, header)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	h.logger.Info("import_job_created", "admin_id", ctxkeys.UserID(r.Context()), "job_id", job.ID)
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"job": job})
}

func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	jobs, nextCursor, err := h.service.ListJobs(r.Context(), q.Get("cursor"), limit)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"jobs": jobs, "nextCursor": nextCursor})
}

func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	job, items, nextCursor, err := h.service.GetJob(r.Context(), r.PathValue("id"), q.Get("cursor"), limit)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"job": job, "items": items, "nextCursor": nextCursor})
}

func (h *Handler) getItem(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.GetItem(r.Context(), r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (h *Handler) updateItem(w http.ResponseWriter, r *http.Request) {
	var req UpdateItemRequest
	if err := httpapi.DecodeJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	item, err := h.service.UpdateItem(r.Context(), r.PathValue("id"), req)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (h *Handler) approveItem(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.ApproveItem(r.Context(), r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (h *Handler) publishItem(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.PublishItem(r.Context(), ctxkeys.UserID(r.Context()), r.PathValue("id"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"item": item})
}
