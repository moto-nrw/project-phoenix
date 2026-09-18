package compose

import (
	"context"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Engine methods for the operator MFA flows and both portals' passkey
// ceremonies (#3331). A composition without the MFA dependencies reports
// ErrOperatorMFAUnavailable; one without the relying-party facts reports
// ErrPasskeyFlowsUnavailable for the ceremonies alone.

func (e engine) operatorMFA() (*application.OperatorMFAFlows, error) {
	if e.mfaFlows.operator == nil {
		return nil, identityaccess.ErrOperatorMFAUnavailable
	}
	return e.mfaFlows.operator, nil
}

func (e engine) HasOperatorMFAEnrollment(ctx context.Context, operatorID int64) (bool, error) {
	flows, err := e.operatorMFA()
	if err != nil {
		return false, err
	}
	enrolled, err := flows.HasEnrollment(ctx, operatorID)
	return enrolled, operatorFlowError(err)
}

func (e engine) OperatorTrustedDeviceDays() int {
	flows, err := e.operatorMFA()
	if err != nil {
		return 0
	}
	return flows.TrustedDeviceDays()
}

func (e engine) StartOperatorMFAChallenge(ctx context.Context, operatorID int64, ip net.IP) (string, error) {
	flows, err := e.operatorMFA()
	if err != nil {
		return "", err
	}
	token, err := flows.StartChallenge(ctx, operatorID, ip)
	return token, operatorFlowError(err)
}

func (e engine) VerifyOperatorMFAChallenge(ctx context.Context, challengeToken, code string) (int64, error) {
	flows, err := e.operatorMFA()
	if err != nil {
		return 0, err
	}
	verified, err := flows.VerifyChallenge(ctx, challengeToken, code)
	if err != nil {
		return 0, operatorFlowError(err)
	}
	return verified.OperatorID, nil
}

func (e engine) ResendOperatorMFAChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	flows, err := e.operatorMFA()
	if err != nil {
		return "", err
	}
	token, err := flows.ResendChallenge(ctx, challengeToken, ip)
	return token, operatorFlowError(err)
}

func (e engine) VerifyOperatorMFACode(ctx context.Context, operatorID int64, code string) error {
	flows, err := e.operatorMFA()
	if err != nil {
		return err
	}
	return operatorFlowError(flows.VerifyCodeForOperator(ctx, operatorID, code))
}

func (e engine) EnrollOperatorMFA(ctx context.Context, operatorID int64) error {
	flows, err := e.operatorMFA()
	if err != nil {
		return err
	}
	return operatorFlowError(flows.Enroll(ctx, operatorID))
}

func (e engine) DisableOperatorMFA(ctx context.Context, operatorID int64) error {
	flows, err := e.operatorMFA()
	if err != nil {
		return err
	}
	return operatorFlowError(flows.Disable(ctx, operatorID))
}

func (e engine) IssueOperatorTrustedDevice(ctx context.Context, operatorID int64, userAgent string, ip net.IP) (string, time.Time, error) {
	flows, err := e.operatorMFA()
	if err != nil {
		return "", time.Time{}, err
	}
	cookie, expiresAt, err := flows.IssueTrustedDevice(ctx, operatorID, userAgent, ip)
	return cookie, expiresAt, operatorFlowError(err)
}

func (e engine) VerifyOperatorTrustedDevice(ctx context.Context, operatorID int64, signedCookie string) (bool, error) {
	flows, err := e.operatorMFA()
	if err != nil {
		return false, err
	}
	verified, err := flows.VerifyTrustedDevice(ctx, operatorID, signedCookie)
	return verified, operatorFlowError(err)
}

func (e engine) ListOperatorTrustedDevices(ctx context.Context, operatorID int64) ([]identityaccess.OperatorTrustedDevice, error) {
	flows, err := e.operatorMFA()
	if err != nil {
		return nil, err
	}
	devices, err := flows.ListTrustedDevices(ctx, operatorID)
	if err != nil {
		return nil, operatorFlowError(err)
	}
	result := make([]identityaccess.OperatorTrustedDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, identityaccess.OperatorTrustedDevice(device))
	}
	return result, nil
}

