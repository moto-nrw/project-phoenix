package ports

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// OfferingChangeRows reads and writes enrollment.offering_change_requests in
// the caller's tenant transaction. FindForUpdate reports a missing row as
// careplan.ErrOfferingChangeNotFound; the guarded writes report a decided row
// as careplan.ErrOfferingChangeNotPending. Every other failure keeps the
// shape the staff and parent routes render.
type OfferingChangeRows interface {
	Create(ctx context.Context, row careplan.OfferingChangeRequest) (careplan.OfferingChangeRequest, error)
	Find(ctx context.Context, id int64) (careplan.OfferingChangeRequest, error)
	FindForUpdate(ctx context.Context, id int64) (careplan.OfferingChangeRequest, error)
	// PendingForStudent returns the child's open request, nil when none.
	PendingForStudent(ctx context.Context, studentID int64) (*careplan.OfferingChangeRequest, error)
	// ListByStudent returns the child's requests, newest decision first.
	ListByStudent(ctx context.Context, studentID int64) ([]careplan.OfferingChangeRequest, error)
	// ListPending returns the tenant's open requests, oldest submission first.
	ListPending(ctx context.Context) ([]careplan.OfferingChangeRequest, error)
	UpdateEffectiveFrom(ctx context.Context, id int64, effectiveFrom calendar.Date) error
	UpdateApprovedCompleteWithdrawal(ctx context.Context, id int64, complete bool) error
	UpdatePending(ctx context.Context, id int64, payload json.RawMessage, effectiveFrom calendar.Date, note *string) error
	Decide(ctx context.Context, id int64, status string, reason *string, reviewedBy *int64, applied bool) error
	UpdateDecisionSnapshot(ctx context.Context, id int64, snapshot json.RawMessage) error
}

// OfferingCarePeriod is one approved enrollment of a student with the care
// window of its phase.
type OfferingCarePeriod struct {
	RequestChildID    int64
	RequestID         int64
	PhaseID           int64
	PhaseName         string
	ServiceStart      calendar.Date
	ServiceEnd        calendar.Date
	TargetGradeLevel  *int16
	TargetSchoolClass *string
}

// OfferingChangeChild is the request child an offering change books.
type OfferingChangeChild struct {
	ID                int64
	RequestID         int64
	TargetGradeLevel  *int16
	TargetSchoolClass *string
}

// OfferingCatalogState is a child's booking at a date and the capacity
// peaks of the phase's offerings in the rest of the service window.
type OfferingCatalogState struct {
	Current       []careplan.BookedOffering
	CapacityPeaks map[int64]int
}

// OfferingChangeEnrollment reads the Enrollment requests, children, phases
// and booked selections an offering change validates against. A missing
// child, request or phase is (nil, nil).
type OfferingChangeEnrollment interface {
	StudentCarePeriods(ctx context.Context, studentID int64) ([]OfferingCarePeriod, error)
	Child(ctx context.Context, id int64) (*OfferingChangeChild, error)
	Children(ctx context.Context, ids []int64) ([]OfferingChangeChild, error)
	Request(ctx context.Context, id int64) (*BookingRequest, error)
	Phase(ctx context.Context, id int64) (*BookingPhase, error)
	// SelectionsAt lists the child's selections in force on the date.
	SelectionsAt(ctx context.Context, childID int64, on calendar.Date) ([]careplan.BookedOffering, error)
	// EffectiveSelectionsAt lists the selections in force on each child's
	// date.
	EffectiveSelectionsAt(ctx context.Context, dates map[int64]calendar.Date) ([]careplan.BookedOffering, error)
	CatalogState(ctx context.Context, phaseID, childID int64, on, until calendar.Date) (OfferingCatalogState, error)
	// CapacityPeak counts the peak occupancy of an offering in [from, until)
	// without the excluded children.
	CapacityPeak(ctx context.Context, offeringID int64, excludeChildIDs []int64, from, until calendar.Date) (int, error)
}

// OfferingChangeStudents reads the People Directory students an offering
// change authorizes against.
type OfferingChangeStudents interface {
	FindStudent(ctx context.Context, id int64) (*ReviewStudent, error)
	// LockStudent takes the student row FOR UPDATE.
	LockStudent(ctx context.Context, id int64) (*ReviewStudent, error)
	// HasPendingWithdrawalCompletion reports an open complete-withdrawal
	// follow-up of the student.
	HasPendingWithdrawalCompletion(ctx context.Context, studentID int64) (bool, error)
}

// OfferingChangeSettings resolves the tenant settings offering changes obey.
type OfferingChangeSettings interface {
	BookingSettings
	OfferingChangesEnabled(ctx context.Context) (bool, error)
	CourseRequestsEnabled(ctx context.Context) (bool, error)
	// OfferingChangeLeadDays returns the configured notice period as stored.
	OfferingChangeLeadDays(ctx context.Context) (string, error)
}

// OfferingChangeCatalog lists care offerings of Care Plan's catalog.
type OfferingChangeCatalog interface {
	ListCareOfferings(ctx context.Context, filter careplan.CareOfferingFilter) ([]careplan.CareOffering, error)
}

// OfferingChangeBookings applies an approved or direct booking switch
// through the booking materialization (#3560).
type OfferingChangeBookings interface {
	careplan.OfferingAdjustments
	LockOfferingDerivedWrites(ctx context.Context) error
}

// ManualPlanningOccurrence is a planned slot without a care-offering source.
// Date is a calendar date in YYYY-MM-DD form.
type ManualPlanningOccurrence struct {
	ActivityGroupID   int64
	ActivityGroupName string
	InstanceID        int64
	Date              string
}

// CourseOfferingReference names an offering and its optional legacy course
// group.
type CourseOfferingReference struct {
	OfferingID      int64
	ActivityGroupID *int64
}

// CourseGroup is the Timetable projection of an AG a course request books.
type CourseGroup struct {
	ID                  int64
	Active              bool
	ParticipantLimit    *int
	ScheduledWeekdays   []int
	SourceGradeLevels   []int
	SourceSchoolClasses []string
}

// OfferingChangePlanning reads the Timetable facts offering changes and
// course requests need: manual planning a switch leaves uncovered, the AGs
// an offering feeds, their locks and their rosters.
type OfferingChangePlanning interface {
	ListManualPlanningOccurrences(ctx context.Context, studentID int64, from, to string) ([]ManualPlanningOccurrence, error)
	CourseGroupsForOfferings(ctx context.Context, offerings []CourseOfferingReference, effectiveOn calendar.Date) (map[int64][]CourseGroup, error)
	LockCourseGroups(ctx context.Context, groupIDs []int64) ([]CourseGroup, error)
	CountActiveCourseEnrollments(ctx context.Context, groupIDs []int64, from, until calendar.Date, excludeStudentID int64) (map[int64]int, error)
}
