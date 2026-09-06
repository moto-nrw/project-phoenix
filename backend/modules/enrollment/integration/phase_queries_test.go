package integration

import (
	"context"
	"testing"
	"time"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestPhaseCalendarReferencesTenantIsolationAndFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := enrollmentCompose.New()
	var schoolIDs, periodIDs []int64
	for _, name := range []string{"first school", "second school"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			period := testpkg.CreateTestCalendarPeriod(t, db, "Phase references", testpkg.Date(2030, time.August, 1), testpkg.Date(2031, time.July, 31))
			phase := testpkg.CreateTestEnrollmentPhase(t, db)
			err := testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
				_, err := tx.NewUpdate().TableExpr("enrollment.phases").Set("calendar_period_id = ?", period.ID).Where("id = ?", phase.ID).Exec(ctx)
				return err
			})
			require.NoError(t, err)
			schoolIDs = append(schoolIDs, testpkg.Tenant(t))
			periodIDs = append(periodIDs, period.ID)
		})
	}
	for i, schoolID := range schoolIDs {
		ctx := testpkg.ContextForTenant(testpkg.Ctx(t), schoolID)
		counter := testpkg.CaptureQueriesForContext(t, db)
		counts, err := module.PhaseCountsByCalendarPeriod(counter.Context(ctx))
		require.NoError(t, err)
		require.Equal(t, map[int64]int{periodIDs[i]: 1}, counts)
		require.Len(t, counter.Selects("enrollment.phases"), 1, "phase counts must use one grouped owner query")
		counter.Stop()
		err = tenant.WithAdminTx(ctx, db, func(adminCtx context.Context, _ bun.Tx) error {
			_, queryErr := module.PhaseCountsByCalendarPeriod(adminCtx)
			require.ErrorContains(t, queryErr, "tenant ID is required")
			counts, queryErr = module.PhaseCountsByCalendarPeriod(testpkg.ContextForTenant(adminCtx, schoolID))
			require.NoError(t, queryErr)
			require.Equal(t, map[int64]int{periodIDs[i]: 1}, counts)
			return nil
		})
		require.NoError(t, err)
	}
	ctx := testpkg.ContextForTenant(testpkg.Ctx(t), schoolIDs[0])
	err := testpkg.WithTenantTx(t, ctx, db, schoolIDs[0], func(txCtx context.Context, tx bun.Tx) error {
		_, err := tx.NewRaw("SELECT 1 / 0").Exec(txCtx)
		require.Error(t, err)
		_, err = module.PhaseCountsByCalendarPeriod(txCtx)
		require.ErrorContains(t, err, "failed to count enrollment phase calendar references")
		return err
	})
	require.Error(t, err)
	counts, err := module.PhaseCountsByCalendarPeriod(ctx)
	require.NoError(t, err)
	require.Equal(t, map[int64]int{periodIDs[0]: 1}, counts)
}
