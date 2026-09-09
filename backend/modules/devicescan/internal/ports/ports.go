// Package ports declares the consumer-owned seams of the device-scan
// workflow that no public owner facade serves yet: the request principals,
// the people lookups, the room stay and session transitions of the retained
// presence service, the activity catalog, the education groups, the pickup
// plan, the tenant settings, the request unit of work, and the clock.
// modules/devicescan/compose binds them; the workflow never sees the legacy
// types behind them.
package ports

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

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

// Staff is the staff member a verified account PIN bound to the request.
type Staff struct{ ID int64 }

// Principals reads the identities the device authentication bound.
type Principals interface {
	Device(ctx context.Context) (*Device, bool)
	Staff(ctx context.Context) (*Staff, bool)
}

// Person is the directory identity behind a card.
type Person struct {
	ID        int64
	FirstName string
	LastName  string
	HasTag    bool
}

// Student is the school membership of a person. Alumnus marks a graduated
// child the kiosk must treat like an unknown card.
type Student struct {
	ID          int64
	PersonID    int64
	GroupID     *int64
	SchoolClass string
	Alumnus     bool
}

// StaffMember is the staff record of a person.
type StaffMember struct{ ID int64 }

// ErrPersonNotFound reports a card that belongs to nobody.
var ErrPersonNotFound = errors.New("person not found")

// People resolves cards to persons, students and staff.
type People interface {
	// NormalizeTag canonicalizes a scanned tag.
	NormalizeTag(tag string) string
	// FindPersonByTag resolves a tag; an unknown tag is ErrPersonNotFound.
	FindPersonByTag(ctx context.Context, tag string) (*Person, error)
	// FindStudentByPerson is nil, nil when the person is no student.
	FindStudentByPerson(ctx context.Context, personID int64) (*Student, error)
	// FindStaffByPerson is nil, nil when the person is no staff member.
	FindStaffByPerson(ctx context.Context, personID int64) (*StaffMember, error)
}

// Room is the room of a session as the presence service knows it.
type Room struct {
	ID   int64
	Name string
}

// SessionRef locates the session an open visit belongs to.
type SessionRef struct {
	ID        int64
	RoomID    int64
	StartTime time.Time
	DeviceID  *int64
	Room      *Room
}

// CurrentVisit is a student's open room stay.
type CurrentVisit struct {
	ID        int64
	EntryTime time.Time
	Session   *SessionRef
}

// RoomCapacityExceeded rejects a visit into a full room.
type RoomCapacityExceeded struct {
	RoomID           int64
	RoomName         string
	CurrentOccupancy int
	MaxCapacity      int
}

func (e *RoomCapacityExceeded) Error() string {
	return fmt.Sprintf("room capacity exceeded: %s (%d/%d)", e.RoomName, e.CurrentOccupancy, e.MaxCapacity)
}

// Presence outcomes the adapters classify. The wrapped cause keeps the
// wording of the retained service.
var (
	// ErrStudentAlreadyActive reports a duplicate open visit.
	ErrStudentAlreadyActive = errors.New("student already has an active visit")
	// ErrStudentNotInCare reports a graduated child or an ended care.
	ErrStudentNotInCare = errors.New("student is not in care")
	// ErrNoAttendanceRecord reports a daily checkout without today's row.
	ErrNoAttendanceRecord = errors.New("student has no attendance record for today")
	// ErrConflict, ErrNotFound and ErrInvalid classify presence refusals
	// the kiosk renders with the retained service's own wording.
	ErrConflict = errors.New("presence conflict")
	ErrNotFound = errors.New("presence not found")
	ErrInvalid  = errors.New("presence invalid")
)

// Visits are the room stay transitions of one student.
type Visits interface {
	// Current is nil, nil when the student has no open visit.
	Current(ctx context.Context, studentID int64) (*CurrentVisit, error)
	End(ctx context.Context, visitID int64) error
	// Record opens a visit. A full room is *RoomCapacityExceeded, a
	// duplicate ErrStudentAlreadyActive, a departed child ErrStudentNotInCare.
	Record(ctx context.Context, studentID, sessionID int64) (int64, error)
}

// Activity is one activity template of the timetable.
type Activity struct {
	ID              int64
	Name            string
	MaxParticipants int
	PlannedRoomID   *int64
	IsSystem        bool
	IsOpen          bool
}

// HasParticipantLimit reports a bounded activity.
func (a Activity) HasParticipantLimit() bool { return a.MaxParticipants > 0 }

// Session is one running or ended room session.
type Session struct {
	ID         int64
	RoomID     int64
	StartTime  time.Time
	EndTime    *time.Time
	DeviceID   *int64
	TemplateID *int64
	// Activity is the loaded template when the source carried it.
	Activity *Activity
	// RoomName and ActivityName are set by lookups that join the names.
	RoomName     string
	ActivityName string
}

