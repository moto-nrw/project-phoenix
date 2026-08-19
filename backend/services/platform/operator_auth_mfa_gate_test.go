package platform_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/models/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// operatorGatePassword is generated at runtime so no plaintext literal
// appears in the source tree. The fixed "Aa1!" prefix guarantees the
// strength rules (upper + lower + digit + special) are satisfied
// regardless of the hex-randomness that follows.
var operatorGatePassword = randomOperatorGatePassword()

func randomOperatorGatePassword() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return "Aa1!" + hex.EncodeToString(b)
}

// stubOperatorMFAService is a minimal OperatorMFAService implementation
// covering only the methods LoginWithMFAGate calls (HasEnrollment,
// VerifyTrustedDevice, StartChallenge). All other methods panic so tests
// surface accidental wiring changes loudly.
type stubOperatorMFAService struct {
	hasEnrollmentFn       func(ctx context.Context, operatorID int64) (bool, error)
	verifyTrustedDeviceFn func(ctx context.Context, operatorID int64, cookie string) (bool, error)
	startChallengeFn      func(ctx context.Context, operatorID int64, ip net.IP) (string, error)
}

func (s *stubOperatorMFAService) HasEnrollment(ctx context.Context, operatorID int64) (bool, error) {
	if s.hasEnrollmentFn != nil {
		return s.hasEnrollmentFn(ctx, operatorID)
	}
	return false, nil
}

func (s *stubOperatorMFAService) VerifyTrustedDevice(ctx context.Context, operatorID int64, cookie string) (bool, error) {
	if s.verifyTrustedDeviceFn != nil {
		return s.verifyTrustedDeviceFn(ctx, operatorID, cookie)
	}
	return false, nil
}

func (s *stubOperatorMFAService) StartChallenge(ctx context.Context, operatorID int64, ip net.IP) (string, error) {
	if s.startChallengeFn != nil {
		return s.startChallengeFn(ctx, operatorID, ip)
	}
	return "test-challenge-token", nil
}

// Remaining interface methods panic — the LoginWithMFAGate flow must not
// touch them. If a future refactor pulls them in, the panic will flag it.
func (s *stubOperatorMFAService) VerifyChallenge(context.Context, string, string) (*platformSvc.OperatorVerifiedChallenge, error) {
	panic("unexpected VerifyChallenge")
}
func (s *stubOperatorMFAService) ResendChallenge(context.Context, string, net.IP) (string, error) {
	panic("unexpected ResendChallenge")
}
func (s *stubOperatorMFAService) VerifyCodeForOperator(context.Context, int64, string) error {
	panic("unexpected VerifyCodeForOperator")
}
func (s *stubOperatorMFAService) Enroll(context.Context, int64) error  { panic("unexpected Enroll") }
func (s *stubOperatorMFAService) Disable(context.Context, int64) error { panic("unexpected Disable") }
func (s *stubOperatorMFAService) IssueTrustedDevice(context.Context, int64, string, net.IP) (string, time.Time, error) {
	panic("unexpected IssueTrustedDevice")
}
func (s *stubOperatorMFAService) ListTrustedDevices(context.Context, int64) ([]*platform.OperatorMFATrustedDevice, error) {
	panic("unexpected ListTrustedDevices")
}
func (s *stubOperatorMFAService) RevokeTrustedDevice(context.Context, int64, int64) error {
	panic("unexpected RevokeTrustedDevice")
}

// newOperatorAuthServiceForGate builds an operatorAuthService backed by the
// stubs above plus the existing shared mockOperatorRepo / mockAuditLogRepoShared.
// Returns the service plus the active operator the FindByEmail stub serves.
func newOperatorAuthServiceForGate(t *testing.T, mfa *stubOperatorMFAService) (platformSvc.OperatorAuthService, *platform.Operator) {
	t.Helper()
	hash, err := userpass.HashPassword(operatorGatePassword, nil)
	require.NoError(t, err)

	op := &platform.Operator{
		Email:        "gate-test@example.com",
		PasswordHash: hash,
		Active:       true,
	}
	op.ID = 4242 // model has embedded base.Model; setting ID directly via the field

	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(_ context.Context, email string) (*platform.Operator, error) {
			if strings.EqualFold(strings.TrimSpace(email), op.Email) {
				return op, nil
			}
			return nil, nil
		},
		updateLastLoginFn: func(context.Context, int64) error { return nil },
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	svc, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo:     operatorRepo,
		AuditLogRepo:     auditLogRepo,
		RefreshTokenRepo: &mockOperatorRefreshTokenRepo{},
		DB:               &bun.DB{},
		Logger:           slog.Default(),
	})
	require.NoError(t, err)
	if mfa != nil {
		svc.SetMFAService(mfa)
	}
	return svc, op
}

