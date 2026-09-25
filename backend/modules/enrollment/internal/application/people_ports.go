package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// People Directory records the decision flow reads and writes. The root
// binds them to the directory's rows; every field of a row travels, so an
// update writes back exactly what was read plus the decision's changes.

// Student is a People Directory student as the decision flow renews and
// enriches it.
type Student struct {
	ID                int64
	TenantID          int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	PersonID          int64
	SchoolClass       string
	GroupID           *int64
	AddressStreet     *string
	AddressCity       *string
	AddressPostalCode *string
	ExtraInfo         *string
	SupervisorNotes   *string
	HealthInfo        *string
	PickupStatus      *string
	// DepartureDays, AllowedDepartureModes, BusDays and PickupDays are the
	// child's departure plan; AllowedDepartureModes is the source of truth
	// and the others its derived views (#1610).
	DepartureDays          departure.DepartureDays
	AllowedDepartureModes  departure.AllowedDepartureModes
	DepartureCompanionNote *string
	// DepartureCompanionDays is not persisted: the weekdays a "läuft mit"
	// link covers, which satisfy the note requirement of the plan.
	DepartureCompanionDays map[string]bool
	// DepartureBaseline is not persisted: the plan as it was read, so an
	// update can tell an intentional change from a hydrated copy (#1694).
	DepartureBaseline        *DeparturePlanSnapshot
	PickupDays               departure.PickupDays
	BusDays                  departure.BusDays
	Sick                     *bool
	SickSince                *time.Time
	Excused                  *bool
	ExcusedSince             *time.Time
	Status                   string
	EnrolledFrom             *calendar.Date
	EnrolledUntil            *calendar.Date
	PhotoPath                *string
	PhotoConsentGivenAt      *time.Time
	PhotoConsentGivenBy      *int64
	AGBAcceptedAt            *time.Time
	DataProcessingAcceptedAt *time.Time
	EmailContactAcceptedAt   *time.Time
}

// Lifecycle statuses of a student the decision flow writes.
const (
	studentStatusActive  = "active"
	studentStatusPending = "pending"
)

// DeparturePlanSnapshot is a normalized copy of the four departure-plan
// fields as they were read. It is pure data: it records what was loaded, it
// decides nothing.
type DeparturePlanSnapshot struct {
	DepartureDays         departure.DepartureDays
	AllowedDepartureModes departure.AllowedDepartureModes
	BusDays               departure.BusDays
	PickupDays            departure.PickupDays
}

// snapshotDeparturePlan records the student's current departure plan as the
// hydrated baseline. Every value is normalized, which also deep-copies the
// maps, so a caller mutating a plan map in place cannot move the baseline
// with it.
func (s *Student) snapshotDeparturePlan() {
	s.DepartureBaseline = &DeparturePlanSnapshot{
		DepartureDays:         s.DepartureDays.Normalize(),
		AllowedDepartureModes: s.AllowedDepartureModes.Normalize(),
		BusDays:               s.BusDays.Normalize(),
		PickupDays:            s.PickupDays.Normalize(),
	}
}

// markDepartureCompanionDays records weekdays whose "mit wem" a structured
// companion link answers. Purely additive.
func (s *Student) markDepartureCompanionDays(days ...string) {
	if len(days) == 0 {
		return
	}
	if s.DepartureCompanionDays == nil {
		s.DepartureCompanionDays = make(map[string]bool, len(days))
	}
	for _, day := range days {
		s.DepartureCompanionDays[day] = true
	}
}

// Person is the People Directory person behind a child.
type Person struct {
	ID        int64
	TenantID  int64
	CreatedAt time.Time
	UpdatedAt time.Time
	FirstName string
	LastName  string
	Birthday  *calendar.Date
	TagID     *string
	AccountID *int64
}

