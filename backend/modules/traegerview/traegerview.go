// Package traegerview is the public contract of the Träger projection (ADR
// 0084 section 1): the only place that reads data of several schools for the
// Träger-Portal. It reads in an administrative transaction, and every query
// filters on the schools of the Träger of the session, which Organisation &
// Tenancy supplies. A ratchet test checks that filter on every query of the
// projection's Postgres package (backend/test/traeger_view_scope_ratchet_test.go).
// The projection never writes; writes go to the school's owner in the
// school's tenant transaction.
package traegerview

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

var (
	// ErrTraegerRequired reports a call without the Träger of the session.
	ErrTraegerRequired = errors.New("traeger view: the Träger of the session is required")
	// ErrInvalidAccessLogFilter reports an access log filter outside its
	// bounds: a missing or reversed window, a window over a year, an unknown
	// kind or a negative page.
	ErrInvalidAccessLogFilter = errors.New("traeger view: invalid access log filter")
	// ErrAccessLogExportTooLarge reports an export over AccessLogExportLimit
	// entries; a narrower filter exports it.
	ErrAccessLogExportTooLarge = errors.New("traeger view: the access log export has too many entries")
)

// School is one school of the Träger in the school list (#4649). It carries
// no personal data.
type School struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Subdomain string `json:"subdomain"`
	City      string `json:"city"`
}

// SchoolIDs resolves the schools of a Träger at Organisation & Tenancy: the
// undeleted schools of an active Träger, none for any other. It joins the
// projection's administrative transaction.
type SchoolIDs interface {
	ListTraegerSchoolIDs(ctx context.Context, organizationID int64) ([]int64, error)
}

// Ports are what the projection reads from and writes to at other owners;
// the root binds them.
type Ports struct {
	// SchoolIDs is the school set of Organisation & Tenancy.
	SchoolIDs SchoolIDs
	// Trail and Recorder are the Audit Platform's access trail and the
	// access log of a school.
	Trail    AccessTrail
	Recorder AccessRecorder
	// Persons are the persons of People Directory, Bureau the Träger-Büro
	// addresses of Identity & Access.
	Persons PersonNames
	Bureau  BureauAddresses
}

// Overview answers the Träger-Portal reads.
type Overview interface {
	// Schools lists the schools of the Träger, ordered by name.
	Schools(ctx context.Context, organizationID int64) ([]School, error)
	// AccessLog lists one page of the access log, newest first.
	AccessLog(ctx context.Context, viewer Viewer, filter AccessLogFilter, page int) (AccessLogPage, error)
	// AccessLogExport lists every entry of the filter, newest first, up to
	// AccessLogExportLimit.
	AccessLogExport(ctx context.Context, viewer Viewer, filter AccessLogFilter) ([]AccessLogEntry, error)
	// AccessLogPeople lists the people with an entry in the window, for the
	// person filter.
	AccessLogPeople(ctx context.Context, viewer Viewer, from, to calendar.Date) ([]AccessLogPerson, error)
}

// The access log of the Träger-Portal (#4655, ADR 0084 section 7, ADR 0111):
// who read or changed personal data at a school of the Träger, and when.
// Reading it is itself a read of personal data (the names of the people), so
// every answer is written to the access log of each school it shows.

// The kinds of an access log entry.
const (
	AccessKindRead   = "read"
	AccessKindChange = "change"
)

// AccessLogPageSize is the number of entries on one page.
const AccessLogPageSize = 50

// AccessLogExportLimit bounds one CSV export.
const AccessLogExportLimit = 10000

// AccessLogMaxDays bounds the window of one filter.
const AccessLogMaxDays = 366

// Viewer is the person of the Träger-Büro who reads the access log.
type Viewer struct {
	OrganizationID int64
	AccountID      int64
}

// AccessLogFilter selects entries: the Berlin days From to To (both
// included, required) and optionally one school, one person and one kind.
type AccessLogFilter struct {
	From      calendar.Date
	To        calendar.Date
	SchoolID  int64
	AccountID int64
	Kind      string
}

