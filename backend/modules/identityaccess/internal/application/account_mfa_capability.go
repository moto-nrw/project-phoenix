package application

import (
	"context"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// composedAccountMFA serves the public MFA capability over the flows this
// package composed, translating every internal sentinel on the way out. The
// engine reaches the second factor through this one shape, so a composition
// that supplies the capability itself (MFADependencies.Capability) and the
// composed one are interchangeable everywhere.
type ComposedAccountMFA struct {
	Flows  *AccountMFAFlows
	Attach func(context.Context) context.Context
}

func (c ComposedAccountMFA) ctx(ctx context.Context) context.Context {
	if c.Attach == nil {
		return ctx
	}
	return c.Attach(ctx)
}

func (c ComposedAccountMFA) IsRequired(ctx context.Context, accountID int64, permissions []string, tenantID int64) (bool, error) {
	required, err := c.Flows.IsRequired(c.ctx(ctx), accountID, permissions, tenantID)
	return required, MFAError(err)
}

func (c ComposedAccountMFA) ResolveMFAPolicy(ctx context.Context, accountID, tenantID int64) (identityaccess.MFAPolicy, error) {
	policy, err := c.Flows.ResolvePolicy(c.ctx(ctx), accountID, tenantID)
	if err != nil {
		return nil, MFAError(err)
	}
	return policy, nil
}

func (c ComposedAccountMFA) ResolveMFAPolicyInTx(ctx context.Context, accountID, tenantID int64) (identityaccess.MFAPolicy, error) {
	policy, err := c.Flows.ResolvePolicyInTx(c.ctx(ctx), accountID, tenantID)
	if err != nil {
		return nil, MFAError(err)
	}
	return policy, nil
}

func (c ComposedAccountMFA) HasMFAEnrollment(ctx context.Context, accountID int64) (bool, error) {
	enrolled, err := c.Flows.HasEnrollment(c.ctx(ctx), accountID)
	return enrolled, MFAError(err)
}

func (c ComposedAccountMFA) AccountBelongsToSchool(ctx context.Context, accountID, tenantID int64) (bool, error) {
	belongs, err := c.Flows.records.AccountBelongsToTenant(c.ctx(ctx), accountID, tenantID)
	return belongs, MFAError(err)
}

func (c ComposedAccountMFA) IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool {
	return c.Flows.IsTrustedDeviceEnabled(c.ctx(ctx), tenantID)
}

func (c ComposedAccountMFA) TrustedDeviceDays(ctx context.Context, tenantID int64) int {
	return c.Flows.TrustedDeviceDays(c.ctx(ctx), tenantID)
}

func (c ComposedAccountMFA) StartMFAChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error) {
	token, err := c.Flows.StartChallenge(c.ctx(ctx), accountID, tenantID, scope, ip)
	return token, MFAError(err)
}

func (c ComposedAccountMFA) VerifyMFAChallenge(ctx context.Context, challengeToken, code string) (identityaccess.VerifiedMFAChallenge, error) {
	return c.VerifyMFAChallengeForScope(ctx, challengeToken, code, identityaccess.MFAChallengeScopeTenant)
}

func (c ComposedAccountMFA) VerifyMFAChallengeForScope(ctx context.Context, challengeToken, code, expectedScope string) (identityaccess.VerifiedMFAChallenge, error) {
	verified, err := c.Flows.VerifyChallengeForScope(c.ctx(ctx), challengeToken, code, expectedScope)
	return identityaccess.VerifiedMFAChallenge(verified), MFAError(err)
}

func (c ComposedAccountMFA) VerifyMFAChallengeForOwner(ctx context.Context, challengeToken, code, expectedScope string, accountID, tenantID int64) (identityaccess.VerifiedMFAChallenge, error) {
	verified, err := c.Flows.VerifyChallengeForOwner(c.ctx(ctx), challengeToken, code, expectedScope, accountID, tenantID)
	return identityaccess.VerifiedMFAChallenge(verified), MFAError(err)
}

func (c ComposedAccountMFA) ResendMFAChallengeForScope(ctx context.Context, challengeToken string, ip net.IP, expectedScope string) (string, error) {
	token, err := c.Flows.ResendChallengeForScope(c.ctx(ctx), challengeToken, ip, expectedScope)
	return token, MFAError(err)
}

func (c ComposedAccountMFA) ResendMFAChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	token, err := c.Flows.ResendChallenge(c.ctx(ctx), challengeToken, ip)
	return token, MFAError(err)
}

