package presence

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// PresenceOperations is the consumer-owned port for the retained presence
// commands and reads these routes still delegate. The root composition binds
// it to the retained active service; every value and error it exchanges is
// the Student Presence owner's public contract, so this package classifies
// outcomes without the retained service.
type PresenceOperations interface {
	// Sessions
	StartSession(context.Context, studentpresence.LiveGroup) (studentpresence.LiveGroup, error)
	ReviseSession(context.Context, studentpresence.LiveGroup) (studentpresence.LiveGroup, error)
	RemoveSession(context.Context, int64) error
	EndSession(context.Context, int64) error
	// TouchSession refreshes the session's last activity so web check-ins
	// keep the session from timing out.
	TouchSession(context.Context, int64) error

	// Visits
	AdmitVisit(context.Context, studentpresence.Visit) (studentpresence.Visit, error)
	AmendVisit(context.Context, studentpresence.Visit) error
	RemoveVisit(context.Context, int64) error
	EndVisit(context.Context, int64) error
	PresenceMode(context.Context) (string, error)

	// Supervision
	AssignSupervision(context.Context, studentpresence.GroupSupervision) (studentpresence.GroupSupervision, error)
	AmendSupervision(context.Context, studentpresence.GroupSupervision) error
	RemoveSupervisionRecord(context.Context, int64) error
	EndSupervision(context.Context, int64) error
	ClaimSupervision(context.Context, int64, int64, string) (studentpresence.ClaimedSupervision, error)
	UnclaimedSessions(context.Context) ([]studentpresence.UnclaimedSession, error)

	// Combinations
	CreateCombination(context.Context, studentpresence.CombinedGroup, []int64) (studentpresence.CombinedGroup, error)
	AmendCombination(context.Context, studentpresence.CombinedGroup) (studentpresence.CombinedGroup, error)
	RemoveCombination(context.Context, int64) error
	CloseCombination(context.Context, int64) error

	// Attendance
	StudentAttendanceStatus(context.Context, int64) (*studentpresence.AttendanceStatus, error)
	StudentsAttendanceStatuses(context.Context, []int64) (map[int64]*studentpresence.AttendanceStatus, error)
	CheckOutStudent(context.Context, int64, int64) (studentpresence.CheckoutOutcome, error)

	// Moves
	AssignTransitStudents(context.Context, []int64, int64, studentpresence.StudentMoveAuthorization) (studentpresence.TransitAssignResult, error)
	MoveStudentsToSession(context.Context, []int64, int64, studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error)
	MoveStudentsToTransit(context.Context, []int64, studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error)

	// Reads
	SessionVisitsWithDisplay(context.Context, int64) ([]studentpresence.VisitDisplay, error)
	SessionRooms(context.Context, []int64) ([]studentpresence.SessionRoomSummary, error)
	DashboardAnalytics(context.Context) (studentpresence.DashboardAnalytics, error)
	CrossTenantStudents(context.Context, int64) ([]studentpresence.CrossTenantStudent, error)
	TrackingIndicators(context.Context, []int64, []string) (map[int64][]bool, error)
}

// presenceError names the failed presence read together with its cause. Its
// message keeps the historical "active: <op>: <cause>" wire text.
type presenceError struct {
	Op  string
	Err error
}

func (e *presenceError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("active: %s: unknown error", e.Op)
	}
	return fmt.Sprintf("active: %s: %v", e.Op, e.Err)
}

func (e *presenceError) Unwrap() error { return e.Err }
