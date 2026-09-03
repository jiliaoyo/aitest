package imports

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/aishuati/backend/internal/ai"
	"github.com/aishuati/backend/internal/content"
	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/jobs"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pdf "github.com/ledongthuc/pdf"
)

const (
	structurePromptVersion = "import_structure.v1"
	maxExtractedBytes      = 1 << 20
	maxZipUncompressed     = 50 << 20
)

//go:embed prompts/import_structure.v1.md
var structurePrompt string

type Service struct {
	pool      *pgxpool.Pool
	store     *Store
	content   *content.Service
	client    *ai.Client
	uploadDir string
	maxBytes  int64
	logger    *slog.Logger
}

func NewService(pool *pgxpool.Pool, contentService *content.Service, client *ai.Client, uploadDir string, maxBytes int64, logger *slog.Logger) *Service {
	return &Service{pool: pool, store: NewStore(pool), content: contentService, client: client,
		uploadDir: uploadDir, maxBytes: maxBytes, logger: logger}
}

func (s *Service) Handlers() map[string]jobs.Handler {
	return map[string]jobs.Handler{
		"extract_import_file":         s.handleExtract,
		"structure_import_content_ai": s.handleStructure,
	}
}

// Upload 保存原文件后再在同一数据库事务中创建任务；数据库失败时清理刚写入的文件。
func (s *Service) Upload(ctx context.Context, adminID string, file multipart.File, header *multipart.FileHeader) (Job, error) {
	name := filepath.Base(header.Filename)
	if name == "." || name == "" || len([]rune(name)) > 255 {
		return Job{}, httpapi.ValidationError(map[string]string{"file": "文件名不合法"})
	}
	ext := strings.ToLower(filepath.Ext(name))
	if !allowedExt(ext) {
		return Job{}, httpapi.ValidationError(map[string]string{"file": "仅支持 TXT、Markdown、CSV、DOCX 和可提取文字的 PDF"})
	}
	storedPath, size, digest, mimeType, err := s.saveUpload(file, ext)
	if err != nil {
		return Job{}, err
	}
	var jobID string
	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		jobID, err = s.store.InsertJob(ctx, tx, adminID, name, storedPath, digest, mimeType, size)
		if err != nil {
			return err
		}
		return s.store.EnqueueExtract(ctx, tx, jobID)
	})
	if err != nil {
		_ = os.Remove(storedPath)
		return Job{}, fmt.Errorf("创建导入任务失败: %w", err)
	}
	return s.store.JobByID(ctx, jobID)
}

func (s *Service) saveUpload(file multipart.File, ext string) (string, int64, string, string, error) {
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
	mimeType, err := detectFileType(tmpPath, ext)
	if err != nil {
		return "", 0, "", "", err
	}
	var randomName [16]byte
	if _, err := rand.Read(randomName[:]); err != nil {
		return "", 0, "", "", fmt.Errorf("生成文件名失败: %w", err)
	}
	finalPath := filepath.Join(dir, hex.EncodeToString(randomName[:])+ext)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return "", 0, "", "", fmt.Errorf("保存上传文件失败: %w", err)
	}
	return finalPath, n, hex.EncodeToString(digest.Sum(nil)), mimeType, nil
}

func detectFileType(filePath, ext string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("读取上传文件失败: %w", err)
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	detected := http.DetectContentType(buf[:n])
	valid := strings.HasPrefix(detected, "text/") || detected == "application/octet-stream"
	switch ext {
	case ".pdf":
		valid = detected == "application/pdf"
	case ".docx":
		valid = detected == "application/zip" || detected == "application/octet-stream"
	}
	if !valid {
		return "", httpapi.ValidationError(map[string]string{"file": "文件类型与扩展名不匹配"})
	}
	if detected == "application/octet-stream" {
		detected = mimeForExt(ext)
	}
	return detected, nil
}

func mimeForExt(ext string) string {
	switch ext {
	case ".md":
		return "text/markdown"
	case ".csv":
		return "text/csv"
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	default:
		return "text/plain"
	}
}

func allowedExt(ext string) bool {
	switch ext {
	case ".txt", ".md", ".csv", ".docx", ".pdf":
		return true
	default:
		return false
	}
}

