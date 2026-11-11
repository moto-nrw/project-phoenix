package application

import (
	"slices"
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

// germanNamesMonth counts a month whose names sort differently in German
// dictionary order than in the byte order of the lowercased names: an
// umlaut in a last name and in a first name, and two children with the
// same name.
func germanNamesMonth() mealplan.BillingMonth {
	participants := []mealplan.DailyParticipant{
		{StudentID: 1, FirstName: "Anna", LastName: "Zimmermann", SchoolClass: "2a"},
		{StudentID: 2, FirstName: "Ayşe", LastName: "Özdemir", SchoolClass: "3b"},
		{StudentID: 3, FirstName: "Oskar", LastName: "Ernst", SchoolClass: "1a"},
		{StudentID: 4, FirstName: "Ömer", LastName: "Ernst", SchoolClass: "1a"},
		{StudentID: 9, FirstName: "Lena", LastName: "Müller", SchoolClass: "4c"},
		{StudentID: 5, FirstName: "Lena", LastName: "Müller", SchoolClass: "2b"},
	}
	days := []mealplan.DailyList{
		{Date: "2026-03-02", Participants: participants},
		{Date: "2026-03-03", Participants: participants[1:]},
	}
	present := map[calendar.Date]map[int64]bool{"2026-03-02": {1: true, 2: true, 3: true, 5: true}, "2026-03-03": {4: true, 9: true}}
	return countBillingMonth(calendar.NewDate(2026, 3, 1), days, nil, present)
}

func childIDs(children []mealplan.BillingMonthChild) []int64 {
	ids := make([]int64, len(children))
	for i, child := range children {
		ids[i] = child.StudentID
	}
	return ids
}

// TestBillingMonthShowsChildrenInGermanOrder pins the shown order (#6464):
// last name, first name in German dictionary order, then the child's ID.
func TestBillingMonthShowsChildrenInGermanOrder(t *testing.T) {
	t.Parallel()
	month := germanNamesMonth()

	assert.Equal(t, []int64{4, 3, 5, 9, 2, 1}, childIDs(month.Children),
		"Ernst Ömer, Ernst Oskar, Müller Lena (5, 9), Özdemir, Zimmermann")

	stored := storedMonth(calendar.NewDate(2026, 3, 1), month)
	slices.Reverse(stored.Children)
	assert.Equal(t, []int64{4, 3, 5, 9, 2, 1}, childIDs(releasedView(stored).Children), "a released month shows the German order too")
}

// TestMonthFingerprintKeepsItsOrder pins the fingerprint of a month to the
// value the code before the German sort computed for it (#6464): a preview
// opened before still releases, and any change of a child still changes it.
func TestMonthFingerprintKeepsItsOrder(t *testing.T) {
	t.Parallel()
	const beforeGermanSort = "2e47adc82a9874f70f3332aaac06b0d8"
	month := germanNamesMonth()
	assert.Equal(t, beforeGermanSort, monthFingerprint(month))

	reversed := month
	reversed.Children = slices.Clone(month.Children)
	slices.Reverse(reversed.Children)
	assert.Equal(t, beforeGermanSort, monthFingerprint(reversed), "the shown order does not change the fingerprint")

	for name, change := range map[string]func(*mealplan.BillingMonth){
		"a meal less": func(m *mealplan.BillingMonth) { m.Children[0].MealDays-- },
		"a meal moved between the two Lenas": func(m *mealplan.BillingMonth) {
			m.Children[2].MealDays--
			m.Children[3].MealDays++
		},
		"another class": func(m *mealplan.BillingMonth) { m.Children[4].SchoolClass = "3c" },
		"a child less":  func(m *mealplan.BillingMonth) { m.Children = m.Children[1:] },
	} {
		changed := month
		changed.Children = slices.Clone(month.Children)
		change(&changed)
		assert.NotEqual(t, beforeGermanSort, monthFingerprint(changed), name)
	}
}
