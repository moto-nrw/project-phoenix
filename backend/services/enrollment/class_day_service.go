package enrollment

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// StudentStatusDayCancelled is the class-day-only marker for a pickup
// exception without a time: the care for this day was called off ("kommt
// heute nicht"). Not a persisted active.student_status_days status.
const StudentStatusDayCancelled = "cancelled"

// ClassDayRow is one student of the class on the requested day, reduced to
// what the handoff after lessons needs (#1772). Deliberately NO guardian
// names or contact details: the Lehrkraft view is a privacy-reduced
// projection of the class roster — who stays, who goes home, and how.
type ClassDayRow struct {
	StudentID int64  `json:"student_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	// ListEntry marks a class-list-only entry (#2382): a child of the class
	// cohort with NO OGS record ("Keine Betreuung"). StudentID is 0;
	// ListEntryID carries the users.class_list_entries id, serialized as a
	// JSON string because JavaScript clients round numbers beyond 2^53. The
	// row never stays, never has offerings, times or a departure plan — and
	// unlike a missing plan of an OGS child, that is a statement, not a gap.
	ListEntry   bool     `json:"list_entry,omitempty"`
	ListEntryID int64    `json:"list_entry_id,string,omitempty"`
	GroupName   string   `json:"group_name,omitempty"`
	Registered  bool     `json:"registered"`
	StaysToday  bool     `json:"stays_today"`
	Offerings   []string `json:"offerings"`
	Arrival     string   `json:"arrival,omitempty"`
	Pickup      string   `json:"pickup,omitempty"`
	Departure   string   `json:"departure,omitempty"`
	// Status is the scheduled day status ("sick" / "excused" / "class_trip",
	// plus the derived "cancelled" when a pickup exception calls the care day
	// off), empty when none is reported. The free-text note stays private.
	Status string `json:"status,omitempty"`
	// PickupChanged marks a pickup time that deviates from the child's
	// recurring plan for this weekday (#2294). Without it a Lehrkraft reads
	// "bis 12:15" as the normal plan and cannot tell that this child may go
	// home earlier today — the decision the issue is about.
	PickupChanged bool `json:"pickup_changed,omitempty"`
	// PickupRegular is the recurring plan's time for the weekday ("15:00"),
	// set only alongside PickupChanged and only when the plan has one.
	PickupRegular string `json:"pickup_regular,omitempty"`
	// ReportedAt is when the deviation became known: the status day's report
	// time, or, for a pure pickup change, the day exception's creation time.
	// Empty for rows without a deviation. It is what separates a change filed
	// this morning from one planned two weeks ago.
	ReportedAt *time.Time `json:"reported_at,omitempty"`
}

// classDayFacts are the per-student day facts the report projects onto the
// roster rows. Grouped instead of passed as ten parallel maps: every one of
// them is keyed by student ID for the same date, and the builder reads them
// together.
type classDayFacts struct {
	statuses         map[int64]string
	statusReportedAt map[int64]time.Time
	departures       map[int64]string
	arrivals         map[int64]string
	pickups          map[int64]string
	pickupRegular    map[int64]string
	pickupChanged    map[int64]bool
	pickupChangedAt  map[int64]time.Time
	notScheduled     map[int64]bool
}

func newClassDayFacts() classDayFacts {
	return classDayFacts{
		statuses:         map[int64]string{},
		statusReportedAt: map[int64]time.Time{},
		departures:       map[int64]string{},
		arrivals:         map[int64]string{},
		pickups:          map[int64]string{},
		pickupRegular:    map[int64]string{},
		pickupChanged:    map[int64]bool{},
		pickupChangedAt:  map[int64]time.Time{},
		notScheduled:     map[int64]bool{},
	}
}

type ClassDayTotals struct {
	Students int `json:"students"`
	Staying  int `json:"staying"`
	Leaving  int `json:"leaving"`
	Absent   int `json:"absent"`
	// ListEntries counts the class-list-only entries (#2382) among Students.
	// They are neither staying nor leaving: "Keine Betreuung" is a roster
	// statement, not a handoff instruction, so they get their own bucket
	// instead of inflating "Gehen nach Hause".
	ListEntries int `json:"list_entries"`
}

type ClassDayReport struct {
	SchoolClass string        `json:"school_class"`
	Date        timezone.Date `json:"date"`
	// Weekday is the report day key ("mon".."fri"), empty on weekends.
	Weekday   string `json:"weekday"`
	SchoolDay bool   `json:"school_day"`
	PhaseName string `json:"phase_name,omitempty"`
	// EnrollmentKnown is false when no enrollment phase covers the date: the
	// stays/leaves split is then unknowable, NOT "nobody stays" — consumers
	// must render a neutral class list instead of "alle gehen nach Hause".
	EnrollmentKnown bool           `json:"enrollment_known"`
	Totals          ClassDayTotals `json:"totals"`
	Rows            []ClassDayRow  `json:"rows"`
}

// classDayWeekdayKey maps a calendar date onto the report day keys used by
// OfferingsByDay / PickupByDay ("mon".."fri"). Weekend dates return "".
func classDayWeekdayKey(date timezone.Date) string {
	switch date.Weekday() {
	case time.Monday:
		return "mon"
	case time.Tuesday:
		return "tue"
	case time.Wednesday:
		return "wed"
	case time.Thursday:
		return "thu"
	case time.Friday:
		return "fri"
	default:
		return ""
	}
}

// classDayCoveringPhase is one enrollment phase whose service window covers
// the requested date.
type classDayCoveringPhase struct {
	id    int64
	name  string
	start timezone.Date
}

// classDayPhases returns every ACTIVE phase whose service window covers the
// date, latest start first (deterministic). Overlaps are real — rollover
// windows and a parallel Ferien phase both cover the same day — and a child
// enrolled in only ONE of them must still count as registered, so the class
// day view merges across all of them instead of picking a single "best"
// phase. Deactivated phases are excluded even when their window still
// covers the date: every other enrollment path treats !IsActive as a hard
// reject, and a cloned-then-deactivated Schuljahr phase whose window still
// runs must not put a child approved only there into "Bleiben in der
// Betreuung" with stale offerings and pickup times.
func (s *reportService) classDayPhases(ctx context.Context, date timezone.Date) ([]classDayCoveringPhase, error) {
	phases, err := s.PhaseRepo.ListByTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("class day report: list phases: %w", err)
	}
	covering := make([]classDayCoveringPhase, 0, 2)
	for _, phase := range phases {
		if phase == nil || !phase.IsActive {
			continue
		}
		if date.Before(phase.ServiceStartDate) || date.After(phase.ServiceEndDate) {
			continue
		}
		covering = append(covering, classDayCoveringPhase{id: phase.ID, name: phase.Name, start: phase.ServiceStartDate})
	}
	sort.SliceStable(covering, func(i, j int) bool {
		return covering[j].start.Before(covering[i].start)
	})
	return covering, nil
}

// mergeClassDayRosters folds the per-phase rosters of the SAME class into
// one row set: a student registered in any covering phase counts as
// registered, offerings and day maps union, the first non-empty value wins
// for scalar fields. Row order follows the first roster (already sorted).
func mergeClassDayRosters(rosters [][]ClassRosterRow) []ClassRosterRow {
	if len(rosters) == 0 {
		return nil
	}
	merged := make([]ClassRosterRow, len(rosters[0]))
	copy(merged, rosters[0])
	index := make(map[int64]int, len(merged))
	for i := range merged {
		index[merged[i].StudentID] = i
	}
	for _, roster := range rosters[1:] {
		for _, row := range roster {
			at, ok := index[row.StudentID]
			if !ok {
				index[row.StudentID] = len(merged)
				merged = append(merged, row)
				continue
			}
			base := &merged[at]
			if !row.Registered {
				continue
			}
			if !base.Registered {
				base.Registered = true
				base.EnrollmentSummary = row.EnrollmentSummary
			}
			base.Offerings = append(base.Offerings, row.Offerings...)
			for day, names := range row.OfferingsByDay {
				if base.OfferingsByDay == nil {
					base.OfferingsByDay = map[string][]string{}
				}
				base.OfferingsByDay[day] = appendMissingStrings(base.OfferingsByDay[day], names)
			}
			base.CareDays = appendMissingStrings(base.CareDays, row.CareDays)
			for day, value := range row.ArrivalByDay {
				if base.ArrivalByDay == nil {
					base.ArrivalByDay = map[string]string{}
				}
				if base.ArrivalByDay[day] == "" {
					base.ArrivalByDay[day] = value
				}
			}
			for day, value := range row.PickupByDay {
				if base.PickupByDay == nil {
					base.PickupByDay = map[string]string{}
				}
				if base.PickupByDay[day] == "" {
					base.PickupByDay[day] = value
				}
			}
		}
	}
	return merged
}

// appendMissingStrings appends the values not already present (exact match).
func appendMissingStrings(base []string, add []string) []string {
	seen := make(map[string]bool, len(base))
	for _, value := range base {
		seen[value] = true
	}
	for _, value := range add {
		if !seen[value] {
			seen[value] = true
			base = append(base, value)
		}
	}
	return base
}

// classDayStatuses loads the scheduled day statuses of the listed students.
// Precedence mirrors the dashboard counts: sick wins, class trip beats a
// plain excuse.
func (s *reportService) classDayStatuses(ctx context.Context, studentIDs []int64, date timezone.Date) (statuses map[int64]string, reportedAt map[int64]time.Time, err error) {
	out := make(map[int64]string, len(studentIDs))
	stamps := make(map[int64]time.Time, len(studentIDs))
	if s.StudentStatusDayRepo == nil || len(studentIDs) == 0 {
		return out, stamps, nil
	}
	entries, err := s.StudentStatusDayRepo.FindActiveByStudentIDsAndDate(ctx, studentIDs, date)
	if err != nil {
		return nil, nil, fmt.Errorf("class day report: load status days: %w", err)
	}
	// The scale leaves rank 1 free for UNKNOWN statuses: the status set is
	// an explicit extension point (StudentStatusDayStatuses()), and a value
	// this map does not know still means "reported for the day". Dropping
	// it silently would render a reported-absent child as "bleibt in der
	// Betreuung" — the exact failure this view exists to prevent. Known
	// statuses keep precedence over an unknown one.
	rank := map[string]int{
		activeModels.StudentStatusDaySick:      6,
		activeModels.StudentStatusDayClassTrip: 4,
		activeModels.StudentStatusDayExcused:   2,
	}
	statusRank := func(status string) int {
		if status == "" {
			return 0
		}
		if known, ok := rank[status]; ok {
			return known
		}
		return 1
	}
	for _, entry := range entries {
		if entry == nil || entry.Status == "" {
			continue
		}
		if statusRank(entry.Status) > statusRank(out[entry.StudentID]) {
			out[entry.StudentID] = entry.Status
			// The stamp follows the winning status, not the newest row: it
			// answers "since when is THIS the situation" (#2294).
			if !entry.ReportedAt.IsZero() {
				stamps[entry.StudentID] = entry.ReportedAt
			} else {
				delete(stamps, entry.StudentID)
			}
		}
	}
	return out, stamps, nil
}

// classDayRosterRows builds class-roster rows for an already-loaded class
// without an enrollment phase: full class list and group names — every
// "Keine Anmeldung" default the roster row builder produces for a nil
// enrollment. No companion links: the day view renders departures
// exclusively from classDayDepartures, the roster's per-day departure map
// never reaches the sheet.
func (s *reportService) classDayRosterRows(ctx context.Context, students []*userModels.Student) ([]ClassRosterRow, error) {
	if s.PersonRepo == nil || s.EducationGroupRepo == nil {
		return nil, fmt.Errorf("class day report: repos not configured")
	}
	persons, err := s.PersonRepo.FindByIDs(ctx, classRosterPersonIDs(students))
	if err != nil {
		return nil, fmt.Errorf("class day report: load persons: %w", err)
	}
	groups, err := s.classRosterGroupNames(ctx, students)
	if err != nil {
		return nil, err
	}
	rows := make([]ClassRosterRow, 0, len(students))
	for _, student := range students {
		if student == nil {
			continue
		}
		row, err := classRosterRow(student, persons[student.PersonID], classRosterGroupName(student, groups), nil, nil, nil, nil, nil, true)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	sortClassRosterRows(rows)
	return rows, nil
}

// ClassDay builds the read-only per-class day view for the Lehrkraft handoff
// (#1772): the full class (including students without any enrollment), per
// student whether they stay in care on that day, which offerings apply, the
// arrival/pickup times, and the departure plan. Caller scoping (which classes
// the account may see) is the API layer's job via usercontext — this method
// only knows the class name.
func (s *reportService) ClassDay(ctx context.Context, schoolClass string, date timezone.Date, actorAccountID int64, actorRole string) (*ClassDayReport, error) {
	schoolClass = strings.TrimSpace(schoolClass)
	if schoolClass == "" {
		return nil, fmt.Errorf("class day report: school class required: %w", ErrReportInvalidFilter)
	}
	// Fail fast instead of degrading: a partially wired service would serve
	// a sheet where a sick or abgemeldetes Kind shows as "bleibt in der
	// Betreuung" — the exact failure this view exists to prevent.
	if s.StudentStatusDayRepo == nil || s.CareDaySvc == nil || s.PickupScheduleSvc == nil || s.ArrivalScheduleSvc == nil {
		return nil, fmt.Errorf("class day report: status/schedule dependencies not configured")
	}

	if s.StudentRepo == nil {
		return nil, fmt.Errorf("class day report: repos not configured")
	}

	phases, err := s.classDayPhases(ctx, date)
	if err != nil {
		return nil, err
	}

	weekday := classDayWeekdayKey(date)

	// The class is loaded ONCE and shared across every covering phase's
	// roster build and the departure rendering below. The view fans out per
	// class on the teachers' landing page — re-reading identical students
	// per phase (and again for the departures) was pure query fan-out.
	// The date matters for more than the offering selection: a child whose
	// care ended belongs on the sheets of the days they were still there and
	// on none after that (#2487).
	students, err := s.classRosterStudents(ctx, ClassRosterFilters{SchoolClass: schoolClass, OfferingDate: &date})
	if err != nil {
		return nil, err
	}
	if len(students) > maxReportRows {
		return nil, fmt.Errorf("class day report: %d students: %w", len(students), ErrReportExportTooLarge)
	}

	var rosterRows []ClassRosterRow
	switch {
	case len(phases) > 0:
		// Merge across EVERY covering phase (rollover overlap, parallel
		// Ferien phase): a child enrolled in only one of them must still
		// count as registered. OfferingDate pins the selection to the
		// requested day (Stichtags-Apply, #1665). SkipGuardianData: the day
		// view serves no guardian contacts and renders departures from the
		// live per-day plan, so the roster's guardian/companion queries
		// would run per phase for nothing.
		rosters := make([][]ClassRosterRow, 0, len(phases))
		for _, phase := range phases {
			roster, err := s.classRosterForStudents(ctx, ClassRosterFilters{PhaseID: phase.id, SchoolClass: schoolClass, OfferingDate: &date, SkipGuardianData: true}, students)
			if err != nil {
				return nil, err
			}
			rosters = append(rosters, roster.Rows)
		}
		rosterRows = mergeClassDayRosters(rosters)
	default:
		rosterRows, err = s.classDayRosterRows(ctx, students)
		if err != nil {
			return nil, err
		}
	}

	// The class-list-only entries (#2382) complete the Klassenverband:
	// children without any OGS record, marked "Keine Betreuung". They sort in
	// alphabetically like every student row and can never stay in care.
	entryRows, err := s.classListEntryRows(ctx, schoolClass, false)
	if err != nil {
		return nil, err
	}
	if len(entryRows) > 0 {
		if len(rosterRows)+len(entryRows) > maxReportRows {
			return nil, fmt.Errorf("class day report: %d rows: %w", len(rosterRows)+len(entryRows), ErrReportExportTooLarge)
		}
		rosterRows = append(rosterRows, entryRows...)
		sortClassRosterRows(rosterRows)
	}

	studentIDs := make([]int64, 0, len(rosterRows))
	for _, row := range rosterRows {
		if row.StudentID > 0 {
			studentIDs = append(studentIDs, row.StudentID)
		}
	}

	// Weekends render "Kein Schultag" — skip the status/departure/schedule
	// enrichment queries entirely; only the roster (names + count) is served.
	facts := newClassDayFacts()
	if weekday != "" {
		facts.statuses, facts.statusReportedAt, err = s.classDayStatuses(ctx, studentIDs, date)
		if err != nil {
			return nil, err
		}
		facts.departures, err = s.classDayDepartures(ctx, students, weekday)
		if err != nil {
			return nil, err
		}
		if err := s.classDayEffectiveTimes(ctx, studentIDs, date, &facts); err != nil {
			return nil, err
		}
		cancelled, err := s.classDayCancellations(ctx, studentIDs, date, &facts)
		if err != nil {
			return nil, err
		}
		for studentID := range cancelled {
			// A stronger reported status (sick / class trip / excused) keeps
			// precedence over the plain "kommt heute nicht" cancellation.
			if facts.statuses[studentID] == "" {
				facts.statuses[studentID] = StudentStatusDayCancelled
				// The cancellation is a timeless day exception; its creation
				// time is when the abmeldung became known (#2294).
				if stamp, ok := facts.pickupChangedAt[studentID]; ok {
					facts.statusReportedAt[studentID] = stamp
				}
			}
		}
	}

	phaseNames := make([]string, 0, len(phases))
	for _, phase := range phases {
		phaseNames = append(phaseNames, phase.name)
	}
	report := buildClassDayReport(schoolClass, date, strings.Join(phaseNames, ", "), rosterRows, facts)
	report.EnrollmentKnown = len(phases) > 0
	if !report.EnrollmentKnown {
		// Without a covering phase the stays/leaves split is unknowable —
		// zero the counters so no consumer can print "alle gehen nach Hause".
		report.Totals.Staying = 0
		report.Totals.Leaving = 0
	}

	if err := s.recordClassDayViewAudit(ctx, report, actorAccountID, actorRole); err != nil {
		return nil, err
	}
	return report, nil
}

// classDayDepartureUnknown renders a student without any departure data for
// the day. Deliberately distinct from an explicit "Geht alleine" plan: on
// the handoff sheet, missing data is a question for the office, not an
// instruction.
const classDayDepartureUnknown = "Keine Angabe"

// classDayModeLabels are the day-view labels for a single day's departure
// modes for the handoff sheet; the roster cells use their own lowercase
// phrasing (classRosterDayModeLabels).
var classDayModeLabels = map[userModels.DepartureMode]string{
	userModels.DepartureAlone:       "Geht alleine",
	userModels.DepartureBus:         "Bus",
	userModels.DeparturePickup:      "Abholung",
	userModels.DepartureAccompanied: "Mit anderem Kind",
}

// classDayEffectiveTimes loads the effective arrival/pickup times of the
// students for the date from the materialized schedule tables (weekly plan
// plus day exceptions) — the current truth the roster's form-answer snapshot
// may lag behind. Times only: the "kommt heute nicht" decision belongs to
// classDayCancellations.
func (s *reportService) classDayEffectiveTimes(ctx context.Context, studentIDs []int64, date timezone.Date, facts *classDayFacts) error {
	if len(studentIDs) == 0 {
		return nil
	}
	if s.PickupScheduleSvc != nil {
		effective, err := s.PickupScheduleSvc.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, date)
		if err != nil {
			return fmt.Errorf("class day report: load effective pickup times: %w", err)
		}
		for studentID, entry := range effective {
			if entry == nil {
				continue
			}
			applyClassDayPickup(facts, studentID, entry)
		}
	}
	if s.ArrivalScheduleSvc != nil {
		effective, err := s.ArrivalScheduleSvc.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, date)
		if err != nil {
			return fmt.Errorf("class day report: load effective arrival times: %w", err)
		}
		for studentID, entry := range effective {
			if entry != nil && entry.ArrivalTime != nil {
				facts.arrivals[studentID] = entry.ArrivalTime.Format("15:04")
			}
		}
	}
	return nil
}

// applyClassDayPickup records one student's pickup facts for the day: the
// effective time, and whether it deviates from the recurring plan.
//
// A day exception alone is NOT a deviation — a parent may re-enter the time
// the plan already holds, and announcing "geht heute um 15:00 statt 15:00"
// would train the Lehrkraft to ignore the block. The deviation is the
// comparison: a different clock time, or a time on a day the plan has none
// (the child normally is not in care then). A timeless exception carries no
// time at all; that is "kommt heute nicht" and travels as a status, not as a
// changed pickup.
func applyClassDayPickup(facts *classDayFacts, studentID int64, entry *scheduleService.EffectivePickupTime) {
	if entry.PickupTime != nil {
		facts.pickups[studentID] = entry.PickupTime.Format("15:04")
	}
	if !entry.IsException {
		return
	}
	if entry.ChangedAt != nil && !entry.ChangedAt.IsZero() {
		facts.pickupChangedAt[studentID] = *entry.ChangedAt
	}
	if entry.PickupTime == nil {
		return
	}
	effective := entry.PickupTime.Format("15:04")
	regular := ""
	if entry.RegularPickupTime != nil {
		regular = entry.RegularPickupTime.Format("15:04")
	}
	if regular == effective {
		return
	}
	facts.pickupChanged[studentID] = true
	facts.pickupRegular[studentID] = regular
}

// classDayCancellations resolves which students are not coming that day,
// through the schedule domain's own CareDayService: a timeless exception on
// EITHER leg (arrival or pickup) cancels the day, with the same precedence
// the student search, parent portal and Tagesauswertung use. Deliberately
// not derived from raw entries here. CareDayNotScheduled ("an dem Tag nicht
// gebucht", e.g. the parents struck the weekday from the care plan while the
// approved offering still lists it) is returned separately: not an absence,
// but the child must not be listed as staying either — every other reader
// treats it as not-expected.
func (s *reportService) classDayCancellations(ctx context.Context, studentIDs []int64, date timezone.Date, facts *classDayFacts) (cancelled map[int64]bool, err error) {
	cancelled = map[int64]bool{}
	if s.CareDaySvc == nil || len(studentIDs) == 0 {
		return cancelled, nil
	}
	statuses, err := s.CareDaySvc.ResolveForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, fmt.Errorf("class day report: resolve care days: %w", err)
	}
	for studentID, status := range statuses {
		switch status {
		case scheduleService.CareDayCancelled:
			cancelled[studentID] = true
		case scheduleService.CareDayNotScheduled:
			facts.notScheduled[studentID] = true
		}
	}
	return cancelled, nil
}

// classDayDepartures renders the departure plan of every student REDUCED to
// the requested weekday ("Abholung" instead of "Mo: Abholung, Di: …"): the
// handoff sheet answers today, not the week. Source is the live student row
// (AllowedDepartureModes with the DepartureDays fallback) — the current
// truth, present for every child, enrolled or not, and already loaded once
// by ClassDay (no re-query per consumer). Companion names are attached only
// for children of this very class (see classDayDeparture). Empty map on
// weekends.
func (s *reportService) classDayDepartures(ctx context.Context, students []*userModels.Student, weekday string) (map[int64]string, error) {
	out := make(map[int64]string, len(students))
	if weekday == "" || len(students) == 0 {
		return out, nil
	}
	studentIDs := classRosterStudentIDs(students)
	companions, err := s.classRosterCompanions(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	// The class being served IS the disclosure boundary of this view, so it
	// is also the set of names a companion entry may mention.
	onSheet := make(map[int64]bool, len(studentIDs))
	for _, id := range studentIDs {
		onSheet[id] = true
	}
	for _, student := range students {
		if student == nil {
			continue
		}
		out[student.ID] = classDayDeparture(student, weekday, companions[student.ID], onSheet)
	}
	return out, nil
}

// classDayCompanionsOnSheet drops companion links pointing at children the
// caller does not already see on this sheet. A Laufgemeinschaft may pair a 1a
// child with a 4c child (users.student_companions is tenant-scoped, not
// class-scoped), and the class-day view is deliberately narrower than the
// tenant-wide student directory: a Lehrkraft holds class_day:read for their
// own education.class_teachers assignments, never users:read.
func classDayCompanionsOnSheet(links []userModels.CompanionLink, onSheet map[int64]bool) []userModels.CompanionLink {
	kept := make([]userModels.CompanionLink, 0, len(links))
	for _, link := range links {
		if onSheet[link.CompanionStudentID] {
			kept = append(kept, link)
		}
	}
	return kept
}

// classDayDeparture renders one student's departure for one weekday. No
// plan data for the day means UNKNOWN, never "Geht alleine": on a sheet
// whose purpose is "wer geht wie nach Hause", missing data must not read as
// the instruction to let the child leave unaccompanied. The empty string
// makes buildClassDayReport render classDayDepartureUnknown — deliberately
// NOT the roster's form answer, whose per-day map is never empty and
// floors at "geht alleine" itself.
//
// onSheet holds the students of the class being served; an accompanied
// departure names only companions from that set. The free-text
// DepartureCompanionNote is deliberately NOT a fallback here: parents write
// it, and "Abholung durch Nachbarin Frau Meier, 0171-…" is exactly the
// guardian name and contact detail this view promises never to show. An
// accompanied departure the sheet cannot name stays "Mit anderem Kind" — a
// question for the office, like every other gap on this sheet.
func classDayDeparture(student *userModels.Student, weekday string, companions []userModels.CompanionLink, onSheet map[int64]bool) string {
	allowed := student.AllowedDepartureModes.Normalize()
	if !allowed.HasAny() {
		allowed = userModels.AllowedDepartureModesFromDeparture(student.DepartureDays)
	}
	modes := allowed[weekday]
	if len(modes) == 0 {
		return ""
	}
	labels := make([]string, 0, len(modes))
	accompanied := false
	for _, mode := range modes {
		if mode == userModels.DepartureAccompanied {
			accompanied = true
		}
		if label := classDayModeLabels[mode]; label != "" {
			labels = append(labels, label)
		}
	}
	summary := strings.Join(labels, ", ")
	if accompanied {
		onDay := userModels.FilterCompanionLinksToDays(companions, map[string]bool{weekday: true})
		if linked := userModels.FormatCompanionLinks(classDayCompanionsOnSheet(onDay, onSheet)); linked != "" {
			summary += " (" + linked + ")"
		}
	}
	return summary
}

// classDayReportedAt answers "since when is this known" for a deviating row,
// and only for one: a row that matches the regular plan has nothing to date.
// A reported day status outranks a changed pickup time — it is the stronger
// statement, and the badge the reader sees follows the same precedence.
func classDayReportedAt(facts classDayFacts, studentID int64, status string, pickupChanged bool) *time.Time {
	if status != "" {
		if stamp, ok := facts.statusReportedAt[studentID]; ok {
			return &stamp
		}
		return nil
	}
	if pickupChanged {
		if stamp, ok := facts.pickupChangedAt[studentID]; ok {
			return &stamp
		}
	}
	return nil
}

// buildClassDayReport projects full roster rows onto one calendar day: the
// weekday's offerings decide who stays, a reported day status wins over any
// enrollment, a not-scheduled care day (materialized plan says "an dem Tag
// nicht gebucht") overrides the offering, and everyone else goes home after
// lessons. Effective arrival/pickup times (from the live plans) replace the
// roster's form-answer values when available; the departure column comes
// exclusively from the per-day plan (or "Keine Angabe") on school days.
func buildClassDayReport(schoolClass string, date timezone.Date, phaseName string, rosterRows []ClassRosterRow, facts classDayFacts) *ClassDayReport {
	weekday := classDayWeekdayKey(date)
	report := &ClassDayReport{
		SchoolClass: schoolClass,
		Date:        date,
		Weekday:     weekday,
		SchoolDay:   weekday != "",
		PhaseName:   phaseName,
		Rows:        make([]ClassDayRow, 0, len(rosterRows)),
	}
	for _, row := range rosterRows {
		offerings := []string{}
		arrival, pickup := "", ""
		pickupChanged, pickupRegular := false, ""
		if weekday != "" {
			offerings = append(offerings, row.OfferingsByDay[weekday]...)
			arrival = strings.TrimSpace(row.ArrivalByDay[weekday])
			pickup = strings.TrimSpace(row.PickupByDay[weekday])
			if effective := facts.arrivals[row.StudentID]; effective != "" {
				arrival = effective
			}
			if effective := facts.pickups[row.StudentID]; effective != "" {
				pickup = effective
			}
			pickupChanged = facts.pickupChanged[row.StudentID]
			pickupRegular = facts.pickupRegular[row.StudentID]
		}
		status := facts.statuses[row.StudentID]
		// The materialized care plan is the current truth: a weekday the
		// parents struck from the plan beats the approved offering — the
		// same source the effective times above already come from.
		stays := len(offerings) > 0 && status == "" && !facts.notScheduled[row.StudentID]
		// The per-day plan is the ONLY departure source. The roster's map
		// (row.DepartureByDay) is never empty — classRosterFormatDepartureByDay
		// floors every day at "geht alleine" — so falling back to it would
		// fabricate an unaccompanied departure for a child without any plan.
		// Missing data renders as explicit "Keine Angabe"; on a
		// non-school day the column stays empty entirely (mirror of the
		// zeroed totals below) — a weekend request must not serve any
		// departure instruction to non-UI consumers either.
		departure := ""
		if weekday != "" && !row.ListEntry {
			// A class-list-only entry has no departure column at all: "Keine
			// Betreuung" is the whole statement, and rendering "Keine Angabe"
			// would suggest a plan gap the office should fill.
			departure = facts.departures[row.StudentID]
			if departure == "" {
				departure = classDayDepartureUnknown
			}
		}
		// A class-list-only entry has no care plan at all (#2382): it can
		// neither deviate from one nor carry a report time.
		if row.ListEntry {
			pickupChanged, pickupRegular = false, ""
		}
		dayRow := ClassDayRow{
			StudentID:     row.StudentID,
			FirstName:     row.FirstName,
			LastName:      row.LastName,
			ListEntry:     row.ListEntry,
			ListEntryID:   row.ListEntryID,
			GroupName:     row.GroupName,
			Registered:    row.Registered,
			StaysToday:    stays,
			Offerings:     offerings,
			Arrival:       arrival,
			Pickup:        pickup,
			Departure:     departure,
			Status:        status,
			PickupChanged: pickupChanged,
			PickupRegular: pickupRegular,
			ReportedAt:    classDayReportedAt(facts, row.StudentID, status, pickupChanged),
		}
		report.Rows = append(report.Rows, dayRow)
		report.Totals.Students++
		switch {
		case row.ListEntry:
			// A class-list-only entry neither stays nor leaves — "Keine
			// Betreuung" is its own neutral roster bucket, never a
			// "geht nach Hause" claim.
			report.Totals.ListEntries++
		case status != "":
			report.Totals.Absent++
		case stays:
			report.Totals.Staying++
		default:
			report.Totals.Leaving++
		}
	}
	if !report.SchoolDay {
		// Kein Schultag: there is no handoff, so "geht nach Hause" is not a
		// statement either — only the Klassenverband count stays honest.
		report.Totals.Staying = 0
		report.Totals.Leaving = 0
		report.Totals.Absent = 0
		report.Totals.ListEntries = 0
	}
	return report
}

// recordClassDayViewAudit writes the GDPR access log for a class day view:
// care and departure data of an entire class served to one account, under
// its own resource type and the caller's actual roles.
//
// Deduplicated to one row per actor, class, report date and calendar day of
// access: the view revalidates itself (interval + tab focus), and a second
// identical row every few minutes would bloat the append-only table and
// destroy the log's evidential value. A view on a LATER calendar day writes
// again — "looked at it again the next day" stays auditable.
func (s *reportService) recordClassDayViewAudit(ctx context.Context, report *ClassDayReport, actorAccountID int64, actorRole string) error {
	if s.DataAccessLogRepo == nil {
		return nil
	}
	seen, err := s.DataAccessLogRepo.ExistsSince(ctx, actorAccountID, auditModels.ResourceTypeClassDayView,
		map[string]string{
			"school_class": report.SchoolClass,
			"date":         report.Date.String(),
		},
		timezone.TodayDate().BerlinMidnight())
	if err != nil {
		return fmt.Errorf("class day view audit dedupe: %w", err)
	}
	if seen {
		return nil
	}
	entry, err := exportAuditEntry("class day view audit", actorAccountID, actorRole,
		auditModels.ResourceTypeClassDayView,
		report.Date.BerlinMidnight(), report.Date.EndOfDay(), time.Now())
	if err != nil {
		return err
	}
	entry.SetMetadata("report", "class_day")
	entry.SetMetadata("school_class", report.SchoolClass)
	entry.SetMetadata("date", report.Date.String())
	entry.SetMetadata("student_count", report.Totals.Students)
	return writeExportAudit(ctx, s.DataAccessLogRepo, entry, "class day view audit")
}
