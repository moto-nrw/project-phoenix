package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func TestContiguousCarePeriodEnd_StopsBeforeGap(t *testing.T) {
	t.Parallel()

	initial := ports.OfferingCarePeriod{ServiceStart: calendar.NewDate(2026, 8, 1), ServiceEnd: calendar.NewDate(2026, 8, 31)}
	consecutive := ports.OfferingCarePeriod{ServiceStart: calendar.NewDate(2026, 9, 1), ServiceEnd: calendar.NewDate(2026, 9, 30)}
	afterGap := ports.OfferingCarePeriod{ServiceStart: calendar.NewDate(2026, 10, 2), ServiceEnd: calendar.NewDate(2026, 10, 31)}

	latest := contiguousCarePeriodEnd([]ports.OfferingCarePeriod{afterGap, consecutive, initial}, initial)

	assert.Equal(t, consecutive.ServiceEnd, latest)
}

func TestCompleteWithdrawalIsDetectedAfterMaterialization(t *testing.T) {
	t.Parallel()
	care := &careplan.CareOffering{
		ID: 1, Name: "Ganztag", CountsAsCare: true, IsRequired: true,
		DaysOfWeekMode: daysOfWeekModeFixed, AvailableDays: []string{"mon", "tue"},
	}
	lunch := &careplan.CareOffering{ID: 2, Name: "Mittagessen", DaysOfWeekMode: daysOfWeekModeFixed, AvailableDays: []string{"mon"}}
	catalog := map[int64]*careplan.CareOffering{care.ID: care, lunch.ID: lunch}

	materialized, err := materializeForAdjustment(
		selectionChild{OfferingIDs: []int64{lunch.ID}}, catalog, "at_least_one", nil, true,
	)
	require.NoError(t, err)
	assert.False(t, selectionsHaveCareDays(materialized, catalog))
	assert.Len(t, materialized, 1, "a non-care offering may remain without blocking the withdrawal")
}

func TestFixedCareOfferingPreventsCompleteWithdrawalWithoutSelectedDays(t *testing.T) {
	t.Parallel()
	care := &careplan.CareOffering{ID: 1, CountsAsCare: true, DaysOfWeekMode: daysOfWeekModeFixed, AvailableDays: []string{"mon"}}

	assert.True(t, selectionsHaveCareDays(
		[]careplan.OfferingSelection{{OfferingID: care.ID}}, map[int64]*careplan.CareOffering{care.ID: care},
	))
}

func TestCompleteWithdrawalStillEnforcesOfferingGroupUpperBounds(t *testing.T) {
	t.Parallel()
	care := &careplan.CareOffering{
		ID: 1, Name: "Ganztag", CountsAsCare: true, IsRequired: true,
		DaysOfWeekMode: daysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	lunchA := &careplan.CareOffering{
		ID: 2, Name: "Essen A", SelectionGroup: "lunch", SelectionRule: "at_most_one",
		DaysOfWeekMode: daysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	lunchB := &careplan.CareOffering{
		ID: 3, Name: "Essen B", SelectionGroup: "lunch", SelectionRule: "at_most_one",
		DaysOfWeekMode: daysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	catalog := map[int64]*careplan.CareOffering{care.ID: care, lunchA.ID: lunchA, lunchB.ID: lunchB}

	_, err := materializeForAdjustment(
		selectionChild{OfferingIDs: []int64{lunchA.ID, lunchB.ID}}, catalog, "at_least_one", nil, true,
	)
	require.ErrorIs(t, err, errCareOfferingRule)
}

func TestCompleteWithdrawalStillEnforcesPhaseExactlyOneUpperBound(t *testing.T) {
	t.Parallel()
	care := &careplan.CareOffering{
		ID: 1, Name: "Ganztag", CountsAsCare: true, IsRequired: true,
		DaysOfWeekMode: daysOfWeekModeFixed, AvailableDays: []string{"mon"},
	}
	lunchA := &careplan.CareOffering{ID: 2, Name: "Essen A", DaysOfWeekMode: daysOfWeekModeFixed, AvailableDays: []string{"mon"}}
	lunchB := &careplan.CareOffering{ID: 3, Name: "Essen B", DaysOfWeekMode: daysOfWeekModeFixed, AvailableDays: []string{"mon"}}
	catalog := map[int64]*careplan.CareOffering{care.ID: care, lunchA.ID: lunchA, lunchB.ID: lunchB}

	_, err := materializeForAdjustment(
		selectionChild{OfferingIDs: []int64{lunchA.ID, lunchB.ID}}, catalog, "exactly_one", nil, true,
	)
	require.ErrorIs(t, err, errCareOfferingExactlyOneRequired)
}

func TestCompleteWithdrawalDetectionUsesFinalAutoMaterialization(t *testing.T) {
	t.Parallel()
	trigger := &careplan.CareOffering{
		ID: 1, Name: "Frühstück", DaysOfWeekMode: daysOfWeekModeParentChoice, AvailableDays: []string{"mon"},
	}
	automaticCare := &careplan.CareOffering{
		ID: 2, Name: "Frühbetreuung", CountsAsCare: true, DaysOfWeekMode: daysOfWeekModeParentChoice,
		AvailableDays: []string{"mon"}, AutoAddTriggerOfferingIDs: []int64{trigger.ID},
	}
	catalog := map[int64]*careplan.CareOffering{trigger.ID: trigger, automaticCare.ID: automaticCare}

	materialized, err := materializeForAdjustment(selectionChild{
		OfferingIDs:  []int64{trigger.ID},
		OfferingDays: []daySelection{{OfferingID: trigger.ID, SelectedDays: []string{"mon"}}},
	}, catalog, "optional", nil, true)
	require.NoError(t, err)
	assert.True(t, selectionsHaveCareDays(materialized, catalog),
		"an automatically materialized care day prevents a false complete-withdrawal warning")
}
