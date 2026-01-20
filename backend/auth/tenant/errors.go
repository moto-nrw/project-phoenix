package tenant

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
)

// Domain errors for tenant operations.
var (
	// ErrOrgNotFound indicates the organization (OGS) was not found in the database.
	ErrOrgNotFound = errors.New("organization not found")

	// ErrTraegerNotFound indicates the Träger (carrier) was not found.
	ErrTraegerNotFound = errors.New("traeger not found")
)

// ErrResponse renderer type for handling HTTP error responses.
// Compatible with go-chi/render.
type ErrResponse struct {
	Err            error `json:"-"` // low-level runtime error (not exposed in JSON)
	HTTPStatusCode int   `json:"-"` // HTTP response status code

	StatusText string `json:"status"`          // user-level status message
	ErrorText  string `json:"error,omitempty"` // application-level error message
}

// Render implements render.Renderer interface.
// Sets the HTTP status code on the response.
func (e *ErrResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// Standard HTTP error responses for tenant middleware.
var (
	// ErrUnauthorized is returned when authentication fails.
	// This happens when:
	// - No session cookie is present
	// - Session cookie is invalid or expired
	// - BetterAuth service is unavailable
	ErrUnauthorized = &ErrResponse{
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "error",
		ErrorText:      "unauthorized",
	}

	// ErrForbidden is returned when authorization fails.
	// User is authenticated but lacks required permission.
	ErrForbidden = &ErrResponse{
		HTTPStatusCode: http.StatusForbidden,
		StatusText:     "error",
		ErrorText:      "forbidden",
	}

	// ErrNoOrganization is returned when user has no active organization selected.
	// This is a specific auth error - user needs to select an OGS.
	ErrNoOrganization = &ErrResponse{
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "error",
		ErrorText:      "no organization selected",
	}

	// ErrInternalServer is returned for unexpected server errors.
	// Used when middleware fails for reasons other than auth.
	ErrInternalServer = &ErrResponse{
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     "error",
		ErrorText:      "internal server error",
	}
)

// NewErrUnauthorized creates an unauthorized error with a custom message.
func NewErrUnauthorized(msg string) *ErrResponse {
	return &ErrResponse{
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "error",
		ErrorText:      msg,
	}
}

// NewErrForbidden creates a forbidden error with a custom message.
func NewErrForbidden(msg string) *ErrResponse {
	return &ErrResponse{
		HTTPStatusCode: http.StatusForbidden,
		StatusText:     "error",
		ErrorText:      msg,
	}
}

// NewErrInternal creates an internal server error with a custom message.
// Use sparingly - the message is exposed to clients.
func NewErrInternal(msg string) *ErrResponse {
	return &ErrResponse{
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     "error",
		ErrorText:      msg,
	}
}
