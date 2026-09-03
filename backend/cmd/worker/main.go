package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/aishuati/backend/internal/ai"
	"github.com/aishuati/backend/internal/config"
	"github.com/aishuati/backend/internal/jobs"
	"github.com/aishuati/backend/internal/learning"
	"github.com/aishuati/backend/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	pool, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database_open_failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	aiClient := ai.NewClient(ai.Config{
		BaseURL: cfg.AIBaseURL, APIKey: cfg.AIAPIKey, Model: cfg.AIModel, Timeout: cfg.AITimeout,
	}, pool, logger)
	aiService := ai.NewService(pool, aiClient, logger)
	learningHandler := learning.NewHandler(pool, logger)

	handlers := map[string]jobs.Handler{}
	for k, v := range aiService.Handlers() {
		handlers[k] = v
	}
	for k, v := range learningHandler.Handlers() {
		handlers[k] = v
	}

	worker := jobs.NewWorker(pool, cfg.WorkerID, cfg.WorkerConcurrency, handlers, logger)
	worker.Run(ctx)
}
