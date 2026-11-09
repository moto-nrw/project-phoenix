package behavior_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOwnPasswordChangeEndsEveryOtherAccess pins AU-2 (#6289): an account
// that changes its own password keeps the session it changed it in, and
// nothing else the old password could have handed out stays usable: no other
// session at any school (including a family switched out of another device),
// no reset link mailed before the change and no remembered device.
func TestOwnPasswordChangeEndsEveryOtherAccess(t *testing.T) {
	t.Parallel()
	f := newSwitchLineageFixture(t, "own-password-change")
	ctx := testpkg.Ctx(t)
	current := accessSessionOf(t, f.access)

	otherAccess, otherRefresh, err := f.service.IssueTokensForPasskey(ctx, f.accountID, f.homeTenantID, "", "")
	require.NoError(t, err)
	other := accessSessionOf(t, otherAccess)
	switchedAccess, _, err := f.switchToTarget(ctx, other.FamilyID)
	require.NoError(t, err)
	switched := accessSessionOf(t, switchedAccess)

	var email string
	require.NoError(t, f.db.NewRaw(`SELECT email FROM auth.accounts WHERE id = ?`, f.accountID).Scan(ctx, &email))
	link, err := f.module.InitiatePasswordReset(context.Background(), email, identityaccess.PasswordResetScopeStaff)
	require.NoError(t, err)
	require.NotNil(t, link)
	t.Cleanup(func() {
		// The reset mail is global identity mail without a school.
		_, cleanupErr := f.db.NewRaw(`DELETE FROM platform.email_outbox WHERE tenant_id IS NULL AND related_entity_type = 'password_reset' AND related_entity_id = ?`, link.ID).Exec(context.Background())
		require.NoError(t, cleanupErr)
	})

	mfa, _, _ := newTestMFAService(t)
	require.NoError(t, mfa.EnrollMFA(ctx, f.accountID))
	device, _, err := mfa.IssueTrustedDevice(ctx, f.accountID, f.homeTenantID, "Mozilla/5.0 (Test)", net.ParseIP("203.0.113.7"))
	require.NoError(t, err)
	require.NotEmpty(t, device)

	require.NoError(t, f.module.ChangeAccountPassword(ctx, identityaccess.OwnPasswordChange{
		AccountID: f.accountID, SessionFamilyID: current.FamilyID,
		CurrentPassword: testPassword, NewPassword: testNewPassword,
	}))

	require.NoError(t, f.module.VerifyAccessSession(ctx, current), "the session the change was made in stays signed in")
	_, _, err = f.service.RefreshToken(ctx, f.refresh)
	require.NoError(t, err, "the current session keeps renewing")

	require.ErrorIs(t, f.module.VerifyAccessSession(ctx, other), identityaccess.ErrInvalidToken, "another device's session ends")
	_, _, err = f.service.RefreshToken(ctx, otherRefresh)
	require.Error(t, err, "another device's refresh token renews nothing")
	require.ErrorIs(t, f.module.VerifyAccessSession(ctx, switched), identityaccess.ErrInvalidToken,
		"a session at another school ends")
	assert.Zero(t, familyRowCount(t, f.db, other.FamilyID))
	assert.Zero(t, familyRowCount(t, f.db, switched.FamilyID))

	require.Error(t, f.module.ResetPassword(context.Background(), link.Token, testpkg.NewTestPassword()),
		"a reset link mailed before the change overwrites nothing")
	trusted, err := mfa.VerifyTrustedDevice(ctx, f.accountID, f.homeTenantID, device)
	require.NoError(t, err)
	assert.False(t, trusted, "a remembered device no longer skips the second factor")
}

// TestOwnPasswordChangeWithoutSessionEndsEverySession covers a change that
// names no session of its own: none is kept.
func TestOwnPasswordChangeWithoutSessionEndsEverySession(t *testing.T) {
	t.Parallel()
	f := newSwitchLineageFixture(t, "own-password-change-no-session")
	ctx := testpkg.Ctx(t)

	require.NoError(t, f.module.ChangeAccountPassword(ctx, identityaccess.OwnPasswordChange{
		AccountID: f.accountID, CurrentPassword: testPassword, NewPassword: testNewPassword,
	}))

	require.ErrorIs(t, f.module.VerifyAccessSession(ctx, accessSessionOf(t, f.access)), identityaccess.ErrInvalidToken)
	assert.Empty(t, liveTokens(t, f.db, f.accountID))
}

// TestOwnPasswordChangeRefusedKeepsEveryAccess pins the other side: a
// refused change (wrong current password) ends nothing.
func TestOwnPasswordChangeRefusedKeepsEveryAccess(t *testing.T) {
	t.Parallel()
	f := newSwitchLineageFixture(t, "own-password-change-refused")
	ctx := testpkg.Ctx(t)
	other := newOtherDeviceSession(t, f)

	err := f.module.ChangeAccountPassword(ctx, identityaccess.OwnPasswordChange{
		AccountID: f.accountID, SessionFamilyID: accessSessionOf(t, f.access).FamilyID,
		CurrentPassword: testNewPassword, NewPassword: testpkg.NewTestPassword(),
	})
	require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials)

	require.NoError(t, f.module.VerifyAccessSession(ctx, other), "a refused change ends no session")
	assert.Len(t, liveTokens(t, f.db, f.accountID), 2)
}

// TestCatererPasswordChangeRevokesRememberedDevices extends #6289 to the
// caterer's own change (#4496): it already ends every session and spends the
// reset links, and a remembered device no longer skips the second factor
// either.
func TestCatererPasswordChangeRevokesRememberedDevices(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := setupAuthFactory(t, db)
	caterers := module.Caterers()
	school := testpkg.Tenant(t)
	staff := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("caterer-devices-staff-%d@example.com", time.Now().UnixNano()), testpkg.NewTestPassword())
	email := fmt.Sprintf("kueche-devices-%d@example.com", time.Now().UnixNano())
	acceptance, err := accept(t, db, caterers, school, invite(t, db, caterers, school, staff.ID, email), identityaccess.CatererRegistration{Password: catererPassword, ConfirmPassword: catererPassword})
	require.NoError(t, err)
	forgetCaterer(t, db, acceptance)
	t.Cleanup(func() {
		_, cleanupErr := db.NewRaw(`DELETE FROM audit.auth_events WHERE account_id = ?`, acceptance.AccountID).Exec(context.Background())
		require.NoError(t, cleanupErr)
	})
	ctx := testpkg.Ctx(t)
	mfa, _, _ := newTestMFAService(t)
	require.NoError(t, mfa.EnrollMFA(ctx, acceptance.AccountID))
	device, _, err := mfa.IssueTrustedDevice(ctx, acceptance.AccountID, school, "Mozilla/5.0 (Test)", net.ParseIP("203.0.113.7"))
	require.NoError(t, err)
	require.NotEmpty(t, device)

	require.NoError(t, caterers.ChangeCatererPassword(context.Background(), acceptance.AccountID, catererPassword, testpkg.NewTestPassword(), "192.0.2.20", "test"))

	trusted, err := mfa.VerifyTrustedDevice(ctx, acceptance.AccountID, school, device)
	require.NoError(t, err)
	assert.False(t, trusted, "a remembered device no longer skips the second factor")
}
