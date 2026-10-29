package studentpresence

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Soll und Ist je Kind (#4774): the booked care time of each child and day
// against its actual attendance. The definitions are binding and named on
// the screen and in the help guide:
//
//   - Soll = the care time on file in moto: arrival to pickup of the weekly
//     plan, day exceptions included (Care Plan resolves it as PlannedCareDay;
//     the source is a port so a contract in the Träger module can replace
//     it, #4683)
//   - Ist = first check-in to last check-out of the day (active.attendance)
//   - overrun = Ist start earlier than Soll start or Ist end later than Soll
//     end by more than CareTimeToleranceMinutes, or attendance on a day
//     without a booking
//   - a booked day without attendance counts as shorter than booked, unless
//     a sick note, an absence, a closing day or a public holiday explains it

// CareTimeToleranceMinutes is the margin before an early arrival or a late
// pickup counts as an overrun.
const CareTimeToleranceMinutes = 15

// ErrCareTimeStudentNotFound is returned when the child is not enrolled in
// the caller's school during the window.
var ErrCareTimeStudentNotFound = errors.New("care time student not found")

// CareTimeDayKind classifies one child-day of the comparison.
type CareTimeDayKind string

const (
	// CareTimeDayWithinPlan — booked and present, inside the tolerance.
	CareTimeDayWithinPlan CareTimeDayKind = "within_plan"
	// CareTimeDayOverrun — booked and present, earlier or later than booked
	// beyond the tolerance.
	CareTimeDayOverrun CareTimeDayKind = "overrun"
	// CareTimeDayUnbooked — present on a day without a booking (an overrun).
	CareTimeDayUnbooked CareTimeDayKind = "unbooked"
	// CareTimeDayAbsent — booked, not present and not explained: the whole
	// booked time counts as shorter than booked.
	CareTimeDayAbsent CareTimeDayKind = "absent"
	// CareTimeDaySick — booked, not present, sick note on file.
	CareTimeDaySick CareTimeDayKind = "sick"
	// CareTimeDayExcused — booked, not present, absence or class trip on file.
	CareTimeDayExcused CareTimeDayKind = "excused"
	// CareTimeDayCancelled — the day was cancelled for this child ("Kommt
	// heute nicht") and the child did not come.
	CareTimeDayCancelled CareTimeDayKind = "cancelled"
	// CareTimeDayClosed — booked weekday that is a public holiday, closing
	// day or holiday period, and the child did not come.
	CareTimeDayClosed CareTimeDayKind = "closed"
	// CareTimeDayOpen — present without a check-out yet; the end is unknown.
	CareTimeDayOpen CareTimeDayKind = "open"
	// CareTimeDayNoPlan — present, but no care plan is on file at all.
	CareTimeDayNoPlan CareTimeDayKind = "no_plan"
)

// IsOverrun reports whether the day is marked as an overrun.
func (k CareTimeDayKind) IsOverrun() bool {
	return k == CareTimeDayOverrun || k == CareTimeDayUnbooked
}

// CareTimeDay is one child-day of the comparison. Clock values are Berlin
// wall clocks; a nil pointer means the value is not known. Minutes are nil
// where the day does not take part in the comparison.
type CareTimeDay struct {
	Date         calendar.Date
	Kind         CareTimeDayKind
	PlannedStart *calendar.WallClock
	PlannedEnd   *calendar.WallClock
	ActualStart  *calendar.WallClock
	ActualEnd    *calendar.WallClock
	// PlannedMinutes and ActualMinutes are the lengths of the two spans.
	PlannedMinutes *int
	ActualMinutes  *int
	// DeviationMinutes is actual minus planned: positive is longer than
	// booked, negative shorter.
	DeviationMinutes *int
	// EarlyMinutes and LateMinutes are how far the attendance reached before
	// the planned start and after the planned end; zero when it did not.
	EarlyMinutes int
	LateMinutes  int
}

// CareTimeSummary totals the comparison of one child over the window.
type CareTimeSummary struct {
	OverrunDays  int
	UnbookedDays int
	AbsentDays   int
	// ExtraMinutes is the time beyond the booking: early plus late minutes
	// of the overrun days and the attendance of the unbooked days.
	ExtraMinutes     int
	PlannedMinutes   int
	ActualMinutes    int
	DeviationMinutes int
}

// StudentCareTimes is the Soll-und-Ist comparison of one child.
type StudentCareTimes struct {
	StudentID int64
	From      calendar.Date
	To        calendar.Date
	// DataFrom is the first compared day: the attendance and status-day
	// retention may cut the window short, older days are not listed.
	DataFrom         calendar.Date
	ToleranceMinutes int
	Days             []CareTimeDay
	Summary          CareTimeSummary
}

// CareTimeFilters selects the window and the optional group and class
// restriction of the overrun list; group 0 means children without a group.
type CareTimeFilters struct {
	From     calendar.Date
	To       calendar.Date
	GroupIDs []int64
	// SchoolClasses restricts the children to these classes; empty means all.
	SchoolClasses []string
}

// CareTimeOverrunRow is one child with at least one overrun in the window.
type CareTimeOverrunRow struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
	GroupName   string
	Summary     CareTimeSummary
}

// CareTimeOverruns is the overrun list for the school management, sorted by
// the number of overrun days, most first.
type CareTimeOverruns struct {
	From             calendar.Date
	To               calendar.Date
	DataFrom         calendar.Date
	ToleranceMinutes int
	Students         []CareTimeOverrunRow
}

// OverrunCount is the number of days marked as an overrun.
func (s CareTimeSummary) OverrunCount() int { return s.OverrunDays + s.UnbookedDays }
