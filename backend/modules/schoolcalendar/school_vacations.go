package schoolcalendar

import (
	"context"
	"errors"
	"time"
)

// MaxSchoolVacationWindowDays bounds one vacation read: a school year plus
// the weeks a calendar view shows around it, like the holiday list.
const MaxSchoolVacationWindowDays = 400

var (
	// ErrInvalidSchoolVacationWindow rejects a malformed, reversed or too
	// long vacation window.
	ErrInvalidSchoolVacationWindow = errors.New("invalid school vacation window")
	// ErrSchoolVacationNotFound is returned when a school acknowledges a
	// vacation window its federal state does not have.
	ErrSchoolVacationNotFound = errors.New("school vacation not found")
)

// SchoolVacation is one vacation window of a federal state (#5345), as the
// daily sync read it from the OpenHolidays API. StartDate and EndDate use
// DateLayout and are both part of the vacation.
type SchoolVacation struct {
	Name      string `json:"name"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	// Acknowledged reports that the school confirmed it plans care in this
	// window on purpose.
	Acknowledged bool `json:"acknowledged"`
}

// SchoolVacationList is what one school sees: its federal state, the
// vacation windows touching the requested window and when the state's
// vacations were last read. SyncedAt is nil while no sync has stored any
// vacation of the state, so a caller can tell "not read yet" from "none".
type SchoolVacationList struct {
	Region    HolidayRegion
	Vacations []SchoolVacation
	SyncedAt  *time.Time
}

// SchoolVacationAcknowledgement records that the school plans care in one
// vacation window of its federal state on purpose. AccountID names the
// person who confirmed it; nil when no account is known.
type SchoolVacationAcknowledgement struct {
	StartDate string
	EndDate   string
	AccountID *int64
}

// SchoolVacations is the school's view of the vacations of its federal
// state (operations.federal_state) and its confirmations of planned care.
type SchoolVacations interface {
	// TenantSchoolVacations lists the vacation windows of the tenant's
	// federal state touching [from, to], ordered by start date.
	TenantSchoolVacations(ctx context.Context, from, to string) (SchoolVacationList, error)
	// AcknowledgeSchoolVacation records the school's confirmation for one
	// stored vacation window; confirming twice keeps the first record.
	AcknowledgeSchoolVacation(ctx context.Context, acknowledgement SchoolVacationAcknowledgement) error
}

// SchoolVacationSyncResult counts what one sync stored.
type SchoolVacationSyncResult struct {
	Regions   int
	Vacations int
}

// SchoolVacationSync renews the stored vacations of every federal state from
// the external source. It runs for all schools at once, outside any tenant.
// A failed fetch keeps the stored vacations and returns the error.
type SchoolVacationSync interface {
	SyncSchoolVacations(ctx context.Context) (SchoolVacationSyncResult, error)
}

// SchoolVacationSeed stores fixed vacation windows for every federal state
// without the external source. Only the local demo seeder reaches it, so a
// seeded stack shows vacations offline.
type SchoolVacationSeed interface {
	SeedSchoolVacations(ctx context.Context, vacations []SchoolVacation) (int, error)
}

// SchoolVacationOwner is the whole school vacation surface the composition
// hands out: the school's reads and confirmations, the sync and the local
// seed.
type SchoolVacationOwner interface {
	SchoolVacations
	SchoolVacationSync
	SchoolVacationSeed
}
