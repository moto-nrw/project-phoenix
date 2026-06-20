package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	baseModels "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

func TestMaterializeOfferingSelectionsAddsAutomaticOfferingForMatchingGrade(t *testing.T) {
	grade := int16(1)
	primaryOfferingID := int64(101)
	automaticOfferingID := int64(202)
	openByID := map[int64]*enrollmentModels.CareOffering{
		primaryOfferingID: {
			Model:          baseModels.Model{ID: primaryOfferingID},
			Name:           "Ganztag",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
			SortOrder:      1,
		},
		automaticOfferingID: {
			Model:                     baseModels.Model{ID: automaticOfferingID},
			Name:                      "Randstunde",
			DaysOfWeekMode:            enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon", "tue", "wed", "thu", "fri"},
			AutoAddTriggerOfferingIDs: []int64{primaryOfferingID},
			AutoAddGradeLevels:        []int{1, 2},
			SortOrder:                 2,
		},
	}
	child := SubmitChild{
		TargetGradeLevel: &grade,
		OfferingIDs:      []int64{primaryOfferingID, automaticOfferingID},
		OfferingDays: []SubmitOfferingDays{
			{OfferingID: primaryOfferingID, SelectedDays: []string{"mon", "tue", "wed", "thu"}},
			{OfferingID: automaticOfferingID, SelectedDays: []string{"fri"}},
		},
	}

	selections, err := materializeOfferingSelections(child, openByID)

	require.NoError(t, err)
	require.Len(t, selections, 2)
	assert.Equal(t, automaticOfferingID, selections[1].OfferingID)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu", "fri"}, selections[1].SelectedDays)
	assert.Equal(t, []string{"fri"}, selections[1].ManualSelectedDays)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu"}, selections[1].AutomaticSelectedDays)
}

func TestMaterializeOfferingSelectionsSkipsAutomaticOfferingForNonMatchingGrade(t *testing.T) {
	grade := int16(3)
	primaryOfferingID := int64(303)
	automaticOfferingID := int64(404)
	openByID := map[int64]*enrollmentModels.CareOffering{
		primaryOfferingID: {
			Model:          baseModels.Model{ID: primaryOfferingID},
			Name:           "Ganztag",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon"},
		},
		automaticOfferingID: {
			Model:                     baseModels.Model{ID: automaticOfferingID},
			Name:                      "Randstunde",
			DaysOfWeekMode:            enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon"},
			AutoAddTriggerOfferingIDs: []int64{primaryOfferingID},
			AutoAddGradeLevels:        []int{1, 2},
		},
	}
	child := SubmitChild{
		TargetGradeLevel: &grade,
		OfferingIDs:      []int64{primaryOfferingID},
		OfferingDays:     []SubmitOfferingDays{{OfferingID: primaryOfferingID, SelectedDays: []string{"mon"}}},
	}

	selections, err := materializeOfferingSelections(child, openByID)

	require.NoError(t, err)
	require.Len(t, selections, 1)
	assert.Equal(t, primaryOfferingID, selections[0].OfferingID)
}

func TestMaterializeOfferingSelectionsRequiredLunchFollowsCareDays(t *testing.T) {
	grade := int16(1)
	careOfferingID := int64(505)
	lunchOfferingID := int64(606)
	openByID := map[int64]*enrollmentModels.CareOffering{
		careOfferingID: {
			Model:          baseModels.Model{ID: careOfferingID},
			Name:           "Ganztag",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
			CountsAsCare:   true,
			SortOrder:      1,
		},
		lunchOfferingID: {
			Model:          baseModels.Model{ID: lunchOfferingID},
			Name:           "Mittagessen",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
			IncludesLunch:  true,
			IsRequired:     true,
			SortOrder:      2,
		},
	}
	child := SubmitChild{
		TargetGradeLevel: &grade,
		OfferingIDs:      []int64{careOfferingID, lunchOfferingID},
		OfferingDays:     []SubmitOfferingDays{{OfferingID: careOfferingID, SelectedDays: []string{"mon", "wed", "fri"}}},
	}

	selections, err := materializeOfferingSelections(child, openByID)

	require.NoError(t, err)
	require.Len(t, selections, 2)
	assert.Equal(t, lunchOfferingID, selections[1].OfferingID)
	assert.Equal(t, []string{"mon", "wed", "fri"}, selections[1].SelectedDays)
	assert.Nil(t, selections[1].ManualSelectedDays)
	assert.Equal(t, []string{"mon", "wed", "fri"}, selections[1].AutomaticSelectedDays)
}

