package services

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/platform"
)

// Identity & Access owns the account and operator second factor and both
// portals' passkey ceremonies since #3331. The HTTP surfaces that still live
// outside the module consume the retained ports in services/auth and
// services/platform; this file is the only place that knows both sides. Each
// method is a pass-through: it hands the call to the module and translates
// the public error identities into the retained sentinels those handlers
// classify on, so no status code or error string changes with the move.

// mfaRetainedSentinels extend the session translation with the second factor
// and the passkey ceremonies.
var mfaRetainedSentinels = []retainedSentinel{
	{identityaccess.ErrMFAChallengeTokenInvalid, auth.ErrMFAChallengeTokenInvalid},
	{identityaccess.ErrMFACodeInvalid, auth.ErrMFACodeInvalid},
	{identityaccess.ErrMFALocked, auth.ErrMFALocked},
	{identityaccess.ErrMFARateLimited, auth.ErrMFARateLimited},
	{identityaccess.ErrMFANotEnrolled, auth.ErrMFANotEnrolled},
	{identityaccess.ErrMFAAlreadyEnrolled, auth.ErrMFAAlreadyEnrolled},
	{identityaccess.ErrMFAPermissionDenied, auth.ErrMFAPermissionDenied},
	{identityaccess.ErrMFAInvalidOverride, auth.ErrMFAInvalidOverride},
	{identityaccess.ErrMFAUnsupportedScope, auth.ErrMFAUnsupportedScope},
	{identityaccess.ErrMFAStatusUnavailable, auth.ErrMFAStatusUnavailable},
	{identityaccess.ErrPasskeyOriginInvalid, auth.ErrPasskeyOriginInvalid},
	{identityaccess.ErrPasskeySessionInvalid, auth.ErrPasskeySessionInvalid},
	{identityaccess.ErrPasskeyNotFound, auth.ErrPasskeyNotFound},
}

// --- the account second factor ----------------------------------------------

// accountMFAPort serves auth.MFAService over the public module.
type accountMFAPort struct{ module identityaccess.AccountMFA }

func newAccountMFAPort(module identityaccess.AccountMFA) auth.MFAService {
	if module == nil {
		return nil
	}
	return accountMFAPort{module: module}
}

func (p accountMFAPort) IsRequired(ctx context.Context, accountID int64, roleNames []string, tenantID int64) (bool, error) {
	required, err := p.module.IsRequired(ctx, accountID, roleNames, tenantID)
	return required, authServiceError(err)
}

func (p accountMFAPort) ResolveMFAPolicy(ctx context.Context, accountID, tenantID int64) (auth.MFAPolicy, error) {
	policy, err := p.module.ResolveMFAPolicy(ctx, accountID, tenantID)
	if err != nil {
		return nil, authServiceError(err)
	}
	return policy, nil
}

func (p accountMFAPort) ResolveMFAPolicyInTx(ctx context.Context, accountID, tenantID int64) (auth.MFAPolicy, error) {
	policy, err := p.module.ResolveMFAPolicyInTx(ctx, accountID, tenantID)
	if err != nil {
		return nil, authServiceError(err)
	}
	return policy, nil
}

func (p accountMFAPort) HasMFAEnrollment(ctx context.Context, accountID int64) (bool, error) {
	enrolled, err := p.module.HasMFAEnrollment(ctx, accountID)
	return enrolled, authServiceError(err)
}

func (p accountMFAPort) AccountBelongsToSchool(ctx context.Context, accountID, tenantID int64) (bool, error) {
	belongs, err := p.module.AccountBelongsToSchool(ctx, accountID, tenantID)
	return belongs, authServiceError(err)
}

func (p accountMFAPort) IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool {
	return p.module.IsTrustedDeviceEnabled(ctx, tenantID)
}

func (p accountMFAPort) TrustedDeviceDays(ctx context.Context, tenantID int64) int {
	return p.module.TrustedDeviceDays(ctx, tenantID)
}

func (p accountMFAPort) StartMFAChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error) {
	token, err := p.module.StartMFAChallenge(ctx, accountID, tenantID, scope, ip)
	return token, authServiceError(err)
}