func (s *Service) handleExtract(ctx context.Context, attempts, maxAttempts int, payload json.RawMessage) error {
	var req struct {
		JobID string `json:"jobId"`
	}
	if err := strictDecode(payload, &req); err != nil {
		return err
	}
	job, err := s.store.ExtractSource(ctx, req.JobID)
	if err != nil {
		return err
	}
	if job.Status == StatusStructuring || job.Status == StatusReviewReady || job.Status == StatusPublished {
		return nil
	}
	if err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error { return s.store.SetExtracting(ctx, tx, req.JobID) }); err != nil {
		return err
	}
	text, err := extractText(job.StoredPath)
	if err != nil {
		return s.failStage(ctx, attempts, maxAttempts, req.JobID, "extracting", err)
	}
	if err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.store.SaveExtracted(ctx, tx, req.JobID, text); err != nil {
			return err
		}
		return s.store.EnqueueStructure(ctx, tx, req.JobID)
	}); err != nil {
		return s.failStage(ctx, attempts, maxAttempts, req.JobID, "extracting", err)
	}
	return nil
}

func (s *Service) handleStructure(ctx context.Context, attempts, maxAttempts int, payload json.RawMessage) error {
	var req struct {
		JobID string `json:"jobId"`
	}
	if err := strictDecode(payload, &req); err != nil {
		return err
	}
	status, text, err := s.store.StructureSource(ctx, req.JobID)
	if err != nil {
		return err
	}
	if status == StatusReviewReady || status == StatusPublished {
		return nil
	}
	if status != StatusStructuring {
		return s.failStage(ctx, attempts, maxAttempts, req.JobID, "structuring", errors.New("导入任务尚未完成文本提取"))
	}
	if err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error { return s.store.SetStructuring(ctx, tx, req.JobID) }); err != nil {
		return err
	}
	catalogData, err := s.loadCatalog(ctx)
	if err != nil {
		return s.failStage(ctx, attempts, maxAttempts, req.JobID, "structuring", err)
	}
	input, _ := json.Marshal(map[string]any{"catalog": catalogData, "text": text})
	out, err := s.client.RunPrompt(ctx, "import_structure", structurePromptVersion, req.JobID, structurePrompt, string(input))
	if err != nil {
		return s.failStage(ctx, attempts, maxAttempts, req.JobID, "structuring", err)
	}
	var response struct {
		Items []aiDraft `json:"items"`
	}
	if err := strictDecode(out, &response); err != nil {
		return s.failStage(ctx, attempts, maxAttempts, req.JobID, "structuring", err)
	}
	if len(response.Items) == 0 || len(response.Items) > 500 {
		return s.failStage(ctx, attempts, maxAttempts, req.JobID, "structuring", errors.New("AI 没有返回有效题目"))
	}
	items := make([]Item, 0, len(response.Items))
	for i, raw := range response.Items {
		draft := raw.toDraft()
		if err := validateDraft(&draft); err != nil {
			return s.failStage(ctx, attempts, maxAttempts, req.JobID, "structuring", fmt.Errorf("第 %d 题：%w", i+1, err))
		}
		if err := s.validateReferences(ctx, draft); err != nil {
			return s.failStage(ctx, attempts, maxAttempts, req.JobID, "structuring", fmt.Errorf("第 %d 题：%w", i+1, err))
		}
		rawExcerpt := strings.TrimSpace(raw.RawExcerpt)
		if rawExcerpt == "" || len([]rune(rawExcerpt)) > 5000 {
			return s.failStage(ctx, attempts, maxAttempts, req.JobID, "structuring", errors.New("原文定位片段缺失或过长"))
		}
		anomalies := raw.Anomalies
		if anomalies == nil {
			anomalies = []string{}
		}
		items = append(items, Item{Position: i + 1, RawExcerpt: rawExcerpt, Draft: &draft, Anomalies: anomalies})
	}
	if err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		return s.store.InsertItemsAndReady(ctx, tx, req.JobID, items)
	}); err != nil {
		return s.failStage(ctx, attempts, maxAttempts, req.JobID, "structuring", err)
	}
	s.logger.Info("import_structure_done", "job_id", req.JobID, "items", len(items))
	return nil
}

type aiDraft struct {
	RawExcerpt        string               `json:"rawExcerpt"`
	MaterialKey       string               `json:"materialKey"`
	Type              string               `json:"type"`
	Stem              string               `json:"stem"`
	Options           []content.Option     `json:"options"`
	MaterialTitle     string               `json:"materialTitle"`
	MaterialContent   string               `json:"materialContent"`
	LevelID           string               `json:"levelId"`
	SubjectID         string               `json:"subjectId"`
	SourceSectionID   *string              `json:"sourceSectionId"`
	Difficulty        int                  `json:"difficulty"`
	KnowledgePointIDs []string             `json:"knowledgePointIds"`
	SourceAnswer      *content.AnswerInput `json:"sourceAnswer"`
	AISuggestedAnswer *AnswerSuggestion    `json:"aiSuggestedAnswer"`
	Anomalies         []string             `json:"anomalies"`
}

