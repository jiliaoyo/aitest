package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aishuati/backend/internal/config"
	"github.com/aishuati/backend/internal/httpapi"
)

func TestDevOriginsAllowViteFallbackPort(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	h := httpapi.OriginCheck("http://localhost:5173", devOrigins(config.Config{AppEnv: "dev"}), next)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", nil)
	req.Header.Set("Origin", "http://localhost:5174")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("dev Vite fallback origin should be allowed, got %d", resp.Code)
	}
}
