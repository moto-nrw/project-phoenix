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
	StudentID  int64    `json:"student_id"`
	FirstName  string   `json:"first_name"`
	LastName   string   `json:"last_name"`
	GroupName  string   `json:"group_name,omitempty"`
	Registered bool     `json:"registered"`
	StaysToday bool     `json:"stays_today"`
	Offerings  []string `json:"offerings"`
	Arrival    string   `json:"arrival,omitempty"`
	Pickup     string   `json:"pickup,omitempty"`
	Departure  string   `json:"departure,omitempty"`
	// Status is the scheduled day status ("sick" / "excused" / "class_trip",
	// plus the derived "cancelled" when a pickup exception calls the care day
	// off), empty when none is reported. The free-text note stays private.
	Status string `json:"status,omitempty"`
}

type ClassDayTotals struct {
	Students int `json:"students"`
	Staying  int `json:"staying"`
	Leaving  int `json:"leaving"`
	Absent   int `json:"absent"`
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
func (s *reportService) classDayStatuses(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]string, error) {
	out := make(map[int64]string, len(studentIDs))
	if s.StudentStatusDayRepo == nil || len(studentIDs) == 0 {
		return out, nil
	}
	entries, err := s.StudentStatusDayRepo.FindActiveByStudentIDsAndDate(ctx, studentIDs, date)
	if err != nil {
		return nil, fmt.Errorf("class day report: load status days: %w", err)
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
		}
	}
	return out, nil
}

