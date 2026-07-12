package rooms

import (
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	modelFacilities "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/services/facilities"
)

// errorRules map facilities-service sentinels to HTTP responses, applied to
// the INNER error of a *facilities.FacilitiesError so the JSON `error` field
// carries just the (German) message instead of the "facilities error during
// {Op}: …" prefix. The trailing Match rule catches model-level Validate()
// sentinels (name required, capacity negative, invalid color format)
// propagated through the service — without it, a forgotten mapping turns
// "name required" into a 500.
var errorRules = []common.ErrorRule{
	{Target: facilities.ErrRoomNotFound, Render: common.ErrorNotFound},
	{Target: facilities.ErrRoomInUse, Render: common.ErrorConflict},
	{Target: facilities.ErrRoomRequiredByCareOffering, Render: common.ErrorConflict},
	{Target: facilities.ErrSystemRoomProtected, Render: common.ErrorForbidden},
	{Target: facilities.ErrSystemRoomNameReserved, Render: common.ErrorInvalidRequest},
	{Target: facilities.ErrDuplicateRoom, Render: common.ErrorConflict},
	{Target: facilities.ErrDuplicateToiletRoom, Render: common.ErrorConflict},
	{Target: facilities.ErrColorAlreadyInUse, Render: colorConflict},
	{Target: facilities.ErrColorReserved, Render: common.ErrorInvalidRequest},
	{Match: modelFacilities.IsValidationError, Render: common.ErrorInvalidRequest},
}

func colorConflict(err error) render.Renderer {
	return common.ErrorConflictWithCode(err, "color_already_in_use")
}

// unwrapped handles errors that never went through a *FacilitiesError wrap.
// Defence-in-depth: a future caller may forget to wrap a model validation
// error; IsValidationError walks the chain so this still renders 400
// instead of 500. Everything else keeps the full error for server logs.
var unwrapped = []common.ErrorRule{
	{Match: modelFacilities.IsValidationError, Render: common.ErrorInvalidRequest},
}

// ErrorRenderer maps service-layer errors to appropriate HTTP responses.
var ErrorRenderer = common.UnwrapRenderer[*facilities.FacilitiesError](errorRules,
	common.RulesRenderer(unwrapped, common.ErrorInternalServer))
