package behavior_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var catererPassword = testpkg.NewTestPassword()

// forgetCaterer deletes the rows an acceptance writes outside any school:
// the caterer, its account link and the account.
func forgetCaterer(t *testing.T, db *bun.DB, acceptance identityaccess.CatererAcceptance) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_, err := db.NewRaw(`DELETE FROM auth.caterers WHERE id = ?`, acceptance.CatererID).Exec(ctx)
		require.NoError(t, err)
		_, err = db.NewRaw(`DELETE FROM auth.tokens WHERE account_id = ?`, acceptance.AccountID).Exec(ctx)
		require.NoError(t, err)
		_, err = db.NewRaw(`DELETE FROM auth.accounts WHERE id = ?`, acceptance.AccountID).Exec(ctx)
		require.NoError(t, err)
	})
}

// inSchool runs fn in the school's transaction, as the caterer portal and
// the school's routes do (#4088).
func inSchool(t *testing.T, db *bun.DB, school int64, fn func(context.Context) error) error {
	t.Helper()
	return testpkg.WithinTenantContext(t, context.Background(), db, school, fn)
}

// invite has the school invite the address and reads the link from the
// mail, as only the caterer can: the school never sees it.
func invite(t *testing.T, db *bun.DB, caterers *identityaccess.CatererAccess, school, staffID int64, email string) string {
	t.Helper()
	var invitation identityaccess.CatererInvitation
	require.NoError(t, inSchool(t, db, school, func(ctx context.Context) error {
		var err error
		invitation, err = caterers.CreateCatererInvitation(ctx, identityaccess.CatererInvitationRequest{
			Email: email, CatererName: "Küche am Markt", DeliveryAddress: "Schulstraße 1", CreatedBy: staffID,
		})
		return err
	}))
	var raw string
	require.NoError(t, db.NewRaw(`SELECT payload::text FROM platform.email_outbox
		WHERE kind = 'caterer_invitation' AND related_entity_type = 'caterer_invitation' AND related_entity_id = ?`, invitation.ID).Scan(context.Background(), &raw))
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &payload))
	assert.Equal(t, email, payload["recipient_email"])
	link, _ := payload["invitation_url"].(string)
	prefix := fmt.Sprintf("http://caterer.localhost:3000/einladung/%d/", school)
	require.True(t, strings.HasPrefix(link, prefix), link)
	return strings.TrimPrefix(link, prefix)
}

func accept(t *testing.T, db *bun.DB, caterers *identityaccess.CatererAccess, school int64, token string, registration identityaccess.CatererRegistration) (identityaccess.CatererAcceptance, error) {
	t.Helper()
	var acceptance identityaccess.CatererAcceptance
	err := inSchool(t, db, school, func(ctx context.Context) error {
		var err error
		acceptance, err = caterers.AcceptCatererInvitation(ctx, token, registration)
		return err
	})
	return acceptance, err
}