func (d aiDraft) toDraft() Draft {
	return Draft{MaterialKey: d.MaterialKey, Type: d.Type, Stem: d.Stem, Options: d.Options,
		MaterialTitle: d.MaterialTitle, MaterialContent: d.MaterialContent, LevelID: d.LevelID,
		SubjectID: d.SubjectID, SourceSectionID: d.SourceSectionID, Difficulty: d.Difficulty,
		KnowledgePointIDs: d.KnowledgePointIDs, Answer: d.SourceAnswer, SourceAnswer: d.SourceAnswer,
		AISuggestedAnswer: d.AISuggestedAnswer}
}

func validateDraft(d *Draft) error {
	if d.Difficulty == 0 {
		d.Difficulty = 3
	}
	if d.Difficulty < 1 || d.Difficulty > 5 {
		return httpapi.ValidationError(map[string]string{"difficulty": "难度必须是 1 到 5"})
	}
	if d.Options == nil {
		d.Options = []content.Option{}
	}
	if d.KnowledgePointIDs == nil {
		d.KnowledgePointIDs = []string{}
	}
	if fields := content.ValidateInput(content.QuestionInput{
		Type: d.Type, Stem: d.Stem, Options: d.Options, LevelID: d.LevelID, SubjectID: d.SubjectID,
		SourceSectionID: d.SourceSectionID, Difficulty: d.Difficulty, KnowledgePointIDs: d.KnowledgePointIDs,
		Answer: d.Answer,
	}); len(fields) > 0 {
		return httpapi.ValidationError(fields)
	}
	if d.SourceAnswer != nil {
		if d.SourceAnswer.Authority != content.AuthorityOfficial {
			return httpapi.ValidationError(map[string]string{"sourceAnswer": "来源答案必须标记为 official"})
		}
		if err := content.ValidateAnswerValue(d.Type, d.Options, d.SourceAnswer.Value); err != nil {
			return httpapi.ValidationError(map[string]string{"sourceAnswer": err.Error()})
		}
	}
	if d.AISuggestedAnswer != nil {
		if err := content.ValidateAnswerValue(d.Type, d.Options, d.AISuggestedAnswer.Value); err != nil {
			return httpapi.ValidationError(map[string]string{"aiSuggestedAnswer": err.Error()})
		}
	}
	return nil
}

type catalogContext struct {
	Levels          []catalogChoice `json:"levels"`
	Subjects        []catalogChoice `json:"subjects"`
	KnowledgePoints []catalogChoice `json:"knowledgePoints"`
}

type catalogChoice struct {
	ID        string `json:"id"`
	Code      string `json:"code,omitempty"`
	Name      string `json:"name"`
	LevelID   string `json:"levelId,omitempty"`
	SubjectID string `json:"subjectId,omitempty"`
}

func (s *Service) loadCatalog(ctx context.Context) (catalogContext, error) {
	levels, err := store.CollectRows[catalogChoice](ctx, s.pool,
		`SELECT id::text, code, name FROM exam_levels ORDER BY sort_order`)
	if err != nil {
		return catalogContext{}, err
	}
	subjects, err := store.CollectRows[catalogChoice](ctx, s.pool,
		`SELECT id::text, code, name FROM subjects ORDER BY sort_order`)
	if err != nil {
		return catalogContext{}, err
	}
	kps, err := store.CollectRows[catalogChoice](ctx, s.pool,
		`SELECT id::text, '', name, level_id::text, subject_id::text
		 FROM knowledge_points WHERE status <> 'retired' ORDER BY created_at`)
	if err != nil {
		return catalogContext{}, err
	}
	return catalogContext{Levels: levels, Subjects: subjects, KnowledgePoints: kps}, nil
}

