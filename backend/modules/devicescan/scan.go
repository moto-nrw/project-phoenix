package devicescan

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Wire strings of the kiosk scan flows. PyrePortal substring-matches these
// (PyrePortal/src/services/apiErrors.ts) and the guard test in api/iot pins
// them; the HTTP adapter puts them on the wire unchanged (#2698).
const (
	MessageRFIDTagNotFound              = "RFID tag not found"
	CodeRFIDTagNotFound                 = "rfid_tag_not_found"
	MessageRFIDTagUnassigned            = "RFID tag not assigned to any person"
	MessageRFIDTagNotStudentOrStaff     = "RFID tag not assigned to student or staff"
	MessagePersonNotStudent             = "person is not a student"
	MessageStudentRFIDRequiredForPickup = "student RFID tag required for pickup query"
	MessageRFIDParameterRequired        = "RFID parameter is required"
	MessageInternalServerError          = "Internal server error"
	MessageDeviceAPIKeyRequired         = "device API key is required"

	MessageEndVisitFailed          = "failed to end visit record"
	MessageCreateVisitFailed       = "failed to create visit record"
	MessageGetRoomFailed           = "failed to get room information"
	MessageCheckRoomCapacityFailed = "failed to check room capacity"
	MessageGetActivityFailed       = "failed to get activity information"
	MessageCheckActivityCapFailed  = "failed to check activity capacity"
	MessageFindActiveGroupsFailed  = "error finding active groups in room"
	MessageSpontaneousUnsupported  = "spontaneous sessions are not supported on this check-in path"
	MessageSchulhofNotConfigured   = "schulhof activity not configured"
	MessageWCNotConfigured         = "WC activity not configured"
	MessageWCNeedsStaff            = "WC activity auto-create requires staff context"
	MessageCreateSchulhofFailed    = "failed to create Schulhof session"
	MessageCreateWCFailed          = "failed to create WC session"
	MessageCreateSessionFailed     = "failed to create session"
	MessageNoGroupsInRoom          = "no active groups in specified room"
	MessageNoActiveSession         = "no active session - please start an activity first"
	MessageLoadSupervisorsFailed   = "failed to load session supervisors"
	MessageUpdateSupervisorsFailed = "failed to update session supervisors"
	MessageRoomIDRequired          = "room_id is required for check-in"
	MessageStudentAlreadyActive    = "student already has an active visit"
)

// Scan actions the kiosk renders.
const (
	ScanActionCheckedIn               = "checked_in"
	ScanActionCheckedOut              = "checked_out"
	ScanActionCheckedOutDaily         = "checked_out_daily"
	ScanActionTransferred             = "transferred"
	ScanActionNoAction                = "no_action"
	ScanActionSupervisorAuthenticated = "supervisor_authenticated"
	ScanActionPickupInfo              = "pickup_info"
)

// Attendance toggle actions and destinations the kiosk sends.
const (
	AttendanceActionConfirm       = "confirm"
	AttendanceActionCancel        = "cancel"
	AttendanceActionDailyCheckout = "confirm_daily_checkout"
	AttendanceActionCancelled     = "cancelled"

	DestinationHome    = "zuhause"
	DestinationTransit = "unterwegs"
)

// FailureKind is the HTTP class a classified refusal renders as.
type FailureKind int

const (
	FailureInternal FailureKind = iota
	FailureUnauthorized
	FailureInvalidRequest
	FailureNotFound
	FailureConflict
)

// Failure is a classified kiosk refusal. Message is the text on the wire,
// Code the optional stable code PyrePortal maps, and Cause the logged
// reason that never reaches the device.
type Failure struct {
	Kind    FailureKind
	Message string
	Code    string
	Cause   error
}

func (f *Failure) Error() string {
	if f.Cause != nil {
		return fmt.Sprintf("%s: %v", f.Message, f.Cause)
	}
	return f.Message
}

func (f *Failure) Unwrap() error { return f.Cause }

// NotFound classifies a refusal the kiosk renders as 404.
func NotFound(message string) *Failure { return &Failure{Kind: FailureNotFound, Message: message} }

// NotFoundWithCode classifies a 404 that carries a stable code.
func NotFoundWithCode(message, code string) *Failure {
	return &Failure{Kind: FailureNotFound, Message: message, Code: code}
}

// InvalidRequest classifies a refusal the kiosk renders as 400.
func InvalidRequest(message string) *Failure {
	return &Failure{Kind: FailureInvalidRequest, Message: message}
}

// Conflict classifies a refusal the kiosk renders as 409.
func Conflict(message string) *Failure { return &Failure{Kind: FailureConflict, Message: message} }

// Internal classifies a server fault. The message is the wire text, the
// cause stays in the log.
func Internal(message string, cause error) *Failure {
	return &Failure{Kind: FailureInternal, Message: message, Cause: cause}
}

// ErrDeviceUnauthorized reports a request without an authenticated device.
var ErrDeviceUnauthorized = &Failure{Kind: FailureUnauthorized, Message: MessageDeviceAPIKeyRequired}

// RoomCapacityExceededError rejects a check-in into a full room. Details
// reports whether the tenant lets the kiosk show the occupancy figures.
type RoomCapacityExceededError struct {
	RoomID           int64
	RoomName         string
	CurrentOccupancy int
	MaxCapacity      int
	Details          bool
}

func (e *RoomCapacityExceededError) Error() string {
	return fmt.Sprintf("room capacity exceeded: %s (%d/%d)", e.RoomName, e.CurrentOccupancy, e.MaxCapacity)
}

// ActivityCapacityExceededError rejects a check-in into a full activity.
type ActivityCapacityExceededError struct {
	ActivityID       int64
	ActivityName     string
	CurrentOccupancy int
	MaxCapacity      int
	Details          bool
}

