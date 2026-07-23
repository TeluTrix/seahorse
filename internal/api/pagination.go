package api

import (
	"net/http"
	"strconv"
)

const (
	defaultPageSize = 48
	maxPageSize     = 200
)

// parsePagination reads ?page= and ?page_size= from the request, applying
// sane defaults and bounds.
func parsePagination(r *http.Request) (page, pageSize int) {
	page = 1
	if v, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && v > 0 {
		page = v
	}

	pageSize = defaultPageSize
	if v, err := strconv.Atoi(r.URL.Query().Get("page_size")); err == nil && v > 0 {
		pageSize = v
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	return page, pageSize
}