// Deliberately NOT parallel: platform announcements and operators are
// tenant-less. The fixtures reuse fixed operator e-mails and the assertions
// count rows the whole clone shares, so two of these tests running side by
// side see each other's data.
func TestOperatorLoginWithMFAGate_NoMFAService_ReturnsTokens(t *testing.T) {
	svc, op := newOperatorAuthServiceForGate(t, nil)

	result, err := svc.LoginWithMFAGate(
		context.Background(), op.Email, operatorGatePassword,
		"127.0.0.1", "ua-test", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, platformSvc.OperatorLoginStatusAuthenticated, result.Status)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	assert.Empty(t, result.ChallengeToken)
	assert.False(t, result.MFAEnrollmentRequired)
}

// Deliberately NOT parallel: platform announcements and operators are
// tenant-less. The fixtures reuse fixed operator e-mails and the assertions
// count rows the whole clone shares, so two of these tests running side by
// side see each other's data.
func TestOperatorLoginWithMFAGate_NotEnrolled_IssuesEnrollmentToken(t *testing.T) {
	// Post-#1430 review (Item #1) contract: an unenrolled operator must
	// receive a narrow enrollment-scoped JWT (no refresh token), NOT a
	// full operator session. The previous design returned `authenticated`
	// + MFAEnrollmentRequired flag — an advisory-only signal that any
	// direct API client could ignore, bypassing the mandatory operator
	// MFA entirely.
	mfa := &stubOperatorMFAService{
		hasEnrollmentFn: func(context.Context, int64) (bool, error) { return false, nil },
	}
	svc, op := newOperatorAuthServiceForGate(t, mfa)

	result, err := svc.LoginWithMFAGate(
		context.Background(), op.Email, operatorGatePassword,
		"127.0.0.1", "ua-test", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, platformSvc.OperatorLoginStatusMFAEnrollmentRequired, result.Status,
		"unenrolled operators must receive the narrow enrollment status, not a full session")
	assert.NotEmpty(t, result.AccessToken,
		"AccessToken carries the enrollment-scoped JWT for /operator/auth/mfa/enroll/*")
	assert.Empty(t, result.RefreshToken,
		"no refresh token before MFA enrollment — closes the pre-MFA-bypass hole")
	assert.True(t, result.MFAEnrollmentRequired,
		"MFAEnrollmentRequired stays as a legacy hint for clients that branch on the boolean")
	assert.Empty(t, result.ChallengeToken, "no challenge before enrollment")
	assert.NotEmpty(t, result.MaskedEmail, "enrollment screen renders the masked address")
	require.NotNil(t, result.Operator)
}

// Deliberately NOT parallel: platform announcements and operators are
// tenant-less. The fixtures reuse fixed operator e-mails and the assertions
// count rows the whole clone shares, so two of these tests running side by
// side see each other's data.
func TestOperatorLoginWithMFAGate_EnrolledNoCookie_ReturnsChallenge(t *testing.T) {
	mfa := &stubOperatorMFAService{
		hasEnrollmentFn:  func(context.Context, int64) (bool, error) { return true, nil },
		startChallengeFn: func(context.Context, int64, net.IP) (string, error) { return "challenge-xyz", nil },
	}
	svc, op := newOperatorAuthServiceForGate(t, mfa)

	result, err := svc.LoginWithMFAGate(
		context.Background(), op.Email, operatorGatePassword,
		"127.0.0.1", "ua-test", "", // no trusted-device cookie
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, platformSvc.OperatorLoginStatusMFARequired, result.Status)
	assert.Equal(t, "challenge-xyz", result.ChallengeToken)
	assert.NotEmpty(t, result.MaskedEmail)
	assert.Empty(t, result.AccessToken, "no access token until challenge verifies")
	assert.True(t, result.TrustedDeviceEnabled)
	assert.Positive(t, result.TrustedDeviceDays)
}

