package presence

import "github.com/moto-nrw/project-phoenix/modules/studentpresence"

// Common service errors. Every sentinel is the owner's public value so every
// consumer classifies it without this package.
var (
	ErrActiveGroupNotFound     = studentpresence.ErrGroupNotFound
	ErrVisitNotFound           = studentpresence.ErrVisitNotFound
	ErrGroupSupervisorNotFound = studentpresence.ErrGroupSupervisorNotFound
	ErrCombinedGroupNotFound   = studentpresence.ErrCombinedGroupNotFound
	ErrGroupMappingNotFound    = studentpresence.ErrGroupMappingNotFound
	ErrStaffNotFound           = studentpresence.ErrStaffNotFound
	ErrStudentNotFound         = studentpresence.ErrStudentNotFound
	// ErrStudentGraduated guards the check-in write path against a graduated
	// (alumnus) student. It is the atomic backstop to the grade-transition
	// checked-in guard: the write path locks the student row FOR UPDATE and
	// rejects an alumnus, closing the race where resolution saw an active
	// student but a concurrent graduation committed mid-request (#405).
	ErrStudentGraduated = studentpresence.ErrStudentGraduated
	// ErrStudentCareEnded guards every presence write against a child whose
	// care has ended (#2487). Same shape as ErrStudentGraduated: resolution may
	// have seen a child still in care while the exit took effect mid-request,
	// so the write path re-checks under the row lock. The message is part of
	// the /api/iot/* wire contract PyrePortal maps to German text.
	ErrStudentCareEnded                 = studentpresence.ErrStudentCareEnded
	ErrActiveGroupAlreadyEnded          = studentpresence.ErrGroupAlreadyEnded
	ErrVisitAlreadyEnded                = studentpresence.ErrVisitAlreadyEnded
	ErrSupervisionAlreadyEnded          = studentpresence.ErrSupervisionAlreadyEnded
	ErrCombinedGroupAlreadyEnded        = studentpresence.ErrCombinedGroupAlreadyEnded
	ErrStudentAlreadyInGroup            = studentpresence.ErrStudentAlreadyInGroup
	ErrGroupAlreadyInCombination        = studentpresence.ErrGroupAlreadyInCombination
	ErrInvalidTimeRange                 = studentpresence.ErrInvalidTimeRange
	ErrCannotDeleteActiveGroup          = studentpresence.ErrCannotDeleteActiveGroup
	ErrStudentAlreadyActive             = studentpresence.ErrStudentAlreadyActive
	ErrStaffAlreadySupervising          = studentpresence.ErrStaffAlreadySupervising
	ErrStudentsNotPresent               = studentpresence.ErrStudentsNotPresent
	ErrStudentMoveForbidden             = studentpresence.ErrStudentMoveForbidden
	ErrInvalidData                      = studentpresence.ErrInvalidData
	ErrDatabaseOperation                = studentpresence.ErrDatabaseOperation
	ErrNoAttendanceRecordForCheckout    = studentpresence.ErrNoAttendanceRecordForCheckout
	ErrDeviceAlreadyActive              = studentpresence.ErrDeviceAlreadyActive
	ErrNoActiveSession                  = studentpresence.ErrNoActiveSession
	ErrSessionConflict                  = studentpresence.ErrSessionConflict
	ErrInvalidActivitySession           = studentpresence.ErrInvalidActivitySession
	ErrRoomConflict                     = studentpresence.ErrRoomConflict
	ErrRoomCapacityExceeded             = studentpresence.ErrRoomCapacityExceeded
	ErrActivityParticipantLimitExceeded = studentpresence.ErrActivityParticipantLimitExceeded
	// ErrNoRoomAvailable: no room was selected and the activity has no planned
	// room. There is no safe default here — room id 1 belongs to one specific
	// school, so any hardcoded fallback trips fk_active_groups_room_tenant for
	// every other tenant. The message is part of the PyrePortal contract.
	ErrNoRoomAvailable = studentpresence.ErrNoRoomAvailable
)

// RoomCapacityError preserves the service contract for the presence domain error.
type RoomCapacityError = studentpresence.RoomCapacityError

// ActivityParticipantLimitError preserves the service contract for the
// participant-limit refusal of web assignments.
type ActivityParticipantLimitError = studentpresence.ActivityParticipantLimitError

// ActiveError is the owner's operation error; the service wraps every
// classified sentinel in it.
type ActiveError = studentpresence.OperationError
