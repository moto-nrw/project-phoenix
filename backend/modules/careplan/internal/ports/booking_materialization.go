package ports

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ErrBookingRowNotFound marks an enrollment request, request child, phase or
// student the booking materialization asked for that does not exist in the
// tenant. The composition wraps the owner's own not-found error with it and
// keeps its text.
var ErrBookingRowNotFound = errors.New("booking materialization: row not found")

// RosterEnrollment is one Timetable roster row (activities.student_enrollments)
// as the materialization reads and writes it. ValidUntil is exclusive; an
// empty SelectedWeekdays means every weekday.
type RosterEnrollment struct {
	ID                       int64
	StudentID                int64
	ActivityGroupID          int64
	ValidFrom                calendar.Date
	ValidUntil               *calendar.Date
	CalendarPeriodID         *int64
	EnrollmentRequestChildID *int64
	SelectedWeekdays         []int
	Weekday                  *int
	// StudentAlumnus is only read for a template's rows.
	StudentAlumnus bool
}

// SourcedTemplate is a Timetable template with its offering-source rule.
type SourcedTemplate struct {
	ID                    int64
	Name                  string
	SeriesRootID          *int64
	CalendarPeriodID      *int64
	SourceCareOfferingIDs []int64
	SourceGradeLevels     []int
	SourceSchoolClasses   []string
}

// BookingRosters reads and writes the Timetable rosters booking
// materialization derives, in the caller's tenant transaction.
type BookingRosters interface {
	// GroupEnrollments lists a template's rows in validity order, with each
	// student's alumnus flag.
	GroupEnrollments(ctx context.Context, groupID int64) ([]RosterEnrollment, error)
	// StudentEnrollments lists a student's rows in validity order.
	StudentEnrollments(ctx context.Context, studentID int64) ([]RosterEnrollment, error)
	CreateEnrollment(ctx context.Context, row RosterEnrollment) error
	DeleteEnrollment(ctx context.Context, id int64) error
	SetEnrollmentValidUntil(ctx context.Context, id int64, validUntil calendar.Date) error
	// DeleteRequestChildEnrollments removes the rows materialized from one
	// request child for the student.
	DeleteRequestChildEnrollments(ctx context.Context, studentID, requestChildID int64) error
	// BackfillRequestChildSource stamps the request child on the student's
	// untagged rows of the given groups.
	BackfillRequestChildSource(ctx context.Context, studentID, requestChildID int64, groupIDs []int64) error
	// ReconcileTemplateRosters aligns a template's already-materialized
	// future occurrences with its current rows for the students. prior is the
	// template's rows before the caller's writes; nil re-establishes
	// coverage from scratch.
	ReconcileTemplateRosters(ctx context.Context, templateID int64, studentIDs []int64, from calendar.Date, prior []RosterEnrollment) error
}

// BookingTemplates reads the offering-sourced Timetable templates and
// rewrites a template's source rule.
type BookingTemplates interface {
	TemplatesWithOfferingSource(ctx context.Context) ([]SourcedTemplate, error)
	// TemplatesSourcedFrom lists the live templates sourcing any of the
	// offerings, in id order.
	TemplatesSourcedFrom(ctx context.Context, offeringIDs []int64) ([]SourcedTemplate, error)
	// Groups reads the groups with their split-series root.
	Groups(ctx context.Context, ids []int64) ([]SourcedTemplate, error)
	UpdateTemplateOfferingSource(ctx context.Context, templateID int64, offeringIDs []int64, gradeLevels []int, schoolClasses []string) error
}

// BookingPhase is an enrollment phase's service window and the rule how many
// of its choosable offerings a child selects.
type BookingPhase struct {
	careplan.OfferingPhase
	CareOfferingSelectionMode string
}

// BookingRequest is the enrollment request an adjusted child belongs to.
type BookingRequest struct {
	ID      int64
	PhaseID int64
}

// BookingChild is the request child an adjustment switches.
type BookingChild struct {
	ID               int64
	RequestID        int64
	Approved         bool
	CreatedStudentID *int64
	TargetGradeLevel *int16
}