// ADR 0077: a school invites its caterer, the caterer accepts with a
// password, and signs in to the caterer portal only.
func TestCatererInvitationAcceptanceAndLogin(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := setupAuthFactory(t, db)
	caterers := module.Caterers()
	require.NotNil(t, caterers, "the composed module serves the caterer flows")
	school := testpkg.Tenant(t)
	staff := testpkg.CreateTestAccount(t, db, "caterer-inviter")
	email := fmt.Sprintf("kueche-%d@example.com", time.Now().UnixNano())
	token := invite(t, db, caterers, school, staff.ID, email)

	var preview identityaccess.CatererInvitationPreview
	require.NoError(t, inSchool(t, db, school, func(ctx context.Context) error {
		var err error
		preview, err = caterers.PreviewCatererInvitation(ctx, token)
		return err
	}))
	assert.Equal(t, email, preview.Email)
	assert.False(t, preview.HasCatererAccount)
	assert.False(t, preview.AddressTaken)

	acceptance, err := accept(t, db, caterers, school, token, identityaccess.CatererRegistration{Password: catererPassword, ConfirmPassword: catererPassword})
	require.NoError(t, err)
	forgetCaterer(t, db, acceptance)
	assert.Equal(t, school, acceptance.TenantID)
	require.NoError(t, inSchool(t, db, school, func(ctx context.Context) error {
		supply, active, err := caterers.SchoolCaterer(ctx)
		require.True(t, active, "the supply starts in the acceptance")
		assert.Equal(t, acceptance.CatererID, supply.Caterer.ID)
		assert.Equal(t, "Küche am Markt", supply.Caterer.Name)
		assert.Equal(t, "Schulstraße 1", supply.DeliveryAddress)
		return err
	}))
	var startedBy int64
	require.NoError(t, db.NewRaw(`SELECT started_by_account_id FROM auth.caterer_schools WHERE tenant_id = ? AND ended_at IS NULL`, school).Scan(context.Background(), &startedBy))
	assert.Equal(t, acceptance.AccountID, startedBy)
	_, err = accept(t, db, caterers, school, token, identityaccess.CatererRegistration{Password: catererPassword, ConfirmPassword: catererPassword})
	require.ErrorIs(t, err, identityaccess.ErrInvitationUsed)

	caterer, schools, err := caterers.CatererSchools(context.Background(), acceptance.AccountID)
	require.NoError(t, err)
	assert.Equal(t, acceptance.CatererID, caterer.ID)
	assert.Equal(t, []int64{school}, schools)

	access, refresh, err := caterers.LoginCatererWithAudit(context.Background(), email, catererPassword, "", "")
	require.NoError(t, err)
	claims, err := module.ValidateSessionTokens(context.Background(), access, refresh, "caterer")
	require.NoError(t, err)
	assert.Equal(t, acceptance.AccountID, claims.AccountID)
	for _, other := range []string{"tenant", "parent", "school"} {
		_, err = module.ValidateSessionTokens(context.Background(), access, refresh, other)
		require.Error(t, err, other)
	}

	renewedAccess, renewedRefresh, err := module.RefreshTokenWithAudit(context.Background(), refresh, "", "", "")
	require.NoError(t, err)
	_, err = module.ValidateSessionTokens(context.Background(), renewedAccess, renewedRefresh, "caterer")
	require.NoError(t, err, "a refreshed caterer session stays a caterer session")

	_, _, err = caterers.LoginCatererWithAudit(context.Background(), email, "wrong-password", "", "")
	require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials)

	require.NoError(t, inSchool(t, db, school, func(ctx context.Context) error {
		ended, err := caterers.EndCatererSchool(ctx, staff.ID)
		assert.True(t, ended)
		return err
	}))
	var endedBy int64
	require.NoError(t, db.NewRaw(`SELECT ended_by_account_id FROM auth.caterer_schools WHERE tenant_id = ?`, school).Scan(context.Background(), &endedBy))
	assert.Equal(t, staff.ID, endedBy, "the supply records who ended it")
	_, _, err = caterers.LoginCatererWithAudit(context.Background(), email, catererPassword, "", "")
	require.ErrorIs(t, err, identityaccess.ErrCatererWithoutSchool, "without a school there is nothing to sign in to")
}

