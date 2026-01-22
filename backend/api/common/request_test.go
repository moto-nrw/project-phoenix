package common_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// ParsePagination Tests
// =============================================================================

func TestParsePagination_DefaultValues(t *testing.T) {
	r := httptest.NewRequest("GET", "/test", nil)

	page, pageSize := common.ParsePagination(r)

	assert.Equal(t, common.DefaultPage, page)
	assert.Equal(t, common.DefaultPageSize, pageSize)
}

func TestParsePagination_ValidParams(t *testing.T) {
	r := httptest.NewRequest("GET", "/test?page=3&page_size=25", nil)

	page, pageSize := common.ParsePagination(r)

	assert.Equal(t, 3, page)
	assert.Equal(t, 25, pageSize)
}

func TestParsePagination_InvalidPage(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		expectedPage int
	}{
		{"non-numeric page", "/test?page=abc", common.DefaultPage},
		{"zero page", "/test?page=0", common.DefaultPage},
		{"negative page", "/test?page=-1", common.DefaultPage},
		{"empty page", "/test?page=", common.DefaultPage},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", tc.query, nil)
			page, _ := common.ParsePagination(r)
			assert.Equal(t, tc.expectedPage, page)
		})
	}
}

func TestParsePagination_InvalidPageSize(t *testing.T) {
	tests := []struct {
		name             string
		query            string
		expectedPageSize int
	}{
		{"non-numeric page_size", "/test?page_size=abc", common.DefaultPageSize},
		{"zero page_size", "/test?page_size=0", common.DefaultPageSize},
		{"negative page_size", "/test?page_size=-5", common.DefaultPageSize},
		{"empty page_size", "/test?page_size=", common.DefaultPageSize},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", tc.query, nil)
			_, pageSize := common.ParsePagination(r)
			assert.Equal(t, tc.expectedPageSize, pageSize)
		})
	}
}

func TestParsePagination_MixedValidInvalid(t *testing.T) {
	tests := []struct {
		name             string
		query            string
		expectedPage     int
		expectedPageSize int
	}{
		{"valid page, invalid page_size", "/test?page=5&page_size=abc", 5, common.DefaultPageSize},
		{"invalid page, valid page_size", "/test?page=abc&page_size=10", common.DefaultPage, 10},
		{"only page", "/test?page=7", 7, common.DefaultPageSize},
		{"only page_size", "/test?page_size=100", common.DefaultPage, 100},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", tc.query, nil)
			page, pageSize := common.ParsePagination(r)
			assert.Equal(t, tc.expectedPage, page)
			assert.Equal(t, tc.expectedPageSize, pageSize)
		})
	}
}

func TestParsePagination_LargeValues(t *testing.T) {
	r := httptest.NewRequest("GET", "/test?page=9999&page_size=1000", nil)

	page, pageSize := common.ParsePagination(r)

	assert.Equal(t, 9999, page)
	assert.Equal(t, 1000, pageSize)
}

// =============================================================================
// ParseIDParam Tests
// =============================================================================

// chiRouteContext creates a request with chi URL params
func chiRouteContext(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestParseIDParam_ValidID(t *testing.T) {
	r := httptest.NewRequest("GET", "/test/123", nil)
	r = chiRouteContext(r, map[string]string{"id": "123"})

	id, err := common.ParseIDParam(r, "id")

	require.NoError(t, err)
	assert.Equal(t, int64(123), id)
}

func TestParseIDParam_LargeID(t *testing.T) {
	r := httptest.NewRequest("GET", "/test/9223372036854775807", nil)
	r = chiRouteContext(r, map[string]string{"id": "9223372036854775807"})

	id, err := common.ParseIDParam(r, "id")

	require.NoError(t, err)
	assert.Equal(t, int64(9223372036854775807), id)
}

func TestParseIDParam_InvalidID(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"non-numeric", "abc"},
		{"empty string", ""},
		{"special chars", "!@#"},
		{"float", "1.5"},
		{"overflow", "9223372036854775808"}, // int64 max + 1
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/test/"+tc.value, nil)
			r = chiRouteContext(r, map[string]string{"id": tc.value})

			_, err := common.ParseIDParam(r, "id")

			assert.Error(t, err)
		})
	}
}

