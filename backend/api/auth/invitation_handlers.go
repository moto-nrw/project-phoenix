package auth

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// renderInvitationError maps invitation service errors to appropriate HTTP responses.
func renderInvitationError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}

	var authErr *authService.AuthError
	if errors.As(err, &authErr) && authErr.Err != nil {
		err = authErr.Err
	}

	switch {
	case errors.Is(err, authService.ErrInvitationNotFound):
		if renderErr := render.Render(w, r, common.ErrorNotFound(authService.ErrInvitationNotFound)); renderErr != nil {
			return false
		}
		return true
	case errors.Is(err, authService.ErrInvitationExpired), errors.Is(err, authService.ErrInvitationUsed):
		if renderErr := render.Render(w, r, common.ErrorGone(err)); renderErr != nil {
			return false
		}
		return true
	default:
		return false
	}
}
