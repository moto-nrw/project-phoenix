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

// ErrSchoolOutsideTraeger reports a school that is not a school of the Träger
// of the session. A school of another Träger and an unknown ID read the same,
// so the answer reveals no school.
var ErrSchoolOutsideTraeger = errors.New("traeger view: the school does not belong to the Träger")

// ErrTraegerUnavailable reports a Träger that is deactivated, deleted or
// unknown: its session reaches no school any more (#4657).
var ErrTraegerUnavailable = errors.New("traeger view: the Träger is deactivated or deleted")

// School is one school of the Träger in the school list (#4649). It carries
// no personal data.
type School struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Subdomain string `json:"subdomain"`
	City      string `json:"city"`
}

// SchoolContact is one school with the contact data it is reachable under. It
// names no child and no staff member.
type SchoolContact struct {
	School
	Address string `json:"address"`
	Zip     string `json:"zip"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
}

// SchoolView is one school of the Träger in its own view (#5686).
type SchoolView struct {
	SchoolContact
	Figures SchoolFigures `json:"figures"`
}

// SchoolFigures are the live figures of one school. AsOf is the instant they
// were read at. The binding definitions (ADR 0151):
//
//   - ChildrenTotal is the Stichtagszahl of School Membership (#2791), the
//     same rule the operator billing report captures (#4673).
//   - ChildrenPresent are the children with an open attendance of the day or
//     an open room visit, as on the school's own dashboard.
//   - ChildrenSick are the counted children with the live sick flag or an
//     uncleared sick day, as on the school's own dashboard.
//   - StaffTotal are the staff members without a current external caregiver
//     record, as on the school's time tracking dashboard.
//   - StaffPresent are those of them on site. Home office does not count.
//   - CareRatio is ChildrenPresent per StaffPresent; nil without present staff.
type SchoolFigures struct {
	ChildrenTotal   int       `json:"children_total"`
	ChildrenPresent int       `json:"children_present"`
	ChildrenSick    int       `json:"children_sick"`
	StaffTotal      int       `json:"staff_total"`
	StaffPresent    int       `json:"staff_present"`
	CareRatio       *float64  `json:"care_ratio"`
	AsOf            time.Time `json:"as_of"`
}

// The kinds of a hint on the Träger dashboard (#4675).
const (
	// AlertStaffShortage: present children and no present staff member, or
	// more present children per present staff member than the threshold.
	AlertStaffShortage = "staff_shortage"
	// AlertSickChildren: at least the threshold share of the children sick
	// today.
	AlertSickChildren = "sick_children"
	// AlertOpenEnrollments: at least the threshold of children of the online
	// enrollment waiting for a decision.
	AlertOpenEnrollments = "open_enrollments"
)

// AlertThresholds are the Träger's settings for the hints of its dashboard
// (#4675). 0 switches a hint off.
type AlertThresholds struct {
	ChildrenPerStaff int `json:"children_per_staff"`
	SickPercent      int `json:"sick_percent"`
	OpenEnrollments  int `json:"open_enrollments"`
}

// SchoolAlerts is one school of the Träger with the figures its hints come
// from and the hints that apply, in the order of the kinds above. It carries
// no personal data.
type SchoolAlerts struct {
	School
	Figures         SchoolFigures `json:"figures"`
	OpenEnrollments int           `json:"open_enrollments"`
	Alerts          []string      `json:"alerts"`
}

// SchoolsAlerts are the hints of every school of the Träger, ordered by
// name, and the thresholds they were checked against (#4675). Every school is
// listed, also one without a hint, so the dashboard can filter by school.
type SchoolsAlerts struct {
	Thresholds AlertThresholds `json:"thresholds"`
	Schools    []SchoolAlerts  `json:"schools"`
	AsOf       time.Time       `json:"as_of"`
}

// AlertSettings reads the Träger's thresholds at the settings system. It
// opens its own Träger transaction, so the projection calls it outside its
// administrative unit.
type AlertSettings interface {
	AlertThresholds(ctx context.Context, organizationID int64) (AlertThresholds, error)
}

// OpenEnrollments counts the children of the online enrollment waiting for
// a decision, at Enrollment. It joins the projection's administrative
// transaction and reads only the given schools.
type OpenEnrollments interface {
	OpenEnrollmentChildrenByTenant(ctx context.Context, schools []int64) (map[int64]int, error)
}

// SchoolRosters is who a school counts, at School Membership: the children
// with their memberships and the staff members. It joins the projection's
// administrative transaction and reads only the given schools, as the other
// figure ports do.
type SchoolRosters interface {
	ActiveStudentMembershipsByTenant(ctx context.Context, schools []int64, at time.Time) (map[int64]map[int64]int64, error)
	ActiveStaffIDsByTenant(ctx context.Context, schools []int64, at time.Time) (map[int64][]int64, error)
}

// PresentStudents are the children who are present, at Student Presence.
type PresentStudents interface {
	PresentStudentIDsByTenant(ctx context.Context, schools []int64, at time.Time) (map[int64][]int64, error)
}

// SickStudents is the two marks of a sick child, at Care Plan.
type SickStudents interface {
	SickDayStudentIDsByTenant(ctx context.Context, schools []int64, at time.Time) (map[int64][]int64, error)
	SickFlagMembershipIDsByTenant(ctx context.Context, schools []int64) (map[int64][]int64, error)
}

// OnSiteStaff is who is at work on site, at Workforce.
type OnSiteStaff interface {
	OnSiteStaffIDsByTenant(ctx context.Context, schools []int64, at time.Time) (map[int64][]int64, error)
}

// KeyDateCount is one captured Stichtagszahl of a school (#4673): the
// actively managed children and the active terminals on the key date of a
// month. The figures are the rows of the operator billing report (#2791),
// captured once and never changed; the binding definition of both numbers is
// "Stichtagszahl" in CONTEXT.md. Period is the month's first day; RecordedAt
// is a day after KeyDate when the capture ran late.
type KeyDateCount struct {
	Period          calendar.Date `json:"period"`
	KeyDate         calendar.Date `json:"key_date"`
	ActiveStudents  int           `json:"active_students"`
	ActiveTerminals int           `json:"active_terminals"`
	RecordedAt      time.Time     `json:"recorded_at"`
}

// SchoolKeyDateCount is a KeyDateCount with the school it belongs to.
type SchoolKeyDateCount struct {
	SchoolID int64
	KeyDateCount
}

// SchoolKeyDateCounts is one school of the Träger with its captured months,
// newest first. A school captured in no month yet has none.
type SchoolKeyDateCounts struct {
	School
	Counts []KeyDateCount `json:"counts"`
}

// KeyDateMonth is the sum of one captured month over the schools of the
// Träger: Schools counts the schools with a row in that month.
type KeyDateMonth struct {
	Period          calendar.Date `json:"period"`
	KeyDate         calendar.Date `json:"key_date"`
	ActiveStudents  int           `json:"active_students"`
	ActiveTerminals int           `json:"active_terminals"`
	Schools         int           `json:"schools"`
}

// KeyDateCounts are the Stichtagszahlen of every school of the Träger,
// ordered by name, and the sum of every captured month, newest first.
type KeyDateCounts struct {
	Schools []SchoolKeyDateCounts `json:"schools"`
	Months  []KeyDateMonth        `json:"months"`
}

// KeyDateCountReads reads the captured Stichtagszahlen of the schools of a
// Träger at Organisation & Tenancy, the owner of the operator billing report
// and of the Träger's school set. It joins the projection's administrative
// transaction; a deactivated, deleted or unknown Träger is
// ErrTraegerUnavailable.
type KeyDateCountReads interface {
	ListTraegerKeyDateCounts(ctx context.Context, organizationID int64) ([]SchoolKeyDateCount, error)
}

// Traeger is the Träger of the session as the portal shows it in its frame.
// TestSystem marks a test Träger with invented sample data (#4658).
type Traeger struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	TestSystem bool   `json:"test_system"`
}

// SchoolIDs resolves the schools of a Träger at Organisation & Tenancy: the
// undeleted schools of an active Träger. Any other Träger is
// ErrTraegerUnavailable. It joins the projection's administrative
// transaction.
type SchoolIDs interface {
	ListTraegerSchoolIDs(ctx context.Context, organizationID int64) ([]int64, error)
	// ListAccountTraeger returns the active, undeleted Träger where the
	// account's membership is active. It runs in Organisation & Tenancy's own
	// read unit; Träger names are no school data.
	ListAccountTraeger(ctx context.Context, accountID int64) ([]Traeger, error)
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
	// Holiday is Enrollment's trail of the Träger's holiday registrations.
	Holiday HolidayTrail
	// Persons are the persons of People Directory, Staff the staff members
	// of School Membership, Bureau the Träger-Büro addresses of Identity &
	// Access.
	Persons PersonDirectory
	Staff   StaffDirectory
	Bureau  BureauAddresses
	// Rosters, Present, Sick and OnSite are the owners' reads behind the
	// figures of a school (ADR 0151).
	Rosters SchoolRosters
	Present PresentStudents
	Sick    SickStudents
	OnSite  OnSiteStaff
	// KeyDates are the captured Stichtagszahlen of Organisation & Tenancy
	// (#4673).
	KeyDates KeyDateCountReads
	// Enrollments are the waiting children of Enrollment and Alerts the
	// Träger's thresholds at the settings system, behind the hints of the
	// dashboard (#4675).
	Enrollments OpenEnrollments
	Alerts      AlertSettings
	// Occupancy are the reads behind the occupancy of the schools (#4676).
	Occupancy OccupancyPorts
}

// Overview answers the Träger-Portal reads.
type Overview interface {
	// Traeger returns the Träger of the session while its account is still
	// an active member of it.
	Traeger(ctx context.Context, organizationID, accountID int64) (Traeger, error)
	// Schools lists the schools of the Träger, ordered by name.
	Schools(ctx context.Context, organizationID int64) ([]School, error)
	// School returns one school of the Träger with its figures. Any other
	// school is ErrSchoolOutsideTraeger.
	School(ctx context.Context, organizationID, schoolID int64) (SchoolView, error)
	// KeyDateCounts returns the captured Stichtagszahlen of every school of
	// the Träger and the sum of each month (#4673).
	KeyDateCounts(ctx context.Context, organizationID int64) (KeyDateCounts, error)
	// SchoolsAlerts returns the hints of every school of the Träger against
	// the Träger's thresholds (#4675).
	SchoolsAlerts(ctx context.Context, organizationID int64) (SchoolsAlerts, error)
	// Occupancy returns the children of every school of the Träger by OGS
	// group and care form, today (#4676).
	Occupancy(ctx context.Context, organizationID int64) (Occupancy, error)
	// AccessLog lists one page of the access log, newest first.
	AccessLog(ctx context.Context, viewer Viewer, filter AccessLogFilter, page int) (AccessLogPage, error)
	// AccessLogExport lists every entry of the filter, newest first, up to
	// AccessLogExportLimit.
	AccessLogExport(ctx context.Context, viewer Viewer, filter AccessLogFilter) ([]AccessLogEntry, error)
	// AccessLogPeople lists the people with an entry in the window, for the
	// person filter.
	AccessLogPeople(ctx context.Context, viewer Viewer, from, to calendar.Date) ([]AccessLogPerson, error)
}

// The access log of the Träger-Portal (#4655, ADR 0084 section 7, ADR 0109):
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
// AsOf, when set, keeps a list to the entries before that instant: the first
// page answers its AsOf, and the following pages send it back, so the reads
// the access log itself records meanwhile never shift a later page.
type AccessLogFilter struct {
	From      calendar.Date
	To        calendar.Date
	SchoolID  int64
	AccountID int64
	Kind      string
	AsOf      time.Time
}

// AccessLogEntry is one read or change. ID is stable across pages.
// AccountID is nil when no person is recorded; Person is the display name, a
// fallback text when none is known.
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
	// AsOf is the instant the list reads up to; the next page sends it back.
	AsOf time.Time `json:"as_of"`
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
	StaffIDs  []int64
	Kind      string
	Limit     int
	Offset    int
}

// AccessTrailEntry is one trail entry as the Audit Platform stores it. An
// entry of the HolidayTrail has no school: SchoolID is 0.
type AccessTrailEntry struct {
	ID                int64
	SchoolID          int64
	OccurredAt        time.Time
	AccountID         *int64
	StaffID           *int64
	ActorNameSnapshot string
	Kind              string
	Data              string
}

// HolidayTrail reads who of the Träger-Büro read or changed the Träger's
// holiday registrations (#5326). A registration belongs to the Träger, not
// to a school (ADR 0114 point 6). Enrollment serves it; it joins the
// projection's administrative transaction and filters on the given Träger.
type HolidayTrail interface {
	// ListHolidayTrail lists the entries of the query, newest first, as
	// trail entries without a school.
	ListHolidayTrail(ctx context.Context, organizationID int64, query HolidayTrailQuery) ([]AccessTrailEntry, error)
	// ListHolidayTrailActors lists the accounts with an entry between from
	// (inclusive) and to (exclusive) as actors without a school.
	ListHolidayTrailActors(ctx context.Context, organizationID int64, from, to time.Time) ([]AccessTrailActor, error)
}

// HolidayTrailQuery selects holiday trail entries between the instants From
// (inclusive) and To (exclusive), optionally of one account and one kind.
type HolidayTrailQuery struct {
	From      time.Time
	To        time.Time
	AccountID int64
	Kind      string
	Limit     int
}

// AccessTrailActor is an account or a staff member (one of AccountID and
// StaffID is set) with an entry at a school.
type AccessTrailActor struct {
	SchoolID          int64
	AccountID         int64
	StaffID           int64
	ActorNameSnapshot string
}

// AccountName names an account in the Träger-Büro by its e-mail address.
type AccountName struct {
	AccountID int64
	Name      string
}

// Person is a person of a school with the account it belongs to, if any.
type Person struct {
	PersonID  int64
	SchoolID  int64
	AccountID *int64
	Name      string
}

// PersonDirectory reads persons by account and by ID, at every school.
// People Directory serves it in the projection's administrative transaction;
// the projection uses only the persons of the Träger's schools.
type PersonDirectory interface {
	PersonsByAccount(ctx context.Context, accountIDs []int64) ([]Person, error)
	PersonsByID(ctx context.Context, personIDs []int64) ([]Person, error)
}

// StaffMember ties a staff ID at a school to its person.
type StaffMember struct {
	StaffID  int64
	SchoolID int64
	PersonID int64
}

// StaffDirectory reads staff members of the given schools, also former ones,
// by staff ID or by person. School Membership serves it in the projection's
// administrative transaction.
type StaffDirectory interface {
	StaffByID(ctx context.Context, schools, staffIDs []int64) ([]StaffMember, error)
	StaffOfPersons(ctx context.Context, schools, personIDs []int64) ([]StaffMember, error)
}

// BureauAddresses resolves the e-mail addresses of those accounts that are
// members of the Träger-Büro of the Träger. Identity & Access serves it in
// the projection's administrative transaction.
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
