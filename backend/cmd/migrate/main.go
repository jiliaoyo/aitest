// migrate 是一个与 goose 文件格式兼容的最小迁移器：
// 按文件名顺序执行 migrations/*.sql 中 "-- +goose Up" 段落，并在 schema_migrations 记录版本。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

type migration struct {
	name string
	up   string
}

func main() {
	url := flag.String("url", os.Getenv("DATABASE_URL"), "数据库连接串")
	dir := flag.String("dir", "migrations", "迁移目录")
	flag.Parse()
	if *url == "" {
		fmt.Fprintln(os.Stderr, "缺少 DATABASE_URL")
		os.Exit(1)
	}
	ctx := context.Background()
	cfg, err := pgx.ParseConfig(*url)
	if err != nil {
		fail("解析连接串失败", err)
	}
	// 多语句迁移需要简单协议
	cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		fail("连接数据库失败", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
		   version text PRIMARY KEY,
		   applied_at timestamptz NOT NULL DEFAULT now()
		 )`); err != nil {
		fail("创建 schema_migrations 失败", err)
	}

	files, err := os.ReadDir(*dir)
	if err != nil {
		fail("读取迁移目录失败", err)
	}
	var names []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".sql") {
			names = append(names, f.Name())
		}
	}
	sort.Strings(names)

	applied := map[string]bool{}
	rows, err := conn.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		fail("查询已应用迁移失败", err)
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			fail("读取迁移记录失败", err)
		}
		applied[v] = true
	}
	rows.Close()

	for _, name := range names {
		if applied[name] {
			continue
		}
		data, err := os.ReadFile(*dir + "/" + name)
		if err != nil {
			fail("读取迁移文件失败", err)
		}
		up, err := upSection(string(data))
		if err != nil {
			fail(name, err)
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			fail("开启迁移事务失败", err)
		}
		if _, err := tx.Exec(ctx, up); err != nil {
			_ = tx.Rollback(ctx)
			fail("执行迁移 "+name+" 失败", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			fail("记录迁移版本失败", err)
		}
		if err := tx.Commit(ctx); err != nil {
			fail("提交迁移事务失败", err)
		}
		fmt.Println("applied", name)
	}
	fmt.Println("migrations up to date")
}

func upSection(sqlText string) (string, error) {
	upIdx := strings.Index(sqlText, "-- +goose Up")
	if upIdx < 0 {
		return "", fmt.Errorf("缺少 +goose Up 段")
	}
	rest := sqlText[upIdx+len("-- +goose Up"):]
	downIdx := strings.Index(rest, "-- +goose Down")
	if downIdx >= 0 {
		rest = rest[:downIdx]
	}
	return strings.TrimSpace(rest), nil
}

func fail(msg string, err error) {
	fmt.Fprintln(os.Stderr, msg+":", err)
	os.Exit(1)
}
