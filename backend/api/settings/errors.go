package settings

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
)

var (
	// ErrSettingNotFound is returned when a setting is not found
	ErrSettingNotFound = errors.New("setting not found")

	// ErrSettingAlreadyExists is returned when a setting with the same key already exists
	ErrSettingAlreadyExists = errors.New("setting with this key already exists")

	// ErrInvalidSettingData is returned when invalid setting data is provided
	ErrInvalidSettingData = errors.New("invalid setting data")

	// ErrInvalidSettingID is returned when an invalid setting ID is provided
	ErrInvalidSettingID = errors.New("invalid setting ID")
)

// ErrResponse renderer type for handling all sorts of errors.
//
// In the best case scenario, the excellent github.com/pkg/errors package
// helps reveal information on the error, setting it on Err, and in the Render
// method, using it to set the application-specific error code in AppCode.
type ErrResponse struct {
	Err            error  `json:"error,omitempty"`
	HTTPStatusCode int    `json:"status"`         // http response status code
	StatusText     string `json:"status_text"`    // user-level status message
	AppCode        int64  `json:"code,omitempty"` // application-specific error code
}

// Render implements render.Renderer interface
func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// ErrInvalidRequest returns a 400 error for invalid requests
func ErrInvalidRequest(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "Invalid request.",
	}
}

// ErrNotFound returns a 404 error for requests where the resource doesn't exist
func ErrNotFound() render.Renderer {
	return &ErrResponse{
		Err:            ErrSettingNotFound,
		HTTPStatusCode: http.StatusNotFound,
		StatusText:     "Resource not found.",
	}
}

// ErrInternalServer returns a 500 error for unforeseen server-side errors
func ErrInternalServer(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     "Internal server error.",
	}
}

// ErrConflict returns a 409 error for resource conflicts
func ErrConflict(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusConflict,
		StatusText:     "Resource conflict.",
	}
}
