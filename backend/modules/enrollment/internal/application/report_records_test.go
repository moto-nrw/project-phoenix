package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func TestCareUsageScheduleReadsGuardianLevelFields(t *testing.T) {
	t.Parallel()

	schemaID := int64(89)
	req := &enrollmentModels.Request{
		ID: 11,
		CustomData: map[string]any{
			"guardian_pickup":  map[string]any{"mon": "15:30"},
			"guardian_arrival": map[string]any{"mon": "11:15"},
		},
		GuardianFirstName: "Eva",
		GuardianLastName:  "Muster",
		SchemaID:          &schemaID,
	}
	child := &reportChild{
		ID:         21,
		RequestID:  11,
		FirstName:  "Lina",
		LastName:   "Muster",
		Status:     enrollmentModels.ChildStatusApproved,
		CustomData: map[string]any{},
	}
	schemas := map[int64]*enrollment.FormSchema{
		schemaID: {
			Fields: []enrollment.FormField{
				{Key: "guardian_pickup", Target: enrollment.TargetSchedulePickup, Type: enrollment.FormFieldWeekdaySchedule, AppliesToCh: false},
				{Key: "guardian_arrival", Target: enrollment.TargetScheduleArrival, Type: enrollment.FormFieldWeekdaySchedule, AppliesToCh: false},
			},
		},
	}

	pickupByDay, err := careUsagePickupByDay(req, child, schemas)
	require.NoError(t, err)
	arrivalByDay, err := careUsageScheduleByTarget(req, child, schemas, enrollment.TargetScheduleArrival)
	require.NoError(t, err)
	links := []*enrollment.RequestChildOfferingRecord{
		{RequestChildID: 21, CareOfferingID: 1, SelectedDays: []string{"mon"}},
	}
	offerings := map[int64]*enrollmentModels.CareOffering{
		1: {Name: "Randstunde", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice},
	}
	row := careUsageRow(req, child, links, offerings, map[int64]bool{1: true}, pickupByDay)

	assert.Equal(t, "15:30", pickupByDay["mon"])
	assert.Equal(t, "11:15", arrivalByDay["mon"])
	assert.True(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", PickupTime: "15:30"}))
	assert.False(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", PickupTime: "14:30"}))
}
