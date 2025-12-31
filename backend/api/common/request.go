package common

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// DefaultPage is the default page number for pagination
const DefaultPage = 1

// DefaultPageSize is the default page size for pagination
const DefaultPageSize = 50

// ParsePagination extracts page and page_size from query parameters.
// Returns default values (page=1, pageSize=50) if not provided or invalid.
func ParsePagination(r *http.Request) (page int, pageSize int) {
	page = DefaultPage
	pageSize = DefaultPageSize

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	return page, pageSize
}

// ParseIDParam extracts an int64 ID from a URL parameter.
// Returns the ID and any parsing error.
func ParseIDParam(r *http.Request, param string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, param), 10, 64)
}

// ParseID is a convenience function for parsing the common "id" URL parameter.
func ParseID(r *http.Request) (int64, error) {
	return ParseIDParam(r, "id")
}
