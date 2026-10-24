package mealplan_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonthExportCountsOrderedMealsPerChild(t *testing.T) {
	t.Parallel()
	mia := mealplan.OrderChild{StudentID: 1, FirstName: "Mia", LastName: "Meier", SchoolClass: "2a"}
	ben := mealplan.OrderChild{StudentID: 2, FirstName: "Ben", LastName: "Bauer", SchoolClass: "1b"}
	gone := mealplan.OrderChild{StudentID: 3, SchoolClass: "4c"}
	week := mealplan.OrderWeek{Days: []mealplan.OrderDay{
		{Date: "2026-09-07", Status: mealplan.OrderOrdered, Children: []mealplan.OrderChild{mia, ben}},
		{Date: "2026-09-08", Status: mealplan.OrderConfirmed, Children: []mealplan.OrderChild{mia, gone}},
		{Date: "2026-09-09", Status: mealplan.OrderOpen, Children: []mealplan.OrderChild{mia, ben}},
	}}

	export, err := week.MonthExport("2026-09-01")
	require.NoError(t, err)
	assert.Equal(t, "Bestellte Essen im September 2026", export.Subtitle)
	assert.Equal(t, [][]string{{"Bauer, Ben", "1b", "1"}, {"Meier, Mia", "2a", "2"}, {"Kind 3", "4c", "1"}}, export.Lines, "open days do not count yet")
	assert.Equal(t, []string{"Mit allen Nachmeldungen", "3 Kinder", "4 Essen"}, export.Filters)
	assert.Equal(t, "mittagessen-2026-09", export.Filename)
}

func TestMonthWindowCoversTheWholeMonth(t *testing.T) {
	t.Parallel()
	from, to, err := mealplan.MonthWindow("2026-02-17")
	require.NoError(t, err)
	assert.Equal(t, mealplan.Date("2026-02-01"), from)
	assert.Equal(t, mealplan.Date("2026-02-28"), to)
	_, _, err = mealplan.MonthWindow("2026-13-01")
	require.Error(t, err)
}
