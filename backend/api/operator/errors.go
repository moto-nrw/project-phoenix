package operator

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// ErrInvalidRequest creates an error response for invalid requests
func ErrInvalidRequest(err error) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "error",
		ErrorText:      err.Error(),
	}
}

// ErrInvalidCredentials creates an error response for invalid credentials
func ErrInvalidCredentials() render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "error",
		ErrorText:      "Invalid email or password",
	}
}

// ErrNotFound creates a not found error response
func ErrNotFound(message string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: http.StatusNotFound,
		StatusText:     "error",
		ErrorText:      message,
	}
}

// ErrForbidden creates a forbidden error response
func ErrForbidden(message string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: http.StatusForbidden,
		StatusText:     "error",
		ErrorText:      message,
	}
}

// ErrInternal creates an internal server error response
func ErrInternal(message string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     "error",
		ErrorText:      message,
	}
}

// AuthErrorRenderer maps auth service errors to HTTP responses
func AuthErrorRenderer(err error) render.Renderer {
	var invalidCreds *platformSvc.InvalidCredentialsError
	var operatorInactive *platformSvc.OperatorInactiveError
	var operatorNotFound *platformSvc.OperatorNotFoundError

	switch {
	case errors.As(err, &invalidCreds):
		return ErrInvalidCredentials()
	case errors.As(err, &operatorInactive):
		return ErrForbidden("Operator account is inactive")
	case errors.As(err, &operatorNotFound):
		return ErrInvalidCredentials()
	default:
		return ErrInternal("Authentication failed")
	}
}

// AnnouncementErrorRenderer maps announcement service errors to HTTP responses
func AnnouncementErrorRenderer(err error) render.Renderer {
	var notFound *platformSvc.AnnouncementNotFoundError
	var invalidData *platformSvc.InvalidDataError

	switch {
	case errors.As(err, &notFound):
		return ErrNotFound("Announcement not found")
	case errors.As(err, &invalidData):
		return ErrInvalidRequest(err)
	default:
		return ErrInternal("An error occurred")
	}
}

// SuggestionsErrorRenderer maps suggestions service errors to HTTP responses
func SuggestionsErrorRenderer(err error) render.Renderer {
	var postNotFound *platformSvc.PostNotFoundError
	var commentNotFound *platformSvc.CommentNotFoundError
	var invalidData *platformSvc.InvalidDataError

	switch {
	case errors.As(err, &postNotFound):
		return ErrNotFound("Suggestion post not found")
	case errors.As(err, &commentNotFound):
		return ErrNotFound("Comment not found")
	case errors.As(err, &invalidData):
		return ErrInvalidRequest(err)
	default:
		return ErrInternal("An error occurred")
	}
}
