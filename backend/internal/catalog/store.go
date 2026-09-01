package catalog

import (
	"context"
	"errors"

	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

type Store struct{ db store.DBTx }

func NewStore(db store.DBTx) *Store { return &Store{db: db} }

type levelRow struct {
	ID        string
	ExamID    string
	Code      string
	Name      string
	SortOrder int
}

type subjectRow struct {
	ID        string
	ExamID    string
	Code      string
	Name      string
	SortOrder int
}

type examRow struct {
	ID   string
	Code string
	Name string
}

func (s *Store) Catalog(ctx context.Context) ([]Exam, error) {
	rows, err := store.CollectRows[examRow](ctx, s.db, `SELECT id, code, name FROM exams ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	exams := make([]Exam, 0, len(rows))
	for _, r := range rows {
		exams = append(exams, Exam{ID: r.ID, Code: r.Code, Name: r.Name})
	}
	levels, err := store.CollectRows[levelRow](ctx, s.db,
		`SELECT id, exam_id, code, name, sort_order FROM exam_levels ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	subjects, err := store.CollectRows[subjectRow](ctx, s.db,
		`SELECT id, exam_id, code, name, sort_order FROM subjects ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	levelsByExam := map[string][]Level{}
	for _, l := range levels {
		levelsByExam[l.ExamID] = append(levelsByExam[l.ExamID], Level{ID: l.ID, Code: l.Code, Name: l.Name, SortOrder: l.SortOrder})
	}
	subjectsByExam := map[string][]Subject{}
	for _, sub := range subjects {
		subjectsByExam[sub.ExamID] = append(subjectsByExam[sub.ExamID], Subject{ID: sub.ID, Code: sub.Code, Name: sub.Name, SortOrder: sub.SortOrder})
	}
	for i := range exams {
		exams[i].Levels = levelsByExam[exams[i].ID]
		exams[i].Subjects = subjectsByExam[exams[i].ID]
		if exams[i].Levels == nil {
			exams[i].Levels = []Level{}
		}
		if exams[i].Subjects == nil {
			exams[i].Subjects = []Subject{}
		}
	}
	return exams, nil
}

func (s *Store) LevelExists(ctx context.Context, id string) (bool, error) {
	return store.Exists(ctx, s.db, `SELECT true FROM exam_levels WHERE id = $1`, id)
}

func (s *Store) SubjectExists(ctx context.Context, id string) (bool, error) {
	return store.Exists(ctx, s.db, `SELECT true FROM subjects WHERE id = $1`, id)
}

// KnowledgePointExists 供 content 模块校验题目引用的知识点。
func (s *Store) KnowledgePointExists(ctx context.Context, id string) (bool, error) {
	return store.Exists(ctx, s.db, `SELECT true FROM knowledge_points WHERE id = $1`, id)
}

type kpRow struct {
	ID             string
	ExamID         string
	LevelID        string
	SubjectID      string
	ParentID       *string
	Name           string
	Description    string
	CommonMistakes string
	Examples       string
	Status         string
	QuestionCount  int
}

const kpColumns = `kp.id, kp.exam_id, kp.level_id::text, kp.subject_id::text, kp.parent_id, kp.name,
 kp.description, kp.common_mistakes, kp.examples, kp.status,
 (SELECT count(*) FROM question_version_knowledge_points qvkp
  JOIN question_versions v ON v.id = qvkp.question_version_id
  WHERE qvkp.knowledge_point_id = kp.id) AS question_count`

func toKP(r kpRow) KnowledgePoint {
	return KnowledgePoint{
		ID: r.ID, ExamID: r.ExamID, LevelID: r.LevelID, SubjectID: r.SubjectID,
		ParentID: r.ParentID, Name: r.Name, Description: r.Description,
		CommonMistakes: r.CommonMistakes, Examples: r.Examples,
		Status: r.Status, QuestionCount: r.QuestionCount,
	}
}

func (s *Store) KnowledgePointByID(ctx context.Context, id string) (KnowledgePoint, error) {
	r, err := store.CollectOneRow[kpRow](ctx, s.db,
		`SELECT `+kpColumns+` FROM knowledge_points kp WHERE kp.id = $1`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return KnowledgePoint{}, httpapi.ErrNotFound
	}
	if err != nil {
		return KnowledgePoint{}, err
	}
	return toKP(r), nil
}

func (s *Store) ListKnowledgePointsAdmin(ctx context.Context, levelID string) ([]KnowledgePoint, error) {
	rows, err := store.CollectRows[kpRow](ctx, s.db,
		`SELECT `+kpColumns+` FROM knowledge_points kp
		 WHERE ($1 = '' OR kp.level_id::text = $1)
		 ORDER BY kp.created_at`, levelID)
	if err != nil {
		return nil, err
	}
	out := make([]KnowledgePoint, 0, len(rows))
	for _, r := range rows {
		out = append(out, toKP(r))
	}
	return out, nil
}

func (s *Store) CreateKnowledgePoint(ctx context.Context, kp *KnowledgePoint) error {
	var id string
	err := s.db.QueryRow(ctx,
		`INSERT INTO knowledge_points (exam_id, level_id, subject_id, parent_id, name, description, common_mistakes, examples, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id`,
		kp.ExamID, kp.LevelID, kp.SubjectID, kp.ParentID, kp.Name,
		kp.Description, kp.CommonMistakes, kp.Examples, kp.Status,
	).Scan(&id)
	if err != nil {
		return err
	}
	kp.ID = id
	return nil
}

func (s *Store) UpdateKnowledgePoint(ctx context.Context, id string, fields map[string]any) (KnowledgePoint, error) {
	allowed := map[string]string{
		"name":           "name",
		"description":    "description",
		"commonMistakes": "common_mistakes",
		"examples":       "examples",
		"status":         "status",
		"parentId":       "parent_id",
	}
	set, args, err := store.BuildUpdate(allowed, fields)
	if err != nil {
		return KnowledgePoint{}, err
	}
	if set != "" {
		args = append(args, id)
		if _, err := s.db.Exec(ctx,
			`UPDATE knowledge_points SET `+set+`, updated_at = now() WHERE id = $`+store.Itoa(len(args)), args...); err != nil {
			return KnowledgePoint{}, err
		}
	}
	return s.KnowledgePointByID(ctx, id)
}

// ParentScope 返回父知识点所属 level/subject，用于校验子知识点不跨科目挂接。
func (s *Store) ParentScope(ctx context.Context, parentID string) (levelID, subjectID string, err error) {
	err = s.db.QueryRow(ctx,
		`SELECT level_id::text, subject_id::text FROM knowledge_points WHERE id = $1`, parentID,
	).Scan(&levelID, &subjectID)
	return
}