// A caterer supplies several schools, also of different Träger: a second
// school's invitation goes to the same account, confirmed with its password,
// and ending one supply leaves the other.
func TestCatererSuppliesSeveralSchools(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := setupAuthFactory(t, db)
	caterers := module.Caterers()
	first := testpkg.Tenant(t)
	second, _ := testpkg.CreateTestTenant(t, db)
	staff := testpkg.CreateTestAccount(t, db, "caterer-two-schools")
	email := fmt.Sprintf("kueche-zwei-%d@example.com", time.Now().UnixNano())

	acceptance, err := accept(t, db, caterers, first, invite(t, db, caterers, first, staff.ID, email), identityaccess.CatererRegistration{Password: catererPassword, ConfirmPassword: catererPassword})
	require.NoError(t, err)
	forgetCaterer(t, db, acceptance)

	secondToken := invite(t, db, caterers, second, staff.ID, email)
	require.NoError(t, inSchool(t, db, second, func(ctx context.Context) error {
		preview, err := caterers.PreviewCatererInvitation(ctx, secondToken)
		assert.True(t, preview.HasCatererAccount, "the second school's page asks for the existing password")
		return err
	}))
	_, err = accept(t, db, caterers, second, secondToken, identityaccess.CatererRegistration{Password: "wrong-password"})
	require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials)
	var again identityaccess.CatererAcceptance
	err = inSchool(t, db, second, func(ctx context.Context) error {
		var acceptErr error
		again, acceptErr = caterers.AcceptCatererInvitation(ctx, secondToken, identityaccess.CatererRegistration{Password: catererPassword})
		// Retention must wait until acceptance commits. NO KEY UPDATE is
		// compatible with foreign-key locks, so only the account lock taken
		// by acceptance prevents this competing transaction.
		competing, beginErr := db.BeginTx(context.Background(), nil)
		require.NoError(t, beginErr)
		defer func() { _ = competing.Rollback() }()
		_, lockErr := competing.ExecContext(context.Background(), `SELECT id FROM auth.accounts WHERE id = ? FOR NO KEY UPDATE NOWAIT`, acceptance.AccountID)
		require.ErrorContains(t, lockErr, "could not obtain lock", "acceptance holds the existing account until commit")
		return acceptErr
	})
	require.NoError(t, err)
	assert.Equal(t, acceptance.AccountID, again.AccountID, "no second account")
	assert.Equal(t, acceptance.CatererID, again.CatererID)

	_, schools, err := caterers.CatererSchools(context.Background(), acceptance.AccountID)
	require.NoError(t, err)
	assert.Equal(t, []int64{first, second}, schools)
	t.Cleanup(func() {
		_, err := db.NewRaw(`DELETE FROM audit.auth_events WHERE account_id = ?`, acceptance.AccountID).Exec(context.Background())
		require.NoError(t, err)
	})
	_, refresh, err := caterers.LoginCatererWithAudit(context.Background(), email, catererPassword, "192.0.2.11", "test")
	require.NoError(t, err)

	require.NoError(t, inSchool(t, db, first, func(ctx context.Context) error {
		_, err := caterers.EndCatererSchool(ctx, staff.ID)
		return err
	}))
	_, schools, err = caterers.CatererSchools(context.Background(), acceptance.AccountID)
	require.NoError(t, err)
	assert.Equal(t, []int64{second}, schools, "the first school's end leaves the second")

	// The session pinned to the first school moves to the second on refresh,
	// so the first school no longer logs the caterer's sign-ins.
	_, successor, err := module.RefreshTokenWithAudit(context.Background(), refresh, "example-recovery-proof", "192.0.2.11", "test")
	require.NoError(t, err)
	var pinned, refreshedAt int64
	require.NoError(t, db.NewRaw(`SELECT tenant_id FROM auth.tokens WHERE account_id = ? ORDER BY id DESC LIMIT 1`, acceptance.AccountID).Scan(context.Background(), &pinned))
	assert.Equal(t, second, pinned)

	// Retry the predecessor after losing the repinning response, with a fresh
	// service instance so recovery must use the persisted handoff.
	restarted := setupAuthFactory(t, db)
	access, recovered, err := restarted.RefreshTokenWithAudit(context.Background(), refresh, "example-recovery-proof", "", "")
	require.NoError(t, err)
	_, err = restarted.ValidateSessionTokens(context.Background(), access, recovered, "caterer")
	require.NoError(t, err)
	assert.NotEmpty(t, successor)
	var generations []int
	require.NoError(t, db.NewRaw(`SELECT generation FROM auth.tokens WHERE account_id = ? ORDER BY generation`, acceptance.AccountID).Scan(context.Background(), &generations))
	assert.Equal(t, []int{0, 1}, generations, "recovery reuses the repinned successor")
	require.NoError(t, db.NewRaw(`SELECT tenant_id FROM audit.auth_events WHERE account_id = ? AND success ORDER BY id DESC LIMIT 1`, acceptance.AccountID).Scan(context.Background(), &refreshedAt))
	assert.Equal(t, second, refreshedAt, "the refresh is logged at the school that still supplies the caterer")
	_, _, err = caterers.LoginCatererWithAudit(context.Background(), email, catererPassword, "", "")
	require.NoError(t, err)
}

