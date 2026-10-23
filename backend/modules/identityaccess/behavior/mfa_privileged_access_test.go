package behavior_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrivilegedMFAIsSchoolScopedAndIgnoresLegacyExemptions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := newMFATestModule(t, db, withDeliveringMailer(nil)...)
	service := newFixtureAuthService(t, db, module.Auth)
	email, accountID, schoolA, slugA := registerAccountAtTenant(t, db, service, "mfa-floor")
	schoolB, slugB := newSchoolTenant(t, db)
	testpkg.EnsureAccountTenant(t, db, accountID, schoolB)
	testpkg.GrantTestPermission(t, db, schoolA, accountID, "student_documents:health")
	// Arrange a pre-cutover exemption, before the new write guard existed.
	_, err := db.ExecContext(context.Background(), `INSERT INTO auth.mfa_overrides
  (account_id, tenant_id, override, set_by, set_by_type, reason)
  VALUES (?, ?, 'force_off', ?, 'account', 'legacy exemption fixture')`, accountID, schoolA, accountID)
	require.NoError(t, err)
	resultA, err := service.LoginWithMFAGate(context.Background(), email, testPassword, "", "", slugA, "")
	require.NoError(t, err)
	assert.Equal(t, identityaccess.LoginStatusMFAEnrollmentRequired, resultA.Status)
	assert.Empty(t, resultA.RefreshToken)
	resultB, err := service.LoginWithMFAGate(context.Background(), email, testPassword, "", "", slugB, "")
	require.NoError(t, err)
	assert.Equal(t, identityaccess.LoginStatusAuthenticated, resultB.Status, "rights at A must not impose school A's staff authority on B")
	_, successor, err := service.RefreshTokenWithAudit(context.Background(), resultB.RefreshToken, "", "", "")
	require.NoError(t, err)
	assert.NotEmpty(t, successor)
	_, _, err = service.SwitchTenant(context.Background(), accountID, slugA, "")
	require.ErrorIs(t, err, identityaccess.ErrInvalidToken, "switching schools cannot turn an unverified session into privileged access")
	require.ErrorIs(t, module.MFA.SetMFAOverride(context.Background(), accountID, schoolA, accountID, identityaccess.MFAAdminOverrideForceOff, "test refusal", []string{"users:manage"}), identityaccess.ErrMFAPermissionDenied)
	operator := testpkg.CreateTestOperator(t, db)
	require.ErrorIs(t, module.MFA.OperatorSetGlobalMFAOverride(context.Background(), operator.ID, accountID, identityaccess.MFAAdminOverrideForceOff, "test refusal"), identityaccess.ErrMFAPermissionDenied)
}

func TestPrivilegedMFAVerifiedSessionRotatesButResetRemovesProof(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := newMFATestModule(t, db, withDeliveringMailer(nil)...)
	service := newFixtureAuthService(t, db, module.Auth)
	email, accountID, school, slug := registerAccountAtTenant(t, db, service, "mfa-proof")
	_, legacyRefresh, err := service.LoginWithAudit(context.Background(), email, testPassword, "", "", slug)
	require.NoError(t, err)
	testpkg.GrantTestPermission(t, db, school, accountID, "staff:financial")
	access, successor, err := service.RefreshTokenWithAudit(context.Background(), legacyRefresh, "", "", "")
	require.ErrorIs(t, err, identityaccess.ErrInvalidToken)
	assert.Empty(t, access)
	assert.Empty(t, successor)
	// This capability is called only after a verified email/passkey exchange.
	_, verifiedRefresh, err := service.IssueTokensForAuthenticatedAccount(context.Background(), accountID, school, "", "")
	require.NoError(t, err)
	for range 3 {
		_, verifiedRefresh, err = service.RefreshTokenWithAudit(context.Background(), verifiedRefresh, "", "", "")
		require.NoError(t, err, "MFA proof must survive every rotation")
	}
	require.NoError(t, module.MFA.DisableMFA(testpkg.TenantContext(school), accountID))
	access, successor, err = service.RefreshTokenWithAudit(context.Background(), verifiedRefresh, "", "", "")
	require.ErrorIs(t, err, identityaccess.ErrInvalidToken, "resetting the factor must invalidate proof of all existing sessions")
	assert.Empty(t, access)
	assert.Empty(t, successor)
	result, err := service.LoginWithMFAGate(context.Background(), email, testPassword, "", "", slug, "")
	require.NoError(t, err)
	assert.Equal(t, identityaccess.LoginStatusMFAEnrollmentRequired, result.Status)
}

