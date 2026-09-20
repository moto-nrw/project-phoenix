package behavior_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestSchoolAccountListingsScopeAndInvitationSuppression(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	query, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	home := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	account := testpkg.CreateTestAccount(t, db, "listing-scope")
	testpkg.MapAccountToTenant(t, db, account.ID, other)
	role := testpkg.CreateTestRole(t, db, "listing-home")
	foreignRole := testpkg.CreateTestRoleForTenant(t, db, "listing-other", other)
	_, err = db.NewRaw("INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?), (?, ?, ?)",
		account.ID, role.ID, home, account.ID, foreignRole.ID, other).Exec(ctx)
	require.NoError(t, err)
	invitation := testpkg.CreateTestInvitationToken(t, db, "listing-invitation", role.ID, account.ID, time.Now().Add(time.Hour))
	_, err = db.NewRaw("UPDATE auth.invitation_tokens SET email = ? WHERE id = ?", strings.ToUpper(account.Email), invitation.ID).Exec(ctx)
	require.NoError(t, err)
	rows, err := query.ListSchoolAccountListings(ctx, []int64{home})
	require.NoError(t, err)
	require.Len(t, rows, 1, "an active mapping suppresses a case-insensitive duplicate invitation")
	require.Equal(t, home, rows[0].SchoolID)
	require.Equal(t, account.ID, rows[0].AccountID)
	require.Equal(t, role.Name, rows[0].RoleName, "another school's role must not leak into the list")
	empty, err := query.ListSchoolAccountListings(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, empty, "an empty school set is never a platform-wide lookup")

	rollback := errors.New("rollback listing fixture changes")
	err = tenant.WithAdminTx(ctx, db, func(txCtx context.Context, tx bun.Tx) error {
		_, updateErr := tx.NewRaw("UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?", account.ID, home).Exec(txCtx)
		require.NoError(t, updateErr)
		inside, readErr := query.ListSchoolAccountListings(txCtx, []int64{home})
		require.NoError(t, readErr)
		require.Len(t, inside, 2, "an inactive mapping remains visible and no longer suppresses the invitation")
		require.Equal(t, "inactive", inside[0].Status)
		require.Equal(t, "invited", inside[1].Status)
		require.Zero(t, inside[1].AccountID)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	after, err := query.ListSchoolAccountListings(ctx, []int64{home})
	require.NoError(t, err)
	require.Equal(t, rows, after, "the projection must use, not escape, an ambient transaction")
}

func TestOperatorAccountListingCaregiverEnrichment(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Ada", "Listing")
	query, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	persons, err := repositories.NewPeopleDirectory(db)
	require.NoError(t, err)
	membership, err := repositories.NewSchoolMembership(db)
	require.NoError(t, err)
	schools, err := repositories.NewOrganizationTenancy(db)
	require.NoError(t, err)
	directory := repositories.NewOperatorAccountDirectory(query, persons, membership, schools)
	ctx := testpkg.Ctx(t)
	rows, err := directory.ListAccountsByTenantID(ctx, testpkg.Tenant(t))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, account.ID, rows[0].AccountID)
	require.Equal(t, "Ada", rows[0].FirstName)
	require.Equal(t, "Listing", rows[0].LastName)
	require.True(t, rows[0].HasCaregiverProfile)
	require.Equal(t, rows[0].HasUserRole, rows[0].IsActiveCaregiver)
	require.NoError(t, membership.DeleteTeacher(ctx, teacher.ID))
	rows, err = directory.ListAccountsByTenantID(ctx, testpkg.Tenant(t))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.False(t, rows[0].HasCaregiverProfile)
	require.False(t, rows[0].IsActiveCaregiver)
}

var _ identityaccess.SchoolAccountListings = (*identityaccess.Module)(nil)