// classDayRosterRows builds class-roster rows for the class without an
// enrollment phase: full class list, group names, and the departure plan
// stored on the student — every "Keine Anmeldung" default the roster row
// builder produces for a nil enrollment.
func (s *reportService) classDayRosterRows(ctx context.Context, schoolClass string) ([]ClassRosterRow, error) {
	if s.StudentRepo == nil || s.PersonRepo == nil || s.EducationGroupRepo == nil {
		return nil, fmt.Errorf("class day report: repos not configured")
	}
	students, err := s.classRosterStudents(ctx, ClassRosterFilters{SchoolClass: schoolClass})
	if err != nil {
		return nil, err
	}
	if len(students) > maxReportRows {
		return nil, fmt.Errorf("class day report: %d students: %w", len(students), ErrReportExportTooLarge)
	}
	persons, err := s.PersonRepo.FindByIDs(ctx, classRosterPersonIDs(students))
	if err != nil {
		return nil, fmt.Errorf("class day report: load persons: %w", err)
	}
	groups, err := s.classRosterGroupNames(ctx, students)
	if err != nil {
		return nil, err
	}
	companions, err := s.classRosterCompanions(ctx, classRosterStudentIDs(students))
	if err != nil {
		return nil, err
	}
	rows := make([]ClassRosterRow, 0, len(students))
	for _, student := range students {
		if student == nil {
			continue
		}
		row, err := classRosterRow(student, persons[student.PersonID], classRosterGroupName(student, groups), nil, nil, nil, nil, companions[student.ID])
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

	phases, err := s.classDayPhases(ctx, date)
	if err != nil {
		return nil, err
	}

	weekday := classDayWeekdayKey(date)

	var rosterRows []ClassRosterRow
	switch {
	case len(phases) > 0:
		// Merge across EVERY covering phase (rollover overlap, parallel
		// Ferien phase): a child enrolled in only one of them must still
		// count as registered. OfferingDate pins the selection to the
		// requested day (Stichtags-Apply, #1665).
		rosters := make([][]ClassRosterRow, 0, len(phases))
		for _, phase := range phases {
			roster, err := s.ClassRoster(ctx, ClassRosterFilters{PhaseID: phase.id, SchoolClass: schoolClass, OfferingDate: &date})
			if err != nil {
				return nil, err
			}
			rosters = append(rosters, roster.Rows)
		}
		rosterRows = mergeClassDayRosters(rosters)
	default:
		rosterRows, err = s.classDayRosterRows(ctx, schoolClass)
		if err != nil {
			return nil, err
		}
	}

	studentIDs := make([]int64, 0, len(rosterRows))
	for _, row := range rosterRows {
		studentIDs = append(studentIDs, row.StudentID)
	}

	// Weekends render "Kein Schultag" — skip the status/departure/schedule
	// enrichment queries entirely; only the roster (names + count) is served.
	statuses := map[int64]string{}
	departures := map[int64]string{}
	arrivals := map[int64]string{}
	pickups := map[int64]string{}
	notScheduled := map[int64]bool{}
	if weekday != "" {
		statuses, err = s.classDayStatuses(ctx, studentIDs, date)
		if err != nil {
			return nil, err
		}
		departures, err = s.classDayDepartures(ctx, schoolClass, studentIDs, weekday)
		if err != nil {
			return nil, err
		}
		arrivals, pickups, err = s.classDayEffectiveTimes(ctx, studentIDs, date)
		if err != nil {
			return nil, err
		}
		var cancelled map[int64]bool
		cancelled, notScheduled, err = s.classDayCancellations(ctx, studentIDs, date)
		if err != nil {
			return nil, err
		}
		for studentID := range cancelled {
			// A stronger reported status (sick / class trip / excused) keeps
			// precedence over the plain "kommt heute nicht" cancellation.
			if statuses[studentID] == "" {
				statuses[studentID] = StudentStatusDayCancelled
			}
		}
	}

	phaseNames := make([]string, 0, len(phases))
	for _, phase := range phases {
		phaseNames = append(phaseNames, phase.name)
	}
	report := buildClassDayReport(schoolClass, date, strings.Join(phaseNames, ", "), rosterRows, statuses, departures, arrivals, pickups, notScheduled)
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
// modes; unlike the roster's week summary there is no day prefix.
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
func (s *reportService) classDayEffectiveTimes(ctx context.Context, studentIDs []int64, date timezone.Date) (arrivals, pickups map[int64]string, err error) {
	arrivals = map[int64]string{}
	pickups = map[int64]string{}
	if len(studentIDs) == 0 {
		return arrivals, pickups, nil
	}
	if s.PickupScheduleSvc != nil {
		effective, err := s.PickupScheduleSvc.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, date)
		if err != nil {
			return nil, nil, fmt.Errorf("class day report: load effective pickup times: %w", err)
		}
		for studentID, entry := range effective {
			if entry != nil && entry.PickupTime != nil {
				pickups[studentID] = entry.PickupTime.Format("15:04")
			}
		}
	}
	if s.ArrivalScheduleSvc != nil {
		effective, err := s.ArrivalScheduleSvc.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, date)
		if err != nil {
			return nil, nil, fmt.Errorf("class day report: load effective arrival times: %w", err)
		}
		for studentID, entry := range effective {
			if entry != nil && entry.ArrivalTime != nil {
				arrivals[studentID] = entry.ArrivalTime.Format("15:04")
			}
		}
	}
	return arrivals, pickups, nil
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
func (s *reportService) classDayCancellations(ctx context.Context, studentIDs []int64, date timezone.Date) (cancelled, notScheduled map[int64]bool, err error) {
	cancelled = map[int64]bool{}
	notScheduled = map[int64]bool{}
	if s.CareDaySvc == nil || len(studentIDs) == 0 {
		return cancelled, notScheduled, nil
	}
	statuses, err := s.CareDaySvc.ResolveForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, nil, fmt.Errorf("class day report: resolve care days: %w", err)
	}
	for studentID, status := range statuses {
		switch status {
		case scheduleService.CareDayCancelled:
			cancelled[studentID] = true
		case scheduleService.CareDayNotScheduled:
			notScheduled[studentID] = true
		}
	}
	return cancelled, notScheduled, nil
}

