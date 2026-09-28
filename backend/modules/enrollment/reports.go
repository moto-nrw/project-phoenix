package enrollment

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Sentinels of the enrollment reports. The HTTP adapters classify them; the
// messages are part of the wire text of a refused request.
var (
	ErrReportPhaseNotFound  = errors.New("enrollment report phase not found")
	ErrReportExportTooLarge = errors.New("enrollment report has too many rows for a single export")
	ErrReportInvalidFilter  = errors.New("enrollment report filter is invalid")
)

// ClassListEntryNoCareLabel marks a class-list-only entry (#2382) on every
// class-list surface: the child has no OGS record at all — deliberately
// distinct from "Keine Anmeldung" (a known OGS child without an enrollment in
// this phase).
const ClassListEntryNoCareLabel = "Keine Betreuung"

// Reports are Enrollment's staff reports over one phase — the care usage and
// the class roster, each with its audited export — and the class roster of
// one calendar day that the class-day view reads.
type Reports interface {
	CareUsage(ctx context.Context, filters CareUsageFilters) (*CareUsageReport, error)
	// ExportCareUsage builds the care usage report and records the export in
	// the GDPR access log.
	ExportCareUsage(ctx context.Context, filters CareUsageFilters, actorAccountID int64, actorRole, format string, compact bool) (*CareUsageReport, error)
	// ExportClassRoster builds the class roster and records the export in
	// the GDPR access log.
	ExportClassRoster(ctx context.Context, filters ClassRosterFilters, actorAccountID int64, actorRole, format string) (*ClassRosterReport, error)
	// ClassRosterDay is the roster of one class on one calendar day, merged
	// across every covering phase. It writes no access log: the consumer
	// serves the day view and audits it.
	ClassRosterDay(ctx context.Context, schoolClass string, date calendar.Date) (*ClassRosterDay, error)
}

// ReportOfferingDate selects the current point within a phase. Reports for a
// future phase show the selection that will apply at its start; reports for a
// completed phase show the final selection instead of mixing all intervals.
func ReportOfferingDate(today calendar.Date, phase *Phase) calendar.Date {
	if phase == nil {
		return today
	}
	start, end := calendar.Date(phase.ServiceStartDate), calendar.Date(phase.ServiceEndDate)
	if start.IsZero() && end.IsZero() {
		return today
	}
	if !start.IsZero() && today.Before(start) {
		return start
	}
	if !end.IsZero() && today.After(end) {
		return end
	}
	return today
}

type CareUsageFilters struct {
	PhaseID            int64   `json:"phase_id"`
	Status             string  `json:"status,omitempty"`
	CareOfferingIDs    []int64 `json:"care_offering_ids,omitempty"`
	CareOfferingIDsSet bool    `json:"-"`
	DayCount           *int    `json:"day_count,omitempty"`
	GradeLevel         *int16  `json:"grade_level,omitempty"`
	Weekday            string  `json:"weekday,omitempty"`
	PickupTime         string  `json:"pickup_time,omitempty"`
	Search             string  `json:"search,omitempty"`
}

type CareUsageReport struct {
	Phase         CareUsagePhase          `json:"phase"`
	Filters       CareUsageAppliedFilters `json:"filters"`
	Totals        CareUsageTotals         `json:"totals"`
	ByOffering    []CareUsageOfferingStat `json:"by_offering"`
	FilterOptions CareUsageFilterOptions  `json:"filter_options"`
	Rows          []CareUsageRow          `json:"rows"`
}

type CareUsagePhase struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type CareUsageAppliedFilters struct {
	PhaseID         int64   `json:"phase_id"`
	Status          string  `json:"status"`
	CareOfferingIDs []int64 `json:"care_offering_ids"`
	DayCount        *int    `json:"day_count,omitempty"`
	GradeLevel      *int16  `json:"grade_level,omitempty"`
	Weekday         string  `json:"weekday,omitempty"`
	PickupTime      string  `json:"pickup_time,omitempty"`
	Search          string  `json:"search,omitempty"`
}

