package domain

import (
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The values the care lifecycle (#3427) receives from the owners it drives.
// Care Plan decides on them; the ports in internal/ports carry them.

// CareStudent is the People Directory row the lifecycle decides on.
type CareStudent struct {
	ID            int64
	PersonID      int64
	SchoolClass   string
	Status        string
	EnrolledFrom  *calendar.Date
	EnrolledUntil *calendar.Date
	UpdatedAt     time.Time
}

// CareEndedOn reports whether the care has already ended on day. The
// enrolment interval's upper bound is inclusive, so the last care day itself
// still counts as care.
func (s CareStudent) CareEndedOn(day calendar.Date) bool {
	return s.EnrolledUntil != nil && day.After(*s.EnrolledUntil)
}

// IsAlumnus reports whether the child left with a grade transition.
func (s CareStudent) IsAlumnus() bool { return s.Status == careplan.StudentStatusAlumnus }

// CarePerson is the name and bracelet state of one person row.
type CarePerson struct {
	FirstName  string
	LastName   string
	HasRFIDTag bool
}

// CareExitBooking is one activity booking a care exit deleted, as the
// Timetable owner reported it.
type CareExitBooking struct {
	ID                       int64
	TenantID                 int64
	StudentID                int64
	ActivityGroupID          int64
	ValidFrom                calendar.Date
	ValidUntil               *calendar.Date
	CalendarPeriodID         *int64
	EnrollmentRequestChildID *int64
	SelectedWeekdays         []int
	AttendanceStatus         *string
	Weekday                  *int
}

// CareExitBookingCap is one activity booking a care exit ended early.
type CareExitBookingCap struct {
	StudentID          int64
	ID                 int64
	PreviousValidUntil *calendar.Date
}

// CareExitBookingChanges is what ending the bookings did.
type CareExitBookingChanges struct {
	Deleted []CareExitBooking
	Capped  []CareExitBookingCap
}

// CareExitBookingRestore is one ledger entry the Timetable owner replays.
type CareExitBookingRestore struct {
	CareExitBooking
	WasDeleted         bool
	PreviousValidUntil *calendar.Date
}

// CareExitApplication is one Enrollment application child linked to a
// student. Approved reports Enrollment's own decision; only approved
// applications book care.
type CareExitApplication struct {
	ID               int64
	TenantID         int64
	CreatedStudentID *int64
	MatchedStudentID *int64
	Approved         bool
}

// CareExitOfferingLink is one source booking of an application child.
type CareExitOfferingLink struct {
	ID             int64
	TenantID       int64
	RequestChildID int64
	CareOfferingID int64
	SelectedDays   []string
	ValidFrom      *calendar.Date
	ValidUntil     *calendar.Date
}

// CareExitOfferingSnapshot is the verbatim previous image of one source
// booking a care exit ends or deletes.
type CareExitOfferingSnapshot struct {
	TenantID       int64
	StudentID      int64
	RequestChildID int64
	SourceRowID    int64
	WasDeleted     bool
	Snapshot       json.RawMessage
}

// CareExitOfferingRestore names one snapshot the restore replays.
type CareExitOfferingRestore struct {
	SourceRowID int64
	WasDeleted  bool
	Snapshot    json.RawMessage
}

// CareBookingStudent is the identity and enrolment bound of one child the
// booking evaluation interprets.
type CareBookingStudent struct {
	StudentID     int64
	FirstName     string
	LastName      string
	SchoolClass   string
	EnrolledUntil *calendar.Date
}

// EndedCareStudent is the directory half of one archived child.
type EndedCareStudent struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
	LastCareDay calendar.Date
}
