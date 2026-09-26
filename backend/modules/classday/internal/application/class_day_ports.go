package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// DayRoster is Enrollment's roster of one class on one calendar day: the
// covering phases, the roster rows and the loaded students with the
// departure modes their live plan allows that weekday.
type DayRoster struct {
	PhaseNames []string
	Rows       []DayRosterRow
	Students   []DayRosterStudent
}

// DayRosterRow is the part of a roster row the day view projects.
type DayRosterRow struct {
	StudentID      int64
	FirstName      string
	LastName       string
	ListEntry      bool
	ListEntryID    int64
	GroupName      string
	Registered     bool
	OfferingsByDay map[string][]string
	ArrivalByDay   map[string]string
	PickupByDay    map[string]string
}

// DayRosterStudent is one loaded student of the class with the departure
// modes its live plan allows on the day's weekday.
type DayRosterStudent struct {
	ID             int64
	DepartureModes []peopledirectory.DepartureMode
}

// StatusDayEntry is one active scheduled day status (sick / excused / class
// trip) of a student.
type StatusDayEntry struct {
	StudentID  int64
	Status     string
	ReportedAt time.Time
}

// EffectivePickup is one child's effective pickup of a day. PickupTime is
// nil for a timeless day exception ("kommt heute nicht"); RegularPickupTime
// is the recurring plan's time, set alongside a day exception.
type EffectivePickup struct {
	PickupTime        *time.Time
	RegularPickupTime *time.Time
	IsException       bool
	// ChangedAt is when the day exception was entered.
	ChangedAt *time.Time
}

// EffectiveArrival is one child's effective arrival of a day. ArrivalTime is
// nil for a timeless day exception.
type EffectiveArrival struct {
	ArrivalTime *time.Time
	IsException bool
	// ChangedAt is when the day exception was entered.
	ChangedAt *time.Time
}

// ClassArrivalException is one class-wide arrival day exception.
type ClassArrivalException struct {
	SchoolClass string
	Date        timezone.Date
	// ArrivalTime is a wall-clock value; only hour and minute are used.
	ArrivalTime time.Time
	Reason      *string
	CreatedAt   time.Time
	// Origin is "ogs" or "school", empty for rows older than the column.
	Origin string
}

// ClassArrivalExceptionWrite is one class-wide arrival day exception to
// store, attributed to a users.staff row.
type ClassArrivalExceptionWrite struct {
	SchoolClass string
	Date        timezone.Date
	ArrivalTime time.Time
	Reason      *string
	Origin      string
	CreatedBy   int64
}

// SheetStudent is the part of a student the supervision sheet reads.
type SheetStudent struct {
	ID                    int64
	PersonID              int64
	SchoolClass           string
	AllowedDepartureModes peopledirectory.AllowedDepartureModes
	DepartureDays         peopledirectory.DepartureDays
}

// SheetPerson is a person's name.
type SheetPerson struct {
	FirstName string
	LastName  string
}

// EmergencyContactRow is one (guardian, phone number) row of a child's
// guardians in the order of their priority.
type EmergencyContactRow struct {
	StudentID          int64
	GuardianProfileID  int64
	FirstName          string
	LastName           string
	RelationshipType   string
	PickupNotes        string
	PhoneNumber        string
	CanPickup          bool
	IsEmergencyContact bool
}

// AccessRecord is one GDPR access-log row of the school portal.
type AccessRecord struct {
	ActorAccountID int64
	ActorRole      string
	RangeStart     time.Time
	RangeEnd       time.Time
	AccessedAt     time.Time
	StudentID      *int64
	Metadata       map[string]any
}

// Reads and writes of the owners the school-portal capability combines. The
// composition root binds Enrollment's day roster, Care Plan, People
// Directory, the tenant settings, the timetable and the audit log to them.
type (
	// DayRosters reads Enrollment's class roster of a day.
	DayRosters interface {
		ClassRosterDay(ctx context.Context, schoolClass string, date timezone.Date) (*DayRoster, error)
	}
	// StatusDays reads the active scheduled day statuses of many students.
	StatusDays interface {
		ActiveStatusDays(ctx context.Context, studentIDs []int64, date timezone.Date) ([]StatusDayEntry, error)
	}
	// EffectiveDayTimes reads the effective pickup and arrival times (weekly
	// plan plus day exceptions) of many students. A child without an entry
	// has no plan that day.
	EffectiveDayTimes interface {
		EffectivePickups(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]EffectivePickup, error)
		EffectiveArrivals(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]EffectiveArrival, error)
	}
	// ClassArrivalExceptions reads and writes the class-wide arrival day
	// exceptions (#2962). Refusals answer errors.Is for the classday
	// sentinels; a deployment without the store answers
	// ports.ErrClassArrivalExceptionsNotConfigured.
	ClassArrivalExceptions interface {
		ListClassArrivalExceptions(ctx context.Context, schoolClass string, from, to timezone.Date) ([]ClassArrivalException, error)
		UpsertClassArrivalException(ctx context.Context, in ClassArrivalExceptionWrite) (ClassArrivalException, error)
		DeleteClassArrivalException(ctx context.Context, schoolClass string, date timezone.Date) error
	}
	// Companions reads the "läuft mit" links of many students.
	Companions interface {
		ListLinksForStudents(ctx context.Context, studentIDs []int64) (map[int64][]peopledirectory.CompanionLink, error)
	}
	// SheetStudents reads the one child a supervision sheet is about.
	SheetStudents interface {
		FindSheetStudent(ctx context.Context, studentID int64) (*SheetStudent, error)
		PersonsByID(ctx context.Context, personIDs []int64) (map[int64]SheetPerson, error)
	}
	// EmergencyContacts reads the guardian contact rows of students.
	EmergencyContacts interface {
		EmergencyContactRows(ctx context.Context, studentIDs []int64) ([]EmergencyContactRow, error)
	}
	// AccessLog appends the school portal's reads to the GDPR access log.
	AccessLog interface {
		// ClassDayViewSeenSince reports whether the actor was served the day
		// view of the class and date since the given instant.
		ClassDayViewSeenSince(ctx context.Context, actorAccountID int64, schoolClass, date string, since time.Time) (bool, error)
		RecordClassDayView(ctx context.Context, entry AccessRecord) error
		RecordSupervisionSheet(ctx context.Context, entry AccessRecord) error
	}
	// ArrivalWriteScope applies operations.school_portal_write_scope.
	ArrivalWriteScope interface {
		SchoolMayWriteClassArrivalExceptions(ctx context.Context) (bool, error)
	}
	// BlockStarts answers the "Unterricht fällt aus" preset.
	BlockStarts interface {
		EarliestPlannedBlockStartForClass(ctx context.Context, schoolClass string, date timezone.Date) (string, error)
	}
	// ArrivalScheduleAnnouncer tells the OGS live views, once the caller's
	// transaction committed, that a class-wide arrival exception changed.
	ArrivalScheduleAnnouncer interface {
		AnnounceArrivalScheduleChange(ctx context.Context)
	}
)
