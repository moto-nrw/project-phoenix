package compose

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// The MFA and passkey composition owns its persistence. School settings,
// challenge signing, code hashing, mail and the operator action log arrive
// through consumer-owned seams.

// newMFACore composes the second factor itself. It runs before the session
// flows, because the login paths consult the gate it produces; the passkey
// ceremonies come after them, because a completed ceremony mints a session.
func newMFACore(
	service *application.Service,
	operatorRecords *application.OperatorMFA,
	records ports.AccountMFARecords,
	sessions *identityPorts.SessionDependencies,
	deps *identityPorts.MFADependencies,
) (application.MFAFlows, error) {
	if deps == nil {
		return application.MFAFlows{}, nil
	}
	if sessions == nil {
		return application.MFAFlows{}, errors.New("identity access compose: mfa requires the session dependencies")
	}
	switch {
	case records == nil, deps.Settings == nil, deps.Codec == nil, deps.Codes == nil,
		deps.Mail == nil, deps.OperatorAudit == nil, strings.TrimSpace(deps.JWTSecret) == "":
		return application.MFAFlows{}, errors.New("identity access compose: every mfa dependency is required")
	}
	runtime := mfaRuntime(sessions)
	records = application.NewAccountMFACommands(records, runtime)
	trail := application.MFAAuditTrail{AuthAudit: application.AuthAudit{Source: sessions.Audit}, Operator: deps.OperatorAudit}
	secret := domain.DeriveMFASecret(deps.JWTSecret)

	account, err := application.NewAccountMFAFlows(application.AccountMFAFlowDependencies{
		Records: records, Settings: deps.Settings,
		Codec: application.MFAChallengeCodec{Source: deps.Codec}, Codes: deps.Codes, Mail: application.MFAMail{Source: deps.Mail},
		Audit: trail, Runtime: runtime, Secret: secret, Logger: deps.Logger,
	})
	if err != nil {
		return application.MFAFlows{}, err
	}
	var capability identityaccess.AccountMFA = application.ComposedAccountMFA{Flows: account, Attach: runtime.attach}
	if deps.Capability != nil {
		capability = deps.Capability
	}
	operator, err := application.NewOperatorMFAFlows(service, application.OperatorMFAFlowDependencies{
		Records: operatorRecords, Codec: application.MFAChallengeCodec{Source: deps.Codec}, Codes: deps.Codes,
		Mail: application.MFAMail{Source: deps.Mail}, Audit: trail, Runtime: runtime, Secret: secret, Logger: deps.Logger,
	})
	if err != nil {
		return application.MFAFlows{}, err
	}
	return application.MFAFlows{Records: records, Account: account, Capability: capability, Operator: operator}, nil
}

// --- the engine methods ------------------------------------------------------

// withPasskeyFlows composes both portals' ceremonies once the session flows
// exist and returns the second factor carrying them. A composition without
// the relying-party facts keeps them nil, and every ceremony then reports
// ErrPasskeyFlowsUnavailable.
func withPasskeyFlows(
	flows application.MFAFlows,
	service *application.Service,
	auth *application.AccountAuthentication,
	operatorAuth *application.OperatorAuthentication,
	accountPasskeys *application.AccountPasskey,
	operatorPasskeys *application.OperatorPasskey,
	sessions *identityPorts.SessionDependencies,
	deps *identityPorts.MFADependencies,
) (application.MFAFlows, error) {
	return application.NewPasskeyFlowsBinding(flows, service, auth, operatorAuth, accountPasskeys, operatorPasskeys, deps, func() ports.Runtime { return mfaRuntime(sessions) })
}

func mfaRuntime(sessions *identityPorts.SessionDependencies) tenantRuntime {
	return tenantRuntime{attach: sessions.TenantRuntime, runner: application.NewTransactionRunner()}
}
