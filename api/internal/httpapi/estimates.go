package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/OcheOps/aws-calculator/api/internal/auth"
	"github.com/OcheOps/aws-calculator/api/internal/estimates"
	"github.com/OcheOps/aws-calculator/api/internal/export"
	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
)

type EstimatesDeps struct {
	Store     *estimates.Store
	Repricer  *estimates.Repricer
	AuthStore *auth.Store
	BaseURL   string
}

func MountEstimates(r chi.Router, d EstimatesDeps) {
	r.Post("/estimates", createEstimate(d))
	r.Group(func(r chi.Router) {
		r.Use(RequireUser)
		r.Get("/estimates", listEstimates(d))
	})
	r.Route("/estimates/{id}", func(r chi.Router) {
		r.Get("/", getEstimate(d))
		r.Patch("/", updateEstimate(d))
		r.Delete("/", deleteEstimate(d))
		r.Post("/items", addItem(d))
		r.Delete("/items/{item_id}", deleteItem(d))
		r.Post("/reprice", reprice(d))
		r.Post("/share", shareEstimate(d))
		r.Get("/export", exportEstimate(d))
	})
}

func createEstimate(d EstimatesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name     string `json:"name"`
			Currency string `json:"currency"`
			Notes    string `json:"notes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			WriteError(w, http.StatusBadRequest, "bad_request", "name is required")
			return
		}
		if body.Currency == "" {
			body.Currency = "USD"
		}
		in := estimates.CreateInput{Name: body.Name, Currency: body.Currency, Notes: body.Notes}
		if u, ok := UserFrom(r.Context()); ok {
			org, err := d.AuthStore.EnsureDefaultOrg(r.Context(), u)
			if err != nil {
				WriteError(w, http.StatusInternalServerError, "internal", err.Error())
				return
			}
			in.OrgID = &org.ID
			in.CreatedBy = &u.ID
		}
		est, editToken, err := d.Store.Create(r.Context(), in)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		resp := map[string]any{"estimate": est}
		if editToken != "" {
			resp["edit_token"] = editToken
		}
		WriteData(w, http.StatusCreated, resp, nil)
	}
}

func listEstimates(d EstimatesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, _ := UserFrom(r.Context())
		org, err := d.AuthStore.EnsureDefaultOrg(r.Context(), u)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		list, err := d.Store.List(r.Context(), org.ID)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		WriteData(w, http.StatusOK, list, &Meta{Count: len(list)})
	}
}

func getEstimate(d EstimatesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		est, err := d.Store.Get(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "estimate not found")
			return
		}
		if !authorizeAccess(r, est, d) {
			WriteError(w, http.StatusForbidden, "forbidden", "access denied")
			return
		}
		WriteData(w, http.StatusOK, est, nil)
	}
}

func updateEstimate(d EstimatesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		est, err := d.Store.Get(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "estimate not found")
			return
		}
		if !authorizeAccess(r, est, d) {
			WriteError(w, http.StatusForbidden, "forbidden", "access denied")
			return
		}
		var body struct {
			Name     *string `json:"name,omitempty"`
			Currency *string `json:"currency,omitempty"`
			Notes    *string `json:"notes,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if err := d.Store.Update(r.Context(), id, estimates.UpdatePatch{Name: body.Name, Currency: body.Currency, Notes: body.Notes}); err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		updated, _ := d.Store.Get(r.Context(), id)
		WriteData(w, http.StatusOK, updated, nil)
	}
}

func deleteEstimate(d EstimatesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		est, err := d.Store.Get(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "estimate not found")
			return
		}
		if !authorizeAccess(r, est, d) {
			WriteError(w, http.StatusForbidden, "forbidden", "access denied")
			return
		}
		if err := d.Store.Delete(r.Context(), id); err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		WriteData(w, http.StatusOK, map[string]bool{"deleted": true}, nil)
	}
}