func (e *ActivityCapacityExceededError) Error() string {
	return fmt.Sprintf("activity capacity exceeded: %s (%d/%d)", e.ActivityName, e.CurrentOccupancy, e.MaxCapacity)
}

// StudentAlreadyActiveError rejects a duplicate check-in. Every field but
// StudentID is optional: the existing visit may have closed concurrently.
type StudentAlreadyActiveError struct {
	StudentID       int64
	ExistingVisitID int64
	EntryTime       *time.Time
	RoomID          *int64
	RoomName        string
}

func (e *StudentAlreadyActiveError) Error() string { return MessageStudentAlreadyActive }

// Device is the authenticated kiosk of a request.
type Device struct {
	ID         int64
	DeviceID   string
	DeviceType string
	Name       *string
	Status     string
	LastSeen   *time.Time
	Active     bool
}

// ScanCommand is one card scan at a kiosk. RoomID is the room the kiosk
// checks the child into; nil means a plain checkout.
type ScanCommand struct {
	RFIDTag string
	RoomID  *int64
}

// ScanOutcome names the wire shape of a scan result.
type ScanOutcome int

const (
	// ScanOutcomeVisit is a detailed-mode room stay transition.
	ScanOutcomeVisit ScanOutcome = iota
	// ScanOutcomeAttendance is a binary-mode attendance toggle.
	ScanOutcomeAttendance
	// ScanOutcomeSupervisor is a staff card that joined the device session.
	ScanOutcomeSupervisor
)

// ScanResult is the kiosk projection of a processed scan.
type ScanResult struct {
	Outcome ScanOutcome
	// PersonID is the student id, or the staff id for a supervisor scan.
	PersonID   int64
	PersonName string
	Action     string
	Message    string
	RoomName   string
	// VisitID is set for visit outcomes; nil means no visit was touched.
	VisitID                *int64
	PreviousRoomName       string
	DailyCheckoutAvailable bool
	FeedbackEnabled        bool
	ActiveStudents         *int
	PickupTime             *string
	ProcessedAt            time.Time
}

// PickupInfo is the read-only pickup projection of a student scan.
type PickupInfo struct {
	StudentID   int64
	StudentName string
	// PickupTime is "HH:MM" or empty when no plan applies today.
	PickupTime  string
	PickupNote  string
	ProcessedAt time.Time
}

// DevicePing is the kiosk heartbeat answer.
type DevicePing struct {
	Device        Device
	IsOnline      bool
	SessionActive bool
	PingTime      time.Time
}

// DeviceStatus is the authenticated device's own view.
type DeviceStatus struct {
	Device          Device
	IsOnline        bool
	AuthenticatedAt time.Time
}

// AttendanceGroup names the child's education group.
type AttendanceGroup struct {
	ID   int64
	Name string
}

// AttendanceStudent is the child an attendance answer is about.
type AttendanceStudent struct {
	ID        int64
	FirstName string
	LastName  string
	Group     *AttendanceGroup
}

// AttendanceState is today's attendance row as the kiosk shows it. Date is
// the public calendar string (YYYY-MM-DD) of the port's canonical Date.
type AttendanceState struct {
	Status       string
	Date         string
	CheckInTime  *time.Time
	CheckOutTime *time.Time
	CheckedInBy  string
	CheckedOutBy string
}

// AttendanceStatus answers the attendance status lookup.
type AttendanceStatus struct {
	Student    AttendanceStudent
	Attendance AttendanceState
}

// AttendanceToggleCommand is one attendance action the kiosk confirms.
type AttendanceToggleCommand struct {
	RFIDTag     string
	Action      string
	Destination string
}

// AttendanceToggleResult is the state after an attendance action. Attendance
// is nil for the daily checkout, whose wire shape carries no row.
type AttendanceToggleResult struct {
	Action          string
	Student         AttendanceStudent
	Attendance      *AttendanceState
	Message         string
	FeedbackEnabled *bool
}

// DeviceIdentity names the authenticated device of a request.
type DeviceIdentity interface {
	// Device returns the authenticated device or ErrDeviceUnauthorized.
	Device(ctx context.Context) (Device, error)
}

// Scanner is the student and supervisor scan contract of the kiosk.
type Scanner interface {
	DeviceIdentity
	// Scan processes one card scan: a student check-in, checkout or
	// transfer, a binary-mode attendance toggle, or a supervisor joining
	// the device session. Requires an existing tenant transaction; the HTTP
	// middleware supplies it. No mutation occurs if that transaction is absent.
	Scan(ctx context.Context, command ScanCommand) (*ScanResult, error)
	// PickupInfo reads today's pickup time and notes without writing.
	PickupInfo(ctx context.Context, rfidTag string) (*PickupInfo, error)
	// Ping records device activity and refreshes the device session.
	Ping(ctx context.Context) (*DevicePing, error)
	// Status reports the authenticated device.
	Status(ctx context.Context) (*DeviceStatus, error)
}

// Attendance is the daily attendance contract of the kiosk.
type Attendance interface {
	DeviceIdentity
	AttendanceStatus(ctx context.Context, rfidTag string) (*AttendanceStatus, error)
	// ToggleAttendance requires an existing tenant transaction.
	ToggleAttendance(ctx context.Context, command AttendanceToggleCommand) (*AttendanceToggleResult, error)
}

// DeviceScan is the whole kiosk scan capability.
type DeviceScan interface {
	Scanner
	Attendance
}

// IsFailure reports the classified refusal behind err, if any.
func IsFailure(err error) (*Failure, bool) {
	return errors.AsType[*Failure](err)
}