// GuardianProfile is a tenant's guardian profile.
type GuardianProfile struct {
	ID                     int64
	TenantID               int64
	CreatedAt              time.Time
	UpdatedAt              time.Time
	FirstName              string
	LastName               string
	Email                  *string
	AddressStreet          *string
	AddressCity            *string
	AddressPostalCode      *string
	AccountID              *int64
	HasAccount             bool
	PreferredContactMethod string
	LanguagePreference     string
	PortalLocale           *string
	Notes                  *string
}

// StudentGuardian is the relationship between a student and a guardian
// profile, with its guardian role and parent-portal permissions.
type StudentGuardian struct {
	ID                 int64
	TenantID           int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	StudentID          int64
	GuardianProfileID  int64
	RelationshipType   string
	GuardianRole       string
	IsPrimary          bool
	IsEmergencyContact bool
	CanPickup          bool
	PickupNotes        *string
	EmergencyPriority  int
	IsPayer            bool
	Permissions        map[string]any
}

// GuardianPhone is one phone number of a guardian profile.
type GuardianPhone struct {
	ID                int64
	TenantID          int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	GuardianProfileID int64
	PhoneNumber       string
	PhoneType         string
	Label             *string
	IsPrimary         bool
	Priority          int
}

// phoneTypeMobile is the phone type of a number the parent entered in the
// form.
const phoneTypeMobile = "mobile"

// CompanionEdge is one undirected "walks home with" edge between two
// children on one ISO weekday (1..5).
type CompanionEdge struct {
	ID            int64
	StudentLowID  int64
	StudentHighID int64
	Weekday       int
}

// other returns the id at the far end of the edge as seen from studentID,
// and whether studentID is part of the edge at all.
func (e *CompanionEdge) other(studentID int64) (int64, bool) {
	switch studentID {
	case e.StudentLowID:
		return e.StudentHighID, true
	case e.StudentHighID:
		return e.StudentLowID, true
	default:
		return 0, false
	}
}

// GuardianRole is the role preset an approval gives a student-guardian
// relationship. The People Directory rules map it to the stored role and its
// parent-portal permissions.
type GuardianRole int

const (
	GuardianRolePrimary GuardianRole = iota + 1
	GuardianRoleEmergency
	GuardianRolePickupOnly
	GuardianRoleCustom
)

// PeopleRules are People Directory's validation and role rules. The
// validators normalize the record they check (trimmed names, lower-cased
// email) the way a directory write would.
type PeopleRules interface {
	ValidatePerson(*Person) error
	ValidateStudent(*Student) error
	ValidateGuardianProfile(*GuardianProfile) error
	ValidateStudentGuardian(*StudentGuardian) error
	// ApplyGuardianRole sets the stored role and its parent-portal
	// permissions.
	ApplyGuardianRole(*StudentGuardian, GuardianRole)
	// IsFullGuardianRole reports whether a stored role is a full guardian
	// preset (primary, legal or co-guardian).
	IsFullGuardianRole(role string) bool
	// MapRelationshipType maps a submitted German relationship label onto the
	// stored relationship type; unknown values land on "other".
	MapRelationshipType(label string) string
}

// Persons creates and renames the persons behind the children and resolves a
// reviewer's person.
type Persons interface {
	CreatePerson(context.Context, *Person) error
	PersonByID(context.Context, int64) (*Person, error)
	UpdatePerson(context.Context, *Person) error
	PersonByAccount(context.Context, int64) (*Person, error)
}

// ReviewerStaff resolves the staff member a reviewer's person is. A person
// without staff answers the runtime's not-found error.
type ReviewerStaff interface {
	StaffIDByPerson(context.Context, int64) (int64, error)
}

// StudentRecords confirms a student exists. A missing student answers the
// runtime's not-found error.
type StudentRecords interface {
	FindStudent(context.Context, int64) error
}

