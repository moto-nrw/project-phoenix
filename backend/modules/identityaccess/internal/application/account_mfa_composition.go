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

// MFAError is the one translation the MFA flows use: the operation
// envelope keeps its text and every internal sentinel gains its public
// counterpart, so a caller switches on one identity across the boundary.
func MFAError(err error) error { return authenticationError(err) }

var errAccountMFAUnavailable = identityaccess.ErrAccountMFAUnavailable

// accountMFACapability serves the account second factor: the composed flows
// or the one a root supplied. The operator's school-scoped admin writes run
// under the school they name, and a composition without the second factor
// reports ErrAccountMFAUnavailable.
type accountMFACapability struct {
	identityaccess.AccountMFA
}

func newAccountMFACapability(capability identityaccess.AccountMFA) accountMFACapability {
	if capability == nil {
		return accountMFACapability{AccountMFA: unavailableAccountMFA{}}
	}
	return accountMFACapability{AccountMFA: capability}
}

func (c accountMFACapability) OperatorDisableMFA(ctx context.Context, operatorID, schoolID, targetAccountID int64, reason string) error {
	return c.AccountMFA.OperatorDisableMFA(operatorSchoolContext(ctx, schoolID), operatorID, schoolID, targetAccountID, reason)
}

func (c accountMFACapability) OperatorSetMFAOverride(ctx context.Context, operatorID, schoolID, targetAccountID int64, override, reason string) error {
	return c.AccountMFA.OperatorSetMFAOverride(operatorSchoolContext(ctx, schoolID), operatorID, schoolID, targetAccountID, override, reason)
}

// unavailableAccountMFA answers every second-factor call of a composition
// without the MFA dependencies.
type unavailableAccountMFA struct{}

func (unavailableAccountMFA) IsRequired(context.Context, int64, []string, int64) (bool, error) {
	return false, errAccountMFAUnavailable
}

func (unavailableAccountMFA) ResolveMFAPolicy(context.Context, int64, int64) (identityaccess.MFAPolicy, error) {
	return nil, errAccountMFAUnavailable
}

func (unavailableAccountMFA) ResolveMFAPolicyInTx(context.Context, int64, int64) (identityaccess.MFAPolicy, error) {
	return nil, errAccountMFAUnavailable
}

func (unavailableAccountMFA) HasMFAEnrollment(context.Context, int64) (bool, error) {
	return false, errAccountMFAUnavailable
}

func (unavailableAccountMFA) AccountBelongsToSchool(context.Context, int64, int64) (bool, error) {
	return false, errAccountMFAUnavailable
}

func (unavailableAccountMFA) IsTrustedDeviceEnabled(context.Context, int64) bool { return false }

func (unavailableAccountMFA) TrustedDeviceDays(context.Context, int64) int { return 0 }

func (unavailableAccountMFA) StartMFAChallenge(context.Context, int64, int64, string, net.IP) (string, error) {
	return "", errAccountMFAUnavailable
}

func (unavailableAccountMFA) VerifyMFAChallenge(context.Context, string, string) (identityaccess.VerifiedMFAChallenge, error) {
	return identityaccess.VerifiedMFAChallenge{}, errAccountMFAUnavailable
}

func (unavailableAccountMFA) VerifyMFAChallengeForScope(context.Context, string, string, string) (identityaccess.VerifiedMFAChallenge, error) {
	return identityaccess.VerifiedMFAChallenge{}, errAccountMFAUnavailable
}

func (unavailableAccountMFA) VerifyMFAChallengeForOwner(context.Context, string, string, string, int64, int64) (identityaccess.VerifiedMFAChallenge, error) {
	return identityaccess.VerifiedMFAChallenge{}, errAccountMFAUnavailable
}

func (unavailableAccountMFA) ResendMFAChallengeForScope(context.Context, string, net.IP, string) (string, error) {
	return "", errAccountMFAUnavailable
}

func (unavailableAccountMFA) ResendMFAChallenge(context.Context, string, net.IP) (string, error) {
	return "", errAccountMFAUnavailable
}

func (unavailableAccountMFA) EnrollMFA(context.Context, int64) error { return errAccountMFAUnavailable }

func (unavailableAccountMFA) DisableMFA(context.Context, int64) error {
	return errAccountMFAUnavailable
}

func (unavailableAccountMFA) ResetMFAInTransaction(context.Context, int64) error {
	return errAccountMFAUnavailable
}

func (unavailableAccountMFA) PrepareTrustedDevice(context.Context, int64, int64, string, net.IP) (identityaccess.TrustedDeviceIssue, error) {
	return nil, errAccountMFAUnavailable
}

func (unavailableAccountMFA) IssueTrustedDevice(context.Context, int64, int64, string, net.IP) (string, time.Time, error) {
	return "", time.Time{}, errAccountMFAUnavailable
}

func (unavailableAccountMFA) VerifyTrustedDevice(context.Context, int64, int64, string) (bool, error) {
	return false, errAccountMFAUnavailable
}

func (unavailableAccountMFA) ListTrustedDevices(context.Context, int64, int64) ([]identityaccess.AccountTrustedDevice, error) {
	return nil, errAccountMFAUnavailable
}

func (unavailableAccountMFA) RevokeTrustedDevice(context.Context, int64, int64, int64) error {
	return errAccountMFAUnavailable
}

func (unavailableAccountMFA) AdminDisableMFA(context.Context, int64, int64, int64, string, []string) error {
	return errAccountMFAUnavailable
}

func (unavailableAccountMFA) SetMFAOverride(context.Context, int64, int64, int64, string, string, []string) error {
	return errAccountMFAUnavailable
}

func (unavailableAccountMFA) GetTenantMFAOverride(context.Context, int64, int64) (string, error) {
	return "", errAccountMFAUnavailable
}

func (unavailableAccountMFA) GetMFAAdminState(context.Context, int64, int64, int64, []string) (identityaccess.MFAAdminState, error) {
	return identityaccess.MFAAdminState{}, errAccountMFAUnavailable
}

func (unavailableAccountMFA) OperatorDisableMFA(context.Context, int64, int64, int64, string) error {
	return errAccountMFAUnavailable
}

func (unavailableAccountMFA) OperatorSetMFAOverride(context.Context, int64, int64, int64, string, string) error {
	return errAccountMFAUnavailable
}

func (unavailableAccountMFA) OperatorSetGlobalMFAOverride(context.Context, int64, int64, string, string) error {
	return errAccountMFAUnavailable
}

func (unavailableAccountMFA) GetGlobalMFAOverride(context.Context, int64) (string, error) {
	return "", errAccountMFAUnavailable
}

func (unavailableAccountMFA) VerifyMFACodeForAccount(context.Context, int64, int64, string, string) (identityaccess.VerifiedMFAChallenge, error) {
	return identityaccess.VerifiedMFAChallenge{}, errAccountMFAUnavailable
}
