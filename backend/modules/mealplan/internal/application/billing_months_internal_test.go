package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The count of the Mensa-Monatsfreigabe (#4686), by hand for March 2026.
// The kitchen lists are what the registration gives after every
// cancellation; the attendance is what Student Presence has on file.
//
//	Mo 02  Di 03  Mi 04  Do 05  Fr 06 | Mo 09 closing day | Mo 30, Di 31 Easter holidays
//	Mia:  registered 02–05, cancelled by her parents on 06, absent on 04,
//	      present 02, 03, 05 and on 06 (present but cancelled: no meal)
//	Ben:  enters on 05, registered and present 05 and 06
//	Lea:  registered on the closing day 09 and in the holidays only
//
// School days: 02–06 = 5 (09 closed, 30 and 31 holidays, 27 Feb outside).
// Mia: registered 4, meals 3 (02, 03, 05), absent on 04.
// Ben: registered 2, meals 2. Lea: no registered school day, not listed.
func TestCountBillingMonthByHand(t *testing.T) {
	t.Parallel()
	mia := mealplan.DailyParticipant{StudentID: 1, FirstName: "Mia", LastName: "Albers", SchoolClass: "2a"}
	ben := mealplan.DailyParticipant{StudentID: 2, FirstName: "Ben", LastName: "Brandt", SchoolClass: "1b"}
	lea := mealplan.DailyParticipant{StudentID: 3, FirstName: "Lea", LastName: "Cramer", SchoolClass: "3c"}
	list := func(date string, closed bool, participants ...mealplan.DailyParticipant) mealplan.DailyList {
		return mealplan.DailyList{Date: mealplan.Date(date), Closed: closed, Participants: participants}
	}
	days := []mealplan.DailyList{
		list("2026-02-27", false, mia),
		list("2026-03-02", false, mia), list("2026-03-03", false, mia), list("2026-03-04", false, mia),
		list("2026-03-05", false, mia, ben), list("2026-03-06", false, ben),
		list("2026-03-09", true, mia, lea),
		list("2026-03-30", false, lea), list("2026-03-31", false, lea),
	}
	holidays := []mealplan.HolidayPeriod{{Start: "2026-03-30", End: "2026-04-10"}}
	present := map[calendar.Date]map[int64]bool{
		"2026-02-27": {1: true},
		"2026-03-02": {1: true}, "2026-03-03": {1: true}, "2026-03-05": {1: true, 2: true},
		"2026-03-06": {1: true, 2: true}, "2026-03-09": {1: true}, "2026-03-30": {3: true},
	}

	month := countBillingMonth(calendar.NewDate(2026, 3, 1), days, holidays, present)

	assert.Equal(t, 5, month.SchoolDays)
	assert.Equal(t, []mealplan.BillingMonthChild{
		{StudentID: 1, FirstName: "Mia", LastName: "Albers", SchoolClass: "2a", RegisteredDays: 4, MealDays: 3, AbsentDates: []mealplan.Date{"2026-03-04"}},
		{StudentID: 2, FirstName: "Ben", LastName: "Brandt", SchoolClass: "1b", RegisteredDays: 2, MealDays: 2, AbsentDates: []mealplan.Date{}},
	}, month.Children)
	assert.Equal(t, 2, month.ChildCount)
	assert.Equal(t, 5, month.MealCount)
	assert.Equal(t, 1, month.AbsenceCount)
}

// TestBillingMonthWindow pins which months the OGS-Leitung sees: the
// current one and the eleven before, a month on its first day.
func TestBillingMonthWindow(t *testing.T) {
	t.Parallel()
	months := &BillingMonths{today: func() calendar.Date { return calendar.NewDate(2026, 4, 2) }}
	for value, ok := range map[string]bool{
		"2026-04-01": true, "2025-05-01": true, "2025-04-01": false, "2026-05-01": false, "2026-03-02": false, "kein": false,
	} {
		_, err := months.parseBillingMonth(mealplan.Date(value))
		assert.Equal(t, ok, err == nil, value)
	}
	assert.Equal(t, mealplan.BillingMonthRunning, months.statusOf(calendar.NewDate(2026, 4, 1)), "April runs until its last day")
	assert.Equal(t, mealplan.BillingMonthOpen, months.statusOf(calendar.NewDate(2026, 3, 1)), "March has ended")
}