func addItem(d EstimatesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		est, err := d.Store.Get(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "estimate not found")
			return
		}
		if !authorizeAccess(r, est, d) {
			WriteError(w, http.StatusForbidden, "forbidden", "access denied")
			return
		}
		var body struct {
			ServiceID  int64             `json:"service_id"`
			RegionCode string            `json:"region_code"`
			SKU        string            `json:"sku"`
			Quantity   string            `json:"quantity"`
			Attributes map[string]string `json:"attributes"`
			UnitPrice  string            `json:"unit_price"`
			Unit       string            `json:"unit"`
			Currency   string            `json:"currency"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		qty, _ := decimal.NewFromString(body.Quantity)
		price, _ := decimal.NewFromString(body.UnitPrice)
		item, err := d.Store.AddItem(r.Context(), id, estimates.AddItemInput{
			ServiceID: body.ServiceID, RegionCode: body.RegionCode, SKU: body.SKU,
			Quantity: qty, Attributes: body.Attributes,
			UnitPrice: price, Unit: body.Unit, Currency: body.Currency,
		})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		WriteData(w, http.StatusCreated, item, nil)
	}
}

func deleteItem(d EstimatesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		itemID, _ := strconv.ParseInt(chi.URLParam(r, "item_id"), 10, 64)
		est, err := d.Store.Get(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "estimate not found")
			return
		}
		if !authorizeAccess(r, est, d) {
			WriteError(w, http.StatusForbidden, "forbidden", "access denied")
			return
		}
		if err := d.Store.DeleteItem(r.Context(), id, itemID); err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		WriteData(w, http.StatusOK, map[string]bool{"deleted": true}, nil)
	}
}

func reprice(d EstimatesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		est, err := d.Store.Get(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "estimate not found")
			return
		}
		if !authorizeAccess(r, est, d) {
			WriteError(w, http.StatusForbidden, "forbidden", "access denied")
			return
		}
		deltas, err := d.Repricer.Preview(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		if r.URL.Query().Get("apply") == "true" {
			if err := d.Repricer.Apply(r.Context(), id, deltas); err != nil {
				WriteError(w, http.StatusInternalServerError, "internal", err.Error())
				return
			}
		}
		WriteData(w, http.StatusOK, deltas, nil)
	}
}

func shareEstimate(d EstimatesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		est, err := d.Store.Get(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "estimate not found")
			return
		}
		if !authorizeAccess(r, est, d) {
			WriteError(w, http.StatusForbidden, "forbidden", "access denied")
			return
		}
		raw, err := d.Store.CreateShare(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		url := fmt.Sprintf("%s/s/%s", d.BaseURL, raw)
		WriteData(w, http.StatusOK, map[string]string{"share_url": url, "share_token": raw}, nil)
	}
}

func exportEstimate(d EstimatesDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		est, err := d.Store.Get(r.Context(), id)
		if err != nil {
			WriteError(w, http.StatusNotFound, "not_found", "estimate not found")
			return
		}
		if !authorizeAccess(r, est, d) {
			WriteError(w, http.StatusForbidden, "forbidden", "access denied")
			return
		}
		format := r.URL.Query().Get("format")
		switch format {
		case "csv":
			w.Header().Set("Content-Type", "text/csv")
			w.Header().Set("Content-Disposition", `attachment; filename="estimate.csv"`)
			_ = export.WriteCSV(w, est)
		case "json":
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Disposition", `attachment; filename="estimate.json"`)
			_ = export.WriteJSON(w, est)
		case "pdf":
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", `attachment; filename="estimate.pdf"`)
			_ = export.WritePDF(w, est)
		default:
			WriteError(w, http.StatusBadRequest, "bad_request", "format must be pdf, csv, or json")
		}
	}
}

func authorizeAccess(r *http.Request, est *estimates.Estimate, d EstimatesDeps) bool {
	if est.OrgID == nil {
		return d.Store.MatchesEditToken(r.Context(), est.ID, r.Header.Get("X-Edit-Token"))
	}
	u, isAuthed := UserFrom(r.Context())
	if !isAuthed {
		return false
	}
	orgs, err := d.AuthStore.GetOrgsForUser(r.Context(), u.ID)
	if err != nil {
		return false
	}
	for _, o := range orgs {
		if o.ID == *est.OrgID {
			return true
		}
	}
	return false
}
