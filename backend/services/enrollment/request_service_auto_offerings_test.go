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
	openByID := map[int64]*enrollmentModels.CareOffering{
		1: {
			Model:          baseModels.Model{ID: 1},
			Name:           "Ganztag",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
			SortOrder:      1,
		},
		2: {
			Model:                     baseModels.Model{ID: 2},
			Name:                      "Randstunde",
			DaysOfWeekMode:            enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon", "tue", "wed", "thu", "fri"},
			AutoAddTriggerOfferingIDs: []int64{1},
			AutoAddGradeLevels:        []int{1, 2},
			SortOrder:                 2,
		},
	}
	child := SubmitChild{
		TargetGradeLevel: &grade,
		OfferingIDs:      []int64{1, 2},
		OfferingDays: []SubmitOfferingDays{
			{OfferingID: 1, SelectedDays: []string{"mon", "tue", "wed", "thu"}},
			{OfferingID: 2, SelectedDays: []string{"fri"}},
		},
	}

	selections, err := materializeOfferingSelections(child, openByID)

	require.NoError(t, err)
	require.Len(t, selections, 2)
	assert.Equal(t, int64(2), selections[1].OfferingID)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu", "fri"}, selections[1].SelectedDays)
	assert.Equal(t, []string{"fri"}, selections[1].ManualSelectedDays)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu"}, selections[1].AutomaticSelectedDays)
}

func TestMaterializeOfferingSelectionsSkipsAutomaticOfferingForNonMatchingGrade(t *testing.T) {
	grade := int16(3)
	openByID := map[int64]*enrollmentModels.CareOffering{
		1: {
			Model:          baseModels.Model{ID: 1},
			Name:           "Ganztag",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon"},
		},
		2: {
			Model:                     baseModels.Model{ID: 2},
			Name:                      "Randstunde",
			DaysOfWeekMode:            enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon"},
			AutoAddTriggerOfferingIDs: []int64{1},
			AutoAddGradeLevels:        []int{1, 2},
		},
	}
	child := SubmitChild{
		TargetGradeLevel: &grade,
		OfferingIDs:      []int64{1},
		OfferingDays:     []SubmitOfferingDays{{OfferingID: 1, SelectedDays: []string{"mon"}}},
	}

	selections, err := materializeOfferingSelections(child, openByID)

	require.NoError(t, err)
	require.Len(t, selections, 1)
	assert.Equal(t, int64(1), selections[0].OfferingID)
}

func TestMaterializeOfferingSelectionsRequiredLunchFollowsCareDays(t *testing.T) {
	grade := int16(1)
	openByID := map[int64]*enrollmentModels.CareOffering{
		1: {
			Model:          baseModels.Model{ID: 1},
			Name:           "Ganztag",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
			CountsAsCare:   true,
			SortOrder:      1,
		},
		2: {
			Model:          baseModels.Model{ID: 2},
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
		OfferingIDs:      []int64{1, 2},
		OfferingDays:     []SubmitOfferingDays{{OfferingID: 1, SelectedDays: []string{"mon", "wed", "fri"}}},
	}

	selections, err := materializeOfferingSelections(child, openByID)

	require.NoError(t, err)
	require.Len(t, selections, 2)
	assert.Equal(t, int64(2), selections[1].OfferingID)
	assert.Equal(t, []string{"mon", "wed", "fri"}, selections[1].SelectedDays)
	assert.Nil(t, selections[1].ManualSelectedDays)
	assert.Equal(t, []string{"mon", "wed", "fri"}, selections[1].AutomaticSelectedDays)
}

func TestMaterializeOfferingSelectionsRequiredLunchIgnoresNonCareOfferings(t *testing.T) {
	openByID := map[int64]*enrollmentModels.CareOffering{
		1: {
			Model:          baseModels.Model{ID: 1},
			Name:           "AG",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"fri"},
			CountsAsCare:   false,
			SortOrder:      1,
		},
		2: {
			Model:          baseModels.Model{ID: 2},
			Name:           "Mittagessen",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"fri"},
			IncludesLunch:  true,
			IsRequired:     true,
			SortOrder:      2,
		},
	}
	child := SubmitChild{
		OfferingIDs:  []int64{1, 2},
		OfferingDays: []SubmitOfferingDays{{OfferingID: 1, SelectedDays: []string{"fri"}}},
	}

	_, err := materializeOfferingSelections(child, openByID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one day")
}
