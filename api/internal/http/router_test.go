package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type alwaysReady struct{}

func (alwaysReady) Ready(ctx context.Context) (bool, string) { return true, "ok" }

func TestRouter_ExposesHealthAndReady(t *testing.T) {
	r := NewRouter(Deps{Ready: alwaysReady{}})
	srv := httptest.NewServer(r)
	defer srv.Close()

	for _, path := range []string{"/healthz", "/readyz"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", path, resp.StatusCode)
		}
	}
}

func TestRouter_AttachesRequestIDHeader(t *testing.T) {
	r := NewRouter(Deps{Ready: alwaysReady{}})
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("X-Request-Id") == "" {
		t.Error("expected X-Request-Id header")
	}
}
