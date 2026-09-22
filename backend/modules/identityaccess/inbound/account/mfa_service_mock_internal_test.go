package account

import (
	"context"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// MFAServiceMock is a func-field test double for the Identity & Access
// account MFA capability (#3331). A nil field answers the zero value, so a
// test configures only the calls it cares about.
type MFAServiceMock struct {
	IsRequiredFn                   func(ctx context.Context, accountID int64, roleNames []string, tenantID int64) (bool, error)
	ResolveMFAPolicyFn             func(ctx context.Context, accountID, tenantID int64) (identityaccess.MFAPolicy, error)
	ResolveMFAPolicyInTxFn         func(ctx context.Context, accountID, tenantID int64) (identityaccess.MFAPolicy, error)
	HasMFAEnrollmentFn             func(ctx context.Context, accountID int64) (bool, error)
	AccountBelongsToSchoolFn       func(ctx context.Context, accountID, tenantID int64) (bool, error)
	StartMFAChallengeFn            func(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error)
	VerifyMFAChallengeFn           func(ctx context.Context, challengeToken, code string) (identityaccess.VerifiedMFAChallenge, error)
	VerifyMFAChallengeForScopeFn   func(ctx context.Context, challengeToken, code, expectedScope string) (identityaccess.VerifiedMFAChallenge, error)
	VerifyMFAChallengeForOwnerFn   func(ctx context.Context, challengeToken, code, expectedScope string, accountID, tenantID int64) (identityaccess.VerifiedMFAChallenge, error)
	ResendMFAChallengeFn           func(ctx context.Context, challengeToken string, ip net.IP) (string, error)
	ResendMFAChallengeForScopeFn   func(ctx context.Context, challengeToken string, ip net.IP, expectedScope string) (string, error)
	VerifyMFACodeForAccountFn      func(ctx context.Context, accountID, tenantID int64, code, expectedScope string) error
	EnrollMFAFn                    func(ctx context.Context, accountID int64) error
	DisableMFAFn                   func(ctx context.Context, accountID int64) error
	IssueTrustedDeviceFn           func(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (string, time.Time, error)
	VerifyTrustedDeviceFn          func(ctx context.Context, accountID, tenantID int64, signedCookie string) (bool, error)
	ListTrustedDevicesFn           func(ctx context.Context, accountID, tenantID int64) ([]identityaccess.AccountTrustedDevice, error)
	RevokeTrustedDeviceFn          func(ctx context.Context, accountID, tenantID, deviceID int64) error
	IsTrustedDeviceEnabledFn       func(ctx context.Context, tenantID int64) bool
	TrustedDeviceDaysFn            func(ctx context.Context, tenantID int64) int
	AdminDisableMFAFn              func(ctx context.Context, actorID, actorTenantID, targetAccountID int64, reason string, actorPermissions []string) error
	SetMFAOverrideFn               func(ctx context.Context, actorID, actorTenantID, targetAccountID int64, override, reason string, actorPermissions []string) error
	GetTenantMFAOverrideFn         func(ctx context.Context, accountID, tenantID int64) (string, error)
	GetGlobalMFAOverrideFn         func(ctx context.Context, accountID int64) (string, error)
	OperatorDisableMFAFn           func(ctx context.Context, operatorID, schoolID, targetAccountID int64, reason string) error
	OperatorSetMFAOverrideFn       func(ctx context.Context, operatorID, schoolID, targetAccountID int64, override, reason string) error
	OperatorSetGlobalMFAOverrideFn func(ctx context.Context, operatorID, targetAccountID int64, override, reason string) error
	GetMFAAdminStateFn             func(ctx context.Context, actorID, actorTenantID, targetAccountID int64, actorPermissions []string) (identityaccess.MFAAdminState, error)
}

var _ identityaccess.AccountMFA = (*MFAServiceMock)(nil)

// forcedPolicy answers a fixed verdict regardless of the role set.
type forcedPolicy bool

func (p forcedPolicy) RequiredFor([]string) bool { return bool(p) }

func (m *MFAServiceMock) IsRequired(ctx context.Context, accountID int64, roleNames []string, tenantID int64) (bool, error) {
	if m.IsRequiredFn != nil {
		return m.IsRequiredFn(ctx, accountID, roleNames, tenantID)
	}
	return false, nil
}

// ResolveMFAPolicy defaults to the verdict IsRequiredFn describes, so a mock
// that configures only IsRequiredFn keeps working on the paths that resolve
// the policy first (the school login gate) instead of silently answering
// "MFA off".
func (m *MFAServiceMock) ResolveMFAPolicy(ctx context.Context, accountID, tenantID int64) (identityaccess.MFAPolicy, error) {
	if m.ResolveMFAPolicyFn != nil {
		return m.ResolveMFAPolicyFn(ctx, accountID, tenantID)
	}
	if m.IsRequiredFn != nil {
		required, err := m.IsRequiredFn(ctx, accountID, nil, tenantID)
		if err != nil {
			return nil, err
		}
		return forcedPolicy(required), nil
	}
	return forcedPolicy(false), nil
}

// ResolveMFAPolicyInTx falls back to ResolveMFAPolicy, so a mock that
// describes one MFA state describes it on both sides of the mint
// transaction: a test configuring "MFA required" must not see the guard's
// in-transaction re-read answer "off".
func (m *MFAServiceMock) ResolveMFAPolicyInTx(ctx context.Context, accountID, tenantID int64) (identityaccess.MFAPolicy, error) {
	if m.ResolveMFAPolicyInTxFn != nil {
		return m.ResolveMFAPolicyInTxFn(ctx, accountID, tenantID)
	}
	return m.ResolveMFAPolicy(ctx, accountID, tenantID)
}

func (m *MFAServiceMock) HasMFAEnrollment(ctx context.Context, accountID int64) (bool, error) {
	if m.HasMFAEnrollmentFn != nil {
		return m.HasMFAEnrollmentFn(ctx, accountID)
	}
	return false, nil
}

func (m *MFAServiceMock) AccountBelongsToSchool(ctx context.Context, accountID, tenantID int64) (bool, error) {
	if m.AccountBelongsToSchoolFn != nil {
		return m.AccountBelongsToSchoolFn(ctx, accountID, tenantID)
	}
	return false, nil
}

func (m *MFAServiceMock) StartMFAChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error) {
	if m.StartMFAChallengeFn != nil {
		return m.StartMFAChallengeFn(ctx, accountID, tenantID, scope, ip)
	}
	return "", nil
}

func (m *MFAServiceMock) VerifyMFAChallenge(ctx context.Context, challengeToken, code string) (identityaccess.VerifiedMFAChallenge, error) {
	if m.VerifyMFAChallengeFn != nil {
		return m.VerifyMFAChallengeFn(ctx, challengeToken, code)
	}
	return identityaccess.VerifiedMFAChallenge{}, nil
}

// VerifyMFAChallengeForScope falls back to VerifyMFAChallengeFn when no
// scope-aware override is set: the real flows differ only by the scope
// check, so a test that stubs the unscoped behavior means it for both.
func (m *MFAServiceMock) VerifyMFAChallengeForScope(ctx context.Context, challengeToken, code, expectedScope string) (identityaccess.VerifiedMFAChallenge, error) {
	if m.VerifyMFAChallengeForScopeFn != nil {
		return m.VerifyMFAChallengeForScopeFn(ctx, challengeToken, code, expectedScope)
	}
	if m.VerifyMFAChallengeFn != nil {
		return m.VerifyMFAChallengeFn(ctx, challengeToken, code)
	}
	return identityaccess.VerifiedMFAChallenge{}, nil
}

// VerifyMFAChallengeForOwner falls back through the scope-aware override to
// the unscoped one: the real flow adds only the owner comparison. Set
// VerifyMFAChallengeForOwnerFn to assert on the pinned account and school.
func (m *MFAServiceMock) VerifyMFAChallengeForOwner(ctx context.Context, challengeToken, code, expectedScope string, accountID, tenantID int64) (identityaccess.VerifiedMFAChallenge, error) {
	if m.VerifyMFAChallengeForOwnerFn != nil {
		return m.VerifyMFAChallengeForOwnerFn(ctx, challengeToken, code, expectedScope, accountID, tenantID)
	}
	return m.VerifyMFAChallengeForScope(ctx, challengeToken, code, expectedScope)
}

func (m *MFAServiceMock) ResendMFAChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	if m.ResendMFAChallengeFn != nil {
		return m.ResendMFAChallengeFn(ctx, challengeToken, ip)
	}
	return "", nil
}

