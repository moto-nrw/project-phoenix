package application

import (
	"context"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// mfaFlows is what New composes from the MFA dependencies. Its members are
// nil when a root composed the module without them.
// MFAFlows carries the second factor and the passkey ceremonies the engine
// serves. Its members are nil when the module was composed without them.
type MFAFlows struct {
	Records ports.AccountMFARecords
	Account *AccountMFAFlows
	// capability is what the engine and the gate reach the account second
	// factor through: the composed flows, or the one a root supplied.
	Capability      identityaccess.AccountMFA
	Operator        *OperatorMFAFlows
	AccountPasskey  *AccountPasskeyFlows
	OperatorPasskey *OperatorPasskeyFlows
}

func (e engine) accountMFA() (identityaccess.AccountMFA, error) {
	if e.mfaFlows.Capability == nil {
		return nil, identityaccess.ErrAccountMFAUnavailable
	}
	return e.mfaFlows.Capability, nil
}

func (e engine) IsRequired(ctx context.Context, accountID int64, roleNames []string, tenantID int64) (bool, error) {
	mfa, err := e.accountMFA()
	if err != nil {
		return false, err
	}
	return mfa.IsRequired(ctx, accountID, roleNames, tenantID)
}

func (e engine) ResolveMFAPolicy(ctx context.Context, accountID, tenantID int64) (identityaccess.MFAPolicy, error) {
	mfa, err := e.accountMFA()
	if err != nil {
		return nil, err
	}
	return mfa.ResolveMFAPolicy(ctx, accountID, tenantID)
}

func (e engine) ResolveMFAPolicyInTx(ctx context.Context, accountID, tenantID int64) (identityaccess.MFAPolicy, error) {
	mfa, err := e.accountMFA()
	if err != nil {
		return nil, err
	}
	return mfa.ResolveMFAPolicyInTx(ctx, accountID, tenantID)
}

func (e engine) HasMFAEnrollment(ctx context.Context, accountID int64) (bool, error) {
	mfa, err := e.accountMFA()
	if err != nil {
		return false, err
	}
	return mfa.HasMFAEnrollment(ctx, accountID)
}

func (e engine) AccountBelongsToSchool(ctx context.Context, accountID, tenantID int64) (bool, error) {
	mfa, err := e.accountMFA()
	if err != nil {
		return false, err
	}
	return mfa.AccountBelongsToSchool(ctx, accountID, tenantID)
}

func (e engine) IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool {
	mfa, err := e.accountMFA()
	if err != nil {
		return false
	}
	return mfa.IsTrustedDeviceEnabled(ctx, tenantID)
}

func (e engine) TrustedDeviceDays(ctx context.Context, tenantID int64) int {
	mfa, err := e.accountMFA()
	if err != nil {
		return 0
	}
	return mfa.TrustedDeviceDays(ctx, tenantID)
}

func (e engine) StartMFAChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error) {
	mfa, err := e.accountMFA()
	if err != nil {
		return "", err
	}
	return mfa.StartMFAChallenge(ctx, accountID, tenantID, scope, ip)
}

func (e engine) VerifyMFAChallenge(ctx context.Context, challengeToken, code string) (identityaccess.VerifiedMFAChallenge, error) {
	mfa, err := e.accountMFA()
	if err != nil {
		return identityaccess.VerifiedMFAChallenge{}, err
	}
	return mfa.VerifyMFAChallenge(ctx, challengeToken, code)
}

func (e engine) VerifyMFAChallengeForScope(ctx context.Context, challengeToken, code, expectedScope string) (identityaccess.VerifiedMFAChallenge, error) {
	mfa, err := e.accountMFA()
	if err != nil {
		return identityaccess.VerifiedMFAChallenge{}, err
	}
	return mfa.VerifyMFAChallengeForScope(ctx, challengeToken, code, expectedScope)
}

func (e engine) VerifyMFAChallengeForOwner(ctx context.Context, challengeToken, code, expectedScope string, accountID, tenantID int64) (identityaccess.VerifiedMFAChallenge, error) {
	mfa, err := e.accountMFA()
	if err != nil {
		return identityaccess.VerifiedMFAChallenge{}, err
	}
	return mfa.VerifyMFAChallengeForOwner(ctx, challengeToken, code, expectedScope, accountID, tenantID)
}

func (e engine) VerifyMFACodeForAccount(ctx context.Context, accountID, tenantID int64, code, expectedScope string) error {
	mfa, err := e.accountMFA()
	if err != nil {
		return err
	}
	return mfa.VerifyMFACodeForAccount(ctx, accountID, tenantID, code, expectedScope)
}

func (e engine) ResendMFAChallengeForScope(ctx context.Context, challengeToken string, ip net.IP, expectedScope string) (string, error) {
	mfa, err := e.accountMFA()
	if err != nil {
		return "", err
	}
	return mfa.ResendMFAChallengeForScope(ctx, challengeToken, ip, expectedScope)
}

func (e engine) ResendMFAChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	mfa, err := e.accountMFA()
	if err != nil {
		return "", err
	}
	return mfa.ResendMFAChallenge(ctx, challengeToken, ip)
}

