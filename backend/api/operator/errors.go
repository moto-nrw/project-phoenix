package operator

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	suggestionsSvc "github.com/moto-nrw/project-phoenix/services/suggestions"
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

// ErrUnauthorized creates an unauthorized error response for invalid/expired tokens
func ErrUnauthorized() render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "error",
		ErrorText:      "Unauthorized",
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

// ErrConflict creates a conflict error response.
func ErrConflict(message string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: http.StatusConflict,
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

// ErrTooManyRequests creates a rate limit error response
func ErrTooManyRequests(message string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: http.StatusTooManyRequests,
		StatusText:     "Too Many Requests",
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

// ErrServiceUnavailable creates a 503 response. Used when a transient
// dependency makes a security decision impossible and the safe behaviour
// is to refuse this caller without globally locking everyone out.
func ErrServiceUnavailable(message string) render.Renderer {
	return &ErrResponse{
		HTTPStatusCode: http.StatusServiceUnavailable,
		StatusText:     "Service Unavailable",
		ErrorText:      message,
	}
}

// AuthErrorRenderer maps auth service errors to HTTP responses
func AuthErrorRenderer(err error) render.Renderer {
	var invalidCreds *platformSvc.InvalidCredentialsError
	var operatorInactive *platformSvc.OperatorInactiveError
	var operatorNotFound *platformSvc.OperatorNotFoundError
	var invalidRefresh *platformSvc.OperatorRefreshTokenInvalidError

	switch {
	case errors.As(err, &invalidRefresh):
		return ErrUnauthorized()
	case errors.As(err, &invalidCreds):
		return ErrInvalidCredentials()
	case errors.As(err, &operatorInactive):
		return ErrForbidden("Operator account is inactive")
	case errors.As(err, &operatorNotFound):
		return ErrInvalidCredentials()
	case errors.Is(err, authService.ErrMFARateLimited):
		return ErrTooManyRequests("Too many code requests, please wait")
	case errors.Is(err, authService.ErrMFALocked):
		return ErrTooManyRequests("MFA account temporarily locked")
	case errors.Is(err, authService.ErrMFAChallengeTokenInvalid),
		errors.Is(err, authService.ErrMFACodeInvalid):
		return ErrInvalidCredentials()
	case errors.Is(err, authService.ErrMFAStatusUnavailable):
		return ErrServiceUnavailable("MFA status temporarily unavailable, please retry")
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

	// Also check for suggestions service errors (unified comment system)
	var suggestionsCommentNotFound *suggestionsSvc.CommentNotFoundError
	var suggestionsForbidden *suggestionsSvc.ForbiddenError

	switch {
	case errors.As(err, &postNotFound):
		return ErrNotFound("Suggestion post not found")
	case errors.As(err, &commentNotFound):
		return ErrNotFound("Comment not found")
	case errors.As(err, &suggestionsCommentNotFound):
		return ErrNotFound("Comment not found")
	case errors.As(err, &suggestionsForbidden):
		return ErrForbidden(err.Error())
	case errors.As(err, &invalidData):
		return ErrInvalidRequest(err)
	default:
		return ErrInternal("An error occurred")
	}
}
