package peopledirectory

import (
	"context"
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
