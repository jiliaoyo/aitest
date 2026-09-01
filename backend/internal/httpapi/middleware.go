package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aishuati/backend/internal/httpapi/ctxkeys"
)

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// RequestID 中间件为每个请求生成或透传 request_id，写入响应头与 context。
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(ctxkeys.WithRequestID(r.Context(), id)))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

// AccessLog 输出结构化访问日志：方法、路径、状态码、耗时、用户与请求 ID。
func AccessLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			logger.Info("http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"user_id", ctxkeys.UserID(r.Context()),
				"request_id", ctxkeys.RequestID(r.Context()),
			)
		}()
		next.ServeHTTP(rec, r)
	})
}

// Recover 把 handler panic 转成 500，不让进程退出。
func Recover(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic_recovered", "panic", rec, "path", r.URL.Path,
					"request_id", ctxkeys.RequestID(r.Context()))
				WriteError(w, r, ErrInternal)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// OriginCheck 对非 GET 请求校验 Origin，防御跨站请求伪造；无 Origin 的非浏览器客户端放行。
func OriginCheck(publicOrigin string, extraOrigins []string, next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range append([]string{publicOrigin}, extraOrigins...) {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			allowed[u.Scheme+"://"+u.Host] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" && !allowed[strings.TrimRight(origin, "/")] {
			WriteError(w, r, E(http.StatusForbidden, "csrf_rejected", "请求来源不被允许"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAuth 通过注入的校验函数恢复当前用户，避免 httpapi 依赖 auth 模块。
func RequireAuth(validate func(r *http.Request) (userID, role string, err error), next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, role, err := validate(r)
		if err != nil || userID == "" {
			WriteError(w, r, ErrUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctxkeys.WithUser(r.Context(), userID, role)))
	})
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ctxkeys.UserRole(r.Context()) != "admin" {
			WriteError(w, r, ErrForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
