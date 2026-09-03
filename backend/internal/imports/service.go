package imports

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aishuati/backend/internal/content"
	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service 只接受本地 OCR 服务导出的结构化 JSON：同步校验后直接生成待审核草稿，
// 不做任何文件格式识别或 AI 结构化，因此没有异步阶段和失败重试。
type Service struct {
	pool      *pgxpool.Pool
	store     *Store
	content   *content.Service
	uploadDir string
	maxBytes  int64
	logger    *slog.Logger
}

func NewService(pool *pgxpool.Pool, contentService *content.Service, uploadDir string, maxBytes int64, logger *slog.Logger) *Service {
	return &Service{pool: pool, store: NewStore(pool), content: contentService,
		uploadDir: uploadDir, maxBytes: maxBytes, logger: logger}
}

// Upload 保存原文件并立即解析导入；数据库失败时清理刚写入的文件。
func (s *Service) Upload(ctx context.Context, adminID string, file multipart.File, header *multipart.FileHeader) (Job, error) {
	name := filepath.Base(header.Filename)
	if name == "." || name == "" || len([]rune(name)) > 255 {
		return Job{}, httpapi.ValidationError(map[string]string{"file": "文件名不合法"})
	}
	if strings.ToLower(filepath.Ext(name)) != ".json" {
		return Job{}, httpapi.ValidationError(map[string]string{"file": "仅支持本地 OCR 服务导出的结构化 JSON 文件"})
	}
	storedPath, size, digest, mimeType, err := s.saveUpload(file)
	if err != nil {
		return Job{}, err
	}
	job, err := s.importJSON(ctx, adminID, name, storedPath, digest, mimeType, size)
	if err != nil {
		_ = os.Remove(storedPath)
		return Job{}, err
	}
	return job, nil
}

func (s *Service) saveUpload(file multipart.File) (string, int64, string, string, error) {
	dir, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return "", 0, "", "", fmt.Errorf("解析上传目录失败: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", 0, "", "", fmt.Errorf("创建上传目录失败: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return "", 0, "", "", fmt.Errorf("创建上传临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	digest := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(tmp, digest), io.LimitReader(file, s.maxBytes+1))
	if closeErr := tmp.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return "", 0, "", "", fmt.Errorf("保存上传文件失败: %w", copyErr)
	}
	if n > s.maxBytes {
		return "", 0, "", "", httpapi.E(http.StatusRequestEntityTooLarge, "file_too_large", "文件超过大小限制")
	}
	if err := checkJSONLike(tmpPath); err != nil {
		return "", 0, "", "", err
	}
	var randomName [16]byte
	if _, err := rand.Read(randomName[:]); err != nil {
		return "", 0, "", "", fmt.Errorf("生成文件名失败: %w", err)
	}
	finalPath := filepath.Join(dir, hex.EncodeToString(randomName[:])+".json")
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return "", 0, "", "", fmt.Errorf("保存上传文件失败: %w", err)
	}
	return finalPath, n, hex.EncodeToString(digest.Sum(nil)), "application/json", nil
}

func checkJSONLike(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("读取上传文件失败: %w", err)
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	detected := http.DetectContentType(buf[:n])
	if !strings.HasPrefix(detected, "text/") && detected != "application/octet-stream" {
		return httpapi.ValidationError(map[string]string{"file": "文件类型与扩展名不匹配"})
	}
	return nil
}

func (s *Service) ListJobs(ctx context.Context, cursor string, limit int) ([]Job, string, error) {
	return s.store.ListJobs(ctx, cursor, limit)
}

func (s *Service) GetJob(ctx context.Context, id, cursor string, limit int) (Job, []Item, string, error) {
	job, err := s.store.JobByID(ctx, id)
	if err != nil {
		return Job{}, nil, "", err
	}
	items, nextCursor, err := s.store.ItemsByJob(ctx, id, cursor, limit)
	return job, items, nextCursor, err
}

func (s *Service) GetItem(ctx context.Context, id string) (Item, error) {
	return s.store.ItemByID(ctx, id)
}

func (s *Service) UpdateItem(ctx context.Context, id string, req UpdateItemRequest) (Item, error) {
	if err := validateDraft(&req.Draft); err != nil {
		return Item{}, err
	}
	if err := s.validateReferences(ctx, req.Draft); err != nil {
		return Item{}, err
	}
	if err := s.store.UpdateDraft(ctx, id, req.Draft); err != nil {
		return Item{}, err
	}
	return s.store.ItemByID(ctx, id)
}

func (s *Service) ApproveItem(ctx context.Context, id string) (Item, error) {
	item, err := s.store.ItemByID(ctx, id)
	if err != nil {
		return Item{}, err
	}
	if item.Draft == nil {
		return Item{}, httpapi.E(http.StatusConflict, "import_item_invalid", "导入项没有结构化内容")
	}
	if err := validateDraft(item.Draft); err != nil {
		return Item{}, err
	}
	if err := s.validateReferences(ctx, *item.Draft); err != nil {
		return Item{}, err
	}
	if err := s.store.ApproveItem(ctx, id); err != nil {
		return Item{}, err
	}
	return s.store.ItemByID(ctx, id)
}

func (s *Service) PublishItem(ctx context.Context, adminID, id string) (Item, error) {
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		item, err := s.store.ItemForPublish(ctx, tx, id)
		if err != nil {
			return err
		}
		if item.PublishedQuestionID != nil {
			return nil
		}
		if item.ReviewStatus != ReviewApproved {
			return httpapi.E(http.StatusConflict, "import_item_not_approved", "请先保存并审核导入项")
		}
		if item.Draft == nil {
			return httpapi.E(http.StatusConflict, "import_item_invalid", "导入项没有结构化内容")
		}
		if err := validateDraft(item.Draft); err != nil {
			return err
		}
		sharedMaterialID, err := s.store.SharedMaterialID(ctx, tx, item.JobID, item.Draft.MaterialKey)
		if err != nil {
			return err
		}
		in := questionInput(*item.Draft, sharedMaterialID)
		questionID, err := s.content.CreateAndPublishTx(ctx, tx, adminID, in)
		if err != nil {
			return err
		}
		if err := s.store.MarkPublished(ctx, tx, id, questionID); err != nil {
			return err
		}
		return s.store.RecordAudit(ctx, tx, adminID, "import_item_published", "import_item", id,
			map[string]any{"questionId": questionID})
	})
	if err != nil {
		return Item{}, err
	}
	return s.store.ItemByID(ctx, id)
}

func questionInput(d Draft, materialID *string) content.QuestionInput {
	materialContent := d.MaterialContent
	if materialID != nil {
		materialContent = ""
	}
	return content.QuestionInput{Type: d.Type, Stem: d.Stem, Options: d.Options, MaterialID: materialID,
		MaterialTitle: d.MaterialTitle, MaterialContent: materialContent, LevelID: d.LevelID,
		SubjectID: d.SubjectID, SourceSectionID: d.SourceSectionID, Difficulty: d.Difficulty,
		KnowledgePointIDs: d.KnowledgePointIDs, Answer: d.Answer}
}