func (p accountMFAPort) VerifyMFAChallenge(ctx context.Context, challengeToken, code string) (auth.VerifiedMFAChallenge, error) {
	verified, err := p.module.VerifyMFAChallenge(ctx, challengeToken, code)
	return auth.VerifiedMFAChallenge(verified), authServiceError(err)
}

func (p accountMFAPort) VerifyMFAChallengeForScope(ctx context.Context, challengeToken, code, expectedScope string) (auth.VerifiedMFAChallenge, error) {
	verified, err := p.module.VerifyMFAChallengeForScope(ctx, challengeToken, code, expectedScope)
	return auth.VerifiedMFAChallenge(verified), authServiceError(err)
}

func (p accountMFAPort) VerifyMFAChallengeForOwner(ctx context.Context, challengeToken, code, expectedScope string, accountID, tenantID int64) (auth.VerifiedMFAChallenge, error) {
	verified, err := p.module.VerifyMFAChallengeForOwner(ctx, challengeToken, code, expectedScope, accountID, tenantID)
	return auth.VerifiedMFAChallenge(verified), authServiceError(err)
}

func (p accountMFAPort) VerifyMFACodeForAccount(ctx context.Context, accountID, tenantID int64, code, expectedScope string) error {
	return authServiceError(p.module.VerifyMFACodeForAccount(ctx, accountID, tenantID, code, expectedScope))
}

func (p accountMFAPort) ResendMFAChallengeForScope(ctx context.Context, challengeToken string, ip net.IP, expectedScope string) (string, error) {
	token, err := p.module.ResendMFAChallengeForScope(ctx, challengeToken, ip, expectedScope)
	return token, authServiceError(err)
}

func (p accountMFAPort) ResendMFAChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	token, err := p.module.ResendMFAChallenge(ctx, challengeToken, ip)
	return token, authServiceError(err)
}

func (p accountMFAPort) EnrollMFA(ctx context.Context, accountID int64) error {
	return authServiceError(p.module.EnrollMFA(ctx, accountID))
}

func (p accountMFAPort) DisableMFA(ctx context.Context, accountID int64) error {
	return authServiceError(p.module.DisableMFA(ctx, accountID))
}

func (p accountMFAPort) IssueTrustedDevice(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (string, time.Time, error) {
	cookie, expiresAt, err := p.module.IssueTrustedDevice(ctx, accountID, tenantID, userAgent, ip)
	return cookie, expiresAt, authServiceError(err)
}

func (p accountMFAPort) VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, signedCookie string) (bool, error) {
	trusted, err := p.module.VerifyTrustedDevice(ctx, accountID, tenantID, signedCookie)
	return trusted, authServiceError(err)
}

func (p accountMFAPort) ListTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]auth.AccountTrustedDevice, error) {
	devices, err := p.module.ListTrustedDevices(ctx, accountID, tenantID)
	if err != nil {
		return nil, authServiceError(err)
	}
	retained := make([]auth.AccountTrustedDevice, 0, len(devices))
	for _, device := range devices {
		retained = append(retained, auth.AccountTrustedDevice(device))
	}
	return retained, nil
}

func (p accountMFAPort) RevokeTrustedDevice(ctx context.Context, accountID, tenantID, deviceID int64) error {
	return authServiceError(p.module.RevokeTrustedDevice(ctx, accountID, tenantID, deviceID))
}

func (p accountMFAPort) AdminDisableMFA(ctx context.Context, actorID, actorTenantID, targetAccountID int64, reason string, actorPermissions []string) error {
	return authServiceError(p.module.AdminDisableMFA(ctx, actorID, actorTenantID, targetAccountID, reason, actorPermissions))
}

func (p accountMFAPort) SetMFAOverride(ctx context.Context, actorID, actorTenantID, targetAccountID int64, override, reason string, actorPermissions []string) error {
	return authServiceError(p.module.SetMFAOverride(ctx, actorID, actorTenantID, targetAccountID, override, reason, actorPermissions))
}

func (p accountMFAPort) GetTenantMFAOverride(ctx context.Context, accountID, tenantID int64) (string, error) {
	override, err := p.module.GetTenantMFAOverride(ctx, accountID, tenantID)
	return override, authServiceError(err)
}

