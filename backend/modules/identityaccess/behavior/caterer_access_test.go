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

const catererPassword = "Kuechenpass-2026!"

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
		acceptance, err = caterers.AcceptCatererInvitation(ctx, token, registration, func(context.Context, identityaccess.CatererAcceptance) error { return nil })
		return err
	})
	return acceptance, err
}

// ADR 0074: a school invites its caterer, the caterer accepts with a
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

	var started identityaccess.CatererAcceptance
	var acceptance identityaccess.CatererAcceptance
	require.NoError(t, inSchool(t, db, school, func(ctx context.Context) error {
		var err error
		acceptance, err = caterers.AcceptCatererInvitation(ctx, token, identityaccess.CatererRegistration{Password: catererPassword, ConfirmPassword: catererPassword},
			func(_ context.Context, accepted identityaccess.CatererAcceptance) error {
				started = accepted
				return nil
			})
		return err
	}))
	forgetCaterer(t, db, acceptance)
	assert.Equal(t, school, acceptance.TenantID)
	assert.Equal(t, "Schulstraße 1", acceptance.DeliveryAddress)
	assert.Equal(t, acceptance, started, "the supply starts in the acceptance")
	_, err := accept(t, db, caterers, school, token, identityaccess.CatererRegistration{Password: catererPassword, ConfirmPassword: catererPassword})
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
		ended, err := caterers.EndCatererSchool(ctx)
		assert.True(t, ended)
		return err
	}))
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
	again, err := accept(t, db, caterers, second, secondToken, identityaccess.CatererRegistration{Password: catererPassword})
	require.NoError(t, err)
	assert.Equal(t, acceptance.AccountID, again.AccountID, "no second account")
	assert.Equal(t, acceptance.CatererID, again.CatererID)

	_, schools, err := caterers.CatererSchools(context.Background(), acceptance.AccountID)
	require.NoError(t, err)
	assert.Equal(t, []int64{first, second}, schools)
	t.Cleanup(func() {
		_, _ = db.NewRaw(`DELETE FROM audit.auth_events WHERE account_id = ?`, acceptance.AccountID).Exec(context.Background())
	})
	_, refresh, err := caterers.LoginCatererWithAudit(context.Background(), email, catererPassword, "192.0.2.11", "test")
	require.NoError(t, err)

	require.NoError(t, inSchool(t, db, first, func(ctx context.Context) error {
		_, err := caterers.EndCatererSchool(ctx)
		return err
	}))
	_, schools, err = caterers.CatererSchools(context.Background(), acceptance.AccountID)
	require.NoError(t, err)
	assert.Equal(t, []int64{second}, schools, "the first school's end leaves the second")

	// The session pinned to the first school moves to the second on refresh,
	// so the first school no longer logs the caterer's sign-ins.
	_, _, err = module.RefreshTokenWithAudit(context.Background(), refresh, "", "192.0.2.11", "test")
	require.NoError(t, err)
	var pinned, refreshedAt int64
	require.NoError(t, db.NewRaw(`SELECT tenant_id FROM auth.tokens WHERE account_id = ? ORDER BY id DESC LIMIT 1`, acceptance.AccountID).Scan(context.Background(), &pinned))
	assert.Equal(t, second, pinned)
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
	staffPassword := "Lehrkraft-2026!"
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
		_, _ = db.NewRaw(`DELETE FROM audit.auth_events WHERE account_id = ?`, acceptance.AccountID).Exec(context.Background())
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
		_, err := caterers.EndCatererSchool(ctx)
		return err
	}))
	_, _, err = module.RefreshTokenWithAudit(context.Background(), refresh, "", "", "")
	require.Error(t, err, "without a school the session cannot be renewed")
}

// ADR 0074: the caterer portal's reset serves caterer accounts only, and the
// new password signs the caterer in.
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

	link, err := module.InitiatePasswordReset(context.Background(), email, identityaccess.PasswordResetScopeCaterer)
	require.NoError(t, err)
	require.NotNil(t, link)
	const renewed = "Neues-Kuechenpass-2026!"
	require.NoError(t, module.ResetPassword(context.Background(), link.Token, renewed))
	_, _, err = caterers.LoginCatererWithAudit(context.Background(), email, renewed, "", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.NewRaw(`DELETE FROM platform.email_outbox WHERE tenant_id IS NULL AND related_entity_type = 'password_reset' AND related_entity_id = ?`, link.ID).Exec(context.Background())
		_, _ = db.NewRaw(`DELETE FROM auth.password_reset_tokens WHERE account_id = ?`, acceptance.AccountID).Exec(context.Background())
		_, _ = db.NewRaw(`DELETE FROM audit.auth_events WHERE account_id = ?`, acceptance.AccountID).Exec(context.Background())
	})
}
