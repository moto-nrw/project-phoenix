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

func TestSchemaQueriesTenantIsolationAndReadFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := enrollmentCompose.New()
	account := testpkg.CreateTestAccount(t, db, "schema-query")
	tenantIDs := make([]int64, 0, 2)
	schemaIDs := make([]int64, 0, 2)
	for _, name := range []string{"first school", "second school"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			tenantID := testpkg.Tenant(t)
			tenantIDs = append(tenantIDs, tenantID)
			var id int64
			err := testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
				return tx.NewRaw(`
     INSERT INTO enrollment.form_schemas (tenant_id, name, version, fields, is_active, created_by)
     VALUES (?, 'Enrollment form', 1, '[]'::jsonb, true, ?) RETURNING id
    `, tenantID, account.ID).Scan(ctx, &id)
			})
			require.NoError(t, err)
			schemaIDs = append(schemaIDs, id)
		})
	}
	ctx := testpkg.ContextForTenant(testpkg.Ctx(t), tenantIDs[0])
	active, err := module.ActiveSchema(ctx)
	require.NoError(t, err)
	require.Equal(t, schemaIDs[0], active.ID)
	require.Equal(t, tenantIDs[0], active.TenantID)
	versions, err := module.SchemaVersions(ctx)
	require.NoError(t, err)
	require.Len(t, versions, 1)
	require.Equal(t, schemaIDs[0], versions[0].ID)
	_, err = module.Schema(ctx, schemaIDs[1])
	require.ErrorIs(t, err, sql.ErrNoRows)

	// Admin transactions bypass RLS. The owner query must still honor its tenant.
	err = tenant.WithAdminTx(ctx, db, func(adminCtx context.Context, _ bun.Tx) error {
		_, readErr := module.Schema(adminCtx, schemaIDs[1])
		require.ErrorContains(t, readErr, "tenant ID is required")
		_, readErr = module.Schema(testpkg.ContextForTenant(adminCtx, tenantIDs[0]), schemaIDs[1])
		require.ErrorIs(t, readErr, sql.ErrNoRows)
		return nil
	})
	require.NoError(t, err)

	// A failed SQL statement poisons this transaction, not the package pool.
	err = testpkg.WithTenantTx(t, ctx, db, tenantIDs[0], func(txCtx context.Context, tx bun.Tx) error {
		_, queryErr := tx.NewRaw("SELECT 1 / 0").Exec(txCtx)
		require.Error(t, queryErr)
		_, readErr := module.ActiveSchema(txCtx)
		require.ErrorContains(t, readErr, "failed to find active form schema")
		require.NotErrorIs(t, readErr, sql.ErrNoRows)
		return readErr
	})
	require.Error(t, err)
	active, err = module.ActiveSchema(ctx)
	require.NoError(t, err, "a rolled-back read failure must not poison the retry")
	require.Equal(t, schemaIDs[0], active.ID)

	_, err = module.RenameSchema(ctx, schemaIDs[1], "Other name")
	require.ErrorIs(t, err, enrollment.ErrFormSchemaNotFound)
	renameFailure := errors.New("failure after schema rename")
	err = testpkg.WithTenantTx(t, ctx, db, tenantIDs[0], func(txCtx context.Context, _ bun.Tx) error {
		renamed, renameErr := module.RenameSchema(txCtx, schemaIDs[0], "New name")
		require.NoError(t, renameErr)
		require.Equal(t, "New name", renamed.Name)
		return renameFailure
	})
	require.ErrorIs(t, err, renameFailure)
	active, err = module.Schema(ctx, schemaIDs[0])
	require.NoError(t, err)
	require.Equal(t, "Enrollment form", active.Name, "rename must roll back with its caller")
	renamed, err := module.RenameSchema(ctx, schemaIDs[0], "New name")
	require.NoError(t, err)
	require.Equal(t, "New name", renamed.Name)
	foreign, err := module.Schema(testpkg.ContextForTenant(testpkg.Ctx(t), tenantIDs[1]), schemaIDs[1])
	require.NoError(t, err)
	require.Equal(t, "Enrollment form", foreign.Name)
	_, err = module.RenameSchema(ctx, schemaIDs[0], "Enrollment form")
	require.NoError(t, err)

	_, err = module.DeleteUnusedSchema(ctx, schemaIDs[1])
	require.ErrorIs(t, err, enrollment.ErrFormSchemaNotFound)
	injected := errors.New("failure after schema deletion")
	err = testpkg.WithTenantTx(t, ctx, db, tenantIDs[0], func(txCtx context.Context, _ bun.Tx) error {
		name, deleteErr := module.DeleteUnusedSchema(txCtx, schemaIDs[0])
		require.NoError(t, deleteErr)
		require.Equal(t, "Enrollment form", name)
		return injected
	})
	require.ErrorIs(t, err, injected)
	_, err = module.Schema(ctx, schemaIDs[0])
	require.NoError(t, err, "schema deletion must roll back with its caller")
	name, err := module.DeleteUnusedSchema(ctx, schemaIDs[0])
	require.NoError(t, err)
	require.Equal(t, "Enrollment form", name)
	_, err = module.Schema(ctx, schemaIDs[0])
	require.ErrorIs(t, err, sql.ErrNoRows)
	_, err = module.Schema(testpkg.ContextForTenant(testpkg.Ctx(t), tenantIDs[1]), schemaIDs[1])
	require.NoError(t, err, "deleting a lineage must not affect another school")
}