// Only a caterer account signs in to the caterer portal, and an invitation
// for the address of another kind of account cannot be accepted.
func TestCatererAccessRefusesOtherAccounts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caterers := setupAuthFactory(t, db).Caterers()
	school := testpkg.Tenant(t)
	staffPassword := testpkg.NewTestPassword()
	staff := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("caterer-staff-%d@example.com", time.Now().UnixNano()), staffPassword)

	_, _, err := caterers.LoginCatererWithAudit(context.Background(), staff.Email, staffPassword, "", "")
	require.ErrorIs(t, err, identityaccess.ErrAccountNotCaterer)

	token := invite(t, db, caterers, school, staff.ID, staff.Email)
	require.NoError(t, inSchool(t, db, school, func(ctx context.Context) error {
		preview, err := caterers.PreviewCatererInvitation(ctx, token)
		assert.True(t, preview.AddressTaken)
		return err
	}))
	_, err = accept(t, db, caterers, school, token, identityaccess.CatererRegistration{Password: staffPassword})
	require.ErrorIs(t, err, identityaccess.ErrCatererAddressTaken)

	require.NoError(t, inSchool(t, db, school, func(ctx context.Context) error {
		invitations, err := caterers.ListCatererInvitations(ctx)
		require.NoError(t, err)
		require.Len(t, invitations, 1)
		return caterers.RevokeCatererInvitation(ctx, invitations[0].ID)
	}))
	err = inSchool(t, db, school, func(ctx context.Context) error {
		_, err := caterers.PreviewCatererInvitation(ctx, token)
		return err
	})
	require.ErrorIs(t, err, identityaccess.ErrInvitationRevoked)
	err = inSchool(t, db, school, func(ctx context.Context) error {
		_, err := caterers.CreateCatererInvitation(ctx, identityaccess.CatererInvitationRequest{Email: "keine-adresse", CatererName: "Küche", CreatedBy: staff.ID})
		return err
	})
	require.ErrorIs(t, err, identityaccess.ErrInvalidCatererInvitation)
	_, err = caterers.CreateCatererInvitation(context.Background(), identityaccess.CatererInvitationRequest{Email: "a@b.de", CatererName: "Küche", CreatedBy: staff.ID})
	require.Error(t, err, "without the school's transaction nothing is written")
}

// A link names its school: the token of one school opens nothing at another.
func TestCatererInvitationBelongsToItsSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caterers := setupAuthFactory(t, db).Caterers()
	school := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	staff := testpkg.CreateTestAccount(t, db, "caterer-own-school")
	token := invite(t, db, caterers, school, staff.ID, fmt.Sprintf("kueche-eigen-%d@example.com", time.Now().UnixNano()))

	err := inSchool(t, db, other, func(ctx context.Context) error {
		_, err := caterers.PreviewCatererInvitation(ctx, token)
		return err
	})
	require.ErrorIs(t, err, identityaccess.ErrInvitationNotFound)
}

// A caterer's login and refresh land in the school's login log, a refresh
// ends with the last supply, and a school changes its caterer only after
// ending the current supply.
func TestCatererSessionsAreLoggedAndEndWithTheLastSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := setupAuthFactory(t, db)
	caterers := module.Caterers()
	school := testpkg.Tenant(t)
	staff := testpkg.CreateTestAccount(t, db, "caterer-logged")
	email := fmt.Sprintf("kueche-log-%d@example.com", time.Now().UnixNano())
	acceptance, err := accept(t, db, caterers, school, invite(t, db, caterers, school, staff.ID, email), identityaccess.CatererRegistration{Password: catererPassword, ConfirmPassword: catererPassword})
	require.NoError(t, err)
	forgetCaterer(t, db, acceptance)
	t.Cleanup(func() {
		_, err := db.NewRaw(`DELETE FROM audit.auth_events WHERE account_id = ?`, acceptance.AccountID).Exec(context.Background())
		require.NoError(t, err)
	})

	_, refresh, err := caterers.LoginCatererWithAudit(context.Background(), email, catererPassword, "192.0.2.10", "test")
	require.NoError(t, err)
	_, refresh, err = module.RefreshTokenWithAudit(context.Background(), refresh, "", "192.0.2.10", "test")
	require.NoError(t, err)
	var logged []struct {
		TenantID  int64  `bun:"tenant_id"`
		EventType string `bun:"event_type"`
	}
	require.NoError(t, db.NewRaw(`SELECT tenant_id, event_type FROM audit.auth_events WHERE account_id = ? AND success ORDER BY id`, acceptance.AccountID).Scan(context.Background(), &logged))
	require.Len(t, logged, 2, "login and refresh are logged")
	assert.Equal(t, school, logged[0].TenantID)

	err = inSchool(t, db, school, func(ctx context.Context) error {
		_, err := caterers.CreateCatererInvitation(ctx, identityaccess.CatererInvitationRequest{Email: "andere@example.com", CatererName: "Andere Küche", CreatedBy: staff.ID})
		return err
	})
	require.ErrorIs(t, err, identityaccess.ErrCatererAlreadySupplies)

	require.NoError(t, inSchool(t, db, school, func(ctx context.Context) error {
		_, err := caterers.EndCatererSchool(ctx, staff.ID)
		return err
	}))
	_, _, err = module.RefreshTokenWithAudit(context.Background(), refresh, "", "", "")
	require.Error(t, err, "without a school the session cannot be renewed")
}

