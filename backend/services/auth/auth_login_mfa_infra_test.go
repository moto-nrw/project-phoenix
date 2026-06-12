package auth_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authmodel "github.com/moto-nrw/project-phoenix/models/auth"
	authsvc "github.com/moto-nrw/project-phoenix/services/auth"
)

// stubMFAService implements authsvc.MFAService and lets a test inject error
// outcomes for IsRequired / HasEnrollment specifically. Every other method
// panics so an accidental call surfaces immediately — Item #3 only changes
// behaviour at the inquiry layer, and the tests must not silently exercise
// unrelated code paths.
type stubMFAService struct {
	isRequiredResult bool
	isRequiredErr    error
	hasEnrollmentRes bool
	hasEnrollmentErr error
}

func (s *stubMFAService) AccountBelongsToTenant(_ context.Context, _, _ int64) (bool, error) {
	return true, nil
}

func (s *stubMFAService) IsRequired(_ context.Context, _ *authmodel.Account, _ int64) (bool, error) {
	return s.isRequiredResult, s.isRequiredErr
}

func (s *stubMFAService) HasEnrollment(_ context.Context, _ int64) (bool, error) {
	return s.hasEnrollmentRes, s.hasEnrollmentErr
}

func (s *stubMFAService) StartChallenge(_ context.Context, _, _ int64, _ string, _ net.IP) (string, error) {
	panic("stubMFAService.StartChallenge must not be called in these tests")
}

func (s *stubMFAService) VerifyChallenge(_ context.Context, _, _ string) (*authsvc.VerifiedChallenge, error) {
	panic("stubMFAService.VerifyChallenge must not be called in these tests")
}

func (s *stubMFAService) ResendChallenge(_ context.Context, _ string, _ net.IP) (string, error) {
	panic("stubMFAService.ResendChallenge must not be called in these tests")
}

func (s *stubMFAService) VerifyCodeForAccount(_ context.Context, _ int64, _ string) error {
	panic("stubMFAService.VerifyCodeForAccount must not be called in these tests")
}

func (s *stubMFAService) Enroll(_ context.Context, _ int64) error {
	panic("stubMFAService.Enroll must not be called in these tests")
}

func (s *stubMFAService) Disable(_ context.Context, _ int64) error {
	panic("stubMFAService.Disable must not be called in these tests")
}

func (s *stubMFAService) IssueTrustedDevice(_ context.Context, _, _ int64, _ string, _ net.IP) (string, time.Time, error) {
	panic("stubMFAService.IssueTrustedDevice must not be called in these tests")
}

func (s *stubMFAService) VerifyTrustedDevice(_ context.Context, _, _ int64, _ string) (bool, error) {
	panic("stubMFAService.VerifyTrustedDevice must not be called in these tests")
}

func (s *stubMFAService) ListTrustedDevices(_ context.Context, _, _ int64) ([]*authmodel.MFATrustedDevice, error) {
	panic("stubMFAService.ListTrustedDevices must not be called in these tests")
}

func (s *stubMFAService) RevokeTrustedDevice(_ context.Context, _, _, _ int64) error {
	panic("stubMFAService.RevokeTrustedDevice must not be called in these tests")
}

func (s *stubMFAService) IsTrustedDeviceEnabled(_ context.Context, _ int64) bool {
	panic("stubMFAService.IsTrustedDeviceEnabled must not be called in these tests")
}

func (s *stubMFAService) TrustedDeviceDays(_ context.Context, _ int64) int {
	panic("stubMFAService.TrustedDeviceDays must not be called in these tests")
}

func (s *stubMFAService) AdminDisable(_ context.Context, _, _, _ int64, _ string, _ []string) error {
	panic("stubMFAService.AdminDisable must not be called in these tests")
}

func (s *stubMFAService) SetMFAOverride(_ context.Context, _, _, _ int64, _, _ string, _ []string) error {
	panic("stubMFAService.SetMFAOverride must not be called in these tests")
}

func (s *stubMFAService) GetTenantMFAOverride(_ context.Context, _, _ int64) (string, error) {
	panic("stubMFAService.GetTenantMFAOverride must not be called in these tests")
}

func (s *stubMFAService) GetGlobalMFAOverride(_ context.Context, _ int64) (string, error) {
	panic("stubMFAService.GetGlobalMFAOverride must not be called in these tests")
}