// AccessLogEntry is one read or change. ID is stable across pages. AccountID is nil when no person is
// recorded; Person is the display name, a fallback text when none is known.
// Data names the data that was read or changed; DataLabel says it in plain
// German.
type AccessLogEntry struct {
	ID         string    `json:"id"`
	OccurredAt time.Time `json:"occurred_at"`
	SchoolID   int64     `json:"school_id"`
	SchoolName string    `json:"school_name"`
	AccountID  *int64    `json:"account_id"`
	Person     string    `json:"person"`
	Kind       string    `json:"kind"`
	Data       string    `json:"data"`
	DataLabel  string    `json:"data_label"`
}

// AccessLogPage is one page of the access log.
type AccessLogPage struct {
	Entries []AccessLogEntry `json:"entries"`
	HasMore bool             `json:"has_more"`
}

// AccessLogPerson is a person the access log can be filtered by.
type AccessLogPerson struct {
	AccountID int64  `json:"account_id"`
	Name      string `json:"name"`
}

// AccessTrail reads the reads and changes of personal data at a school set.
// The Audit Platform serves it; it joins the projection's administrative
// transaction and filters on the given schools.
type AccessTrail interface {
	ListAccessTrail(ctx context.Context, query AccessTrailQuery) ([]AccessTrailEntry, error)
	ListAccessTrailActors(ctx context.Context, schools []int64, from, to time.Time) ([]AccessTrailActor, error)
}

// AccessTrailQuery selects trail entries of Schools between the instants
// From (inclusive) and To (exclusive).
type AccessTrailQuery struct {
	Schools   []int64
	From      time.Time
	To        time.Time
	SchoolID  int64
	AccountID int64
	Kind      string
	Limit     int
	Offset    int
}

// AccessTrailEntry is one trail entry as the Audit Platform stores it.
type AccessTrailEntry struct {
	ID                int64
	SchoolID          int64
	OccurredAt        time.Time
	AccountID         *int64
	ActorNameSnapshot string
	Kind              string
	Data              string
}

// AccessTrailActor is an account with an entry at a school.
type AccessTrailActor struct {
	SchoolID          int64
	AccountID         int64
	ActorNameSnapshot string
}

// AccountName names an account: at a school (its person there) or, with
// SchoolID 0, in the Träger-Büro (its e-mail address).
type AccountName struct {
	AccountID int64
	SchoolID  int64
	Name      string
}

// PersonNames resolves the persons of accounts at every school, with their
// school. People Directory serves it in the projection's administrative
// transaction; the projection uses only the names at the Träger's schools.
type PersonNames interface {
	PersonNames(ctx context.Context, accountIDs []int64) ([]AccountName, error)
}

// BureauAddresses resolves the e-mail addresses of those accounts that are
// members of the Träger-Büro of the Träger (SchoolID 0). Identity & Access
// serves it in the projection's administrative transaction.
type BureauAddresses interface {
	BureauAddresses(ctx context.Context, organizationID int64, accountIDs []int64) ([]AccountName, error)
}

// TraegerAccess is one read of the Träger-Portal at one school, for the
// school's access log: who read, for which Träger, which part (View: list,
// people or export), the window, the page of a list and the number of rows
// the answer showed at the school.
type TraegerAccess struct {
	SchoolID       int64
	AccountID      int64
	OrganizationID int64
	ResourceType   string
	RangeStart     time.Time
	RangeEnd       time.Time
	View           string
	Page           int
	Rows           int
}

// The resource types the Träger-Portal writes to the access log of a school.
const (
	ResourceTypeTraegerAccessLog       = "traeger_access_log"
	ResourceTypeTraegerAccessLogExport = "traeger_access_log_export"
)

// AccessRecorder appends a read of the Träger-Portal to the access log of
// one school, in that school's tenant transaction (ADR 0084 section 7). The
// Audit Platform serves it.
type AccessRecorder interface {
	RecordTraegerAccess(ctx context.Context, access TraegerAccess) error
}
