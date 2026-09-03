package imports

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/aishuati/backend/internal/httpapi"
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

func (s *Store) ListJobs(ctx context.Context, cursor string, limit int) ([]Job, string, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	args := []any{}
	where := "true"
	if cursor != "" {
		parts := strings.Split(cursor, "\x00")
		if len(parts) != 2 {
			return nil, "", httpapi.ValidationError(map[string]string{"cursor": "游标无效"})
		}
		args = append(args, parts[0], parts[1])
		where = "(ij.created_at, ij.id) < ($1::timestamptz, $2::uuid)"
	}
	args = append(args, limit)
	rows, err := store.CollectRows[jobRow](ctx, s.db,
		`SELECT `+jobListColumns+` FROM import_jobs ij WHERE `+where+`
		 ORDER BY ij.created_at DESC, ij.id DESC LIMIT $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, "", err
	}
	out := make([]Job, 0, len(rows))
	for _, r := range rows {
		out = append(out, toJob(r))
	}
	next := ""
	if len(rows) == limit {
		r := rows[len(rows)-1]
		next = r.CreatedAt + "\x00" + r.ID
	}
	return out, next, nil
}

func (s *Store) InsertJob(ctx context.Context, tx pgx.Tx, adminID, fileName, storedPath, sha256, mimeType string, size int64) (string, error) {
	var id string
	err := tx.QueryRow(ctx,
		`INSERT INTO import_jobs (created_by, file_name, stored_path, file_sha256, mime_type, size_bytes)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id::text`,
		adminID, fileName, storedPath, sha256, mimeType, size).Scan(&id)
	return id, err
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

func (s *Store) ItemsByJob(ctx context.Context, jobID, cursor string, limit int) ([]Item, string, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	args := []any{jobID}
	where := "ii.import_job_id = $1"
	if cursor != "" {
		if _, err := strconv.Atoi(cursor); err != nil {
			return nil, "", httpapi.ValidationError(map[string]string{"cursor": "游标无效"})
		}
		args = append(args, cursor)
		where += " AND ii.position > $" + strconv.Itoa(len(args))
	}
	args = append(args, limit)
	rows, err := store.CollectRows[itemRow](ctx, s.db,
		`SELECT `+itemColumns+` FROM import_items ii JOIN import_jobs ij ON ij.id = ii.import_job_id
		 WHERE `+where+` ORDER BY ii.position LIMIT $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, "", err
	}
	out := make([]Item, 0, len(rows))
	for _, r := range rows {
		item, err := itemFromRow(r)
		if err != nil {
			return nil, "", err
		}
		out = append(out, item)
	}
	next := ""
	if len(rows) == limit {
		next = strconv.Itoa(rows[len(rows)-1].Position)
	}
	return out, next, nil
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
