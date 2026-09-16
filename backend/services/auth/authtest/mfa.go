package authtest

import (
	"context"
	"net"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	svcauth "github.com/moto-nrw/project-phoenix/services/auth"
)

// MFAServiceMock is a func-field test double for auth.MFAService.
type MFAServiceMock struct {
	IsRequiredFn                   func(ctx context.Context, account *authModels.Account, tenantID int64) (bool, error)
	ResolvePolicyFn                func(ctx context.Context, accountID, tenantID int64) (svcauth.MFAPolicy, error)
	ResolvePolicyInTxFn            func(ctx context.Context, accountID, tenantID int64) (svcauth.MFAPolicy, error)
	HasEnrollmentFn                func(ctx context.Context, accountID int64) (bool, error)
	AccountBelongsToTenantFn       func(ctx context.Context, accountID, tenantID int64) (bool, error)
	StartChallengeFn               func(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error)
	VerifyChallengeFn              func(ctx context.Context, challengeToken, code string) (*svcauth.VerifiedChallenge, error)
	VerifyChallengeForScopeFn      func(ctx context.Context, challengeToken, code, expectedScope string) (*svcauth.VerifiedChallenge, error)
	VerifyChallengeForOwnerFn      func(ctx context.Context, challengeToken, code, expectedScope string, accountID, tenantID int64) (*svcauth.VerifiedChallenge, error)
	ResendChallengeFn              func(ctx context.Context, challengeToken string, ip net.IP) (string, error)
	ResendChallengeForScopeFn      func(ctx context.Context, challengeToken string, ip net.IP, expectedScope string) (string, error)
	VerifyCodeForAccountFn         func(ctx context.Context, accountID, tenantID int64, code, expectedScope string) error
	EnrollFn                       func(ctx context.Context, accountID int64) error
	DisableFn                      func(ctx context.Context, accountID int64) error
	IssueTrustedDeviceFn           func(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (string, time.Time, error)
	VerifyTrustedDeviceFn          func(ctx context.Context, accountID, tenantID int64, signedCookie string) (bool, error)
	ListTrustedDevicesFn           func(ctx context.Context, accountID, tenantID int64) ([]*authModels.MFATrustedDevice, error)
	RevokeTrustedDeviceFn          func(ctx context.Context, accountID, tenantID, deviceID int64) error
	IsTrustedDeviceEnabledFn       func(ctx context.Context, tenantID int64) bool
	TrustedDeviceDaysFn            func(ctx context.Context, tenantID int64) int
	AdminDisableFn                 func(ctx context.Context, actorID, actorTenantID, targetAccountID int64, reason string, actorPermissions []string) error
	SetMFAOverrideFn               func(ctx context.Context, actorID, actorTenantID, targetAccountID int64, override, reason string, actorPermissions []string) error
	GetTenantMFAOverrideFn         func(ctx context.Context, accountID, tenantID int64) (string, error)
	GetGlobalMFAOverrideFn         func(ctx context.Context, accountID int64) (string, error)
	OperatorAdminDisableFn         func(ctx context.Context, operatorID, schoolID, targetAccountID int64, reason string) error
	OperatorSetMFAOverrideFn       func(ctx context.Context, operatorID, schoolID, targetAccountID int64, override, reason string) error
	OperatorSetGlobalMFAOverrideFn func(ctx context.Context, operatorID, targetAccountID int64, override, reason string) error
	GetAdminStateFn                func(ctx context.Context, actorID, actorTenantID, targetAccountID int64, actorPermissions []string) (svcauth.MFAAdminState, error)
}

var _ svcauth.MFAService = (*MFAServiceMock)(nil)

func (m *MFAServiceMock) IsRequired(ctx context.Context, account *authModels.Account, tenantID int64) (bool, error) {
	if m.IsRequiredFn != nil {
		return m.IsRequiredFn(ctx, account, tenantID)
	}
	return false, nil
}

// ResolvePolicy defaults to the policy IsRequiredFn describes: a mock that only
// configures IsRequiredFn keeps working on the paths that resolve the policy
// first (the school login gate), instead of silently answering "MFA off".
func (m *MFAServiceMock) ResolvePolicy(ctx context.Context, accountID, tenantID int64) (svcauth.MFAPolicy, error) {
	if m.ResolvePolicyFn != nil {
		return m.ResolvePolicyFn(ctx, accountID, tenantID)
	}
	if m.IsRequiredFn != nil {
		account := &authModels.Account{}
		account.ID = accountID
		required, err := m.IsRequiredFn(ctx, account, tenantID)
		if err != nil {
			return svcauth.MFAPolicy{}, err
		}
		return svcauth.MFAPolicyForced(required), nil
	}
	return svcauth.MFAPolicy{}, nil
}

// ResolvePolicyInTx falls back to ResolvePolicy so a mock that describes one
// MFA state describes it on both sides of the mint transaction — a test that
// configures "MFA required" must not see the guard's in-transaction re-read
// answer "off".
func (m *MFAServiceMock) ResolvePolicyInTx(ctx context.Context, accountID, tenantID int64) (svcauth.MFAPolicy, error) {
	if m.ResolvePolicyInTxFn != nil {
		return m.ResolvePolicyInTxFn(ctx, accountID, tenantID)
	}
	return m.ResolvePolicy(ctx, accountID, tenantID)
}

func (m *MFAServiceMock) HasEnrollment(ctx context.Context, accountID int64) (bool, error) {
	if m.HasEnrollmentFn != nil {
		return m.HasEnrollmentFn(ctx, accountID)
	}
	return false, nil
}

func (m *MFAServiceMock) AccountBelongsToTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	if m.AccountBelongsToTenantFn != nil {
		return m.AccountBelongsToTenantFn(ctx, accountID, tenantID)
	}
	return false, nil
}

