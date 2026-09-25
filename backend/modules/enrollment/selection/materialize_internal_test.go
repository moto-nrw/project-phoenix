package selection

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Offering validation and materialization of one child's picks,
// including the automatic shares (auto-add rules and required lunch).

func TestValidateOfferingSelections_AcceptsKnownOfferings(t *testing.T) {
	t.Parallel()

	open := map[int64]*Offering{
		1: {},
		2: {},
	}
	children := []Child{
		{OfferingIDs: []int64{1}},
		{OfferingIDs: []int64{2, 1}},
	}
	assert.NoError(t, validateOfferingSelections(children, open))
}

func TestValidateOfferingSelections_RejectsUnknownOffering(t *testing.T) {
	t.Parallel()

	open := map[int64]*Offering{1: {}}
	children := []Child{
		{OfferingIDs: []int64{1, 99}}, // 99 not in catalog
	}
	err := validateOfferingSelections(children, open)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCareOfferingClosed),
		"stale-client picks must surface ErrCareOfferingClosed")
}

func TestValidateOfferingSelections_EmptyChildrenIsOK(t *testing.T) {
	t.Parallel()

	assert.NoError(t, validateOfferingSelections(nil, nil))
}

func TestValidateOfferingSelections_ChildWithNoPicksIsOK(t *testing.T) {
	t.Parallel()

	// Phase-level care offering selection is enforced separately. This
	// helper only checks that everything the parent DID pick is in the
	// open catalog, a child with no picks is silently fine here.
	assert.NoError(t, validateOfferingSelections([]Child{{}}, map[int64]*Offering{}))
}

func TestValidateRequiredOfferings_NoRequiredIsOK(t *testing.T) {
	t.Parallel()

	open := map[int64]*Offering{
		1: {IsRequired: false},
		2: {IsRequired: false},
	}
	children := []Child{{OfferingIDs: nil}}
	assert.NoError(t, validateRequiredOfferings(children, open))
}

func TestValidateRequiredOfferings_AcceptsWhenRequiredSelected(t *testing.T) {
	t.Parallel()

	open := map[int64]*Offering{
		1: {IsRequired: true},
		2: {IsRequired: false},
	}
	children := []Child{
		{OfferingIDs: []int64{1}},
		{OfferingIDs: []int64{2, 1}},
	}
	assert.NoError(t, validateRequiredOfferings(children, open))
}

func TestValidateRequiredOfferings_RejectsWhenRequiredMissing(t *testing.T) {
	t.Parallel()

	open := map[int64]*Offering{
		1: {IsRequired: true},
	}
	children := []Child{
		{OfferingIDs: []int64{}}, // required offering 1 not selected
	}
	err := validateRequiredOfferings(children, open)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRequiredCareOfferingMissing),
		"a missing required offering must surface ErrRequiredCareOfferingMissing")
}

func TestValidateRequiredOfferings_RejectsWhenOnlySomeChildrenComply(t *testing.T) {
	t.Parallel()

	open := map[int64]*Offering{
		1: {IsRequired: true},
	}
	children := []Child{
		{OfferingIDs: []int64{1}}, // ok
		{OfferingIDs: []int64{}},  // missing
	}
	err := validateRequiredOfferings(children, open)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRequiredCareOfferingMissing))
}

func TestValidateRequiredOfferings_EmptyCatalogIsOK(t *testing.T) {
	t.Parallel()

	assert.NoError(t, validateRequiredOfferings([]Child{{}}, map[int64]*Offering{}))
}

