package integration

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestPhaseStorageTenantIsolationDatesRollbackAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := enrollmentCompose.New()
	var phases []*enrollment.Phase
	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			phase := &enrollment.Phase{Name: name, ServiceStartDate: "2030-03-31", ServiceEndDate: "2030-10-27", IsActive: true}
			require.NoError(t, module.InsertPhase(testpkg.Ctx(t), phase))
			require.Equal(t, testpkg.Tenant(t), phase.TenantID)
			require.Equal(t, enrollment.Date("2030-03-31"), phase.ServiceStartDate)
			require.Equal(t, enrollment.Date("2030-10-27"), phase.ServiceEndDate)
			phases = append(phases, phase)
		})
	}
	ctx := testpkg.ContextForTenant(testpkg.Ctx(t), phases[0].TenantID)
	rows, err := module.Phases(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, phases[0].ID, rows[0].ID)
	_, err = module.Phase(ctx, phases[1].ID)
	require.ErrorIs(t, err, sql.ErrNoRows)
	foreign := *phases[1]
	foreign.Name = "must not change"
	mode := enrollment.PhaseRolloverModeOptIn
	invalidSuccessor := &enrollment.Phase{Name: "Foreign source", ServiceStartDate: "2031-08-01", ServiceEndDate: "2032-07-31", RolloverMode: &mode, RolloverSourcePhaseID: &foreign.ID}
	require.ErrorIs(t, module.InsertPhase(ctx, invalidSuccessor), sql.ErrNoRows)
	require.ErrorIs(t, module.UpdatePhase(ctx, &foreign), sql.ErrNoRows)
	require.Error(t, module.DeletePhase(ctx, foreign.ID))
	err = tenant.WithAdminTx(ctx, db, func(adminCtx context.Context, _ bun.Tx) error {
		_, err := module.Phase(adminCtx, phases[0].ID)
		require.ErrorContains(t, err, "tenant ID is required")
		_, err = module.Phase(testpkg.ContextForTenant(adminCtx, phases[0].TenantID), foreign.ID)
		require.ErrorIs(t, err, sql.ErrNoRows)
		return nil
	})
	require.NoError(t, err)
	failure := errors.New("after phase write")
	updated := *phases[0]
	updated.Name = "updated"
	err = testpkg.WithTenantTx(t, ctx, db, phases[0].TenantID, func(txCtx context.Context, _ bun.Tx) error {
		require.NoError(t, module.UpdatePhase(txCtx, &updated))
		return failure
	})
	require.ErrorIs(t, err, failure)
	current, err := module.Phase(ctx, updated.ID)
	require.NoError(t, err)
	require.Equal(t, "first", current.Name)
	require.NoError(t, module.UpdatePhase(ctx, &updated))
	current, err = module.Phase(ctx, updated.ID)
	require.NoError(t, err)
	require.Equal(t, "updated", current.Name)
	err = testpkg.WithTenantTx(t, ctx, db, updated.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		require.NoError(t, module.DeletePhase(txCtx, updated.ID))
		return failure
	})
	require.ErrorIs(t, err, failure)
	_, err = module.Phase(ctx, updated.ID)
	require.NoError(t, err, "delete must roll back")
	require.NoError(t, module.DeletePhase(ctx, updated.ID))
	_, err = module.Phase(ctx, updated.ID)
	require.ErrorIs(t, err, sql.ErrNoRows)
	foreignCtx := testpkg.ContextForTenant(testpkg.Ctx(t), foreign.TenantID)
	current, err = module.Phase(foreignCtx, foreign.ID)
	require.NoError(t, err)
	require.Equal(t, "second", current.Name)
}
