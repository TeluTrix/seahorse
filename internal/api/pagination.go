package api

import (
	"net/http"
	"strconv"
)

// parsePagination reads ?page= and ?page_size= from the request, applying
// h's configured default and max bounds.
func (h *Handlers) parsePagination(r *http.Request) (page, pageSize int) {
	page = 1
	if v, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && v > 0 {
		page = v
	}

	pageSize = h.ClientConfig.DefaultPageSize
	if v, err := strconv.Atoi(r.URL.Query().Get("page_size")); err == nil && v > 0 {
		pageSize = v
	}
	if pageSize > h.MaxPageSize {
		pageSize = h.MaxPageSize
	}

	return page, pageSize
}
