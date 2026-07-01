package httpx

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Deps struct {
	Ready  Readiness
	Logger *slog.Logger
}

func NewRouter(d Deps) http.Handler {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	r := chi.NewRouter()
	r.Use(RequestID)
	r.Use(Recoverer)
	r.Use(AccessLog(d.Logger))

	r.Get("/healthz", HealthHandler().ServeHTTP)
	r.Get("/readyz", ReadyHandler(d.Ready).ServeHTTP)

	r.Route("/api/v1", func(r chi.Router) {
		// Filled in by later plans.
	})

	return r
}
