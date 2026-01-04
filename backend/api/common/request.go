package common

import (
	"errors"
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

// ParseIntIDWithError parses an int ID from a URL parameter and renders an error if parsing fails.
// Returns the parsed ID and true if successful, or 0 and false if parsing failed (error already rendered).
// This helper reduces code duplication for the common pattern of parsing IDs and handling errors.
func ParseIntIDWithError(w http.ResponseWriter, r *http.Request, param string, errMsg string) (int, bool) {
	idStr := chi.URLParam(r, param)
	id, err := strconv.Atoi(idStr)
	if err != nil {
		RenderError(w, r, ErrorInvalidRequest(errors.New(errMsg)))
		return 0, false
	}
	return id, true
}

// ParseInt64IDWithError parses an int64 ID from a URL parameter and renders an error if parsing fails.
// Returns the parsed ID and true if successful, or 0 and false if parsing failed (error already rendered).
// Use this for APIs that work with int64 IDs (most domain entities).
func ParseInt64IDWithError(w http.ResponseWriter, r *http.Request, param string, errMsg string) (int64, bool) {
	id, err := ParseIDParam(r, param)
	if err != nil {
		RenderError(w, r, ErrorInvalidRequest(errors.New(errMsg)))
		return 0, false
	}
	return id, true
}
