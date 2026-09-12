package selection_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment/selection"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestAutomaticSharesSeparateRulesFromManualAndRequiredLunch(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	care := testpkg.CreateTestCareOffering(t, db, phase.ID, "Care")
	extra := testpkg.CreateTestCareOffering(t, db, phase.ID, "Extra")
	lunch := testpkg.CreateTestCareOffering(t, db, phase.ID, "Lunch")
	catalog := map[int64]*selection.Offering{
		care.ID:  {ID: care.ID, DaysOfWeekMode: "fixed", AvailableDays: []string{"mon", "fri"}, CountsAsCare: true},
		extra.ID: {ID: extra.ID, DaysOfWeekMode: "parent_choice", AvailableDays: []string{"mon", "fri"}, AutoAddTriggerOfferingIDs: []int64{care.ID}},
		lunch.ID: {ID: lunch.ID, DaysOfWeekMode: "parent_choice", AvailableDays: []string{"mon", "fri"}, IsRequired: true, IncludesLunch: true, AutoAddTriggerOfferingIDs: []int64{care.ID}},
	}
	materialized := []selection.Selection{
		{OfferingID: care.ID},
		{OfferingID: extra.ID, SelectedDays: []string{"mon", "fri"}, ManualSelectedDays: []string{"mon"}, AutomaticSelectedDays: []string{"mon", "fri"}},
		{OfferingID: lunch.ID, SelectedDays: []string{"mon", "fri"}, AutomaticSelectedDays: []string{"mon", "fri"}},
	}
	got := selection.AutomaticShares(materialized, catalog)
	require.NotContains(t, got, care.ID)
	require.Equal(t, []string{"mon", "fri"}, got[extra.ID].AutomaticDays)
	require.Equal(t, []string{"fri"}, got[extra.ID].RuleDays)
	require.Equal(t, []string{"mon"}, got[extra.ID].DaysWithoutRules)
	require.Equal(t, []int64{care.ID}, got[extra.ID].TriggerIDs)
	require.Equal(t, []string{"mon", "fri"}, got[lunch.ID].AutomaticDays)
	require.Empty(t, got[lunch.ID].RuleDays, "required lunch is not attributed to an optional rule")
	require.Empty(t, got[lunch.ID].TriggerIDs)
	got[extra.ID].AutomaticDays[0] = "sun"
	require.Equal(t, "mon", materialized[1].AutomaticSelectedDays[0], "annotations must not mutate selections")
	delete(catalog, extra.ID)
	got = selection.AutomaticShares(materialized, catalog)
	require.Equal(t, []string{"mon", "fri"}, got[extra.ID].AutomaticDays)
	require.Empty(t, got[extra.ID].RuleDays, "missing catalog facts preserve only known automatic days")
}
