// Package ports declares the consumer-owned seams of the device-scan
// workflow that no public owner facade serves yet: the request principals,
// the people lookups, the room stay and session transitions of the retained
// presence service, the activity catalog, the education groups, the pickup
// plan, the tenant settings, the request unit of work, and the clock. The
// block rosters are the one seam bound to a public owner, Timetable &
// Activities. modules/devicescan/compose binds them; the workflow never sees
// the legacy types behind them.
package ports

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Person is the directory identity behind a card.
type Person struct {
	ID        int64
	FirstName string
	LastName  string
	HasTag    bool
	TagID     *string
}

// Student is the school membership of a person. Alumnus marks a graduated
// child the kiosk must treat like an unknown card.
type Student struct {
	ID            int64
	PersonID      int64
	GroupID       *int64
	SchoolClass   string
	Alumnus       bool
	EnrolledUntil *calendar.Date
}

// StaffMember is the staff record of a person.
type StaffMember struct{ ID, PersonID int64 }

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
	// ErrNoRoomAvailable narrows ErrInvalid: a session start without a room
	// for an activity that has none planned.
	ErrNoRoomAvailable = errors.New("presence no room available")
	// ErrNoActiveSession narrows ErrNotFound for the timeout refusal.
	ErrNoActiveSession = errors.New("presence no active session")
)

// Visits are the room stay transitions of one student.
type Visits interface {
	// Current is nil, nil when the student has no open visit.
	Current(ctx context.Context, studentID int64) (*CurrentVisit, error)
	EndVisit(ctx context.Context, visitID int64) error
	// Record opens a visit. A full room is *RoomCapacityExceeded, a
	// duplicate ErrStudentAlreadyActive, a departed child ErrStudentNotInCare.
	Record(ctx context.Context, studentID, sessionID int64) (int64, error)
}

// AttendanceState is today's attendance row of a student.
type AttendanceState struct {
	Status       string
	Date         calendar.Date
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

// Rosters reads the day rosters of running timetable blocks.
type Rosters interface {
	// SessionsRosteringStudent returns those of sessionIDs whose running
	// block lists the student in its day roster.
	SessionsRosteringStudent(ctx context.Context, studentID int64, sessionIDs []int64) ([]int64, error)
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
	Effective(ctx context.Context, studentID int64, day calendar.Date) (*Pickup, error)
}

// CapacityKind names a capacity-detail disclosure setting.
type CapacityKind int

const (
	CapacityRoom CapacityKind = iota
	CapacityActivity
)

// Settings resolves the tenant settings the scan flows read.
type Settings interface {
	GetPresenceMode(ctx context.Context) (string, error)
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
	Day(time.Time) calendar.Date
}
