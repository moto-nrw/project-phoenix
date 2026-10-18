package mealplan_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDailyListExportLaysOutTheTagesliste(t *testing.T) {
	t.Parallel()
	list := mealplan.DailyList{Date: "2026-10-19", CutoffTime: "09:30", Participants: []mealplan.DailyParticipant{
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

func TestFaultsCarryTheirClientTexts(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "feature_disabled", mealplan.ErrDisabled.Detail)
	assert.Equal(t, "meal plan covers weekdays only (Monday-Friday)", mealplan.ErrInvalidMealDate.Detail)
	assert.Equal(t, "meal_registration_disabled", mealplan.ErrRegistrationDisabled.Detail)
	assert.Equal(t, "meal_participation_cutoff_passed", mealplan.ErrParticipationCutoff.Detail)
	assert.Empty(t, mealplan.ErrInvalidDishes.Detail, "the code is the client text")
	assert.Equal(t, "invalid_meal_date", mealplan.ErrInvalidMealDate.Error())
}
