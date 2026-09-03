package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aishuati/backend/internal/httpapi/ctxkeys"
)

func TestOriginCheckRejectsUnexpectedWriteOrigin(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := OriginCheck("https://app.example", nil, next)
	bad := httptest.NewRequest(http.MethodPost, "/api/v1/me", nil)
	bad.Header.Set("Origin", "https://evil.example")
	badRec := httptest.NewRecorder()
	h.ServeHTTP(badRec, bad)
	if badRec.Code != http.StatusForbidden {
		t.Fatalf("unexpected origin should be rejected, got %d", badRec.Code)
	}

	good := httptest.NewRequest(http.MethodPost, "/api/v1/me", nil)
	good.Header.Set("Origin", "https://app.example/")
	goodRec := httptest.NewRecorder()
	h.ServeHTTP(goodRec, good)
	if goodRec.Code != http.StatusNoContent {
		t.Fatalf("configured origin should pass, got %d", goodRec.Code)
	}
}

func TestRequireAuthAndAdmin(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ctxkeys.UserRole(r.Context()) != "admin" {
			t.Errorf("expected admin role in context")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	h := RequireAuth(func(*http.Request) (string, string, error) { return "user-1", "admin", nil }, RequireAdmin(next))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/questions", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("admin should pass auth and permission checks, got %d", rec.Code)
	}

	unauthenticated := RequireAuth(func(*http.Request) (string, string, error) { return "", "", nil }, next)
	unauthRec := httptest.NewRecorder()
	unauthenticated.ServeHTTP(unauthRec, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("missing session should be rejected, got %d", unauthRec.Code)
	}

	learner := RequireAdmin(next)
	learnerRec := httptest.NewRecorder()
	learner.ServeHTTP(learnerRec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/questions", nil).WithContext(ctxkeys.WithUser(context.Background(), "user-1", "learner")))
	if learnerRec.Code != http.StatusForbidden {
		t.Fatalf("learner should be rejected by admin middleware, got %d", learnerRec.Code)
	}
}
