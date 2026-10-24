package domain

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

func TestDiffOrderListsWhoCameAndWhoLeft(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2026, 9, 14)
	ordered := map[int64]string{1: "2a", 2: "2a", 3: "3b"}
	current := map[int64]string{2: "2a", 3: "4c", 4: "1a"}

	changes := DiffOrder(day, ordered, current)
	assert.Equal(t, []OrderChange{
		{Date: day, StudentID: 1, SchoolClass: "2a", Delta: -1},
		{Date: day, StudentID: 4, SchoolClass: "1a", Delta: 1},
	}, changes, "a class change alone is no change of the order")
	assert.Empty(t, DiffOrder(day, ordered, ordered))
}

func TestOrderedChildrenAppliesChangesInSequence(t *testing.T) {
	t.Parallel()
	monday, tuesday := calendar.NewDate(2026, 9, 14), calendar.NewDate(2026, 9, 15)
	state := OrderState{
		Lines: []OrderLine{{Date: monday, StudentID: 1, SchoolClass: "2a"}, {Date: tuesday, StudentID: 9, SchoolClass: "4a"}},
		Changes: []OrderChange{
			{Sequence: 3, Date: monday, StudentID: 2, SchoolClass: "2b", Delta: -1},
			{Sequence: 1, Date: monday, StudentID: 2, SchoolClass: "2b", Delta: 1},
			{Sequence: 2, Date: monday, StudentID: 1, SchoolClass: "2a", Delta: -1},
		},
	}
	assert.Equal(t, map[int64]string{}, state.OrderedChildren(monday), "added, then removed again")
	assert.Equal(t, map[int64]string{9: "4a"}, state.OrderedChildren(tuesday))
}

// A change a read detected but nobody recorded yet has no sequence and
// applies after every recorded one.
func TestOrderedChildrenAppliesUnnumberedChangesLast(t *testing.T) {
	t.Parallel()
	monday := calendar.NewDate(2026, 9, 14)
	state := OrderState{
		Lines: []OrderLine{{Date: monday, StudentID: 1, SchoolClass: "2a"}},
		Changes: []OrderChange{
			{Date: monday, StudentID: 1, SchoolClass: "2a", Delta: 1},
			{Sequence: 4, Date: monday, StudentID: 1, SchoolClass: "2a", Delta: -1},
		},
	}
	assert.Equal(t, map[int64]string{1: "2a"}, state.OrderedChildren(monday), "removed, then back again")
}

func TestOrderPeriodIsTheDayOrItsWeek(t *testing.T) {
	t.Parallel()
	wednesday := calendar.NewDate(2026, 9, 16)
	start, end := OrderPeriod(RhythmDaily, wednesday)
	assert.Equal(t, wednesday, start)
	assert.Equal(t, wednesday, end)
	start, end = OrderPeriod(RhythmWeekly, wednesday)
	assert.Equal(t, calendar.NewDate(2026, 9, 14), start)
	assert.Equal(t, calendar.NewDate(2026, 9, 18), end)
}

// A deleted child's meals stay in the order with their class (#3873): each
// counts on its own, a deleted child's change applies to a meal of its
// class, and no diff ever cancels them.
func TestDeletedChildrenKeepTheirMeals(t *testing.T) {
	t.Parallel()
	monday := calendar.NewDate(2026, 9, 14)
	state := OrderState{
		Lines: []OrderLine{
			{Date: monday, StudentID: DeletedChild, SchoolClass: "2a"},
			{Date: monday, StudentID: DeletedChild, SchoolClass: "2a"},
			{Date: monday, StudentID: DeletedChild, SchoolClass: "3b"},
			{Date: monday, StudentID: 7, SchoolClass: "2a"},
		},
		Changes: []OrderChange{{Sequence: 1, Date: monday, StudentID: DeletedChild, SchoolClass: "3b", Delta: -1}},
	}
	assert.Equal(t, []ClassCount{{SchoolClass: "2a", Count: 3}, {SchoolClass: "3b", Count: 1}}, CountByClass(state.FrozenChildren(monday)))
	ordered := state.OrderedChildren(monday)
	assert.Equal(t, []ClassCount{{SchoolClass: "2a", Count: 3}}, CountByClass(ordered))
	assert.Empty(t, DiffOrder(monday, ordered, map[int64]string{7: "2a"}), "nobody can cancel a deleted child's meal")

	day := OrderDay{Children: ordered}
	day.Name(map[int64]DailyCandidate{7: {StudentID: 7, FirstName: "Ida", LastName: "Da", SchoolClass: "2a"}})
	ids := make([]int64, 0, len(day.Eating))
	for _, eating := range day.Eating {
		ids = append(ids, eating.StudentID)
	}
	assert.ElementsMatch(t, []int64{DeletedChild, DeletedChild, 7}, ids, "a deleted child names nobody")
}
