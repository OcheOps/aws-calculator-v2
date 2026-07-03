package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteData_ProducesEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteData(rec, http.StatusOK, map[string]int{"n": 42}, nil)
	if rec.Code != 200 {
		t.Fatalf("code %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["data"]; !ok {
		t.Error("expected data field")
	}
}

func TestWriteError_ProducesEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusBadRequest, "bad_request", "missing field")
	var body struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Error.Code != "bad_request" || body.Error.Message != "missing field" {
		t.Errorf("got %+v", body)
	}
}