func (s *stubMFAService) OperatorSetGlobalMFAOverride(_ context.Context, _, _ int64, _, _ string) error {
	panic("stubMFAService.OperatorSetGlobalMFAOverride must not be called in these tests")
}

func (s *stubMFAService) OperatorAdminDisable(_ context.Context, _, _, _ int64, _ string) error {
	panic("stubMFAService.OperatorAdminDisable must not be called in these tests")
}

func (s *stubMFAService) OperatorSetMFAOverride(_ context.Context, _, _, _ int64, _, _ string) error {
	panic("stubMFAService.OperatorSetMFAOverride must not be called in these tests")
}

func (s *stubMFAService) GetAdminState(_ context.Context, _, _, _ int64, _ []string) (authsvc.MFAAdminState, error) {
	panic("stubMFAService.GetAdminState must not be called in these tests")
}

// TestLoginWithMFAGate_IsRequiredInfraError_ReturnsMFAStatusUnavailable proves
// the Item #3 contract: when IsRequired surfaces a non-not-found infra error
// (settings DB outage, etc.) the login is refused with ErrMFAStatusUnavailable.
// Before the fix the gate silently treated the failure as "MFA off" and
// issued a full session token pair — an attacker who could DoS the settings
// table could downgrade MFA-required tenants to no-MFA-at-all.
func TestLoginWithMFAGate_IsRequiredInfraError_ReturnsMFAStatusUnavailable(t *testing.T) {
	sc := newLoginGateScenario(t, false) // no real MFA service — we inject our own

	stub := &stubMFAService{
		isRequiredErr: errors.New("settings DB timed out"),
	}
	sc.svc.SetMFAService(stub)

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	assert.Nil(t, result, "infra-error path must not issue tokens")
	require.Error(t, err)

	var authErr *authsvc.AuthError
	require.True(t, errors.As(err, &authErr), "must wrap as AuthError so handleLoginError can switch on it")
	assert.True(t, errors.Is(authErr.Err, authsvc.ErrMFAStatusUnavailable),
		"infra-error from IsRequired must surface as ErrMFAStatusUnavailable")
}

// TestLoginWithMFAGate_HasEnrollmentInfraError_ReturnsMFAStatusUnavailable
// mirrors the previous test for the enrollment-lookup branch. The fix lifts
// HasEnrollment from "swallow ALL errors as (false, nil)" to
// "swallow only sql.ErrNoRows; propagate everything else".
func TestLoginWithMFAGate_HasEnrollmentInfraError_ReturnsMFAStatusUnavailable(t *testing.T) {
	sc := newLoginGateScenario(t, false)

	stub := &stubMFAService{
		isRequiredResult: true, // MFA required for this account
		hasEnrollmentErr: errors.New("mfa_credentials lookup failed: conn reset"),
	}
	sc.svc.SetMFAService(stub)

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	assert.Nil(t, result, "infra-error path must not issue tokens")
	require.Error(t, err)

	var authErr *authsvc.AuthError
	require.True(t, errors.As(err, &authErr), "must wrap as AuthError")
	assert.True(t, errors.Is(authErr.Err, authsvc.ErrMFAStatusUnavailable),
		"infra-error from HasEnrollment must surface as ErrMFAStatusUnavailable")
}

// TestLoginWithMFAGate_StubReturnsNotEnrolledNoError_TreatedAsNotEnrolled
// proves that when HasEnrollment correctly returns (false, nil) — which is
// exactly the contract the real implementation upholds for sql.ErrNoRows —
// the gate proceeds normally and issues tokens (or, for required+enrolled,
// the enrollment token, depending on isRequired). The complementary
// sql.ErrNoRows-translation-inside-HasEnrollment behaviour is verified
// against the real DB by TestMFAService_EnrollDisableLifecycle (a fresh
// account hits FindByAccountID's sql.ErrNoRows path and HasEnrollment must
// still return (false, nil)).
func TestLoginWithMFAGate_StubReturnsNotEnrolledNoError_TreatedAsNotEnrolled(t *testing.T) {
	sc := newLoginGateScenario(t, false)

	stub := &stubMFAService{
		isRequiredResult: false, // MFA off → plain token pair, regardless of enrollment
		hasEnrollmentRes: false,
		hasEnrollmentErr: nil,
	}
	sc.svc.SetMFAService(stub)

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, authsvc.LoginStatusAuthenticated, result.Status)
}
