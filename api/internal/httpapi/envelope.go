package httpapi

import (
	"encoding/json"
	"net/http"
	"reflect"
)

type Meta struct {
	NextCursor string `json:"next_cursor,omitempty"`
	Count      int    `json:"count,omitempty"`
}

type response struct {
	Data  any     `json:"data,omitempty"`
	Meta  *Meta   `json:"meta,omitempty"`
	Error *apiErr `json:"error,omitempty"`
}

type apiErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteData(w http.ResponseWriter, code int, data any, meta *Meta) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(response{Data: normalize(data), Meta: meta})
}

func WriteError(w http.ResponseWriter, code int, errCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(response{Error: &apiErr{Code: errCode, Message: message}})
}

// normalize converts a typed nil slice into an empty slice so clients never
// receive `null` where they expect an iterable.
func normalize(v any) any {
	if v == nil {
		return v
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice && rv.IsNil() {
		return reflect.MakeSlice(rv.Type(), 0, 0).Interface()
	}
	return v
}
