package peopledirectory

import (
	"context"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Care-status windows of the staff directory. Running and Ended are the two
// sides of the enrolment interval; All lets the caller manage both.
const (
	StudentCareStatusRunning = "running"
	StudentCareStatusEnded   = "ended"
	StudentCareStatusAll     = "all"
)

// StudentScope says whether a lookup counts graduates. A roster read does not:
// a graduate's row survives only so a grade transition can be reverted. An
// administrative or statistical read may, and says so.
type StudentScope string

const (
	StudentScopeEnrolled StudentScope = "enrolled"
	StudentScopeAll      StudentScope = "all"
)

// StudentRecord is the whole child row as its owner holds it. Student stays
// the narrow projection other owners read; this is what the directory's own
// staff surfaces carry, so they keep every field they render without a second
// path to the table.
//
// Calendar dates are in BirthdayLayout, empty when unset.
type StudentRecord struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	TenantID  int64     `json:"tenant_id"`

	PersonID    int64  `json:"person_id"`
	SchoolClass string `json:"school_class"`
	GroupID     *int64 `json:"group_id,omitempty"`
	Status      string `json:"status"`

	EnrolledFrom  string `json:"enrolled_from,omitempty"`
	EnrolledUntil string `json:"enrolled_until,omitempty"`

	AddressStreet     *string `json:"address_street,omitempty"`
	AddressCity       *string `json:"address_city,omitempty"`
	AddressPostalCode *string `json:"address_postal_code,omitempty"`

	ExtraInfo    *string `json:"extra_info,omitempty"`
	HealthInfo   *string `json:"health_info,omitempty"`
	PickupStatus *string `json:"pickup_status,omitempty"`

	DepartureDays          departure.DepartureDays         `json:"departure_days,omitempty"`
	AllowedDepartureModes  departure.AllowedDepartureModes `json:"allowed_departure_modes,omitempty"`
	PickupDays             departure.PickupDays            `json:"pickup_days,omitempty"`
	BusDays                departure.BusDays               `json:"bus_days,omitempty"`
	DepartureCompanionNote *string                         `json:"departure_companion_note,omitempty"`

	Sick         *bool      `json:"sick,omitempty"`
	SickSince    *time.Time `json:"sick_since,omitempty"`
	Excused      *bool      `json:"excused,omitempty"`
	ExcusedSince *time.Time `json:"excused_since,omitempty"`

	PhotoPath           *string    `json:"-"`
	PhotoConsentGivenAt *time.Time `json:"photo_consent_given_at,omitempty"`
	PhotoConsentGivenBy *int64     `json:"photo_consent_given_by,omitempty"`

	AGBAcceptedAt            *time.Time `json:"agb_accepted_at,omitempty"`
	DataProcessingAcceptedAt *time.Time `json:"data_processing_accepted_at,omitempty"`
	EmailContactAcceptedAt   *time.Time `json:"email_contact_accepted_at,omitempty"`
}

func (r StudentRecord) IsAlumnus() bool { return r.Status == StudentStatusAlumnus }

// CareEndedOn reports whether the child's care has ended by day: a last care
// day is set and day lies after it.
func (r StudentRecord) CareEndedOn(day calendar.Date) bool {
	return r.EnrolledUntil != "" && day.After(calendar.Date(r.EnrolledUntil))
}

// PickupEligibleOn reports a child a pickup plan applies to on day: not
// graduated and still in care.
func (r StudentRecord) PickupEligibleOn(day calendar.Date) bool {
	return !r.IsAlumnus() && !r.CareEndedOn(day)
}

// StudentClassRecords is the class read HasActiveClass runs over.
type StudentClassRecords interface {
	ListStudentRecordsByClass(context.Context, []string, StudentScope) ([]StudentRecord, error)
}

// HasActiveClass reports whether class still has an enrolled child whose care
// has not ended by today.
func HasActiveClass(ctx context.Context, students StudentClassRecords, class string, today calendar.Date) (bool, error) {
	rows, err := students.ListStudentRecordsByClass(ctx, []string{class}, StudentScopeEnrolled)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if !row.CareEndedOn(today) {
			return true, nil
		}
	}
	return false, nil
}

