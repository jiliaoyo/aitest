package jobs

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const lease = 10 * time.Minute
const reapInterval = time.Minute

// Worker 周期领取任务并按 kind 调度；首版用一个明确的 switch/map，不做插件注册框架。
type Worker struct {
	pool        *pgxpool.Pool
	id          string
	concurrency int
	handlers    map[string]Handler
	logger      *slog.Logger
}

func NewWorker(pool *pgxpool.Pool, id string, concurrency int, handlers map[string]Handler, logger *slog.Logger) *Worker {
	return &Worker{pool: pool, id: id, concurrency: concurrency, handlers: handlers, logger: logger}
}

// Run 阻塞运行直到 ctx 取消。每个 worker 槽位串行处理任务，槽位数由配置控制。
func (w *Worker) Run(ctx context.Context) {
	w.logger.Info("worker_started", "worker_id", w.id, "concurrency", w.concurrency)
	if err := ReleaseExpired(ctx, w.pool); err != nil {
		w.logger.Warn("release_expired_failed", "error", err)
	}
	var slots sync.WaitGroup
	for range w.concurrency {
		slots.Add(1)
		go func() {
			defer slots.Done()
			w.runSlot(ctx)
		}()
	}
	ticker := time.NewTicker(reapInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slots.Wait()
			return
		case <-ticker.C:
			if err := ReleaseExpired(ctx, w.pool); err != nil {
				w.logger.Warn("release_expired_failed", "error", err)
			}
		}
	}
}

func (w *Worker) runSlot(ctx context.Context) {
	for ctx.Err() == nil {
		job, err := Claim(ctx, w.pool, w.id, lease)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.logger.Error("claim_failed", "error", err, "worker_id", w.id)
			sleepCtx(ctx, time.Second)
			continue
		}
		if job.ID == "" {
			sleepCtx(ctx, 500*time.Millisecond)
			continue
		}
		w.runJob(ctx, job)
	}
}

func (w *Worker) runJob(ctx context.Context, job Job) {
	start := time.Now()
	logger := w.logger.With("job_id", job.ID, "kind", job.Kind, "attempt", job.Attempts)
	handler, ok := w.handlers[job.Kind]
	if !ok {
		logger.Error("unknown_job_kind")
		_ = Fail(ctx, w.pool, job, errUnknownKind(job.Kind))
		return
	}
	err := func() (err error) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("job_panic", "panic", rec)
				err = errPanic
			}
		}()
		jobCtx, cancel := context.WithTimeout(ctx, lease)
		defer cancel()
		return handler(jobCtx, job.Attempts, job.MaxAttempts, job.Payload)
	}()
	if err != nil {
		logger.Error("job_failed", "error", err, "duration_ms", time.Since(start).Milliseconds())
		_ = Fail(ctx, w.pool, job, err)
		return
	}
	if err := Complete(ctx, w.pool, job.ID); err != nil {
		logger.Error("complete_failed", "error", err)
		return
	}
	logger.Info("job_done", "duration_ms", time.Since(start).Milliseconds())
}

func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

type simpleError string

func (e simpleError) Error() string { return string(e) }

const errPanic = simpleError("handler panicked")

func errUnknownKind(kind string) error {
	return simpleError("unknown job kind: " + kind)
}
