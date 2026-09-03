package content

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

type Store struct{ db store.DBTx }

func NewStore(db store.DBTx) *Store { return &Store{db: db} }

// With 返回绑定到给定事务/连接的视图；service 在事务内必须使用它执行写操作。
func (s *Store) With(db store.DBTx) *Store { return &Store{db: db} }

// CountPublishedVersions 统计满足筛选的已发布题目数量，用于练习可用量。
func (s *Store) CountPublishedVersions(ctx context.Context, f SelectionFilter) (int, error) {
	f.Limit = 0
	kpAny := f.KnowledgePointIDs
	if kpAny == nil {
		kpAny = []string{}
	}
	qIDs := f.QuestionIDs
	if qIDs == nil {
		qIDs = []string{}
	}
	var n int
	err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM questions q
		 JOIN question_versions v ON v.id = q.published_version_id
		 LEFT JOIN source_sections ss ON ss.id = v.source_section_id
		 WHERE q.retired_at IS NULL
		   AND v.level_id::text = $1
		   AND ($2 = '' OR v.subject_id::text = $2)
		   AND ($3 = '' OR v.source_section_id::text = $3)
		   AND ($4 = '' OR ss.source_id::text = $4)
		   AND ($5::uuid[] = '{}' OR q.id = ANY($5::uuid[]))
		   AND ($6::uuid[] = '{}' OR EXISTS (
		     SELECT 1 FROM question_version_knowledge_points qvkp
		     WHERE qvkp.question_version_id = v.id AND qvkp.knowledge_point_id = ANY($6::uuid[])))`,
		f.LevelID, f.SubjectID, f.SourceSectionID, f.SourceID, qIDs, kpAny).Scan(&n)
	return n, err
}

// ---------- 来源与章节 ----------

type sourceRow struct {
	ID           string
	Name         string
	Kind         string
	Author       string
	Publisher    string
	Year         *int
	LicenseNote  string
	InternalNote string
}

func (s *Store) ListSources(ctx context.Context) ([]Source, error) {
	rows, err := store.CollectRows[sourceRow](ctx, s.db,
		`SELECT id, name, kind, author, publisher, year, license_note, internal_note FROM sources ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	out := make([]Source, 0, len(rows))
	for _, r := range rows {
		out = append(out, Source{
			ID: r.ID, Name: r.Name, Kind: r.Kind, Author: r.Author, Publisher: r.Publisher,
			Year: r.Year, LicenseNote: r.LicenseNote, InternalNote: r.InternalNote,
			Sections: []SourceSection{},
		})
	}
	sections, err := store.CollectRows[SourceSection](ctx, s.db,
		`SELECT id, source_id, name, sort_order FROM source_sections ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	idx := map[string]int{}
	for i := range out {
		idx[out[i].ID] = i
	}
	for _, sec := range sections {
		if i, ok := idx[sec.SourceID]; ok {
			out[i].Sections = append(out[i].Sections, sec)
		}
	}
	return out, nil
}

func (s *Store) ListPracticeSources(ctx context.Context, levelID, subjectID string) ([]PracticeSource, error) {
	rows, err := store.CollectRows[struct {
		SourceID      string
		SourceName    string
		SectionID     string
		SectionName   string
		QuestionCount int
	}](ctx, s.db,
		`SELECT src.id::text, src.name, ss.id::text, ss.name, count(q.id)::int
		 FROM source_sections ss
		 JOIN sources src ON src.id = ss.source_id
		 JOIN question_versions v ON v.source_section_id = ss.id
		   AND v.level_id::text = $1
		   AND ($2 = '' OR v.subject_id::text = $2)
		 JOIN questions q ON q.published_version_id = v.id AND q.retired_at IS NULL
		 GROUP BY src.id, src.name, src.created_at, src.kind, ss.id, ss.name, ss.sort_order
		 ORDER BY CASE WHEN src.kind = 'book' THEN 0 ELSE 1 END, src.created_at, src.id, ss.sort_order`, levelID, subjectID)
	if err != nil {
		return nil, err
	}
	out := make([]PracticeSource, 0, len(rows))
	byID := map[string]int{}
	for _, r := range rows {
		i, ok := byID[r.SourceID]
		if !ok {
			byID[r.SourceID] = len(out)
			out = append(out, PracticeSource{ID: r.SourceID, Name: r.SourceName, Sections: []PracticeSourceSection{}})
			i = len(out) - 1
		}
		out[i].QuestionCount += r.QuestionCount
		out[i].Sections = append(out[i].Sections, PracticeSourceSection{ID: r.SectionID, Name: r.SectionName, QuestionCount: r.QuestionCount})
	}
	return out, nil
}

func (s *Store) CreateSource(ctx context.Context, src *Source, createdBy string) error {
	return s.db.QueryRow(ctx,
		`INSERT INTO sources (name, kind, author, publisher, year, license_note, internal_note, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		src.Name, src.Kind, src.Author, src.Publisher, src.Year, src.LicenseNote, src.InternalNote, createdBy,
	).Scan(&src.ID)
}

func (s *Store) UpdateSource(ctx context.Context, id string, fields map[string]any) error {
	set, args, err := store.BuildUpdate(map[string]string{
		"name": "name", "kind": "kind", "author": "author", "publisher": "publisher",
		"year": "year", "licenseNote": "license_note", "internalNote": "internal_note",
	}, fields)
	if err != nil || set == "" {
		if err != nil {
			return err
		}
		return nil
	}
	args = append(args, id)
	_, err = s.db.Exec(ctx,
		`UPDATE sources SET `+set+`, updated_at = now() WHERE id = $`+store.Itoa(len(args)), args...)
	return err
}

func (s *Store) CreateSection(ctx context.Context, sourceID, name string) (SourceSection, error) {
	var sec SourceSection
	err := s.db.QueryRow(ctx,
		`INSERT INTO source_sections (source_id, name, sort_order)
		 SELECT $1, $2, coalesce(max(sort_order), 0) + 1 FROM source_sections WHERE source_id = $1
		 RETURNING id, source_id::text, name, sort_order`,
		sourceID, name,
	).Scan(&sec.ID, &sec.SourceID, &sec.Name, &sec.SortOrder)
	return sec, err
}

func (s *Store) SourceExists(ctx context.Context, id string) (bool, error) {
	return store.Exists(ctx, s.db, `SELECT true FROM sources WHERE id = $1`, id)
}

func (s *Store) SectionExists(ctx context.Context, id string) (bool, error) {
	return store.Exists(ctx, s.db, `SELECT true FROM source_sections WHERE id = $1`, id)
}

// ---------- 材料与题目版本 ----------

func (s *Store) CreateMaterial(ctx context.Context, title, content, createdBy string) (materialID, versionID string, err error) {
	// 注意：不能用一条 data-modifying CTE 同时插入并回填 current_version_id——
	// 同一命令内的 CTE 与主语句共享快照，看不到彼此的插入。拆成三条语句。
	err = s.db.QueryRow(ctx,
		`INSERT INTO materials (created_by) VALUES ($1) RETURNING id::text`, createdBy,
	).Scan(&materialID)
	if err != nil {
		return "", "", err
	}
	err = s.db.QueryRow(ctx,
		`INSERT INTO material_versions (material_id, version_no, title, content, created_by)
		 VALUES ($1, 1, $2, $3, $4) RETURNING id::text`,
		materialID, title, content, createdBy,
	).Scan(&versionID)
	if err != nil {
		return "", "", err
	}
	_, err = s.db.Exec(ctx,
		`UPDATE materials SET current_version_id = $2 WHERE id = $1`, materialID, versionID)
	return materialID, versionID, err
}

// BumpMaterialVersion 为已有材料创建新版本并返回新版本 ID。
func (s *Store) BumpMaterialVersion(ctx context.Context, materialID, title, content, createdBy string) (versionID string, err error) {
	err = s.db.QueryRow(ctx,
		`WITH v AS (
		   INSERT INTO material_versions (material_id, version_no, title, content, created_by)
		   SELECT $1, coalesce(max(version_no), 0) + 1, $2, $3, $4 FROM material_versions WHERE material_id = $1
		   RETURNING id
		 )
		 UPDATE materials SET current_version_id = v.id FROM v WHERE materials.id = $1
		 RETURNING v.id::text`,
		materialID, title, content, createdBy,
	).Scan(&versionID)
	return
}

func (s *Store) MaterialExists(ctx context.Context, id string) (bool, error) {
	return store.Exists(ctx, s.db, `SELECT true FROM materials WHERE id = $1`, id)
}

func (s *Store) materialLatestVersion(ctx context.Context, materialID string) (string, error) {
	var vid string
	err := s.db.QueryRow(ctx,
		`SELECT id::text FROM material_versions WHERE material_id = $1 ORDER BY version_no DESC LIMIT 1`,
		materialID).Scan(&vid)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", httpapi.ValidationError(map[string]string{"materialId": "材料没有可用版本"})
	}
	return vid, err
}

