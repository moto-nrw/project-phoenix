package studentpresence

import (
	"errors"
	"fmt"
	"time"
)

// Presence modes resolved from the tenant's presence_mode setting.
const (
	PresenceModeDetailed = "detailed"
	PresenceModeBinary   = "binary"
)

// Operation errors shared by the presence commands and their consumers. The
// messages are wire contracts: the web routes render them verbatim and the
// kiosk maps several of them to German text.
var (
	ErrGroupSupervisorNotFound   = errors.New("group supervisor not found")
	ErrGroupMappingNotFound      = errors.New("group mapping not found")
	ErrStaffNotFound             = errors.New("staff member not found")
	ErrStudentNotFound           = errors.New("student not found")
	ErrStudentGraduated          = errors.New("student has graduated and cannot check in")
	ErrStudentCareEnded          = errors.New("student care has ended and cannot check in")
	ErrGroupAlreadyEnded         = errors.New("active group session already ended")
	ErrVisitAlreadyEnded         = errors.New("visit already ended")
	ErrSupervisionAlreadyEnded   = errors.New("supervision already ended")
	ErrCombinedGroupAlreadyEnded = errors.New("combined group already ended")
	ErrStudentAlreadyInGroup     = errors.New("student already present in this group")
	ErrGroupAlreadyInCombination = errors.New("group already part of this combination")
	ErrInvalidTimeRange          = errors.New("invalid time range")
	ErrCannotDeleteActiveGroup   = errors.New("cannot delete active group with active visits")
	ErrStudentAlreadyActive      = errors.New("student already has an active visit")
	ErrStaffAlreadySupervising   = errors.New("staff member already supervising this group")
	ErrStudentsNotPresent        = errors.New("no requested students are currently present")
	ErrStudentMoveForbidden      = errors.New("not authorized to move the selected students")
	ErrInvalidData               = errors.New("invalid data provided")
	ErrDatabaseOperation         = errors.New("database operation failed")
	ErrRoomConflict              = errors.New("room is already occupied by another active group")
	ErrRoomCapacityExceeded      = errors.New("room capacity exceeded")
	// ErrActivityParticipantLimitExceeded classifies
	// ActivityParticipantLimitError with errors.Is (#3632).
	ErrActivityParticipantLimitExceeded = errors.New("activity participant limit exceeded")
	ErrNoRoomAvailable                  = errors.New("no room available for this activity")
	// ErrNoAttendanceRecordForCheckout is returned by ConfirmDailyCheckout when
	// the student has no attendance record for today — a daily checkout makes no
	// sense because the student was never checked in. The message is a cross-repo
	// contract mapped to German UI text in PyrePortal; do not change it.
	ErrNoAttendanceRecordForCheckout = errors.New("student has no attendance record for today")
	// Kiosk session errors. ErrDeviceAlreadyActive and ErrNoActiveSession are
	// PyrePortal contract strings; do not change them.
	ErrDeviceAlreadyActive    = errors.New("device is already running an activity session")
	ErrNoActiveSession        = errors.New("no active session found")
	ErrSessionConflict        = errors.New("session conflict detected")
	ErrInvalidActivitySession = errors.New("invalid activity session parameters")
)

// OperationError names the presence operation that failed around the
// classified sentinel. Consumers classify the wrapped error with errors.Is;
// the kiosk mapping also inspects the outer wrapper.
type OperationError struct {
	Op  string // Operation that failed
	Err error  // Underlying error
}

// Error returns the error message
func (e *OperationError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("active: %s: unknown error", e.Op)
	}
	return fmt.Sprintf("active: %s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *OperationError) Unwrap() error {
	return e.Err
}

// RoomCapacityError reports a presence admission beyond the room's capacity.
type RoomCapacityError struct {
	RoomID           int64
	RoomName         string
	CurrentOccupancy int
	MaxCapacity      int
}