func (c ComposedAccountMFA) EnrollMFA(ctx context.Context, accountID int64) error {
	return MFAError(c.Flows.Enroll(c.ctx(ctx), accountID))
}

func (c ComposedAccountMFA) DisableMFA(ctx context.Context, accountID int64) error {
	return MFAError(c.Flows.Disable(c.ctx(ctx), accountID))
}

// ResetMFAInTransaction is the Disable cascade as a participating command
// (ADR 0067): the moto team resets the second factor of a Träger-Büro member
// in the administrative unit of Organisation & Tenancy, which proves the
// membership and writes the operator audit (#4651). It refuses without that
// ambient administrative transaction and never opens one. The account sets
// the factor up again at its next sign-in; its sessions lose their proof, so
// a Träger session ends at its next refresh.
func (c ComposedAccountMFA) ResetMFAInTransaction(ctx context.Context, accountID int64) error {
	ctx = c.ctx(ctx)
	if !c.Flows.runtime.HasTransaction(ctx) || !c.Flows.runtime.IsAdminTx(ctx) {
		return identityaccess.ErrTraegerTransactionRequired
	}
	if accountID <= 0 {
		return identityaccess.ErrAccountNotFound
	}
	return MFAError(c.Flows.disableCascade(ctx, accountID))
}

func (c ComposedAccountMFA) PrepareTrustedDevice(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (identityaccess.TrustedDeviceIssue, error) {
	issue, err := c.Flows.PrepareTrustedDevice(c.ctx(ctx), accountID, tenantID, userAgent, ip)
	if err != nil || issue == nil {
		return nil, MFAError(err)
	}
	return func(issueCtx context.Context) (string, time.Time, error) {
		cookie, expiresAt, err := issue(c.ctx(issueCtx))
		return cookie, expiresAt, MFAError(err)
	}, nil
}

func (c ComposedAccountMFA) IssueTrustedDevice(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (string, time.Time, error) {
	issue, err := c.PrepareTrustedDevice(ctx, accountID, tenantID, userAgent, ip)
	if err != nil || issue == nil {
		return "", time.Time{}, err
	}
	return issue(ctx)
}

func (c ComposedAccountMFA) VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, signedCookie string) (bool, error) {
	verified, err := c.Flows.VerifyTrustedDevice(c.ctx(ctx), accountID, tenantID, signedCookie)
	return verified, MFAError(err)
}

func (c ComposedAccountMFA) ListTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]identityaccess.AccountTrustedDevice, error) {
	devices, err := c.Flows.records.ListActiveTrustedDevices(c.ctx(ctx), accountID, tenantID)
	if err != nil {
		return nil, MFAError(err)
	}
	result := make([]identityaccess.AccountTrustedDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, identityaccess.AccountTrustedDevice(device))
	}
	return result, nil
}

func (c ComposedAccountMFA) RevokeTrustedDevice(ctx context.Context, accountID, tenantID, deviceID int64) error {
	return MFAError(c.Flows.RevokeTrustedDevice(c.ctx(ctx), accountID, tenantID, deviceID))
}

func (c ComposedAccountMFA) AdminDisableMFA(ctx context.Context, actorID, actorTenantID, targetAccountID int64, reason string, actorPermissions []string) error {
	return MFAError(c.Flows.AdminDisable(c.ctx(ctx), actorID, actorTenantID, targetAccountID, reason, actorPermissions))
}

func (c ComposedAccountMFA) SetMFAOverride(ctx context.Context, actorID, actorTenantID, targetAccountID int64, override, reason string, actorPermissions []string) error {
	return MFAError(c.Flows.SetMFAOverride(c.ctx(ctx), actorID, actorTenantID, targetAccountID, override, reason, actorPermissions))
}

func (c ComposedAccountMFA) GetTenantMFAOverride(ctx context.Context, accountID, tenantID int64) (string, error) {
	override, err := c.Flows.GetTenantMFAOverride(c.ctx(ctx), accountID, tenantID)
	return override, MFAError(err)
}

func (c ComposedAccountMFA) GetMFAAdminState(ctx context.Context, actorID, actorTenantID, targetAccountID int64, actorPermissions []string) (identityaccess.MFAAdminState, error) {
	state, err := c.Flows.GetAdminState(c.ctx(ctx), actorID, actorTenantID, targetAccountID, actorPermissions)
	return identityaccess.MFAAdminState(state), MFAError(err)
}

