package peopledirectory

import (
	"context"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
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

	// The four retained guardian columns. The guardian tables are
	// authoritative; these survive for the legacy import and search paths.
	GuardianName    *string `json:"guardian_name,omitempty"`
	GuardianContact *string `json:"guardian_contact,omitempty"`
	GuardianEmail   *string `json:"guardian_email,omitempty"`
	GuardianPhone   *string `json:"guardian_phone,omitempty"`

	AddressStreet     *string `json:"address_street,omitempty"`
	AddressCity       *string `json:"address_city,omitempty"`
	AddressPostalCode *string `json:"address_postal_code,omitempty"`

	ExtraInfo       *string `json:"extra_info,omitempty"`
	SupervisorNotes *string `json:"supervisor_notes,omitempty"`
	HealthInfo      *string `json:"health_info,omitempty"`
	PickupStatus    *string `json:"pickup_status,omitempty"`

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
	// GuardianNameContains is a case-insensitive substring match.
	GuardianNameContains string
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
	// ListStudentRecordsByGuardianContact finds children by one of the two
	// retained guardian columns; the guardian tables stay authoritative.
	ListStudentRecordsByGuardianContact(context.Context, string, string) ([]StudentRecord, error)
	// ListStudentRecordsDueForStatus returns the children a lifecycle tick is
	// due to move, for the named bound of the enrolment interval.
	ListStudentRecordsDueForStatus(context.Context, string, string, string) ([]StudentRecord, error)
	// CountStudentsByGroup counts the non-alumni children of each group.
	CountStudentsByGroup(context.Context, []int64) (map[int64]int, error)
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

func (m *Module) ListStudentDirectory(ctx context.Context, filter StudentDirectoryFilter) ([]StudentRecord, error) {
	filter, err := normalizeStudentDirectoryFilter(filter)
	if err != nil {
		return nil, err
	}
	return m.engine.ListStudentDirectory(ctx, filter)
}

func (m *Module) CountStudentDirectory(ctx context.Context, filter StudentDirectoryFilter) (int, error) {
	filter, err := normalizeStudentDirectoryFilter(filter)
	if err != nil {
		return 0, err
	}
	// The total names the selection, never the page.
	filter.Page, filter.PageSize = 0, 0
	return m.engine.CountStudentDirectory(ctx, filter)
}

func (m *Module) ListStudentDirectoryIDs(ctx context.Context) ([]int64, error) {
	return m.engine.ListStudentDirectoryIDs(ctx)
}

func (m *Module) FindStudentRecordForMutation(ctx context.Context, studentID int64) (StudentRecord, error) {
	if studentID <= 0 {
		return StudentRecord{}, invalidStudent("student ID is required")
	}
	return m.engine.FindStudentRecordForMutation(ctx, studentID)
}

func (m *Module) ListStudentRecordsByID(ctx context.Context, ids []int64) ([]StudentRecord, error) {
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return []StudentRecord{}, nil
	}
	return m.engine.ListStudentRecordsByID(ctx, ids)
}

func (m *Module) LockStudentPhotoFeature(ctx context.Context) error {
	return m.engine.LockStudentPhotoFeature(ctx)
}

