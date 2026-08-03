package schedule

import (
	"errors"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
)

// WeekdayRosterAssignment is the staff and child roster of ONE weekday of a
// recurring template (issue #2129). Schools whose daily routine is identical
// Monday to Friday but whose responsibilities rotate no longer have to keep
// five near-identical Regeltermine.
//
// It is a deviation from the template's shared default: only the weekdays a
// planner actually differentiated need an entry. Every other weekday of the
// series keeps the shared lists.
type WeekdayRosterAssignment struct {
	Weekday        int
	StudentIDs     []int64
	StaffIDs       []int64
	PrimaryStaffID *int64
}

// resolvedRosterRow is one roster membership ready to persist: a person plus
// the weekday scope the row carries. A nil Weekday is the series-wide default.
type resolvedRosterRow struct {
	PersonID  int64
	Weekday   *int
	IsPrimary bool
}

// resolvedTemplateRoster is the full set of roster rows a template write must
// persist, already expanded from "shared default + per-weekday deviations".
type resolvedTemplateRoster struct {
	Students []resolvedRosterRow
	Staff    []resolvedRosterRow
}

// protectedStudentCoverage records both the presence of an externally-owned
// enrollment and the scheduled weekdays it actually covers. Presence matters
// even when none of the successor's weekdays overlap: a series-wide manual row
// would still collide with the protected row's NULL weekday in the active
// unique index, so it must be expanded onto the uncovered concrete weekdays.
type protectedStudentCoverage struct {
	weekdays map[int]struct{}
}

// ErrWeekdayAssignmentUnscheduled reports a per-weekday roster for a weekday
// the template does not run on. Silently dropping it would lose the planner's
// input; silently adding the weekday would change the recurrence behind their
// back.
var ErrWeekdayAssignmentUnscheduled = errors.New("weekday assignment refers to a weekday the template is not scheduled on")

// ErrWeekdayAssignmentDuplicate reports two roster entries for the same weekday.
var ErrWeekdayAssignmentDuplicate = errors.New("weekday assignment is listed twice")

// resolveTemplateRoster expands a template's shared roster plus its per-weekday
// deviations into the concrete rows to persist.
//
// Two shapes come out of this, and which one applies is decided solely by
// whether any deviation was submitted:
//
//   - No assignments: every row is series-wide (Weekday nil). This is the
//     shape every template had before #2129 and the shape they keep unless a
//     planner opts into per-weekday staffing.
//   - Any assignment: EVERY weekday of the series gets its own rows — an
//     explicit deviation where one was given, a copy of the shared default
//     everywhere else. Readers therefore never have to reason about "does the
//     shared row still apply here?"; the predicate stays
//     `weekday IS NULL OR weekday = ISODOW(date)`.
//
// The cost is a row per (person, weekday) instead of a row per person, which
// for template rosters is a handful of rows either way.
func resolveTemplateRoster(
	weekdays []int,
	sharedStudentIDs, sharedStaffIDs []int64,
	sharedPrimaryStaffID *int64,
	assignments []WeekdayRosterAssignment,
) (resolvedTemplateRoster, error) {
	if len(assignments) == 0 {
		return resolvedTemplateRoster{
			Students: sharedRosterRows(sharedStudentIDs, nil),
			Staff:    sharedRosterRows(sharedStaffIDs, sharedPrimaryStaffID),
		}, nil
	}

	byWeekday, err := indexWeekdayAssignments(weekdays, assignments)
	if err != nil {
		return resolvedTemplateRoster{}, err
	}

	scheduled := uniqueSortedWeekdays(weekdays)

	out := resolvedTemplateRoster{
		Students: make([]resolvedRosterRow, 0, len(scheduled)*len(sharedStudentIDs)),
		Staff:    make([]resolvedRosterRow, 0, len(scheduled)*len(sharedStaffIDs)),
	}
	for _, weekday := range scheduled {
		studentIDs, staffIDs, primaryStaffID := sharedStudentIDs, sharedStaffIDs, sharedPrimaryStaffID
		if assignment, ok := byWeekday[weekday]; ok {
			studentIDs, staffIDs, primaryStaffID = assignment.StudentIDs, assignment.StaffIDs, assignment.PrimaryStaffID
		}
		scope := weekday
		out.Students = append(out.Students, scopedRosterRows(studentIDs, nil, &scope)...)
		out.Staff = append(out.Staff, scopedRosterRows(staffIDs, primaryStaffID, &scope)...)
	}
	return out, nil
}

