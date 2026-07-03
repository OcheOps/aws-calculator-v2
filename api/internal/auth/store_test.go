package auth

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

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

func TestStore_UpsertUserIsIdempotent(t *testing.T) {
	s, ctx := setup(t)
	u1, err := s.UpsertUser(ctx, "Alice@Example.COM")
	if err != nil {
		t.Fatalf("upsert1: %v", err)
	}
	u2, err := s.UpsertUser(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("upsert2: %v", err)
	}
	if u1.ID != u2.ID {
		t.Errorf("different user IDs (%d vs %d)", u1.ID, u2.ID)
	}
	if u1.Email != "alice@example.com" {
		t.Errorf("email = %q", u1.Email)
	}
}

func TestStore_MagicLinkFlow(t *testing.T) {
	s, ctx := setup(t)
	u, _ := s.UpsertUser(ctx, "b@x.com")
	raw := "raw-token"
	if err := s.SaveMagicLink(ctx, u.ID, HashToken(raw), time.Now().Add(15*time.Minute), "1.2.3.4", "ua"); err != nil {
		t.Fatalf("save: %v", err)
	}
	uid, err := s.ConsumeMagicLink(ctx, HashToken(raw))
	if err != nil || uid != u.ID {
		t.Fatalf("consume: uid=%d err=%v", uid, err)
	}
	if _, err := s.ConsumeMagicLink(ctx, HashToken(raw)); err == nil {
		t.Error("expected second consume to fail")
	}
}

func TestStore_EnsureDefaultOrg(t *testing.T) {
	s, ctx := setup(t)
	u, _ := s.UpsertUser(ctx, "c@d.com")
	org, err := s.EnsureDefaultOrg(ctx, u)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if !strings.HasPrefix(org.Slug, "c") {
		t.Errorf("slug=%q", org.Slug)
	}
	org2, _ := s.EnsureDefaultOrg(ctx, u)
	if org2.ID != org.ID {
		t.Errorf("ensure not idempotent")
	}
}
