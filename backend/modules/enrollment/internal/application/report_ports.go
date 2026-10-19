package application

import (
	"context"
	"time"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Enrollment owner reads of the reports.
type (
	ReportRequests interface {
		AdminRequests(context.Context, enrollmentNative.RequestListFilters) ([]*enrollmentNative.Request, error)
	}
	ReportChildren interface {
		RequestChildOfferingsForChildrenAtDate(context.Context, []int64, enrollmentNative.Date) ([]*enrollmentNative.RequestChildOffering, error)
		ChildrenForRequests(context.Context, []int64) ([]*enrollmentNative.RequestChild, error)
	}
	ReportGuardians interface {
		RequestGuardians(context.Context, []int64) ([]*enrollmentNative.RequestGuardian, error)
	}
	ReportSchemas interface {
		Schemas(context.Context, []int64) ([]*enrollmentNative.FormSchema, error)
	}
	ReportPhases interface {
		Phase(context.Context, int64) (*enrollmentNative.Phase, error)
		Phases(context.Context) ([]*enrollmentNative.Phase, error)
	}
	// ReportOfferings lists the care offerings of a phase from Care Plan.
	ReportOfferings interface {
		ListByPhase(ctx context.Context, phaseID int64) ([]*offeringContract.CareOfferingRecord, error)
	}
)

// Departure values of People Directory's contract the roster ports carry.
type (
	DepartureModes = departure.AllowedDepartureModes
	DepartureDays  = departure.DepartureDays
	CompanionLink  = departure.CompanionLink
)

// RosterStudent is the part of a student the class roster reads.
type RosterStudent struct {
	ID                     int64
	PersonID               int64
	SchoolClass            string
	GroupID                *int64
	AllowedDepartureModes  DepartureModes
	DepartureDays          DepartureDays
	DepartureCompanionNote *string
}

// RosterPerson is a student's name.
type RosterPerson struct {
	FirstName string
	LastName  string
}

// GuardianContactRow is one (guardian, phone number) row of a student's
// guardian contacts in the order of their priority.
type GuardianContactRow struct {
	StudentID         int64
	GuardianProfileID int64
	FirstName         string
	LastName          string
	Email             string
	PhoneNumber       string
}

// ClassListEntry is one class-list-only child (#2382) as the roster reads
// it: a name and a free-text class, nothing else exists.
type ClassListEntry struct {
	ID          int64
	FirstName   string
	LastName    string
	SchoolClass string
}

// Reads of the other owners the class roster combines. The root binds them to
// People Directory, School Structure, School Membership and Care Plan.
type (
	// RosterStudents lists the students of one class, or every student when
	// schoolClass is empty.
	RosterStudents interface {
		ListClassRoster(ctx context.Context, schoolClass string) ([]*RosterStudent, error)
	}
	// RosterPersons resolves person names by person id.
	RosterPersons interface {
		PersonsByID(ctx context.Context, personIDs []int64) (map[int64]*RosterPerson, error)
	}
	// RosterGroups resolves education group names by group id.
	RosterGroups interface {
		GroupNamesByID(ctx context.Context, groupIDs []int64) (map[int64]string, error)
	}
	// RosterGuardianContacts lists the guardian contact rows of students.
	RosterGuardianContacts interface {
		GuardianContacts(ctx context.Context, studentIDs []int64) ([]GuardianContactRow, error)
	}
	// RosterCompanions lists the "läuft mit" links of students.
	RosterCompanions interface {
		ListLinksForStudents(ctx context.Context, studentIDs []int64) (map[int64][]CompanionLink, error)
	}
	// ClassListEntries hands over the class-list-only entries of one class,
	// or of every class when schoolClass is empty.
	ClassListEntries interface {
		ListClassListEntries(ctx context.Context, schoolClass string) ([]ClassListEntry, error)
	}
	// PickupSchedules reads the recurring pickup plans in force on a date.
	PickupSchedules interface {
		GetWeeklySchedulesByStudentIDsForDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*careplan.PickupSchedule, error)
	}
	// CareParticipation is Care Plan's dated operational participation.
	CareParticipation interface {
		ResolveListParticipation(ctx context.Context, studentIDs []int64, on, today calendar.Date, includePending bool) (*careplan.CareParticipationResolution, error)
	}
	// ReportSettings resolves enrollmentNative.care_offerings_enabled so the class
	// roster matches the form: a leftover active catalog must not constrain
	// pickup times when offerings are turned off.
	ReportSettings interface {
		CareOfferingsEnabled(ctx context.Context) (bool, error)
	}
)

// ExportAccess is one GDPR access-log row of an enrollment export.
type ExportAccess struct {
	ActorAccountID int64
	ActorRole      string
	RangeStart     time.Time
	RangeEnd       time.Time
	AccessedAt     time.Time
	Metadata       map[string]any
}

// ExportAccessLog appends the enrollment export rows to the GDPR access log:
// a phase export, or the export of one student's enrollments.
type ExportAccessLog interface {
	RecordPhaseExport(ctx context.Context, entry ExportAccess) error
	RecordStudentExport(ctx context.Context, studentID int64, entry ExportAccess) error
}

// ReportDependencies bind the reports to their owners. Guardians,
// GuardianContacts, Companions, ClassListEntries, PickupSchedules and
// Settings are optional where the report degrades without them; every other
// nil dependency fails the report that needs it.
type ReportDependencies struct {
	Requests          ReportRequests
	Children          ReportChildren
	Guardians         ReportGuardians
	Schemas           ReportSchemas
	Phases            ReportPhases
	Offerings         ReportOfferings
	AccessLog         ExportAccessLog
	Students          RosterStudents
	Persons           RosterPersons
	Groups            RosterGroups
	GuardianContacts  RosterGuardianContacts
	Companions        RosterCompanions
	ClassListEntries  ClassListEntries
	PickupSchedules   PickupSchedules
	CareParticipation CareParticipation
	Settings          ReportSettings
	Now               func() time.Time
}

// Reports builds Enrollment's phase reports.
type Reports struct {
	deps ReportDependencies
}

var _ enrollmentNative.Reports = (*Reports)(nil)

// NewReports creates the reports.
func NewReports(deps ReportDependencies) *Reports {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &Reports{deps: deps}
}

func (s *Reports) today() calendar.Date {
	if s.deps.Now == nil {
		return calendar.TodayDate()
	}
	return calendar.DateFromTime(s.deps.Now())
}
