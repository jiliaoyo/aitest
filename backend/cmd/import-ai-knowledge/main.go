// import-ai-knowledge 将 AI 知识点候选导入为独立草稿分类。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

var levels = map[string]bool{"n1": true, "n2": true, "n3": true, "n4": true, "n5": true}
var subjects = map[string]string{"grammar": "语法", "vocabulary": "文字词汇", "reading": "阅读"}

type point struct {
	ID             string `json:"id"`
	Slug           string `json:"slug"`
	Level          string `json:"level"`
	Subject        string `json:"subject"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	CommonMistakes string `json:"commonMistakes"`
	Examples       string `json:"examples"`
	Status         string `json:"status"`
}

type batch struct {
	Level   string  `json:"level"`
	Subject string  `json:"subject"`
	Points  []point `json:"points"`
}

func main() {
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL 连接串")
	dir := flag.String("dir", "../scripts/data/knowledge_points_ai_batches", "AI 知识点批次目录")
	flag.Parse()
	if *databaseURL == "" {
		fail(errors.New("缺少 DATABASE_URL 或 -database-url"))
	}
	points := readBatches(*dir)

	ctx := context.Background()
	p, err := store.Open(ctx, *databaseURL)
	if err != nil {
		fail(err)
	}
	defer p.Close()

	insertedRoots, insertedPoints, err := importPoints(ctx, p, points)
	if err != nil {
		fail(err)
	}
	fmt.Printf("AI 知识点候选导入完成：根分类 %d 个，知识点 %d 个（重复项跳过）\n", insertedRoots, insertedPoints)
}

func readBatches(dir string) []point {
	paths, err := filepath.Glob(filepath.Join(dir, "n[1-5]_*.json"))
	if err != nil {
		fail(fmt.Errorf("查找 AI 知识点批次失败: %w", err))
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		fail(fmt.Errorf("没有找到 AI 知识点批次: %s", dir))
	}
	seenIDs := map[string]bool{}
	seenSlugs := map[string]bool{}
	var out []point
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			fail(fmt.Errorf("读取 %s 失败: %w", path, err))
		}
		var b batch
		if err := json.Unmarshal(data, &b); err != nil {
			fail(fmt.Errorf("解析 %s 失败: %w", path, err))
		}
		if !levels[b.Level] || subjects[b.Subject] == "" {
			fail(fmt.Errorf("%s 的级别或科目不合法", path))
		}
		for _, p := range b.Points {
			if p.Level != b.Level || p.Subject != b.Subject || p.ID == "" || p.Slug == "" || p.Name == "" || p.Status != "draft" {
				fail(fmt.Errorf("%s 含有不完整或非草稿知识点: %s", path, p.Name))
			}
			if seenIDs[p.ID] || seenSlugs[p.Slug] {
				fail(fmt.Errorf("知识点 ID 或 slug 重复: %s", p.Slug))
			}
			seenIDs[p.ID] = true
			seenSlugs[p.Slug] = true
			out = append(out, p)
		}
	}
	return out
}

func importPoints(ctx context.Context, pool *pgxpool.Pool, points []point) (int64, int64, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("开启 AI 知识点导入事务失败: %w", err)
	}
	defer tx.Rollback(ctx)

	rootIDs := map[string]string{}
	var roots, inserted int64
	for _, p := range points {
		key := p.Level + ":" + p.Subject
		rootID, ok := rootIDs[key]
		if !ok {
			if err := tx.QueryRow(ctx, `
				SELECT md5($1)::uuid::text`, "aishuati:ai-knowledge-root:"+key).Scan(&rootID); err != nil {
				return 0, 0, fmt.Errorf("生成 AI 知识点根分类 ID 失败 %s: %w", key, err)
			}
			tag, err := tx.Exec(ctx, `
				INSERT INTO knowledge_points
					(id, exam_id, level_id, subject_id, name, description, common_mistakes, examples, status)
				SELECT $1::uuid, e.id, l.id, s.id, $2, $3, $4, $5, 'draft'
				FROM exams e
				JOIN exam_levels l ON l.exam_id = e.id AND l.code = $6
				JOIN subjects s ON s.exam_id = e.id AND s.code = $7
				WHERE e.code = 'jlpt'
				ON CONFLICT (id) DO NOTHING`, rootID,
				"AI 生成候选（"+p.Level+" · "+subjects[p.Subject]+"）",
				"AI 批量生成的待审核知识点分类。", "导入后需人工审核。", "", p.Level, p.Subject)
			if err != nil {
				return 0, 0, fmt.Errorf("写入 AI 知识点根分类失败 %s: %w", key, err)
			}
			roots += tag.RowsAffected()
			rootIDs[key] = rootID
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO knowledge_points
				(id, exam_id, level_id, subject_id, parent_id, name, description, common_mistakes, examples, status)
			SELECT $1::uuid, e.id, l.id, s.id, $2::uuid, $3, $4, $5, $6, 'draft'
			FROM exams e
			JOIN exam_levels l ON l.exam_id = e.id AND l.code = $7
			JOIN subjects s ON s.exam_id = e.id AND s.code = $8
			WHERE e.code = 'jlpt'
			ON CONFLICT (id) DO NOTHING`, p.ID, rootID, p.Name, p.Description, p.CommonMistakes, p.Examples, p.Level, p.Subject)
		if err != nil {
			return 0, 0, fmt.Errorf("写入 AI 知识点失败 %s: %w", p.Slug, err)
		}
		inserted += tag.RowsAffected()
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, fmt.Errorf("提交 AI 知识点导入事务失败: %w", err)
	}
	return roots, inserted, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
