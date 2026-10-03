package presence

import (
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// errorRules maps presence sentinels to their HTTP status in the shared error
// envelope. Matched via errors.Is, so bare sentinels and operation-wrapped
// ones classify identically.
//
// The ErrStudentAlreadyActive 409: issue #844 added a DB-level partial
// unique index on active.visits; the presence command translates the
// resulting 23505 to ErrStudentAlreadyActive for ALL visit admissions, so the
// admin POST /active/visits route must answer 409 like the IoT checkin path,
// not 400.
var errorRules = []common.ErrorRule{
	{Target: studentpresence.ErrGroupNotFound, Render: common.ErrorNotFound},
	{Target: studentpresence.ErrVisitNotFound, Render: common.ErrorNotFound},
	{Target: studentpresence.ErrGroupSupervisorNotFound, Render: common.ErrorNotFound},
	{Target: studentpresence.ErrCombinedGroupNotFound, Render: common.ErrorNotFound},
	{Target: studentpresence.ErrGroupMappingNotFound, Render: common.ErrorNotFound},
	{Target: studentpresence.ErrInvalidData, Render: common.ErrorInvalidRequest},
	{Target: studentpresence.ErrGroupAlreadyEnded, Render: common.ErrorInvalidRequest},
	{Target: studentpresence.ErrVisitAlreadyEnded, Render: common.ErrorInvalidRequest},
	{Target: studentpresence.ErrSupervisionAlreadyEnded, Render: common.ErrorInvalidRequest},
	{Target: studentpresence.ErrCombinedGroupAlreadyEnded, Render: common.ErrorInvalidRequest},
	{Target: studentpresence.ErrGroupAlreadyInCombination, Render: common.ErrorInvalidRequest},
	{Target: studentpresence.ErrStudentAlreadyInGroup, Render: common.ErrorInvalidRequest},
	{Target: studentpresence.ErrStudentAlreadyActive, Render: common.ErrorConflict},
	{Target: studentpresence.ErrStudentsNotPresent, Render: common.ErrorConflict},
	{Target: studentpresence.ErrStudentMoveForbidden, Render: common.ErrorForbidden},
	{Target: studentpresence.ErrStaffAlreadySupervising, Render: common.ErrorInvalidRequest},
	{Target: studentpresence.ErrCannotDeleteActiveGroup, Render: common.ErrorInvalidRequest},
	{Target: studentpresence.ErrInvalidTimeRange, Render: common.ErrorInvalidRequest},
	{Target: studentpresence.ErrRoomConflict, Render: common.ErrorConflict},
	// A full room is a business rejection too (#3633): 409 with its code and
	// the numbers.
	{Target: studentpresence.ErrRoomCapacityExceeded, Render: common.ErrorBusinessRejectionOr(studentpresence.RoomCapacityCode)},
	// A web assignment beyond the activity's participant limit (#3632) is a
	// business rejection: 409 with its code and the numbers as details.
	{Target: studentpresence.ErrActivityParticipantLimitExceeded, Render: common.ErrorBusinessRejection},
	{Target: studentpresence.ErrNoRoomAvailable, Render: common.ErrorInvalidRequest},
	{Target: studentpresence.ErrStudentNotFound, Render: common.ErrorNotFound},
	{Target: studentpresence.ErrStaffNotFound, Render: common.ErrorNotFound},
	// A graduated (alumnus) student is treated like an unknown/absent student
	// (404), matching the IoT check-in mapper — a stale web/timetable request or
	// a graduation race must not fall through to a 500 (#405).
	{Target: studentpresence.ErrStudentGraduated, Render: common.ErrorNotFound},
	{Target: studentpresence.ErrStudentCareEnded, Render: common.ErrorNotFound},
}

// ErrorRenderer returns a render.Renderer for the given error
var ErrorRenderer = common.RulesRenderer(errorRules, common.ErrorInternalServer)
