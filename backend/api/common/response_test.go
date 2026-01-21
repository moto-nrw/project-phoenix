package common_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Time.MarshalJSON Tests
// =============================================================================

func TestTime_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "standard time",
			time:     time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			expected: `"2024-01-15T10:30:00Z"`,
		},
		{
			name:     "time with timezone",
			time:     time.Date(2024, 6, 20, 14, 45, 30, 0, time.FixedZone("CET", 3600)),
			expected: `"2024-06-20T14:45:30+01:00"`,
		},
		{
			name:     "midnight",
			time:     time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
			expected: `"2024-12-31T00:00:00Z"`,
		},
		{
			name:     "end of day",
			time:     time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			expected: `"2024-12-31T23:59:59Z"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ct := common.Time(tc.time)
			data, err := ct.MarshalJSON()

			require.NoError(t, err)
			assert.Equal(t, tc.expected, string(data))
		})
	}
}

func TestTime_MarshalJSON_ZeroTime(t *testing.T) {
	ct := common.Time(time.Time{})
	data, err := ct.MarshalJSON()

	require.NoError(t, err)
	// Zero time marshals to RFC3339 format
	assert.Contains(t, string(data), "0001-01-01")
}

// =============================================================================
// NewResponse Tests
// =============================================================================

func TestNewResponse(t *testing.T) {
	tests := []struct {
		name    string
		data    interface{}
		message string
	}{
		{
			name:    "with string data",
			data:    "test data",
			message: "Success",
		},
		{
			name:    "with struct data",
			data:    struct{ ID int }{ID: 123},
			message: "Record created",
		},
		{
			name:    "with nil data",
			data:    nil,
			message: "No content",
		},
		{
			name:    "with slice data",
			data:    []int{1, 2, 3},
			message: "List retrieved",
		},
		{
			name:    "with empty message",
			data:    "data",
			message: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := common.NewResponse(tc.data, tc.message)

			assert.Equal(t, "success", resp.Status)
			assert.Equal(t, tc.data, resp.Data)
			assert.Equal(t, tc.message, resp.Message)
		})
	}
}

func TestResponse_Render(t *testing.T) {
	resp := common.NewResponse("test", "message")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := resp.Render(w, r)

	assert.NoError(t, err)
}

// =============================================================================
// Respond Tests
// =============================================================================

func TestRespond_Success(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	data := map[string]string{"key": "value"}

	common.Respond(w, r, http.StatusOK, data, "success message")

	assert.Equal(t, http.StatusOK, w.Code)

	var response common.Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response.Status)
	assert.Equal(t, "success message", response.Message)
}

func TestRespond_Created(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", nil)
	data := struct {
		ID int `json:"id"`
	}{ID: 42}

	common.Respond(w, r, http.StatusCreated, data, "resource created")

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, "resource created", response["message"])
}

func TestRespond_NilData(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	common.Respond(w, r, http.StatusOK, nil, "no data")

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response["status"])
	_, hasData := response["data"]
	assert.False(t, hasData, "data should be omitted when nil")
}

// =============================================================================
// RespondWithError Tests
// =============================================================================

func TestRespondWithError_BadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	common.RespondWithError(w, r, http.StatusBadRequest, "invalid input")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "error", response["status"])
	assert.Equal(t, "invalid input", response["error"])
}

func TestRespondWithError_Unauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	common.RespondWithError(w, r, http.StatusUnauthorized, "not authorized")

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "error", response["status"])
	assert.Equal(t, "not authorized", response["error"])
}

func TestRespondWithError_InternalServerError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	common.RespondWithError(w, r, http.StatusInternalServerError, "server error")

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "error", response["status"])
	assert.Equal(t, "server error", response["error"])
}

// =============================================================================
// RespondNoContent Tests
// =============================================================================

func TestRespondNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/test/1", nil)

	common.RespondNoContent(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.Bytes())
}

// =============================================================================
// RespondWithJSON Tests
// =============================================================================

func TestRespondWithJSON_Success(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	data := map[string]interface{}{
		"items": []string{"a", "b", "c"},
		"count": 3,
	}

	common.RespondWithJSON(w, r, http.StatusOK, data)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, float64(3), response["count"])
}

func TestRespondWithJSON_CustomStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"OK", http.StatusOK},
		{"Created", http.StatusCreated},
		{"Accepted", http.StatusAccepted},
		{"Bad Request", http.StatusBadRequest},
		{"Not Found", http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/test", nil)

			common.RespondWithJSON(w, r, tc.status, map[string]string{"test": "data"})

			assert.Equal(t, tc.status, w.Code)
		})
	}
}

// =============================================================================
// Pagination Tests
// =============================================================================

func TestPagination_Struct(t *testing.T) {
	p := common.Pagination{
		CurrentPage:  2,
		PageSize:     25,
		TotalPages:   4,
		TotalRecords: 100,
	}

	data, err := json.Marshal(p)
	require.NoError(t, err)

	var decoded common.Pagination
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, 2, decoded.CurrentPage)
	assert.Equal(t, 25, decoded.PageSize)
	assert.Equal(t, 4, decoded.TotalPages)
	assert.Equal(t, 100, decoded.TotalRecords)
}

func TestPaginationParams(t *testing.T) {
	params := common.PaginationParams{
		Page:     3,
		PageSize: 50,
		Total:    150,
	}

	assert.Equal(t, 3, params.Page)
	assert.Equal(t, 50, params.PageSize)
	assert.Equal(t, 150, params.Total)
}

// =============================================================================
// NewPaginatedResponse Tests
// =============================================================================

func TestNewPaginatedResponse_StandardCase(t *testing.T) {
	data := []string{"item1", "item2", "item3"}

	resp := common.NewPaginatedResponse(data, 1, 10, 25, "Items retrieved")

	assert.Equal(t, "success", resp.Status)
	assert.Equal(t, data, resp.Data)
	assert.Equal(t, "Items retrieved", resp.Message)
	require.NotNil(t, resp.Pagination)
	assert.Equal(t, 1, resp.Pagination.CurrentPage)
	assert.Equal(t, 10, resp.Pagination.PageSize)
	assert.Equal(t, 3, resp.Pagination.TotalPages) // 25 / 10 = 3 (ceiling)
	assert.Equal(t, 25, resp.Pagination.TotalRecords)
}

func TestNewPaginatedResponse_TotalPages_EdgeCases(t *testing.T) {
	tests := []struct {
		name              string
		total             int
		pageSize          int
		expectedTotalPages int
	}{
		{"exact division", 100, 25, 4},
		{"with remainder", 101, 25, 5},
		{"single page", 5, 10, 1},
		{"zero total", 0, 10, 0},
		{"total equals page size", 10, 10, 1},
		{"large total", 1000, 50, 20},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := common.NewPaginatedResponse(nil, 1, tc.pageSize, tc.total, "")
			assert.Equal(t, tc.expectedTotalPages, resp.Pagination.TotalPages)
		})
	}
}

func TestNewPaginatedResponse_ZeroPageSize(t *testing.T) {
	// Zero page size should not cause division by zero
	resp := common.NewPaginatedResponse(nil, 1, 0, 100, "")

	assert.NotNil(t, resp.Pagination)
	assert.Equal(t, 0, resp.Pagination.TotalPages)
}

func TestNewPaginatedResponse_EmptyData(t *testing.T) {
	resp := common.NewPaginatedResponse([]int{}, 1, 50, 0, "No items found")

	assert.Equal(t, "success", resp.Status)
	assert.Empty(t, resp.Data)
	assert.Equal(t, 0, resp.Pagination.TotalRecords)
	assert.Equal(t, 0, resp.Pagination.TotalPages)
}

func TestPaginatedResponse_Render(t *testing.T) {
	resp := common.NewPaginatedResponse(nil, 1, 10, 0, "")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := resp.Render(w, r)

	assert.NoError(t, err)
}

// =============================================================================
// RespondPaginated Tests
// =============================================================================

func TestRespondPaginated_Success(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	data := []map[string]int{{"id": 1}, {"id": 2}}
	params := common.PaginationParams{Page: 2, PageSize: 10, Total: 50}

	common.RespondPaginated(w, r, http.StatusOK, data, params, "page 2 of 5")

	assert.Equal(t, http.StatusOK, w.Code)

	var response common.PaginatedResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response.Status)
	assert.Equal(t, "page 2 of 5", response.Message)
	require.NotNil(t, response.Pagination)
	assert.Equal(t, 2, response.Pagination.CurrentPage)
	assert.Equal(t, 10, response.Pagination.PageSize)
	assert.Equal(t, 5, response.Pagination.TotalPages)
	assert.Equal(t, 50, response.Pagination.TotalRecords)
}

func TestRespondPaginated_EmptyResults(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	params := common.PaginationParams{Page: 1, PageSize: 50, Total: 0}

	common.RespondPaginated(w, r, http.StatusOK, []interface{}{}, params, "no results")

	assert.Equal(t, http.StatusOK, w.Code)

	var response common.PaginatedResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, 0, response.Pagination.TotalRecords)
	assert.Equal(t, 0, response.Pagination.TotalPages)
}

func TestRespondPaginated_LastPage(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	data := []string{"item"} // Single item on last page
	params := common.PaginationParams{Page: 5, PageSize: 10, Total: 41}

	common.RespondPaginated(w, r, http.StatusOK, data, params, "last page")

	assert.Equal(t, http.StatusOK, w.Code)

	var response common.PaginatedResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, 5, response.Pagination.CurrentPage)
	assert.Equal(t, 5, response.Pagination.TotalPages)
}