func (e *RoomCapacityError) Error() string {
	return fmt.Sprintf("room capacity exceeded: %s (%d/%d)", e.RoomName, e.CurrentOccupancy, e.MaxCapacity)
}

func (e *RoomCapacityError) Unwrap() error { return ErrRoomCapacityExceeded }

// ActivityParticipantLimitCode is the stable error code of a web assignment
// refused because the activity's participant limit is reached (#3632).
// Clients map it to their own text.
const ActivityParticipantLimitCode = "presence.activity_participant_limit_reached"

// ActivityParticipantLimitError refuses a web assignment that would put more
// children into a session than its activity's participant limit allows.
// CurrentOccupancy is the number of open visits of the session before the
// write, Incoming the number of children the write would add. Nothing of the
// write is applied, a bulk assignment included.
//
// It is a business rejection: ErrorCode and ErrorDetails let an HTTP adapter
// answer 409 with the code and the numbers. It stays distinct from
// RoomCapacityError, which limits the room rather than the activity.
type ActivityParticipantLimitError struct {
	ActivityID       int64
	ActivityName     string
	CurrentOccupancy int
	MaxParticipants  int
	Incoming         int
}

func (e *ActivityParticipantLimitError) Error() string {
	return fmt.Sprintf("activity participant limit exceeded: activity %d (%d/%d, %d incoming)",
		e.ActivityID, e.CurrentOccupancy, e.MaxParticipants, e.Incoming)
}

func (e *ActivityParticipantLimitError) Unwrap() error { return ErrActivityParticipantLimitExceeded }

func (e *ActivityParticipantLimitError) ErrorCode() string { return ActivityParticipantLimitCode }

// ActivityParticipantLimitDetails are the values a refusal names, in wire form.
type ActivityParticipantLimitDetails struct {
	ActivityID       int64  `json:"activity_id"`
	ActivityName     string `json:"activity_name"`
	CurrentOccupancy int    `json:"current_occupancy"`
	MaxParticipants  int    `json:"max_participants"`
	IncomingStudents int    `json:"incoming_students"`
}

func (e *ActivityParticipantLimitError) ErrorDetails() any {
	return ActivityParticipantLimitDetails{
		ActivityID: e.ActivityID, ActivityName: e.ActivityName,
		CurrentOccupancy: e.CurrentOccupancy, MaxParticipants: e.MaxParticipants, IncomingStudents: e.Incoming,
	}
}

// AttendanceStatus is a student's school attendance for one calendar day,
// with the check-in and check-out staff resolved to display names.
type AttendanceStatus struct {
	StudentID int64
	// Status is not_checked_in, checked_in, on_yard or checked_out.
	Status       string
	Date         string
	CheckInTime  *time.Time
	CheckOutTime *time.Time
	YardSince    *time.Time
	CheckedInBy  string
	CheckedOutBy string
}

// CheckoutOutcome is the outcome of an explicit school checkout.
type CheckoutOutcome struct {
	Action       string
	AttendanceID int64
}

// StudentMoveAuthorization carries the caller facts a bulk move revalidates
// against the locked move state inside the command.
type StudentMoveAuthorization struct {
	StaffID              int64
	BypassResourceChecks bool
	// SchoolWideAttendanceEligible is set only after the inbound adapter
	// verified an OGS staff actor with the move permission.
	SchoolWideAttendanceEligible bool
}

// StudentMoveSkipped names a student a move could not relocate.
type StudentMoveSkipped struct {
	StudentID int64  `json:"student_id"`
	Reason    string `json:"reason"`
}

// StudentMoveResult is the wire result of a bulk room or transit move.
type StudentMoveResult struct {
	Moved         []int64              `json:"moved"`
	Unchanged     []int64              `json:"unchanged"`
	Skipped       []StudentMoveSkipped `json:"skipped"`
	ActiveGroupID *int64               `json:"active_group_id,omitempty"`
	RoomID        *int64               `json:"room_id,omitempty"`
	// PreviousActiveGroupIDs records the source observed after move serialization.
	// It is service metadata and intentionally not part of the bulk-move API.
	PreviousActiveGroupIDs map[int64]int64 `json:"-"`
}

