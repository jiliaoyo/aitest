package imports

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/jobs"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

type Store struct{ db store.DBTx }

func NewStore(db store.DBTx) *Store { return &Store{db: db} }

type jobRow struct {
	ID            string
	FileName      string
	MimeType      string
	SizeBytes     int64
	Status        string
	StageError    string
	ExtractedText string
	ItemCount     int
	CreatedAt     string
	UpdatedAt     string
}

const jobColumns = `ij.id::text, ij.file_name, ij.mime_type, ij.size_bytes, ij.status,
 ij.stage_error, ij.extracted_text, (SELECT count(*) FROM import_items ii WHERE ii.import_job_id = ij.id),
 ij.created_at::text, ij.updated_at::text`

const jobListColumns = `ij.id::text, ij.file_name, ij.mime_type, ij.size_bytes, ij.status,
 ij.stage_error, ''::text, (SELECT count(*) FROM import_items ii WHERE ii.import_job_id = ij.id),
 ij.created_at::text, ij.updated_at::text`

func toJob(r jobRow) Job {
	return Job{ID: r.ID, FileName: r.FileName, MimeType: r.MimeType, SizeBytes: r.SizeBytes,
		Status: r.Status, StageError: r.StageError, ExtractedText: r.ExtractedText,
		ItemCount: r.ItemCount, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func (s *Store) JobByID(ctx context.Context, id string) (Job, error) {
	r, err := store.CollectOneRow[jobRow](ctx, s.db,
		`SELECT `+jobColumns+` FROM import_jobs ij WHERE ij.id = $1`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, httpapi.ErrNotFound
	}
	if err != nil {
		return Job{}, err
	}
	return toJob(r), nil
}

func (s *Store) ListJobs(ctx context.Context, limit int) ([]Job, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	rows, err := store.CollectRows[jobRow](ctx, s.db,
		`SELECT `+jobListColumns+` FROM import_jobs ij ORDER BY ij.created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	out := make([]Job, 0, len(rows))
	for _, r := range rows {
		out = append(out, toJob(r))
	}
	return out, nil
}

func (s *Store) InsertJob(ctx context.Context, tx pgx.Tx, adminID, fileName, storedPath, sha256, mimeType string, size int64) (string, error) {
	var id string
	err := tx.QueryRow(ctx,
		`INSERT INTO import_jobs (created_by, file_name, stored_path, file_sha256, mime_type, size_bytes)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id::text`,
		adminID, fileName, storedPath, sha256, mimeType, size).Scan(&id)
	return id, err
}

func (s *Store) EnqueueExtract(ctx context.Context, tx pgx.Tx, jobID string) error {
	return jobs.EnqueueTx(ctx, tx, "extract_import_file", map[string]string{"jobId": jobID})
}

type extractRow struct {
	StoredPath    string
	Status        string
	ExtractedText string
}

func (s *Store) ExtractSource(ctx context.Context, id string) (extractRow, error) {
	r, err := store.CollectOneRow[extractRow](ctx, s.db,
		`SELECT stored_path, status, extracted_text FROM import_jobs WHERE id = $1`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return extractRow{}, httpapi.ErrNotFound
	}
	return r, err
}

func (s *Store) SetExtracting(ctx context.Context, tx pgx.Tx, id string) error {
	_, err := tx.Exec(ctx,
		`UPDATE import_jobs
		 SET status = 'extracting', stage_error = '',
		     stage_times = stage_times || jsonb_build_object('extractingStartedAt', now()), updated_at = now()
		 WHERE id = $1`, id)
	return err
}

func (s *Store) SaveExtracted(ctx context.Context, tx pgx.Tx, id, text string) error {
	_, err := tx.Exec(ctx,
		`UPDATE import_jobs
		 SET extracted_text = $2, status = 'structuring', stage_error = '',
		     stage_times = stage_times || jsonb_build_object('extractingFinishedAt', now(), 'structuringStartedAt', now()),
		     updated_at = now()
		 WHERE id = $1`, id, text)
	return err
}

func (s *Store) EnqueueStructure(ctx context.Context, tx pgx.Tx, jobID string) error {
	return jobs.EnqueueTx(ctx, tx, "structure_import_content_ai", map[string]string{"jobId": jobID})
}

func (s *Store) StructureSource(ctx context.Context, id string) (string, string, error) {
	var status, text string
	err := s.db.QueryRow(ctx,
		`SELECT status, extracted_text FROM import_jobs WHERE id = $1`, id).Scan(&status, &text)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", httpapi.ErrNotFound
	}
	return status, text, err
}

func (s *Store) SetStructuring(ctx context.Context, tx pgx.Tx, id string) error {
	_, err := tx.Exec(ctx,
		`UPDATE import_jobs
		 SET status = 'structuring', stage_error = '',
		     stage_times = stage_times || jsonb_build_object('structuringStartedAt', now()), updated_at = now()
		 WHERE id = $1`, id)
	return err
}

func (s *Store) InsertItemsAndReady(ctx context.Context, tx pgx.Tx, jobID string, items []Item) error {
	if _, err := tx.Exec(ctx,
		`DELETE FROM import_items WHERE import_job_id = $1 AND published_question_id IS NULL`, jobID); err != nil {
		return err
	}
	for _, item := range items {
		draft, err := json.Marshal(item.Draft)
		if err != nil {
			return err
		}
		anomalies, err := json.Marshal(item.Anomalies)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO import_items (import_job_id, position, raw_excerpt, ai_draft, anomalies)
			 VALUES ($1, $2, $3, $4, $5)`, jobID, item.Position, item.RawExcerpt, draft, anomalies); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx,
		`UPDATE import_jobs
		 SET status = 'review_ready', stage_error = '',
		     stage_times = stage_times || jsonb_build_object('structuringFinishedAt', now(), 'reviewReadyAt', now()),
		     updated_at = now() WHERE id = $1`, jobID)
	return err
}

func (s *Store) MarkFailed(ctx context.Context, id, stage string, cause error) error {
	message := cause.Error()
	if len(message) > 500 {
		message = message[:500]
	}
	_, err := s.db.Exec(ctx,
		`UPDATE import_jobs SET status = 'failed', stage_error = $2,
		 stage_times = stage_times || jsonb_build_object($3::text, now()), updated_at = now() WHERE id = $1`,
		id, message, stage+"FailedAt")
	return err
}

type itemRow struct {
	ID                  string
	JobID               string
	Position            int
	RawExcerpt          string
	DraftJSON           *string
	AnomaliesJSON       string
	ReviewStatus        string
	PublishedQuestionID *string
	JobStatus           string
	CreatedAt           string
	UpdatedAt           string
}

func itemFromRow(r itemRow) (Item, error) {
	item := Item{ID: r.ID, JobID: r.JobID, Position: r.Position, RawExcerpt: r.RawExcerpt,
		Anomalies: []string{}, ReviewStatus: r.ReviewStatus, PublishedQuestionID: r.PublishedQuestionID,
		JobStatus: r.JobStatus, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
	if r.DraftJSON != nil && *r.DraftJSON != "null" {
		var draft Draft
		if err := json.Unmarshal([]byte(*r.DraftJSON), &draft); err != nil {
			return Item{}, err
		}
		item.Draft = &draft
	}
	if err := json.Unmarshal([]byte(r.AnomaliesJSON), &item.Anomalies); err != nil {
		return Item{}, err
	}
	return item, nil
}

const itemColumns = `ii.id::text, ii.import_job_id::text, ii.position, ii.raw_excerpt, ii.ai_draft::text,
 ii.anomalies::text, ii.review_status, ii.published_question_id::text, ij.status,
 ii.created_at::text, ii.updated_at::text`

func (s *Store) ItemsByJob(ctx context.Context, jobID string) ([]Item, error) {
	rows, err := store.CollectRows[itemRow](ctx, s.db,
		`SELECT `+itemColumns+` FROM import_items ii JOIN import_jobs ij ON ij.id = ii.import_job_id
		 WHERE ii.import_job_id = $1 ORDER BY ii.position`, jobID)
	if err != nil {
		return nil, err
	}
	out := make([]Item, 0, len(rows))
	for _, r := range rows {
		item, err := itemFromRow(r)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Store) ItemByID(ctx context.Context, id string) (Item, error) {
	r, err := store.CollectOneRow[itemRow](ctx, s.db,
		`SELECT `+itemColumns+` FROM import_items ii JOIN import_jobs ij ON ij.id = ii.import_job_id
		 WHERE ii.id = $1`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, httpapi.ErrNotFound
	}
	if err != nil {
		return Item{}, err
	}
	return itemFromRow(r)
}

func (s *Store) UpdateDraft(ctx context.Context, id string, draft Draft) error {
	b, err := json.Marshal(draft)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx,
		`UPDATE import_items SET ai_draft = $2, review_status = 'pending', updated_at = now()
		 WHERE id = $1 AND published_question_id IS NULL`, id, b)
	return err
}

func (s *Store) ApproveItem(ctx context.Context, id string) error {
	result, err := s.db.Exec(ctx,
		`UPDATE import_items SET review_status = 'approved', updated_at = now()
		 WHERE id = $1 AND published_question_id IS NULL`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return httpapi.ErrNotFound
	}
	return nil
}

func (s *Store) ItemForPublish(ctx context.Context, tx pgx.Tx, id string) (Item, error) {
	r, err := store.CollectOneRow[itemRow](ctx, tx,
		`SELECT `+itemColumns+` FROM import_items ii JOIN import_jobs ij ON ij.id = ii.import_job_id
		 WHERE ii.id = $1 FOR UPDATE`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, httpapi.ErrNotFound
	}
	if err != nil {
		return Item{}, err
	}
	return itemFromRow(r)
}

func (s *Store) SharedMaterialID(ctx context.Context, tx pgx.Tx, jobID, materialKey string) (*string, error) {
	if materialKey == "" {
		return nil, nil
	}
	var id string
	err := tx.QueryRow(ctx,
		`SELECT mv.material_id::text
		 FROM import_items ii
		 JOIN questions q ON q.id = ii.published_question_id
		 JOIN question_versions v ON v.id = q.current_version_id
		 JOIN material_versions mv ON mv.id = v.material_version_id
		 WHERE ii.import_job_id = $1 AND ii.published_question_id IS NOT NULL
		   AND ii.ai_draft->>'materialKey' = $2 LIMIT 1`, jobID, materialKey).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &id, err
}

func (s *Store) MarkPublished(ctx context.Context, tx pgx.Tx, itemID, questionID string) error {
	if _, err := tx.Exec(ctx,
		`UPDATE import_items SET published_question_id = $2, review_status = 'published', updated_at = now()
		 WHERE id = $1 AND published_question_id IS NULL`, itemID, questionID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx,
		`UPDATE import_jobs ij SET status = CASE WHEN NOT EXISTS (
			SELECT 1 FROM import_items ii WHERE ii.import_job_id = ij.id AND ii.review_status <> 'published'
		) THEN 'published' ELSE ij.status END, updated_at = now()
		 WHERE ij.id = (SELECT import_job_id FROM import_items WHERE id = $1)`, itemID)
	return err
}

func (s *Store) RecordAudit(ctx context.Context, tx pgx.Tx, actorID, action, objectType, objectID string, detail map[string]any) error {
	b, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO audit_logs (actor_user_id, action, object_type, object_id, detail)
		 VALUES ($1, $2, $3, $4, $5)`, actorID, action, objectType, objectID, b)
	return err
}

func (s *Store) RetryJob(ctx context.Context, tx pgx.Tx, id, status string) error {
	result, err := tx.Exec(ctx,
		`UPDATE import_jobs SET status = $2, stage_error = '', updated_at = now()
		 WHERE id = $1 AND status = 'failed'`, id, status)
	if err == nil && result.RowsAffected() == 0 {
		return httpapi.E(http.StatusConflict, "import_not_failed", "只有失败的导入任务可以重试")
	}
	return err
}
