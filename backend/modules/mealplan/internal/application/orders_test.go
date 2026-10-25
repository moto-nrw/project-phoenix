package application

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenDaysSkipsPastAndOrderedDays(t *testing.T) {
	t.Parallel()
	rules := defaultOrderingRules("09:00")
	monday, wednesday := calendar.NewDate(2026, 9, 14), calendar.NewDate(2026, 9, 16)
	state := domain.OrderState{Orders: []domain.Order{{ID: 7, Days: []domain.Date{calendar.NewDate(2026, 9, 17)}}}}

	open, deadline := openDays(rules, state, monday, monday.AddDays(6), wednesday)
	assert.Equal(t, []domain.Date{wednesday, calendar.NewDate(2026, 9, 18)}, open, "Monday and Tuesday are over, Thursday is ordered, the weekend has no meal")
	assert.Equal(t, rules.FreezeAt(wednesday), deadline, "the earliest deadline among the open days")

	none, zero := openDays(rules, state, monday, monday.AddDays(1), wednesday)
	assert.Empty(t, none)
	assert.True(t, zero.IsZero())
}

func TestNextPeriodMovesByDayOrWeek(t *testing.T) {
	t.Parallel()
	monday, friday := calendar.NewDate(2026, 9, 14), calendar.NewDate(2026, 9, 18)
	assert.Equal(t, calendar.NewDate(2026, 9, 15), nextPeriod(domain.RhythmDaily, monday, monday))
	assert.Equal(t, calendar.NewDate(2026, 9, 21), nextPeriod(domain.RhythmWeekly, monday, friday), "the next Monday, never Saturday")
}

func TestOpenOrderViewUsesRemainingPeriodDeadline(t *testing.T) {
	t.Parallel()
	monday := calendar.NewDate(2026, 9, 14)
	friday := monday.AddDays(4)
	for _, tc := range []struct {
		name   string
		rhythm domain.Rhythm
		today  domain.Date
		want   domain.Date
	}{
		{"daily", domain.RhythmDaily, monday, friday},
		{"weekly before Monday", domain.RhythmWeekly, monday.AddDays(-1), monday},
		{"weekly from Wednesday", domain.RhythmWeekly, monday.AddDays(2), monday.AddDays(2)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rules := defaultOrderingRules("09:00")
			rules.Rhythm = tc.rhythm
			// Even a Friday-only view must use the whole remaining period.
			view := buildOrderView(rules, orderWindow{}, nil, friday, friday, tc.today)
			require.Len(t, view.Days, 1)
			assert.Equal(t, rules.FreezeAt(tc.want), view.Days[0].Deadline)
		})
	}

	rules := defaultOrderingRules("09:00")
	rules.Rhythm = domain.RhythmWeekly
	state := domain.OrderState{Orders: []domain.Order{{Days: []domain.Date{monday}, DeadlineAt: rules.FreezeAt(monday)}}}
	view := buildOrderView(rules, orderWindow{state: state}, nil, monday, friday, monday)
	require.Len(t, view.Days, 5)
	assert.Equal(t, rules.FreezeAt(monday), view.Days[0].Deadline, "stored orders keep their deadline")
	for _, day := range view.Days[1:] {
		assert.Equal(t, rules.FreezeAt(monday.AddDays(1)), day.Deadline, "remaining days share the next open deadline")
	}
}

func TestRecordChangesNamesOnlyTheWrittenChild(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 9, 15)
	yesterday, tomorrow := today.AddDays(-1), today.AddDays(1)
	state := domain.OrderState{
		Orders: []domain.Order{{ID: 3, Days: []domain.Date{yesterday, today}}},
		Lines:  []domain.OrderLine{{Date: yesterday, StudentID: 1}, {Date: today, StudentID: 1}, {Date: today, StudentID: 2}},
	}
	current := map[domain.Date]dayChildren{
		yesterday: {classes: map[int64]string{}},
		today:     {classes: map[int64]string{3: "1a"}},
		tomorrow:  {classes: map[int64]string{1: "2a"}},
	}

	author := &changeAuthor{studentID: 2, accountID: 40}
	changes := recordChanges(state, current, today, author)
	assert.Len(t, changes, 3, "yesterday keeps its order and tomorrow has none yet")
	byStudent := map[int64]domain.OrderChange{}
	for _, change := range changes {
		assert.Equal(t, today, change.Date)
		assert.Equal(t, state.Orders[0].ID, change.OrderID)
		byStudent[change.StudentID] = change
	}
	assert.Nil(t, byStudent[1].ChangedBy, "another child's change names nobody")
	assert.Nil(t, byStudent[3].ChangedBy)
	if assert.NotNil(t, byStudent[2].ChangedBy) {
		assert.Equal(t, author.accountID, *byStudent[2].ChangedBy)
	}
	assert.Equal(t, -1, byStudent[2].Delta)
	assert.Equal(t, 1, byStudent[3].Delta)
	assert.Empty(t, recordChanges(state, current, today.AddDays(2), nil), "nothing from a day already over")
}

