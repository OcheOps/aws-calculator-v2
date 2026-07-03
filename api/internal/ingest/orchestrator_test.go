package ingest

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/OcheOps/aws-calculator/api/internal/db"
	"github.com/OcheOps/aws-calculator/api/internal/providers"
)

type recordingProvider struct {
	called int
	err    error
}

func (r *recordingProvider) Metadata() providers.Metadata {
	return providers.Metadata{Code: "aws", Name: "AWS"}
}

func (r *recordingProvider) Ingest(ctx context.Context, deps providers.IngestDeps) error {
	r.called++
	return r.err
}

func setupOrchestratorDB(t *testing.T) (context.Context, *Orchestrator, *recordingProvider) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	t.Cleanup(pool.Close)
	_, _ = pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
	if err := db.Migrate(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	prov := &recordingProvider{}
	orch := NewOrchestrator(pool)
	return ctx, orch, prov
}

func TestOrchestrator_SuccessRecordsSuccessRun(t *testing.T) {
	ctx, orch, prov := setupOrchestratorDB(t)
	if err := orch.Run(ctx, prov); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if prov.called != 1 {
		t.Errorf("provider called %d times, want 1", prov.called)
	}
	var status string
	orch.pool.QueryRow(ctx, `SELECT status FROM ingestion_runs ORDER BY id DESC LIMIT 1`).Scan(&status)
	if status != "success" {
		t.Errorf("status = %q, want success", status)
	}
}

func TestOrchestrator_FailureRecordsError(t *testing.T) {
	ctx, orch, prov := setupOrchestratorDB(t)
	prov.err = errors.New("boom")
	err := orch.Run(ctx, prov)
	if err == nil {
		t.Fatal("expected error")
	}
	var status, msg string
	orch.pool.QueryRow(ctx, `SELECT status, COALESCE(error,'') FROM ingestion_runs ORDER BY id DESC LIMIT 1`).Scan(&status, &msg)
	if status != "failed" || msg == "" {
		t.Errorf("status=%q msg=%q", status, msg)
	}
}
