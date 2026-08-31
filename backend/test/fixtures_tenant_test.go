package test

import (
	"context"
	"sync"
	"testing"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/stretchr/testify/require"
)

func TestEnsureTestTenant_IsConcurrentSafe(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	tenantID := UniqueTestTenantID(t)
	start := make(chan struct{})
	errs := make(chan error, 16)
	var wg sync.WaitGroup

	for range cap(errs) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			errs <- ensureTestTenant(ctx, db, tenantID)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
}

func TestCleanupTenantTestDataPreservesForeignKeysAndSharedAccounts(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	ownedTenantID := Tenant(t)
	otherTenantID := UniqueTestTenantID(t)
	EnsureTestTenant(t, db, otherTenantID)

	role := CreateTestRoleForTenant(t, db, "cleanup-role", ownedTenantID)
	permission := CreateTestPermission(t, db, "cleanup-permission", "cleanup-resource", "read")
	rolePermission := &authModels.RolePermission{RoleID: role.ID, PermissionID: permission.ID}
	_, err := db.NewInsert().Model(rolePermission).ModelTableExpr("auth.role_permissions").Exec(Ctx(t))
	require.NoError(t, err)

	account := CreateTestAccount(t, db, "cleanup-shared-account")
	EnsureAccountTenant(t, db, account.ID, otherTenantID)

	cleanupTenantTestData(t, db, ownedTenantID)

	var rolePermissionCount int
	err = db.NewSelect().TableExpr("auth.role_permissions").
		ColumnExpr("COUNT(*)").
		Where("role_id = ?", role.ID).
		Scan(context.Background(), &rolePermissionCount)
	require.NoError(t, err)
	require.Zero(t, rolePermissionCount)

	var accountCount int
	err = db.NewSelect().TableExpr("auth.accounts").
		ColumnExpr("COUNT(*)").
		Where("id = ?", account.ID).
		Scan(context.Background(), &accountCount)
	require.NoError(t, err)
	require.Equal(t, 1, accountCount)

	var otherMappingCount int
	err = db.NewSelect().TableExpr("auth.account_tenants").
		ColumnExpr("COUNT(*)").
		Where("account_id = ? AND tenant_id = ?", account.ID, otherTenantID).
		Scan(context.Background(), &otherMappingCount)
	require.NoError(t, err)
	require.Equal(t, 1, otherMappingCount)
}