func TestMaterializeOfferingSelectionsAddsAutomaticOfferingForMatchingGrade(t *testing.T) {
	t.Parallel()

	grade := int16(1)
	primaryOfferingID := int64(101)
	automaticOfferingID := int64(202)
	openByID := map[int64]*Offering{
		primaryOfferingID: {
			ID:             primaryOfferingID,
			DaysOfWeekMode: daysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
			SortOrder:      1,
		},
		automaticOfferingID: {
			ID:                        automaticOfferingID,
			DaysOfWeekMode:            daysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon", "tue", "wed", "thu", "fri"},
			AutoAddTriggerOfferingIDs: []int64{primaryOfferingID},
			AutoAddGradeLevels:        []int{1, 2},
			SortOrder:                 2,
		},
	}
	child := Child{
		TargetGradeLevel: &grade,
		OfferingIDs:      []int64{primaryOfferingID, automaticOfferingID},
		OfferingDays: []DaySelection{
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
	t.Parallel()

	grade := int16(3)
	primaryOfferingID := int64(303)
	automaticOfferingID := int64(404)
	openByID := map[int64]*Offering{
		primaryOfferingID: {
			ID:             primaryOfferingID,
			DaysOfWeekMode: daysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon"},
		},
		automaticOfferingID: {
			ID:                        automaticOfferingID,
			DaysOfWeekMode:            daysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon"},
			AutoAddTriggerOfferingIDs: []int64{primaryOfferingID},
			AutoAddGradeLevels:        []int{1, 2},
		},
	}
	child := Child{
		TargetGradeLevel: &grade,
		OfferingIDs:      []int64{primaryOfferingID},
		OfferingDays:     []DaySelection{{OfferingID: primaryOfferingID, SelectedDays: []string{"mon"}}},
	}

	selections, err := materializeOfferingSelections(child, openByID)

	require.NoError(t, err)
	require.Len(t, selections, 1)
	assert.Equal(t, primaryOfferingID, selections[0].OfferingID)
}

func TestMaterializeOfferingSelectionsRequiredLunchFollowsCareDays(t *testing.T) {
	t.Parallel()

	grade := int16(1)
	careOfferingID := int64(505)
	lunchOfferingID := int64(606)
	openByID := map[int64]*Offering{
		careOfferingID: {
			ID:             careOfferingID,
			DaysOfWeekMode: daysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
			CountsAsCare:   true,
			SortOrder:      1,
		},
		lunchOfferingID: {
			ID:             lunchOfferingID,
			DaysOfWeekMode: daysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
			IncludesLunch:  true,
			IsRequired:     true,
			SortOrder:      2,
		},
	}
	child := Child{
		TargetGradeLevel: &grade,
		OfferingIDs:      []int64{careOfferingID, lunchOfferingID},
		OfferingDays:     []DaySelection{{OfferingID: careOfferingID, SelectedDays: []string{"mon", "wed", "fri"}}},
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
	t.Parallel()

	nonCareOfferingID := int64(707)
	lunchOfferingID := int64(808)
	openByID := map[int64]*Offering{
		nonCareOfferingID: {
			ID:             nonCareOfferingID,
			DaysOfWeekMode: daysOfWeekModeParentChoice,
			AvailableDays:  []string{"fri"},
			CountsAsCare:   false,
			SortOrder:      1,
		},
		lunchOfferingID: {
			ID:             lunchOfferingID,
			DaysOfWeekMode: daysOfWeekModeParentChoice,
			AvailableDays:  []string{"fri"},
			IncludesLunch:  true,
			IsRequired:     true,
			SortOrder:      2,
		},
	}
	child := Child{
		OfferingIDs:  []int64{nonCareOfferingID, lunchOfferingID},
		OfferingDays: []DaySelection{{OfferingID: nonCareOfferingID, SelectedDays: []string{"fri"}}},
	}

	_, err := materializeOfferingSelections(child, openByID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one day")
}

func TestMaterializeOfferingSelectionsResolvesChainedAutoAddDeterministically(t *testing.T) {
	t.Parallel()

	grade := int16(1)
	primaryOfferingID := int64(901)
	firstAutomaticID := int64(902)
	secondAutomaticID := int64(903)
	openByID := map[int64]*Offering{
		secondAutomaticID: {
			ID:                        secondAutomaticID,
			DaysOfWeekMode:            daysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon", "tue", "wed", "thu", "fri"},
			AutoAddTriggerOfferingIDs: []int64{firstAutomaticID},
			SortOrder:                 3,
		},
		primaryOfferingID: {
			ID:             primaryOfferingID,
			DaysOfWeekMode: daysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
			SortOrder:      1,
		},
		firstAutomaticID: {
			ID:                        firstAutomaticID,
			DaysOfWeekMode:            daysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon", "tue", "wed", "thu", "fri"},
			AutoAddTriggerOfferingIDs: []int64{primaryOfferingID},
			SortOrder:                 2,
		},
	}
	child := Child{
		TargetGradeLevel: &grade,
		OfferingIDs:      []int64{primaryOfferingID},
		OfferingDays:     []DaySelection{{OfferingID: primaryOfferingID, SelectedDays: []string{"mon", "wed"}}},
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

// Opt-out (#2370): an excluded auto-add target loses its rule-derived days but
// keeps the days the parents picked themselves.
func TestMaterializeOfferingSelectionsExcludedTargetKeepsManualShare(t *testing.T) {
	t.Parallel()

	grade := int16(1)
	triggerID := int64(1101)
	targetID := int64(1102)
	openByID := map[int64]*Offering{
		triggerID: {
			ID:             triggerID,
			DaysOfWeekMode: daysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
			SortOrder:      1,
		},
		targetID: {
			ID:                        targetID,
			DaysOfWeekMode:            daysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon", "tue", "wed", "thu", "fri"},
			AutoAddTriggerOfferingIDs: []int64{triggerID},
			SortOrder:                 2,
		},
	}
	child := Child{
		TargetGradeLevel:         &grade,
		OfferingIDs:              []int64{triggerID, targetID},
		ExcludedAutoAddTargetIDs: map[int64]bool{targetID: true},
		OfferingDays: []DaySelection{
			{OfferingID: triggerID, SelectedDays: []string{"mon", "tue", "wed", "thu", "fri"}},
			{OfferingID: targetID, SelectedDays: []string{"mon"}},
		},
	}

	selections, err := materializeOfferingSelections(child, openByID)

	require.NoError(t, err)
	require.Len(t, selections, 2)
	assert.Equal(t, targetID, selections[1].OfferingID)
	assert.Equal(t, []string{"mon"}, selections[1].SelectedDays)
	assert.Equal(t, []string{"mon"}, selections[1].ManualSelectedDays)
	assert.Empty(t, selections[1].AutomaticSelectedDays)
}

// A purely automatic excluded target disappears entirely, and an offering that
// was only triggered by the excluded one falls away with it (the chain keeps no
// orphaned bookings).
func TestMaterializeOfferingSelectionsExclusionCascadesThroughChain(t *testing.T) {
	t.Parallel()

	grade := int16(1)
	primaryID := int64(1201)
	firstAutoID := int64(1202)
	secondAutoID := int64(1203)
	openByID := map[int64]*Offering{
		primaryID: {
			ID:             primaryID,
			DaysOfWeekMode: daysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
			SortOrder:      1,
		},
		firstAutoID: {
			ID:                        firstAutoID,
			DaysOfWeekMode:            daysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon", "tue", "wed", "thu", "fri"},
			AutoAddTriggerOfferingIDs: []int64{primaryID},
			SortOrder:                 2,
		},
		secondAutoID: {
			ID:                        secondAutoID,
			DaysOfWeekMode:            daysOfWeekModeParentChoice,
			AvailableDays:             []string{"mon", "tue", "wed", "thu", "fri"},
			AutoAddTriggerOfferingIDs: []int64{firstAutoID},
			SortOrder:                 3,
		},
	}
	child := Child{
		TargetGradeLevel:         &grade,
		OfferingIDs:              []int64{primaryID},
		ExcludedAutoAddTargetIDs: map[int64]bool{firstAutoID: true},
		OfferingDays:             []DaySelection{{OfferingID: primaryID, SelectedDays: []string{"mon", "wed"}}},
	}

	selections, err := materializeOfferingSelections(child, openByID)

	require.NoError(t, err)
	require.Len(t, selections, 1)
	assert.Equal(t, primaryID, selections[0].OfferingID)
}

// The exclusion switches off only the Mitbuchungs-Regel. Required-lunch days
// are not overridable and keep being derived.
func TestMaterializeOfferingSelectionsExclusionKeepsRequiredLunchDays(t *testing.T) {
	t.Parallel()

	grade := int16(1)
	careID := int64(1301)
	lunchID := int64(1302)
	openByID := map[int64]*Offering{
		careID: {
			ID:             careID,
			DaysOfWeekMode: daysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
			CountsAsCare:   true,
			SortOrder:      1,
		},
		lunchID: {
			ID:             lunchID,
			DaysOfWeekMode: daysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
			IsRequired:     true,
			IncludesLunch:  true,
			SortOrder:      2,
		},
	}
	child := Child{
		TargetGradeLevel:         &grade,
		OfferingIDs:              []int64{careID},
		ExcludedAutoAddTargetIDs: map[int64]bool{lunchID: true},
		OfferingDays:             []DaySelection{{OfferingID: careID, SelectedDays: []string{"mon", "tue"}}},
	}

	selections, err := materializeOfferingSelections(child, openByID)

	require.NoError(t, err)
	require.Len(t, selections, 2)
	assert.Equal(t, lunchID, selections[1].OfferingID)
	assert.Equal(t, []string{"mon", "tue"}, selections[1].AutomaticSelectedDays)
}