func (m *MFAServiceMock) StartChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error) {
	if m.StartChallengeFn != nil {
		return m.StartChallengeFn(ctx, accountID, tenantID, scope, ip)
	}
	return "", nil
}

func (m *MFAServiceMock) VerifyChallenge(ctx context.Context, challengeToken, code string) (*svcauth.VerifiedChallenge, error) {
	if m.VerifyChallengeFn != nil {
		return m.VerifyChallengeFn(ctx, challengeToken, code)
	}
	return nil, nil
}

// VerifyChallengeForScope falls back to VerifyChallengeFn when no scope-aware
// override is set — in the real service the two differ only by the scope
// check, so a test that stubs the unscoped behavior means it for both.
func (m *MFAServiceMock) VerifyChallengeForScope(ctx context.Context, challengeToken, code, expectedScope string) (*svcauth.VerifiedChallenge, error) {
	if m.VerifyChallengeForScopeFn != nil {
		return m.VerifyChallengeForScopeFn(ctx, challengeToken, code, expectedScope)
	}
	if m.VerifyChallengeFn != nil {
		return m.VerifyChallengeFn(ctx, challengeToken, code)
	}
	return nil, nil
}

// VerifyChallengeForOwner falls back through the scope-aware override to the
// unscoped one, same ladder as VerifyChallengeForScope: the real service adds
// only the owner comparison on top, so a test that stubs the looser behavior
// means it here too. Set VerifyChallengeForOwnerFn to assert on the pinned
// account/school.
func (m *MFAServiceMock) VerifyChallengeForOwner(ctx context.Context, challengeToken, code, expectedScope string, accountID, tenantID int64) (*svcauth.VerifiedChallenge, error) {
	if m.VerifyChallengeForOwnerFn != nil {
		return m.VerifyChallengeForOwnerFn(ctx, challengeToken, code, expectedScope, accountID, tenantID)
	}
	return m.VerifyChallengeForScope(ctx, challengeToken, code, expectedScope)
}

func (m *MFAServiceMock) ResendChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	if m.ResendChallengeFn != nil {
		return m.ResendChallengeFn(ctx, challengeToken, ip)
	}
	return "", nil
}

// ResendChallengeForScope mirrors VerifyChallengeForScope: without a
// scope-aware override it runs whatever ResendChallengeFn the test set.
func (m *MFAServiceMock) ResendChallengeForScope(ctx context.Context, challengeToken string, ip net.IP, expectedScope string) (string, error) {
	if m.ResendChallengeForScopeFn != nil {
		return m.ResendChallengeForScopeFn(ctx, challengeToken, ip, expectedScope)
	}
	if m.ResendChallengeFn != nil {
		return m.ResendChallengeFn(ctx, challengeToken, ip)
	}
	return "", nil
}

func (m *MFAServiceMock) VerifyCodeForAccount(ctx context.Context, accountID, tenantID int64, code, expectedScope string) error {
	if m.VerifyCodeForAccountFn != nil {
		return m.VerifyCodeForAccountFn(ctx, accountID, tenantID, code, expectedScope)
	}
	return nil
}

func (m *MFAServiceMock) Enroll(ctx context.Context, accountID int64) error {
	if m.EnrollFn != nil {
		return m.EnrollFn(ctx, accountID)
	}
	return nil
}

