package platform_test

// Additional coverage for operator MFA service branches not exercised by
// the existing tests: full email-code happy path (drives VerifyChallenge
// to the consume + audit branches), VerifyCodeForOperator JWT-less path,
// lockout / rate-limit guards, ResendChallenge, and the dispatcher-driven
// email send (drives dispatchChallengeEmail + dispatchTrustedDeviceAddedEmail).

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	authsvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// --- capturing mailer (file-local to avoid name clashes) ---

// --- shared fixture for the dispatcher-driven tests ---

func newOperatorMFAWithDispatcher(t *testing.T) (platform.OperatorMFAService, *repositories.Factory, *bun.DB, *testpkg.CapturingMailer) {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	mailer := testpkg.NewCapturingMailer()
	dispatcher := email.NewDispatcher(mailer, slog.Default())
	dispatcher.SetDefaults(1, []time.Duration{time.Millisecond})

	tokenAuth, err := authjwt.NewTokenAuthWithSecret(operatorMFATestJWTSecret)
	require.NoError(t, err)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	svc, err := platform.NewOperatorMFAService(platform.OperatorMFAServiceConfig{
		Repos:       repos,
		TokenAuth:   tokenAuth,
		Dispatcher:  dispatcher,
		DefaultFrom: email.NewEmail("Operator Tests", "ops-tests@example.test"),
		FrontendURL: "https://moto.test/",
		JWTSecret:   operatorMFATestJWTSecret,
		DB:          db,
	})
	require.NoError(t, err)
	return svc, repos, db, mailer
}

// --- Tests ---

func TestOperatorMFAService_StartChallenge_DispatchesEmail(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, db, mailer := newOperatorMFAWithDispatcher(t)

	op := testpkg.CreateTestOperator(t, db)
	require.NoError(t, svc.Enroll(ctx, op.ID))

	_, err := svc.StartChallenge(ctx, op.ID, net.ParseIP("203.0.113.7"))
	require.NoError(t, err)

	if !mailer.WaitForMessages(1, 3*time.Second) {
		t.Fatal("expected dispatcher to fire an operator MFA email")
	}
	templates := mailer.Templates()
	require.NotEmpty(t, templates)
	assert.Equal(t, "mfa-email-code.html", templates[0],
		"operator StartChallenge must dispatch the mfa-email-code template")
}

func TestOperatorMFAService_IssueTrustedDevice_DispatchesAddedEmail(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, db, mailer := newOperatorMFAWithDispatcher(t)

	op := testpkg.CreateTestOperator(t, db)
	require.NoError(t, svc.Enroll(ctx, op.ID))

	cookie, _, err := svc.IssueTrustedDevice(ctx, op.ID,
		"Mozilla/5.0 (Windows NT 10.0) Chrome/121.0", net.ParseIP("203.0.113.8"))
	require.NoError(t, err)
	require.NotEmpty(t, cookie)

	if !mailer.WaitForMessages(1, 3*time.Second) {
		t.Fatal("expected dispatcher to fire a trusted-device-added email")
	}
	templates := mailer.Templates()
	require.NotEmpty(t, templates)
	assert.Equal(t, "trusted-device-added.html", templates[0],
		"operator IssueTrustedDevice must dispatch the trusted-device-added template")
}

// --- VerifyCodeForOperator: no-active-challenge, unknown-id, wrong-code ---

func TestOperatorMFAService_VerifyCodeForOperator_NoActiveChallenge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, db := newTestOperatorMFAService(t)

	op := testpkg.CreateTestOperator(t, db)
	require.NoError(t, svc.Enroll(ctx, op.ID))

	err := svc.VerifyCodeForOperator(ctx, op.ID, "123456")
	require.Error(t, err)
	assert.ErrorIs(t, err, platform.ErrOperatorMFACodeInvalid,
		"no active challenge must surface as ErrOperatorMFACodeInvalid")
}

func TestOperatorMFAService_VerifyCodeForOperator_UnknownOperator(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, _ := newTestOperatorMFAService(t)

	err := svc.VerifyCodeForOperator(ctx, 9876543210, "123456")
	require.Error(t, err)
	assert.ErrorIs(t, err, platform.ErrOperatorMFACodeInvalid,
		"unknown operator id must map to ErrOperatorMFACodeInvalid, not a 500")
}

func TestOperatorMFAService_VerifyCodeForOperator_WrongCode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, db := newTestOperatorMFAService(t)

	op := testpkg.CreateTestOperator(t, db)
	require.NoError(t, svc.Enroll(ctx, op.ID))

	_, err := svc.StartChallenge(ctx, op.ID, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)

	err = svc.VerifyCodeForOperator(ctx, op.ID, "000000")
	require.Error(t, err)
	assert.ErrorIs(t, err, platform.ErrOperatorMFACodeInvalid)
}

// --- VerifyChallenge: token rejected when scope is the tenant one ---

func TestOperatorMFAService_VerifyChallenge_RejectsTenantScopeToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, db := newTestOperatorMFAService(t)

	op := testpkg.CreateTestOperator(t, db)

	tokenAuth, err := authjwt.NewTokenAuthWithSecret(operatorMFATestJWTSecret)
	require.NoError(t, err)
	// Mint a tenant-scope challenge JWT and pass it to the operator service —
	// the operator path must reject anything that isn't MFAChallengeScopePlatform.
	tenantToken, err := tokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
		AccountID: op.ID,
		Scope:     authjwt.MFAChallengeScopeTenant,
		TenantID:  70010001, // sentinel — operator service never reads tenant_id, claim is just plumbing
	}, time.Minute)
	require.NoError(t, err)

	_, err = svc.VerifyChallenge(ctx, tenantToken, "123456")
	require.Error(t, err)
	assert.ErrorIs(t, err, platform.ErrOperatorMFAChallengeTokenInvalid,
		"a tenant-scope challenge token must be refused on the operator path so cross-realm tokens can't redeem")
}

