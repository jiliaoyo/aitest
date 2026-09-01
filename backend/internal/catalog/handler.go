package catalog

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/httpapi/ctxkeys"
	"github.com/jackc/pgx/v5"
)

type Handler struct {
	store  *Store
	logger *slog.Logger
}

func NewHandler(store *Store, logger *slog.Logger) *Handler {
	return &Handler{store: store, logger: logger}
}

// RegisterRoutes 挂载 GET /catalog 与管理端知识点路由；mux/adminMux 传 nil 表示不挂载。
func (h *Handler) RegisterRoutes(mux *http.ServeMux, adminMux *http.ServeMux) {
	if mux != nil {
		mux.HandleFunc("GET /api/v1/catalog", h.catalog)
	}
	if adminMux != nil {
		adminMux.HandleFunc("GET /api/v1/admin/knowledge-points", h.adminListKnowledgePoints)
		adminMux.HandleFunc("POST /api/v1/admin/knowledge-points", h.adminCreateKnowledgePoint)
		adminMux.HandleFunc("PATCH /api/v1/admin/knowledge-points/{id}", h.adminUpdateKnowledgePoint)
	}
}

func (h *Handler) catalog(w http.ResponseWriter, r *http.Request) {
	exams, err := h.store.Catalog(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"exams": exams})
}

func (h *Handler) adminListKnowledgePoints(w http.ResponseWriter, r *http.Request) {
	levelID := r.URL.Query().Get("levelId")
	kps, err := h.store.ListKnowledgePointsAdmin(r.Context(), levelID)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"knowledgePoints": kps})
}

func (h *Handler) adminCreateKnowledgePoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LevelID        string  `json:"levelId"`
		SubjectID      string  `json:"subjectId"`
		ParentID       *string `json:"parentId"`
		Name           string  `json:"name"`
		Description    string  `json:"description"`
		CommonMistakes string  `json:"commonMistakes"`
		Examples       string  `json:"examples"`
	}
	if err := httpapi.DecodeJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	fields := map[string]string{}
	if req.Name == "" {
		fields["name"] = "请填写知识点名称"
	}
	if req.SubjectID == "" {
		fields["subjectId"] = "请选择科目"
	}
	if len(fields) > 0 {
		httpapi.WriteError(w, r, httpapi.ValidationError(fields))
		return
	}
	levelOK, err := h.store.LevelExists(r.Context(), req.LevelID)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if !levelOK {
		httpapi.WriteError(w, r, httpapi.ValidationError(map[string]string{"levelId": "级别不存在"}))
		return
	}
	subjectOK, err := h.store.SubjectExists(r.Context(), req.SubjectID)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if !subjectOK {
		httpapi.WriteError(w, r, httpapi.ValidationError(map[string]string{"subjectId": "科目不存在"}))
		return
	}
	var subjectExamID, levelExamID string
	if err := h.store.db.QueryRow(r.Context(),
		`SELECT exam_id::text FROM subjects WHERE id = $1`, req.SubjectID).Scan(&subjectExamID); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if err := h.store.db.QueryRow(r.Context(),
		`SELECT exam_id::text FROM exam_levels WHERE id = $1`, req.LevelID).Scan(&levelExamID); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if subjectExamID != levelExamID {
		httpapi.WriteError(w, r, httpapi.ValidationError(map[string]string{"subjectId": "科目与级别必须属于同一考试"}))
		return
	}
	examID := levelExamID
	if req.ParentID != nil && *req.ParentID != "" {
		pLevel, pSubject, err := h.store.ParentScope(r.Context(), *req.ParentID)
		if errors.Is(err, pgx.ErrNoRows) {
			httpapi.WriteError(w, r, httpapi.ValidationError(map[string]string{"parentId": "父知识点不存在"}))
			return
		}
		if err != nil {
			httpapi.WriteError(w, r, err)
			return
		}
		if pLevel != req.LevelID || pSubject != req.SubjectID {
			httpapi.WriteError(w, r, httpapi.ValidationError(map[string]string{"parentId": "父知识点必须属于同一级别和科目"}))
			return
		}
	}
	kp := KnowledgePoint{
		ExamID: examID, LevelID: req.LevelID, SubjectID: req.SubjectID, ParentID: req.ParentID,
		Name: req.Name, Description: req.Description,
		CommonMistakes: req.CommonMistakes, Examples: req.Examples,
		Status: "draft",
	}
	if err := h.store.CreateKnowledgePoint(r.Context(), &kp); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	h.logger.Info("admin_kp_created", "admin_id", ctxkeys.UserID(r.Context()), "kp_id", kp.ID)
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"knowledgePoint": kp})
}

func (h *Handler) adminUpdateKnowledgePoint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var fields map[string]any
	if err := httpapi.DecodeJSON(w, r, &fields); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	allowed := map[string]bool{"name": true, "description": true, "commonMistakes": true, "examples": true, "status": true, "parentId": true}
	for k := range fields {
		if !allowed[k] {
			httpapi.WriteError(w, r, httpapi.E(http.StatusBadRequest, "bad_request", "包含不允许修改的字段: "+k))
			return
		}
	}
	if s, ok := fields["status"].(string); ok && s != "draft" && s != "published" && s != "retired" {
		httpapi.WriteError(w, r, httpapi.ValidationError(map[string]string{"status": "状态不合法"}))
		return
	}
	kp, err := h.store.UpdateKnowledgePoint(r.Context(), id, fields)
	if errors.Is(err, pgx.ErrNoRows) {
		httpapi.WriteError(w, r, httpapi.ErrNotFound)
		return
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	h.logger.Info("admin_kp_updated", "admin_id", ctxkeys.UserID(r.Context()), "kp_id", id)
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"knowledgePoint": kp})
}