func TestParseIDParam_DifferentParamNames(t *testing.T) {
	params := map[string]string{
		"group_id":   "456",
		"student_id": "789",
		"room_id":    "101",
	}

	r := httptest.NewRequest("GET", "/test", nil)
	r = chiRouteContext(r, params)

	for name, expected := range map[string]int64{
		"group_id":   456,
		"student_id": 789,
		"room_id":    101,
	} {
		t.Run(name, func(t *testing.T) {
			id, err := common.ParseIDParam(r, name)
			require.NoError(t, err)
			assert.Equal(t, expected, id)
		})
	}
}

// =============================================================================
// ParseID Tests
// =============================================================================

func TestParseID_ValidID(t *testing.T) {
	r := httptest.NewRequest("GET", "/test/555", nil)
	r = chiRouteContext(r, map[string]string{"id": "555"})

	id, err := common.ParseID(r)

	require.NoError(t, err)
	assert.Equal(t, int64(555), id)
}

func TestParseID_InvalidID(t *testing.T) {
	r := httptest.NewRequest("GET", "/test/invalid", nil)
	r = chiRouteContext(r, map[string]string{"id": "invalid"})

	_, err := common.ParseID(r)

	assert.Error(t, err)
}

// =============================================================================
// ParseIntIDWithError Tests
// =============================================================================

func TestParseIntIDWithError_ValidID(t *testing.T) {
	r := httptest.NewRequest("GET", "/test/42", nil)
	r = chiRouteContext(r, map[string]string{"id": "42"})
	w := httptest.NewRecorder()

	id, ok := common.ParseIntIDWithError(w, r, "id", "invalid ID")

	assert.True(t, ok)
	assert.Equal(t, 42, id)
	assert.Equal(t, http.StatusOK, w.Code) // No error rendered
}

func TestParseIntIDWithError_InvalidID(t *testing.T) {
	r := httptest.NewRequest("GET", "/test/invalid", nil)
	r = chiRouteContext(r, map[string]string{"id": "invalid"})
	w := httptest.NewRecorder()

	id, ok := common.ParseIntIDWithError(w, r, "id", "invalid ID")

	assert.False(t, ok)
	assert.Equal(t, 0, id)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseIntIDWithError_CustomErrorMessage(t *testing.T) {
	r := httptest.NewRequest("GET", "/test/abc", nil)
	r = chiRouteContext(r, map[string]string{"group_id": "abc"})
	w := httptest.NewRecorder()

	_, ok := common.ParseIntIDWithError(w, r, "group_id", "invalid group ID")

	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid group ID")
}

// =============================================================================
// ParseInt64IDWithError Tests
// =============================================================================

func TestParseInt64IDWithError_ValidID(t *testing.T) {
	r := httptest.NewRequest("GET", "/test/123456789012", nil)
	r = chiRouteContext(r, map[string]string{"id": "123456789012"})
	w := httptest.NewRecorder()

	id, ok := common.ParseInt64IDWithError(w, r, "id", "invalid ID")

	assert.True(t, ok)
	assert.Equal(t, int64(123456789012), id)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestParseInt64IDWithError_InvalidID(t *testing.T) {
	r := httptest.NewRequest("GET", "/test/not-a-number", nil)
	r = chiRouteContext(r, map[string]string{"id": "not-a-number"})
	w := httptest.NewRecorder()

	id, ok := common.ParseInt64IDWithError(w, r, "id", "invalid ID")

	assert.False(t, ok)
	assert.Equal(t, int64(0), id)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseInt64IDWithError_EmptyID(t *testing.T) {
	r := httptest.NewRequest("GET", "/test/", nil)
	r = chiRouteContext(r, map[string]string{"id": ""})
	w := httptest.NewRecorder()

	id, ok := common.ParseInt64IDWithError(w, r, "id", "invalid ID")

	assert.False(t, ok)
	assert.Equal(t, int64(0), id)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// =============================================================================
// Constants Tests
// =============================================================================

func TestPaginationConstants(t *testing.T) {
	assert.Equal(t, 1, common.DefaultPage)
	assert.Equal(t, 50, common.DefaultPageSize)
}
