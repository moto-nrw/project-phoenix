package compose

import (
	"context"
	"errors"
	"log/slog"
	"net"
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
	trail := mfaAuditTrail{events: authAudit{sessions.Audit}, operator: deps.OperatorAudit}
	secret := domain.DeriveMFASecret(deps.JWTSecret)
	if deps.DecorateRecords != nil {
		records = accountMFARecords{source: deps.DecorateRecords(publicAccountMFARecords{source: records})}
	}

	account, err := application.NewAccountMFAFlows(application.AccountMFAFlowDependencies{
		Records: records, Settings: deps.Settings,
		Codec: mfaChallengeCodec{deps.Codec}, Codes: deps.Codes, Mail: mfaMail{deps.Mail},
		Audit: trail, Runtime: runtime, Secret: secret, Logger: deps.Logger,
	})
	if err != nil {
		return application.MFAFlows{}, err
	}
	var capability identityaccess.AccountMFA = composedAccountMFA{flows: account, attach: runtime.attach}
	if deps.Capability != nil {
		capability = deps.Capability
	}
	operator, err := application.NewOperatorMFAFlows(service, application.OperatorMFAFlowDependencies{
		Records: operatorRecords, Codec: mfaChallengeCodec{deps.Codec}, Codes: deps.Codes,
		Mail: mfaMail{deps.Mail}, Audit: trail, Runtime: runtime, Secret: secret, Logger: deps.Logger,
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

// errMFANotComposed refuses a challenge on a module composed without the
// second factor.
var errMFANotComposed = errors.New("identity access: mfa is not composed")

// parseGateIP reads the client address the login carries as a string.
func parseGateIP(ipAddress string) net.IP {
	return net.ParseIP(strings.TrimSpace(ipAddress))
}

func mfaRuntime(sessions *SessionDependencies) tenantRuntime {
	attach := sessions.TenantRuntime
	if attach == nil {
		attach = func(ctx context.Context) context.Context { return ctx }
	}
	return tenantRuntime{attach: attach, runner: newTransactionRunner()}
}

// operatorMFAGate adapts the composed operator flows to the gate the
// operator login consults. Operator MFA is mandatory, so an unconfigured
// gate issues the token pair directly, as the early phases did.
type operatorMFAGate struct{ flows *application.OperatorMFAFlows }

// operatorLoginGate is the gate the operator login consults; the demo
// environment may leave it unconfigured (DemoDependencies.OperatorWithoutSecondFactor).
func operatorLoginGate(flows application.MFAFlows, demo *DemoDependencies) operatorMFAGate {
	if demo != nil && demo.OperatorWithoutSecondFactor {
		return operatorMFAGate{}
	}
	return operatorMFAGate{flows.Operator}
}

func (g operatorMFAGate) Configured() bool { return g.flows != nil }

func (g operatorMFAGate) HasEnrollment(ctx context.Context, operatorID int64) (bool, error) {
	if g.flows == nil {
		return false, nil
	}
	return g.flows.HasEnrollment(ctx, operatorID)
}

func (g operatorMFAGate) VerifyTrustedDevice(ctx context.Context, operatorID int64, cookie string) (bool, error) {
	if g.flows == nil {
		return false, nil
	}
	return g.flows.VerifyTrustedDevice(ctx, operatorID, cookie)
}

func (g operatorMFAGate) StartChallenge(ctx context.Context, operatorID int64, ipAddress string) (string, error) {
	if g.flows == nil {
		return "", errors.New("identity access: operator mfa is not composed")
	}
	return g.flows.StartChallenge(ctx, operatorID, net.ParseIP(strings.TrimSpace(ipAddress)))
}

func (g operatorMFAGate) TrustedDeviceDays() int {
	if g.flows == nil {
		return 0
	}
	return g.flows.TrustedDeviceDays()
}

// --- the seams the root fills ------------------------------------------------

type mfaChallengeCodec struct {
	source identityPorts.MFAChallengeCodec
}

func (c mfaChallengeCodec) IssueChallengeToken(claims domain.MFAChallengeClaims, ttl time.Duration) (string, error) {
	return c.source.IssueChallengeToken(identityaccess.MFAChallengeClaims(claims), ttl)
}

func (c mfaChallengeCodec) ParseChallengeToken(token string) (domain.MFAChallengeClaims, error) {
	claims, err := c.source.ParseChallengeToken(token)
	return domain.MFAChallengeClaims(claims), err
}

type mfaMail struct{ source identityPorts.MFAMail }

func (m mfaMail) DeliverCode(ctx context.Context, message domain.MFACodeMail) error {
	return m.source.DeliverCode(ctx, identityaccess.MFACodeMail(message))
}

func (m mfaMail) NotifyTrustedDeviceAdded(ctx context.Context, message domain.TrustedDeviceMail) {
	m.source.NotifyTrustedDeviceAdded(ctx, identityaccess.TrustedDeviceMail(message))
}

// mfaAuditTrail routes the account events to the authentication ledger and
// the operator entries to the operator action log.
type mfaAuditTrail struct {
	events   authAudit
	operator identityPorts.OperatorActionLog
}

func (t mfaAuditTrail) RecordAuthEvent(ctx context.Context, event domain.AuthEvent) error {
	return t.events.RecordAuthEvent(ctx, event)
}

func (t mfaAuditTrail) RecordOperatorActionAsync(entry domain.OperatorAuditEntry) {
	t.operator.RecordOperatorActionAsync(publicOperatorAuditEntry(entry))
}

// --- the account MFA records -------------------------------------------------

type accountMFARecords struct {
	source identityPorts.AccountMFARecords
}

func (r accountMFARecords) FindAccountIdentity(ctx context.Context, accountID int64) (domain.AccountIdentity, bool, error) {
	identity, found, err := r.source.FindAccountIdentity(ctx, accountID)
	return domain.AccountIdentity(identity), found, err
}

func (r accountMFARecords) AccountBelongsToTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	return r.source.AccountBelongsToTenant(ctx, accountID, tenantID)
}

func (r accountMFARecords) LockAccountForOverrideWrite(ctx context.Context, accountID int64) error {
	return r.source.LockAccountForOverrideWrite(ctx, accountID)
}

func (r accountMFARecords) IncrementMFAAttempts(ctx context.Context, accountID int64, threshold int, duration time.Duration) (domain.AccountLockout, error) {
	lockout, err := r.source.IncrementMFAAttempts(ctx, accountID, threshold, duration)
	return domain.AccountLockout(lockout), err
}

func (r accountMFARecords) ResetMFAAttempts(ctx context.Context, accountID int64) error {
	return r.source.ResetMFAAttempts(ctx, accountID)
}

func (r accountMFARecords) FindCredential(ctx context.Context, accountID int64) (domain.AccountMFACredential, bool, error) {
	credential, found, err := r.source.FindCredential(ctx, accountID)
	return domain.AccountMFACredential(credential), found, err
}

func (r accountMFARecords) CreateCredential(ctx context.Context, credential domain.AccountMFACredential) error {
	return r.source.CreateCredential(ctx, identityaccess.AccountMFACredential(credential))
}

func (r accountMFARecords) TouchCredential(ctx context.Context, id int64, usedAt time.Time) error {
	return r.source.TouchCredential(ctx, id, usedAt)
}

func (r accountMFARecords) DeleteCredentials(ctx context.Context, accountID int64) error {
	return r.source.DeleteCredentials(ctx, accountID)
}

func (r accountMFARecords) CreateChallenge(ctx context.Context, challenge domain.AccountMFAChallenge) (domain.AccountMFAChallenge, error) {
	stored, err := r.source.CreateChallenge(ctx, identityaccess.AccountMFAChallenge(challenge))
	return domain.AccountMFAChallenge(stored), err
}

func (r accountMFARecords) ActivateChallenge(ctx context.Context, id int64) error {
	return r.source.ActivateChallenge(ctx, id)
}

func (r accountMFARecords) ConsumeChallenge(ctx context.Context, id int64, consumedAt time.Time) error {
	return r.source.ConsumeChallenge(ctx, id, consumedAt)
}

func (r accountMFARecords) CountChallengesSince(ctx context.Context, accountID int64, since time.Time) (int, error) {
	return r.source.CountChallengesSince(ctx, accountID, since)
}

func (r accountMFARecords) FindActiveChallengeForAccount(ctx context.Context, id, accountID int64) (domain.AccountMFAChallenge, bool, error) {
	challenge, found, err := r.source.FindActiveChallengeForAccount(ctx, id, accountID)
	return domain.AccountMFAChallenge(challenge), found, err
}

func (r accountMFARecords) FindActiveChallengeInScope(ctx context.Context, accountID, tenantID int64, scope string) (domain.AccountMFAChallenge, bool, error) {
	challenge, found, err := r.source.FindActiveChallengeInScope(ctx, accountID, tenantID, scope)
	return domain.AccountMFAChallenge(challenge), found, err
}

func (r accountMFARecords) CreateTrustedDevice(ctx context.Context, device domain.AccountTrustedDevice) (domain.AccountTrustedDevice, error) {
	stored, err := r.source.CreateTrustedDevice(ctx, identityaccess.AccountTrustedDevice(device))
	return domain.AccountTrustedDevice(stored), err
}

func (r accountMFARecords) FindActiveTrustedDevice(ctx context.Context, accountID, tenantID int64, tokenHash string) (domain.AccountTrustedDevice, bool, error) {
	device, found, err := r.source.FindActiveTrustedDevice(ctx, accountID, tenantID, tokenHash)
	return domain.AccountTrustedDevice(device), found, err
}

func (r accountMFARecords) ListActiveTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]domain.AccountTrustedDevice, error) {
	devices, err := r.source.ListActiveTrustedDevices(ctx, accountID, tenantID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.AccountTrustedDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, domain.AccountTrustedDevice(device))
	}
	return result, nil
}

