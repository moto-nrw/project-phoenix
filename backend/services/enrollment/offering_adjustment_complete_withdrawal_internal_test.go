package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

func TestCompleteWithdrawalIsDetectedAfterMaterialization(t *testing.T) {
	t.Parallel()
	care := &enrollmentModels.CareOffering{
		Model: base.Model{ID: 1}, Name: "Ganztag", CountsAsCare: true,
		IsRequired: true, DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays: []string{"mon", "tue"},
	}
	lunch := &enrollmentModels.CareOffering{
		Model: base.Model{ID: 2}, Name: "Mittagessen", CountsAsCare: false,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	catalog := map[int64]*enrollmentModels.CareOffering{care.ID: care, lunch.ID: lunch}
	children := []SubmitChild{{OfferingIDs: []int64{lunch.ID}}}

	_, err := materializeAndValidateChildrenOfferingSelectionsGrandfathering(
		children, catalog, enrollmentModels.PhaseCareOfferingSelectionAtLeastOne, GrandfatheredOfferings{},
	)
	require.Error(t, err, "ordinary saves must still enforce required care and minimum selection")

	materialized, err := materializeAndValidateChildrenOfferingSelectionsForAdjustment(
		children, catalog, enrollmentModels.PhaseCareOfferingSelectionAtLeastOne, GrandfatheredOfferings{}, true,
	)
	require.NoError(t, err)
	require.Len(t, materialized, 1)
	assert.False(t, materializedSelectionsHaveCareDays(materialized[0], catalog))
	assert.Len(t, materialized[0], 1, "a non-care offering may remain without blocking the withdrawal")
}

func TestFixedCareOfferingPreventsCompleteWithdrawalWithoutSelectedDays(t *testing.T) {
	t.Parallel()
	care := &enrollmentModels.CareOffering{
		Model: base.Model{ID: 1}, CountsAsCare: true,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	assert.True(t, materializedSelectionsHaveCareDays(
		[]materializedOfferingSelection{{OfferingID: care.ID}}, map[int64]*enrollmentModels.CareOffering{care.ID: care},
	))
}

func TestCompleteWithdrawalStillEnforcesOfferingGroupUpperBounds(t *testing.T) {
	t.Parallel()
	care := &enrollmentModels.CareOffering{
		Model: base.Model{ID: 1}, Name: "Ganztag", CountsAsCare: true,
		CountsAsCareSet: true, IsRequired: true,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	lunchA := &enrollmentModels.CareOffering{
		Model: base.Model{ID: 2}, Name: "Essen A", CountsAsCareSet: true,
		SelectionGroup: "lunch", SelectionRule: enrollmentModels.SelectionRuleAtMostOne,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	lunchB := &enrollmentModels.CareOffering{
		Model: base.Model{ID: 3}, Name: "Essen B", CountsAsCareSet: true,
		SelectionGroup: "lunch", SelectionRule: enrollmentModels.SelectionRuleAtMostOne,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	catalog := map[int64]*enrollmentModels.CareOffering{care.ID: care, lunchA.ID: lunchA, lunchB.ID: lunchB}
	children := []SubmitChild{{OfferingIDs: []int64{lunchA.ID, lunchB.ID}}}

	_, err := materializeAndValidateChildrenOfferingSelectionsForAdjustment(
		children, catalog, enrollmentModels.PhaseCareOfferingSelectionAtLeastOne, GrandfatheredOfferings{}, true,
	)
	require.ErrorIs(t, err, ErrCareOfferingRule)
}

func TestCompleteWithdrawalStillEnforcesPhaseExactlyOneUpperBound(t *testing.T) {
	t.Parallel()
	care := &enrollmentModels.CareOffering{
		Model: base.Model{ID: 1}, Name: "Ganztag", CountsAsCare: true,
		CountsAsCareSet: true, IsRequired: true,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	lunchA := &enrollmentModels.CareOffering{
		Model: base.Model{ID: 2}, Name: "Essen A", CountsAsCareSet: true,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	lunchB := &enrollmentModels.CareOffering{
		Model: base.Model{ID: 3}, Name: "Essen B", CountsAsCareSet: true,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	catalog := map[int64]*enrollmentModels.CareOffering{care.ID: care, lunchA.ID: lunchA, lunchB.ID: lunchB}
	children := []SubmitChild{{OfferingIDs: []int64{lunchA.ID, lunchB.ID}}}

	_, err := materializeAndValidateChildrenOfferingSelectionsForAdjustment(
		children, catalog, enrollmentModels.PhaseCareOfferingSelectionExactlyOne, GrandfatheredOfferings{}, true,
	)
	require.ErrorIs(t, err, ErrCareOfferingExactlyOneRequired)
}

func TestCompleteWithdrawalDetectionUsesFinalAutoMaterialization(t *testing.T) {
	t.Parallel()
	trigger := &enrollmentModels.CareOffering{
		Model: base.Model{ID: 1}, Name: "Frühstück", CountsAsCareSet: true,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
		AvailableDays:  []string{"mon"},
	}
	automaticCare := &enrollmentModels.CareOffering{
		Model: base.Model{ID: 2}, Name: "Frühbetreuung", CountsAsCare: true, CountsAsCareSet: true,
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
		AvailableDays:  []string{"mon"}, AutoAddTriggerOfferingIDs: []int64{trigger.ID},
	}
	catalog := map[int64]*enrollmentModels.CareOffering{trigger.ID: trigger, automaticCare.ID: automaticCare}
	children := []SubmitChild{{
		OfferingIDs:  []int64{trigger.ID},
		OfferingDays: []SubmitOfferingDays{{OfferingID: trigger.ID, SelectedDays: []string{"mon"}}},
	}}

	materialized, err := materializeAndValidateChildrenOfferingSelectionsForAdjustment(
		children, catalog, enrollmentModels.PhaseCareOfferingSelectionOptional, GrandfatheredOfferings{}, true,
	)
	require.NoError(t, err)
	assert.True(t, materializedSelectionsHaveCareDays(materialized[0], catalog),
		"an automatically materialized care day prevents a false complete-withdrawal warning")
}