type CareUsageTotals struct {
	Children            int                       `json:"children"`
	ByDayCount          map[string]int            `json:"by_day_count"`
	ByWeekdayPickupTime map[string]map[string]int `json:"by_weekday_pickup_time"`
}

type CareUsageOfferingStat struct {
	OfferingID   int64          `json:"offering_id"`
	OfferingName string         `json:"offering_name"`
	Children     int            `json:"children"`
	ByDayCount   map[string]int `json:"by_day_count"`
}

type CareUsageFilterOptions struct {
	Offerings   []CareUsageOfferingOption `json:"offerings"`
	GradeLevels []int16                   `json:"grade_levels"`
	PickupTimes []string                  `json:"pickup_times"`
}

type CareUsageOfferingOption struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	CountsAsCare bool   `json:"counts_as_care"`
}

type CareUsageRow struct {
	RequestID         int64                  `json:"request_id"`
	ChildID           int64                  `json:"child_id"`
	ChildFirstName    string                 `json:"child_first_name"`
	ChildLastName     string                 `json:"child_last_name"`
	DateOfBirth       string                 `json:"date_of_birth"`
	TargetGradeLevel  *int16                 `json:"target_grade_level,omitempty"`
	TargetSchoolClass *string                `json:"target_school_class,omitempty"`
	Status            string                 `json:"status"`
	Offerings         []CareUsageRowOffering `json:"offerings"`
	EffectiveDays     []string               `json:"effective_days"`
	// CareDays is the display-only set of weekdays used by the compact
	// export. Unlike EffectiveDays, it includes every weekday when care is
	// not constrained by an active offering catalog.
	CareDays          []string          `json:"-"`
	DayCount          int               `json:"day_count"`
	PickupByDay       map[string]string `json:"pickup_by_day"`
	GuardianFirstName string            `json:"guardian_first_name"`
	GuardianLastName  string            `json:"guardian_last_name"`
	GuardianEmail     string            `json:"guardian_email"`
	GuardianPhone     *string           `json:"guardian_phone,omitempty"`
	SubmittedAt       time.Time         `json:"submitted_at"`
	// SchedulePickupByDay is the maintained Kind-Gehzeit (manual rows and
	// rolled-out Angebots-Gehzeiten, #2290) of the student linked to this
	// request child, when one exists. Display-only enrichment for the
	// compact export (#2215): filters and totals keep using the
	// enrollment-form snapshot in PickupByDay.
	SchedulePickupByDay map[string]string `json:"schedule_pickup_by_day"`
	// Guardians are all guardian contacts on the request (primary plus
	// additional request guardians), mirroring the class-roster contact
	// column. The flat Guardian* fields above stay the submitting guardian.
	Guardians []ClassRosterGuardian `json:"guardians"`
}

type CareUsageRowOffering struct {
	ID                    int64    `json:"id"`
	Name                  string   `json:"name"`
	Days                  []string `json:"days"`
	DaysSource            string   `json:"days_source"`
	DaysOfWeekMode        string   `json:"days_of_week_mode"`
	ManualSelectedDays    []string `json:"manual_selected_days,omitempty"`
	AutomaticSelectedDays []string `json:"automatic_selected_days,omitempty"`
}

type ClassRosterFilters struct {
	PhaseID     int64  `json:"phase_id"`
	SchoolClass string `json:"school_class"`
	AllClasses  bool   `json:"all_classes"`
	// OfferingDate pins the offering-link selection to a specific calendar
	// day (#1772 class day view: paging to Monday must show Monday's
	// selection, not today's). Nil keeps the export default: today clamped
	// to the phase window.
	OfferingDate *calendar.Date `json:"offering_date,omitempty"`
	// SkipGuardianData omits the guardian-facing enrichments (emergency
	// contacts, request guardians, companion links). The class day view
	// keeps only Registered/OfferingsByDay/Arrival-/PickupByDay and serves
	// no guardian contact data at all (#1772) — and it rebuilds the roster
	// per covering phase for every class on the teachers' landing page, so
	// these three queries would be pure fan-out waste there. Internal-only,
	// never bound from a request.
	SkipGuardianData bool `json:"-"`
}

