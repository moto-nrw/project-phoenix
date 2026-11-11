package compose

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The store writes a moved bound only while the row still has the bounds the
// command decided on (#6597): a stale decision fails instead of overwriting
// a booking changed in between.
func TestMoveCareOfferingBookingBoundsWritesOnlyTheDecidedRow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	phaseID, _, childID := testpkg.CreateAuditAdjustmentChain(t, db)
	offering := testpkg.CreateTestCareOffering(t, db, phaseID, "Phase bound booking")
	module := NewOfferingBookings()
	store := postgres.New(postgres.NewOfferingBookingsDatabase())
	ctx := testpkg.Ctx(t)
	from, until, careEnd := careplan.Date("2030-09-01"), careplan.Date("2031-08-01"), careplan.Date("2030-12-01")
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, module.RecordCareOfferingBookings, childID,
		[]careplan.CareOfferingBooking{{CareOfferingID: offering.ID, ManualSelectedDays: []string{"mon"}, ValidFrom: &from, ValidUntil: &until}}))
	_, err := testpkg.TenantQuery2(t, ctx, db, module.EndCareOfferingBookings, []int64{childID}, careEnd)
	require.NoError(t, err)
	rows, err := testpkg.TenantQuery1(t, ctx, db, module.CareOfferingBookingHistory, []int64{childID})
	require.NoError(t, err)
	require.Len(t, rows, 1)

	earlier, later := careplan.Date("2030-08-25"), careplan.Date("2031-08-15")
	stale := careplan.CareOfferingBookingBounds{
		ID: rows[0].ID, RequestChildID: childID,
		ValidFromWas: &from, ValidUntilWas: &until, ValidFrom: &earlier, ValidUntil: &later,
	}
	err = testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		return store.MoveCareOfferingBookingBounds(txCtx, []careplan.CareOfferingBookingBounds{stale})
	})
	require.ErrorContains(t, err, "changed during the phase edit")
	unchanged, err := testpkg.TenantQuery1(t, ctx, db, module.CareOfferingBookingHistory, []int64{childID})
	require.NoError(t, err)
	require.Equal(t, rows, unchanged, "the care end stays")

	current := stale
	current.ValidUntilWas, current.ValidUntil = &careEnd, &careEnd
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		return store.MoveCareOfferingBookingBounds(txCtx, []careplan.CareOfferingBookingBounds{current})
	}))
	moved, err := testpkg.TenantQuery1(t, ctx, db, module.CareOfferingBookingHistory, []int64{childID})
	require.NoError(t, err)
	require.Len(t, moved, 1)
	require.Equal(t, earlier, *moved[0].ValidFrom)
	require.Equal(t, careEnd, *moved[0].ValidUntil)
}
