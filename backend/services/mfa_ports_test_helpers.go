package services

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services/auth"
)

// The retained MFA and passkey ports are what the HTTP surfaces classify on
// until the handlers move (#3230, #3231), so the error translation they carry
// is behaviour worth pinning. These helpers serve the ports over a supplied
// capability, the way the factory serves them over the composed module.

// NewAccountMFAPortForTests serves the retained second factor over module.
func NewAccountMFAPortForTests(module identityaccess.AccountMFA) auth.MFAService {
	return newAccountMFAPort(module)
}

// NewAccountPasskeyPortForTests serves the retained school-portal ceremonies
// over module.
func NewAccountPasskeyPortForTests(module identityaccess.AccountPasskeyFlows) auth.PasskeyService {
	return newAccountPasskeyPort(module)
}

// NewModuleAccountMFAForTests adapts a retained second factor back to the
// module's capability, as the test module's gate seam does.
func NewModuleAccountMFAForTests(port auth.MFAService) identityaccess.AccountMFA {
	return newModuleAccountMFA(port)
}

// --- the reverse direction, for a composition that supplies the capability --

// moduleAccountMFA serves identityaccess.AccountMFA over a retained port, so
// a composition that was handed the second factor as the port (the test
// module's gate seam) can still compose the module over it. The error
// translation runs backwards: the module's login gate compares against its
// own sentinels.
type moduleAccountMFA struct{ port auth.MFAService }

// newModuleAccountMFA adapts a retained second factor back to the module's
// capability. A nil port composes nothing.
func newModuleAccountMFA(port auth.MFAService) identityaccess.AccountMFA {
	if port == nil {
		return nil
	}
	return moduleAccountMFA{port: port}
}

// moduleMFAError is authServiceError's inverse over the MFA sentinels: a
// retained identity becomes the public one the module's own paths classify
// on. Anything else passes through unchanged.
func moduleMFAError(err error) error {
	if err == nil {
		return nil
	}
	for _, sentinel := range mfaRetainedSentinels {
		if errors.Is(err, sentinel.retained) {
			if err == sentinel.retained {
				return sentinel.public
			}
			return &retainedError{text: err.Error(), sentinel: sentinel.public, cause: err}
		}
	}
	return err
}

func (m moduleAccountMFA) IsRequired(ctx context.Context, accountID int64, roleNames []string, tenantID int64) (bool, error) {
	required, err := m.port.IsRequired(ctx, accountID, roleNames, tenantID)
	return required, moduleMFAError(err)
}

func (m moduleAccountMFA) ResolveMFAPolicy(ctx context.Context, accountID, tenantID int64) (identityaccess.MFAPolicy, error) {
	policy, err := m.port.ResolveMFAPolicy(ctx, accountID, tenantID)
	if err != nil {
		return nil, moduleMFAError(err)
	}
	return policy, nil
}

func (m moduleAccountMFA) ResolveMFAPolicyInTx(ctx context.Context, accountID, tenantID int64) (identityaccess.MFAPolicy, error) {
	policy, err := m.port.ResolveMFAPolicyInTx(ctx, accountID, tenantID)
	if err != nil {
		return nil, moduleMFAError(err)
	}
	return policy, nil
}

func (m moduleAccountMFA) HasMFAEnrollment(ctx context.Context, accountID int64) (bool, error) {
	enrolled, err := m.port.HasMFAEnrollment(ctx, accountID)
	return enrolled, moduleMFAError(err)
}

func (m moduleAccountMFA) AccountBelongsToSchool(ctx context.Context, accountID, tenantID int64) (bool, error) {
	belongs, err := m.port.AccountBelongsToSchool(ctx, accountID, tenantID)
	return belongs, moduleMFAError(err)
}

func (m moduleAccountMFA) IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool {
	return m.port.IsTrustedDeviceEnabled(ctx, tenantID)
}

func (m moduleAccountMFA) TrustedDeviceDays(ctx context.Context, tenantID int64) int {
	return m.port.TrustedDeviceDays(ctx, tenantID)
}

func (m moduleAccountMFA) StartMFAChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error) {
	token, err := m.port.StartMFAChallenge(ctx, accountID, tenantID, scope, ip)
	return token, moduleMFAError(err)
}

func (m moduleAccountMFA) VerifyMFAChallenge(ctx context.Context, challengeToken, code string) (identityaccess.VerifiedMFAChallenge, error) {
	verified, err := m.port.VerifyMFAChallenge(ctx, challengeToken, code)
	return identityaccess.VerifiedMFAChallenge(verified), moduleMFAError(err)
}

func (m moduleAccountMFA) VerifyMFAChallengeForScope(ctx context.Context, challengeToken, code, expectedScope string) (identityaccess.VerifiedMFAChallenge, error) {
	verified, err := m.port.VerifyMFAChallengeForScope(ctx, challengeToken, code, expectedScope)
	return identityaccess.VerifiedMFAChallenge(verified), moduleMFAError(err)
}

