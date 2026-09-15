package selection_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment/selection"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}

func TestMaterializationSeparatesManualRuleAndRequiredLunchDays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	care := testpkg.CreateTestCareOffering(t, db, phase.ID, "Care")
	lunch := testpkg.CreateTestCareOffering(t, db, phase.ID, "Lunch")
	extra := testpkg.CreateTestCareOffering(t, db, phase.ID, "Automatic")
	catalog := map[int64]*selection.Offering{
		care.ID:  {ID: care.ID, SortOrder: 1, DaysOfWeekMode: "fixed", AvailableDays: []string{"mon", "fri"}, CountsAsCare: true},
		lunch.ID: {ID: lunch.ID, SortOrder: 2, DaysOfWeekMode: "parent_choice", AvailableDays: []string{"mon", "fri"}, IsRequired: true, IncludesLunch: true},
		extra.ID: {ID: extra.ID, SortOrder: 3, DaysOfWeekMode: "parent_choice", AvailableDays: []string{"mon", "fri"}, AutoAddTriggerOfferingIDs: []int64{care.ID}},
	}
	children := []selection.Child{{OfferingIDs: []int64{care.ID}}}
	got, err := selection.MaterializeAdjustments(children, catalog, "exactly_one", selection.Grandfathered{}, false)
	require.NoError(t, err)
	require.Len(t, got[0], 3)
	require.Nil(t, got[0][0].SelectedDays)
	require.Equal(t, []string{"mon", "fri"}, got[0][1].AutomaticSelectedDays)
	require.Equal(t, []string{"mon", "fri"}, got[0][2].AutomaticSelectedDays)
	require.Empty(t, got[0][1].ManualSelectedDays)
	require.Equal(t, []int64{care.ID, lunch.ID, extra.ID}, children[0].OfferingIDs)
	children = []selection.Child{{OfferingIDs: []int64{care.ID}, ExcludedAutoAddTargetIDs: map[int64]bool{extra.ID: true, lunch.ID: true}}}
	got, err = selection.MaterializeAdjustments(children, catalog, "exactly_one", selection.Grandfathered{}, false)
	require.NoError(t, err)
	require.Len(t, got[0], 2, "suppressing a rule does not suppress required lunch")
	require.Equal(t, lunch.ID, got[0][1].OfferingID)
	got, err = selection.MaterializeAdjustments([]selection.Child{{}}, catalog, "exactly_one", selection.Grandfathered{}, true)
	require.NoError(t, err)
	require.Empty(t, got[0], "confirmed complete withdrawal waives required lower bounds")
	grade := int16(2)
	catalog[care.ID].AvailabilityRule = &selection.AvailabilityRule{Match: "all", Conditions: []selection.AvailabilityCondition{{Source: "grade_level", Operator: "in", Value: []int{1}}}}
	children = []selection.Child{{TargetGradeLevel: &grade, OfferingIDs: []int64{care.ID}}}
	_, err = selection.MaterializeAdjustments(children, catalog, "optional", selection.Grandfathered{}, false)
	require.ErrorIs(t, err, selection.ErrCareOfferingUnavailable)
	got, err = selection.MaterializeAdjustments(children, catalog, "optional", selection.Grandfathered{Manual: map[int64]bool{care.ID: true}}, false)
	require.NoError(t, err)
	require.Len(t, got[0], 3, "a retained manual booking may continue to trigger automatic shares")
}

func TestAvailabilityRulePreservesMissingGradeAndValidation(t *testing.T) {
	t.Parallel()
	var unrestricted *selection.AvailabilityRule
	matched, err := unrestricted.MatchesGradeLevel(nil)
	require.NoError(t, err)
	require.True(t, matched)
	rule := &selection.AvailabilityRule{Match: "all", Conditions: []selection.AvailabilityCondition{{Source: "grade_level", Operator: "not_in", Value: []int{3, 1, 3}}}}
	matched, err = rule.MatchesGradeLevel(nil)
	require.NoError(t, err)
	require.False(t, matched, "missing grade never satisfies a negative condition")
	grade := int16(2)
	matched, err = rule.MatchesGradeLevel(&grade)
	require.NoError(t, err)
	require.True(t, matched)
	require.Equal(t, []int{1, 3}, rule.Conditions[0].Value)
	rule.Conditions[0].Source = "unknown"
	_, err = rule.MatchesGradeLevel(&grade)
	require.EqualError(t, err, `availability_rule condition 1 has unknown source "unknown"`)
}
