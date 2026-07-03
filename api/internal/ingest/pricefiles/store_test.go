package pricefiles

import (
	"context"
	"os"
	"testing"

	"github.com/OcheOps/aws-calculator/api/internal/db"
)

func setup(t *testing.T) (*Store, context.Context) {
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
	return NewStore(pool), ctx
}

func TestStore_UpsertAndGet(t *testing.T) {
	s, ctx := setup(t)
	etag, ok, err := s.Get(ctx, "aws", "AmazonEC2", "us-east-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if ok || etag != "" {
		t.Fatalf("expected empty, got %q ok=%v", etag, ok)
	}

	if err := s.Upsert(ctx, Record{
		ProviderCode: "aws", ServiceCode: "AmazonEC2", RegionCode: "us-east-1",
		VersionID: "20260701", ETag: "\"abc123\"", URL: "https://example/file.json",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	etag, ok, err = s.Get(ctx, "aws", "AmazonEC2", "us-east-1")
	if err != nil || !ok || etag != "\"abc123\"" {
		t.Fatalf("get after upsert: etag=%q ok=%v err=%v", etag, ok, err)
	}

	// Overwrite
	if err := s.Upsert(ctx, Record{
		ProviderCode: "aws", ServiceCode: "AmazonEC2", RegionCode: "us-east-1",
		VersionID: "20260702", ETag: "\"def456\"", URL: "https://example/file2.json",
	}); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	etag, _, _ = s.Get(ctx, "aws", "AmazonEC2", "us-east-1")
	if etag != "\"def456\"" {
		t.Errorf("etag not updated: %q", etag)
	}

	if err := s.Delete(ctx, "aws", "AmazonEC2", "us-east-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok, _ := s.Get(ctx, "aws", "AmazonEC2", "us-east-1"); ok {
		t.Error("expected gone after delete")
	}
}
