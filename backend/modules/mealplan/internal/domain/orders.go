package domain

import (
	"sort"
	"time"
)

// OrderFormatVersion is the version of the order as other systems read it
// (#3873). It rises when the meaning of a field changes.
const OrderFormatVersion = 1

// Order is a school's order of a day or a week, fixed at its deadline. A
// due order a read lays out before the worker or a write stored it has no
// ID yet.
type Order struct {
	ID          int64
	PeriodStart Date
	PeriodEnd   Date
	DeadlineAt  time.Time
	FrozenAt    time.Time
	ConfirmedAt *time.Time
	Days        []Date
}

// OrderLine is one child who eats on one day of an order.
type OrderLine struct {
	Date        Date
	StudentID   int64
	SchoolClass string
}

// OrderChange is a change after the deadline: one child more (+1) or less
// (-1) on a day. Sequence numbers the changes of a school; a change a read
// detected but nobody recorded yet has none (0) and follows every numbered
// one.
type OrderChange struct {
	OrderID     int64
	Sequence    int
	Date        Date
	StudentID   int64
	SchoolClass string
	Delta       int
	DetectedAt  time.Time
	ChangedBy   *int64
}

// NewOrder is an order to fix: its period, its deadline, its days (a closed
// day is a day without lines) and the children of each day.
type NewOrder struct {
	PeriodStart Date
	PeriodEnd   Date
	DeadlineAt  time.Time
	Days        []Date
	Lines       []OrderLine
}

// OrderState is what the orders of a window hold: the fixed lines and the
// changes after them.
type OrderState struct {
	Orders  []Order
	Lines   []OrderLine
	Changes []OrderChange
}

// FrozenChildren is the original order of a day, before late changes.
func (s OrderState) FrozenChildren(date Date) map[int64]string {
	children := make(map[int64]string)
	for _, line := range s.Lines {
		if line.Date == date {
			children[line.StudentID] = line.SchoolClass
		}
	}
	return children
}

// OrderedChildren is the net state of a day: the fixed lines with every
// change applied, by student.
func (s OrderState) OrderedChildren(date Date) map[int64]string {
	children := s.FrozenChildren(date)
	changes := make([]OrderChange, 0)
	for _, change := range s.Changes {
		if change.Date == date {
			changes = append(changes, change)
		}
	}
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].before(changes[j]) })
	for _, change := range changes {
		if change.Delta > 0 {
			children[change.StudentID] = change.SchoolClass
		} else {
			delete(children, change.StudentID)
		}
	}
	return children
}

// before orders changes by sequence; an unnumbered change comes last.
func (c OrderChange) before(other OrderChange) bool {
	switch {
	case c.Sequence == 0:
		return false
	case other.Sequence == 0:
		return true
	}
	return c.Sequence < other.Sequence
}

// OrderFor returns the order a day belongs to.
func (s OrderState) OrderFor(date Date) (Order, bool) {
	for _, order := range s.Orders {
		for _, day := range order.Days {
			if day == date {
				return order, true
			}
		}
	}
	return Order{}, false
}

// DiffOrder lists the changes that bring the ordered children of a day to
// the current ones: who eats now but is not ordered (+1), who is ordered but
// no longer eats (-1). Classes come from the current day, the ordered class
// for a child that left. The result is ordered by student for stable
// numbering.
func DiffOrder(date Date, ordered map[int64]string, current map[int64]string) []OrderChange {
	changes := make([]OrderChange, 0)
	for studentID, schoolClass := range current {
		if _, ok := ordered[studentID]; !ok {
			changes = append(changes, OrderChange{Date: date, StudentID: studentID, SchoolClass: schoolClass, Delta: 1})
		}
	}
	for studentID, schoolClass := range ordered {
		if _, ok := current[studentID]; !ok {
			changes = append(changes, OrderChange{Date: date, StudentID: studentID, SchoolClass: schoolClass, Delta: -1})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].StudentID != changes[j].StudentID {
			return changes[i].StudentID < changes[j].StudentID
		}
		return changes[i].Delta < changes[j].Delta
	})
	return changes
}

// OrderPeriod is the period a day is ordered in under the rhythm: the day
// itself, or the weekdays of its ISO week.
func OrderPeriod(rhythm Rhythm, day Date) (Date, Date) {
	if rhythm == RhythmWeekly {
		monday := day.StartOfISOWeek()
		return monday, monday.AddDays(4)
	}
	return day, day
}

// OrderDay is one weekday of the order view: open (no order yet, the
// children are the current registration), ordered (the fixed order with
// every change applied) or unordered (over without an order, as before the
// school started ordering).
type OrderDay struct {
	Date      Date
	Closed    bool
	Unordered bool
	Deadline  time.Time
	Order     *Order
	Children  map[int64]string
	// FrozenChildren holds the fixed lines for counts, without late changes.
	FrozenChildren map[int64]string
	Changes        []OrderChange
	// Eating are the children of the day by name, sorted like the kitchen
	// list.
	Eating []DailyParticipant
}

// Name sorts the day's children into Eating with their names. A child the
// directory no longer knows keeps its class and ID.
func (d *OrderDay) Name(names map[int64]DailyCandidate) {
	d.Eating = make([]DailyParticipant, 0, len(d.Children))
	for studentID, schoolClass := range d.Children {
		name := names[studentID]
		d.Eating = append(d.Eating, DailyParticipant{StudentID: studentID, FirstName: name.FirstName, LastName: name.LastName, SchoolClass: schoolClass})
	}
	SortDailyParticipants(d.Eating)
}

// OrderView is the school's orders of a window, with the rules they follow
// and the names of every child in them.
type OrderView struct {
	Rules OrderingRules
	Days  []OrderDay
	Names map[int64]DailyCandidate
}

// ClassCount is the number of meals of one class.
type ClassCount struct {
	SchoolClass string
	Count       int
}

// CountByClass counts children per class, sorted by class.
func CountByClass(children map[int64]string) []ClassCount {
	counts := make(map[string]int)
	for _, schoolClass := range children {
		counts[schoolClass]++
	}
	result := make([]ClassCount, 0, len(counts))
	for schoolClass, count := range counts {
		result = append(result, ClassCount{SchoolClass: schoolClass, Count: count})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SchoolClass < result[j].SchoolClass })
	return result
}
