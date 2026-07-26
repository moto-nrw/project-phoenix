package checkin

import (
	"fmt"
	"time"
)

// Typed errors for the RFID check-in workflow (issue #575 B8).
//
// The CheckinService performs no HTTP rendering. Business methods that used to
// render an error mid-flow now return one of the typed errors below; the
// api/iot/checkin handler maps each to the exact renderer it rendered before,
// preserving the PyrePortal wire contract byte-for-byte.
//
// WIRE CONTRACT (PyrePortal): the message strings in checkinErrMsg* below are
// substring-matched by the kiosk (PyrePortal/src/services/apiErrors.ts) and
// pinned by TestPyrePortalErrorStringsGuard (which scans this file via
// extraGuardSources). Changing one silently breaks PyrePortal's German UI
// mapping — coordinate a matching PyrePortal change.

// checkinErrorKind classifies a CheckinError so the handler can pick the
// matching api/common renderer without re-deriving intent from the message.
type checkinErrorKind int

const (
	// KindInternal → common.ErrorInternalServer (500, message on the wire).
	KindInternal checkinErrorKind = iota
	// KindInternalWrap → common.ErrorInternalServerWrap (500, message on the
	// wire, cause preserved for Sentry/slog).
	KindInternalWrap
	// KindNotFound → common.ErrorNotFound (404).
	KindNotFound
	// KindInvalidRequest → common.ErrorInvalidRequest (400).
	KindInvalidRequest
)

// CheckinError is a classified, renderer-agnostic error returned by the
// CheckinService for the non-structured error paths (internal failures,
// missing rooms/sessions, bad requests). The handler switches on Kind to
// choose the matching api/common renderer; Message becomes the wire text.
type CheckinError struct {
	Kind    checkinErrorKind
	Message string
	// Cause is preserved for KindInternalWrap (and Sentry/slog on any kind).
	Cause error
}

func (e *CheckinError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *CheckinError) Unwrap() error { return e.Cause }

func newInternalError(msg string) *CheckinError {
	return &CheckinError{Kind: KindInternal, Message: msg}
}

func newInternalWrapError(msg string, cause error) *CheckinError {
	return &CheckinError{Kind: KindInternalWrap, Message: msg, Cause: cause}
}

func newNotFoundError(msg string) *CheckinError {
	return &CheckinError{Kind: KindNotFound, Message: msg}
}

func newInvalidRequestError(msg string) *CheckinError {
	return &CheckinError{Kind: KindInvalidRequest, Message: msg}
}

// Message constants. Kept as a central block so the PyrePortal guard has a
// single, greppable home and so the handler mapping and these producers stay
// in sync.
const (
	checkinErrEndVisit          = "failed to end visit record"
	checkinErrCreateVisit       = "failed to create visit record"
	checkinErrGetRoom           = "failed to get room information"
	checkinErrCheckRoomCapacity = "failed to check room capacity"
	checkinErrGetActivity       = "failed to get activity information"
	checkinErrCheckActivityCap  = "failed to check activity capacity"
	checkinErrFindActiveGroups  = "error finding active groups in room"
	checkinErrSpontaneous       = "spontaneous sessions are not supported on this check-in path"
	checkinErrSchulhofNotConfig = "schulhof activity not configured"
	checkinErrWCNotConfig       = "WC activity not configured"
	checkinErrWCNeedsStaff      = "WC activity auto-create requires staff context"
	checkinErrCreateSchulhof    = "failed to create Schulhof session"
	checkinErrCreateWC          = "failed to create WC session"
	checkinErrCreateSession     = "failed to create session"
	checkinErrNoGroupsInRoom    = "no active groups in specified room"
	checkinErrNoActiveSession   = "no active session - please start an activity first"
	checkinErrLoadSupervisors   = "failed to load session supervisors"
	checkinErrUpdateSupervisors = "failed to update session supervisors"
	checkinErrRoomIDRequired    = "room_id is required for check-in"
)

// RoomCapacityError carries the room-occupancy context for a 409 room-capacity
// conflict. The handler converts it 1:1 to the capacity renderer that emits the
// pinned ROOM_CAPACITY_EXCEEDED wire body.
type RoomCapacityError struct {
	RoomID           int64
	RoomName         string
	CurrentOccupancy int
	MaxCapacity      int
}

func (e *RoomCapacityError) Error() string {
	return fmt.Sprintf("room capacity exceeded: %s (%d/%d)", e.RoomName, e.CurrentOccupancy, e.MaxCapacity)
}

// ActivityCapacityError carries the activity-occupancy context for a 409
// activity-capacity conflict. The handler converts it 1:1 to the capacity
// renderer that emits the pinned ACTIVITY_CAPACITY_EXCEEDED wire body.
type ActivityCapacityError struct {
	ActivityID       int64
	ActivityName     string
	CurrentOccupancy int
	MaxCapacity      int
}

func (e *ActivityCapacityError) Error() string {
	return fmt.Sprintf("activity capacity exceeded: %s (%d/%d)", e.ActivityName, e.CurrentOccupancy, e.MaxCapacity)
}

// StudentAlreadyActiveConflict carries the existing-visit context that the
// kiosk surfaces when a duplicate check-in is rejected (409). All fields except
// StudentID are optional: ExistingVisitID is zero when the prior visit couldn't
// be reloaded, RoomID/RoomName are empty when it had no room. The handler
// converts this 1:1 to the STUDENT_ALREADY_ACTIVE renderer.
type StudentAlreadyActiveConflict struct {
	StudentID       int64
	ExistingVisitID int64
	EntryTime       *time.Time
	RoomID          *int64
	RoomName        string
}

func (e *StudentAlreadyActiveConflict) Error() string {
	// Canonical phrasing is services/active.ErrStudentAlreadyActive; the
	// handler emits that via the STUDENT_ALREADY_ACTIVE renderer.
	return "student already has an active visit"
}