func TestSchoolPrivilegedMFAEnrollmentPrecedesTrustedDevice(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := newMFATestModule(t, db, withDeliveringMailer(nil)...)
	service := newFixtureAuthService(t, db, module.Auth)
	school, _ := newSchoolTenant(t, db)
	email, accountID := createLehrkraftAccount(t, db, service, "school-health-mfa", school)
	testpkg.GrantTestPermission(t, db, school, accountID, "staff_documents:health")
	ctx := context.Background()
	cookie, _, err := module.MFA.IssueTrustedDevice(ctx, accountID, school, "test-device", nil)
	require.NoError(t, err)
	require.NotEmpty(t, cookie)
	result, err := service.LoginSchoolWithMFAGate(ctx, email, testPassword, "", "", cookie)
	require.NoError(t, err)
	assert.Equal(t, identityaccess.LoginStatusMFAEnrollmentRequired, result.Status, "old device proof must not skip missing enrollment")
	require.NoError(t, module.MFA.EnrollMFA(testpkg.TenantContext(school), accountID))
	result, err = service.LoginSchoolWithMFAGate(ctx, email, testPassword, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, identityaccess.LoginStatusMFARequired, result.Status)
	result, err = service.LoginSchoolWithMFAGate(ctx, email, testPassword, "", "", cookie)
	require.NoError(t, err)
	assert.Equal(t, identityaccess.LoginStatusAuthenticated, result.Status)
	_, _, err = service.RefreshTokenWithAudit(ctx, result.RefreshToken, "", "", "school")
	require.NoError(t, err, "trusted-device authentication must persist proof for school refresh")
}

func TestMFABestandQueryCountsOnlyPrivilegesOfTheRespectiveSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	_, accountID, schoolA, _ := registerAccountAtTenant(t, db, service, "mfa-counts")
	schoolB, _ := newSchoolTenant(t, db)
	testpkg.EnsureAccountTenant(t, db, accountID, schoolB)
	testpkg.GrantTestPermission(t, db, schoolA, accountID, "staff:financial")
	query, err := os.ReadFile("../../../../docs/operations/sql/mfa-pflicht-3841-counts.sql")
	require.NoError(t, err)
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback()) }()
	var rows []struct {
		SchoolID               int64
		PrivilegedAccounts     int
		WithoutEmailMfa        int
		WithoutAnySecondFactor int
		WithoutEmailAddress    int
	}
	require.NoError(t, tx.NewRaw(string(query)).Scan(context.Background(), &rows))
	counts := make(map[int64]int)
	for _, row := range rows {
		counts[row.SchoolID] = row.WithoutAnySecondFactor
	}
	assert.Equal(t, 1, counts[schoolA])
	assert.Zero(t, counts[schoolB])
}

func TestTenantTrustedDeviceRevokedDuringLoginCannotMintProof(t *testing.T) {
	t.Parallel()
	checkRevokedDeviceAtMint(t, false)
}

func TestSchoolTrustedDeviceRevokedDuringLoginCannotMintProof(t *testing.T) {
	t.Parallel()
	checkRevokedDeviceAtMint(t, true)
}

func checkRevokedDeviceAtMint(t *testing.T, schoolPortal bool) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	checks := 0
	service := newGatedAuthService(t, db, &MFAServiceMock{
		IsRequiredFn:       func(context.Context, int64, []string, int64) (bool, error) { return true, nil },
		HasMFAEnrollmentFn: func(context.Context, int64) (bool, error) { return true, nil },
		VerifyTrustedDeviceFn: func(context.Context, int64, int64, string) (bool, error) {
			checks++
			return checks == 1, nil // trust is revoked before the account lock is held
		},
	})
	var result *identityaccess.LoginResult
	var err error
	if schoolPortal {
		school, _ := newSchoolTenant(t, db)
		email, _ := createLehrkraftAccount(t, db, service, "school-trust-race", school)
		result, err = service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "revoked-cookie-placeholder")
	} else {
		email, _, _, slug := registerAccountAtTenant(t, db, service, "tenant-trust-race")
		result, err = service.LoginWithMFAGate(context.Background(), email, testPassword, "", "", slug, "revoked-cookie-placeholder")
	}
	require.ErrorIs(t, err, identityaccess.ErrInvalidToken)
	assert.Nil(t, result, "revoked device proof must never create a completed login")
}
