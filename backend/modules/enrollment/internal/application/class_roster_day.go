package application

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// classDayCoveringPhase is one enrollment phase whose service window covers
// the requested date.
type classDayCoveringPhase struct {
	id    int64
	name  string
	start calendar.Date
}

// ClassRosterDay builds the roster of one class on one calendar day for the
// class-day view (#1772): the full class (including students without any
// enrollment), merged across every covering phase, plus the class-list-only
// entries. The class is loaded ONCE and shared across every covering phase's
// roster build: the view fans out per class on the teachers' landing page.
func (s *Reports) ClassRosterDay(ctx context.Context, schoolClass string, date calendar.Date) (*enrollment.ClassRosterDay, error) {
	schoolClass = strings.TrimSpace(schoolClass)
	if s.deps.Students == nil {
		return nil, fmt.Errorf("class day report: repos not configured")
	}
	phases, err := s.classDayPhases(ctx, date)
	if err != nil {
		return nil, err
	}
	// The date matters for more than the offering selection: a child whose
	// care ended belongs on the sheets of the days they were still there and
	// on none after that (#2487).
	students, err := s.classRosterStudents(ctx, enrollment.ClassRosterFilters{SchoolClass: schoolClass, OfferingDate: &date})
	if err != nil {
		return nil, err
	}
	if len(students) > maxReportRows {
		return nil, fmt.Errorf("class day report: %d students: %w", len(students), enrollment.ErrReportExportTooLarge)
	}
	rows, err := s.classDayRosterRows(ctx, schoolClass, date, phases, students)
	if err != nil {
		return nil, err
	}
	out := &enrollment.ClassRosterDay{
		PhaseNames: make([]string, 0, len(phases)),
		Rows:       rows,
		Students:   classDayStudents(students, classDayWeekdayKey(date)),
	}
	for _, phase := range phases {
		out.PhaseNames = append(out.PhaseNames, phase.name)
	}
	return out, nil
}

// classDayRosterRows merges the rosters of every covering phase (or builds
// the enrollment-less roster when none covers the day) and completes the
// Klassenverband with the class-list-only entries (#2382).
func (s *Reports) classDayRosterRows(ctx context.Context, schoolClass string, date calendar.Date, phases []classDayCoveringPhase, students []*RosterStudent) ([]enrollment.ClassRosterRow, error) {
	rows, err := s.classDayPhaseRows(ctx, schoolClass, date, phases, students)
	if err != nil {
		return nil, err
	}
	entryRows, err := s.classListEntryRows(ctx, schoolClass, false)
	if err != nil {
		return nil, err
	}
	if len(entryRows) == 0 {
		return rows, nil
	}
	if len(rows)+len(entryRows) > maxReportRows {
		return nil, fmt.Errorf("class day report: %d rows: %w", len(rows)+len(entryRows), enrollment.ErrReportExportTooLarge)
	}
	rows = append(rows, entryRows...)
	sortClassRosterRows(rows)
	return rows, nil
}

