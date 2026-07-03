package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

type fakeIngestTrigger struct {
	called       bool
	providerCode string
}

func (f *fakeIngestTrigger) Trigger(providerCode string) (int64, error) {
	f.called = true
	f.providerCode = providerCode
	return 42, nil
}

func TestAdmin_RejectsMissingToken(t *testing.T) {
	trig := &fakeIngestTrigger{}
	r := chi.NewRouter()
	MountAdmin(r, "secret-token", trig)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, _ := http.Post(srv.URL+"/admin/ingest?provider=aws", "application/json", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if trig.called {
		t.Error("trigger should not have been called")
	}
}

func TestAdmin_TriggersIngestion(t *testing.T) {
	trig := &fakeIngestTrigger{}
	r := chi.NewRouter()
	MountAdmin(r, "secret-token", trig)
	srv := httptest.NewServer(r)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/admin/ingest?provider=aws", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("status = %d, want 202", resp.StatusCode)
	}
	if !trig.called || trig.providerCode != "aws" {
		t.Errorf("trigger = %+v", trig)
	}
}
