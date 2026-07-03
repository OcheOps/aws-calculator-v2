package httpapi

import (
	"net/http"

	"github.com/OcheOps/aws-calculator/api/internal/estimates"
	"github.com/go-chi/chi/v5"
)

func MountShared(r chi.Router, store *estimates.Store) {
	r.Get("/shared/{token}", func(w http.ResponseWriter, r *http.Request) {
		est, err := store.LookupByShare(r.Context(), chi.URLParam(r, "token"))
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "shared estimate not found")
			return
		}
		WriteData(w, http.StatusOK, est, nil)
	})
}
