package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shaunzlim0123/llm-inference-gateway/internal/config"
)

func TestAuth_ValidAPIKey(t *testing.T) {
	tenants := []config.TenantConfig{
		{ID: "tenant-test", APIKey: "test-key-123", Name: "Test"},
	}

	handler := Auth(tenants)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant := GetTenant(r.Context())
		if tenant == nil {
			t.Fatal("expected tenant in context")
		}
		if tenant.ID != "tenant-test" {
			t.Errorf("expected tenant-test, got %s", tenant.ID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "test-key-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAuth_BearerToken(t *testing.T) {
	tenants := []config.TenantConfig{
		{ID: "tenant-test", APIKey: "test-key-123", Name: "Test"},
	}

	handler := Auth(tenants)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer test-key-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAuth_MissingKey(t *testing.T) {
	handler := Auth(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_InvalidKey(t *testing.T) {
	tenants := []config.TenantConfig{
		{ID: "tenant-test", APIKey: "test-key-123", Name: "Test"},
	}

	handler := Auth(tenants)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