// --- ResendChallenge: invalid token, wrong-scope, happy path ---

func TestOperatorMFAService_ResendChallenge_InvalidToken(t *testing.T) {
	t.Parallel()
	svc, _, _ := newTestOperatorMFAService(t)
	renewed, err := svc.ResendChallenge(context.Background(), "not-a-jwt", net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Empty(t, renewed)
	assert.ErrorIs(t, err, platform.ErrOperatorMFAChallengeTokenInvalid)
}

func TestOperatorMFAService_ResendChallenge_WrongScopeRejected(t *testing.T) {
	t.Parallel()
	tokenAuth, err := authjwt.NewTokenAuthWithSecret(operatorMFATestJWTSecret)
	require.NoError(t, err)
	tenantToken, err := tokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
		AccountID: 4242,
		Scope:     authjwt.MFAChallengeScopeTenant,
	}, time.Minute)
	require.NoError(t, err)

	svc, _, _ := newTestOperatorMFAService(t)
	renewed, err := svc.ResendChallenge(context.Background(), tenantToken, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Empty(t, renewed)
	assert.ErrorIs(t, err, platform.ErrOperatorMFAChallengeTokenInvalid,
		"resending a tenant-scope token via the operator path must fail")
}

func TestOperatorMFAService_ResendChallenge_HappyPathDispatcher(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, db := newTestOperatorMFAService(t)

	op := testpkg.CreateTestOperator(t, db)
	require.NoError(t, svc.Enroll(ctx, op.ID))

	tok, err := svc.StartChallenge(ctx, op.ID, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	renewed, err := svc.ResendChallenge(ctx, tok, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	// JWT iat has 1-second granularity; two consecutive StartChallenge
	// calls inside the same second produce byte-equal tokens. The
	// contract the resend fix established is "returns a usable token",
	// not "returns a different one".
	assert.NotEmpty(t, renewed, "happy path must return a renewed challenge JWT")
}

// --- Rate-limit + lockout: design-doc §11 mirror tests ---

func TestOperatorMFAService_StartChallenge_RateLimitAfter3Codes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, db := newTestOperatorMFAService(t)

	op := testpkg.CreateTestOperator(t, db)
	require.NoError(t, svc.Enroll(ctx, op.ID))

	ip := net.ParseIP("203.0.113.30")
	for i := 0; i < platform.OperatorMFARateLimitMaxSent; i++ {
		_, err := svc.StartChallenge(ctx, op.ID, ip)
		require.NoErrorf(t, err, "operator code %d must still be permitted", i+1)
	}
	_, err := svc.StartChallenge(ctx, op.ID, ip)
	assert.ErrorIs(t, err, platform.ErrOperatorMFARateLimited)
}

func TestOperatorMFAService_VerifyChallenge_LockoutAfter5Failures(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, db := newTestOperatorMFAService(t)

	op := testpkg.CreateTestOperator(t, db)
	require.NoError(t, svc.Enroll(ctx, op.ID))

	challenge, err := svc.StartChallenge(ctx, op.ID, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)

	for i := 0; i < platform.OperatorMFALockoutThreshold; i++ {
		_, verr := svc.VerifyChallenge(ctx, challenge, "000000")
		assert.ErrorIs(t, verr, platform.ErrOperatorMFACodeInvalid,
			"wrong code attempt %d must surface as code-invalid until the threshold is hit", i+1)
	}
	// 6th attempt — the operator must now be locked instead of seeing "code invalid".
	_, err = svc.VerifyChallenge(ctx, challenge, "000000")
	assert.ErrorIs(t, err, platform.ErrOperatorMFALocked)
}

// --- StartChallenge rejects unknown operator id ---

func TestOperatorMFAService_StartChallenge_UnknownOperator_Rejected(t *testing.T) {
	t.Parallel()
	svc, _, _ := newTestOperatorMFAService(t)
	_, err := svc.StartChallenge(context.Background(), 9876543210, net.ParseIP("127.0.0.1"))
	require.Error(t, err, "unknown operator id must surface as an error, not silently mint a code")
}

// --- VerifyCodeForOperator happy path (drives the consume + reset branches) ---

func TestOperatorMFAService_VerifyCodeForOperator_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, repos, db := newTestOperatorMFAService(t)

	op := testpkg.CreateTestOperator(t, db)
	require.NoError(t, svc.Enroll(ctx, op.ID))

	_, err := svc.StartChallenge(ctx, op.ID, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)

	// Read the active challenge directly to learn its plaintext-equivalent code.
	// We can't read the plaintext; instead synthesize a fresh challenge whose
	// hash we control by writing a known code via the auth.HashShortCode helper.
	active, err := repos.OperatorMFAEmailChallenge.FindActiveByOperatorID(ctx, op.ID)
	require.NoError(t, err)
	require.NotNil(t, active)

	const knownCode = "424242"
	hashed, hashErr := authsvc.HashShortCode(knownCode)
	require.NoError(t, hashErr)
	active.CodeHash = hashed
	// Persist the substituted hash so the verify path finds the matching code.
	_, err = db.NewUpdate().
		Model(active).
		ModelTableExpr("platform.operator_mfa_email_challenges").
		Set("code_hash = ?", hashed).
		Where("id = ?", active.ID).
		Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, svc.VerifyCodeForOperator(ctx, op.ID, knownCode))
}