// classDayDepartures renders the departure plan of every student REDUCED to
// the requested weekday ("Abholung" instead of "Mo: Abholung, Di: …"): the
// handoff sheet answers today, not the week. Source is the live student row
// (AllowedDepartureModes with the DepartureDays fallback) — the current
// truth, present for every child, enrolled or not. Companion names are
// attached for days that allow the accompanied mode. Empty map on weekends.
func (s *reportService) classDayDepartures(ctx context.Context, schoolClass string, studentIDs []int64, weekday string) (map[int64]string, error) {
	out := make(map[int64]string, len(studentIDs))
	if weekday == "" || len(studentIDs) == 0 || s.StudentRepo == nil {
		return out, nil
	}
	students, err := s.StudentRepo.FindBySchoolClass(ctx, schoolClass)
	if err != nil {
		return nil, fmt.Errorf("class day report: load departure plans: %w", err)
	}
	companions, err := s.classRosterCompanions(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	for _, student := range students {
		if student == nil {
			continue
		}
		out[student.ID] = classDayDeparture(student, weekday, companions[student.ID])
	}
	return out, nil
}

// classDayDeparture renders one student's departure for one weekday. No
// plan data for the day means UNKNOWN, never "Geht alleine": on a sheet
// whose purpose is "wer geht wie nach Hause", missing data must not read as
// the instruction to let the child leave unaccompanied. The empty string
// makes buildClassDayReport render classDayDepartureUnknown — deliberately
// NOT the roster's form answer, whose week summary is never empty and
// floors at "Geht alleine" itself.
func classDayDeparture(student *userModels.Student, weekday string, companions []userModels.CompanionLink) string {
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
		if linked := userModels.FormatCompanionLinks(onDay); linked != "" {
			summary += " (" + linked + ")"
		} else if note := student.DepartureCompanionNote; note != nil && strings.TrimSpace(*note) != "" {
			summary += " (" + strings.TrimSpace(*note) + ")"
		}
	}
	return summary
}

// buildClassDayReport projects full roster rows onto one calendar day: the
// weekday's offerings decide who stays, a reported day status wins over any
// enrollment, a not-scheduled care day (materialized plan says "an dem Tag
// nicht gebucht") overrides the offering, and everyone else goes home after
// lessons. Effective arrival/pickup times (from the live plans) replace the
// roster's form-answer values when available; the departure column comes
// exclusively from the per-day plan (or "Keine Angabe") on school days.
func buildClassDayReport(schoolClass string, date timezone.Date, phaseName string, rosterRows []ClassRosterRow, statuses map[int64]string, departures, arrivals, pickups map[int64]string, notScheduled map[int64]bool) *ClassDayReport {
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
		if weekday != "" {
			offerings = append(offerings, row.OfferingsByDay[weekday]...)
			arrival = strings.TrimSpace(row.ArrivalByDay[weekday])
			pickup = strings.TrimSpace(row.PickupByDay[weekday])
			if effective := arrivals[row.StudentID]; effective != "" {
				arrival = effective
			}
			if effective := pickups[row.StudentID]; effective != "" {
				pickup = effective
			}
		}
		status := statuses[row.StudentID]
		// The materialized care plan is the current truth: a weekday the
		// parents struck from the plan beats the approved offering — the
		// same source the effective times above already come from.
		stays := len(offerings) > 0 && status == "" && !notScheduled[row.StudentID]
		// The per-day plan is the ONLY departure source. The roster's week
		// summary (row.Departure) is never empty — classRosterFormatDeparture
		// floors at "Geht alleine" — so falling back to it would fabricate an
		// unaccompanied departure for a child without any plan, or print the
		// whole week ("Mo: Bus, Di: Abholung") on a sheet that answers one
		// day. Missing data renders as explicit "Keine Angabe"; on a
		// non-school day the column stays empty entirely (mirror of the
		// zeroed totals below) — a weekend request must not serve any
		// departure instruction to non-UI consumers either.
		departure := ""
		if weekday != "" {
			departure = departures[row.StudentID]
			if departure == "" {
				departure = classDayDepartureUnknown
			}
		}
		dayRow := ClassDayRow{
			StudentID:  row.StudentID,
			FirstName:  row.FirstName,
			LastName:   row.LastName,
			GroupName:  row.GroupName,
			Registered: row.Registered,
			StaysToday: stays,
			Offerings:  offerings,
			Arrival:    arrival,
			Pickup:     pickup,
			Departure:  departure,
			Status:     status,
		}
		report.Rows = append(report.Rows, dayRow)
		report.Totals.Students++
		switch {
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
