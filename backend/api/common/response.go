package common

import (
	"net/http"
	"time"

	"github.com/go-chi/render"
)

// Time is a wrapper for time.Time that properly serializes to JSON in RFC3339 format
type Time time.Time

// MarshalJSON implements the json.Marshaler interface
func (t Time) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(t).Format(time.RFC3339) + `"`), nil
}

// Response is the standard API response structure
type Response struct {
	Status  string      `json:"status"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// NewResponse creates a new successful response
func NewResponse(data interface{}, message string) *Response {
	return &Response{
		Status:  "success",
		Data:    data,
		Message: message,
	}
}

// Render implements the render.Renderer interface for Response
func (r *Response) Render(w http.ResponseWriter, req *http.Request) error {
	return nil
}

// Respond sends a structured response
func Respond(w http.ResponseWriter, r *http.Request, status int, data interface{}, message string) {
	render.Status(r, status)
	if err := render.Render(w, r, NewResponse(data, message)); err != nil {
		// Log the error but don't fail the operation since the response was already started
		// This is a best-effort operation
		http.Error(w, "Error rendering response", http.StatusInternalServerError)
	}
}

// RespondWithError sends a structured error response
func RespondWithError(w http.ResponseWriter, r *http.Request, status int, errorMsg string) {
	render.Status(r, status)
	render.JSON(w, r, map[string]string{
		"status": "error",
		"error":  errorMsg,
	})
}

// RespondNoContent sends a 204 No Content response
func RespondNoContent(w http.ResponseWriter, r *http.Request) {
	render.NoContent(w, r)
}

// RespondWithJSON sends a JSON response with the given status code
func RespondWithJSON(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	render.Status(r, status)
	render.JSON(w, r, data)
}

// Pagination represents pagination metadata for responses
type Pagination struct {
	CurrentPage  int `json:"current_page"`
	PageSize     int `json:"page_size"`
	TotalPages   int `json:"total_pages"`
	TotalRecords int `json:"total_records"`
}

// PaginatedResponse is a response with pagination metadata
type PaginatedResponse struct {
	Status     string      `json:"status"`
	Data       interface{} `json:"data,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
	Message    string      `json:"message,omitempty"`
}

// NewPaginatedResponse creates a new paginated response
func NewPaginatedResponse(data interface{}, page, pageSize, total int, message string) *PaginatedResponse {
	var totalPages int
	if pageSize > 0 {
		totalPages = (total + pageSize - 1) / pageSize // Ceiling division
	}

	return &PaginatedResponse{
		Status: "success",
		Data:   data,
		Pagination: &Pagination{
			CurrentPage:  page,
			PageSize:     pageSize,
			TotalPages:   totalPages,
			TotalRecords: total,
		},
		Message: message,
	}
}

// Render implements the render.Renderer interface for PaginatedResponse
func (p *PaginatedResponse) Render(w http.ResponseWriter, req *http.Request) error {
	return nil
}

// RespondWithPagination sends a paginated response
func RespondWithPagination(w http.ResponseWriter, r *http.Request, status int, data interface{}, page, pageSize, total int, message string) {
	render.Status(r, status)
	if err := render.Render(w, r, NewPaginatedResponse(data, page, pageSize, total, message)); err != nil {
		// Log the error but don't fail the operation since the response was already started
		// This is a best-effort operation
		http.Error(w, "Error rendering paginated response", http.StatusInternalServerError)
	}
}
