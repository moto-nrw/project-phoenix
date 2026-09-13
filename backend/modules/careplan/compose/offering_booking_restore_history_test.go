package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestCareOfferingBookingsRestoreHistoricalWeekdays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	phaseID, _, childID := testpkg.CreateAuditAdjustmentChain(t, db)
	offering := testpkg.CreateTestCareOffering(t, db, phaseID, "Historical restore")
	module := NewOfferingBookings()
	ctx := testpkg.Ctx(t)
	split := careplan.Date("2030-09-01")
	until := careplan.Date("2030-08-15")
	require.NoError(t, module.RecordCareOfferingBookings(ctx, childID, []careplan.CareOfferingBooking{
		{CareOfferingID: offering.ID, ManualSelectedDays: []string{"mon"}, ValidUntil: &split},
		{CareOfferingID: offering.ID, ManualSelectedDays: []string{"tue"}, ValidFrom: &split},
	}))
	// Persisted legacy payloads can contain values that new commands reject.
	_, err := db.NewRaw(`UPDATE enrollment.care_offering_bookings SET manual_selected_days = '["historical-day"]'::jsonb WHERE tenant_id = ? AND request_child_id = ?`, testpkg.Tenant(t), childID).Exec(ctx)
	require.NoError(t, err)
	before, err := module.CareOfferingBookingHistory(ctx, []int64{childID})
	require.NoError(t, err)
	require.Len(t, before, 2)
	changed, err := module.EndCareOfferingBookings(ctx, []int64{childID}, until)
	require.NoError(t, err)
	require.EqualValues(t, 2, changed)
	restores := make([]careplan.CareOfferingBookingRestore, 0, len(before))
	for _, booking := range before {
		restores = append(restores, careplan.CareOfferingBookingRestore{Booking: booking, WasDeleted: booking.ValidFrom != nil})
	}
	_, err = module.RestoreCareOfferingBookings(ctx, restores)
	require.NoError(t, err)
	_, err = module.RestoreCareOfferingBookings(ctx, restores)
	require.NoError(t, err, "historical restoration must allow an identical retry")
	after, err := module.CareOfferingBookingHistory(ctx, []int64{childID})
	require.NoError(t, err)
	require.Len(t, after, len(before))
	// Restoring a capped row is an update; a deleted row keeps snapshot timestamps.
	require.False(t, after[0].UpdatedAt.Before(before[0].UpdatedAt))
	before[0].UpdatedAt = after[0].UpdatedAt
	require.Equal(t, before, after)
	require.Error(t, module.RecordCareOfferingBookings(ctx, childID, before), "new writes must still reject historical invalid weekdays")
}
