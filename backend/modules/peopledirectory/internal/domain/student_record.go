package domain

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// The weekday plans of a child, re-exported so the persistence adapter reads
// and writes them without reaching past this package into the contract.
type (
	DepartureDays         = departure.DepartureDays
	AllowedDepartureModes = departure.AllowedDepartureModes
	PickupDays            = departure.PickupDays
	BusDays               = departure.BusDays
	// DeparturePlan is the plan one write carries; the rules that resolve it
	// belong to the departure package.
	DeparturePlan = departure.Plan
)

// Care-status windows of the staff directory. Running and Ended are the two
// sides of the enrolment interval; All lets the caller manage both.
const (
	StudentCareStatusRunning = "running"
	StudentCareStatusEnded   = "ended"
	StudentCareStatusAll     = "all"
)

// StudentRecord is the whole users.students row as its owner holds it. The
// narrow Student projection stays what other owners read; this is what the
// directory's own reads and writes carry, so the staff surfaces keep every
// field they render without a second path to the table.
//
// Calendar dates are YYYY-MM-DD strings, empty when unset.
type StudentRecord struct {
	ID        int64
	CreatedAt time.Time
	UpdatedAt time.Time
	TenantID  int64

	PersonID    int64
	SchoolClass string
	GroupID     *int64
	Status      string

	EnrolledFrom  string
	EnrolledUntil string

	// The four legacy guardian columns; the guardian tables are authoritative.
	GuardianName    *string
	GuardianContact *string
	GuardianEmail   *string
	GuardianPhone   *string

	AddressStreet     *string
	AddressCity       *string
	AddressPostalCode *string

	ExtraInfo       *string
	SupervisorNotes *string
	HealthInfo      *string
	PickupStatus    *string

	DepartureDays          departure.DepartureDays
	AllowedDepartureModes  departure.AllowedDepartureModes
	PickupDays             departure.PickupDays
	BusDays                departure.BusDays
	DepartureCompanionNote *string

	Sick         *bool
	SickSince    *time.Time
	Excused      *bool
	ExcusedSince *time.Time

	PhotoPath           *string
	PhotoConsentGivenAt *time.Time
	PhotoConsentGivenBy *int64

	AGBAcceptedAt            *time.Time
	DataProcessingAcceptedAt *time.Time
	EmailContactAcceptedAt   *time.Time
}

func (r StudentRecord) IsAlumnus() bool { return r.Status == StudentStatusAlumnus }

// StudentDirectoryFilter narrows the staff directory page. Every field is
// optional; an empty filter is the tenant's whole non-alumni roster.
//
// It is a typed filter, not a generic query object, because the owner decides
// which of its columns a caller may narrow by — and a contract that carried a
// query builder would let any caller reach every column.
type StudentDirectoryFilter struct {
	// IDs restricts the page to these children. Empty means no restriction;
	// a caller that pre-filtered to nothing must not call at all.
	IDs []int64
	// SchoolClasses matches the class name of any of them, trimmed and
	// case-insensitively — a school may pick several classes at once.
	SchoolClasses []string
	// GradeLevels matches the first run of digits in the free-text class
	// name, so "3a" and "Klasse 3a" both count as grade 3 and "13a" does not.
	GradeLevels []string
	// GuardianNameContains is a case-insensitive substring match on the
	// retained guardian_name column.
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
	// Page is 1-based. A PageSize of 0 returns the whole selection, which is
	// what the exports need.
	Page     int
	PageSize int
}