func (p accountMFAPort) GetMFAAdminState(ctx context.Context, actorID, actorTenantID, targetAccountID int64, actorPermissions []string) (auth.MFAAdminState, error) {
	state, err := p.module.GetMFAAdminState(ctx, actorID, actorTenantID, targetAccountID, actorPermissions)
	return auth.MFAAdminState(state), authServiceError(err)
}

func (p accountMFAPort) OperatorDisableMFA(ctx context.Context, operatorID, schoolID, targetAccountID int64, reason string) error {
	return authServiceError(p.module.OperatorDisableMFA(ctx, operatorID, schoolID, targetAccountID, reason))
}

func (p accountMFAPort) OperatorSetMFAOverride(ctx context.Context, operatorID, schoolID, targetAccountID int64, override, reason string) error {
	return authServiceError(p.module.OperatorSetMFAOverride(ctx, operatorID, schoolID, targetAccountID, override, reason))
}

func (p accountMFAPort) OperatorSetGlobalMFAOverride(ctx context.Context, operatorID, targetAccountID int64, override, reason string) error {
	return authServiceError(p.module.OperatorSetGlobalMFAOverride(ctx, operatorID, targetAccountID, override, reason))
}

func (p accountMFAPort) GetGlobalMFAOverride(ctx context.Context, accountID int64) (string, error) {
	override, err := p.module.GetGlobalMFAOverride(ctx, accountID)
	return override, authServiceError(err)
}

// --- the school-portal passkey ceremonies -----------------------------------

// accountPasskeyPort serves auth.PasskeyService over the public module.
type accountPasskeyPort struct {
	module identityaccess.AccountPasskeyFlows
}

func newAccountPasskeyPort(module identityaccess.AccountPasskeyFlows) auth.PasskeyService {
	if module == nil {
		return nil
	}
	return accountPasskeyPort{module: module}
}

func (p accountPasskeyPort) StartAccountPasskeyEnrollment(ctx context.Context, accountID, tenantID int64, ip net.IP) (auth.PasskeyEnrollmentChallenge, error) {
	challenge, err := p.module.StartAccountPasskeyEnrollment(ctx, accountID, tenantID, ip)
	return auth.PasskeyEnrollmentChallenge(challenge), authServiceError(err)
}

func (p accountPasskeyPort) BeginAccountPasskeyRegistration(ctx context.Context, request auth.AccountPasskeyRegistrationStart) (auth.PasskeyCeremonyOptions, error) {
	options, err := p.module.BeginAccountPasskeyRegistration(ctx, identityaccess.AccountPasskeyRegistrationStart(request))
	return auth.PasskeyCeremonyOptions(options), authServiceError(err)
}

func (p accountPasskeyPort) FinishAccountPasskeyRegistration(ctx context.Context, request auth.AccountPasskeyRegistrationFinish) (auth.PasskeyCredentialSummary, error) {
	summary, err := p.module.FinishAccountPasskeyRegistration(ctx, identityaccess.AccountPasskeyRegistrationFinish(request))
	return auth.PasskeyCredentialSummary(summary), authServiceError(err)
}

func (p accountPasskeyPort) BeginAccountPasskeyLogin(ctx context.Context, request auth.AccountPasskeyLoginStart) (auth.PasskeyCeremonyOptions, error) {
	options, err := p.module.BeginAccountPasskeyLogin(ctx, identityaccess.AccountPasskeyLoginStart(request))
	return auth.PasskeyCeremonyOptions(options), authServiceError(err)
}

func (p accountPasskeyPort) FinishAccountPasskeyLogin(ctx context.Context, request auth.AccountPasskeyLoginFinish) (auth.PasskeyLoginResult, error) {
	result, err := p.module.FinishAccountPasskeyLogin(ctx, identityaccess.AccountPasskeyLoginFinish(request))
	return auth.PasskeyLoginResult(result), authServiceError(err)
}

func (p accountPasskeyPort) ListAccountPasskeyCredentials(ctx context.Context, accountID int64) ([]auth.PasskeyCredentialSummary, error) {
	summaries, err := p.module.ListAccountPasskeyCredentials(ctx, accountID)
	if err != nil {
		return nil, authServiceError(err)
	}
	return retainedPasskeySummaries(summaries), nil
}

