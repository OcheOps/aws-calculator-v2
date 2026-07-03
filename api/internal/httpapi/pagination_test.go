package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestParsePagination_Defaults(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	p := ParsePagination(req)
	if p.Limit != 50 || p.Cursor != "" {
		t.Errorf("got %+v", p)
	}
}

func TestParsePagination_ClampsLimit(t *testing.T) {
	req := httptest.NewRequest("GET", "/?limit=1000", nil)
	p := ParsePagination(req)
	if p.Limit != 200 {
		t.Errorf("limit = %d, want 200", p.Limit)
	}
}

func TestParsePagination_ReadsCursor(t *testing.T) {
	req := httptest.NewRequest("GET", "/?cursor=abc&limit=10", nil)
	p := ParsePagination(req)
	if p.Cursor != "abc" || p.Limit != 10 {
		t.Errorf("got %+v", p)
	}
}
