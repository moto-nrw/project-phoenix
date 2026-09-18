package compose

import (
	"context"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// composedAccountMFA serves the public MFA capability over the flows this
// package composed, translating every internal sentinel on the way out. The
// engine reaches the second factor through this one shape, so a composition
// that supplies the capability itself (MFADependencies.Capability) and the
// composed one are interchangeable everywhere.
type composedAccountMFA struct {
	flows  *application.AccountMFAFlows
	attach func(context.Context) context.Context
}

var _ identityaccess.AccountMFA = composedAccountMFA{}

func (c composedAccountMFA) ctx(ctx context.Context) context.Context {
	if c.attach == nil {
		return ctx
	}
	return c.attach(ctx)
}

func (c composedAccountMFA) IsRequired(ctx context.Context, accountID int64, roleNames []string, tenantID int64) (bool, error) {
	required, err := c.flows.IsRequired(c.ctx(ctx), accountID, roleNames, tenantID)
	return required, mfaError(err)
}

func (c composedAccountMFA) ResolveMFAPolicy(ctx context.Context, accountID, tenantID int64) (identityaccess.MFAPolicy, error) {
	policy, err := c.flows.ResolvePolicy(c.ctx(ctx), accountID, tenantID)
	if err != nil {
		return nil, mfaError(err)
	}
	return policy, nil
}

func (c composedAccountMFA) ResolveMFAPolicyInTx(ctx context.Context, accountID, tenantID int64) (identityaccess.MFAPolicy, error) {
	policy, err := c.flows.ResolvePolicyInTx(c.ctx(ctx), accountID, tenantID)
	if err != nil {
		return nil, mfaError(err)
	}
	return policy, nil
}

func (c composedAccountMFA) HasMFAEnrollment(ctx context.Context, accountID int64) (bool, error) {
	enrolled, err := c.flows.HasEnrollment(c.ctx(ctx), accountID)
	return enrolled, mfaError(err)
}

func (c composedAccountMFA) AccountBelongsToSchool(ctx context.Context, accountID, tenantID int64) (bool, error) {
	belongs, err := c.flows.AccountBelongsToTenant(c.ctx(ctx), accountID, tenantID)
	return belongs, mfaError(err)
}

func (c composedAccountMFA) IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool {
	return c.flows.IsTrustedDeviceEnabled(c.ctx(ctx), tenantID)
}

func (c composedAccountMFA) TrustedDeviceDays(ctx context.Context, tenantID int64) int {
	return c.flows.TrustedDeviceDays(c.ctx(ctx), tenantID)
}

func (c composedAccountMFA) StartMFAChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error) {
	token, err := c.flows.StartChallenge(c.ctx(ctx), accountID, tenantID, scope, ip)
	return token, mfaError(err)
}

func (c composedAccountMFA) VerifyMFAChallenge(ctx context.Context, challengeToken, code string) (identityaccess.VerifiedMFAChallenge, error) {
	return c.VerifyMFAChallengeForScope(ctx, challengeToken, code, identityaccess.MFAChallengeScopeTenant)
}

func (c composedAccountMFA) VerifyMFAChallengeForScope(ctx context.Context, challengeToken, code, expectedScope string) (identityaccess.VerifiedMFAChallenge, error) {
	verified, err := c.flows.VerifyChallengeForScope(c.ctx(ctx), challengeToken, code, expectedScope)
	return identityaccess.VerifiedMFAChallenge(verified), mfaError(err)
}

func (c composedAccountMFA) VerifyMFAChallengeForOwner(ctx context.Context, challengeToken, code, expectedScope string, accountID, tenantID int64) (identityaccess.VerifiedMFAChallenge, error) {
	verified, err := c.flows.VerifyChallengeForOwner(c.ctx(ctx), challengeToken, code, expectedScope, accountID, tenantID)
	return identityaccess.VerifiedMFAChallenge(verified), mfaError(err)
}

func (c composedAccountMFA) VerifyMFACodeForAccount(ctx context.Context, accountID, tenantID int64, code, expectedScope string) error {
	return mfaError(c.flows.VerifyCodeForAccount(c.ctx(ctx), accountID, tenantID, code, expectedScope))
}

func (c composedAccountMFA) ResendMFAChallengeForScope(ctx context.Context, challengeToken string, ip net.IP, expectedScope string) (string, error) {
	token, err := c.flows.ResendChallengeForScope(c.ctx(ctx), challengeToken, ip, expectedScope)
	return token, mfaError(err)
}

func (c composedAccountMFA) ResendMFAChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	token, err := c.flows.ResendChallenge(c.ctx(ctx), challengeToken, ip)
	return token, mfaError(err)
}

func (c composedAccountMFA) EnrollMFA(ctx context.Context, accountID int64) error {
	return mfaError(c.flows.Enroll(c.ctx(ctx), accountID))
}

func (c composedAccountMFA) DisableMFA(ctx context.Context, accountID int64) error {
	return mfaError(c.flows.Disable(c.ctx(ctx), accountID))
}

func (c composedAccountMFA) IssueTrustedDevice(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (string, time.Time, error) {
	cookie, expiresAt, err := c.flows.IssueTrustedDevice(c.ctx(ctx), accountID, tenantID, userAgent, ip)
	return cookie, expiresAt, mfaError(err)
}

func (c composedAccountMFA) VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, signedCookie string) (bool, error) {
	verified, err := c.flows.VerifyTrustedDevice(c.ctx(ctx), accountID, tenantID, signedCookie)
	return verified, mfaError(err)
}

