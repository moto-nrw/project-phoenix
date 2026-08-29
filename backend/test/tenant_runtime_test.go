package test

import (
	"context"
	"fmt"
	"testing"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestTenantRuntimePreservesTransactionSettingAndRLSIsolation(t *testing.T) {
	t.Parallel()
	db := SetupTestDB(t)
	owner := TenantScope{TenantID: Tenant(t)}
	other := NewTenantScope(t, db)
	ownerProfile := CreateTestGuardianProfile(t, db, "tenant-runtime-owner")
	otherEmail := fmt.Sprintf("tenant-runtime-other-%d@test.local", other.TenantID)
	otherProfile := &usersModels.GuardianProfile{
		FirstName:              "Other",
		LastName:               "Tenant",
		Email:                  &otherEmail,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	otherProfile.SetTenantID(other.TenantID)
	require.NoError(t, db.NewInsert().Model(otherProfile).ModelTableExpr("users.guardian_profiles").Scan(context.Background()))
	id, err := tenant.NewTenantID(owner.TenantID)
	require.NoError(t, err)
	ctx := tenant.WithRuntime(context.Background(), TenantRuntime(t, db))

	err = tenant.WithTenantTx(ctx, db, id.Int64(), func(txCtx context.Context, outer bun.Tx) error {
		var setting string
		if queryErr := outer.NewRaw("SELECT current_setting('app.current_tenant_id')").Scan(txCtx, &setting); queryErr != nil {
			return queryErr
		}
		assert.Equal(t, fmt.Sprint(id.Int64()), setting)

		current, currentErr := tenant.TenantFromContext(txCtx)
		require.NoError(t, currentErr)
		assert.Equal(t, id, current)

		var ownCount, otherCount int
		require.NoError(t, outer.NewRaw("SELECT count(*) FROM users.guardian_profiles WHERE id = ?", ownerProfile.ID).Scan(txCtx, &ownCount))
		require.NoError(t, outer.NewRaw("SELECT count(*) FROM users.guardian_profiles WHERE id = ?", otherProfile.ID).Scan(txCtx, &otherCount))
		assert.Equal(t, 1, ownCount)
		assert.Zero(t, otherCount, "RLS must hide another tenant")

		return tenant.WithTenantTx(txCtx, db, id.Int64(), func(_ context.Context, nested bun.Tx) error {
			assert.Same(t, outer.Tx, nested.Tx, "nested execution must preserve the active transaction")
			return nil
		})
	})
	require.NoError(t, err)
}

func TestTenantRuntimeRejectsAdminElevationInsideTenantTransaction(t *testing.T) {
	t.Parallel()
	db := SetupTestDB(t)
	tenantID := Tenant(t)
	ctx := WithTenantRuntime(t, context.Background(), db)

	err := tenant.WithTenantTx(ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return tenant.WithAdminTx(txCtx, db, func(context.Context, bun.Tx) error {
			t.Fatal("admin callback must not run inside a tenant transaction")
			return nil
		})
	})

	require.ErrorContains(t, err, "ambient transaction is not administrative")
}

func TestTenantContextIncludesPackageRuntimeAfterDatabaseSetup(t *testing.T) {
	t.Parallel()
	db := SetupTestDB(t)
	tenantID := Tenant(t)
	ctx := TenantContext(tenantID)

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		current, currentErr := tenant.TenantFromContext(txCtx)
		require.NoError(t, currentErr)
		assert.Equal(t, tenantID, current.Int64())
		return nil
	})

	require.NoError(t, err)
	require.NotNil(t, db)
}