// Reasons a transit assignment or a move leaves a student out.
const (
	TransitSkipNotInTransit = "not_in_transit"
	TransitSkipCreateFailed = "create_failed"

	StudentMoveSkipNotPresent = "not_present"
	StudentMoveSkipConflict   = "conflict"
)

// TransitAssignSkipped names a student a transit assignment left out.
type TransitAssignSkipped struct {
	StudentID int64  `json:"student_id"`
	Reason    string `json:"reason"`
}

// TransitAssignResult is the wire result of assigning transit students to a
// room session.
type TransitAssignResult struct {
	Assigned      []int64                `json:"assigned"`
	Skipped       []TransitAssignSkipped `json:"skipped"`
	ActiveGroupID int64                  `json:"active_group_id"`
	RoomID        int64                  `json:"room_id"`
}

// VisitDisplay joins an open visit with the tenant-scoped student display
// facts the supervision views render.
type VisitDisplay struct {
	VisitID       int64
	StudentID     int64
	PersonID      int64
	ActiveGroupID int64
	EntryTime     time.Time
	ExitTime      *time.Time
	FirstName     string
	LastName      string
	SchoolClass   string
	GroupID       *int64 // student's education group_id (nullable)
	OGSGroupName  string
	Sick          *bool
	SickSince     *time.Time
	Excused       *bool
	ExcusedSince  *time.Time
	PhotoPath     *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// SessionRoomSummary is the room projection presence responses embed.
type SessionRoomSummary struct {
	ID       int64
	Name     string
	Building string
	Category *string
	Color    *string
}

// SessionActivitySummary is the activity template projection presence
// responses embed.
type SessionActivitySummary struct {
	ID              int64
	Name            string
	MaxParticipants int
	PlannedRoomID   *int64
	IsSystem        bool
	IsOpen          bool
}

// UnclaimedSession is an open session without supervisors together with the
// room and template projections the claiming view renders.
type UnclaimedSession struct {
	UnclaimedGroup
	Room     *SessionRoomSummary
	Activity *SessionActivitySummary
}

// CrossTenantStudent is a child hosted from another school.
type CrossTenantStudent struct {
	StudentID  int64
	FirstName  string
	LastName   string
	GroupName  string
	HomeTenant string
}

// DashboardAnalytics aggregates the tenant's presence for the dashboard.
type DashboardAnalytics struct {
	StudentsPresent      int
	StudentsInTransit    int
	StudentsOnPlayground int
	StudentsInRooms      int
	StudentsSick         int
	StudentsExcused      int
	StudentsHome         int
	// StudentsAtSchool are expected children before their first check-in
	// (#3260); StudentsHome no longer counts them.
	StudentsAtSchool int

	ActiveActivities    int
	FreeRooms           int
	TotalRooms          int
	CapacityUtilization float64
	ActivityCategories  int

	ActiveOGSGroups      int
	StudentsInGroupRooms int
	SupervisorsToday     int
	StudentsInHomeRoom   int

	RecentActivity      []RecentActivity
	CurrentActivities   []CurrentActivity
	ActiveGroupsSummary []ActiveGroupInfo

	LastUpdated time.Time
}

// RecentActivity is a recent presence change without personal data.
type RecentActivity struct {
	Type      string
	GroupName string
	RoomName  string
	Count     int
	Timestamp time.Time
}

// CurrentActivity is a running activity's occupancy.
type CurrentActivity struct {
	ID           int64
	Name         string
	Category     string
	Participants int
	MaxCapacity  *int
	Status       string
}

// ActiveGroupInfo summarizes one running group for the dashboard.
type ActiveGroupInfo struct {
	Name         string
	Type         string
	StudentCount int
	Location     string
	Status       string
}
