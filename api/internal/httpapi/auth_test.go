package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OcheOps/aws-calculator/api/internal/auth"
	"github.com/OcheOps/aws-calculator/api/internal/db"
	"github.com/OcheOps/aws-calculator/api/internal/mail"
	"github.com/go-chi/chi/v5"
)

func setupAuthHandlers(t *testing.T) (*httptest.Server, *mail.FakeMailer) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, _ := db.Open(ctx, url)
	t.Cleanup(pool.Close)
	pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
	db.Migrate(url)
	fake := mail.NewFake()
	svc := auth.NewService(auth.NewStore(pool), fake, auth.NewLimiter(10, time.Hour), auth.NewLimiter(10, time.Hour), "https://calc.example")
	r := chi.NewRouter()
	r.Use(WithSession(svc))
	MountAuth(r, svc, AuthOptions{SecureCookie: false})
	return httptest.NewServer(r), fake
}

func TestAuthEndToEnd(t *testing.T) {
	srv, fake := setupAuthHandlers(t)
	defer srv.Close()
	c := &http.Client{}

	body, _ := json.Marshal(map[string]string{"email": "e2e@x.com"})
	resp, _ := c.Post(srv.URL+"/auth/request-link", "application/json", bytes.NewReader(body))
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("request-link status = %d", resp.StatusCode)
	}
	msg := fake.Messages[0].Body
	i := strings.Index(msg, "token=")
	token := msg[i+len("token="):]
	if j := strings.IndexAny(token, `" `); j != -1 {
		token = token[:j]
	}
	body, _ = json.Marshal(map[string]string{"token": token})
	resp, _ = c.Post(srv.URL+"/auth/verify", "application/json", bytes.NewReader(body))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify status = %d", resp.StatusCode)
	}
	var session string
	for _, ck := range resp.Cookies() {
		if ck.Name == "session" {
			session = ck.Value
		}
	}
	if session == "" {
		t.Fatal("no session cookie")
	}
	req, _ := http.NewRequest("GET", srv.URL+"/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: session})
	resp, _ = c.Do(req)
	if resp.StatusCode != 200 {
		t.Fatalf("me status = %d", resp.StatusCode)
	}
	var me struct{ Data auth.User }
	json.NewDecoder(resp.Body).Decode(&me)
	if me.Data.Email != "e2e@x.com" {
		t.Errorf("me = %+v", me.Data)
	}
	req, _ = http.NewRequest("POST", srv.URL+"/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: session})
	resp, _ = c.Do(req)
	if resp.StatusCode != 200 {
		t.Fatalf("logout status = %d", resp.StatusCode)
	}
}
