package auth

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OcheOps/aws-calculator/api/internal/db"
	"github.com/OcheOps/aws-calculator/api/internal/mail"
)

func setupService(t *testing.T) (*Service, *mail.FakeMailer, context.Context) {
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
	mailer := mail.NewFake()
	svc := NewService(NewStore(pool), mailer, NewLimiter(5, time.Hour), NewLimiter(5, time.Hour), "https://calc.example")
	return svc, mailer, ctx
}

func TestService_RequestLinkSendsEmail(t *testing.T) {
	svc, mailer, ctx := setupService(t)
	if err := svc.RequestLink(ctx, "x@y.com", "1.1.1.1", "ua"); err != nil {
		t.Fatalf("request: %v", err)
	}
	if len(mailer.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mailer.Messages))
	}
	if !strings.Contains(mailer.Messages[0].Body, "https://calc.example/auth/verify?token=") {
		t.Errorf("body missing link: %s", mailer.Messages[0].Body)
	}
}

func TestService_VerifyIssuesSession(t *testing.T) {
	svc, mailer, ctx := setupService(t)
	svc.RequestLink(ctx, "x@y.com", "1.1.1.1", "ua")
	link := mailer.Messages[0].Body
	i := strings.Index(link, "token=")
	if i == -1 {
		t.Fatal("no token in link")
	}
	token := link[i+len("token="):]
	if j := strings.IndexAny(token, "\" '<"); j != -1 {
		token = token[:j]
	}
	user, sessionToken, _, err := svc.Verify(ctx, token, nil)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if user.Email != "x@y.com" || sessionToken == "" {
		t.Errorf("user=%+v sess=%q", user, sessionToken)
	}
}

func TestService_RateLimitedRequestLink(t *testing.T) {
	svc, _, ctx := setupService(t)
	for i := 0; i < 5; i++ {
		if err := svc.RequestLink(ctx, "z@y.com", "1.1.1.1", "ua"); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if err := svc.RequestLink(ctx, "z@y.com", "1.1.1.1", "ua"); err == nil {
		t.Error("expected rate limit error")
	}
}
