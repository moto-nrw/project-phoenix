package api

// Hermetic tests for the #1419 email-recipient lookups:
// ListStaffWithPermission (approver fan-out) and GetStaffContactInfo
// (requester address). Fixtures via testpkg; the vacation:approve
// permission row comes from migration 1.15.105.

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListStaffWithPermission_RoleGrantOnly(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Staff
	ctx := testpkg.Ctx(t)

	// Staff WITH an active tenant mapping (calendar helper adds mapping + base
	// user role); vacation:approve arrives first as a retired direct grant,
	// then through a role.
	approver, approverAccount := testpkg.CreateTestCalendarStaff(t, db, "Lena", "Approver")
	// Staff without the permission — must not appear.
	bystander, _ := testpkg.CreateTestCalendarStaff(t, db, "Bodo", "Bystander")

	var permissionID int64
	sqlCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.permissions").
		Where("resource = ? AND action = ?", "vacation", "approve").
		Scan(sqlCtx, &permissionID)
	require.NoError(t, err, "vacation:approve permission must exist (migration 1.15.105)")

	_, err = db.NewInsert().
		Model(&map[string]any{
			"account_id":    approverAccount.ID,
			"permission_id": permissionID,
			"granted":       true,
			"tenant_id":     testpkg.Tenant(t),
		}).
		TableExpr(`auth.account_permissions`).
		Exec(sqlCtx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("auth.account_permissions").
			Where("account_id = ?", approverAccount.ID).Exec(context.Background())
	})

	listed := func() map[int64]string {
		t.Helper()
		result, err := testpkg.TenantQuery1(t, ctx, db, repo.ListStaffWithPermission, "vacation:approve")
		require.NoError(t, err)
		ids := make(map[int64]string)
		for _, info := range result {
			ids[info.StaffID] = info.Email
		}
		return ids
	}
	require.NotContains(t, listed(), approver.ID, "a retired direct grant no longer qualifies (#4485)")

	testpkg.GrantTestPermission(t, db, testpkg.Tenant(t), approverAccount.ID, "vacation:approve")
	ids := listed()
	require.Contains(t, ids, approver.ID, "staff granted through a role must be listed")
	assert.Equal(t, approverAccount.Email, ids[approver.ID], "email must come from the account")
	assert.NotContains(t, ids, bystander.ID, "staff without the permission must not be listed")
}

func TestGetStaffContactInfo_ReturnsNameAndEmail(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Staff
	ctx := testpkg.Ctx(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Mila", "Muster")

	info, err := testpkg.TenantQuery1(t, ctx, db, repo.GetStaffContactInfo, staff.ID)
	require.NoError(t, err)
	assert.Equal(t, staff.ID, info.StaffID)
	assert.Equal(t, "Mila", info.FirstName)
	assert.Equal(t, "Muster", info.LastName)
	assert.Equal(t, account.Email, info.Email)
}
