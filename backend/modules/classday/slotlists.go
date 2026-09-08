// Package classday is the public seam of the class-day read projection
// (#2701). It serves two read-only views over the owner queries of Timetable
// & Activities, Student Presence, People Directory, School Structure,
// Facilities and Care Plan:
//
//   - the slot lists of a day (issue #1565): printable lists from planned
//     timetable slots (schedule.activity_instances + schedule.instance_students)
//     and the live attendance picture (active.visits / active.attendance),
//     kept apart in three data modes — planned, actual, reconciliation — and
//     scoped either to concrete slots or to a Ganztag pickup cohort;
//   - the per-class day view of the school portal (#1772): which children of
//     a class stay in care, which go home, and how, plus the class-wide
//     arrival day exceptions a Lehrkraft may enter (#2970).
//
// The projection persists nothing and is never a write owner. Every consumer
// (the timetable list endpoints, the school portal, the PDF/XLSX export) reads
// it through the interfaces declared here; the composition lives in
// modules/classday/compose.
package classday

import (
	"context"
	"errors"

	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

// Date is a calendar day in "YYYY-MM-DD" form. The projection resolves it in
// the Berlin calendar; a malformed value is rejected before any read.
type Date string

// String returns the ISO calendar day.
func (d Date) String() string { return string(d) }

// ErrPickupCohortPastDate is returned when a Ganztag pickup list is requested
// for a date before today. Recurring pickup schedules
// (schedule.student_pickup_schedules) carry no validity interval and no
// historical snapshot, so a past pickup plan cannot be faithfully
// reconstructed: the *current* schedule would be projected onto the past date
// and silently move children between cohorts or show a time that was never
// planned then (#1565 review). Slot-based lists remain available for past dates
// — only pickup cohorts are refused. The handler maps this to HTTP 400.
var ErrPickupCohortPastDate = errors.New("Ganztag-Listen sind nur für heute und künftige Tage verfügbar") //nolint:staticcheck // ST1005: user-facing German message

// ErrReconciliationFutureDate is returned when an Abgleich (reconciliation) list
// is requested for a strictly-future date. Reconciliation compares the plan
// against documented presence, and a future day has no presence evidence yet, so
// every planned child would fall through to "Fehlt" — a printable list claiming
// the whole group is missing before the day has even started (#1565 review). Plan
// and Ist lists stay available for future dates; only the merge is refused. The
// handler maps this to HTTP 400.
var ErrReconciliationFutureDate = errors.New("Ein Abgleich ist nur für heute und vergangene Tage möglich, nicht für künftige Tage") //nolint:staticcheck // ST1005: user-facing German message

// ErrListDrifted is returned by RenderList when the freshly rebuilt list no
// longer matches the content signature the client verified in its preview
// (Params.ExpectedSignature). Live attendance, roster or plan data changed
// between the client's preview verification and this export render, so honoring
// the export would hand out a file the user never reviewed. The handler maps
// this to HTTP 409 so the client can refresh the preview and ask the user to
// re-check before exporting again (#1565 review pass 2).
var ErrListDrifted = errors.New("Die Liste hat sich seit der Vorschau geändert. Bitte erneut prüfen und exportieren.") //nolint:staticcheck // ST1005: user-facing German message

// ErrTimetableDisabled is returned before any list data is read when the
// tenant has disabled timetable.enabled. The slot-list HTTP handlers map this
// to 403, making the feature flag authoritative even for callers who have the
// schedule/user permissions but cannot read the settings schema themselves.
var ErrTimetableDisabled = errors.New("timetable feature is disabled")

// SlotLists builds slot lists for preview (JSON) and export (PDF/XLSX).
type SlotLists interface {
	BuildList(ctx context.Context, params Params) (*Result, error)
	ListOptions(ctx context.Context, date Date) (*OptionsResult, error)
	RenderList(ctx context.Context, params Params, format listexport.Format) (listexport.File, error)
}

// Target selects which data domain a list is built from.
type Target string

const (
	TargetSlots        Target = "slots"
	TargetPickupCohort Target = "pickup_cohort"
)

// Valid reports whether t is a known target.
func (t Target) Valid() bool {
	switch t {
	case TargetSlots, TargetPickupCohort:
		return true
	default:
		return false
	}
}

func (t Target) PickupBased() bool {
	return t == TargetPickupCohort
}

// Label returns the German display name used in UI and document titles.
func (t Target) Label() string {
	switch t {
	case TargetSlots:
		return "Freie Angebotsauswahl"
	case TargetPickupCohort:
		return "Ganztag"
	default:
		return string(t)
	}
}

// ListKind selects a stable timetable list classification on activity
// templates/instances. Empty means the manual "Freie Angebotsauswahl" flow.
type ListKind string

const (
	ListKindNone         ListKind = ""
	ListKindEdgeHours    ListKind = activitiesModel.ListKindEdgeHours
	ListKindLearningTime ListKind = activitiesModel.ListKindLearningTime
	ListKindActivity     ListKind = activitiesModel.ListKindActivity
	ListKindMensa        ListKind = activitiesModel.ListKindMensa
)

var AllListKinds = []ListKind{
	ListKindEdgeHours,
	ListKindLearningTime,
	ListKindActivity,
	ListKindMensa,
}

func (k ListKind) Valid() bool {
	return activitiesModel.IsValidListKind(string(k))
}

func (k ListKind) Label() string {
	return activitiesModel.ListKindLabel(string(k))
}

// PickupCohort selects the configured Ganztag pickup bucket.
type PickupCohort string

const (
	PickupCohortShortDay PickupCohort = "short_day"
	PickupCohortLongDay  PickupCohort = "long_day"
)

func (p PickupCohort) Valid() bool {
	switch p {
	case PickupCohortShortDay, PickupCohortLongDay:
		return true
	default:
		return false
	}
}

// GroupBy selects how preview rows and export sections are grouped. Empty
// means a single flat list. The valid set depends on the target (see ValidFor):
// slot/room only make sense for slot lists, pickup_time only for Ganztag pickup
// cohorts; class works for every target.
type GroupBy string

const (
	GroupByNone       GroupBy = ""
	GroupBySlot       GroupBy = "slot"
	GroupByRoom       GroupBy = "room"
	GroupByClass      GroupBy = "class"
	GroupByPickupTime GroupBy = "pickup_time"
)

// ValidFor reports whether g is an allowed grouping for the given target.
func (g GroupBy) ValidFor(target Target) bool {
	switch g {
	case GroupByNone, GroupByClass:
		return true
	case GroupBySlot, GroupByRoom:
		return target == TargetSlots
	case GroupByPickupTime:
		return target == TargetPickupCohort
	default:
		return false
	}
}

// Label returns the German display name of the grouping.
func (g GroupBy) Label() string {
	switch g {
	case GroupBySlot:
		return "Angebot"
	case GroupByRoom:
		return "Raum"
	case GroupByClass:
		return "Klasse"
	case GroupByPickupTime:
		return "Abholzeit"
	default:
		return ""
	}
}

// Source selects the data mode of the list.
type Source string

const (
	SourcePlanned        Source = "planned"
	SourceActual         Source = "actual"
	SourceReconciliation Source = "reconciliation"
)

// Valid reports whether s is a known source.
func (s Source) Valid() bool {
	switch s {
	case SourcePlanned, SourceActual, SourceReconciliation:
		return true
	default:
		return false
	}
}

// Label returns the German display name of the data mode.
func (s Source) Label() string {
	switch s {
	case SourcePlanned:
		return "Plan"
	case SourceActual:
		return "Ist"
	case SourceReconciliation:
		return "Abgleich"
	default:
		return string(s)
	}
}

// Params are the inputs of one list build. The three filter slices are all
// optional and combine with AND; an empty slice means "no restriction". The
// full available option sets are always reported back in Result so the UI can
// offer re-selection regardless of the current filter.
type Params struct {
	Date   Date
	Target Target
	Source Source
	// PickupCohort is required when Target is pickup_cohort and ignored for
	// slot-based lists.
	PickupCohort PickupCohort
	// InstanceIDs restricts a slot-based list to those activity instances (the
	// selected slots). Empty means all non-cancelled slots on the selected date.
	// Ignored for pickup cohorts. When ListKind is set, this optionally
	// restricts within that list kind.
	InstanceIDs []int64
	// InstanceIDsSet distinguishes an omitted instance_ids field (all slots)
	// from an explicit empty array (no slots selected).
	InstanceIDsSet bool
	// ListKind restricts a slot-based list to instances explicitly classified
	// for that list kind. Empty keeps the manual free-offer selection.
	ListKind ListKind
	// GroupIDs restricts to children of those education groups. Applies to
	// every target, including Ganztag pickup cohorts.
	GroupIDs []int64
	// Classes restricts to those school classes. Applies to every target.
	Classes []string
	// GroupBy sections the preview/export. Empty = one flat list. Invalid
	// combinations (see GroupBy.ValidFor) are rejected at the API boundary.
	GroupBy GroupBy
	// ExpectedSignature is the content hash (Result.Signature) of the preview
	// the client reviewed. When set, RenderList refuses (ErrListDrifted) if its
	// fresh build no longer matches it, so an export never differs from the
	// approved preview (#1565 review pass 2). Empty on preview requests and on
	// older clients — the guard is then skipped for backward compatibility.
	ExpectedSignature string
}

// Slot is one activity instance on the selected date, returned so the UI can
// offer concrete single- or multi-slot selection and explain empty results.
type Slot struct {
	InstanceID int64  `json:"instance_id,string"`
	Title      string `json:"title"`
	TimeRange  string `json:"time_range"`
	Status     string `json:"status"`
	// ListKind and RoomName are echoed so the frontend's pre-export options
	// guard can detect document-affecting drift that leaves the active
	// instance-ID set unchanged (#1565 review pass 4): a list_kind reassignment
	// moves a slot in or out of a classified list (the backend intersects
	// list_kind with instance_ids, so request.instance_ids stays equal yet a
	// different slot exports), and a room change — reassignment or a rename of
	// the same room — reshapes a room-grouped export. Empty when unset.
	ListKind string `json:"list_kind,omitempty"`
	RoomName string `json:"room_name,omitempty"`
}

// GroupOption is one selectable education group in the group filter.
type GroupOption struct {
	ID   int64  `json:"id,string"`
	Name string `json:"name"`
}

// Row is one child on the list.
type Row struct {
	StudentID   int64  `json:"student_id,string"`
	Name        string `json:"name"`
	SchoolClass string `json:"school_class"`
	GroupName   string `json:"group_name"`
	GroupID     *int64 `json:"group_id,string,omitempty"`
	InstanceID  int64  `json:"instance_id,string,omitempty"`
	Slot        string `json:"slot"`
	RoomName    string `json:"room_name,omitempty"`
	PickupTime  string `json:"pickup_time,omitempty"`
	Planned     bool   `json:"planned"`
	Present     bool   `json:"present"`
	Unplanned   bool   `json:"unplanned"`
	// Excused is a planned-but-absent child whose absence carries an explicit
	// instance_students.substatus. Lifecycle no-shows flip to status=absent
	// without substatus and still count as unexplained "Fehlt".
	Excused     bool   `json:"excused"`
	StatusLabel string `json:"status_label"`
	// GroupTitle is the section heading this row belongs to when GroupBy is
	// set (empty for a flat list). Echoed to the UI and stamped on exports.
	GroupTitle string `json:"group_title,omitempty"`
}

// Counters summarize the list. Which counters are meaningful depends on the
// source: planned fills Planned, actual fills Present, reconciliation fills
// all four.
type Counters struct {
	Planned int `json:"planned"`
	Present int `json:"present"`
	// Missing counts planned children who are absent without a registered
	// sign-off (unexplained). Excused (abgemeldet) is counted separately.
	Missing   int `json:"missing"`
	Excused   int `json:"excused"`
	Unplanned int `json:"unplanned"`
}

// Result is the preview payload (and the input of the export rendering).
// Slots/Groups/Classes are the full available option sets (before the filters
// are applied), so the UI dropdowns stay populated while rows/counters reflect
// the active selection.
type Result struct {
	Date         string        `json:"date"`
	Target       Target        `json:"target"`
	PickupCohort PickupCohort  `json:"pickup_cohort,omitempty"`
	ListKind     ListKind      `json:"list_kind,omitempty"`
	ListLabel    string        `json:"list_label"`
	Source       Source        `json:"source"`
	GroupBy      GroupBy       `json:"group_by,omitempty"`
	Provenance   string        `json:"provenance"`
	Slots        []Slot        `json:"slots"`
	Groups       []GroupOption `json:"groups"`
	Classes      []string      `json:"classes"`
	Counters     Counters      `json:"counters"`
	Rows         []Row         `json:"rows"`
	// Signature is a stable content hash of the rendered list (label,
	// provenance, counters and every row). The export endpoint re-derives the
	// list in a second request after the client verified this preview, so live
	// attendance/roster changes in that window would otherwise hand out a file
	// the user never reviewed. The client echoes this value back as the export
	// request's expected_signature; RenderList refuses (ErrListDrifted) when its
	// own fresh build no longer matches (#1565 review pass 2).
	Signature string `json:"signature"`
	// DeferredSlots holds instance IDs that appear in Slots as selectable
	// context but were deliberately excluded from the merge (a reconciliation
	// slot that has not started yet). It never serializes; it exists only so
	// the export "Enthalten" summary does not count slots the export contains
	// no rows for (#1565 review). Consumers must treat it as opaque.
	DeferredSlots map[int64]struct{} `json:"-"`
	// ExportHeaderSignature is the rendered document header (title + every
	// export filter line) fingerprinted into Signature. The row/counter hash
	// alone does not cover the "Enthalten" slot summary, which is derived from
	// Slots plus the deferred set — so a rowless selected slot cancelled between
	// the verified preview and the export rebuild would change the printed
	// header while the row/counter hash stayed equal and slipped past the 409
	// drift guard (#1565 review pass 7). Precomputed in BuildList because the
	// header needs Params (the slot selection, classes, grouping); it never
	// serializes. Consumers must treat it as opaque.
	ExportHeaderSignature string `json:"-"`
}

// PickupCohortOption describes whether a Ganztag pickup bucket has rows on a
// date. Slots themselves are returned separately as concrete options.
type PickupCohortOption struct {
	Cohort    PickupCohort `json:"cohort"`
	Label     string       `json:"label"`
	Available bool         `json:"available"`
	RowCount  int          `json:"row_count"`
}

type ListKindOption struct {
	Kind      ListKind `json:"kind"`
	Label     string   `json:"label"`
	Available bool     `json:"available"`
	SlotCount int      `json:"slot_count"`
	RowCount  int      `json:"row_count"`
}

type OptionsResult struct {
	Date          string               `json:"date"`
	Slots         []Slot               `json:"slots"`
	PickupCohorts []PickupCohortOption `json:"pickup_cohorts"`
	ListKinds     []ListKindOption     `json:"list_kinds"`
}