// StudentRosterEntry is a child together with the identity it renders under.
// Every roster surface needs both, and both are this owner's rows.
type StudentRosterEntry struct {
	Record    *StudentState `json:"student"`
	GroupName string        `json:"group_name"`
	FirstName string        `json:"first_name"`
	LastName  string        `json:"last_name"`
	TagID     *string       `json:"tag_id,omitempty"`
	AccountID *int64        `json:"account_id,omitempty"`
}

// StudentDirectoryFilter narrows the staff directory page. Every field is
// optional; an empty filter is the tenant's whole non-alumni roster.
//
// It is a typed filter rather than a query object on purpose: the owner
// decides which of its columns a caller may narrow by, and a contract
// carrying a query builder would open every column to every caller.
type StudentDirectoryFilter struct {
	// IDs restricts the page to these children; empty means no restriction.
	IDs []int64
	// SchoolClasses matches any of them, trimmed and case-insensitively.
	SchoolClasses []string
	// GradeLevels matches the first run of digits in the free-text class
	// name, so "3a" and "Klasse 3a" both count as grade 3 and "13a" does not.
	GradeLevels []int
	// KeepAlumni are the graduates the page keeps anyway — a child with a
	// still-open presence stays visible. Every other alumnus is excluded.
	KeepAlumni []int64
	// CareStatus selects the side of the enrolment interval; empty means
	// running.
	CareStatus string
	// CareStatusOn is the caller's frozen calendar day for that boundary, so
	// a request crossing Berlin midnight keeps one notion of "today".
	CareStatusOn string
	// Page is 1-based; a PageSize of 0 returns the whole selection, which is
	// what the exports need.
	Page     int
	PageSize int
}

// StudentDirectoryQuery is the staff-facing directory read: one filtered,
// ordered page and the total of the same selection.
type StudentDirectoryQuery interface {
	StudentDirectorySelection
	StudentRosterQuery
}

// StudentDirectorySelection reads filtered student records and directory counts.
type StudentDirectorySelection interface {
	ListStudentDirectory(context.Context, StudentDirectoryFilter) ([]StudentRecord, error)
	CountStudentDirectory(context.Context, StudentDirectoryFilter) (int, error)
	// ListStudentDirectoryIDs returns the ids of every non-alumni child.
	ListStudentDirectoryIDs(context.Context) ([]int64, error)
	// ListStudentRecordsByID reads the owned rows of the given children,
	// alumni included, ordered by id.
	ListStudentRecordsByID(context.Context, []int64) ([]StudentRecord, error)
	// ListAllStudentIDs returns every child of the tenant, alumni included:
	// a graduate who is actually present today still has to be reachable.
	ListAllStudentIDs(context.Context) ([]int64, error)
	// FindStudentRecord reads one owned row.
	FindStudentRecord(context.Context, int64) (StudentRecord, error)
	// ListStudentRecordsByPerson resolves the children of the given identities,
	// alumni included.
	ListStudentRecordsByPerson(context.Context, []int64) ([]StudentRecord, error)
	// ListStudentRecordsByGroup returns the children of the groups in scope.
	ListStudentRecordsByGroup(context.Context, []int64, StudentScope) ([]StudentRecord, error)
	// ListStudentRecordsByClass returns the children of the classes in scope.
	ListStudentRecordsByClass(context.Context, []string, StudentScope) ([]StudentRecord, error)
	// ListStudentRecords returns every child of the tenant in scope, ordered
	// by id. It backs the whole-school reads: the class roster over every
	// class, and the platform statistics.
	ListStudentRecords(context.Context, StudentScope) ([]StudentRecord, error)
	// ListStudentRecordsDueForStatus returns the children a lifecycle tick is
	// due to move, for the named bound of the enrolment interval.
	ListStudentRecordsDueForStatus(context.Context, string, string, string) ([]StudentRecord, error)
	// CountStudentsByGroup counts the non-alumni children of each group.
	CountStudentsByGroup(context.Context, []int64) (map[int64]int, error)
}