func (p accountPasskeyPort) RevokeAccountPasskeyCredential(ctx context.Context, accountID, credentialID int64) error {
	return authServiceError(p.module.RevokeAccountPasskeyCredential(ctx, accountID, credentialID))
}

// --- the operator second factor ---------------------------------------------

// operatorMFAPort serves platform.OperatorMFAService over the public module.
type operatorMFAPort struct {
	module identityaccess.OperatorMFAFlows
}

func newOperatorMFAPort(module identityaccess.OperatorMFAFlows) platform.OperatorMFAService {
	if module == nil {
		return nil
	}
	return operatorMFAPort{module: module}
}

func (p operatorMFAPort) HasOperatorMFAEnrollment(ctx context.Context, operatorID int64) (bool, error) {
	enrolled, err := p.module.HasOperatorMFAEnrollment(ctx, operatorID)
	return enrolled, authServiceError(err)
}

func (p operatorMFAPort) OperatorTrustedDeviceDays() int {
	return p.module.OperatorTrustedDeviceDays()
}

func (p operatorMFAPort) StartOperatorMFAChallenge(ctx context.Context, operatorID int64, ip net.IP) (string, error) {
	token, err := p.module.StartOperatorMFAChallenge(ctx, operatorID, ip)
	return token, authServiceError(err)
}

func (p operatorMFAPort) VerifyOperatorMFAChallenge(ctx context.Context, challengeToken, code string) (int64, error) {
	operatorID, err := p.module.VerifyOperatorMFAChallenge(ctx, challengeToken, code)
	return operatorID, authServiceError(err)
}

func (p operatorMFAPort) ResendOperatorMFAChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	token, err := p.module.ResendOperatorMFAChallenge(ctx, challengeToken, ip)
	return token, authServiceError(err)
}

func (p operatorMFAPort) VerifyOperatorMFACode(ctx context.Context, operatorID int64, code string) error {
	return authServiceError(p.module.VerifyOperatorMFACode(ctx, operatorID, code))
}

func (p operatorMFAPort) EnrollOperatorMFA(ctx context.Context, operatorID int64) error {
	return authServiceError(p.module.EnrollOperatorMFA(ctx, operatorID))
}

func (p operatorMFAPort) DisableOperatorMFA(ctx context.Context, operatorID int64) error {
	return authServiceError(p.module.DisableOperatorMFA(ctx, operatorID))
}

func (p operatorMFAPort) IssueOperatorTrustedDevice(ctx context.Context, operatorID int64, userAgent string, ip net.IP) (string, time.Time, error) {
	cookie, expiresAt, err := p.module.IssueOperatorTrustedDevice(ctx, operatorID, userAgent, ip)
	return cookie, expiresAt, authServiceError(err)
}

func (p operatorMFAPort) VerifyOperatorTrustedDevice(ctx context.Context, operatorID int64, signedCookie string) (bool, error) {
	trusted, err := p.module.VerifyOperatorTrustedDevice(ctx, operatorID, signedCookie)
	return trusted, authServiceError(err)
}

func (p operatorMFAPort) ListOperatorTrustedDevices(ctx context.Context, operatorID int64) ([]platform.OperatorTrustedDevice, error) {
	devices, err := p.module.ListOperatorTrustedDevices(ctx, operatorID)
	if err != nil {
		return nil, authServiceError(err)
	}
	retained := make([]platform.OperatorTrustedDevice, 0, len(devices))
	for _, device := range devices {
		retained = append(retained, platform.OperatorTrustedDevice(device))
	}
	return retained, nil
}

func (p operatorMFAPort) RevokeOperatorTrustedDeviceOwned(ctx context.Context, operatorID, deviceID int64) error {
	return authServiceError(p.module.RevokeOperatorTrustedDeviceOwned(ctx, operatorID, deviceID))
}

// --- the operator passkey ceremonies -----------------------------------------

// operatorPasskeyPort serves platform.OperatorPasskeyService over the public
// module.
type operatorPasskeyPort struct {
	module identityaccess.OperatorPasskeyFlows
}

