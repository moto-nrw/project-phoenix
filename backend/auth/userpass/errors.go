package userpass

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
)

// ErrorResponse is the response that represents an error.
type ErrorResponse struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	StatusText string `json:"status"`          // user-level status message
	AppCode    int64  `json:"code,omitempty"`  // application-specific error code
	ErrorText  string `json:"error,omitempty"` // application-level error message, for debugging
}

// Render sets the application-specific error code in AppCode.
func (e *ErrorResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// ErrInvalidLogin returns a 401 Unauthorized error for invalid login attempts.
func ErrInvalidLogin(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "Invalid login.",
		ErrorText:      err.Error(),
	}
}

// ErrInvalidCredentials returns a 401 Unauthorized error for invalid credentials.
func ErrInvalidCredentials(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "Invalid credentials.",
		ErrorText:      err.Error(),
	}
}

// ErrUnknownLogin returns a 401 Unauthorized error for unknown logins.
func ErrUnknownLogin(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "Unknown login.",
		ErrorText:      err.Error(),
	}
}

// ErrLoginDisabled returns a 401 Unauthorized error for disabled accounts.
func ErrLoginDisabled(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "Login disabled.",
		ErrorText:      err.Error(),
	}
}

// ErrInvalidRegistration returns a 400 Bad Request error for invalid registration.
func ErrInvalidRegistration(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "Invalid registration.",
		ErrorText:      err.Error(),
	}
}

// ErrPasswordMismatch returns a 400 Bad Request error for password mismatch.
func ErrPasswordMismatch(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "Passwords do not match.",
		ErrorText:      err.Error(),
	}
}

// ErrPasswordComplexity returns a 400 Bad Request error for password complexity issues.
func ErrPasswordComplexity(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "Password does not meet complexity requirements.",
		ErrorText:      err.Error(),
	}
}

// ErrInternalServerError returns a 500 Internal Server Error.
var ErrInternalServerError = &ErrorResponse{
	HTTPStatusCode: http.StatusInternalServerError,
	StatusText:     "Internal server error.",
}

// Custom error variables
var (
	ErrInvalidPassword       = errors.New("invalid password")
	ErrEmailAlreadyExists    = errors.New("email already exists")
	ErrUsernameAlreadyExists = errors.New("username already exists")
	ErrPasswordRequired      = errors.New("password is required")
	ErrPasswordTooShort      = errors.New("password is too short (minimum 8 characters)")
	ErrPasswordNoUpper       = errors.New("password must contain at least one uppercase letter")
	ErrPasswordNoLower       = errors.New("password must contain at least one lowercase letter")
	ErrPasswordNoNumber      = errors.New("password must contain at least one number")
	ErrPasswordNoSpecial     = errors.New("password must contain at least one special character")
)

// Error string constants
const (
	InvalidLogin     = "invalid login"
	UnknownLogin     = "unknown login"
	LoginDisabled    = "login disabled"
	PasswordMismatch = "passwords do not match"
)
