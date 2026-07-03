package httpapi

import (
	"net/http"

	"github.com/OcheOps/aws-calculator/api/internal/catalog"
	"github.com/go-chi/chi/v5"
)

var supportedCurrencies = []string{"USD", "EUR", "GBP", "NGN"}

func MountCatalog(r chi.Router, store *catalog.Store) {
	r.Get("/providers", func(w http.ResponseWriter, r *http.Request) {
		provs, err := store.ListProviders(r.Context())
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		WriteData(w, http.StatusOK, provs, &Meta{Count: len(provs)})
	})

	r.Get("/services", func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Query().Get("provider")
		if provider == "" {
			provider = "aws"
		}
		page := ParsePagination(r)
		svcs, next, err := store.ListServices(r.Context(), catalog.ListServicesQuery{
			ProviderCode: provider,
			Search:       r.URL.Query().Get("q"),
			Cursor:       page.Cursor,
			Limit:        page.Limit,
		})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		WriteData(w, http.StatusOK, svcs, &Meta{NextCursor: next, Count: len(svcs)})
	})

	r.Get("/services/{code}", func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Query().Get("provider")
		if provider == "" {
			provider = "aws"
		}
		code := chi.URLParam(r, "code")
		svc, err := store.GetServiceByCode(r.Context(), provider, code)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "service not found")
			return
		}
		dims, err := store.ListDimensions(r.Context(), svc.ID)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		WriteData(w, http.StatusOK, map[string]any{
			"service":    svc,
			"dimensions": dims,
		}, nil)
	})

	r.Get("/services/{code}/dimension-values", func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Query().Get("provider")
		if provider == "" {
			provider = "aws"
		}
		code := chi.URLParam(r, "code")
		dimKey := r.URL.Query().Get("dim")
		if dimKey == "" {
			WriteError(w, http.StatusBadRequest, "bad_request", "dim is required")
			return
		}
		svc, err := store.GetServiceByCode(r.Context(), provider, code)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "service not found")
			return
		}
		dims, err := store.ListDimensions(r.Context(), svc.ID)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		var dimID int64
		for _, d := range dims {
			if d.Key == dimKey {
				dimID = d.ID
				break
			}
		}
		if dimID == 0 {
			WriteError(w, http.StatusNotFound, "not_found", "dimension not found")
			return
		}
		vals, err := store.ListDimensionValues(r.Context(), dimID)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		WriteData(w, http.StatusOK, vals, &Meta{Count: len(vals)})
	})

	r.Get("/regions", func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Query().Get("provider")
		if provider == "" {
			provider = "aws"
		}
		regions, err := store.ListRegions(r.Context(), provider)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		WriteData(w, http.StatusOK, regions, &Meta{Count: len(regions)})
	})

	r.Get("/currencies", func(w http.ResponseWriter, r *http.Request) {
		WriteData(w, http.StatusOK, supportedCurrencies, &Meta{Count: len(supportedCurrencies)})
	})
}
