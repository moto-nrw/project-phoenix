package users

import (
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// ErrorRenderer renders an error to an HTTP response
func ErrorRenderer(err error) render.Renderer {
	// Check if the error is a specific users service error
	if usrErr, ok := err.(*usersSvc.UsersError); ok {
		// Map specific users service errors to appropriate HTTP status codes
		switch usrErr.Unwrap() {
		case usersSvc.ErrPersonNotFound:
			return common.ErrorNotFound(usrErr)
		case usersSvc.ErrAccountNotFound:
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
