package httpapi

import (
	"net/http"
	"strconv"
)

type Pagination struct {
	Cursor string
	Limit  int
}

func ParsePagination(r *http.Request) Pagination {
	p := Pagination{Cursor: r.URL.Query().Get("cursor"), Limit: 50}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			p.Limit = n
		}
	}
	if p.Limit > 200 {
		p.Limit = 200
	}
	return p
}
