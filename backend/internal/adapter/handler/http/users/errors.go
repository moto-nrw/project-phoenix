package users

import (
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	authSvc "github.com/moto-nrw/project-phoenix/internal/core/service/auth"
	usersSvc "github.com/moto-nrw/project-phoenix/internal/core/service/users"
)

// ErrorRenderer renders an error to an HTTP response
func ErrorRenderer(err error) render.Renderer {
	// Check if the error is a specific users service error
	if usrErr, ok := err.(*usersSvc.UsersError); ok {
		// Map specific users service errors to appropriate HTTP status codes
		switch usrErr.Unwrap() {
		case usersSvc.ErrPersonNotFound:
			return common.ErrorNotFound(usrErr)
		case authSvc.ErrAccountNotFound:
			return common.ErrorNotFound(usrErr)
		case usersSvc.ErrRFIDCardNotFound:
			return common.ErrorNotFound(usrErr)
		case usersSvc.ErrAccountAlreadyLinked, usersSvc.ErrRFIDCardAlreadyLinked:
			return common.ErrorConflict(usrErr)
		case usersSvc.ErrPersonIdentifierRequired:
			return common.ErrorInvalidRequest(usrErr)
		default:
			return common.ErrorInternalServer(usrErr)
		}
	}

	// For unknown errors, return a generic internal server error
	return common.ErrorInternalServer(err)
}
