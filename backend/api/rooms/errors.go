package rooms

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	modelFacilities "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/services/facilities"
)

// Common error variables
var (
	ErrInvalidRequest   = errors.New("invalid request")
	ErrInternalServer   = errors.New("internal server error")
	ErrResourceNotFound = errors.New("resource not found")
)

// ErrorRenderer maps service-layer errors to appropriate HTTP responses.
//
// The branches are ordered:
//  1. Service-level FacilitiesError sentinels (the common path).
//  2. Bare model-level Validate() sentinels (defensive — fires when a
//     future code path returns an unwrapped facilities.ErrXxx). Without
//     this, a forgotten wrap turns "name required" into 500.
//  3. Default 500.
func ErrorRenderer(err error) render.Renderer {
	var facErr *facilities.FacilitiesError
	if errors.As(err, &facErr) {
		// Inner sentinel from the service layer (Op + Err wrap).
		switch facErr.Unwrap() {
		case facilities.ErrRoomNotFound:
			return common.ErrorNotFound(facErr)
		case facilities.ErrRoomInUse:
			return common.ErrorConflict(facErr)
		case facilities.ErrSystemRoomProtected:
			return common.ErrorForbidden(facErr)
		case facilities.ErrDuplicateRoom:
			return common.ErrorConflict(facErr)
		case facilities.ErrColorAlreadyInUse:
			return common.ErrorConflictWithCode(facErr, "color_already_in_use")
		case facilities.ErrColorReserved:
			return common.ErrorInvalidRequest(facErr)
		}

		// Model-level Validate() sentinels propagated through the service
		// (name required, capacity negative, invalid color format).
		if modelFacilities.IsValidationError(facErr.Unwrap()) {
			return common.ErrorInvalidRequest(facErr)
		}

		return common.ErrorInternalServer(facErr)
	}

	// Defence-in-depth: a future caller may forget to wrap a model
	// validation error in *FacilitiesError. errors.Is walks the chain so
	// this still catches the intent and renders 400 instead of 500.
	if modelFacilities.IsValidationError(err) {
		return common.ErrorInvalidRequest(err)
	}

	return common.ErrorInternalServer(err)
}

// ErrorInvalidRequest returns a 400 Bad Request error response
func ErrorInvalidRequest(err error) render.Renderer {
	return common.ErrorInvalidRequest(err)
}
