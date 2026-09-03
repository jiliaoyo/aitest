// backfill-knowledge 为已有发布题目创建带知识点的新版本；旧练习继续引用旧版本。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type knowledgePoint struct {
	ID       string  `json:"id"`
	Slug     string  `json:"slug"`
	Level    string  `json:"level"`
	Subject  string  `json:"subject"`
	Parent   *string `json:"parentId"`
	Name     string  `json:"name"`
	Desc     string  `json:"description"`
	Mistakes string  `json:"commonMistakes"`
	Examples string  `json:"examples"`
	Status   string  `json:"status"`
}

type mapping struct {
	QuestionID        string   `json:"questionId"`
	KnowledgePointIDs []string `json:"knowledgePointIds"`
}

type questionRow struct {
	ID          string
	PublishedID string
	Level       string
	Subject     string
	Stem        string
	HasAnswer   bool
}

type reviewFile struct {
	Version int          `json:"version"`
	Items   []reviewItem `json:"items"`
}

type reviewItem struct {
	Source                     string            `json:"source"`
	QuestionID                 string            `json:"questionId,omitempty"`
	Key                        []json.RawMessage `json:"key,omitempty"`
	Level                      string            `json:"level"`
	Subject                    string            `json:"subject"`
	Stem                       string            `json:"stem,omitempty"`
	KnowledgePointIDs          []string          `json:"knowledgePointIds"`
	Method                     string            `json:"method"`
	Confidence                 float64           `json:"confidence"`
	SuggestedKnowledgePointIDs []string          `json:"suggestedKnowledgePointIds,omitempty"`
	SuggestedConfidence        *float64          `json:"suggestedConfidence,omitempty"`
	SuggestedReviewStatus      string            `json:"suggestedReviewStatus,omitempty"`
	ReviewStatus               string            `json:"reviewStatus"`
	ReviewReason               string            `json:"reviewReason,omitempty"`
	Basis                      string            `json:"basis,omitempty"`
}

func main() {
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL 连接串")
	knowledgePath := flag.String("knowledge", defaultFile("scripts/data/knowledge_points_n4n5.json", "../scripts/data/knowledge_points_n4n5.json"), "知识点 JSON")
	mappingPath := flag.String("mapping", defaultFile("scripts/data/question_knowledge_mapping.json", "../scripts/data/question_knowledge_mapping.json"), "题目映射 JSON")
	reviewPath := flag.String("review", defaultFile("scripts/data/knowledge_mapping_review.json", "../scripts/data/knowledge_mapping_review.json"), "复核记录 JSON")
	flag.Parse()
	if *databaseURL == "" {
		fail(fmt.Errorf("缺少 DATABASE_URL 或 -database-url"))
	}

	ctx := context.Background()
	points := readPoints(*knowledgePath)
	mappings := readMappings(*mappingPath)
	reviews := readReviews(*reviewPath)

	pool, err := store.Open(ctx, *databaseURL)
	if err != nil {
		fail(err)
	}
	defer pool.Close()

	created, fallback, err := backfill(ctx, pool, points, mappings, &reviews)
	if err != nil {
		fail(err)
	}
	if err := writeReviews(*reviewPath, reviews); err != nil {
		fail(err)
	}
	fmt.Printf("backfill knowledge versions created=%d fallback=%d reviews=%d\n", created, fallback, len(reviews.Items))
}

func defaultFile(primary, fallback string) string {
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	return fallback
}

func readPoints(path string) []knowledgePoint {
	var file struct {
		Points []knowledgePoint `json:"points"`
	}
	decodeFile(path, &file)
	if len(file.Points) == 0 {
		fail(fmt.Errorf("知识点数据为空: %s", path))
	}
	return file.Points
}

func readMappings(path string) map[string]mapping {
	var file struct {
		Questions []mapping `json:"questions"`
	}
	decodeFile(path, &file)
	out := make(map[string]mapping, len(file.Questions))
	for _, item := range file.Questions {
		if item.QuestionID == "" || len(item.KnowledgePointIDs) == 0 {
			fail(fmt.Errorf("题目映射缺少题目或知识点"))
		}
		out[item.QuestionID] = item
	}
	return out
}

func readReviews(path string) reviewFile {
	var file reviewFile
	decodeFile(path, &file)
	if file.Version == 0 {
		file.Version = 1
	}
	if file.Items == nil {
		file.Items = []reviewItem{}
	}
	for i := range file.Items {
		if file.Items[i].ReviewStatus == "" {
			file.Items[i].ReviewStatus = "pending"
		}
	}
	return file
}

