package httpapi

import (
	"context"
	"net/http"

	"github.com/OcheOps/aws-calculator/api/internal/auth"
)

type ctxKey int

const ctxKeyUser ctxKey = iota

func WithSession(svc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err == nil && cookie.Value != "" {
				if user, err := svc.Session(r.Context(), cookie.Value); err == nil {
					ctx := context.WithValue(r.Context(), ctxKeyUser, user)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func UserFrom(ctx context.Context) (*auth.User, bool) {
	u, ok := ctx.Value(ctxKeyUser).(*auth.User)
	return u, ok
}

func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFrom(r.Context()); !ok {
			WriteError(w, http.StatusUnauthorized, "unauthorized", "sign in required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
