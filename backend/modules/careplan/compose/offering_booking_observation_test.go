package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestOfferingBookingObservationsIncludeValidationErrorsAndActualQueryRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	phaseID, _, childID := testpkg.CreateAuditAdjustmentChain(t, db)
	offering := testpkg.CreateTestCareOffering(t, db, phaseID, "Observed booking")
	var events []careplan.OfferingBookingObservation
	module := newOfferingBookings(func(event careplan.OfferingBookingObservation) { events = append(events, event) })
	ctx := testpkg.Ctx(t)
	bookings := []careplan.CareOfferingBooking{{CareOfferingID: offering.ID, ManualSelectedDays: []string{"mon"}}}
	require.NoError(t, module.RecordCareOfferingBookings(ctx, childID, bookings))
	require.NoError(t, module.RecordCareOfferingBookings(ctx, childID, bookings))
	rows, err := module.CareOfferingBookingHistory(ctx, []int64{childID})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	bookings[0].ManualSelectedDays = []string{"tue"}
	require.ErrorIs(t, module.RecordCareOfferingBookings(ctx, childID, bookings), careplan.ErrCareOfferingBookingConflict)
	require.Error(t, module.ScheduleCareOfferingBookings(ctx, childID, "invalid-date", bookings))
	require.Len(t, events, 5)
	for _, event := range events {
		require.Positive(t, event.Duration)
		require.EqualValues(t, 1, event.InputRows)
	}
	for _, event := range events[:2] {
		require.Equal(t, "record", event.Operation)
		require.Equal(t, "command", event.Kind)
		require.EqualValues(t, -1, event.OutputRows, "an identical retry must not be reported as another persisted row")
		require.NoError(t, event.Err)
	}
	require.Equal(t, "history", events[2].Operation)
	require.Equal(t, "query", events[2].Kind)
	require.EqualValues(t, 1, events[2].OutputRows)
	require.ErrorIs(t, events[3].Err, careplan.ErrCareOfferingBookingConflict)
	require.Equal(t, "schedule", events[4].Operation)
	require.Error(t, events[4].Err, "validation failures must be observable before a transaction starts")
}
