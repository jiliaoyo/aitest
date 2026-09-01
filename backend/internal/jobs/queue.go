// Package jobs 提供基于 PostgreSQL 任务表的异步任务队列：入队、领取、租约、重试。
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	StatusQueued = "queued"
	StatusRunning = "running"
)

// EnqueueTx 在业务事务内创建任务，保证数据变更与任务入队原子生效。
func EnqueueTx(ctx context.Context, tx pgx.Tx, kind string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化任务 payload 失败: %w", err)
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO jobs (kind, payload) VALUES ($1, $2)`, kind, data)
	return err
}

type Job struct {
	ID          string
	Kind        string
	Payload     json.RawMessage
	Attempts    int
	MaxAttempts int
}

// Handler 处理一种任务；返回错误触发退避重试。attempts/maxAttempts 供处理器在最终失败时落库业务状态。
type Handler func(ctx context.Context, attempts, maxAttempts int, payload json.RawMessage) error

// Claim 原子领取一条到期任务（FOR UPDATE SKIP LOCKED），并写租约。
func Claim(ctx context.Context, pool *pgxpool.Pool, workerID string, lease time.Duration) (Job, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback(ctx)
	var j Job
	err = tx.QueryRow(ctx,
		`UPDATE jobs SET status = 'running', locked_by = $1, locked_until = $2,
		        attempts = attempts + 1, updated_at = now()
		 WHERE id = (
		   SELECT id FROM jobs
		   WHERE status = 'queued' AND available_at <= now()
		   ORDER BY created_at
		   FOR UPDATE SKIP LOCKED
		   LIMIT 1
		 )
		 RETURNING id::text, kind, payload, attempts, max_attempts`,
		workerID, time.Now().Add(lease),
	).Scan(&j.ID, &j.Kind, &j.Payload, &j.Attempts, &j.MaxAttempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, nil
	}
	if err != nil {
		return Job{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Job{}, err
	}
	return j, nil
}

// Release 租约到期任务回收为可领取；进程崩溃后由 reaper 兜底。
func ReleaseExpired(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx,
		`UPDATE jobs SET status = 'queued', locked_by = NULL, locked_until = NULL, updated_at = now()
		 WHERE status = 'running' AND locked_until < now()`)
	return err
}

// Complete 把任务标记成功。
func Complete(ctx context.Context, pool *pgxpool.Pool, jobID string) error {
	_, err := pool.Exec(ctx,
		`UPDATE jobs SET status = 'succeeded', locked_by = NULL, locked_until = NULL,
		        last_error = '', updated_at = now()
		 WHERE id = $1`, jobID)
	return err
}

// Fail 处理失败：未超重试次数则指数退避重新入队，超过则标记 failed 并保留错误摘要。
func Fail(ctx context.Context, pool *pgxpool.Pool, j Job, jobErr error) error {
	msg := jobErr.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	if j.Attempts >= j.MaxAttempts {
		_, err := pool.Exec(ctx,
			`UPDATE jobs SET status = 'failed', locked_by = NULL, locked_until = NULL,
			        last_error = $2, updated_at = now()
			 WHERE id = $1`, j.ID, msg)
		return err
	}
	backoff := time.Duration(1<<uint(j.Attempts)) * 30 * time.Second
	_, err := pool.Exec(ctx,
		`UPDATE jobs SET status = 'queued', locked_by = NULL, locked_until = NULL,
		        available_at = $2, last_error = $3, updated_at = now()
		 WHERE id = $1`, j.ID, time.Now().Add(backoff), msg)
	return err
}

// EnqueueNow 在事务外补建任务（重试入口等场景）。
func EnqueueNow(ctx context.Context, pool *pgxpool.Pool, kind string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `INSERT INTO jobs (kind, payload) VALUES ($1, $2)`, kind, data)
	return err
}
