package application

import (
	"testing"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCareUsageScheduleReadsGuardianLevelFields(t *testing.T) {
	t.Parallel()

	schemaID := int64(89)
	req := &Request{
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
		Status:     enrollmentNative.ChildStatusApproved,
		CustomData: map[string]any{},
	}
	schemas := map[int64]*enrollmentNative.FormSchema{
		schemaID: {
			Fields: []enrollmentNative.FormField{
				{Key: "guardian_pickup", Target: enrollmentNative.TargetSchedulePickup, Type: enrollmentNative.FormFieldWeekdaySchedule, AppliesToCh: false},
				{Key: "guardian_arrival", Target: enrollmentNative.TargetScheduleArrival, Type: enrollmentNative.FormFieldWeekdaySchedule, AppliesToCh: false},
			},
		},
	}

	pickupByDay, err := careUsagePickupByDay(req, child, schemas)
	require.NoError(t, err)
	arrivalByDay, err := careUsageScheduleByTarget(req, child, schemas, enrollmentNative.TargetScheduleArrival)
	require.NoError(t, err)
	links := []*enrollmentNative.RequestChildOfferingRecord{
		{RequestChildID: 21, CareOfferingID: 1, SelectedDays: []string{"mon"}},
	}
	offerings := map[int64]*offeringContract.CareOfferingRecord{
		1: {Name: "Randstunde", DaysOfWeekMode: offeringContract.DaysOfWeekModeParentChoice},
	}
	row := careUsageRow(req, child, links, offerings, map[int64]bool{1: true}, pickupByDay)

	assert.Equal(t, "15:30", pickupByDay["mon"])
	assert.Equal(t, "11:15", arrivalByDay["mon"])
	assert.True(t, careUsageRowMatches(row, enrollmentNative.CareUsageFilters{Status: "all", PickupTime: "15:30"}))
	assert.False(t, careUsageRowMatches(row, enrollmentNative.CareUsageFilters{Status: "all", PickupTime: "14:30"}))
}
