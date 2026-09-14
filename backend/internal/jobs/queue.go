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
	StatusQueued  = "queued"
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

// EnqueueUserLearningRebuildTx 合并同一账号尚未领取的统计重建任务。
// 正在运行的任务不合并，避免新提交落在其事务快照之后而无人再次重建。
func EnqueueUserLearningRebuildTx(ctx context.Context, tx pgx.Tx, userID string) error {
	data, err := json.Marshal(map[string]string{"userId": userID})
	if err != nil {
		return fmt.Errorf("序列化学习统计任务 payload 失败: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO jobs (kind, payload)
		VALUES ('rebuild_user_knowledge_stats', $1)
		ON CONFLICT DO NOTHING`, data)
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

var ErrLeaseLost = errors.New("job lease lost")

type leaseClaim struct {
	jobID    string
	workerID string
	attempt  int
}

type leaseKey struct{}

func withLease(ctx context.Context, jobID, workerID string, attempt int) context.Context {
	return context.WithValue(ctx, leaseKey{}, leaseClaim{jobID: jobID, workerID: workerID, attempt: attempt})
}

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// GuardLease 在业务写事务内锁定当前任务行，防止租约失效后的旧处理器写回结果。
// 直接调用 handler 的测试和运维代码没有租约上下文，此时保持原有行为。
func GuardLease(ctx context.Context, db queryRower) error {
	claim, ok := ctx.Value(leaseKey{}).(leaseClaim)
	if !ok {
		return nil
	}
	var id string
	err := db.QueryRow(ctx, `SELECT id::text FROM jobs
		WHERE id = $1 AND status = 'running' AND locked_by = $2 AND attempts = $3
		  AND locked_until > now() FOR UPDATE`, claim.jobID, claim.workerID, claim.attempt).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLeaseLost
	}
	return err
}

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

// ReleaseExpired 回收租约到期任务；耗尽尝试次数时直接收敛到失败。
func ReleaseExpired(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx,
		`UPDATE jobs
		 SET status = CASE WHEN attempts >= max_attempts THEN 'failed' ELSE 'queued' END,
		     locked_by = NULL, locked_until = NULL,
		     last_error = CASE WHEN attempts >= max_attempts
		       THEN 'worker lease expired after final attempt' ELSE 'worker lease expired' END,
		     updated_at = now()
		 WHERE status = 'running' AND locked_until < now()
		 RETURNING kind, payload, status`)
	if err != nil {
		return err
	}
	type expiredJob struct {
		kind    string
		payload json.RawMessage
		status  string
	}
	var expired []expiredJob
	for rows.Next() {
		var job expiredJob
		if err := rows.Scan(&job.kind, &job.payload, &job.status); err != nil {
			rows.Close()
			return err
		}
		expired = append(expired, job)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, job := range expired {
		if job.status == "failed" {
			if err := failTerminalBusiness(ctx, tx, job.kind, job.payload); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

func failTerminalBusiness(ctx context.Context, tx pgx.Tx, kind string, payload json.RawMessage) error {
	var target struct {
		SessionID string `json:"sessionId"`
		ItemID    string `json:"itemId"`
	}
	if json.Unmarshal(payload, &target) != nil || target.SessionID == "" {
		return nil
	}
	switch kind {
	case "generate_ai_practice_session":
		_, err := tx.Exec(ctx, `UPDATE practice_sessions
			SET status = 'generation_failed', ai_summary_status = 'failed',
			    ai_summary = 'AI 出题失败，请重新开始。', updated_at = now()
			WHERE id = $1 AND status = 'generating'`, target.SessionID)
		return err
	case "analyze_practice_session_ai":
		if _, err := tx.Exec(ctx, `UPDATE grading_results
			SET status = 'failed', explanation = COALESCE(NULLIF(explanation, ''), 'AI 批次分析失败，稍后可重试。'),
			    explanation_source = 'ai', updated_at = now()
			WHERE session_id = $1 AND source = 'ai' AND status = 'pending'`, target.SessionID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE practice_sessions
			SET status = 'analysis_failed', ai_summary_status = 'failed',
			    ai_summary = 'AI 批次分析失败，稍后可重试。', completed_at = now(), updated_at = now()
			WHERE id = $1 AND status IN ('grading', 'completed')`, target.SessionID)
		return err
	case "grade_practice_item_ai":
		if target.ItemID == "" {
			return nil
		}
		_, err := tx.Exec(ctx, `UPDATE grading_results
			SET status = 'failed', explanation = 'AI 判定失败，稍后可重试。',
			    explanation_source = 'ai', updated_at = now()
			WHERE session_id = $1 AND item_id = $2 AND source = 'ai' AND status = 'pending'`,
			target.SessionID, target.ItemID)
		return err
	default:
		return nil
	}
}

// Complete 把任务标记成功。
func Complete(ctx context.Context, pool *pgxpool.Pool, j Job, workerID string) error {
	tag, err := pool.Exec(ctx,
		`UPDATE jobs SET status = 'succeeded', locked_by = NULL, locked_until = NULL,
		        last_error = '', updated_at = now()
		 WHERE id = $1 AND status = 'running' AND locked_by = $2 AND attempts = $3
		   AND locked_until > now()`, j.ID, workerID, j.Attempts)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrLeaseLost
	}
	return err
}

// Fail 处理失败：未超重试次数则指数退避重新入队，超过则标记 failed 并保留错误摘要。
func Fail(ctx context.Context, pool *pgxpool.Pool, j Job, workerID string, jobErr error) error {
	msg := jobErr.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	if j.Attempts >= j.MaxAttempts {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		tag, err := tx.Exec(ctx,
			`UPDATE jobs SET status = 'failed', locked_by = NULL, locked_until = NULL,
			        last_error = $2, updated_at = now()
			 WHERE id = $1 AND status = 'running' AND locked_by = $3 AND attempts = $4
			   AND locked_until > now()`, j.ID, msg, workerID, j.Attempts)
		if err == nil && tag.RowsAffected() == 0 {
			return ErrLeaseLost
		}
		if err != nil {
			return err
		}
		if err := failTerminalBusiness(ctx, tx, j.Kind, j.Payload); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	backoff := time.Duration(1<<uint(j.Attempts)) * 30 * time.Second
	tag, err := pool.Exec(ctx,
		`UPDATE jobs SET status = 'queued', locked_by = NULL, locked_until = NULL,
		        available_at = $2, last_error = $3, updated_at = now()
		 WHERE id = $1 AND status = 'running' AND locked_by = $4 AND attempts = $5
		   AND locked_until > now()`, j.ID, time.Now().Add(backoff), msg, workerID, j.Attempts)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrLeaseLost
	}
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