func normalizeStudentDirectoryFilter(filter StudentDirectoryFilter) (StudentDirectoryFilter, error) {
	filter.IDs = uniquePositive(filter.IDs)
	filter.KeepAlumni = uniquePositive(filter.KeepAlumni)
	filter.SchoolClasses = uniqueClasses(filter.SchoolClasses)
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

func (m *Module) FindStudentRecord(ctx context.Context, studentID int64) (StudentRecord, error) {
	if studentID <= 0 {
		return StudentRecord{}, invalidStudent("student ID is required")
	}
	return m.engine.FindStudentRecord(ctx, studentID)
}

func (m *Module) ListStudentRecordsByPerson(ctx context.Context, personIDs []int64) ([]StudentRecord, error) {
	personIDs = uniquePositive(personIDs)
	if len(personIDs) == 0 {
		return []StudentRecord{}, nil
	}
	return m.engine.ListStudentRecordsByPerson(ctx, personIDs)
}

func (m *Module) ListStudentRecordsByGroup(
	ctx context.Context,
	groupIDs []int64,
	scope StudentScope,
) ([]StudentRecord, error) {
	groupIDs = uniquePositive(groupIDs)
	if len(groupIDs) == 0 {
		return []StudentRecord{}, nil
	}
	return m.engine.ListStudentRecordsByGroup(ctx, groupIDs, scope)
}

func (m *Module) ListStudentRecords(ctx context.Context, scope StudentScope) ([]StudentRecord, error) {
	return m.engine.ListStudentRecords(ctx, scope)
}

func (m *Module) ListStudentRecordsByClass(
	ctx context.Context,
	classes []string,
	scope StudentScope,
) ([]StudentRecord, error) {
	classes = uniqueClasses(classes)
	if len(classes) == 0 {
		return []StudentRecord{}, nil
	}
	return m.engine.ListStudentRecordsByClass(ctx, classes, scope)
}

func (m *Module) ListStudentRecordsByGuardianContact(ctx context.Context, email, phone string) ([]StudentRecord, error) {
	email, phone = strings.TrimSpace(email), strings.TrimSpace(phone)
	if email == "" && phone == "" {
		return []StudentRecord{}, nil
	}
	return m.engine.ListStudentRecordsByGuardianContact(ctx, email, phone)
}

func (m *Module) ListStudentRecordsDueForStatus(ctx context.Context, status, bound, asOf string) ([]StudentRecord, error) {
	if status == "" {
		return nil, invalidStudent("status is required")
	}
	switch bound {
	case StudentBoundCareStart, StudentBoundCareEnd:
	default:
		return nil, invalidStudent("enrolment bound is invalid")
	}
	if _, err := time.Parse(BirthdayLayout, asOf); err != nil {
		return nil, invalidStudent("the day must be a calendar date in YYYY-MM-DD format")
	}
	return m.engine.ListStudentRecordsDueForStatus(ctx, status, bound, asOf)
}

func (m *Module) LockStudentRecordsByID(ctx context.Context, ids []int64) ([]StudentRecord, error) {
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return []StudentRecord{}, nil
	}
	return m.engine.LockStudentRecordsByID(ctx, ids)
}

func (m *Module) CountStudentsByGroup(ctx context.Context, groupIDs []int64) (map[int64]int, error) {
	groupIDs = uniquePositive(groupIDs)
	if len(groupIDs) == 0 {
		return map[int64]int{}, nil
	}
	return m.engine.CountStudentsByGroup(ctx, groupIDs)
}

func (m *Module) ListEnrolledStudentIDsByNameAndBirthday(
	ctx context.Context,
	tenantID int64,
	firstName, lastName, birthday string,
) ([]int64, error) {
	if tenantID <= 0 {
		return nil, invalidStudent("tenant is required")
	}
	if strings.TrimSpace(firstName) == "" || strings.TrimSpace(lastName) == "" {
		return nil, invalidStudent("both names are required")
	}
	// A child without a recorded birthday cannot be matched this way, and a
	// blank one must not match every child of the name.
	if _, err := time.Parse(BirthdayLayout, birthday); err != nil {
		return nil, invalidStudent("birthday must be a calendar date in YYYY-MM-DD format")
	}
	return m.engine.ListEnrolledStudentIDsByNameAndBirthday(ctx, tenantID, firstName, lastName, birthday)
}

func (m *Module) ListAllStudentIDs(ctx context.Context) ([]int64, error) {
	return m.engine.ListAllStudentIDs(ctx)
}

func (m *Module) FindStudentRecordForMutationNoWait(ctx context.Context, studentID int64) (StudentRecord, error) {
	if studentID <= 0 {
		return StudentRecord{}, invalidStudent("student ID is required")
	}
	return m.engine.FindStudentRecordForMutationNoWait(ctx, studentID)
}