func (m *MFAServiceMock) Disable(ctx context.Context, accountID int64) error {
	if m.DisableFn != nil {
		return m.DisableFn(ctx, accountID)
	}
	return nil
}

func (m *MFAServiceMock) IssueTrustedDevice(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (string, time.Time, error) {
	if m.IssueTrustedDeviceFn != nil {
		return m.IssueTrustedDeviceFn(ctx, accountID, tenantID, userAgent, ip)
	}
	return "", time.Time{}, nil
}

func (m *MFAServiceMock) VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, signedCookie string) (bool, error) {
	if m.VerifyTrustedDeviceFn != nil {
		return m.VerifyTrustedDeviceFn(ctx, accountID, tenantID, signedCookie)
	}
	return false, nil
}

func (m *MFAServiceMock) ListTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]*authModels.MFATrustedDevice, error) {
	if m.ListTrustedDevicesFn != nil {
		return m.ListTrustedDevicesFn(ctx, accountID, tenantID)
	}
	return nil, nil
}

func (m *MFAServiceMock) RevokeTrustedDevice(ctx context.Context, accountID, tenantID, deviceID int64) error {
	if m.RevokeTrustedDeviceFn != nil {
		return m.RevokeTrustedDeviceFn(ctx, accountID, tenantID, deviceID)
	}
	return nil
}

func (m *MFAServiceMock) IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool {
	if m.IsTrustedDeviceEnabledFn != nil {
		return m.IsTrustedDeviceEnabledFn(ctx, tenantID)
	}
	return false
}

func (m *MFAServiceMock) TrustedDeviceDays(ctx context.Context, tenantID int64) int {
	if m.TrustedDeviceDaysFn != nil {
		return m.TrustedDeviceDaysFn(ctx, tenantID)
	}
	return 0
}

func (m *MFAServiceMock) AdminDisable(ctx context.Context, actorID, actorTenantID, targetAccountID int64, reason string, actorPermissions []string) error {
	if m.AdminDisableFn != nil {
		return m.AdminDisableFn(ctx, actorID, actorTenantID, targetAccountID, reason, actorPermissions)
	}
	return nil
}

func (m *MFAServiceMock) SetMFAOverride(ctx context.Context, actorID, actorTenantID, targetAccountID int64, override, reason string, actorPermissions []string) error {
	if m.SetMFAOverrideFn != nil {
		return m.SetMFAOverrideFn(ctx, actorID, actorTenantID, targetAccountID, override, reason, actorPermissions)
	}
	return nil
}

func (m *MFAServiceMock) GetTenantMFAOverride(ctx context.Context, accountID, tenantID int64) (string, error) {
	if m.GetTenantMFAOverrideFn != nil {
		return m.GetTenantMFAOverrideFn(ctx, accountID, tenantID)
	}
	return "", nil
}

func (m *MFAServiceMock) GetGlobalMFAOverride(ctx context.Context, accountID int64) (string, error) {
	if m.GetGlobalMFAOverrideFn != nil {
		return m.GetGlobalMFAOverrideFn(ctx, accountID)
	}
	return "", nil
}

func (m *MFAServiceMock) OperatorAdminDisable(ctx context.Context, operatorID, schoolID, targetAccountID int64, reason string) error {
	if m.OperatorAdminDisableFn != nil {
		return m.OperatorAdminDisableFn(ctx, operatorID, schoolID, targetAccountID, reason)
	}
	return nil
}

func (m *MFAServiceMock) OperatorSetMFAOverride(ctx context.Context, operatorID, schoolID, targetAccountID int64, override, reason string) error {
	if m.OperatorSetMFAOverrideFn != nil {
		return m.OperatorSetMFAOverrideFn(ctx, operatorID, schoolID, targetAccountID, override, reason)
	}
	return nil
}

func (m *MFAServiceMock) OperatorSetGlobalMFAOverride(ctx context.Context, operatorID, targetAccountID int64, override, reason string) error {
	if m.OperatorSetGlobalMFAOverrideFn != nil {
		return m.OperatorSetGlobalMFAOverrideFn(ctx, operatorID, targetAccountID, override, reason)
	}
	return nil
}

func (m *MFAServiceMock) GetAdminState(ctx context.Context, actorID, actorTenantID, targetAccountID int64, actorPermissions []string) (svcauth.MFAAdminState, error) {
	if m.GetAdminStateFn != nil {
		return m.GetAdminStateFn(ctx, actorID, actorTenantID, targetAccountID, actorPermissions)
	}
	return svcauth.MFAAdminState{}, nil
}
