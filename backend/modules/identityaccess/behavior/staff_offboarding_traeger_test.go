package behavior_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestLastSchoolOffboardingPreservesTraegerMembership(t *testing.T) {
	t.Parallel()
	for _, operator := range []bool{false, true} {
		for _, status := range []string{"active", "pending", "inactive"} {
			for _, organizationState := range []string{"active", "inactive", "deleted"} {
				t.Run(fmt.Sprintf("operator=%t/%s/%s", operator, status, organizationState), func(t *testing.T) {
					testpkg.OwnTenant(t)
					db := testpkg.SetupTestDB(t)
					ctx := testpkg.Ctx(t)
					account := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("offboarding-%d@example.invalid", testpkg.Tenant(t)), testPassword)
					if !operator {
						assignSeededRole(t, db, account.ID, testpkg.Tenant(t), "user")
					}
					organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
						Name: "Offboarding", Slug: fmt.Sprintf("offboarding-%d", testpkg.Tenant(t)), Active: true,
					})
					if organizationState == "inactive" {
						_, organizationErr := db.ExecContext(ctx, `UPDATE platform.organizations SET active = false WHERE id = ?`, organization.ID)
						require.NoError(t, organizationErr)
					}
					var organizationActive bool
					require.NoError(t, db.NewRaw(`SELECT active FROM platform.organizations WHERE id = ?`, organization.ID).Scan(ctx, &organizationActive))
					require.Equal(t, organizationState != "inactive", organizationActive, "the fixture must exercise the intended organization state")
					testpkg.CreateTestTraegerMember(t, db, organization.ID, account.ID, "read_only")
					_, err := db.ExecContext(ctx, `UPDATE platform.organization_accounts SET status = ? WHERE organization_id = ? AND account_id = ?`, status, organization.ID, account.ID)
					require.NoError(t, err)
					if organizationState == "deleted" {
						_, err = db.ExecContext(ctx, `UPDATE platform.organizations SET deleted_at = NOW() WHERE id = ?`, organization.ID)
						require.NoError(t, err)
					}
					owner := setupAuthFactory(t, db)
					preserved := status != "inactive"
					if operator {
						actor := testpkg.CreateTestOperator(t, db)
						_, err = owner.RevokeAccountTenantAccess(context.Background(), account.ID, testpkg.Tenant(t), actor.ID, testClientIP.String())
					} else {
						preview, previewErr := owner.PreviewStaffOffboarding(ctx, account.ID)
						require.NoError(t, previewErr)
						require.Equal(t, !preserved, preview.DeactivateAccount)
						result, executeErr := owner.ExecuteStaffOffboarding(ctx, account.ID, preview.Revision)
						err = executeErr
						require.Equal(t, !preserved, result.AccountDeactivated)
					}
					require.NoError(t, err)
					assertAccountActive(t, db, account.ID, preserved)
					var mappingStatus string
					require.NoError(t, db.NewRaw(`SELECT status FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?`, account.ID, testpkg.Tenant(t)).Scan(ctx, &mappingStatus))
					require.Equal(t, "inactive", mappingStatus)
					if status == "active" && organizationState == "active" {
						login, loginErr := owner.LoginTraeger(context.Background(), account.Email, testPassword, "", "", organization.ID)
						require.NoError(t, loginErr, "Träger sign-in must still accept the account")
						require.Contains(t, []identityaccess.LoginStatus{identityaccess.TraegerLoginAuthenticated, identityaccess.TraegerLoginMFAEnrollmentRequired, identityaccess.TraegerLoginMFARequired}, login.Status)
					}
				})
			}
		}
	}
}

func TestStaffOffboardingRejectsChangedTraegerRetention(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "offboarding-retention-revision")
	var organizationID int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, testpkg.Tenant(t)).Scan(ctx, &organizationID))
	testpkg.CreateTestTraegerMember(t, db, organizationID, account.ID)
	owner := setupAuthFactory(t, db)
	for _, status := range []string{"inactive", "pending"} {
		preview, err := owner.PreviewStaffOffboarding(ctx, account.ID)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `UPDATE platform.organization_accounts SET status = ? WHERE organization_id = ? AND account_id = ?`, status, organizationID, account.ID)
		require.NoError(t, err)
		_, err = owner.ExecuteStaffOffboarding(ctx, account.ID, preview.Revision)
		require.ErrorIs(t, err, identityaccess.ErrStaffOffboardingConflict)
		active, err := owner.VerifyAccountTenantMembership(ctx, account.ID, testpkg.Tenant(t))
		require.NoError(t, err)
		require.True(t, active, "a stale preview must change nothing")
		assertAccountActive(t, db, account.ID, true)
	}
}

func TestStaffOffboardingFailsClosedWhenTraegerReadFails(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "offboarding-retention-unavailable")
	owner := setupAuthFactory(t, db)
	preview, err := owner.PreviewStaffOffboarding(ctx, account.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DROP FUNCTION platform.account_has_retained_traeger_membership(BIGINT)`)
	require.NoError(t, err)
	_, err = owner.ExecuteStaffOffboarding(ctx, account.ID, preview.Revision)
	require.Error(t, err)
	active, err := owner.VerifyAccountTenantMembership(ctx, account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	require.True(t, active)
	assertAccountActive(t, db, account.ID, true)
}
