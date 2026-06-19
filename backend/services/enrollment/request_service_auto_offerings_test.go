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
