package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// CollectRows 把查询结果按列位置扫描进结构体切片（字段顺序必须与 SELECT 列顺序一致）。
func CollectRows[T any](ctx context.Context, db DBTx, sql string, args ...any) ([]T, error) {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[T])
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CollectOneRow 扫描单行；无结果返回 pgx.ErrNoRows。
func CollectOneRow[T any](ctx context.Context, db DBTx, sql string, args ...any) (T, error) {
	var v T
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	data, err := pgx.CollectOneRow(rows, pgx.RowToStructByPos[T])
	if err != nil {
		return v, err
	}
	return data, nil
}

func Exists(ctx context.Context, db DBTx, sql string, args ...any) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, sql, args...).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return ok, err
}

// BuildUpdate 把 JSON 字段名映射为列名并构造 SET 子句。
// 返回形如 "col1 = $1, col2 = $2" 的片段与按序参数；忽略未提供的字段。
func BuildUpdate(allowed map[string]string, fields map[string]any) (string, []any, error) {
	var sets []string
	var args []any
	for key, col := range allowed {
		v, ok := fields[key]
		if !ok {
			continue
		}
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if len(sets) == 0 {
		return "", nil, nil
	}
	return strings.Join(sets, ", "), args, nil
}

// Itoa 供模块 store 拼接参数占位符使用。
func Itoa(n int) string { return strconv.Itoa(n) }
