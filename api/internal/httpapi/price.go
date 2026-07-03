package httpapi

import (
	"net/http"
	"strings"

	"github.com/OcheOps/aws-calculator/api/internal/pricing"
	"github.com/go-chi/chi/v5"
)

func MountPricing(r chi.Router, lookup *pricing.Lookup) {
	r.Get("/price", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		attrs := map[string]string{}
		for k, vs := range q {
			if strings.HasPrefix(k, "attr[") && strings.HasSuffix(k, "]") && len(vs) > 0 {
				name := k[len("attr[") : len(k)-1]
				attrs[name] = vs[0]
			}
		}
		in := pricing.LookupInput{
			Provider:    stringOr(q.Get("provider"), "aws"),
			ServiceCode: q.Get("service"),
			Region:      q.Get("region"),
			TermType:    stringOr(q.Get("term"), "OnDemand"),
			Attributes:  attrs,
			Currency:    stringOr(q.Get("currency"), "USD"),
		}
		if in.ServiceCode == "" || in.Region == "" {
			WriteError(w, http.StatusBadRequest, "bad_request", "service and region are required")
			return
		}
		res, err := lookup.Price(r.Context(), in)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		WriteData(w, http.StatusOK, res, nil)
	})
}

func stringOr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