// ADR 0077: the caterer portal's reset serves caterer accounts only, the
// staff portal's serves no caterer account, and the new password signs the
// caterer in.
func TestCatererPasswordResetServesCaterersOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := setupAuthFactory(t, db)
	caterers := module.Caterers()
	school := testpkg.Tenant(t)
	staff := testpkg.CreateTestAccount(t, db, "caterer-reset-staff")
	email := fmt.Sprintf("kueche-reset-%d@example.com", time.Now().UnixNano())
	acceptance, err := accept(t, db, caterers, school, invite(t, db, caterers, school, staff.ID, email), identityaccess.CatererRegistration{Password: catererPassword, ConfirmPassword: catererPassword})
	require.NoError(t, err)
	forgetCaterer(t, db, acceptance)

	staffLink, err := module.InitiatePasswordReset(context.Background(), staff.Email, identityaccess.PasswordResetScopeCaterer)
	require.NoError(t, err, "another account answers like a caterer")
	assert.Nil(t, staffLink)
	catererStaffLink, err := module.InitiatePasswordReset(context.Background(), email, identityaccess.PasswordResetScopeStaff)
	require.NoError(t, err, "a caterer answers the staff portal like an unknown address")
	assert.Nil(t, catererStaffLink, "the staff portal sends no link to a caterer account")

	link, err := module.InitiatePasswordReset(context.Background(), email, identityaccess.PasswordResetScopeCaterer)
	require.NoError(t, err)
	require.NotNil(t, link)
	renewed := testpkg.NewTestPassword()
	require.NoError(t, module.ResetPassword(context.Background(), link.Token, renewed))
	_, _, err = caterers.LoginCatererWithAudit(context.Background(), email, renewed, "", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.NewRaw(`DELETE FROM platform.email_outbox WHERE tenant_id IS NULL AND related_entity_type = 'password_reset' AND related_entity_id = ?`, link.ID).Exec(context.Background())
		require.NoError(t, err)
		_, err = db.NewRaw(`DELETE FROM auth.password_reset_tokens WHERE account_id = ?`, acceptance.AccountID).Exec(context.Background())
		require.NoError(t, err)
		_, err = db.NewRaw(`DELETE FROM audit.auth_events WHERE account_id = ?`, acceptance.AccountID).Exec(context.Background())
		require.NoError(t, err)
	})
}

// ADR 0077: a caterer account joins no school: a staff invitation to its
// address answers like an address that is taken.
func TestCatererAccountJoinsNoSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	factory := setupAuthFactory(t, db)
	caterers := factory.Caterers()
	school := testpkg.Tenant(t)
	staff := testpkg.CreateTestAccount(t, db, "caterer-joins-no-school")
	email := fmt.Sprintf("kueche-schule-%d@example.com", time.Now().UnixNano())
	acceptance, err := accept(t, db, caterers, school, invite(t, db, caterers, school, staff.ID, email), identityaccess.CatererRegistration{Password: catererPassword, ConfirmPassword: catererPassword})
	require.NoError(t, err)
	forgetCaterer(t, db, acceptance)

	invitations := &fixtureOwnedInvitationService{InvitationCapability: factory, t: t, db: db}
	role := testpkg.CreateTestRoleForTenant(t, db, "caterer-staff-role", school)
	invitation, err := invitations.CreateSchoolInvitation(testpkg.TenantContext(school), identityaccess.SchoolInvitationRequest{
		Email: email, RoleID: role.ID, TenantID: school, CreatedBy: staff.ID,
		FirstName: testpkg.StrPtr("Küche"), LastName: testpkg.StrPtr("Markt"),
	})
	require.NoError(t, err)
	_, err = invitations.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{
		FirstName: "Küche", LastName: "Markt", Password: catererPassword, ConfirmPassword: catererPassword,
	})
	require.ErrorIs(t, err, identityaccess.ErrEmailAlreadyExists)
}

