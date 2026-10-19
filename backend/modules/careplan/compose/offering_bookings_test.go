package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestCareOfferingBookingsPreserveHalfOpenIntervalsAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	phaseID, _, childID := testpkg.CreateAuditAdjustmentChain(t, db)
	offering := testpkg.CreateTestCareOffering(t, db, phaseID, "Effective booking")
	module := NewOfferingBookings()
	ctx := testpkg.Ctx(t)
	from, until := careplan.Date("2030-08-01"), careplan.Date("2031-08-01")
	bookings := []careplan.CareOfferingBooking{{CareOfferingID: offering.ID, ManualSelectedDays: []string{"mon"}, AutomaticSelectedDays: []string{"wed"}, ValidFrom: &from, ValidUntil: &until}}
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, module.RecordCareOfferingBookings, childID, bookings))
	first, err := testpkg.TenantQuery1(t, ctx, db, module.CareOfferingBookingHistory, []int64{childID})
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, []string{"mon", "wed"}, first[0].EffectiveSelectedDays())
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, module.RecordCareOfferingBookings, childID, bookings))
	retry, err := testpkg.TenantQuery1(t, ctx, db, module.CareOfferingBookingHistory, []int64{childID})
	require.NoError(t, err)
	require.Equal(t, first, retry)
	for _, scenario := range []struct {
		date  careplan.Date
		count int
	}{{"2030-07-31", 0}, {from, 1}, {"2031-07-31", 1}, {until, 0}} {
		rows, err := testpkg.TenantQuery1(t, ctx, db, module.CareOfferingBookingsAtDates, map[int64]careplan.Date{childID: scenario.date})
		require.NoError(t, err)
		require.Len(t, rows, scenario.count)
	}
}

func TestCareOfferingBookingsEnforceTwoTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := NewOfferingBookings()
	type school struct {
		ctx                 context.Context
		childID, offeringID int64
	}
	var schools []school
	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			phaseID, _, childID := testpkg.CreateAuditAdjustmentChain(t, db)
			offering := testpkg.CreateTestCareOffering(t, db, phaseID, "Isolated booking")
			ctx := testpkg.Ctx(t)
			require.NoError(t, testpkg.TenantCommand2(t, ctx, db, module.RecordCareOfferingBookings, childID, []careplan.CareOfferingBooking{{CareOfferingID: offering.ID}}))
			schools = append(schools, school{ctx: ctx, childID: childID, offeringID: offering.ID})
		})
	}
	for i, own := range schools {
		foreign := schools[1-i]
		count, err := testpkg.TenantQuery1(t, own.ctx, db, module.CountCareOfferingBookings, []int64{own.childID, foreign.childID})
		require.NoError(t, err)
		require.Equal(t, 1, count, "deletion counts include only the caller's school")
		count, err = testpkg.TenantQuery1(t, own.ctx, db, module.CountCareOfferingBookings, nil)
		require.NoError(t, err)
		require.Zero(t, count, "an empty child selection must not count the whole school")
		rows, err := testpkg.TenantQuery1(t, own.ctx, db, module.CareOfferingBookingHistory, []int64{own.childID, foreign.childID})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, own.childID, rows[0].RequestChildID)
		rows, err = testpkg.TenantQuery1(t, own.ctx, db, module.CareOfferingBookingsAtDates, map[int64]careplan.Date{own.childID: "2030-08-01", foreign.childID: "2030-08-01"})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, own.childID, rows[0].RequestChildID)
		require.Error(t, testpkg.TenantCommand2(t, own.ctx, db, module.RecordCareOfferingBookings, foreign.childID, []careplan.CareOfferingBooking{{CareOfferingID: own.offeringID}}))
		require.Error(t, testpkg.TenantCommand2(t, own.ctx, db, module.RecordCareOfferingBookings, own.childID, []careplan.CareOfferingBooking{{CareOfferingID: foreign.offeringID}}))
		require.NoError(t, testpkg.TenantCommand3(t, own.ctx, db, module.ScheduleCareOfferingBookings, foreign.childID, "2030-08-01", nil))
		rows, err = testpkg.TenantQuery1(t, foreign.ctx, db, module.CareOfferingBookingHistory, []int64{foreign.childID})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Nil(t, rows[0].ValidUntil, "foreign schedule must not modify the other tenant's booking")
	}
}