func decodeFile(path string, dst any) {
	data, err := os.ReadFile(path)
	if err != nil {
		fail(fmt.Errorf("读取 %s 失败: %w", path, err))
	}
	if err := json.Unmarshal(data, dst); err != nil {
		fail(fmt.Errorf("解析 %s 失败: %w", path, err))
	}
}

func backfill(ctx context.Context, pool *pgxpool.Pool, points []knowledgePoint, mappings map[string]mapping, reviews *reviewFile) (int, int, error) {
	pointBySlug := make(map[string]knowledgePoint, len(points))
	pointByID := make(map[string]knowledgePoint, len(points))
	for _, point := range points {
		pointBySlug[point.Slug] = point
		pointByID[point.ID] = point
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("开启知识点回填事务失败: %w", err)
	}
	defer tx.Rollback(ctx)
	for _, point := range points {
		if _, err := tx.Exec(ctx,
			`INSERT INTO knowledge_points (id, exam_id, level_id, subject_id, parent_id, name, description, common_mistakes, examples, status)
			 SELECT $1, e.id, l.id, s.id, $2, $3, $4, $5, $6, $7
			 FROM exams e JOIN exam_levels l ON l.exam_id = e.id JOIN subjects s ON s.exam_id = e.id
			 WHERE e.code = 'jlpt' AND l.code = $8 AND s.code = $9
			 ON CONFLICT (id) DO NOTHING`,
			point.ID, point.Parent, point.Name, point.Desc, point.Mistakes, point.Examples, point.Status, point.Level, point.Subject); err != nil {
			return 0, 0, fmt.Errorf("写入知识点 %s 失败: %w", point.Slug, err)
		}
	}

	rows, err := tx.Query(ctx, `
		SELECT q.id::text, q.published_version_id::text, l.code, s.code, v.stem, q.has_answer
		FROM questions q
		JOIN question_versions v ON v.id = q.published_version_id
		JOIN exam_levels l ON l.id = v.level_id
		JOIN exams e ON e.id = l.exam_id
		JOIN subjects s ON s.id = v.subject_id AND s.exam_id = e.id
		WHERE q.status = 'published'
		  AND e.code = 'jlpt'
		  AND l.code IN ('n4', 'n5')
		ORDER BY q.id`)
	if err != nil {
		return 0, 0, fmt.Errorf("查询待回填题目失败: %w", err)
	}
	var questions []questionRow
	for rows.Next() {
		var q questionRow
		if err := rows.Scan(&q.ID, &q.PublishedID, &q.Level, &q.Subject, &q.Stem, &q.HasAnswer); err != nil {
			rows.Close()
			return 0, 0, fmt.Errorf("读取待回填题目失败: %w", err)
		}
		questions = append(questions, q)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, 0, fmt.Errorf("遍历待回填题目失败: %w", err)
	}
	rows.Close()
	var created, fallback int
	for _, q := range questions {
		item, ok := mappings[q.ID]
		isFallback := !ok
		ids := item.KnowledgePointIDs
		if isFallback {
			ids = fallbackIDs(pointBySlug, q.Level, q.Subject)
			addReview(reviews, q, ids)
		}
		if len(ids) == 0 {
			return 0, 0, fmt.Errorf("题目 %s 没有可用知识点", q.ID)
		}
		existingIDs, err := currentMappingIDs(ctx, tx, q.PublishedID)
		if err != nil {
			return 0, 0, fmt.Errorf("读取题目现有知识点失败 %s: %w", q.ID, err)
		}
		if sameIDs(existingIDs, ids) {
			continue
		}
		if isFallback {
			fallback++
		}
		levelID, subjectID, err := pointScope(ctx, tx, pointByID, ids[0])
		if err != nil {
			return 0, 0, err
		}
		var versionNo int
		if err := tx.QueryRow(ctx, `SELECT coalesce(max(version_no), 0) + 1 FROM question_versions WHERE question_id = $1`, q.ID).Scan(&versionNo); err != nil {
			return 0, 0, fmt.Errorf("读取题目版本号失败 %s: %w", q.ID, err)
		}
		var newVersionID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO question_versions
				(question_id, version_no, type, stem, material_version_id, options, level_id, subject_id, source_section_id, difficulty, source_order, created_by)
			SELECT $1, $2, v.type, v.stem, v.material_version_id, v.options, $3, $4, v.source_section_id, v.difficulty, v.source_order, NULL
			FROM question_versions v WHERE v.id = $5
			RETURNING id::text`, q.ID, versionNo, levelID, subjectID, q.PublishedID).Scan(&newVersionID); err != nil {
			return 0, 0, fmt.Errorf("创建题目知识点版本失败 %s: %w", q.ID, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO answer_keys (question_version_id, value, authority, explanation, created_by)
			SELECT $1, value, authority, explanation, created_by
			FROM answer_keys WHERE question_version_id = $2`, newVersionID, q.PublishedID); err != nil {
			return 0, 0, fmt.Errorf("复制权威答案失败 %s: %w", q.ID, err)
		}
		for _, id := range ids {
			if _, err := tx.Exec(ctx,
				`INSERT INTO question_version_knowledge_points (question_version_id, knowledge_point_id) VALUES ($1, $2)`, newVersionID, id); err != nil {
				return 0, 0, fmt.Errorf("写入题目知识点映射失败 %s: %w", q.ID, err)
			}
		}
		if _, err := tx.Exec(ctx,
			`UPDATE questions SET current_version_id = $2, published_version_id = $2, has_answer = $3, updated_at = now() WHERE id = $1`,
			q.ID, newVersionID, q.HasAnswer); err != nil {
			return 0, 0, fmt.Errorf("切换题目版本失败 %s: %w", q.ID, err)
		}
		created++
	}
	if _, err := tx.Exec(ctx, `UPDATE knowledge_points SET status = 'retired', updated_at = now()
		WHERE name IN ('助词 は 与 が', '助词 に 与 で', 'て形', '时间名词', '形容词活用')
		  AND id <> ALL($1::uuid[])`, legacyIDs(points)); err != nil {
		return 0, 0, fmt.Errorf("收起旧演示知识点失败: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, fmt.Errorf("提交知识点回填事务失败: %w", err)
	}
	return created, fallback, nil
}

