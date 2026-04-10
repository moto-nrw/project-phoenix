package common

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
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
func (r *Response) Render(_ http.ResponseWriter, _ *http.Request) error {
	return nil
}

// Respond sends a structured response
func Respond(w http.ResponseWriter, r *http.Request, status int, data interface{}, message string) {
	render.Status(r, status)
	if err := render.Render(w, r, NewResponse(data, message)); err != nil {
		slog.Default().ErrorContext(r.Context(), "failed to render response", slog.String("error", err.Error()))
		http.Error(w, "Error rendering response", http.StatusInternalServerError)
	}
}

// RespondWithError sends a structured error response.
// For server errors (5xx), it logs the error to slog and captures to Sentry.
func RespondWithError(w http.ResponseWriter, r *http.Request, status int, errorMsg string) {
	if status >= 500 {
		slog.Default().ErrorContext(r.Context(), "server error",
			slog.Int("status", status),
			slog.String("error", errorMsg),
		)
		if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
			hub.CaptureException(errors.New(errorMsg))
		} else {
			sentry.CaptureException(errors.New(errorMsg))
		}
	}
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

// PaginationParams groups parameters for paginated responses to reduce function parameter count
type PaginationParams struct {
	Page     int
	PageSize int
	Total    int
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
func (p *PaginatedResponse) Render(_ http.ResponseWriter, _ *http.Request) error {
	return nil
}

// RespondPaginated sends a paginated response using PaginationParams struct
func RespondPaginated(w http.ResponseWriter, r *http.Request, status int, data interface{}, params PaginationParams, message string) {
	render.Status(r, status)
	if err := render.Render(w, r, NewPaginatedResponse(data, params.Page, params.PageSize, params.Total, message)); err != nil {
		slog.Default().ErrorContext(r.Context(), "failed to render paginated response", slog.String("error", err.Error()))
		http.Error(w, "Error rendering paginated response", http.StatusInternalServerError)
	}
}
