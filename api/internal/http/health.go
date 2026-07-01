package httpx

import (
	"context"
	"encoding/json"
	"net/http"
)

type Readiness interface {
	Ready(ctx context.Context) (bool, string)
}

func HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

func ReadyHandler(r Readiness) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ok, reason := r.Ready(req.Context())
		if ok {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "detail": reason})
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "detail": reason})
	})
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