func indexWeekdayAssignments(
	weekdays []int,
	assignments []WeekdayRosterAssignment,
) (map[int]WeekdayRosterAssignment, error) {
	scheduled := make(map[int]struct{}, len(weekdays))
	for _, weekday := range weekdays {
		scheduled[weekday] = struct{}{}
	}
	byWeekday := make(map[int]WeekdayRosterAssignment, len(assignments))
	for _, assignment := range assignments {
		if !activitiesModel.IsValidWeekday(assignment.Weekday) {
			return nil, fmt.Errorf("invalid weekday %d", assignment.Weekday)
		}
		if _, ok := scheduled[assignment.Weekday]; !ok {
			return nil, fmt.Errorf("%w: %d", ErrWeekdayAssignmentUnscheduled, assignment.Weekday)
		}
		if _, dup := byWeekday[assignment.Weekday]; dup {
			return nil, fmt.Errorf("%w: %d", ErrWeekdayAssignmentDuplicate, assignment.Weekday)
		}
		byWeekday[assignment.Weekday] = assignment
	}
	return byWeekday, nil
}

// uniqueSortedWeekdays deduplicates and orders the template's weekdays so the
// expansion is deterministic regardless of the order the request listed them.
func uniqueSortedWeekdays(weekdays []int) []int {
	seen := make(map[int]struct{}, len(weekdays))
	out := make([]int, 0, len(weekdays))
	for _, weekday := range weekdays {
		if weekday <= 0 {
			continue
		}
		if _, dup := seen[weekday]; dup {
			continue
		}
		seen[weekday] = struct{}{}
		out = append(out, weekday)
	}
	sort.Ints(out)
	return out
}

func sharedRosterRows(personIDs []int64, primaryID *int64) []resolvedRosterRow {
	return scopedRosterRows(personIDs, primaryID, nil)
}

func scopedRosterRows(personIDs []int64, primaryID *int64, weekday *int) []resolvedRosterRow {
	unique := sliceutil.UniquePositive(personIDs)
	rows := make([]resolvedRosterRow, 0, len(unique))
	for _, personID := range unique {
		rows = append(rows, resolvedRosterRow{
			PersonID:  personID,
			Weekday:   weekday,
			IsPrimary: primaryID != nil && *primaryID == personID,
		})
	}
	return rows
}

func buildProtectedStudentCoverage(
	rows []*activitiesModel.StudentEnrollment,
	scheduledWeekdays []int,
	appliesToPeriod func(*activitiesModel.StudentEnrollment) bool,
) map[int64]protectedStudentCoverage {
	coverage := make(map[int64]protectedStudentCoverage)
	weekdays := uniqueSortedWeekdays(scheduledWeekdays)
	for _, row := range rows {
		if row == nil || !appliesToPeriod(row) {
			continue
		}
		studentCoverage, exists := coverage[row.StudentID]
		if !exists {
			studentCoverage = protectedStudentCoverage{weekdays: make(map[int]struct{})}
			coverage[row.StudentID] = studentCoverage
		}
		for _, weekday := range weekdays {
			if protectedEnrollmentAppliesOnWeekday(row, weekday) {
				studentCoverage.weekdays[weekday] = struct{}{}
			}
		}
	}
	return coverage
}

func protectedEnrollmentAppliesOnWeekday(row *activitiesModel.StudentEnrollment, weekday int) bool {
	if row.Weekday != nil && *row.Weekday != weekday {
		return false
	}
	if len(row.SelectedWeekdays) == 0 {
		return true
	}
	for _, selected := range row.SelectedWeekdays {
		if selected == weekday {
			return true
		}
	}
	return false
}

// excludeProtectedStudentWeekdays removes only the manual weekday rows that
// an externally-owned enrollment already supplies. A shared manual row is
// expanded onto uncovered scheduled weekdays so a Monday-only protected row
// cannot erase the same child's manual Tuesday assignment.
func excludeProtectedStudentWeekdays(
	rows []resolvedRosterRow,
	scheduledWeekdays []int,
	coverage map[int64]protectedStudentCoverage,
) []resolvedRosterRow {
	weekdays := uniqueSortedWeekdays(scheduledWeekdays)
	out := make([]resolvedRosterRow, 0, len(rows))
	for _, row := range rows {
		studentCoverage, protected := coverage[row.PersonID]
		if !protected {
			out = append(out, row)
			continue
		}
		if row.Weekday != nil {
			if _, covered := studentCoverage.weekdays[*row.Weekday]; !covered {
				out = append(out, row)
			}
			continue
		}
		for _, weekday := range weekdays {
			if _, covered := studentCoverage.weekdays[weekday]; covered {
				continue
			}
			scope := weekday
			out = append(out, resolvedRosterRow{PersonID: row.PersonID, Weekday: &scope})
		}
	}
	return out
}

// weekdayScopePtr copies a weekday scope so persisted rows never share the
// backing int of the resolver's loop variable.
func weekdayScopePtr(weekday *int) *int {
	if weekday == nil {
		return nil
	}
	cloned := *weekday
	return &cloned
}
