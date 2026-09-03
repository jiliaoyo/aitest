// Package app 组装配置、数据库、各领域模块与 HTTP 路由。
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aishuati/backend/internal/ai"
	"github.com/aishuati/backend/internal/auth"
	"github.com/aishuati/backend/internal/catalog"
	"github.com/aishuati/backend/internal/config"
	"github.com/aishuati/backend/internal/content"
	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/imports"
	"github.com/aishuati/backend/internal/jobs"
	"github.com/aishuati/backend/internal/learning"
	"github.com/aishuati/backend/internal/practice"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	pool, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	finalHandler := newHTTPHandler(ctx, cfg, pool, logger)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           finalHandler,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	errCh := make(chan error, 1)
	go func() {
		logger.Info("server_started", "addr", cfg.HTTPAddr, "app_env", cfg.AppEnv)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("server_stopping")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return fmt.Errorf("HTTP 服务退出: %w", err)
	}
}

func newHTTPHandler(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger) http.Handler {
	// 各领域模块
	authStore := auth.NewStore(pool)
	authService := auth.NewService(authStore, pool, logger, cfg.SessionTTL)
	authHandler := auth.NewHandler(authService, cfg.AppEnv, cfg.SecureCookie(), cfg.TrustedProxyCIDRs)

	catalogStore := catalog.NewStore(pool)
	catalogHandler := catalog.NewHandler(catalogStore, logger)

	contentStore := content.NewStore(pool)
	contentService := content.NewService(pool, catalogStore)
	contentHandler := content.NewHandler(contentService, logger)

	practiceService := practice.NewService(pool, contentStore)
	practiceHandler := practice.NewHandler(practiceService, logger)

	learningHandler := learning.NewHandler(pool, logger)

	aiClient := ai.NewClient(ai.Config{
		BaseURL: cfg.AIBaseURL, APIKey: cfg.AIAPIKey, Model: cfg.AIModel, Timeout: cfg.AITimeout,
	}, pool, logger)
	aiService := ai.NewService(pool, aiClient, logger)
	importService := imports.NewService(pool, contentService, aiClient, cfg.UploadDir, cfg.UploadMaxBytes, logger)

	// worker：生产环境独立进程；开发环境可通过 RUN_WORKER=true 内嵌运行
	if cfg.RunWorker {
		handlers := map[string]jobs.Handler{}
		for k, v := range aiService.Handlers() {
			handlers[k] = v
		}
		for k, v := range importService.Handlers() {
			handlers[k] = v
		}
		for k, v := range learningHandler.Handlers() {
			handlers[k] = v
		}
		worker := jobs.NewWorker(pool, cfg.WorkerID, cfg.WorkerConcurrency, handlers, logger)
		go worker.Run(ctx)
		logger.Info("embedded_worker_started", "worker_id", cfg.WorkerID)
	}

	// 学习端路由（authed）：部分路由公开，其余要求登录
	authed := http.NewServeMux()
	authed.HandleFunc("GET /api/v1/healthz", func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	authHandler.RegisterRoutes(authed)
	catalogHandler.RegisterRoutes(authed, nil)
	practiceHandler.RegisterRoutes(authed)
	learningHandler.RegisterRoutes(authed, nil)

	// 管理端路由（adminMux）：仅管理员
	adminMux := http.NewServeMux()
	contentHandler.RegisterRoutes(adminMux)
	imports.NewHandler(importService, logger).RegisterRoutes(adminMux)
	catalogHandler.RegisterRoutes(nil, adminMux)
	learningHandler.RegisterRoutes(nil, adminMux)

	validate := func(r *http.Request) (string, string, error) {
		u, ok := authService.ValidateSession(r.Context(), auth.Token(r))
		if !ok {
			return "", "", nil
		}
		return u.ID, string(u.Role), nil
	}
	requireAuth := func(next http.Handler) http.Handler {
		return httpapi.RequireAuth(validate, next)
	}

	rootMux := http.NewServeMux()
	// 公开路由：认证入口与健康检查
	rootMux.Handle("POST /api/v1/auth/register", authed)
	rootMux.Handle("POST /api/v1/auth/login", authed)
	rootMux.Handle("POST /api/v1/auth/logout", authed)
	rootMux.Handle("POST /api/v1/auth/password-reset/request", authed)
	rootMux.Handle("POST /api/v1/auth/password-reset/confirm", authed)
	rootMux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	rootMux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			httpapi.WriteError(w, r, httpapi.E(http.StatusServiceUnavailable, "not_ready", "数据库不可用"))
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	// 其余学习端路由要求登录
	rootMux.Handle("/api/v1/", requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAdminPath(r.URL.Path) {
			httpapi.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				adminMux.ServeHTTP(w, req)
			})).ServeHTTP(w, r)
			return
		}
		authed.ServeHTTP(w, r)
	})))

	var finalHandler http.Handler = rootMux
	finalHandler = httpapi.OriginCheck(cfg.PublicOrigin, devOrigins(cfg), finalHandler)
	finalHandler = httpapi.AccessLog(logger, finalHandler)
	finalHandler = httpapi.RequestIDMiddleware(finalHandler)
	finalHandler = httpapi.Recover(logger, finalHandler)
	return finalHandler
}

func isAdminPath(p string) bool {
	const prefix = "/api/v1/admin"
	return len(p) >= len(prefix) && p[:len(prefix)] == prefix
}

func devOrigins(cfg config.Config) []string {
	if cfg.AppEnv != "dev" {
		return nil
	}
	return []string{
		"http://localhost:5173", "http://127.0.0.1:5173",
		"http://localhost:5174", "http://127.0.0.1:5174",
	}
}