func (m moduleAccountMFA) VerifyMFAChallengeForOwner(ctx context.Context, challengeToken, code, expectedScope string, accountID, tenantID int64) (identityaccess.VerifiedMFAChallenge, error) {
	verified, err := m.port.VerifyMFAChallengeForOwner(ctx, challengeToken, code, expectedScope, accountID, tenantID)
	return identityaccess.VerifiedMFAChallenge(verified), moduleMFAError(err)
}

func (m moduleAccountMFA) VerifyMFACodeForAccount(ctx context.Context, accountID, tenantID int64, code, expectedScope string) error {
	return moduleMFAError(m.port.VerifyMFACodeForAccount(ctx, accountID, tenantID, code, expectedScope))
}

func (m moduleAccountMFA) ResendMFAChallengeForScope(ctx context.Context, challengeToken string, ip net.IP, expectedScope string) (string, error) {
	token, err := m.port.ResendMFAChallengeForScope(ctx, challengeToken, ip, expectedScope)
	return token, moduleMFAError(err)
}

func (m moduleAccountMFA) ResendMFAChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	token, err := m.port.ResendMFAChallenge(ctx, challengeToken, ip)
	return token, moduleMFAError(err)
}

func (m moduleAccountMFA) EnrollMFA(ctx context.Context, accountID int64) error {
	return moduleMFAError(m.port.EnrollMFA(ctx, accountID))
}

func (m moduleAccountMFA) DisableMFA(ctx context.Context, accountID int64) error {
	return moduleMFAError(m.port.DisableMFA(ctx, accountID))
}

func (m moduleAccountMFA) IssueTrustedDevice(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (string, time.Time, error) {
	cookie, expiresAt, err := m.port.IssueTrustedDevice(ctx, accountID, tenantID, userAgent, ip)
	return cookie, expiresAt, moduleMFAError(err)
}

func (m moduleAccountMFA) VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, signedCookie string) (bool, error) {
	trusted, err := m.port.VerifyTrustedDevice(ctx, accountID, tenantID, signedCookie)
	return trusted, moduleMFAError(err)
}

func (m moduleAccountMFA) ListTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]identityaccess.AccountTrustedDevice, error) {
	devices, err := m.port.ListTrustedDevices(ctx, accountID, tenantID)
	if err != nil {
		return nil, moduleMFAError(err)
	}
	public := make([]identityaccess.AccountTrustedDevice, 0, len(devices))
	for _, device := range devices {
		public = append(public, identityaccess.AccountTrustedDevice(device))
	}
	return public, nil
}

func (m moduleAccountMFA) RevokeTrustedDevice(ctx context.Context, accountID, tenantID, deviceID int64) error {
	return moduleMFAError(m.port.RevokeTrustedDevice(ctx, accountID, tenantID, deviceID))
}

func (m moduleAccountMFA) AdminDisableMFA(ctx context.Context, actorID, actorTenantID, targetAccountID int64, reason string, actorPermissions []string) error {
	return moduleMFAError(m.port.AdminDisableMFA(ctx, actorID, actorTenantID, targetAccountID, reason, actorPermissions))
}

func (m moduleAccountMFA) SetMFAOverride(ctx context.Context, actorID, actorTenantID, targetAccountID int64, override, reason string, actorPermissions []string) error {
	return moduleMFAError(m.port.SetMFAOverride(ctx, actorID, actorTenantID, targetAccountID, override, reason, actorPermissions))
}

func (m moduleAccountMFA) GetTenantMFAOverride(ctx context.Context, accountID, tenantID int64) (string, error) {
	override, err := m.port.GetTenantMFAOverride(ctx, accountID, tenantID)
	return override, moduleMFAError(err)
}

func (m moduleAccountMFA) GetMFAAdminState(ctx context.Context, actorID, actorTenantID, targetAccountID int64, actorPermissions []string) (identityaccess.MFAAdminState, error) {
	state, err := m.port.GetMFAAdminState(ctx, actorID, actorTenantID, targetAccountID, actorPermissions)
	return identityaccess.MFAAdminState(state), moduleMFAError(err)
}

func (m moduleAccountMFA) OperatorDisableMFA(ctx context.Context, operatorID, schoolID, targetAccountID int64, reason string) error {
	return moduleMFAError(m.port.OperatorDisableMFA(ctx, operatorID, schoolID, targetAccountID, reason))
}

func (m moduleAccountMFA) OperatorSetMFAOverride(ctx context.Context, operatorID, schoolID, targetAccountID int64, override, reason string) error {
	return moduleMFAError(m.port.OperatorSetMFAOverride(ctx, operatorID, schoolID, targetAccountID, override, reason))
}

func (m moduleAccountMFA) OperatorSetGlobalMFAOverride(ctx context.Context, operatorID, targetAccountID int64, override, reason string) error {
	return moduleMFAError(m.port.OperatorSetGlobalMFAOverride(ctx, operatorID, targetAccountID, override, reason))
}

func (m moduleAccountMFA) GetGlobalMFAOverride(ctx context.Context, accountID int64) (string, error) {
	override, err := m.port.GetGlobalMFAOverride(ctx, accountID)
	return override, moduleMFAError(err)
}