func (e engine) RevokeOperatorTrustedDeviceOwned(ctx context.Context, operatorID, deviceID int64) error {
	flows, err := e.operatorMFA()
	if err != nil {
		return err
	}
	return operatorFlowError(flows.RevokeTrustedDevice(ctx, operatorID, deviceID))
}

// --- the school-portal ceremonies --------------------------------------------

func (e engine) accountPasskeyFlows() (*application.AccountPasskeyFlows, error) {
	if e.mfaFlows.accountPasskey == nil {
		return nil, identityaccess.ErrPasskeyFlowsUnavailable
	}
	return e.mfaFlows.accountPasskey, nil
}

func (e engine) StartAccountPasskeyEnrollment(ctx context.Context, accountID, tenantID int64, ip net.IP) (identityaccess.PasskeyEnrollmentChallenge, error) {
	flows, err := e.accountPasskeyFlows()
	if err != nil {
		return identityaccess.PasskeyEnrollmentChallenge{}, err
	}
	challenge, err := flows.StartEnrollmentChallenge(e.attach(ctx), accountID, tenantID, ip)
	return identityaccess.PasskeyEnrollmentChallenge(challenge), passkeyError(err)
}

func (e engine) BeginAccountPasskeyRegistration(ctx context.Context, request identityaccess.AccountPasskeyRegistrationStart) (identityaccess.PasskeyCeremonyOptions, error) {
	flows, err := e.accountPasskeyFlows()
	if err != nil {
		return identityaccess.PasskeyCeremonyOptions{}, err
	}
	options, err := flows.BeginRegistration(e.attach(ctx), domain.AccountPasskeyRegistrationStart(request))
	return identityaccess.PasskeyCeremonyOptions(options), passkeyError(err)
}

func (e engine) FinishAccountPasskeyRegistration(ctx context.Context, request identityaccess.AccountPasskeyRegistrationFinish) (identityaccess.PasskeyCredentialSummary, error) {
	flows, err := e.accountPasskeyFlows()
	if err != nil {
		return identityaccess.PasskeyCredentialSummary{}, err
	}
	summary, err := flows.FinishRegistration(e.attach(ctx), domain.AccountPasskeyRegistrationFinish(request))
	return identityaccess.PasskeyCredentialSummary(summary), passkeyError(err)
}

func (e engine) BeginAccountPasskeyLogin(ctx context.Context, request identityaccess.AccountPasskeyLoginStart) (identityaccess.PasskeyCeremonyOptions, error) {
	flows, err := e.accountPasskeyFlows()
	if err != nil {
		return identityaccess.PasskeyCeremonyOptions{}, err
	}
	options, err := flows.BeginLogin(e.attach(ctx), domain.AccountPasskeyLoginStart(request))
	return identityaccess.PasskeyCeremonyOptions(options), passkeyError(err)
}

func (e engine) FinishAccountPasskeyLogin(ctx context.Context, request identityaccess.AccountPasskeyLoginFinish) (identityaccess.PasskeyLoginResult, error) {
	flows, err := e.accountPasskeyFlows()
	if err != nil {
		return identityaccess.PasskeyLoginResult{}, err
	}
	result, err := flows.FinishLogin(e.attach(ctx), domain.AccountPasskeyLoginFinish(request))
	return identityaccess.PasskeyLoginResult(result), passkeyError(err)
}

func (e engine) ListAccountPasskeyCredentials(ctx context.Context, accountID int64) ([]identityaccess.PasskeyCredentialSummary, error) {
	flows, err := e.accountPasskeyFlows()
	if err != nil {
		return nil, err
	}
	summaries, err := flows.ListCredentials(e.attach(ctx), accountID)
	if err != nil {
		return nil, passkeyError(err)
	}
	return publicPasskeySummaries(summaries), nil
}

func (e engine) RevokeAccountPasskeyCredential(ctx context.Context, accountID, credentialID int64) error {
	flows, err := e.accountPasskeyFlows()
	if err != nil {
		return err
	}
	return passkeyError(flows.RevokeCredential(e.attach(ctx), accountID, credentialID))
}

// --- the operator ceremonies -------------------------------------------------

func (e engine) operatorPasskeyFlows() (*application.OperatorPasskeyFlows, error) {
	if e.mfaFlows.operatorPasskey == nil {
		return nil, identityaccess.ErrPasskeyFlowsUnavailable
	}
	return e.mfaFlows.operatorPasskey, nil
}

