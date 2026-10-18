package compose

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// The MFA and passkey composition owns its persistence. School settings,
// challenge signing, code hashing, mail and the operator action log arrive
// through consumer-owned seams.

// MFASettings resolves the school settings that parameterise the account
// gate. Each answers the school's override for a positive tenant and the
// registry default otherwise.
type MFASettings interface {
	MFAMode(ctx context.Context, tenantID int64) (string, error)
	// MFAModeInTx reads on the caller's transaction, past the
	// request-scoped memo cache.
	MFAModeInTx(ctx context.Context, tenantID int64) (string, error)
	TrustedDeviceEnabled(ctx context.Context, tenantID int64) (bool, error)
	TrustedDeviceDays(ctx context.Context, tenantID int64) (int, error)
	LockoutThreshold(ctx context.Context, tenantID int64) (int, error)
	LockoutDuration(ctx context.Context, tenantID int64) (time.Duration, error)
}

// ShortCodeHasher hashes and verifies the e-mail codes through the
// project-wide Argon2id helper.
type ShortCodeHasher interface {
	HashShortCode(plain string) (string, error)
	VerifyShortCode(plain, encodedHash string) (bool, error)
}

// PasskeyDependencies are the relying-party facts the ceremonies need. A
// composition without them serves the MFA flows and reports
// ErrPasskeyFlowsUnavailable for every ceremony.
type PasskeyDependencies struct {
	// RPID is the configured relying party. A localhost value follows the
	// request's own host, so every school subdomain works in development.
	RPID   string
	RPName string
	// TenantDomain is the base domain the school subdomains live under.
	TenantDomain string
	// OperatorFrontendURL is the operator portal the operator ceremonies are
	// pinned to.
	OperatorFrontendURL string
}

// MFADependencies compose the account and operator MFA flows (#3331). They
// require Sessions, whose audit ledger the account events append to;
// compositions without them report ErrAccountMFAUnavailable and
// ErrOperatorMFAUnavailable.
type MFADependencies struct {
	// DecorateRecords lets behavior tests inject persistence failures.
	// Production leaves it nil and uses the native records directly.
	DecorateRecords func(identityPorts.AccountMFARecords) identityPorts.AccountMFARecords
	Settings        MFASettings
	Codec           identityPorts.MFAChallengeCodec
	Codes           ShortCodeHasher
	Mail            identityPorts.MFAMail
	OperatorAudit   identityPorts.OperatorActionLog
	// JWTSecret derives the trusted-device HMAC key, so the cookie signature
	// and the JWT signing key stay independent.
	JWTSecret string
	Passkeys  *PasskeyDependencies
	Logger    *slog.Logger
	// Capability lets a composition serve the account second factor itself
	// instead of composing it over native records. The service root leaves it nil;
	// a root that already holds the capability — a behaviour test driving
	// the login branches, a future in-process fake — supplies it here, and
	// the login gate, the MFA routes and the passkey registration all reach
	// that one implementation.
	Capability identityaccess.AccountMFA
}

// newMFACore composes the second factor itself. It runs before the session
// flows, because the login paths consult the gate it produces; the passkey
// ceremonies come after them, because a completed ceremony mints a session.
func newMFACore(
	service *application.Service,
	operatorRecords *application.OperatorMFA,
	records ports.AccountMFARecords,
	sessions *SessionDependencies,
	deps *MFADependencies,
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
	trail := application.MFAAuditTrail{Events: application.AuthAudit{Source: sessions.Audit}, Operator: deps.OperatorAudit}
	secret := domain.DeriveMFASecret(deps.JWTSecret)
	if deps.DecorateRecords != nil {
		records = application.AccountMFARecords{Source: deps.DecorateRecords(application.PublicAccountMFARecords{Source: records})}
	}

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
	sessions *SessionDependencies,
	deps *MFADependencies,
) (application.MFAFlows, error) {
	if deps == nil || deps.Passkeys == nil {
		return flows, nil
	}
	if auth == nil || operatorAuth == nil {
		return application.MFAFlows{}, errors.New("identity access compose: the passkey ceremonies require the session and operator flows")
	}
	runtime := mfaRuntime(sessions)
	accountFlows, err := application.NewAccountPasskeyFlows(application.AccountPasskeyFlowDependencies{
		Accounts: flows.Records, Records: accountPasskeys, MFA: flows.Account,
		Sessions: auth, Runtime: runtime, RPID: deps.Passkeys.RPID, RPName: deps.Passkeys.RPName,
		TenantDomain: deps.Passkeys.TenantDomain,
	})
	if err != nil {
		return application.MFAFlows{}, err
	}
	operatorFlows, err := application.NewOperatorPasskeyFlows(service, application.OperatorPasskeyFlowDependencies{
		Records: operatorPasskeys, MFA: flows.Operator, Sessions: operatorAuth, Runtime: runtime,
		RPID: deps.Passkeys.RPID, RPName: deps.Passkeys.RPName,
		OperatorFrontendURL: deps.Passkeys.OperatorFrontendURL,
	})
	if err != nil {
		return application.MFAFlows{}, err
	}
	// The ceremonies are carried in a new value rather than written into the
	// composed one: the composition surface stays free of mutable wiring.
	return application.MFAFlows{
		Records: flows.Records, Account: flows.Account, Capability: flows.Capability, Operator: flows.Operator,
		AccountPasskey: accountFlows, OperatorPasskey: operatorFlows,
	}, nil
}

func mfaRuntime(sessions *SessionDependencies) tenantRuntime {
	return tenantRuntime{attach: sessions.TenantRuntime, runner: newTransactionRunner()}
}

// operatorLoginGate is the gate the operator login consults; the demo
// environment may leave it unconfigured (DemoDependencies.OperatorWithoutSecondFactor).
func operatorLoginGate(flows application.MFAFlows, demo *DemoDependencies) application.OperatorMFAGate {
	if demo != nil && demo.OperatorWithoutSecondFactor {
		return application.OperatorMFAGate{}
	}
	return application.OperatorMFAGate{Flows: flows.Operator}
}

// --- the seams the root fills ------------------------------------------------

// --- the account MFA records -------------------------------------------------

var _ ports.AccountMFARecords = application.AccountMFARecords{}

// --- the engine methods ------------------------------------------------------