func newOperatorPasskeyPort(module identityaccess.OperatorPasskeyFlows) platform.OperatorPasskeyService {
	if module == nil {
		return nil
	}
	return operatorPasskeyPort{module: module}
}

// operatorPasskeyError translates the module's operator outcomes into the
// typed errors the operator surface renders. The retained passkey service
// answered an unknown or deactivated operator with these shapes, and
// api/operator's AuthErrorRenderer still classifies on them, so the
// ceremonies keep their 401 and 403 instead of falling through to a 500.
func operatorPasskeyError(operatorID int64, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, identityaccess.ErrOperatorNotFound):
		return &platform.OperatorNotFoundError{OperatorID: operatorID}
	case errors.Is(err, identityaccess.ErrOperatorInactive):
		return &platform.OperatorInactiveError{OperatorID: operatorID}
	default:
		return authServiceError(err)
	}
}

func (p operatorPasskeyPort) StartOperatorPasskeyEnrollment(ctx context.Context, operatorID int64, ip net.IP) (auth.PasskeyEnrollmentChallenge, error) {
	challenge, err := p.module.StartOperatorPasskeyEnrollment(ctx, operatorID, ip)
	return auth.PasskeyEnrollmentChallenge(challenge), operatorPasskeyError(operatorID, err)
}

func (p operatorPasskeyPort) BeginOperatorPasskeyRegistration(ctx context.Context, request platform.OperatorPasskeyRegistrationStart) (auth.PasskeyCeremonyOptions, error) {
	options, err := p.module.BeginOperatorPasskeyRegistration(ctx, identityaccess.OperatorPasskeyRegistrationStart(request))
	return auth.PasskeyCeremonyOptions(options), operatorPasskeyError(request.OperatorID, err)
}

func (p operatorPasskeyPort) FinishOperatorPasskeyRegistration(ctx context.Context, request platform.OperatorPasskeyRegistrationFinish) (auth.PasskeyCredentialSummary, error) {
	summary, err := p.module.FinishOperatorPasskeyRegistration(ctx, identityaccess.OperatorPasskeyRegistrationFinish(request))
	return auth.PasskeyCredentialSummary(summary), operatorPasskeyError(request.OperatorID, err)
}

func (p operatorPasskeyPort) BeginOperatorPasskeyLogin(ctx context.Context, expectedOrigin string) (auth.PasskeyCeremonyOptions, error) {
	options, err := p.module.BeginOperatorPasskeyLogin(ctx, expectedOrigin)
	return auth.PasskeyCeremonyOptions(options), operatorPasskeyError(0, err)
}

func (p operatorPasskeyPort) FinishOperatorPasskeyLogin(ctx context.Context, request platform.OperatorPasskeyLoginFinish) (auth.PasskeyLoginResult, error) {
	// The discoverable login resolves the operator from the credential, so
	// the id the typed error carries is the one the module reports, not one
	// the request named.
	result, err := p.module.FinishOperatorPasskeyLogin(ctx, identityaccess.OperatorPasskeyLoginFinish(request))
	return auth.PasskeyLoginResult(result), operatorPasskeyError(0, err)
}

func (p operatorPasskeyPort) ListOperatorPasskeyCredentials(ctx context.Context, operatorID int64) ([]auth.PasskeyCredentialSummary, error) {
	summaries, err := p.module.ListOperatorPasskeyCredentials(ctx, operatorID)
	if err != nil {
		return nil, operatorPasskeyError(operatorID, err)
	}
	return retainedPasskeySummaries(summaries), nil
}

func (p operatorPasskeyPort) RevokeOperatorPasskeyCredential(ctx context.Context, operatorID, credentialID int64) error {
	return operatorPasskeyError(operatorID, p.module.RevokeOperatorPasskeyCredential(ctx, operatorID, credentialID))
}

func retainedPasskeySummaries(summaries []identityaccess.PasskeyCredentialSummary) []auth.PasskeyCredentialSummary {
	retained := make([]auth.PasskeyCredentialSummary, 0, len(summaries))
	for _, summary := range summaries {
		retained = append(retained, auth.PasskeyCredentialSummary(summary))
	}
	return retained
}