// StudentRosterQuery reads enrollment-aware student rosters.
type StudentRosterQuery interface {
	// ListStudentRoster is the school's live roster on the given day: the
	// children it currently cares for, with the identity each renders under.
	ListStudentRoster(context.Context, string) ([]StudentRosterEntry, error)
	// ListStudentRosterByGroup narrows the same roster to the given groups.
	ListStudentRosterByGroup(context.Context, []int64, string) ([]StudentRosterEntry, error)
	// ListStudentRosterOverlapping returns the children enrolled on at least
	// one day of a window. Unlike the live roster it keeps children whose care
	// has since ended and omits later enrolments.
	ListStudentRosterOverlapping(context.Context, string, string, string) ([]StudentRosterEntry, error)
	// ListEnrolledStudentIDsByNameAndBirthday resolves already-enrolled
	// children by name and birthday. The tenant is explicit because the parent
	// submit path runs outside a tenant transaction.
	ListEnrolledStudentIDsByNameAndBirthday(context.Context, int64, string, string, string) ([]int64, error)
}

// StudentDirectoryCommand holds the owned rows a caller is about to write.
type StudentDirectoryCommand interface {
	// FindStudentRecordForMutation reads one owned row and holds its lock for
	// the caller's transaction, so a status the caller validates cannot be
	// changed by a concurrent grade transition before its own write commits.
	FindStudentRecordForMutation(context.Context, int64) (StudentRecord, error)
	// LockStudentPhotoFeature takes the per-tenant photo gate, which
	// serializes photo writes against a feature-disable purge. It belongs to
	// the caller that also takes a student row lock, and it is taken first.
	LockStudentPhotoFeature(context.Context) error
	// LockStudentRecordsByID reads and locks the given rows in ascending id
	// order, the project-wide student lock order.
	LockStudentRecordsByID(context.Context, []int64) ([]StudentRecord, error)
	// FindStudentRecordForMutationNoWait is FindStudentRecordForMutation that
	// never blocks: when another transaction holds the row it reports
	// ErrStudentLockBusy instead of waiting.
	//
	// It exists for the one case where waiting is unsafe — taking a lock on an
	// id BELOW one this transaction already holds. Every companion writer
	// acquires rows in ascending id order, so a downward acquisition inverts
	// that order and can deadlock against a writer coming the other way. The
	// companion graph is read while locks are already held, so such
	// acquisitions cannot be designed away; they are made non-blocking instead.
	FindStudentRecordForMutationNoWait(context.Context, int64) (StudentRecord, error)
}

// NormalizeStudentDirectoryFilter validates and normalizes the directory selection.
func NormalizeStudentDirectoryFilter(filter StudentDirectoryFilter) (StudentDirectoryFilter, error) {
	filter.IDs = PositiveIDs(filter.IDs)
	filter.KeepAlumni = PositiveIDs(filter.KeepAlumni)
	filter.SchoolClasses = SchoolClasses(filter.SchoolClasses)
	if filter.Page < 0 || filter.PageSize < 0 {
		return filter, invalidStudent("page and page size must not be negative")
	}
	switch filter.CareStatus {
	case "", StudentCareStatusRunning, StudentCareStatusEnded, StudentCareStatusAll:
	default:
		return filter, invalidStudent("care status is invalid")
	}
	// Both bounded sides compare against a calendar day; without one the
	// boundary would silently widen to every child.
	if filter.CareStatus != StudentCareStatusAll && filter.CareStatusOn == "" {
		return filter, invalidStudent("care status requires a calendar day")
	}
	if filter.CareStatusOn != "" {
		if _, err := time.Parse(BirthdayLayout, filter.CareStatusOn); err != nil {
			return filter, invalidStudent("care status day must be a calendar date in YYYY-MM-DD format")
		}
	}
	return filter, nil
}

// validateRosterDay refuses a roster whose day the caller could not name: the
// care boundary is a date comparison, and a missing day would widen it to
// every child rather than fail.
func ValidateRosterDay(day string) error {
	if _, err := time.Parse(BirthdayLayout, day); err != nil {
		return invalidStudent("the roster day must be a calendar date in YYYY-MM-DD format")
	}
	return nil
}

// ValidateStudentMatch validates an exact enrolled-child identity lookup.
func ValidateStudentMatch(tenantID int64, firstName, lastName, birthday string) error {
	if tenantID <= 0 {
		return invalidStudent("tenant is required")
	}
	if strings.TrimSpace(firstName) == "" || strings.TrimSpace(lastName) == "" {
		return invalidStudent("both names are required")
	}
	// A child without a recorded birthday cannot be matched this way, and a
	// blank one must not match every child of the name.
	if _, err := time.Parse(BirthdayLayout, birthday); err != nil {
		return invalidStudent("birthday must be a calendar date in YYYY-MM-DD format")
	}
	return nil
}
