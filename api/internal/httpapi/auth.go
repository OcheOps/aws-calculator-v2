package httpapi

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/OcheOps/aws-calculator/api/internal/auth"
	"github.com/go-chi/chi/v5"
)

type AuthOptions struct {
	SecureCookie bool
	CookieDomain string
}

func MountAuth(r chi.Router, svc *auth.Service, opts AuthOptions) {
	r.Post("/auth/request-link", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
			WriteError(w, http.StatusBadRequest, "bad_request", "email is required")
			return
		}
		if err := svc.RequestLink(r.Context(), body.Email, clientIP(r), r.UserAgent()); err != nil {
			// Never surface the reason to the client (avoids user enumeration + info leak),
			// but log it so operators can see SMTP / rate-limit failures.
			slog.Warn("request-link failed", "err", err, "email", body.Email)
		}
		WriteData(w, http.StatusAccepted, map[string]bool{"accepted": true}, nil)
	})

	r.Post("/auth/verify", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Token       string   `json:"token"`
			ClaimTokens []string `json:"claim_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Token == "" {
			WriteError(w, http.StatusBadRequest, "bad_request", "token is required")
			return
		}
		user, sessionRaw, exp, err := svc.Verify(r.Context(), body.Token, body.ClaimTokens)
		if err != nil {
			WriteError(w, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    sessionRaw,
			Path:     "/",
			Domain:   opts.CookieDomain,
			Expires:  exp,
			HttpOnly: true,
			Secure:   opts.SecureCookie,
			SameSite: http.SameSiteLaxMode,
		})
		WriteData(w, http.StatusOK, map[string]any{
			"user":       user,
			"expires_at": exp.UTC().Format(time.RFC3339),
		}, nil)
	})

	r.Post("/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("session"); err == nil {
			_ = svc.Logout(r.Context(), c.Value)
		}
		http.SetCookie(w, &http.Cookie{
			Name: "session", Value: "", Path: "/", MaxAge: -1,
			HttpOnly: true, Secure: opts.SecureCookie, SameSite: http.SameSiteLaxMode,
		})
		WriteData(w, http.StatusOK, map[string]bool{"ok": true}, nil)
	})

	r.Get("/auth/me", func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFrom(r.Context())
		if !ok {
			WriteError(w, http.StatusUnauthorized, "unauthorized", "not signed in")
			return
		}
		WriteData(w, http.StatusOK, u, nil)
	})
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		// X-Forwarded-For can be a comma-separated chain; take the leftmost.
		if i := strings.Index(v, ","); i > 0 {
			v = v[:i]
		}
		return strings.TrimSpace(v)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
