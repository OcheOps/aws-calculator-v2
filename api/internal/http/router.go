package httpx

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/OcheOps/aws-calculator/api/internal/catalog"
	"github.com/OcheOps/aws-calculator/api/internal/httpapi"
	"github.com/go-chi/chi/v5"
)

type Deps struct {
	Ready      Readiness
	Logger     *slog.Logger
	Catalog    *catalog.Store
	AdminToken string
	Ingest     httpapi.IngestTrigger
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
		if d.Catalog != nil {
			httpapi.MountCatalog(r, d.Catalog)
		}
		if d.Ingest != nil {
			httpapi.MountAdmin(r, d.AdminToken, d.Ingest)
		}
	})

	return r
}

// CatalogReadiness marks the API ready only once the catalog store reports a
// successful ingestion.
type CatalogReadiness struct {
	Store *catalog.Store
}

func (c CatalogReadiness) Ready(ctx context.Context) (bool, string) {
	ok, err := c.Store.HasSuccessfulIngestion(ctx)
	if err != nil {
		return false, "catalog check failed: " + err.Error()
	}
	if !ok {
		return false, "catalog not populated yet"
	}
	return true, "ok"
}
