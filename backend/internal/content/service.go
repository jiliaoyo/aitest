package content

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool      *pgxpool.Pool
	store     *Store
	catalogKP KPChecker
}

// KPChecker 由 catalog 模块提供知识点存在性校验，避免 content 依赖 catalog 的完整实现。
type KPChecker interface {
	KnowledgePointExists(ctx context.Context, id string) (bool, error)
}

func NewService(pool *pgxpool.Pool, catalogStore KPChecker) *Service {
	return &Service{pool: pool, store: NewStore(pool), catalogKP: catalogStore}
}

func (s *Service) Store() *Store { return s.store }

func (s *Service) Overview(ctx context.Context) (Overview, error) {
	return s.store.Overview(ctx)
}

func (s *Service) GetQuestion(ctx context.Context, id string) (QuestionAdmin, error) {
	return s.store.QuestionAdminByID(ctx, id)
}

func (s *Service) ListQuestions(ctx context.Context, f ListFilter) ([]QuestionAdmin, string, error) {
	if f.Limit < 1 || f.Limit > 100 {
		f.Limit = 20
	}
	return s.store.ListQuestionsAdmin(ctx, f)
}

// CreateQuestion 在一个事务里创建题目第一版（含材料、知识点与标准答案）。
func (s *Service) CreateQuestion(ctx context.Context, adminID string, in QuestionInput) (QuestionAdmin, error) {
	if fields := ValidateInput(in); len(fields) > 0 {
		return QuestionAdmin{}, httpapi.ValidationError(fields)
	}
	if err := s.checkReferences(ctx, in); err != nil {
		return QuestionAdmin{}, err
	}
	var out QuestionAdmin
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		st := s.store.With(tx)
		var materialVersionID *string
		if in.MaterialID == nil && in.MaterialContent != "" {
			_, vID, err := st.CreateMaterial(ctx, in.MaterialTitle, in.MaterialContent, adminID)
			if err != nil {
				return fmt.Errorf("创建材料失败: %w", err)
			}
			materialVersionID = &vID
		} else if in.MaterialID != nil && *in.MaterialID != "" {
			ok, err := st.MaterialExists(ctx, *in.MaterialID)
			if err != nil {
				return err
			}
			if !ok {
				return httpapi.ValidationError(map[string]string{"materialId": "材料不存在"})
			}
			mvID, err := st.materialLatestVersion(ctx, *in.MaterialID)
			if err != nil {
				return err
			}
			materialVersionID = &mvID
		}
		qid, err := st.CreateQuestion(ctx, in.Answer != nil, adminID)
		if err != nil {
			return err
		}
		vid, err := st.InsertVersion(ctx, tx, qid, 1, in, materialVersionID, adminID)
		if err != nil {
			return err
		}
		if err := st.SetCurrentVersion(ctx, tx, qid, vid, in.Answer != nil); err != nil {
			return err
		}
		if err := st.writeAudit(ctx, tx, adminID, "question_created", "question", qid, map[string]any{"version": vid}); err != nil {
			return err
		}
		out, err = s.store.QuestionAdminByID(ctx, qid)
		return err
	})
	return out, err
}

// UpdateQuestion 总是创建新版本；已发布题目在重新发布前继续对外提供旧版本。
func (s *Service) UpdateQuestion(ctx context.Context, adminID, questionID string, in QuestionInput) (QuestionAdmin, error) {
	if fields := ValidateInput(in); len(fields) > 0 {
		return QuestionAdmin{}, httpapi.ValidationError(fields)
	}
	if err := s.checkReferences(ctx, in); err != nil {
		return QuestionAdmin{}, err
	}
	var out QuestionAdmin
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		st := s.store.With(tx)
		current, err := s.store.QuestionAdminByID(ctx, questionID)
		if err != nil {
			return err
		}
		if current.Status == StatusRetired {
			return httpapi.E(http.StatusConflict, "question_retired", "题目已下架，不能编辑")
		}
		var materialVersionID *string
		if in.MaterialID != nil && *in.MaterialID != "" {
			if in.MaterialContent != "" {
				mvID, err := st.BumpMaterialVersion(ctx, *in.MaterialID, in.MaterialTitle, in.MaterialContent, adminID)
				if err != nil {
					return fmt.Errorf("更新材料失败: %w", err)
				}
				materialVersionID = &mvID
			} else {
				mvID, err := st.materialLatestVersion(ctx, *in.MaterialID)
				if err != nil {
					return err
				}
				materialVersionID = &mvID
			}
		} else if in.MaterialContent != "" {
			_, vID, err := st.CreateMaterial(ctx, in.MaterialTitle, in.MaterialContent, adminID)
			if err != nil {
				return fmt.Errorf("创建材料失败: %w", err)
			}
			materialVersionID = &vID
		} else if current.CurrentVersion.MaterialVersionID != nil {
			materialVersionID = current.CurrentVersion.MaterialVersionID
		}
		versionNo, err := st.NextVersionNo(ctx, tx, questionID)
		if err != nil {
			return err
		}
		vid, err := st.InsertVersion(ctx, tx, questionID, versionNo, in, materialVersionID, adminID)
		if err != nil {
			return err
		}
		if err := st.SetCurrentVersion(ctx, tx, questionID, vid, in.Answer != nil); err != nil {
			return err
		}
		if err := st.writeAudit(ctx, tx, adminID, "question_revised", "question", questionID, map[string]any{"version": vid}); err != nil {
			return err
		}
		out, err = s.store.QuestionAdminByID(ctx, questionID)
		return err
	})
	return out, err
}

