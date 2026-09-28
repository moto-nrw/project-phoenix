package contracttest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanTest "github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestOfferingStorageOwnerCommandsRollbackAtEveryBoundary(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, boundary := range []string{"Enrollment", "CarePlan"} {
		t.Run(boundary, func(t *testing.T) {
			testpkg.OwnTenant(t)
			phaseID, _, childID := testpkg.CreateAuditAdjustmentChain(t, db)
			offering := testpkg.CreateTestCareOffering(t, db, phaseID, "Atomic offering")
			choices, bookings := enrollmentTest.New(), carePlanTest.NewOfferingBookings()
			ctx := testpkg.Ctx(t)
			injected := errors.New("failure after owner command")
			run := func(fail bool) error {
				return tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
					if err := choices.RecordSubmittedOfferingChoices(txCtx, childID, []enrollmentTest.SubmittedOfferingChoice{{CareOfferingID: offering.ID, SelectedDays: []string{"mon"}, Notes: new("submitted")}}); err != nil {
						return err
					}
					if fail && boundary == "Enrollment" {
						return injected
					}
					if err := bookings.RecordCareOfferingBookings(txCtx, childID, []careplan.CareOfferingBooking{{CareOfferingID: offering.ID, ManualSelectedDays: []string{"mon"}, AutomaticSelectedDays: []string{"wed"}}}); err != nil {
						return err
					}
					if fail {
						return injected
					}
					return nil
				})
			}
			require.ErrorIs(t, run(true), injected)
			submitted, err := choices.SubmittedOfferingChoices(ctx, []int64{childID})
			require.NoError(t, err)
			require.Empty(t, submitted)
			effective, err := bookings.CareOfferingBookingHistory(ctx, []int64{childID})
			require.NoError(t, err)
			require.Empty(t, effective)
			require.NoError(t, run(false))
			require.NoError(t, run(false))
			submitted, err = choices.SubmittedOfferingChoices(ctx, []int64{childID})
			require.NoError(t, err)
			require.Len(t, submitted, 1)
			require.Equal(t, []string{"mon"}, submitted[0].SelectedDays)
			effective, err = bookings.CareOfferingBookingHistory(ctx, []int64{childID})
			require.NoError(t, err)
			require.Len(t, effective, 1)
			require.Equal(t, []string{"mon", "wed"}, effective[0].EffectiveSelectedDays())
			require.NoError(t, bookings.ReplaceCareOfferingBookings(ctx, childID, []careplan.CareOfferingBooking{{CareOfferingID: offering.ID, ManualSelectedDays: []string{"fri"}}}))
			afterCorrection, err := choices.SubmittedOfferingChoices(ctx, []int64{childID})
			require.NoError(t, err)
			require.Equal(t, submitted, afterCorrection, "correcting effective care must preserve the submitted choice and notes")
		})
	}
}