func TestMaterializeOfferingSelectionsRequiredLunchIgnoresNonCareOfferings(t *testing.T) {
	nonCareOfferingID := int64(707)
	lunchOfferingID := int64(808)
	openByID := map[int64]*enrollmentModels.CareOffering{
		nonCareOfferingID: {
			Model:          baseModels.Model{ID: nonCareOfferingID},
			Name:           "AG",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"fri"},
			CountsAsCare:   false,
			SortOrder:      1,
		},
		lunchOfferingID: {
			Model:          baseModels.Model{ID: lunchOfferingID},
			Name:           "Mittagessen",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"fri"},
			IncludesLunch:  true,
			IsRequired:     true,
			SortOrder:      2,
		},
	}
	child := SubmitChild{
		OfferingIDs:  []int64{nonCareOfferingID, lunchOfferingID},
		OfferingDays: []SubmitOfferingDays{{OfferingID: nonCareOfferingID, SelectedDays: []string{"fri"}}},
	}

	_, err := materializeOfferingSelections(child, openByID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one day")
}

func TestMaterializeOfferingSelectionsResolvesChainedAutoAddDeterministically(t *testing.T) {
	grade := int16(1)
	primaryOfferingID := int64(901)
	firstAutomaticID := int64(902)
	secondAutomaticID := int64(903)
	openByID := map[int64]*enrollmentModels.CareOffering{
		secondAutomaticID: {
			Model:                     baseModels.Model{ID: secondAutomaticID},
			Name:                      "Randstunde 2",
			DaysOfWeekMode:            enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon", "tue", "wed", "thu", "fri"},
			AutoAddTriggerOfferingIDs: []int64{firstAutomaticID},
			SortOrder:                 3,
		},
		primaryOfferingID: {
			Model:          baseModels.Model{ID: primaryOfferingID},
			Name:           "Ganztag",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
			SortOrder:      1,
		},
		firstAutomaticID: {
			Model:                     baseModels.Model{ID: firstAutomaticID},
			Name:                      "Randstunde 1",
			DaysOfWeekMode:            enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon", "tue", "wed", "thu", "fri"},
			AutoAddTriggerOfferingIDs: []int64{primaryOfferingID},
			SortOrder:                 2,
		},
	}
	child := SubmitChild{
		TargetGradeLevel: &grade,
		OfferingIDs:      []int64{primaryOfferingID},
		OfferingDays:     []SubmitOfferingDays{{OfferingID: primaryOfferingID, SelectedDays: []string{"mon", "wed"}}},
	}

	for i := 0; i < 25; i++ {
		selections, err := materializeOfferingSelections(child, openByID)

		require.NoError(t, err)
		require.Len(t, selections, 3)
		assert.Equal(t, primaryOfferingID, selections[0].OfferingID)
		assert.Equal(t, firstAutomaticID, selections[1].OfferingID)
		assert.Equal(t, secondAutomaticID, selections[2].OfferingID)
		assert.Equal(t, []string{"mon", "wed"}, selections[1].AutomaticSelectedDays)
		assert.Equal(t, []string{"mon", "wed"}, selections[2].AutomaticSelectedDays)
	}
}