func (e engine) EnrollMFA(ctx context.Context, accountID int64) error {
	mfa, err := e.accountMFA()
	if err != nil {
		return err
	}
	return mfa.EnrollMFA(ctx, accountID)
}

func (e engine) DisableMFA(ctx context.Context, accountID int64) error {
	mfa, err := e.accountMFA()
	if err != nil {
		return err
	}
	return mfa.DisableMFA(ctx, accountID)
}

func (e engine) IssueTrustedDevice(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (string, time.Time, error) {
	mfa, err := e.accountMFA()
	if err != nil {
		return "", time.Time{}, err
	}
	return mfa.IssueTrustedDevice(ctx, accountID, tenantID, userAgent, ip)
}

func (e engine) VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, signedCookie string) (bool, error) {
	mfa, err := e.accountMFA()
	if err != nil {
		return false, err
	}
	return mfa.VerifyTrustedDevice(ctx, accountID, tenantID, signedCookie)
}

func (e engine) ListTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]identityaccess.AccountTrustedDevice, error) {
	mfa, err := e.accountMFA()
	if err != nil {
		return nil, err
	}
	return mfa.ListTrustedDevices(ctx, accountID, tenantID)
}

func (e engine) RevokeTrustedDevice(ctx context.Context, accountID, tenantID, deviceID int64) error {
	mfa, err := e.accountMFA()
	if err != nil {
		return err
	}
	return mfa.RevokeTrustedDevice(ctx, accountID, tenantID, deviceID)
}

func (e engine) AdminDisableMFA(ctx context.Context, actorID, actorTenantID, targetAccountID int64, reason string, actorPermissions []string) error {
	mfa, err := e.accountMFA()
	if err != nil {
		return err
	}
	return mfa.AdminDisableMFA(ctx, actorID, actorTenantID, targetAccountID, reason, actorPermissions)
}

func (e engine) SetMFAOverride(ctx context.Context, actorID, actorTenantID, targetAccountID int64, override, reason string, actorPermissions []string) error {
	mfa, err := e.accountMFA()
	if err != nil {
		return err
	}
	return mfa.SetMFAOverride(ctx, actorID, actorTenantID, targetAccountID, override, reason, actorPermissions)
}

func (e engine) GetTenantMFAOverride(ctx context.Context, accountID, tenantID int64) (string, error) {
	mfa, err := e.accountMFA()
	if err != nil {
		return "", err
	}
	return mfa.GetTenantMFAOverride(ctx, accountID, tenantID)
}

func (e engine) GetMFAAdminState(ctx context.Context, actorID, actorTenantID, targetAccountID int64, actorPermissions []string) (identityaccess.MFAAdminState, error) {
	mfa, err := e.accountMFA()
	if err != nil {
		return identityaccess.MFAAdminState{}, err
	}
	return mfa.GetMFAAdminState(ctx, actorID, actorTenantID, targetAccountID, actorPermissions)
}

func (e engine) OperatorDisableMFA(ctx context.Context, operatorID, schoolID, targetAccountID int64, reason string) error {
	mfa, err := e.accountMFA()
	if err != nil {
		return err
	}
	return mfa.OperatorDisableMFA(operatorSchoolContext(ctx, schoolID), operatorID, schoolID, targetAccountID, reason)
}

func (e engine) OperatorSetMFAOverride(ctx context.Context, operatorID, schoolID, targetAccountID int64, override, reason string) error {
	mfa, err := e.accountMFA()
	if err != nil {
		return err
	}
	return mfa.OperatorSetMFAOverride(operatorSchoolContext(ctx, schoolID), operatorID, schoolID, targetAccountID, override, reason)
}

// operatorSchoolContext runs an operator's school-scoped MFA admin write
// under that school. The operator routes run outside the tenant middleware,
// and the audit append needs the tenant (#3231: moved here from the operator
// HTTP adapter, which may not name the tenant runtime). A missing school is
// left to the application, which refuses it.
func operatorSchoolContext(ctx context.Context, schoolID int64) context.Context {
	if schoolID <= 0 {
		return ctx
	}
	return tenant.WithTenantID(ctx, schoolID)
}

func (e engine) OperatorSetGlobalMFAOverride(ctx context.Context, operatorID, targetAccountID int64, override, reason string) error {
	mfa, err := e.accountMFA()
	if err != nil {
		return err
	}
	return mfa.OperatorSetGlobalMFAOverride(ctx, operatorID, targetAccountID, override, reason)
}

func (e engine) GetGlobalMFAOverride(ctx context.Context, accountID int64) (string, error) {
	mfa, err := e.accountMFA()
	if err != nil {
		return "", err
	}
	return mfa.GetGlobalMFAOverride(ctx, accountID)
}

// MFAError is the one translation the MFA flows use: the operation
// envelope keeps its text and every internal sentinel gains its public
// counterpart, so a caller switches on one identity across the boundary.
func MFAError(err error) error { return authenticationError(err) }