// ApprovedOfferingChild is one approved booking of an offering with the
// student it materializes for, the school class the Klassen filter matches
// on and the Jahrgang School Structure derives from it (nil when the class
// carries no grade number).
type ApprovedOfferingChild struct {
	Link        careplan.BookedOffering
	StudentID   int64
	SchoolClass string
	GradeLevel  *int16
}

// BookingEnrollment reads the Enrollment requests, children, phases and
// booked selections the materialization derives from.
type BookingEnrollment interface {
	Request(ctx context.Context, id int64) (BookingRequest, error)
	Child(ctx context.Context, id int64) (BookingChild, error)
	Phase(ctx context.Context, id int64) (BookingPhase, error)
	Phases(ctx context.Context) ([]careplan.OfferingPhase, error)
	PhasesByID(ctx context.Context, ids []int64) ([]careplan.OfferingPhase, error)
	// SelectionsAt lists the child's selections in force on the date.
	SelectionsAt(ctx context.Context, childID int64, on calendar.Date) ([]careplan.BookedOffering, error)
	// SelectionHistory lists every dated selection of the child.
	SelectionHistory(ctx context.Context, childID int64) ([]careplan.BookedOffering, error)
	// ApprovedChildren lists the approved bookings of the offerings still in
	// force on or after the date.
	ApprovedChildren(ctx context.Context, offeringIDs []int64, onOrAfter calendar.Date) ([]ApprovedOfferingChild, error)
}

// BookingStudent is the part of a student's record the materialization
// bounds rosters with. GradeLevel is the Jahrgang School Structure derives
// from the school class (nil without a grade number); EnrolledUntil is the
// last care day (inclusive).
type BookingStudent struct {
	ID            int64
	GradeLevel    *int16
	EnrolledUntil *calendar.Date
}

// BookingPeriods lists the tenant's School Calendar planning periods.
type BookingPeriods interface {
	Periods(ctx context.Context) ([]careplan.LinkedPeriod, error)
}

// BookingStudents reads students and takes the People Directory locks in the
// project-wide order.
type BookingStudents interface {
	// Student reads a student, locking the row FOR UPDATE when lock is set.
	Student(ctx context.Context, id int64, lock bool) (BookingStudent, error)
	// LockClassWrites takes the shared class-writes gate.
	LockClassWrites(ctx context.Context) error
	// LockStudents takes the students' care locks in ascending order,
	// skipping students that no longer exist.
	LockStudents(ctx context.Context, studentIDs []int64) error
}

// BookingSettings resolves the tenant settings the materialization obeys.
type BookingSettings interface {
	CareOfferingsEnabled(ctx context.Context) (bool, error)
	BookingsAuthoritative(ctx context.Context) (bool, error)
}

// BookingCommands replaces or schedules a request child's effective bookings.
type BookingCommands interface {
	ReplaceCareOfferingBookings(ctx context.Context, childID int64, bookings []careplan.CareOfferingBooking) error
	ScheduleCareOfferingBookings(ctx context.Context, childID int64, effectiveFrom careplan.Date, bookings []careplan.CareOfferingBooking) error
}

// BookingWithdrawals persists or obsoletes the complete-withdrawal follow-up
// of an authoritative booking change.
type BookingWithdrawals interface {
	ReconcileAuthoritativeBookingChange(ctx context.Context, change careplan.CareWithdrawalBookingChange) error
}

// AdjustmentAudit records an adjustment and names its actor.
type AdjustmentAudit interface {
	// RecordAdjustment writes the audit row and returns its id.
	RecordAdjustment(ctx context.Context, record careplan.OfferingAdjustmentRecord) (int64, error)
	// ActorSnapshot returns the actor's display name and e-mail address, nil
	// when unknown.
	ActorSnapshot(ctx context.Context, accountID int64) (name, email *string)
}

// PickupWeekdayRow is a stored weekly pickup row of a student.
type PickupWeekdayRow struct {
	ID      int64
	Weekday int
}

// BookingPickup reads the date-aware offering pickup projection and changes
// the stored weekly pickup rows it overrides.
type BookingPickup interface {
	OfferingPickupForDate(ctx context.Context, studentID int64, date calendar.Date) (*careplan.PickupSchedule, error)
	WeekdayRows(ctx context.Context, studentID int64) ([]PickupWeekdayRow, error)
	DeleteWeekdayRow(ctx context.Context, id int64) error
}