// Refreshing must use the same live schools as login and the portal. A
// rejected rotation leaves the refresh token usable for another live school.
func TestCatererRefreshRequiresLiveSchool(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"inactive", "deleted"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			module := setupAuthFactory(t, db)
			caterers := module.Caterers()
			first := testpkg.Tenant(t)
			staff := testpkg.CreateTestAccount(t, db, "caterer-refresh-liveness")
			email := fmt.Sprintf("refresh-live-%d@example.com", time.Now().UnixNano())
			registration := identityaccess.CatererRegistration{Password: catererPassword, ConfirmPassword: catererPassword}
			accepted, err := accept(t, db, caterers, first, invite(t, db, caterers, first, staff.ID, email), registration)
			require.NoError(t, err)
			forgetCaterer(t, db, accepted)
			_, refresh, err := caterers.LoginCatererWithAudit(context.Background(), email, catererPassword, "", "")
			require.NoError(t, err)
			if state == "inactive" {
				_, err = db.ExecContext(context.Background(), `UPDATE platform.schools SET active = false WHERE id = ?`, first)
			} else {
				_, err = db.ExecContext(context.Background(), `UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, first)
			}
			require.NoError(t, err)
			_, _, err = module.RefreshTokenWithAudit(context.Background(), refresh, "", "", "")
			require.ErrorIs(t, err, identityaccess.ErrCatererWithoutSchool)

			second, _ := testpkg.CreateTestTenant(t, db)
			_, err = accept(t, db, caterers, second, invite(t, db, caterers, second, staff.ID, email), registration)
			require.NoError(t, err)
			_, _, err = module.RefreshTokenWithAudit(context.Background(), refresh, "", "", "")
			require.NoError(t, err, "a failed rotation must not consume the token")
			var pinned int64
			require.NoError(t, db.NewRaw(`SELECT tenant_id FROM auth.tokens WHERE account_id = ? ORDER BY id DESC LIMIT 1`, accepted.AccountID).Scan(context.Background(), &pinned))
			assert.Equal(t, second, pinned, "the successor session is pinned to a live school")
		})
	}
}

// The caterer portal workflow owns the supply lock (ADR 0067 addendum,
// condition 3): invitation, acceptance and the end of the supply in
// Identity & Access take no lock of their own.
func TestCatererLifecycleTakesNoLockOfItsOwn(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caterers := setupAuthFactory(t, db).Caterers()
	school := testpkg.Tenant(t)
	staff := testpkg.CreateTestAccount(t, db, "caterer-lifecycle-lock")
	free := func() {
		t.Helper()
		competing, err := db.BeginTx(context.Background(), nil)
		require.NoError(t, err)
		defer func() { require.NoError(t, competing.Rollback()) }()
		var acquired bool
		require.NoError(t, competing.NewRaw("SELECT pg_try_advisory_xact_lock(hashtextextended(?, 0))", fmt.Sprintf("caterer-school:%d", school)).Scan(context.Background(), &acquired))
		assert.True(t, acquired, "the workflow, not identity, takes the school's supply lock")
	}
	token := invite(t, db, caterers, school, staff.ID, fmt.Sprintf("lifecycle-%d@example.com", time.Now().UnixNano()))
	require.NoError(t, inSchool(t, db, school, func(ctx context.Context) error {
		acceptance, err := caterers.AcceptCatererInvitation(ctx, token, identityaccess.CatererRegistration{Password: catererPassword, ConfirmPassword: catererPassword})
		require.NoError(t, err)
		forgetCaterer(t, db, acceptance)
		free()
		return nil
	}))
	require.NoError(t, inSchool(t, db, school, func(ctx context.Context) error {
		ended, err := caterers.EndCatererSchool(ctx, staff.ID)
		require.NoError(t, err)
		require.True(t, ended)
		free()
		_, err = caterers.CreateCatererInvitation(ctx, identityaccess.CatererInvitationRequest{
			Email: "lifecycle-invitation@example.com", CatererName: "Küche", CreatedBy: staff.ID,
		})
		require.NoError(t, err)
		free()
		return nil
	}))
}
