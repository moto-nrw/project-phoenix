package staffmessaging_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/models/auth"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// assignSystemRole attaches the named platform system role to the account at
// the test's tenant (the admin sibling of testpkg.AssignLehrkraftSystemRole).
func assignSystemRole(t *testing.T, db *bun.DB, accountID int64, roleName string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var roleID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").TableExpr("auth.roles").
		Where("name = ?", roleName).Where("is_system = TRUE").
		Scan(ctx, &roleID), "seeded %s system role must exist", roleName)

	assignment := &auth.AccountRole{AccountID: accountID, RoleID: roleID}
	assignment.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().Model(assignment).ModelTableExpr(`auth.account_roles`).Exec(ctx)
	require.NoError(t, err)
}

// TestRecipientRoleKinds pins the coarse "who is this" label next to every
// name (#2208): a Lehrkraft, an admin (OGS leadership), a plain colleague, and
// a dual-role account, which reads as admin because that is the side it
// answers for.
func TestRecipientRoleKinds(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newService(t, db)

	_, viewer := testpkg.CreateTestStaffWithAccount(t, db, "Viewer", "Konto")
	_, teacher := testpkg.CreateTestStaffWithAccount(t, db, "Lena", "Lehrkraft")
	_, leader := testpkg.CreateTestStaffWithAccount(t, db, "Lars", "Leitung")
	_, colleague := testpkg.CreateTestStaffWithAccount(t, db, "Ben", "Betreuung")
	_, both := testpkg.CreateTestStaffWithAccount(t, db, "Dana", "Doppelrolle")

	testpkg.AssignLehrkraftSystemRole(t, db, teacher.ID, testpkg.Tenant(t))
	assignSystemRole(t, db, leader.ID, "admin")
	testpkg.AssignLehrkraftSystemRole(t, db, both.ID, testpkg.Tenant(t))
	assignSystemRole(t, db, both.ID, "admin")

	recipients, err := svc.ListMessageableStaff(asAccount(t, viewer.ID))
	require.NoError(t, err)

	kinds := make(map[int64]string, len(recipients))
	for _, r := range recipients {
		kinds[r.AccountID] = r.RoleKind
	}
	assert.Equal(t, usersModels.StaffRoleKindLehrkraft, kinds[teacher.ID])
	assert.Equal(t, usersModels.StaffRoleKindAdmin, kinds[leader.ID])
	assert.Equal(t, usersModels.StaffRoleKindStaff, kinds[colleague.ID])
	assert.Equal(t, usersModels.StaffRoleKindAdmin, kinds[both.ID], "admin wins over lehrkraft for a dual-role account")

	// The same classification travels on the inbox row and the thread detail.
	opened, err := svc.OpenThread(asAccount(t, viewer.ID), teacher.ID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.StaffRoleKindLehrkraft, opened.CounterpartRoleKind)

	_, err = svc.PostMessage(asAccount(t, teacher.ID), opened.ThreadID, "Hallo aus der Schule")
	require.NoError(t, err)
	inbox, err := svc.ListInbox(asAccount(t, viewer.ID), false)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	assert.Equal(t, usersModels.StaffRoleKindLehrkraft, inbox[0].CounterpartRoleKind)
}
