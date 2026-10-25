package behavior_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// acceptedCatererAccount has the school invite a caterer and accept, and
// returns the caterer's account.
func acceptedCatererAccount(t *testing.T, db *bun.DB, module identityaccess.Owner, school int64, label string) (identityaccess.CatererAcceptance, string) {
	t.Helper()
	staff := testpkg.CreateTestAccount(t, db, "caterer-binding-"+label)
	email := fmt.Sprintf("kueche-%s-%d@example.com", label, time.Now().UnixNano())
	acceptance, err := accept(t, db, module.Caterers(), school, invite(t, db, module.Caterers(), school, staff.ID, email),
		identityaccess.CatererRegistration{Password: catererPassword, ConfirmPassword: catererPassword})
	require.NoError(t, err)
	forgetCaterer(t, db, acceptance)
	t.Cleanup(func() {
		_, err := db.NewRaw(`DELETE FROM audit.auth_events WHERE account_id = ?`, acceptance.AccountID).Exec(context.Background())
		require.NoError(t, err)
	})
	return acceptance, email
}

// requireNoSchool asserts the caterer account holds no school mapping and no
// school role: the refusal left nothing behind.
func requireNoSchool(t *testing.T, db *bun.DB, accountID int64) {
	t.Helper()
	mappings, err := db.NewSelect().TableExpr("auth.account_tenants").Where("account_id = ?", accountID).Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, mappings, "a caterer account holds no school mapping")
	roles, err := db.NewSelect().TableExpr("auth.account_roles").Where("account_id = ?", accountID).Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, roles, "a caterer account holds no school role")
}

// ADR 0077: a caterer account joins no school, whichever flow binds an
// account to one. Every flow that writes auth.account_tenants answers like
// an address that is taken and writes nothing.
func TestCatererAccountBindsToNoSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := setupAuthFactory(t, db)
	school := testpkg.Tenant(t)

	t.Run("admin link to the school", func(t *testing.T) {
		acceptance, email := acceptedCatererAccount(t, db, module, school, "link")
		role := testpkg.CreateTestRoleForTenant(t, db, "caterer-link-role", school)
		_, err := module.LinkSchoolAccount(context.Background(), identityaccess.SchoolAccountLink{
			TenantID: school, Email: email, RoleID: &role.ID,
			Identity: &identityaccess.SchoolAccountIdentity{FirstName: "Küche", LastName: "Markt"},
		})
		require.ErrorIs(t, err, identityaccess.ErrEmailAlreadyExists)
		requireNoSchool(t, db, acceptance.AccountID)
	})

	// Enrollment approval, invite-to-student, the bulk invitation and the
	// guardian invitation all grant through this capability.
	t.Run("guardian tenant access", func(t *testing.T) {
		acceptance, _ := acceptedCatererAccount(t, db, module, school, "guardian")
		_, err := module.GrantGuardianTenantAccess(testpkg.Ctx(t), acceptance.AccountID)
		require.ErrorIs(t, err, identityaccess.ErrEmailAlreadyExists)
		requireNoSchool(t, db, acceptance.AccountID)
	})

	t.Run("operator school access", func(t *testing.T) {
		acceptance, _ := acceptedCatererAccount(t, db, module, school, "operator")
		operator := testpkg.CreateTestOperator(t, db)
		_, err := buildOperatorAccountAccess(t, db).GrantAccountTenantAccess(context.Background(), acceptance.AccountID, school,
			accessGrant{RoleID: systemRoleID(t, db, "admin"), FirstName: "Küche", LastName: "Markt"}, operator.ID, testClientIP)
		require.ErrorIs(t, err, identityaccess.ErrEmailAlreadyExists)
		requireNoSchool(t, db, acceptance.AccountID)
	})

	t.Run("staff invitation", func(t *testing.T) {
		acceptance, email := acceptedCatererAccount(t, db, module, school, "invitation")
		invitations := &fixtureOwnedInvitationService{InvitationCapability: module, t: t, db: db}
		role := testpkg.CreateTestRoleForTenant(t, db, "caterer-invitation-role", school)
		inviter := testpkg.CreateTestAccount(t, db, "caterer-binding-inviter")
		invitation, err := invitations.CreateSchoolInvitation(testpkg.TenantContext(school), identityaccess.SchoolInvitationRequest{
			Email: email, RoleID: role.ID, TenantID: school, CreatedBy: inviter.ID,
			FirstName: testpkg.StrPtr("Küche"), LastName: testpkg.StrPtr("Markt"),
		})
		require.NoError(t, err)
		_, err = invitations.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{
			FirstName: "Küche", LastName: "Markt", Password: catererPassword, ConfirmPassword: catererPassword,
		})
		require.ErrorIs(t, err, identityaccess.ErrEmailAlreadyExists)
		requireNoSchool(t, db, acceptance.AccountID)
	})
}
