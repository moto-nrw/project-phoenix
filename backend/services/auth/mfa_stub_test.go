package auth

import (
	"context"
	"net"
	"time"

	authModel "github.com/moto-nrw/project-phoenix/models/auth"
)

// MFAStub is the package-local test double for MFAService, shared by the
// passkey tests (package auth) and the MFA-login-gate tests (package
// auth_test — reachable because external test files in this directory link
// against the test-augmented build of package auth). It cannot live in
// services/auth/authtest: importing that package from here is an import
// cycle.
//
// Inquiry (IsRequired, HasEnrollment) is configurable via the Result/Err
// fields. AccountBelongsToTenant always returns (true, nil). StartChallenge
// and VerifyCodeForAccount track their call arguments and return a
// configurable token/error.
//
// Every other method — the trusted-device and admin-override surface no test
// here reaches — panics when Strict is true, so an accidental call surfaces
// immediately in the MFA-login-gate tests. The passkey tests leave Strict
// false and never exercise those methods.
type MFAStub struct {
	IsRequiredResult bool
	IsRequiredErr    error
	HasEnrollmentRes bool
	HasEnrollmentErr error

	ChallengeToken string
	StartErr       error
	VerifyErr      error

	StartedAccountID  int64
	StartedTenantID   int64
	StartedScope      string
	VerifiedAccountID int64
	VerifiedCode      string

	Strict bool
}

var _ MFAService = (*MFAStub)(nil)

func (s *MFAStub) panicIfStrict(method string) {
	if s.Strict {
		panic("MFAStub." + method + " must not be called in these tests")
	}
}

func (s *MFAStub) IsRequired(context.Context, *authModel.Account, int64) (bool, error) {
	return s.IsRequiredResult, s.IsRequiredErr
}

func (s *MFAStub) HasEnrollment(context.Context, int64) (bool, error) {
	return s.HasEnrollmentRes, s.HasEnrollmentErr
}

func (s *MFAStub) AccountBelongsToTenant(context.Context, int64, int64) (bool, error) {
	return true, nil
}

func (s *MFAStub) StartChallenge(_ context.Context, accountID, tenantID int64, scope string, _ net.IP) (string, error) {
	s.panicIfStrict("StartChallenge")
	s.StartedAccountID = accountID
	s.StartedTenantID = tenantID
	s.StartedScope = scope
	if s.StartErr != nil {
		return "", s.StartErr
	}
	if s.ChallengeToken != "" {
		return s.ChallengeToken, nil
	}
	return "challenge-token", nil
}

func (s *MFAStub) VerifyChallenge(context.Context, string, string) (*VerifiedChallenge, error) {
	s.panicIfStrict("VerifyChallenge")
	return nil, nil
}

func (s *MFAStub) VerifyChallengeForScope(context.Context, string, string, string) (*VerifiedChallenge, error) {
	s.panicIfStrict("VerifyChallengeForScope")
	return nil, nil
}

func (s *MFAStub) VerifyChallengeForOwner(context.Context, string, string, string, int64, int64) (*VerifiedChallenge, error) {
	s.panicIfStrict("VerifyChallengeForOwner")
	return nil, nil
}

func (s *MFAStub) ResendChallenge(context.Context, string, net.IP) (string, error) {
	s.panicIfStrict("ResendChallenge")
	return "challenge-token", nil
}

func (s *MFAStub) ResendChallengeForScope(context.Context, string, net.IP, string) (string, error) {
	s.panicIfStrict("ResendChallengeForScope")
	return "challenge-token", nil
}

func (s *MFAStub) VerifyCodeForAccount(_ context.Context, accountID, _ int64, code, _ string) error {
	s.panicIfStrict("VerifyCodeForAccount")
	s.VerifiedAccountID = accountID
	s.VerifiedCode = code
	return s.VerifyErr
}

func (s *MFAStub) Enroll(context.Context, int64) error {
	s.panicIfStrict("Enroll")
	return nil
}

func (s *MFAStub) Disable(context.Context, int64) error {
	s.panicIfStrict("Disable")
	return nil
}

func (s *MFAStub) IssueTrustedDevice(context.Context, int64, int64, string, net.IP) (string, time.Time, error) {
	s.panicIfStrict("IssueTrustedDevice")
	return "", time.Time{}, nil
}

func (s *MFAStub) VerifyTrustedDevice(context.Context, int64, int64, string) (bool, error) {
	s.panicIfStrict("VerifyTrustedDevice")
	return false, nil
}

func (s *MFAStub) ListTrustedDevices(context.Context, int64, int64) ([]*authModel.MFATrustedDevice, error) {
	s.panicIfStrict("ListTrustedDevices")
	return nil, nil
}

func (s *MFAStub) RevokeTrustedDevice(context.Context, int64, int64, int64) error {
	s.panicIfStrict("RevokeTrustedDevice")
	return nil
}

func (s *MFAStub) IsTrustedDeviceEnabled(context.Context, int64) bool {
	s.panicIfStrict("IsTrustedDeviceEnabled")
	return false
}

func (s *MFAStub) TrustedDeviceDays(context.Context, int64) int {
	s.panicIfStrict("TrustedDeviceDays")
	return 0
}

func (s *MFAStub) AdminDisable(context.Context, int64, int64, int64, string, []string) error {
	s.panicIfStrict("AdminDisable")
	return nil
}

func (s *MFAStub) SetMFAOverride(context.Context, int64, int64, int64, string, string, []string) error {
	s.panicIfStrict("SetMFAOverride")
	return nil
}

func (s *MFAStub) GetTenantMFAOverride(context.Context, int64, int64) (string, error) {
	s.panicIfStrict("GetTenantMFAOverride")
	return "none", nil
}

func (s *MFAStub) GetGlobalMFAOverride(context.Context, int64) (string, error) {
	s.panicIfStrict("GetGlobalMFAOverride")
	return "none", nil
}

func (s *MFAStub) OperatorAdminDisable(context.Context, int64, int64, int64, string) error {
	s.panicIfStrict("OperatorAdminDisable")
	return nil
}

func (s *MFAStub) OperatorSetMFAOverride(context.Context, int64, int64, int64, string, string) error {
	s.panicIfStrict("OperatorSetMFAOverride")
	return nil
}

func (s *MFAStub) OperatorSetGlobalMFAOverride(context.Context, int64, int64, string, string) error {
	s.panicIfStrict("OperatorSetGlobalMFAOverride")
	return nil
}

func (s *MFAStub) GetAdminState(context.Context, int64, int64, int64, []string) (MFAAdminState, error) {
	s.panicIfStrict("GetAdminState")
	return MFAAdminState{}, nil
}