func (s *Store) writeAudit(ctx context.Context, tx pgx.Tx, actorID, action, objectType, objectID string, detail map[string]any) error {
	var payload any
	if detail != nil {
		data, err := json.Marshal(detail)
		if err != nil {
			return err
		}
		payload = data
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO audit_logs (actor_user_id, action, object_type, object_id, detail)
		 VALUES ($1, $2, $3, $4, $5)`,
		actorID, action, objectType, objectID, payload)
	return err
}

func (s *Store) CreateQuestion(ctx context.Context, hasAnswer bool, createdBy string) (string, error) {
	var qid string
	err := s.db.QueryRow(ctx,
		`INSERT INTO questions (status, has_answer, created_by)
		 VALUES ($1, $2, $3) RETURNING id`,
		StatusDraft, hasAnswer, createdBy,
	).Scan(&qid)
	return qid, err
}

// InsertVersion 写入一个新的题目版本并返回其 ID；调用方负责事务与 questions 指针更新。
func (s *Store) InsertVersion(ctx context.Context, tx store.DBTx, questionID string, versionNo int, in QuestionInput, materialVersionID *string, createdBy string) (string, error) {
	var opts any
	if len(in.Options) > 0 {
		data, err := json.Marshal(in.Options)
		if err != nil {
			return "", err
		}
		opts = data
	}
	var vid string
	if err := tx.QueryRow(ctx,
		`INSERT INTO question_versions
		   (question_id, version_no, type, stem, material_version_id, options, level_id, subject_id, source_section_id, difficulty, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id::text`,
		questionID, versionNo, in.Type, in.Stem, materialVersionID, opts,
		in.LevelID, in.SubjectID, in.SourceSectionID, in.Difficulty, createdBy,
	).Scan(&vid); err != nil {
		return "", err
	}
	if len(in.KnowledgePointIDs) > 0 {
		if _, err := tx.Exec(ctx,
			`INSERT INTO question_version_knowledge_points (question_version_id, knowledge_point_id)
			 SELECT $1, x FROM unnest($2::uuid[]) AS x`,
			vid, in.KnowledgePointIDs); err != nil {
			return "", err
		}
	}
	if in.Answer != nil {
		if _, err := tx.Exec(ctx,
			`INSERT INTO answer_keys (question_version_id, value, authority, explanation, created_by)
			 VALUES ($1, $2, $3, $4, $5)`,
			vid, in.Answer.Value, in.Answer.Authority, in.Answer.Explanation, createdBy); err != nil {
			return "", err
		}
	}
	return vid, nil
}

func (s *Store) NextVersionNo(ctx context.Context, tx store.DBTx, questionID string) (int, error) {
	var n int
	err := tx.QueryRow(ctx,
		`SELECT coalesce(max(version_no), 0) + 1 FROM question_versions WHERE question_id = $1`, questionID,
	).Scan(&n)
	return n, err
}

func (s *Store) SetCurrentVersion(ctx context.Context, tx store.DBTx, questionID, versionID string, hasAnswer bool) error {
	_, err := tx.Exec(ctx,
		`UPDATE questions SET current_version_id = $2, has_answer = $3, updated_at = now() WHERE id = $1`,
		questionID, versionID, hasAnswer)
	return err
}

func (s *Store) SetStatus(ctx context.Context, tx store.DBTx, questionID, status string) error {
	_, err := tx.Exec(ctx,
		`UPDATE questions SET status = $2, updated_at = now() WHERE id = $1`, questionID, status)
	return err
}

func (s *Store) Publish(ctx context.Context, tx store.DBTx, questionID, versionID string) error {
	_, err := tx.Exec(ctx,
		`UPDATE questions
		 SET published_version_id = $2, status = $3, published_at = now(), retired_at = NULL, updated_at = now()
		 WHERE id = $1`, questionID, versionID, StatusPublished)
	return err
}

func (s *Store) Retire(ctx context.Context, tx store.DBTx, questionID string) error {
	_, err := tx.Exec(ctx,
		`UPDATE questions SET status = $2, retired_at = now(), updated_at = now() WHERE id = $1`,
		questionID, StatusRetired)
	return err
}

// ---------- 管理端查询 ----------

func (s *Store) QuestionAdminByID(ctx context.Context, id string) (QuestionAdmin, error) {
	var q QuestionAdmin
	err := s.db.QueryRow(ctx,
		`SELECT id, status, has_answer, published_version_id::text, published_at::text, retired_at::text, updated_at::text
		 FROM questions WHERE id = $1`, id,
	).Scan(&q.ID, &q.Status, &q.HasAnswer, &q.PublishedVersionID, &q.PublishedAt, &q.RetiredAt, &q.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return QuestionAdmin{}, httpapi.ErrNotFound
	}
	if err != nil {
		return QuestionAdmin{}, err
	}
	v, err := s.currentVersion(ctx, id)
	if err != nil {
		return QuestionAdmin{}, err
	}
	q.CurrentVersion = &v
	return q, nil
}

func (s *Store) currentVersion(ctx context.Context, questionID string) (QuestionVersion, error) {
	const q = `SELECT v.id::text, v.question_id::text, v.version_no, v.type, v.stem,
	    v.material_version_id::text,
	    coalesce(mv.title, ''),
	    coalesce(mv.content, ''),
	    coalesce(v.options::text, 'null'),
	    v.level_id::text, v.subject_id::text, v.source_section_id::text, v.difficulty,
	    coalesce(ak.value::text, 'null'), coalesce(ak.authority, ''), coalesce(ak.explanation, ''),
	    v.created_at::text
	  FROM question_versions v
	  LEFT JOIN material_versions mv ON mv.id = v.material_version_id
	  LEFT JOIN answer_keys ak ON ak.question_version_id = v.id
	  WHERE v.id = (SELECT current_version_id FROM questions WHERE id = $1)`

	var v QuestionVersion
	var answerValue, answerAuthority, answerExplanation string
	err := s.db.QueryRow(ctx, q, questionID).Scan(
		&v.ID, &v.QuestionID, &v.VersionNo, &v.Type, &v.Stem,
		&v.MaterialVersionID, &v.MaterialTitle, &v.MaterialContent,
		&v.Options, &v.LevelID, &v.SubjectID, &v.SourceSectionID, &v.Difficulty,
		&answerValue, &answerAuthority, &answerExplanation, &v.CreatedAt,
	)
	if err != nil {
		return v, err
	}
	if answerAuthority != "" {
		v.AnswerKey = &AnswerKey{Value: json.RawMessage(answerValue), Authority: answerAuthority, Explanation: answerExplanation}
	}
	kpRows, err := store.CollectRows[struct{ ID string }](ctx, s.db,
		`SELECT knowledge_point_id::text FROM question_version_knowledge_points WHERE question_version_id = $1 ORDER BY knowledge_point_id`, v.ID)
	if err != nil {
		return v, err
	}
	kps := make([]string, 0, len(kpRows))
	for _, row := range kpRows {
		kps = append(kps, row.ID)
	}
	v.KnowledgePointIDs = kps
	return v, nil
}

type questionListRow struct {
	ID        string
	Status    string
	HasAnswer bool
	Type      string
	Stem      string
	LevelID   string
	SubjectID string
	VersionNo int
	UpdatedAt string
}

func (s *Store) ListQuestionsAdmin(ctx context.Context, f ListFilter) ([]QuestionAdmin, string, error) {
	args := []any{}
	conds := []string{"q.current_version_id IS NOT NULL"}
	add := func(cond string, vals ...any) {
		for _, v := range vals {
			args = append(args, v)
			cond = replaceOnce(cond, "?", "$"+store.Itoa(len(args)))
		}
		conds = append(conds, cond)
	}
	if f.Status != "" {
		add("q.status = ?", f.Status)
	}
	if f.LevelID != "" {
		add("v.level_id::text = ?", f.LevelID)
	}
	if f.SubjectID != "" {
		add("v.subject_id::text = ?", f.SubjectID)
	}
	if f.Query != "" {
		add("v.stem ILIKE '%' || ? || '%'", f.Query)
	}
	if f.HasAnswer == "yes" {
		conds = append(conds, "q.has_answer")
	} else if f.HasAnswer == "no" {
		conds = append(conds, "NOT q.has_answer")
	}
	if f.Cursor != "" {
		add("(q.updated_at, q.id) < (?::timestamptz, ?::uuid)", f.CursorAt(), f.CursorID())
	}
	args = append(args, f.Limit)
	limit := "$" + store.Itoa(len(args))

	sql := `SELECT q.id::text, q.status, q.has_answer, v.type, v.stem, v.level_id::text, v.subject_id::text, v.version_no, q.updated_at::text
	  FROM questions q JOIN question_versions v ON v.id = q.current_version_id
	  WHERE ` + joinAnd(conds) + `
	  ORDER BY q.updated_at DESC, q.id DESC
	  LIMIT ` + limit
	rows, err := store.CollectRows[questionListRow](ctx, s.db, sql, args...)
	if err != nil {
		return nil, "", err
	}
	out := make([]QuestionAdmin, 0, len(rows))
	var nextCursor string
	for _, r := range rows {
		out = append(out, QuestionAdmin{
			ID: r.ID, Status: r.Status, HasAnswer: r.HasAnswer,
			CurrentVersion: &QuestionVersion{
				ID: "", QuestionID: r.ID, VersionNo: r.VersionNo, Type: r.Type, Stem: r.Stem,
				LevelID: r.LevelID, SubjectID: r.SubjectID,
			},
			UpdatedAt: r.UpdatedAt,
		})
		nextCursor = r.UpdatedAt + "\x00" + r.ID
	}
	if len(rows) < f.Limit {
		nextCursor = ""
	}
	return out, nextCursor, nil
}

type ListFilter struct {
	Status    string
	LevelID   string
	SubjectID string
	Query     string
	HasAnswer string
	Cursor    string // "updated_at\x00id"
	Limit     int
}

func (f ListFilter) CursorAt() string {
	parts := splitCursor(f.Cursor)
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}

func (f ListFilter) CursorID() string {
	parts := splitCursor(f.Cursor)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

func splitCursor(c string) []string {
	if c == "" {
		return nil
	}
	for i := 0; i < len(c); i++ {
		if c[i] == 0 {
			return []string{c[:i], c[i+1:]}
		}
	}
	return nil
}

func replaceOnce(s, old, new string) string {
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}

func joinAnd(conds []string) string {
	out := ""
	for i, c := range conds {
		if i > 0 {
			out += " AND "
		}
		out += "(" + c + ")"
	}
	return out
}

// ---------- 概览统计 ----------

type Overview struct {
	Draft             int `json:"draft"`
	InReview          int `json:"inReview"`
	Published         int `json:"published"`
	Retired           int `json:"retired"`
	PublishedNoAnswer int `json:"publishedNoAnswer"`
	OpenIssues        int `json:"openIssues"`
}

func (s *Store) Overview(ctx context.Context) (Overview, error) {
	var o Overview
	err := s.db.QueryRow(ctx,
		`SELECT
		   count(*) FILTER (WHERE status = 'draft'),
		   count(*) FILTER (WHERE status = 'in_review'),
		   count(*) FILTER (WHERE status = 'published' AND retired_at IS NULL),
		   count(*) FILTER (WHERE retired_at IS NOT NULL),
		   count(*) FILTER (WHERE status = 'published' AND retired_at IS NULL AND NOT has_answer),
		   (SELECT count(*) FROM issue_reports WHERE status = 'open')
		 FROM questions`).Scan(
		&o.Draft, &o.InReview, &o.Published, &o.Retired, &o.PublishedNoAnswer, &o.OpenIssues)
	return o, err
}

// ---------- 练习选题 ----------

type SelectedQuestion struct {
	QuestionID        string
	VersionID         string
	MaterialVersionID *string
	SourceKind        *string
	SourceCreatedAt   *string
	SourceOrder       *int
	SectionSortOrder  *int
}

type SelectionFilter struct {
	UserID            string
	LevelID           string
	SubjectID         string
	SourceID          string
	SourceSectionID   string
	SelectionOrder    string
	KnowledgePointIDs []string
	QuestionIDs       []string // 限定候选题目（错题重练）
	Limit             int
	ExcludeRecent     bool // 排除用户最近 3 个已提交批次中出现过的题
}

func (s *Store) SelectPublishedVersions(ctx context.Context, tx store.DBTx, f SelectionFilter) ([]SelectedQuestion, error) {
	kpAny := f.KnowledgePointIDs
	if kpAny == nil {
		kpAny = []string{}
	}
	qIDs := f.QuestionIDs
	if qIDs == nil {
		qIDs = []string{}
	}
	const base = `SELECT q.id::text, v.id::text, v.material_version_id::text,
	       src.kind, src.created_at::text, v.source_order, ss.sort_order
	  FROM questions q
	  JOIN question_versions v ON v.id = q.published_version_id
	  LEFT JOIN source_sections ss ON ss.id = v.source_section_id
	  LEFT JOIN sources src ON src.id = ss.source_id
	  WHERE q.retired_at IS NULL
	    AND v.level_id::text = $1
	    AND ($2 = '' OR v.subject_id::text = $2)
	    AND ($3 = '' OR v.source_section_id::text = $3)
	    AND ($4 = '' OR ss.source_id::text = $4)
	    AND ($5::uuid[] = '{}' OR q.id = ANY($5::uuid[]))
	    AND ($6::uuid[] = '{}' OR EXISTS (
	      SELECT 1 FROM question_version_knowledge_points qvkp
	      WHERE qvkp.question_version_id = v.id AND qvkp.knowledge_point_id = ANY($6::uuid[])))
	    %s
	  ORDER BY `

	orderBy := `CASE WHEN src.kind = 'book' THEN 0 ELSE 1 END,
	             src.created_at NULLS LAST,
	             v.source_order NULLS LAST,
	             ss.sort_order NULLS LAST,
	             q.id`
	if f.SelectionOrder == "random" {
		orderBy = "random()"
	}

	exclude := ""
	if f.ExcludeRecent {
		exclude = `AND NOT EXISTS (
	      SELECT 1 FROM practice_items pi
	      WHERE pi.question_id = q.id
	        AND pi.session_id IN (
	          SELECT id FROM practice_sessions
		      WHERE user_id = $8 AND submitted_at IS NOT NULL
	          ORDER BY created_at DESC LIMIT 3))`
	}

	run := func(excludeClause string) ([]SelectedQuestion, error) {
		args := []any{f.LevelID, f.SubjectID, f.SourceSectionID, f.SourceID, qIDs, kpAny, f.Limit}
		if excludeClause != "" {
			args = append(args, f.UserID)
		}
		query := replaceClause(base, "%s", excludeClause) + orderBy + " LIMIT $7"
		return store.CollectRows[SelectedQuestion](ctx, tx, query, args...)
	}

	rows, err := run(exclude)
	if err != nil {
		return nil, err
	}
	if f.ExcludeRecent && len(rows) < f.Limit {
		// 优先最近未做过的题；不足时用做过的题补齐
		more, err := run("")
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		for _, r := range rows {
			seen[r.QuestionID] = true
		}
		for _, m := range more {
			if len(rows) >= f.Limit {
				break
			}
			if !seen[m.QuestionID] {
				rows = append(rows, m)
				seen[m.QuestionID] = true
			}
		}
		if f.SelectionOrder != "random" {
			sort.SliceStable(rows, func(i, j int) bool { return sourceOrderLess(rows[i], rows[j]) })
		}
	}
	return rows, nil
}

func sourceOrderLess(a, b SelectedQuestion) bool {
	if rankA, rankB := sourceKindRank(a.SourceKind), sourceKindRank(b.SourceKind); rankA != rankB {
		return rankA < rankB
	}
	if cmp := compareNullableString(a.SourceCreatedAt, b.SourceCreatedAt); cmp != 0 {
		return cmp < 0
	}
	if cmp := compareNullableInt(a.SourceOrder, b.SourceOrder); cmp != 0 {
		return cmp < 0
	}
	if cmp := compareNullableInt(a.SectionSortOrder, b.SectionSortOrder); cmp != 0 {
		return cmp < 0
	}
	return a.QuestionID < b.QuestionID
}

func sourceKindRank(kind *string) int {
	if kind != nil && *kind == "book" {
		return 0
	}
	return 1
}

func compareNullableString(a, b *string) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return 1
	}
	if b == nil {
		return -1
	}
	if *a < *b {
		return -1
	}
	if *a > *b {
		return 1
	}
	return 0
}

func compareNullableInt(a, b *int) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return 1
	}
	if b == nil {
		return -1
	}
	if *a < *b {
		return -1
	}
	if *a > *b {
		return 1
	}
	return 0
}

func replaceClause(s, old, new string) string {
	out := ""
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			out += new
			i += len(old)
		} else {
			out += string(s[i])
			i++
		}
	}
	return out
}