func (r accountMFARecords) TouchTrustedDevice(ctx context.Context, id int64, usedAt time.Time) error {
	return r.source.TouchTrustedDevice(ctx, id, usedAt)
}

func (r accountMFARecords) RevokeTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) error {
	return r.source.RevokeTrustedDevice(ctx, id, revokedAt)
}

func (r accountMFARecords) RevokeAllTrustedDevices(ctx context.Context, accountID int64, revokedAt time.Time) error {
	return r.source.RevokeAllTrustedDevices(ctx, accountID, revokedAt)
}

func (r accountMFARecords) RevokeTenantTrustedDevices(ctx context.Context, accountID, tenantID int64, revokedAt time.Time) error {
	return r.source.RevokeTenantTrustedDevices(ctx, accountID, tenantID, revokedAt)
}

func (r accountMFARecords) FindGlobalOverride(ctx context.Context, accountID int64) (domain.AccountMFAOverride, bool, error) {
	override, found, err := r.source.FindGlobalOverride(ctx, accountID)
	return domain.AccountMFAOverride(override), found, err
}

func (r accountMFARecords) FindTenantOverride(ctx context.Context, accountID, tenantID int64) (domain.AccountMFAOverride, bool, error) {
	override, found, err := r.source.FindTenantOverride(ctx, accountID, tenantID)
	return domain.AccountMFAOverride(override), found, err
}

func (r accountMFARecords) UpsertGlobalOverride(ctx context.Context, override domain.AccountMFAOverride) error {
	return r.source.UpsertGlobalOverride(ctx, identityaccess.AccountMFAOverride(override))
}

func (r accountMFARecords) UpsertTenantOverride(ctx context.Context, override domain.AccountMFAOverride) error {
	return r.source.UpsertTenantOverride(ctx, identityaccess.AccountMFAOverride(override))
}

func (r accountMFARecords) DeleteGlobalOverride(ctx context.Context, accountID int64) error {
	return r.source.DeleteGlobalOverride(ctx, accountID)
}

func (r accountMFARecords) DeleteTenantOverride(ctx context.Context, accountID, tenantID int64) error {
	return r.source.DeleteTenantOverride(ctx, accountID, tenantID)
}

var _ ports.AccountMFARecords = accountMFARecords{}

// --- the engine methods ------------------------------------------------------