// Supervisor is one staff assignment of a session.
type Supervisor struct {
	StaffID int64
	Ended   bool
}

// NewSession describes the session a special room provisions on scan.
type NewSession struct {
	ActivityID int64
	RoomID     int64
}

// Sessions are the room session transitions of the retained presence service.
type Sessions interface {
	// ListOpenInRoom returns the running sessions of a room, without names.
	ListOpenInRoom(ctx context.Context, roomID int64) ([]Session, error)
	// FindDeviceSessionInRoom is nil, nil when the device runs no session there.
	FindDeviceSessionInRoom(ctx context.Context, roomID, deviceID int64) (*Session, error)
	// Current is the device's running session with its names; nil, nil when none.
	Current(ctx context.Context, deviceID int64) (*Session, error)
	Start(ctx context.Context, session NewSession) (Session, error)
	Delete(ctx context.Context, sessionID int64) error
	// End closes a session; an already ended session is not an error.
	End(ctx context.Context, sessionID int64) error
	// Touch refreshes the session heartbeat.
	Touch(ctx context.Context, sessionID int64) error
	Supervisors(ctx context.Context, sessionID int64) ([]Supervisor, error)
	ReplaceSupervisors(ctx context.Context, sessionID int64, staffIDs []int64) error
}

// AttendanceState is today's attendance row of a student.
type AttendanceState struct {
	Status       string
	Date         timezone.Date
	CheckInTime  *time.Time
	CheckOutTime *time.Time
	CheckedInBy  string
	CheckedOutBy string
}

// Attendance are the daily attendance transitions of the retained presence
// service. Refusals come back classified with the port sentinels.
type Attendance interface {
	Status(ctx context.Context, studentID int64) (*AttendanceState, error)
	// Toggle flips today's row; skipAuthCheck trusts the device layer.
	Toggle(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (string, error)
	// ConfirmDailyCheckout resolves the deferred "nach Hause" decision.
	ConfirmDailyCheckout(ctx context.Context, studentID, deviceID int64, destination string) (string, error)
}

// Category is one activity category.
type Category struct {
	ID   int64
	Name string
}

// NewCategory describes a system category to provision.
type NewCategory struct {
	Name        string
	Description string
	Color       string
	IsSystem    bool
}

// NewActivity describes a system activity to provision.
type NewActivity struct {
	Name            string
	MaxParticipants int
	IsOpen          bool
	CategoryID      int64
	PlannedRoomID   *int64
	IsSystem        bool
}

// Activities is the activity catalog the special rooms provision from.
type Activities interface {
	Find(ctx context.Context, id int64) (*Activity, error)
	ListByName(ctx context.Context, name string) ([]Activity, error)
	ListCategories(ctx context.Context) ([]Category, error)
	CreateCategory(ctx context.Context, category NewCategory) (Category, error)
	CreateActivity(ctx context.Context, activity NewActivity) (Activity, error)
}

// Group is one education group.
type Group struct {
	ID     int64
	Name   string
	RoomID *int64
}

// Groups resolves education groups.
type Groups interface {
	Find(ctx context.Context, id int64) (*Group, error)
}

// Pickup is a student's effective pickup plan on one day.
type Pickup struct {
	Time     *time.Time
	Notes    string
	DayNotes []string
}

// Pickups reads the effective pickup plan.
type Pickups interface {
	// Effective is nil, nil when no plan applies on day.
	Effective(ctx context.Context, studentID int64, day timezone.Date) (*Pickup, error)
}

// CapacityKind names a capacity-detail disclosure setting.
type CapacityKind int

const (
	CapacityRoom CapacityKind = iota
	CapacityActivity
)

// Settings resolves the tenant settings the scan flows read.
type Settings interface {
	PresenceMode(ctx context.Context) (string, error)
	FeedbackEnabled(ctx context.Context) (bool, error)
	CapacityDetailsDisclosed(ctx context.Context, kind CapacityKind) (bool, error)
	// DailyCheckoutTime is the raw "HH:MM" gate or empty when unset.
	DailyCheckoutTime(ctx context.Context) (string, error)
	PerStudentCheckoutEnabled(ctx context.Context) (bool, error)
	PerStudentCheckoutDeltaMinutes(ctx context.Context) (int, error)
	DailyCheckoutFromAllRoomsEnabled(ctx context.Context) (bool, error)
}

// UnitOfWork controls the request transaction.
type UnitOfWork interface {
	// RequireTransaction rejects writes without one surrounding transaction.
	RequireTransaction(ctx context.Context) error
	// MarkRollback requests rollback of the surrounding transaction even
	// when the response is not a server fault.
	MarkRollback(ctx context.Context)
}

// Clock supplies the instant a request is admitted and the school's
// calendar day of an instant.
type Clock interface {
	Now() time.Time
	Day(time.Time) timezone.Date
}
