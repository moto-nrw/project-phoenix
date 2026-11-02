package application

import (
	"maps"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// keptDecisions carries the school's „Bleibt so“ (#5102) from the hand rows
// a rewrite retires onto the rows that replace them: saving a series,
// editing its roster from a date and splitting it rewrite the rows, not the
// school's decision. Per student and ISO weekday a decision survives only
// when every retired hand row of the student that plans the weekday from the
// rewrite on carried one; the earliest date wins. A rewrite therefore never
// hides more than before.
type keptDecisions map[int64]map[int]activityContract.Date

// collectKeptDecisions reads the decisions of the student's hand rows that
// still reach from.
func collectKeptDecisions(rows []*activityContract.StudentEnrollment, from calendar.Date) keptDecisions {
	reaching := reachingHandRows(rows, from)
	result := keptDecisions{}
	for _, row := range reaching {
		result.addEarliest(row)
	}
	for _, row := range reaching {
		result.vetoUndecided(row)
	}
	return result
}

func reachingHandRows(rows []*activityContract.StudentEnrollment, from calendar.Date) []*activityContract.StudentEnrollment {
	reaching := make([]*activityContract.StudentEnrollment, 0, len(rows))
	for _, row := range rows {
		if row != nil && row.EnrollmentRequestChildID == nil &&
			(row.ValidUntil == nil || calendar.Date(*row.ValidUntil).After(from)) {
			reaching = append(reaching, row)
		}
	}
	return reaching
}

// addEarliest keeps the earliest decision per weekday.
func (k keptDecisions) addEarliest(row *activityContract.StudentEnrollment) {
	for weekday, date := range row.KeptWithoutBooking {
		if k[row.StudentID] == nil {
			k[row.StudentID] = map[int]activityContract.Date{}
		}
		if current, found := k[row.StudentID][weekday]; !found || date < current {
			k[row.StudentID][weekday] = date
		}
	}
}

// vetoUndecided drops a weekday the row plans without a decision.
func (k keptDecisions) vetoUndecided(row *activityContract.StudentEnrollment) {
	for weekday := range k[row.StudentID] {
		if _, decided := row.KeptWithoutBooking[weekday]; !decided && rowPlansWeekday(row.Weekday, row.SelectedWeekdays, weekday) {
			delete(k[row.StudentID], weekday)
		}
	}
}

// forRow is the decision a replacing row of the student takes: those of the
// weekdays it plans.
func (k keptDecisions) forRow(studentID int64, weekday *int, selected []int) map[int]activityContract.Date {
	var result map[int]activityContract.Date
	for decidedWeekday, date := range k[studentID] {
		if !rowPlansWeekday(weekday, selected, decidedWeekday) {
			continue
		}
		if result == nil {
			result = map[int]activityContract.Date{}
		}
		result[decidedWeekday] = date
	}
	return result
}

// rowPlansWeekday reports whether a row with this weekday scope and these
// selected weekdays plans the ISO weekday; an unscoped row plans every one.
func rowPlansWeekday(weekday *int, selected []int, isoWeekday int) bool {
	if weekday != nil {
		return *weekday == isoWeekday
	}
	if len(selected) == 0 {
		return true
	}
	for _, day := range selected {
		if day == isoWeekday {
			return true
		}
	}
	return false
}

// cloneKept copies a row's decision for a row that continues it unchanged.
func cloneKept(kept map[int]activityContract.Date) map[int]activityContract.Date {
	if len(kept) == 0 {
		return nil
	}
	return maps.Clone(kept)
}
