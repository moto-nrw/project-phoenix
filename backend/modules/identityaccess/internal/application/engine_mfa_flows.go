package application

import (
	"context"
	"net"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Engine methods for the school-portal passkey ceremonies (#3331). A
// composition without the relying-party facts reports
// ErrPasskeyFlowsUnavailable for the ceremonies alone.

func (e engine) accountPasskeyFlows() (*AccountPasskeyFlows, error) {
	if e.mfaFlows.AccountPasskey == nil {
		return nil, identityaccess.ErrPasskeyFlowsUnavailable
	}
	return e.mfaFlows.AccountPasskey, nil
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

func publicPasskeySummaries(values []domain.PasskeyCredentialSummary) []identityaccess.PasskeyCredentialSummary {
	result := make([]identityaccess.PasskeyCredentialSummary, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.PasskeyCredentialSummary(value))
	}
	return result
}

// passkeyError is MFAError: the ceremony sentinels are translated by the
// same table, and a gated registration reaches through it.
func passkeyError(err error) error { return MFAError(err) }

// operatorFlowError translates the operator second factor and the operator
// ceremonies. They report the operator's own outcomes — an unknown or a
// deactivated operator — alongside the MFA and ceremony sentinels, and
// operatorError knows both: it maps the operator sentinels and falls through
// to the shared translation for everything else. Without it a deactivated
// operator would leave the module carrying an internal identity no consumer
// can classify on.
func operatorFlowError(err error) error { return operatorError(err) }
