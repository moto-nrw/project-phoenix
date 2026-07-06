package rooms

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	modelFacilities "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/services/facilities"
)

// ErrorRenderer maps service-layer errors to appropriate HTTP responses.
//
// The branches are ordered:
//  1. Service-level FacilitiesError sentinels (the common path).
//  2. Bare model-level Validate() sentinels (defensive — fires when a
//     future code path returns an unwrapped facilities.ErrXxx). Without
//     this, a forgotten wrap turns "name required" into 500.
//  3. Default 500.
//
// User-facing branches render the inner sentinel directly so the JSON
// `error` field carries just the (German) message instead of FacilitiesError's
// "facilities error during {Op}: …" prefix. The 500 fallback keeps the
// wrapped facErr so server logs still capture the operation context.
func ErrorRenderer(err error) render.Renderer {
	var facErr *facilities.FacilitiesError
	if errors.As(err, &facErr) {
		// Inner sentinel from the service layer (Op + Err wrap).
		inner := facErr.Unwrap()
		switch inner {
		case facilities.ErrRoomNotFound:
			return common.ErrorNotFound(inner)
		case facilities.ErrRoomInUse:
			return common.ErrorConflict(inner)
		case facilities.ErrSystemRoomProtected:
			return common.ErrorForbidden(inner)
		case facilities.ErrDuplicateRoom:
			return common.ErrorConflict(inner)
		case facilities.ErrDuplicateToiletRoom:
			return common.ErrorConflict(inner)
		case facilities.ErrColorAlreadyInUse:
			return common.ErrorConflictWithCode(inner, "color_already_in_use")
		case facilities.ErrColorReserved:
			return common.ErrorInvalidRequest(inner)
		}

		// Model-level Validate() sentinels propagated through the service
		// (name required, capacity negative, invalid color format).
		if modelFacilities.IsValidationError(inner) {
			return common.ErrorInvalidRequest(inner)
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