func (s *Reports) classDayPhaseRows(ctx context.Context, schoolClass string, date calendar.Date, phases []classDayCoveringPhase, students []*RosterStudent) ([]enrollment.ClassRosterRow, error) {
	if len(phases) == 0 {
		return s.classRosterRowsWithoutPhase(ctx, students)
	}
	// Merge across EVERY covering phase (rollover overlap, parallel Ferien
	// phase): a child enrolled in only one of them must still count as
	// registered. OfferingDate pins the selection to the requested day
	// (Stichtags-Apply, #1665). SkipGuardianData: the day view serves no
	// guardian contacts and renders departures from the live per-day plan,
	// so the roster's guardian/companion queries would run per phase for
	// nothing.
	rosters := make([][]enrollment.ClassRosterRow, 0, len(phases))
	for _, phase := range phases {
		roster, err := s.classRosterForStudents(ctx, enrollment.ClassRosterFilters{PhaseID: phase.id, SchoolClass: schoolClass, OfferingDate: &date, SkipGuardianData: true}, students)
		if err != nil {
			return nil, err
		}
		rosters = append(rosters, roster.Rows)
	}
	return mergeClassDayRosters(rosters), nil
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
func (s *Reports) classDayPhases(ctx context.Context, date calendar.Date) ([]classDayCoveringPhase, error) {
	phases, err := s.deps.Phases.Phases(ctx)
	if err != nil {
		return nil, fmt.Errorf("class day report: list phases: %w", err)
	}
	covering := make([]classDayCoveringPhase, 0, 2)
	for _, phase := range phases {
		if phase == nil || !phase.IsActive {
			continue
		}
		start, end := calendar.Date(phase.ServiceStartDate), calendar.Date(phase.ServiceEndDate)
		if date.Before(start) || date.After(end) {
			continue
		}
		covering = append(covering, classDayCoveringPhase{id: phase.ID, name: phase.Name, start: start})
	}
	sort.SliceStable(covering, func(i, j int) bool {
		return covering[j].start.Before(covering[i].start)
	})
	return covering, nil
}

// classRosterRowsWithoutPhase builds class-roster rows for an already-loaded
// class without an enrollment phase: full class list and group names — every
// "Keine Anmeldung" default the roster row builder produces without an
// enrollment. No companion links: the day view renders departures
// exclusively from the live per-day plan.
func (s *Reports) classRosterRowsWithoutPhase(ctx context.Context, students []*RosterStudent) ([]enrollment.ClassRosterRow, error) {
	if s.deps.Persons == nil || s.deps.Groups == nil {
		return nil, fmt.Errorf("class day report: repos not configured")
	}
	in := &classRosterInputs{students: students}
	var err error
	if in.persons, err = s.deps.Persons.PersonsByID(ctx, classRosterPersonIDs(students)); err != nil {
		return nil, fmt.Errorf("class day report: load persons: %w", err)
	}
	if groupIDs := classRosterGroupIDs(students); len(groupIDs) > 0 {
		if in.groups, err = s.deps.Groups.GroupNamesByID(ctx, groupIDs); err != nil {
			return nil, fmt.Errorf("class roster report: load groups: %w", err)
		}
	}
	rows := make([]enrollment.ClassRosterRow, 0, len(students))
	for _, student := range students {
		if student == nil {
			continue
		}
		row, err := in.row(student)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	sortClassRosterRows(rows)
	return rows, nil
}

// mergeClassDayRosters folds the per-phase rosters of the SAME class into
// one row set: a student registered in any covering phase counts as
// registered, offerings and day maps union, the first non-empty value wins
// for scalar fields. Row order follows the first roster (already sorted).
func mergeClassDayRosters(rosters [][]enrollment.ClassRosterRow) []enrollment.ClassRosterRow {
	if len(rosters) == 0 {
		return nil
	}
	merged := make([]enrollment.ClassRosterRow, len(rosters[0]))
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
			if row.Registered {
				mergeRegisteredRow(&merged[at], row)
			}
		}
	}
	return merged
}

func mergeRegisteredRow(base *enrollment.ClassRosterRow, row enrollment.ClassRosterRow) {
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
	base.ArrivalByDay = fillMissingDays(base.ArrivalByDay, row.ArrivalByDay)
	base.PickupByDay = fillMissingDays(base.PickupByDay, row.PickupByDay)
}

// fillMissingDays copies the values of add into the days base has no value
// for.
func fillMissingDays(base, add map[string]string) map[string]string {
	for day, value := range add {
		if base == nil {
			base = map[string]string{}
		}
		if base[day] == "" {
			base[day] = value
		}
	}
	return base
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

// classDayWeekdayKey maps a calendar date onto the report day keys ("mon"
// .."fri"). Weekend dates return "".
func classDayWeekdayKey(date calendar.Date) string {
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

// classDayStudents lists the loaded students with the departure modes their
// live plan allows on the weekday: the allowed modes, falling back to the
// exclusive day plan.
func classDayStudents(students []*RosterStudent, weekday string) []enrollment.ClassRosterDayStudent {
	out := make([]enrollment.ClassRosterDayStudent, 0, len(students))
	for _, student := range students {
		if student == nil {
			continue
		}
		day := enrollment.ClassRosterDayStudent{ID: student.ID, DepartureModes: []string{}}
		if weekday != "" {
			allowed := student.AllowedDepartureModes.Normalize()
			if !allowed.HasAny() {
				allowed = departure.AllowedDepartureModesFromDeparture(student.DepartureDays)
			}
			for _, mode := range allowed[weekday] {
				day.DepartureModes = append(day.DepartureModes, string(mode))
			}
		}
		out = append(out, day)
	}
	return out
}