func (e engine) StartOperatorPasskeyEnrollment(ctx context.Context, operatorID int64, ip net.IP) (identityaccess.PasskeyEnrollmentChallenge, error) {
	flows, err := e.operatorPasskeyFlows()
	if err != nil {
		return identityaccess.PasskeyEnrollmentChallenge{}, err
	}
	challenge, err := flows.StartEnrollmentChallenge(ctx, operatorID, ip)
	return identityaccess.PasskeyEnrollmentChallenge(challenge), operatorFlowError(err)
}

func (e engine) BeginOperatorPasskeyRegistration(ctx context.Context, request identityaccess.OperatorPasskeyRegistrationStart) (identityaccess.PasskeyCeremonyOptions, error) {
	flows, err := e.operatorPasskeyFlows()
	if err != nil {
		return identityaccess.PasskeyCeremonyOptions{}, err
	}
	options, err := flows.BeginRegistration(ctx, domain.OperatorPasskeyRegistrationStart(request))
	return identityaccess.PasskeyCeremonyOptions(options), operatorFlowError(err)
}

func (e engine) FinishOperatorPasskeyRegistration(ctx context.Context, request identityaccess.OperatorPasskeyRegistrationFinish) (identityaccess.PasskeyCredentialSummary, error) {
	flows, err := e.operatorPasskeyFlows()
	if err != nil {
		return identityaccess.PasskeyCredentialSummary{}, err
	}
	summary, err := flows.FinishRegistration(ctx, domain.OperatorPasskeyRegistrationFinish(request))
	return identityaccess.PasskeyCredentialSummary(summary), operatorFlowError(err)
}

func (e engine) BeginOperatorPasskeyLogin(ctx context.Context, expectedOrigin string) (identityaccess.PasskeyCeremonyOptions, error) {
	flows, err := e.operatorPasskeyFlows()
	if err != nil {
		return identityaccess.PasskeyCeremonyOptions{}, err
	}
	options, err := flows.BeginLogin(ctx, expectedOrigin)
	return identityaccess.PasskeyCeremonyOptions(options), operatorFlowError(err)
}

func (e engine) FinishOperatorPasskeyLogin(ctx context.Context, request identityaccess.OperatorPasskeyLoginFinish) (identityaccess.PasskeyLoginResult, error) {
	flows, err := e.operatorPasskeyFlows()
	if err != nil {
		return identityaccess.PasskeyLoginResult{}, err
	}
	result, err := flows.FinishLogin(ctx, domain.OperatorPasskeyLoginFinish(request))
	return identityaccess.PasskeyLoginResult(result), operatorFlowError(err)
}

func (e engine) ListOperatorPasskeyCredentials(ctx context.Context, operatorID int64) ([]identityaccess.PasskeyCredentialSummary, error) {
	flows, err := e.operatorPasskeyFlows()
	if err != nil {
		return nil, err
	}
	summaries, err := flows.ListCredentials(ctx, operatorID)
	if err != nil {
		return nil, operatorFlowError(err)
	}
	return publicPasskeySummaries(summaries), nil
}

func (e engine) RevokeOperatorPasskeyCredential(ctx context.Context, operatorID, credentialID int64) error {
	flows, err := e.operatorPasskeyFlows()
	if err != nil {
		return err
	}
	return operatorFlowError(flows.RevokeCredential(ctx, operatorID, credentialID))
}

func publicPasskeySummaries(values []domain.PasskeyCredentialSummary) []identityaccess.PasskeyCredentialSummary {
	result := make([]identityaccess.PasskeyCredentialSummary, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.PasskeyCredentialSummary(value))
	}
	return result
}

// passkeyError is mfaError: the ceremony sentinels are translated by the
// same table, and a gated registration reaches through it.
func passkeyError(err error) error { return mfaError(err) }

// operatorFlowError translates the operator second factor and the operator
// ceremonies. They report the operator's own outcomes — an unknown or a
// deactivated operator — alongside the MFA and ceremony sentinels, and
// operatorError knows both: it maps the operator sentinels and falls through
// to the shared translation for everything else. Without it a deactivated
// operator would leave the module carrying an internal identity no consumer
// can classify on.
func operatorFlowError(err error) error { return operatorError(err) }
