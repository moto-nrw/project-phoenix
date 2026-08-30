package test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func newUnitOfWorkGuardianProfile(t *testing.T, suffix string) *usersModels.GuardianProfile {
	t.Helper()
	email := fmt.Sprintf("unit-of-work-%s-%d@test.local", suffix, Tenant(t))
	return &usersModels.GuardianProfile{
		FirstName:              "Unit",
		LastName:               "Work",
		Email:                  &email,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
}

func TestUnitOfWorkCommitsSuccessfulCommand(t *testing.T) {
	t.Parallel()
	db := SetupTestDB(t)
	repo := repositories.NewFactory(db).GuardianProfile
	profile := newUnitOfWorkGuardianProfile(t, "commit")

	err := tenant.WithinCurrentTenant(Ctx(t), func(txCtx context.Context) error {
		return repo.Create(txCtx, profile)
	})

	require.NoError(t, err)
	stored, err := repo.FindByID(Ctx(t), profile.ID)
	require.NoError(t, err)
	assert.Equal(t, profile.ID, stored.ID)
}

func TestUnitOfWorkRollsBackReturnedError(t *testing.T) {
	t.Parallel()
	db := SetupTestDB(t)
	repo := repositories.NewFactory(db).GuardianProfile
	profile := newUnitOfWorkGuardianProfile(t, "error")
	commandErr := errors.New("command failed")

	err := tenant.WithinCurrentTenant(Ctx(t), func(txCtx context.Context) error {
		require.NoError(t, repo.Create(txCtx, profile))
		return commandErr
	})

	require.ErrorIs(t, err, commandErr)
	_, err = repo.FindByID(Ctx(t), profile.ID)
	assert.ErrorIs(t, err, usersModels.ErrGuardianProfileNotFound)
}

func TestUnitOfWorkRollsBackPanic(t *testing.T) {
	t.Parallel()
	db := SetupTestDB(t)
	repo := repositories.NewFactory(db).GuardianProfile
	profile := newUnitOfWorkGuardianProfile(t, "panic")

	assert.PanicsWithValue(t, "command panicked", func() {
		_ = tenant.WithinCurrentTenant(Ctx(t), func(txCtx context.Context) error {
			require.NoError(t, repo.Create(txCtx, profile))
			panic("command panicked")
		})
	})

	_, err := repo.FindByID(Ctx(t), profile.ID)
	assert.ErrorIs(t, err, usersModels.ErrGuardianProfileNotFound)
}

func TestUnitOfWorkReportsPoolWaitAndTransactionDuration(t *testing.T) {
	t.Parallel()
	SetupTestDB(t)
	var observed []tenant.UnitOfWorkEvent
	ctx := tenant.WithUnitOfWorkObserver(Ctx(t), func(event tenant.UnitOfWorkEvent) {
		observed = append(observed, event)
	})

	err := tenant.WithinCurrentTenant(ctx, func(context.Context) error { return nil })

	require.NoError(t, err)
	require.Len(t, observed, 2)
	assert.Equal(t, tenant.UnitOfWorkPoolWait, observed[0].Kind)
	assert.GreaterOrEqual(t, observed[0].Duration, time.Duration(0))
	assert.Equal(t, tenant.UnitOfWorkTransaction, observed[1].Kind)
	assert.Equal(t, tenant.UnitOfWorkCommitted, observed[1].Result)
	assert.GreaterOrEqual(t, observed[1].Duration, time.Duration(0))
}

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
	ctx := tenant.WithUnitOfWork(context.Background(), TenantRuntime(t, db))

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
