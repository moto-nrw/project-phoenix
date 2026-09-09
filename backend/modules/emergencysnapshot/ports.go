package emergencysnapshot

import (
	"context"
	"time"
)

// Date is a calendar day (YYYY-MM-DD) without clock or zone, the shape the
// owner contracts the ports adapt use. Every port reads the tenant from the
// context; none of them writes.
type Date string

// PresenceMode is the school's presence concept: "detailed" tracks the room
// a child is in, "binary" only tracks whether the child is on the premises.
type PresenceMode string

const (
	PresenceModeDetailed PresenceMode = "detailed"
	PresenceModeBinary   PresenceMode = "binary"
)

// School statuses of the binary presence mode, as the Student Presence owner
// reports them.
const (
	StatusCheckedIn = "checked_in"
	StatusOnYard    = "on_yard"
)

// Presence reads the live presence facts of the Student Presence owner.
type Presence interface {
	// PresentStudentIDs lists the students with an open attendance on the
	// day, in the owner's order.
	PresentStudentIDs(ctx context.Context, date Date) ([]int64, error)
	// Mode resolves the school's presence concept.
	Mode(ctx context.Context) (PresenceMode, error)
	// CurrentRoomIDs maps each student with an open visit in a running
	// session to that session's room. Students without one are absent.
	CurrentRoomIDs(ctx context.Context, studentIDs []int64) (map[int64]int64, error)
	// SchoolStatuses reports the binary-mode status of each student on the
	// day (StatusCheckedIn, StatusOnYard, or another owner status). Students
	// without a status are absent.
	SchoolStatuses(ctx context.Context, studentIDs []int64, date Date) (map[int64]string, error)
}

// Rooms resolves room names from the Facilities owner.
type Rooms interface {
	// Names maps the ids the tenant can see to their names. Unknown ids are
	// absent.
	Names(ctx context.Context, roomIDs []int64) (map[int64]string, error)
}

// Student is the projection-relevant record of one child. The legacy
// guardian fields are the free-text contact columns of the student row;
// they are folded into the contact columns behind the linked guardians.
type Student struct {
	ID              int64
	PersonID        int64
	SchoolClass     string
	HealthInfo      string
	GuardianName    string
	GuardianContact string
	GuardianPhone   string
}

// Students reads student records from the People Directory owner, which
// owns users.students.
type Students interface {
	// ByIDs maps the requested ids the tenant can see to their records.
	ByIDs(ctx context.Context, studentIDs []int64) (map[int64]Student, error)
}

// Person is the display identity of one child.
type Person struct {
	ID        int64
	FirstName string
	LastName  string
}

// Persons reads display identities from the People Directory owner.
type Persons interface {
	// ByIDs maps the requested ids the tenant can see to their records.
	ByIDs(ctx context.Context, personIDs []int64) (map[int64]Person, error)
}

// Contact is one (guardian, phone number) pair of a child. A guardian
// without a phone number yields a Contact with an empty Phone.
type Contact struct {
	StudentID int64
	FirstName string
	LastName  string
	Phone     string
}

// Contacts reads the linked guardians of students from the People Directory
// owner. The rows arrive in the owner's order: emergency contacts and
// primary guardians first, then primary and priority phone numbers.
type Contacts interface {
	EmergencyContacts(ctx context.Context, studentIDs []int64) ([]Contact, error)
}

// Settings reads the tenant settings the projection depends on.
type Settings interface {
	// HealthInfoEnabled reports whether the school prints health notes on
	// the Notfallliste (operations.emergency_list_health_info).
	HealthInfoEnabled(ctx context.Context) (bool, error)
}

// Calendar supplies the tenant's calendar day.
type Calendar interface {
	// DayOf is the calendar day (YYYY-MM-DD) of the instant.
	DayOf(at time.Time) Date
}

// Renderer turns the projected document into a downloadable file through
// the Document Rendering platform.
type Renderer interface {
	// RenderPDF renders the document as a PDF named filenameBase plus the
	// format's extension.
	RenderPDF(doc Document, filenameBase string) (File, error)
}