func (s *Service) checkReferences(ctx context.Context, in QuestionInput) error {
	for _, kpID := range in.KnowledgePointIDs {
		ok, err := s.catalogKP.KnowledgePointExists(ctx, kpID)
		if err != nil {
			return err
		}
		if !ok {
			return httpapi.ValidationError(map[string]string{"knowledgePointIds": "知识点不存在: " + kpID})
		}
	}
	if in.SourceSectionID != nil && *in.SourceSectionID != "" {
		ok, err := s.store.SectionExists(ctx, *in.SourceSectionID)
		if err != nil {
			return err
		}
		if !ok {
			return httpapi.ValidationError(map[string]string{"sourceSectionId": "来源章节不存在"})
		}
	}
	return nil
}

// Publish 校验当前版本并切换 published_version_id；旧版本继续支撑历史练习。
func (s *Service) Publish(ctx context.Context, adminID, questionID string) (QuestionAdmin, error) {
	var out QuestionAdmin
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		current, err := s.store.QuestionAdminByID(ctx, questionID)
		if err != nil {
			return err
		}
		if current.Status == StatusRetired {
			return httpapi.E(http.StatusConflict, "question_retired", "题目已下架，不能发布")
		}
		v := current.CurrentVersion
		if v == nil {
			return httpapi.E(http.StatusConflict, "question_invalid", "题目没有可发布版本")
		}
		var opts []Option
		if len(v.Options) > 0 {
			if err := json.Unmarshal(v.Options, &opts); err != nil {
				return httpapi.E(http.StatusConflict, "question_invalid", "选项数据不合法，不能发布")
			}
		}
		if fields := ValidateInput(QuestionInput{
			Type: v.Type, Stem: v.Stem, Options: opts,
			LevelID: v.LevelID, SubjectID: v.SubjectID,
		}); len(fields) > 0 {
			return httpapi.E(http.StatusConflict, "question_invalid", "题目字段不完整，不能发布")
		}
		st := s.store.With(tx)
		if current.PublishedVersionID != nil && *current.PublishedVersionID == v.ID {
			return httpapi.E(http.StatusConflict, "already_published", "当前版本已经发布")
		}
		if err := st.Publish(ctx, tx, questionID, v.ID); err != nil {
			return err
		}
		if err := st.writeAudit(ctx, tx, adminID, "question_published", "question", questionID, map[string]any{"version": v.ID}); err != nil {
			return err
		}
		out, err = s.store.QuestionAdminByID(ctx, questionID)
		return err
	})
	return out, err
}

func (s *Service) Retire(ctx context.Context, adminID, questionID string) (QuestionAdmin, error) {
	var out QuestionAdmin
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		current, err := s.store.QuestionAdminByID(ctx, questionID)
		if err != nil {
			return err
		}
		if current.RetiredAt != nil {
			return httpapi.E(http.StatusConflict, "question_retired", "题目已处于下架状态")
		}
		st := s.store.With(tx)
		if err := st.Retire(ctx, tx, questionID); err != nil {
			return err
		}
		if err := st.writeAudit(ctx, tx, adminID, "question_retired", "question", questionID, nil); err != nil {
			return err
		}
		out, err = s.store.QuestionAdminByID(ctx, questionID)
		return err
	})
	return out, err
}

func (s *Service) SubmitReview(ctx context.Context, adminID, questionID string) (QuestionAdmin, error) {
	var out QuestionAdmin
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		current, err := s.store.QuestionAdminByID(ctx, questionID)
		if err != nil {
			return err
		}
		if current.Status != StatusDraft {
			return httpapi.E(http.StatusConflict, "invalid_status", "只有草稿状态可以提交审核")
		}
		st := s.store.With(tx)
		if err := st.SetStatus(ctx, tx, questionID, StatusInReview); err != nil {
			return err
		}
		if err := st.writeAudit(ctx, tx, adminID, "question_submitted_review", "question", questionID, nil); err != nil {
			return err
		}
		out, err = s.store.QuestionAdminByID(ctx, questionID)
		return err
	})
	return out, err
}

// ---------- 来源管理 ----------

func (s *Service) ListSources(ctx context.Context) ([]Source, error) {
	return s.store.ListSources(ctx)
}

func (s *Service) CreateSource(ctx context.Context, adminID string, src *Source) error {
	if src.Name == "" {
		return httpapi.ValidationError(map[string]string{"name": "请填写来源名称"})
	}
	if !(src.Kind == "book" || src.Kind == "past_exam" || src.Kind == "self_made" || src.Kind == "ai_generated") {
		return httpapi.ValidationError(map[string]string{"kind": "来源类型不合法"})
	}
	if src.LicenseNote == "" {
		return httpapi.ValidationError(map[string]string{"licenseNote": "必须记录授权或使用依据"})
	}
	return s.store.CreateSource(ctx, src, adminID)
}

func (s *Service) UpdateSource(ctx context.Context, id string, fields map[string]any) error {
	return s.store.UpdateSource(ctx, id, fields)
}

func (s *Service) CreateSection(ctx context.Context, sourceID, name string) (SourceSection, error) {
	if name == "" {
		return SourceSection{}, httpapi.ValidationError(map[string]string{"name": "请填写章节名称"})
	}
	return s.store.CreateSection(ctx, sourceID, name)
}