// GuardianProfiles reads and writes the tenant's guardian profiles.
// GuardianProfileByAccount answers nil without error when the account has no
// profile at this school; GuardianProfileByEmail when the address has none.
type GuardianProfiles interface {
	GuardianProfileByAccount(context.Context, int64) (*GuardianProfile, error)
	GuardianProfileByEmail(context.Context, string) (*GuardianProfile, error)
	GuardianProfilesByEmails(context.Context, []string) ([]*GuardianProfile, error)
	GuardianProfilesByID(context.Context, []int64) (map[int64]*GuardianProfile, error)
	CreateGuardianProfile(context.Context, *GuardianProfile) error
	UpdateGuardianProfile(context.Context, *GuardianProfile) error
	LinkGuardianAccount(ctx context.Context, profileID, accountID int64) error
}

// StudentGuardians reads and writes the student-guardian relationships.
// GuardianLinkForUpdate answers ErrGuardianLinkNotFound for a missing link.
type StudentGuardians interface {
	GuardianLinksOfStudent(context.Context, int64) ([]*StudentGuardian, error)
	CreateGuardianLink(context.Context, *StudentGuardian) error
	UpdateGuardianLink(context.Context, *StudentGuardian) error
	DeleteGuardianLink(context.Context, int64) error
	GuardianLinkForUpdate(ctx context.Context, studentID, guardianProfileID int64) (*StudentGuardian, error)
	// LinkGuardianIfMissing inserts the link unless one exists and reports
	// whether it inserted.
	LinkGuardianIfMissing(context.Context, *StudentGuardian) (bool, error)
	// UpdateGuardianLinkRole writes the relationship type, the emergency and
	// pickup flags, the emergency priority, the role and the permissions of
	// the link and reports how many rows it updated.
	UpdateGuardianLinkRole(context.Context, *StudentGuardian) (int64, error)
}

// GuardianPhones reads and adds guardian phone numbers.
type GuardianPhones interface {
	GuardianPhones(context.Context, int64) ([]*GuardianPhone, error)
	GuardianPhonesByGuardian(context.Context, []int64) (map[int64][]*GuardianPhone, error)
	CreateGuardianPhone(context.Context, *GuardianPhone) error
}

// PeopleDirectory bundles the People Directory ports. A nil port skips the
// work that needs it, the way the decision flow always treated an unwired
// repository.
type PeopleDirectory struct {
	Rules            PeopleRules
	Persons          Persons
	Staff            ReviewerStaff
	Students         StudentRecords
	GuardianProfiles GuardianProfiles
	StudentGuardians StudentGuardians
	GuardianPhones   GuardianPhones
	// ErrGuardianLinkNotFound is the directory's missing-link error.
	ErrGuardianLinkNotFound error
	// ErrCompanionWouldLoseDeparture and ErrCompanionLockBusy are the
	// directory's companion refusals; the handlers answer them with an
	// actionable 400/409.
	ErrCompanionWouldLoseDeparture error
	ErrCompanionLockBusy           error
	// IsLockNotAvailable reports a NOWAIT row lock that lost the race.
	IsLockNotAvailable func(error) bool
}

// DepartureCompanions reads the "läuft mit" edges Care Plan keeps and
// deletes those a narrowed plan no longer allows.
type DepartureCompanions interface {
	CompanionsOfStudent(context.Context, int64) ([]*CompanionEdge, error)
	CompanionDaysCoveredExcluding(context.Context, []int64, int64) (map[int64]map[string]bool, error)
	DeleteCompanionEdges(context.Context, []int64) error
}

// WeeklyPickupSchedules replaces a student's weekly pickup times.
type WeeklyPickupSchedules interface {
	DeletePickupSchedules(ctx context.Context, studentID int64) error
	UpsertPickupSchedule(ctx context.Context, studentID int64, weekday int, pickupTime time.Time, createdBy int64) error
}

// WeeklyArrivalSchedules replaces a student's weekly arrival days.
type WeeklyArrivalSchedules interface {
	DeleteArrivalSchedules(ctx context.Context, studentID int64) error
	CreateArrivalSchedule(ctx context.Context, studentID int64, weekday int, createdBy int64) error
}
