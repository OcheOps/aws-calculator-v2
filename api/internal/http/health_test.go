package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeReadiness struct{ ready bool }

func (f fakeReadiness) Ready(ctx context.Context) (bool, string) {
	if f.ready {
		return true, "ok"
	}
	return false, "catalog empty"
}

func TestHealthz_AlwaysOK(t *testing.T) {
	h := HealthHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want ok", body["status"])
	}
}

func TestReadyz_ReturnsReadyState(t *testing.T) {
	h := ReadyHandler(fakeReadiness{ready: true})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ready: status = %d, want 200", rec.Code)
	}
}

func TestReadyz_ReturnsNotReady(t *testing.T) {
	h := ReadyHandler(fakeReadiness{ready: false})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("not ready: status = %d, want 503", rec.Code)
	}
}