func (s *Service) validateReferences(ctx context.Context, d Draft) error {
	var sameScope bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM exam_levels l JOIN subjects sub ON sub.exam_id = l.exam_id WHERE l.id = $1 AND sub.id = $2)`,
		d.LevelID, d.SubjectID).Scan(&sameScope); err != nil {
		return err
	}
	if !sameScope {
		return httpapi.ValidationError(map[string]string{"levelId": "级别与科目不存在或不属于同一考试"})
	}
	var kpCount int
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM knowledge_points WHERE id = ANY($1::uuid[])`, d.KnowledgePointIDs).Scan(&kpCount); err != nil {
		return err
	}
	if kpCount != len(d.KnowledgePointIDs) {
		return httpapi.ValidationError(map[string]string{"knowledgePointIds": "包含不存在的知识点"})
	}
	if d.SourceSectionID != nil && *d.SourceSectionID != "" {
		var ok bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM source_sections WHERE id = $1)`, *d.SourceSectionID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return httpapi.ValidationError(map[string]string{"sourceSectionId": "来源章节不存在"})
		}
	}
	return nil
}

func (s *Service) failStage(ctx context.Context, attempts, maxAttempts int, jobID, stage string, cause error) error {
	if attempts >= maxAttempts {
		if err := s.store.MarkFailed(ctx, jobID, stage, cause); err != nil {
			return fmt.Errorf("记录导入失败状态失败: %w", err)
		}
	}
	return cause
}

func (s *Service) Retry(ctx context.Context, jobID string) (Job, error) {
	job, err := s.store.JobByID(ctx, jobID)
	if err != nil {
		return Job{}, err
	}
	if job.Status != StatusFailed {
		return Job{}, httpapi.E(http.StatusConflict, "import_not_failed", "只有失败的导入任务可以重试")
	}
	status := StatusUploaded
	kind := "extract_import_file"
	if job.ExtractedText != "" {
		status = StatusStructuring
		kind = "structure_import_content_ai"
	}
	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.store.RetryJob(ctx, tx, jobID, status); err != nil {
			return err
		}
		return jobs.EnqueueTx(ctx, tx, kind, map[string]string{"jobId": jobID})
	})
	if err != nil {
		return Job{}, err
	}
	return s.store.JobByID(ctx, jobID)
}

func (s *Service) ListJobs(ctx context.Context, limit int) ([]Job, error) {
	return s.store.ListJobs(ctx, limit)
}

func (s *Service) GetJob(ctx context.Context, id string) (Job, []Item, error) {
	job, err := s.store.JobByID(ctx, id)
	if err != nil {
		return Job{}, nil, err
	}
	items, err := s.store.ItemsByJob(ctx, id)
	return job, items, err
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

func extractText(filePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".txt", ".md", ".csv":
		f, err := os.Open(filePath)
		if err != nil {
			return "", err
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, maxExtractedBytes+1))
		if err != nil {
			return "", err
		}
		if len(data) > maxExtractedBytes {
			return "", errors.New("提取文本超过 1 MB 限制")
		}
		if !utf8.Valid(data) {
			return "", errors.New("文本不是有效 UTF-8，无法可靠提取")
		}
		return cleanText(string(data))
	case ".docx":
		return extractDocx(filePath)
	case ".pdf":
		return extractPDF(filePath)
	default:
		return "", errors.New("不支持的文件格式")
	}
}

func cleanText(text string) (string, error) {
	text = strings.TrimPrefix(text, "\ufeff")
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("文件中没有可提取文字")
	}
	if len([]rune(text)) > maxExtractedBytes {
		return "", errors.New("提取文本超过 1 MB 限制")
	}
	return text, nil
}

func extractDocx(filePath string) (string, error) {
	archive, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("DOCX 不是合法压缩包: %w", err)
	}
	defer archive.Close()
	var total uint64
	var document *zip.File
	for _, f := range archive.File {
		cleanName := path.Clean(f.Name)
		if cleanName == ".." || strings.HasPrefix(cleanName, "../") || strings.HasPrefix(f.Name, "/") {
			return "", errors.New("DOCX 包含不安全路径")
		}
		total += f.UncompressedSize64
		if total > maxZipUncompressed {
			return "", errors.New("压缩包解压大小超过限制")
		}
		if f.Name == "word/document.xml" {
			document = f
		}
	}
	if document == nil {
		return "", errors.New("DOCX 缺少正文")
	}
	r, err := document.Open()
	if err != nil {
		return "", err
	}
	defer r.Close()
	dec := xml.NewDecoder(io.LimitReader(r, maxExtractedBytes+1))
	var out strings.Builder
	textDepth := 0
	for {
		token, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("解析 DOCX 正文失败: %w", err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				textDepth++
			}
		case xml.CharData:
			if textDepth > 0 {
				out.Write([]byte(t))
			}
		case xml.EndElement:
			if t.Name.Local == "t" && textDepth > 0 {
				textDepth--
			}
			if t.Name.Local == "p" {
				out.WriteByte('\n')
			}
		}
		if out.Len() > maxExtractedBytes {
			return "", errors.New("提取文本超过 1 MB 限制")
		}
	}
	return cleanText(out.String())
}

func extractPDF(filePath string) (string, error) {
	f, reader, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("PDF 不是可提取文字的文件: %w", err)
	}
	defer f.Close()
	textReader, err := reader.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("提取 PDF 文字失败: %w", err)
	}
	data, err := io.ReadAll(io.LimitReader(textReader, maxExtractedBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxExtractedBytes {
		return "", errors.New("提取文本超过 1 MB 限制")
	}
	return cleanText(string(data))
}

func strictDecode(data []byte, dst any) error {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("AI 输出 JSON 不合法: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("AI 输出包含多余 JSON 内容")
	}
	return nil
}