func (c ComposedAccountMFA) OperatorDisableMFA(ctx context.Context, operatorID, schoolID, targetAccountID int64, reason string) error {
	return MFAError(c.Flows.OperatorAdminDisable(c.ctx(ctx), operatorID, schoolID, targetAccountID, reason))
}

func (c ComposedAccountMFA) OperatorSetMFAOverride(ctx context.Context, operatorID, schoolID, targetAccountID int64, override, reason string) error {
	return MFAError(c.Flows.OperatorSetMFAOverride(c.ctx(ctx), operatorID, schoolID, targetAccountID, override, reason))
}

func (c ComposedAccountMFA) OperatorSetGlobalMFAOverride(ctx context.Context, operatorID, targetAccountID int64, override, reason string) error {
	return MFAError(c.Flows.OperatorSetGlobalMFAOverride(c.ctx(ctx), operatorID, targetAccountID, override, reason))
}

func (c ComposedAccountMFA) GetGlobalMFAOverride(ctx context.Context, accountID int64) (string, error) {
	override, err := c.Flows.GetGlobalMFAOverride(c.ctx(ctx), accountID)
	return override, MFAError(err)
}

// capabilityGate adapts the public capability to the gate the login paths
// consult, so the composed flows and a supplied capability gate a login the
// same way. A module composed without MFA reports Configured false, which
// the flows read as "not required, not enrolled".
type CapabilityGate struct{ Capability identityaccess.AccountMFA }

func (g CapabilityGate) Configured() bool { return g.Capability != nil }

func (g CapabilityGate) IsRequired(ctx context.Context, accountID int64, _ string, permissions []string, tenantID int64) (bool, error) {
	if g.Capability == nil {
		return false, nil
	}
	return g.Capability.IsRequired(ctx, accountID, permissions, tenantID)
}

func (g CapabilityGate) ResolvePolicy(ctx context.Context, accountID, tenantID int64) (ports.MFAPolicy, error) {
	if g.Capability == nil {
		return domain.MFAPolicy{}, nil
	}
	return g.Capability.ResolveMFAPolicy(ctx, accountID, tenantID)
}

func (g CapabilityGate) ResolvePolicyInTx(ctx context.Context, accountID, tenantID int64) (ports.MFAPolicy, error) {
	if g.Capability == nil {
		return domain.MFAPolicy{}, nil
	}
	return g.Capability.ResolveMFAPolicyInTx(ctx, accountID, tenantID)
}

func (g CapabilityGate) HasEnrollment(ctx context.Context, accountID int64) (bool, error) {
	if g.Capability == nil {
		return false, nil
	}
	return g.Capability.HasMFAEnrollment(ctx, accountID)
}

func (g CapabilityGate) VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, cookie string) (bool, error) {
	if g.Capability == nil {
		return false, nil
	}
	return g.Capability.VerifyTrustedDevice(ctx, accountID, tenantID, cookie)
}

func (g CapabilityGate) StartChallenge(ctx context.Context, accountID, tenantID int64, scope, ipAddress string) (string, error) {
	if g.Capability == nil {
		return "", errMFANotComposed
	}
	challengeScope := identityaccess.MFAChallengeScopeTenant
	switch scope {
	case domain.ScopeSchool:
		challengeScope = identityaccess.MFAChallengeScopeSchool
	case domain.ScopeOrg:
		challengeScope = identityaccess.MFAChallengeScopeOrg
	case domain.ScopeCaterer:
		challengeScope = identityaccess.MFAChallengeScopeCaterer
	}
	return g.Capability.StartMFAChallenge(ctx, accountID, tenantID, challengeScope, parseGateIP(ipAddress))
}

func (g CapabilityGate) IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool {
	if g.Capability == nil {
		return false
	}
	return g.Capability.IsTrustedDeviceEnabled(ctx, tenantID)
}

func (g CapabilityGate) TrustedDeviceDays(ctx context.Context, tenantID int64) int {
	if g.Capability == nil {
		return 0
	}
	return g.Capability.TrustedDeviceDays(ctx, tenantID)
}

func (c ComposedAccountMFA) VerifyMFACodeForAccount(ctx context.Context, accountID, tenantID int64, code, scope string) (identityaccess.VerifiedMFAChallenge, error) {
	proof, err := c.Flows.VerifyEnrollmentCode(c.ctx(ctx), accountID, tenantID, code, scope)
	return identityaccess.VerifiedMFAChallenge(proof), MFAError(err)
}