// defaultOrderingRules are the rules of a school that changed nothing: one
// deadline on the meal day at the given clock time, as before #3873.
func defaultOrderingRules(clock string) domain.OrderingRules {
	deadline := domain.Deadline{Kind: domain.DeadlineDaysBefore, Clock: clock}
	return domain.OrderingRules{
		Model: domain.ModelSingle, Registration: deadline, Cancellation: deadline,
		AnnualChanges: domain.AnnualParentsCancel, Sick: domain.SickStaysOrdered, Rhythm: domain.RhythmDaily, Detail: domain.DetailByClass,
	}
}

// Counts in the delivery list describe the frozen order; late changes appear
// only in their own column, while the named children still show who eats now.
func TestDeliveryLinesKeepFrozenCountsAfterLateChanges(t *testing.T) {
	t.Parallel()
	date := calendar.NewDate(2026, 9, 14)
	for _, tc := range []struct {
		name    string
		lines   int
		changes []domain.OrderChange
		eating  int
		total   []string
		classes [][]string
	}{
		{name: "addition", lines: 10, changes: []domain.OrderChange{{StudentID: 11, SchoolClass: "3c", Delta: 1}}, eating: 11,
			total: []string{"Alle", "10", "+1"}, classes: [][]string{{"1b", "1", ""}, {"2a", "9", ""}, {"3c", "0", "+1"}}},
		{name: "cancellation", lines: 10, changes: []domain.OrderChange{{StudentID: 10, SchoolClass: "1b", Delta: -1}}, eating: 9,
			total: []string{"Alle", "10", "−1"}, classes: [][]string{{"1b", "1", "−1"}, {"2a", "9", ""}}},
		{name: "class replacement", lines: 10, changes: []domain.OrderChange{{StudentID: 10, SchoolClass: "1b", Delta: -1}, {StudentID: 11, SchoolClass: "3c", Delta: 1}}, eating: 10,
			total: []string{"Alle", "10", ""}, classes: [][]string{{"1b", "1", "−1"}, {"2a", "9", ""}, {"3c", "0", "+1"}}},
		{name: "empty frozen order", changes: []domain.OrderChange{{StudentID: 11, SchoolClass: "3c", Delta: 1}}, eating: 1,
			total: []string{"Alle", "0", "+1"}, classes: [][]string{{"3c", "0", "+1"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			state := domain.OrderState{Orders: []domain.Order{{ID: 7, Days: []domain.Date{date}}}}
			for id := 1; id <= tc.lines; id++ {
				schoolClass := "2a"
				if id == 10 {
					schoolClass = "1b"
				}
				state.Lines = append(state.Lines, domain.OrderLine{Date: date, StudentID: int64(id), SchoolClass: schoolClass})
			}
			// A weekly snapshot can also contain lines for another day.
			state.Lines = append(state.Lines, domain.OrderLine{Date: date.AddDays(1), StudentID: 20, SchoolClass: "4a"})
			for index, change := range tc.changes {
				change.Date, change.Sequence = date, index+1
				state.Changes = append(state.Changes, change)
			}
			view := buildOrderView(defaultOrderingRules("09:00"), orderWindow{state: state}, nil, date, date, date)
			require.Len(t, view.Days, 1)
			view.Days[0].Name(nil)
			week := publicOrderWeek(view)
			require.Len(t, week.Days, 1)
			assert.Len(t, week.Days[0].Children, tc.eating)
			assert.Len(t, week.Days[0].Changes, len(tc.changes))
			week.Detail = "total"
			lines, err := week.DeliveryLines("School", "Address")
			require.NoError(t, err)
			require.Len(t, lines, 1)
			assert.Equal(t, tc.total, lines[0][3:])
			week.Detail = "by_class"
			lines, err = week.DeliveryLines("School", "Address")
			require.NoError(t, err)
			var counts [][]string
			for _, line := range lines {
				counts = append(counts, line[3:])
			}
			assert.Equal(t, tc.classes, counts)
		})
	}
}

func TestReadOrderWindowCountsInclusiveCalendarDates(t *testing.T) {
	t.Parallel()
	start := calendar.NewDate(2026, 10, 1) // Includes the autumn clock change.
	for _, tc := range []struct {
		name     string
		from, to mealplan.Date
		valid    bool
	}{
		{"one day", mealplan.Date(start), mealplan.Date(start), true},
		{"six weeks", mealplan.Date(start), mealplan.Date(start.AddDays(41)), true},
		{"six weeks plus one day", mealplan.Date(start), mealplan.Date(start.AddDays(42)), false},
		{"reversed", mealplan.Date(start), mealplan.Date(start.AddDays(-1)), false},
		{"invalid date", "invalid", mealplan.Date(start), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			from, to, err := readWindow(tc.from, tc.to)
			if !tc.valid {
				require.ErrorIs(t, err, mealplan.ErrInvalidParticipation)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, string(tc.from), from.String())
			assert.Equal(t, string(tc.to), to.String())
		})
	}
}