func TestMaterializeAndValidateChildrenOfferingSelectionsValidatesFinalAutoAddedGroupRules(t *testing.T) {
	manualOfferingID := int64(1001)
	automaticOfferingID := int64(1002)
	openByID := map[int64]*enrollmentModels.CareOffering{
		manualOfferingID: {
			Model:          baseModels.Model{ID: manualOfferingID},
			Name:           "Frühbetreuung",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon"},
			SelectionGroup: "randzeiten",
			SelectionRule:  enrollmentModels.SelectionRuleAtMostOne,
			SortOrder:      1,
		},
		automaticOfferingID: {
			Model:                     baseModels.Model{ID: automaticOfferingID},
			Name:                      "Spätbetreuung",
			DaysOfWeekMode:            enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon"},
			AutoAddTriggerOfferingIDs: []int64{manualOfferingID},
			SelectionGroup:            "randzeiten",
			SelectionRule:             enrollmentModels.SelectionRuleAtMostOne,
			SortOrder:                 2,
		},
	}
	children := []SubmitChild{{
		OfferingIDs:  []int64{manualOfferingID},
		OfferingDays: []SubmitOfferingDays{{OfferingID: manualOfferingID, SelectedDays: []string{"mon"}}},
	}}

	_, err := materializeAndValidateChildrenOfferingSelections(children, openByID, enrollmentModels.PhaseCareOfferingSelectionOptional)

	require.ErrorIs(t, err, ErrCareOfferingRule)
}

func TestMaterializeAndValidateChildrenOfferingSelectionsIgnoresAutoAddedOfferingForExactlyOne(t *testing.T) {
	manualOfferingID := int64(1101)
	automaticOfferingID := int64(1102)
	openByID := map[int64]*enrollmentModels.CareOffering{
		manualOfferingID: {
			Model:          baseModels.Model{ID: manualOfferingID},
			Name:           "Ganztag",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "wed"},
			SortOrder:      1,
		},
		automaticOfferingID: {
			Model:                     baseModels.Model{ID: automaticOfferingID},
			Name:                      "Randstunde",
			DaysOfWeekMode:            enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon", "wed"},
			AutoAddTriggerOfferingIDs: []int64{manualOfferingID},
			SortOrder:                 2,
		},
	}
	children := []SubmitChild{{
		OfferingIDs:  []int64{manualOfferingID},
		OfferingDays: []SubmitOfferingDays{{OfferingID: manualOfferingID, SelectedDays: []string{"mon", "wed"}}},
	}}

	selections, err := materializeAndValidateChildrenOfferingSelections(children, openByID, enrollmentModels.PhaseCareOfferingSelectionExactlyOne)

	require.NoError(t, err)
	require.Len(t, selections, 1)
	require.Len(t, selections[0], 2)
	assert.Equal(t, []int64{manualOfferingID, automaticOfferingID}, children[0].OfferingIDs)
	assert.Equal(t, []string{"mon", "wed"}, selections[0][1].AutomaticSelectedDays)
}

func TestMaterializeAndValidateChildrenOfferingSelectionsCountsManualAutoTargetForExactlyOne(t *testing.T) {
	manualOfferingID := int64(1201)
	automaticOfferingID := int64(1202)
	openByID := map[int64]*enrollmentModels.CareOffering{
		manualOfferingID: {
			Model:          baseModels.Model{ID: manualOfferingID},
			Name:           "Ganztag",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon"},
			SortOrder:      1,
		},
		automaticOfferingID: {
			Model:                     baseModels.Model{ID: automaticOfferingID},
			Name:                      "Randstunde",
			DaysOfWeekMode:            enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon"},
			AutoAddTriggerOfferingIDs: []int64{manualOfferingID},
			SortOrder:                 2,
		},
	}
	children := []SubmitChild{{
		OfferingIDs: []int64{manualOfferingID, automaticOfferingID},
		OfferingDays: []SubmitOfferingDays{
			{OfferingID: manualOfferingID, SelectedDays: []string{"mon"}},
			{OfferingID: automaticOfferingID, SelectedDays: []string{"mon"}},
		},
	}}

	_, err := materializeAndValidateChildrenOfferingSelections(children, openByID, enrollmentModels.PhaseCareOfferingSelectionExactlyOne)

	require.ErrorIs(t, err, ErrCareOfferingExactlyOneRequired)
}