func (c composedAccountMFA) ListTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]identityaccess.AccountTrustedDevice, error) {
	devices, err := c.flows.ListTrustedDevices(c.ctx(ctx), accountID, tenantID)
	if err != nil {
		return nil, mfaError(err)
	}
	result := make([]identityaccess.AccountTrustedDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, identityaccess.AccountTrustedDevice(device))
	}
	return result, nil
}

func (c composedAccountMFA) RevokeTrustedDevice(ctx context.Context, accountID, tenantID, deviceID int64) error {
	return mfaError(c.flows.RevokeTrustedDevice(c.ctx(ctx), accountID, tenantID, deviceID))
}

func (c composedAccountMFA) AdminDisableMFA(ctx context.Context, actorID, actorTenantID, targetAccountID int64, reason string, actorPermissions []string) error {
	return mfaError(c.flows.AdminDisable(c.ctx(ctx), actorID, actorTenantID, targetAccountID, reason, actorPermissions))
}

func (c composedAccountMFA) SetMFAOverride(ctx context.Context, actorID, actorTenantID, targetAccountID int64, override, reason string, actorPermissions []string) error {
	return mfaError(c.flows.SetMFAOverride(c.ctx(ctx), actorID, actorTenantID, targetAccountID, override, reason, actorPermissions))
}

func (c composedAccountMFA) GetTenantMFAOverride(ctx context.Context, accountID, tenantID int64) (string, error) {
	override, err := c.flows.GetTenantMFAOverride(c.ctx(ctx), accountID, tenantID)
	return override, mfaError(err)
}

func (c composedAccountMFA) GetMFAAdminState(ctx context.Context, actorID, actorTenantID, targetAccountID int64, actorPermissions []string) (identityaccess.MFAAdminState, error) {
	state, err := c.flows.GetAdminState(c.ctx(ctx), actorID, actorTenantID, targetAccountID, actorPermissions)
	return identityaccess.MFAAdminState(state), mfaError(err)
}

func (c composedAccountMFA) OperatorDisableMFA(ctx context.Context, operatorID, schoolID, targetAccountID int64, reason string) error {
	return mfaError(c.flows.OperatorAdminDisable(c.ctx(ctx), operatorID, schoolID, targetAccountID, reason))
}

func (c composedAccountMFA) OperatorSetMFAOverride(ctx context.Context, operatorID, schoolID, targetAccountID int64, override, reason string) error {
	return mfaError(c.flows.OperatorSetMFAOverride(c.ctx(ctx), operatorID, schoolID, targetAccountID, override, reason))
}

func (c composedAccountMFA) OperatorSetGlobalMFAOverride(ctx context.Context, operatorID, targetAccountID int64, override, reason string) error {
	return mfaError(c.flows.OperatorSetGlobalMFAOverride(c.ctx(ctx), operatorID, targetAccountID, override, reason))
}

func (c composedAccountMFA) GetGlobalMFAOverride(ctx context.Context, accountID int64) (string, error) {
	override, err := c.flows.GetGlobalMFAOverride(c.ctx(ctx), accountID)
	return override, mfaError(err)
}

// capabilityGate adapts the public capability to the gate the login paths
// consult, so the composed flows and a supplied capability gate a login the
// same way. A module composed without MFA reports Configured false, which
// the flows read as "not required, not enrolled".
type capabilityGate struct{ capability identityaccess.AccountMFA }

var _ ports.MFAGate = capabilityGate{}

func (g capabilityGate) Configured() bool { return g.capability != nil }

func (g capabilityGate) IsRequired(ctx context.Context, accountID int64, _ string, roleNames []string, tenantID int64) (bool, error) {
	if g.capability == nil {
		return false, nil
	}
	return g.capability.IsRequired(ctx, accountID, roleNames, tenantID)
}

func (g capabilityGate) ResolvePolicy(ctx context.Context, accountID, tenantID int64) (ports.MFAPolicy, error) {
	if g.capability == nil {
		return domain.MFAPolicy{}, nil
	}
	return g.capability.ResolveMFAPolicy(ctx, accountID, tenantID)
}

func (g capabilityGate) ResolvePolicyInTx(ctx context.Context, accountID, tenantID int64) (ports.MFAPolicy, error) {
	if g.capability == nil {
		return domain.MFAPolicy{}, nil
	}
	return g.capability.ResolveMFAPolicyInTx(ctx, accountID, tenantID)
}

func (g capabilityGate) HasEnrollment(ctx context.Context, accountID int64) (bool, error) {
	if g.capability == nil {
		return false, nil
	}
	return g.capability.HasMFAEnrollment(ctx, accountID)
}

func (g capabilityGate) VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, cookie string) (bool, error) {
	if g.capability == nil {
		return false, nil
	}
	return g.capability.VerifyTrustedDevice(ctx, accountID, tenantID, cookie)
}

func (g capabilityGate) StartChallenge(ctx context.Context, accountID, tenantID int64, scope, ipAddress string) (string, error) {
	if g.capability == nil {
		return "", errMFANotComposed
	}
	challengeScope := identityaccess.MFAChallengeScopeTenant
	if scope == domain.ScopeSchool {
		challengeScope = identityaccess.MFAChallengeScopeSchool
	}
	return g.capability.StartMFAChallenge(ctx, accountID, tenantID, challengeScope, parseGateIP(ipAddress))
}

func (g capabilityGate) IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool {
	if g.capability == nil {
		return false
	}
	return g.capability.IsTrustedDeviceEnabled(ctx, tenantID)
}

func (g capabilityGate) TrustedDeviceDays(ctx context.Context, tenantID int64) int {
	if g.capability == nil {
		return 0
	}
	return g.capability.TrustedDeviceDays(ctx, tenantID)
}