type ClassRosterReport struct {
	Phase   CareUsagePhase            `json:"phase"`
	Filters ClassRosterAppliedFilters `json:"filters"`
	Totals  ClassRosterTotals         `json:"totals"`
	Rows    []ClassRosterRow          `json:"rows"`
}

type ClassRosterAppliedFilters struct {
	PhaseID     int64  `json:"phase_id"`
	SchoolClass string `json:"school_class"`
	AllClasses  bool   `json:"all_classes"`
	Status      string `json:"status"`
}

type ClassRosterTotals struct {
	Students   int `json:"students"`
	Registered int `json:"registered"`
	// ListEntries counts the class-list-only entries (#2382) among Students.
	ListEntries int `json:"list_entries"`
}

type ClassRosterRow struct {
	StudentID         int64  `json:"student_id"`
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	SchoolClass       string `json:"school_class"`
	GroupName         string `json:"group_name,omitempty"`
	Registered        bool   `json:"registered"`
	EnrollmentSummary string `json:"enrollment_summary"`
	// ListEntry marks a class-list-only entry (#2382): a child of the class
	// cohort with NO OGS record. StudentID is 0 for these rows; ListEntryID
	// carries the users.class_list_entries id instead, serialized as a JSON
	// string because JavaScript clients round numbers beyond 2^53. The row
	// exists only so the Klassenverband is complete — it can never carry
	// offerings, times or guardians.
	ListEntry      bool                   `json:"list_entry,omitempty"`
	ListEntryID    int64                  `json:"list_entry_id,string,omitempty"`
	Offerings      []CareUsageRowOffering `json:"offerings"`
	OfferingsByDay map[string][]string    `json:"offerings_by_day"`
	CareDays       []string               `json:"care_days"`
	ArrivalByDay   map[string]string      `json:"arrival_by_day"`
	PickupByDay    map[string]string      `json:"pickup_by_day"`
	// SchedulePickupByDay is the maintained Kind-Gehzeit from
	// schedule.student_pickup_schedules (manual rows and rolled-out
	// Angebots-Gehzeiten). It outranks the enrollment-form answer in the
	// weekday cells (#2290).
	SchedulePickupByDay map[string]string `json:"schedule_pickup_by_day"`
	// DepartureByDay carries the compact Geh-/Abholregelung per weekday code
	// ("mon".."fri"), e.g. "wird abgeholt" — the weekday cells print it next
	// to the pickup time (#2254). Every weekday has an entry; a day without an
	// explicit rule carries the business default "geht alleine".
	DepartureByDay map[string]string     `json:"departure_by_day"`
	Guardians      []ClassRosterGuardian `json:"guardians"`
}

type ClassRosterGuardian struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

// ClassRosterDay is the roster of one class on one calendar day (#1772):
// the students who still belong to the class that day (#2487), merged across
// every active phase whose service window covers the day, plus the
// class-list-only entries (#2382), sorted like every roster.
type ClassRosterDay struct {
	// PhaseNames are the covering phases, latest service start first. Empty
	// when no phase covers the day: the stays/leaves split is then unknown.
	PhaseNames []string
	Rows       []ClassRosterRow
	// Students are the class's students in load order, each with the
	// departure modes its live plan allows on the day's weekday.
	Students []ClassRosterDayStudent
}

// ClassRosterDayStudent is one student of a day roster with the departure
// modes ("alone", "bus", "pickup", "accompanied") its live plan allows on
// the day's weekday: the allowed modes, or the exclusive day plan for a child
// that has none. Empty on a weekend and for a child without a plan.
type ClassRosterDayStudent struct {
	ID             int64
	DepartureModes []string
}
