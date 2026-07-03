package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type IngestTrigger interface {
	Trigger(providerCode string) (int64, error)
}

func MountAdmin(r chi.Router, adminToken string, trig IngestTrigger) {
	r.Post("/admin/ingest", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		want := "Bearer " + adminToken
		if adminToken == "" || !strings.EqualFold(auth, want) {
			WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or missing admin token")
			return
		}
		provider := r.URL.Query().Get("provider")
		if provider == "" {
			provider = "aws"
		}
		runID, err := trig.Trigger(provider)
		if err != nil {
			WriteError(w, http.StatusConflict, "conflict", err.Error())
			return
		}
		WriteData(w, http.StatusAccepted, map[string]any{
			"provider": provider,
			"run_id":   runID,
		}, nil)
	})
}
