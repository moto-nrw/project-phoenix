package mealplan_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDailyListExportLaysOutTheTagesliste(t *testing.T) {
	t.Parallel()
	list := mealplan.DailyList{Date: "2026-10-19", Cutoffs: mealplan.Cutoffs{CutoffTime: "09:30"}, Participants: []mealplan.DailyParticipant{
		{StudentID: 11, FirstName: "Mia", LastName: "Meier", SchoolClass: "2a"},
		{StudentID: 12, FirstName: "Ben", LastName: "Bauer", SchoolClass: "3b"},
	}}

	export, err := list.Export()
	require.NoError(t, err)
	assert.Equal(t, mealplan.DailyListExport{
		Title:    "Mittagessen – Tagesliste",
		Subtitle: "Tagesliste für den 19.10.2026",
		Filters:  []string{"Änderungsfrist: 09:30 Uhr", "2 Kinder"},
		Columns:  []mealplan.DailyListExportColumn{{ID: "student_name", Label: "Kind"}, {ID: "student_class", Label: "Klasse"}},
		Lines:    [][]string{{"Meier, Mia", "2a"}, {"Bauer, Ben", "3b"}},
		Footer:   "Vertraulich behandeln und nach dem Küchendienst sicher entsorgen.",
		Filename: "mittagessen-2026-10-19",
	}, export)

	_, err = mealplan.DailyList{Date: "19.10.2026"}.Export()
	require.Error(t, err)
}

func TestWeeklyListExportLaysOutTheWochenliste(t *testing.T) {
	t.Parallel()
	mia := mealplan.DailyParticipant{StudentID: 11, FirstName: "Mia", LastName: "Meier", SchoolClass: "2a"}
	ben := mealplan.DailyParticipant{StudentID: 12, FirstName: "Ben", LastName: "Bauer", SchoolClass: "3b"}
	day := func(date mealplan.Date, participants ...mealplan.DailyParticipant) mealplan.DailyList {
		return mealplan.DailyList{Date: date, Cutoffs: mealplan.Cutoffs{CutoffTime: "09:30", CutoffLeadDays: 2}, Participants: participants}
	}
	week := mealplan.WeeklyList{WeekStart: "2026-10-19", Cutoffs: mealplan.Cutoffs{CutoffTime: "09:30", CutoffLeadDays: 2}, Days: []mealplan.DailyList{
		day("2026-10-19", ben, mia), day("2026-10-20"), day("2026-10-21", ben), day("2026-10-22", mia), day("2026-10-23", ben, mia),
	}}

	export, err := week.Export()
	require.NoError(t, err)
	assert.Equal(t, mealplan.DailyListExport{
		Title:    "Mittagessen – Wochenliste",
		Subtitle: "Wochenliste vom 19.10.2026 bis 23.10.2026",
		Filters:  []string{"Änderungsfrist: 09:30 Uhr, 2 Werktage vorher", "2 Kinder"},
		Columns: []mealplan.DailyListExportColumn{
			{ID: "student_name", Label: "Kind"}, {ID: "student_class", Label: "Klasse"},
			{ID: "day_1", Label: "Mo 19.10."}, {ID: "day_2", Label: "Di 20.10."}, {ID: "day_3", Label: "Mi 21.10."}, {ID: "day_4", Label: "Do 22.10."}, {ID: "day_5", Label: "Fr 23.10."},
		},
		Lines: [][]string{
			{"Meier, Mia", "2a", "✓", "", "", "✓", "✓"},
			{"Bauer, Ben", "3b", "✓", "", "✓", "", "✓"},
			{"Summe", "", "2", "0", "1", "1", "2"},
		},
		Footer:   "Vertraulich behandeln und nach dem Küchendienst sicher entsorgen.",
		Filename: "mittagessen-woche-2026-10-19",
	}, export)

	_, err = mealplan.WeeklyList{WeekStart: "2026-10-19"}.Export()
	require.Error(t, err)
}

func TestDailyListExportNamesOneLeadDay(t *testing.T) {
	t.Parallel()
	export, err := mealplan.DailyList{Date: "2026-10-19", Cutoffs: mealplan.Cutoffs{CutoffTime: "14:00", CutoffLeadDays: 1}}.Export()
	require.NoError(t, err)
	assert.Equal(t, "Änderungsfrist: 14:00 Uhr, 1 Werktag vorher", export.Filters[0])
}

func TestExportsNameTheWeekdayCancellationAndClosedDays(t *testing.T) {
	t.Parallel()
	cutoffs := mealplan.Cutoffs{CutoffTime: "16:00", CutoffWeekday: mealplan.Wednesday, CancellationCutoffTime: "09:00", CancellationCutoffLeadDays: 1}
	daily, err := mealplan.DailyList{Cutoffs: cutoffs, Date: "2026-10-19", Closed: true}.Export()
	require.NoError(t, err)
	assert.Equal(t, []string{"Änderungsfrist: 16:00 Uhr, Mittwoch der Vorwoche", "Abmeldefrist: 09:00 Uhr, 1 Werktag vorher", "0 Kinder", "Kein Essen: Feiertag oder Schließtag"}, daily.Filters)

	days := make([]mealplan.DailyList, 0, 5)
	for _, date := range []mealplan.Date{"2026-10-19", "2026-10-20", "2026-10-21", "2026-10-22", "2026-10-23"} {
		days = append(days, mealplan.DailyList{Cutoffs: cutoffs, Date: date})
	}
	days[2].Closed = true
	weekly, err := mealplan.WeeklyList{Cutoffs: cutoffs, WeekStart: "2026-10-19", Days: days}.Export()
	require.NoError(t, err)
	assert.Equal(t, []string{"Summe", "", "0", "0", "kein Essen", "0", "0"}, weekly.Lines[len(weekly.Lines)-1])
	assert.Equal(t, "Abmeldefrist: 09:00 Uhr, 1 Werktag vorher", weekly.Filters[1])
}

func TestFaultsCarryTheirClientTexts(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "feature_disabled", mealplan.ErrDisabled.Detail)
	assert.Equal(t, "meal plan covers weekdays only (Monday-Friday)", mealplan.ErrInvalidMealDate.Detail)
	assert.Equal(t, "meal_registration_disabled", mealplan.ErrRegistrationDisabled.Detail)
	assert.Equal(t, "meal_participation_cutoff_passed", mealplan.ErrParticipationCutoff.Detail)
	assert.Empty(t, mealplan.ErrInvalidDishes.Detail, "the code is the client text")
	assert.Equal(t, "invalid_meal_date", mealplan.ErrInvalidMealDate.Error())
}

func TestSortDailyParticipantsOrdersByClassNameAndID(t *testing.T) {
	t.Parallel()
	participants := []mealplan.DailyParticipant{
		{StudentID: 14, FirstName: "Ana", LastName: "Zorn", SchoolClass: "2a"},
		{StudentID: 13, FirstName: "Ben", LastName: "Adam", SchoolClass: "3b"},
		{StudentID: 12, FirstName: "Mia", LastName: "Adam", SchoolClass: "2a"},
		{StudentID: 11, FirstName: "Mia", LastName: "Adam", SchoolClass: "2a"},
		{StudentID: 15, FirstName: "Lea", LastName: "Adam", SchoolClass: "2a"},
	}
	mealplan.SortDailyParticipants(participants)
	ids := make([]int64, 0, len(participants))
	for _, participant := range participants {
		ids = append(ids, participant.StudentID)
	}
	assert.Equal(t, []int64{15, 11, 12, 14, 13}, ids)
}
