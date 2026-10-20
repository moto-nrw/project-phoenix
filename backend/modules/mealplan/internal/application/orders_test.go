package application

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
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
