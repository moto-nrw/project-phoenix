package rooms

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/services/facilities"
)

// Common error variables
var (
	ErrInvalidRequest   = errors.New("invalid request")
	ErrInternalServer   = errors.New("internal server error")
	ErrResourceNotFound = errors.New("resource not found")
)

// ErrorRenderer maps service-layer errors to appropriate HTTP responses.
func ErrorRenderer(err error) render.Renderer {
	var facErr *facilities.FacilitiesError
	if errors.As(err, &facErr) {
		switch facErr.Unwrap() {
		case facilities.ErrRoomNotFound:
			return common.ErrorNotFound(facErr)
		case facilities.ErrRoomInUse:
			return common.ErrorConflict(facErr)
		case facilities.ErrDuplicateRoom:
			return common.ErrorConflict(facErr)
		default:
			return common.ErrorInternalServer(facErr)
		}
	}
	return common.ErrorInternalServer(err)
}

// ErrorInvalidRequest returns a 400 Bad Request error response
func ErrorInvalidRequest(err error) render.Renderer {
	return common.ErrorInvalidRequest(err)
}
