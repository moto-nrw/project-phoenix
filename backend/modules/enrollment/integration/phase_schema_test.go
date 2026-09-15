package integration

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestPhaseSchemaRepointIsolationRollbackAndRetry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := enrollmentCompose.New()
	var schoolIDs, sourceIDs, targetIDs []int64
	for _, school := range []string{"first", "second"} {
		t.Run(school, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			actor := testpkg.CreateTestAccount(t, db, "phase-form-pin")
			source := &enrollment.FormSchema{Name: "Form", Version: 1, IsActive: true, CreatedBy: actor.ID}
			target := &enrollment.FormSchema{Name: "Form", Version: 2, IsActive: true, CreatedBy: actor.ID}
			require.NoError(t, module.InsertSchemaVersion(ctx, source))
			require.NoError(t, module.InsertSchemaVersion(ctx, target))
			phase := testpkg.CreateTestEnrollmentPhase(t, db)
			err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
				_, err := tx.NewUpdate().TableExpr("enrollment.phases").Set("form_schema_id = ?", source.ID).Where("id = ?", phase.ID).Exec(txCtx)
				return err
			})
			require.NoError(t, err)
			schoolIDs = append(schoolIDs, testpkg.Tenant(t))
			sourceIDs = append(sourceIDs, source.ID)
			targetIDs = append(targetIDs, target.ID)
		})
	}
	ctx := testpkg.ContextForTenant(testpkg.Ctx(t), schoolIDs[0])
	invalid := &enrollment.Phase{Name: "Foreign schema", ServiceStartDate: "2030-08-01", ServiceEndDate: "2031-07-31", FormSchemaID: &targetIDs[1]}
	require.ErrorIs(t, module.InsertPhase(ctx, invalid), sql.ErrNoRows)
	localPhases, err := module.Phases(ctx)
	require.NoError(t, err)
	require.Len(t, localPhases, 1, "a rejected foreign reference must not insert a phase")
	invalid.ID = localPhases[0].ID
	require.ErrorIs(t, module.UpdatePhase(ctx, invalid), sql.ErrNoRows)
	count, err := module.CountPhaseSchemaReferences(ctx, sourceIDs)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	_, err = module.RepointPhaseSchemas(ctx, sourceIDs, targetIDs[1])
	require.ErrorIs(t, err, sql.ErrNoRows, "a phase cannot pin another school's schema")
	failure := errors.New("failure after phase repoint")
	err = testpkg.WithTenantTx(t, ctx, db, schoolIDs[0], func(txCtx context.Context, _ bun.Tx) error {
		changed, err := module.RepointPhaseSchemas(txCtx, sourceIDs, targetIDs[0])
		require.NoError(t, err)
		require.EqualValues(t, 1, changed)
		return failure
	})
	require.ErrorIs(t, err, failure)
	count, err = module.CountPhaseSchemaReferences(ctx, sourceIDs)
	require.NoError(t, err)
	require.Equal(t, 1, count, "the original pin survives rollback")
	changed, err := module.RepointPhaseSchemas(ctx, sourceIDs, targetIDs[0])
	require.NoError(t, err)
	require.EqualValues(t, 1, changed)
	changed, err = module.RepointPhaseSchemas(ctx, sourceIDs, targetIDs[0])
	require.NoError(t, err)
	require.Zero(t, changed, "retry does not repoint an already advanced phase")
	count, err = module.CountPhaseSchemaReferences(ctx, targetIDs)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	foreignCtx := testpkg.ContextForTenant(testpkg.Ctx(t), schoolIDs[1])
	count, err = module.CountPhaseSchemaReferences(foreignCtx, sourceIDs)
	require.NoError(t, err)
	require.Equal(t, 1, count, "the other school's phase remains pinned to its source")
}