// ResendMFAChallengeForScope mirrors VerifyMFAChallengeForScope: without a
// scope-aware override it runs whatever ResendMFAChallengeFn the test set.
func (m *MFAServiceMock) ResendMFAChallengeForScope(ctx context.Context, challengeToken string, ip net.IP, expectedScope string) (string, error) {
	if m.ResendMFAChallengeForScopeFn != nil {
		return m.ResendMFAChallengeForScopeFn(ctx, challengeToken, ip, expectedScope)
	}
	if m.ResendMFAChallengeFn != nil {
		return m.ResendMFAChallengeFn(ctx, challengeToken, ip)
	}
	return "", nil
}

func (m *MFAServiceMock) VerifyMFACodeForAccount(ctx context.Context, accountID, tenantID int64, code, expectedScope string) error {
	if m.VerifyMFACodeForAccountFn != nil {
		return m.VerifyMFACodeForAccountFn(ctx, accountID, tenantID, code, expectedScope)
	}
	return nil
}

func (m *MFAServiceMock) EnrollMFA(ctx context.Context, accountID int64) error {
	if m.EnrollMFAFn != nil {
		return m.EnrollMFAFn(ctx, accountID)
	}
	return nil
}

func (m *MFAServiceMock) DisableMFA(ctx context.Context, accountID int64) error {
	if m.DisableMFAFn != nil {
		return m.DisableMFAFn(ctx, accountID)
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

func (m *MFAServiceMock) ListTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]identityaccess.AccountTrustedDevice, error) {
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

func (m *MFAServiceMock) AdminDisableMFA(ctx context.Context, actorID, actorTenantID, targetAccountID int64, reason string, actorPermissions []string) error {
	if m.AdminDisableMFAFn != nil {
		return m.AdminDisableMFAFn(ctx, actorID, actorTenantID, targetAccountID, reason, actorPermissions)
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

func (m *MFAServiceMock) OperatorDisableMFA(ctx context.Context, operatorID, schoolID, targetAccountID int64, reason string) error {
	if m.OperatorDisableMFAFn != nil {
		return m.OperatorDisableMFAFn(ctx, operatorID, schoolID, targetAccountID, reason)
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

func (m *MFAServiceMock) GetMFAAdminState(ctx context.Context, actorID, actorTenantID, targetAccountID int64, actorPermissions []string) (identityaccess.MFAAdminState, error) {
	if m.GetMFAAdminStateFn != nil {
		return m.GetMFAAdminStateFn(ctx, actorID, actorTenantID, targetAccountID, actorPermissions)
	}
	return identityaccess.MFAAdminState{}, nil
}