func currentMappingIDs(ctx context.Context, tx pgx.Tx, versionID string) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT knowledge_point_id::text
		FROM question_version_knowledge_points WHERE question_version_id = $1`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func sameIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]bool, len(a))
	for _, id := range a {
		seen[id] = true
	}
	for _, id := range b {
		if !seen[id] {
			return false
		}
	}
	return true
}

func legacyIDs(points []knowledgePoint) []string {
	ids := make([]string, 0, len(points))
	for _, point := range points {
		ids = append(ids, point.ID)
	}
	return ids
}

func pointScope(ctx context.Context, tx pgx.Tx, points map[string]knowledgePoint, id string) (string, string, error) {
	if point, ok := points[id]; ok {
		var levelID, subjectID string
		err := tx.QueryRow(ctx, `
			SELECT l.id::text, s.id::text FROM exams e
			JOIN exam_levels l ON l.exam_id = e.id JOIN subjects s ON s.exam_id = e.id
			WHERE e.code = 'jlpt' AND l.code = $1 AND s.code = $2
			  AND EXISTS (SELECT 1 FROM knowledge_points kp WHERE kp.id = $3)`,
			point.Level, point.Subject, id).Scan(&levelID, &subjectID)
		if err != nil {
			return "", "", fmt.Errorf("读取知识点范围失败 %s: %w", id, err)
		}
		return levelID, subjectID, nil
	}
	return "", "", fmt.Errorf("映射引用未定义知识点 %s", id)
}

func fallbackIDs(points map[string]knowledgePoint, level, subject string) []string {
	root, rootOK := points[fmt.Sprintf("%s-%s", level, subject)]
	if !rootOK {
		return nil
	}
	return []string{root.ID}
}

func addReview(file *reviewFile, q questionRow, ids []string) {
	for _, item := range file.Items {
		if item.Source == "database_fallback" && item.Level == q.Level && item.Subject == q.Subject && item.Stem == q.Stem {
			return
		}
	}
	confidence := 0.5
	file.Items = append(file.Items, reviewItem{
		Source: "database_fallback", QuestionID: q.ID, Level: q.Level, Subject: q.Subject,
		Stem: q.Stem, KnowledgePointIDs: ids, Confidence: confidence,
		Method: "scope_fallback", ReviewStatus: "pending",
		ReviewReason: "非书籍映射，按级别和科目兜底，需人工确认",
	})
	sort.Slice(file.Items, func(i, j int) bool {
		return reviewSortKey(file.Items[i]) < reviewSortKey(file.Items[j])
	})
}

func reviewSortKey(item reviewItem) string {
	return item.QuestionID + "\x00" + item.Source + "\x00" + item.Stem
}

func writeReviews(path string, file reviewFile) error {
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化复核记录失败: %w", err)
	}
	data = append(data, '\n')
	old, readErr := os.ReadFile(path)
	if readErr == nil && bytes.Equal(old, data) {
		return nil
	}
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("读取现有复核记录失败: %w", readErr)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("写入复核记录失败: %w", err)
	}
	return nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
