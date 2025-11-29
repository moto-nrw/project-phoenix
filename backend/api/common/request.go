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

// ParsePaginationWithDefaults extracts page and page_size from query parameters
// with custom default values.
func ParsePaginationWithDefaults(r *http.Request, defaultPage, defaultPageSize int) (page int, pageSize int) {
	page = defaultPage
	pageSize = defaultPageSize

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

// ParseQueryInt extracts an integer from a query parameter.
// Returns the default value if the parameter is not provided or invalid.
func ParseQueryInt(r *http.Request, param string, defaultValue int) int {
	if str := r.URL.Query().Get(param); str != "" {
		if val, err := strconv.Atoi(str); err == nil {
			return val
		}
	}
	return defaultValue
}

// ParseQueryInt64 extracts an int64 from a query parameter.
// Returns the default value if the parameter is not provided or invalid.
func ParseQueryInt64(r *http.Request, param string, defaultValue int64) int64 {
	if str := r.URL.Query().Get(param); str != "" {
		if val, err := strconv.ParseInt(str, 10, 64); err == nil {
			return val
		}
	}
	return defaultValue
}

// ParseQueryBool extracts a boolean from a query parameter.
// Returns the default value if the parameter is not provided or invalid.
func ParseQueryBool(r *http.Request, param string, defaultValue bool) bool {
	if str := r.URL.Query().Get(param); str != "" {
		if val, err := strconv.ParseBool(str); err == nil {
			return val
		}
	}
	return defaultValue
}
