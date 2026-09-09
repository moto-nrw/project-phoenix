package sessions

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	shared "github.com/moto-nrw/project-phoenix/api/iot/internal/shared"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
)

// ErrorRenderer maps the retained IoT and active service errors of the
// session, timeout and device administration routes to the exact responses
// PyrePortal expects. It used to live in api/iot/internal/shared; the kiosk
// scan flows now classify through the public device-scan contract, so this
// table only serves the routes still bound to the retained services (#2739).
//
// Rules are evaluated in order. The capacity and no-room sentinels match
// wherever they are wrapped; the IoT and active families match only their own
// wrapper, as the hand-written dispatch did.
var ErrorRenderer = common.RulesRenderer(errorRules, common.ErrorInternalServer)

var errPersonNotStudent = errors.New(shared.ErrMsgPersonNotStudent)

var errorRules = []common.ErrorRule{
	{Target: activeSvc.ErrRoomCapacityExceeded, Render: common.ErrorConflict},
	// The normal session-start path returns this sentinel unwrapped.
	{Target: activeSvc.ErrNoRoomAvailable, Render: common.ErrorInvalidRequest},

	// IoT service family.
	{Match: iotIs(iotSvc.ErrDeviceNotFound), Render: common.ErrorNotFound},
	{Match: iotIs(iotSvc.ErrInvalidDeviceData), Render: common.ErrorInvalidRequest},
	{Match: iotIs(iotSvc.ErrDuplicateDeviceID), Render: common.ErrorConflict},
	{Match: iotIs(iotSvc.ErrInvalidStatus), Render: common.ErrorInvalidRequest},
	{Match: iotIs(iotSvc.ErrDeviceProtected), Render: common.ErrorForbidden},
	{Match: iotIs(iotSvc.ErrDeviceOffline), Render: common.ErrorConflict},
	{Match: isIoTError, Render: common.ErrorInternalServer},

	// Active service family. A graduation or care exit committing between the
	// kiosk's student lookup and the write renders the documented contract
	// string, not the wrapped error text PyrePortal knows nothing about.
	{Match: activeIs(activeSvc.ErrStudentGraduated, activeSvc.ErrStudentCareEnded), Render: common.FixedRenderer(common.ErrorNotFound, errPersonNotStudent)},
	{Match: activeIs(
		activeSvc.ErrRoomConflict, activeSvc.ErrSessionConflict, activeSvc.ErrStudentAlreadyInGroup,
		activeSvc.ErrGroupAlreadyInCombination, activeSvc.ErrStudentAlreadyActive, activeSvc.ErrStaffAlreadySupervising,
		activeSvc.ErrDeviceAlreadyActive,
	), Render: common.ErrorConflict},
	{Match: activeIs(
		activeSvc.ErrActiveGroupNotFound, activeSvc.ErrVisitNotFound, activeSvc.ErrGroupSupervisorNotFound,
		activeSvc.ErrCombinedGroupNotFound, activeSvc.ErrGroupMappingNotFound, activeSvc.ErrNoActiveSession,
		activeSvc.ErrStaffNotFound,
	), Render: common.ErrorNotFound},
	{Match: activeIs(
		activeSvc.ErrActiveGroupAlreadyEnded, activeSvc.ErrVisitAlreadyEnded, activeSvc.ErrSupervisionAlreadyEnded,
		activeSvc.ErrCombinedGroupAlreadyEnded, activeSvc.ErrInvalidTimeRange, activeSvc.ErrCannotDeleteActiveGroup,
		activeSvc.ErrInvalidData, activeSvc.ErrInvalidActivitySession, activeSvc.ErrNoRoomAvailable,
	), Render: common.ErrorInvalidRequest},
}

func isIoTError(err error) bool {
	_, ok := err.(*iotSvc.IoTError) //nolint:errorlint // the historic mapping dispatched on the outer wrapper only
	return ok
}

// iotIs matches an IoT service wrapper whose cause is the sentinel; the
// typed device errors unwrap to the same sentinels.
func iotIs(sentinel error) func(error) bool {
	return func(err error) bool {
		iotErr, ok := err.(*iotSvc.IoTError) //nolint:errorlint // the historic mapping dispatched on the outer wrapper only
		return ok && errors.Is(iotErr.Err, sentinel)
	}
}

// activeIs matches an active service wrapper whose cause is one of the
// sentinels.
func activeIs(sentinels ...error) func(error) bool {
	return func(err error) bool {
		activeErr, ok := err.(*activeSvc.ActiveError) //nolint:errorlint // the historic mapping dispatched on the outer wrapper only
		if !ok {
			return false
		}
		for _, sentinel := range sentinels {
			if errors.Is(activeErr.Err, sentinel) {
				return true
			}
		}
		return false
	}
}

// renderError renders a retained service error through the legacy table.
func renderError(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, ErrorRenderer(err))
}

var _ func(error) render.Renderer = ErrorRenderer