func TestCareOfferingBookingSwitchPreservesHistoryAndIsIdempotent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	phaseID, _, childID := testpkg.CreateAuditAdjustmentChain(t, db)
	first := testpkg.CreateTestCareOffering(t, db, phaseID, "Original booking")
	second := testpkg.CreateTestCareOffering(t, db, phaseID, "Replacement booking")
	module := NewOfferingBookings()
	ctx := testpkg.Ctx(t)
	start, change, end := careplan.Date("2030-08-01"), careplan.Date("2031-01-01"), careplan.Date("2031-08-01")
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, module.RecordCareOfferingBookings, childID, []careplan.CareOfferingBooking{{CareOfferingID: first.ID, ManualSelectedDays: []string{"mon"}, ValidFrom: &start, ValidUntil: &end}}))
	replacement := []careplan.CareOfferingBooking{{CareOfferingID: second.ID, ManualSelectedDays: []string{"fri"}, ValidUntil: &end}}
	require.NoError(t, testpkg.TenantCommand3(t, ctx, db, module.ScheduleCareOfferingBookings, childID, change, replacement))
	rows, err := testpkg.TenantQuery1(t, ctx, db, module.CareOfferingBookingsAtDates, map[int64]careplan.Date{childID: "2030-12-31"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, first.ID, rows[0].CareOfferingID)
	rows, err = testpkg.TenantQuery1(t, ctx, db, module.CareOfferingBookingsAtDates, map[int64]careplan.Date{childID: change})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, second.ID, rows[0].CareOfferingID)
	history, err := testpkg.TenantQuery1(t, ctx, db, module.CareOfferingBookingHistory, []int64{childID})
	require.NoError(t, err)
	require.Len(t, history, 2)
	require.NoError(t, testpkg.TenantCommand3(t, ctx, db, module.ScheduleCareOfferingBookings, childID, change, replacement))
	retry, err := testpkg.TenantQuery1(t, ctx, db, module.CareOfferingBookingHistory, []int64{childID})
	require.NoError(t, err)
	require.Equal(t, history, retry, "replaying a switch must preserve IDs and history")
	require.NoError(t, testpkg.TenantCommand3(t, ctx, db, module.ScheduleCareOfferingBookings, childID, change, nil))
	rows, err = testpkg.TenantQuery1(t, ctx, db, module.CareOfferingBookingsAtDates, map[int64]careplan.Date{childID: change})
	require.NoError(t, err)
	require.Empty(t, rows, "empty replacement ends care without erasing the past")
}

func TestCareOfferingBookingReplacementIsAtomicAndRetryable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	phaseID, _, childID := testpkg.CreateAuditAdjustmentChain(t, db)
	first := testpkg.CreateTestCareOffering(t, db, phaseID, "Original choice")
	second := testpkg.CreateTestCareOffering(t, db, phaseID, "Corrected choice")
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	start, end := careplan.Date("2030-08-01"), careplan.Date("2031-08-01")
	original := []careplan.CareOfferingBooking{{CareOfferingID: first.ID, ManualSelectedDays: []string{"mon"}, ValidFrom: &start, ValidUntil: &end}}
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, module.RecordCareOfferingBookings, childID, original))
	before, err := testpkg.TenantQuery1(t, ctx, db, module.CareOfferingBookingHistory, []int64{childID})
	require.NoError(t, err)
	invalid := []careplan.CareOfferingBooking{
		{CareOfferingID: second.ID, ManualSelectedDays: []string{"fri"}, ValidFrom: &start, ValidUntil: &end},
		{CareOfferingID: second.ID, ManualSelectedDays: []string{"thu"}, ValidFrom: &start, ValidUntil: &end},
	}
	require.Error(t, testpkg.TenantCommand2(t, ctx, db, module.ReplaceCareOfferingBookings, childID, invalid))
	afterFailure, err := testpkg.TenantQuery1(t, ctx, db, module.CareOfferingBookingHistory, []int64{childID})
	require.NoError(t, err)
	require.Equal(t, before, afterFailure, "failed replacement restores rows deleted before the conflicting insert")
	replacement := invalid[:1]
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, module.ReplaceCareOfferingBookings, childID, replacement))
	after, err := testpkg.TenantQuery1(t, ctx, db, module.CareOfferingBookingHistory, []int64{childID})
	require.NoError(t, err)
	require.Len(t, after, 1)
	require.Equal(t, second.ID, after[0].CareOfferingID)
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, module.ReplaceCareOfferingBookings, childID, replacement))
	retry, err := testpkg.TenantQuery1(t, ctx, db, module.CareOfferingBookingHistory, []int64{childID})
	require.NoError(t, err)
	require.Equal(t, after, retry)
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, module.ReplaceCareOfferingBookings, childID, nil))
	rows, err := testpkg.TenantQuery1(t, ctx, db, module.CareOfferingBookingHistory, []int64{childID})
	require.NoError(t, err)
	require.Empty(t, rows)
}