// Deliberately NOT parallel: platform announcements and operators are
// tenant-less. The fixtures reuse fixed operator e-mails and the assertions
// count rows the whole clone shares, so two of these tests running side by
// side see each other's data.
func TestOperatorLoginWithMFAGate_EnrolledValidCookie_SkipsMFA(t *testing.T) {
	mfa := &stubOperatorMFAService{
		hasEnrollmentFn: func(context.Context, int64) (bool, error) { return true, nil },
		verifyTrustedDeviceFn: func(_ context.Context, _ int64, cookie string) (bool, error) {
			return cookie == "valid-cookie", nil
		},
	}
	svc, op := newOperatorAuthServiceForGate(t, mfa)

	result, err := svc.LoginWithMFAGate(
		context.Background(), op.Email, operatorGatePassword,
		"127.0.0.1", "ua-test", "valid-cookie",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, platformSvc.OperatorLoginStatusAuthenticated, result.Status,
		"valid trusted-device cookie short-circuits the challenge step")
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	assert.False(t, result.MFAEnrollmentRequired)
	assert.Empty(t, result.ChallengeToken)
}

// Deliberately NOT parallel: platform announcements and operators are
// tenant-less. The fixtures reuse fixed operator e-mails and the assertions
// count rows the whole clone shares, so two of these tests running side by
// side see each other's data.
func TestOperatorLoginWithMFAGate_EnrolledInvalidCookie_FallsThroughToChallenge(t *testing.T) {
	mfa := &stubOperatorMFAService{
		hasEnrollmentFn:       func(context.Context, int64) (bool, error) { return true, nil },
		verifyTrustedDeviceFn: func(context.Context, int64, string) (bool, error) { return false, nil },
		startChallengeFn:      func(context.Context, int64, net.IP) (string, error) { return "fresh-challenge", nil },
	}
	svc, op := newOperatorAuthServiceForGate(t, mfa)

	result, err := svc.LoginWithMFAGate(
		context.Background(), op.Email, operatorGatePassword,
		"127.0.0.1", "ua-test", "stale-cookie",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, platformSvc.OperatorLoginStatusMFARequired, result.Status,
		"a non-verifiable cookie must drop the operator into the challenge flow, not skip MFA")
	assert.Equal(t, "fresh-challenge", result.ChallengeToken)
}

// Deliberately NOT parallel: platform announcements and operators are
// tenant-less. The fixtures reuse fixed operator e-mails and the assertions
// count rows the whole clone shares, so two of these tests running side by
// side see each other's data.
func TestOperatorLoginWithMFAGate_WrongPassword_NoMFACalls(t *testing.T) {
	// The stub methods panic on call; if the password gate fails before
	// the MFA branch the test passes silently.
	mfa := &stubOperatorMFAService{
		hasEnrollmentFn: func(context.Context, int64) (bool, error) {
			t.Fatal("HasEnrollment must not be invoked when password check fails")
			return false, nil
		},
	}
	svc, op := newOperatorAuthServiceForGate(t, mfa)

	_, err := svc.LoginWithMFAGate(
		context.Background(), op.Email, "wrong-password",
		"127.0.0.1", "ua-test", "",
	)
	require.Error(t, err)
}

// Deliberately NOT parallel: platform announcements and operators are
// tenant-less. The fixtures reuse fixed operator e-mails and the assertions
// count rows the whole clone shares, so two of these tests running side by
// side see each other's data.
func TestOperatorLoginWithMFAGate_UnknownEmail_ReturnsInvalidCreds(t *testing.T) {
	mfa := &stubOperatorMFAService{} // every call would panic
	svc, _ := newOperatorAuthServiceForGate(t, mfa)

	_, err := svc.LoginWithMFAGate(
		context.Background(), "nobody@example.com", operatorGatePassword,
		"127.0.0.1", "ua-test", "",
	)
	require.Error(t, err)
	var ic *platformSvc.InvalidCredentialsError
	require.ErrorAs(t, err, &ic)
}

// Deliberately NOT parallel: platform announcements and operators are
// tenant-less. The fixtures reuse fixed operator e-mails and the assertions
// count rows the whole clone shares, so two of these tests running side by
// side see each other's data.
func TestOperatorLoginWithMFAGate_InactiveOperator_ReturnsInactiveError(t *testing.T) {
	hash, err := userpass.HashPassword(operatorGatePassword, nil)
	require.NoError(t, err)

	op := &platform.Operator{
		Email:        "inactive@example.com",
		PasswordHash: hash,
		Active:       false,
	}
	op.ID = 999

	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(_ context.Context, _ string) (*platform.Operator, error) { return op, nil },
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	svc, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, err = svc.LoginWithMFAGate(
		context.Background(), op.Email, operatorGatePassword,
		"127.0.0.1", "ua-test", "",
	)
	require.Error(t, err)
	var ie *platformSvc.OperatorInactiveError
	require.ErrorAs(t, err, &ie)
}
