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
)

// The MFA and passkey composition (#3331). The account MFA rows still live
// in the retained repositories until #3226 moves them, so the root binds
// them through AccountMFARecords; everything else the flows need — the
// school settings, the challenge codec, the code hasher, the two mails and
// the operator action log — is a consumer-owned seam the root fills.

// AccountMFARecords is the account MFA persistence the root binds. A missing
// row is reported as found=false, never as an error; a state change that did
// not apply is an error, which is what keeps a code single-use.
type AccountMFARecords interface {
	FindAccountIdentity(ctx context.Context, accountID int64) (identityaccess.AccountMFAIdentity, bool, error)
	AccountBelongsToTenant(ctx context.Context, accountID, tenantID int64) (bool, error)
	// LockAccountForOverrideWrite takes the account row FOR UPDATE ahead of
	// an override write, so a session mint and an override cannot interleave
	// unordered. A missing account is not an error.
	LockAccountForOverrideWrite(ctx context.Context, accountID int64) error
	IncrementMFAAttempts(ctx context.Context, accountID int64, threshold int, duration time.Duration) (identityaccess.AccountLockout, error)
	ResetMFAAttempts(ctx context.Context, accountID int64) error

	FindCredential(ctx context.Context, accountID int64) (identityaccess.AccountMFACredential, bool, error)
	CreateCredential(ctx context.Context, credential identityaccess.AccountMFACredential) error
	TouchCredential(ctx context.Context, id int64, usedAt time.Time) error
	DeleteCredentials(ctx context.Context, accountID int64) error

	CreateChallenge(ctx context.Context, challenge identityaccess.AccountMFAChallenge) (identityaccess.AccountMFAChallenge, error)
	ActivateChallenge(ctx context.Context, id int64) error
	ConsumeChallenge(ctx context.Context, id int64, consumedAt time.Time) error
	CountChallengesSince(ctx context.Context, accountID int64, since time.Time) (int, error)
	FindActiveChallengeForAccount(ctx context.Context, id, accountID int64) (identityaccess.AccountMFAChallenge, bool, error)
	FindActiveChallengeInScope(ctx context.Context, accountID, tenantID int64, scope string) (identityaccess.AccountMFAChallenge, bool, error)

	CreateTrustedDevice(ctx context.Context, device identityaccess.AccountTrustedDevice) (identityaccess.AccountTrustedDevice, error)
	FindActiveTrustedDevice(ctx context.Context, accountID, tenantID int64, tokenHash string) (identityaccess.AccountTrustedDevice, bool, error)
	ListActiveTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]identityaccess.AccountTrustedDevice, error)
	TouchTrustedDevice(ctx context.Context, id int64, usedAt time.Time) error
	RevokeTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) error
	RevokeAllTrustedDevices(ctx context.Context, accountID int64, revokedAt time.Time) error
	RevokeTenantTrustedDevices(ctx context.Context, accountID, tenantID int64, revokedAt time.Time) error

	FindGlobalOverride(ctx context.Context, accountID int64) (identityaccess.AccountMFAOverride, bool, error)
	FindTenantOverride(ctx context.Context, accountID, tenantID int64) (identityaccess.AccountMFAOverride, bool, error)
	UpsertGlobalOverride(ctx context.Context, override identityaccess.AccountMFAOverride) error
	UpsertTenantOverride(ctx context.Context, override identityaccess.AccountMFAOverride) error
	DeleteGlobalOverride(ctx context.Context, accountID int64) error
	DeleteTenantOverride(ctx context.Context, accountID, tenantID int64) error
}

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

// MFAChallengeCodec signs and parses the short-lived challenge JWTs.
type MFAChallengeCodec interface {
	IssueChallengeToken(claims identityaccess.MFAChallengeClaims, ttl time.Duration) (string, error)
	ParseChallengeToken(token string) (identityaccess.MFAChallengeClaims, error)
}

// ShortCodeHasher hashes and verifies the e-mail codes through the
// project-wide Argon2id helper.
type ShortCodeHasher interface {
	HashShortCode(plain string) (string, error)
	VerifyShortCode(plain, encodedHash string) (bool, error)
}

// MFAMail sends the two MFA messages. DeliverCode is synchronous and
// fail-closed: the module activates a code only after the transport accepted
// it. NotifyTrustedDeviceAdded is fire-and-forget.
type MFAMail interface {
	DeliverCode(ctx context.Context, message identityaccess.MFACodeMail) error
	NotifyTrustedDeviceAdded(ctx context.Context, message identityaccess.TrustedDeviceMail)
}

// OperatorActionLog appends an operator entry outside the caller's outcome,
// so the request never pays for the insert and a failed append never fails a
// login.
type OperatorActionLog interface {
	RecordOperatorActionAsync(entry identityaccess.OperatorAuditEntry)
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
	Records       AccountMFARecords
	Settings      MFASettings
	Codec         MFAChallengeCodec
	Codes         ShortCodeHasher
	Mail          MFAMail
	OperatorAudit OperatorActionLog
	// JWTSecret derives the trusted-device HMAC key, so the cookie signature
	// and the JWT signing key stay independent.
	JWTSecret string
	Passkeys  *PasskeyDependencies
	Logger    *slog.Logger
	// Capability lets a composition serve the account second factor itself
	// instead of composing it over Records. The service root leaves it nil;
	// a root that already holds the capability — a behaviour test driving
	// the login branches, a future in-process fake — supplies it here, and
	// the login gate, the MFA routes and the passkey registration all reach
	// that one implementation.
	Capability identityaccess.AccountMFA
}

// mfaFlows is what New composes from the MFA dependencies. Its members are
// nil when a root composed the module without them.
type mfaFlows struct {
	account *application.AccountMFAFlows
	// capability is what the engine and the gate reach the account second
	// factor through: the composed flows, or the one a root supplied.
	capability      identityaccess.AccountMFA
	operator        *application.OperatorMFAFlows
	accountPasskey  *application.AccountPasskeyFlows
	operatorPasskey *application.OperatorPasskeyFlows
}

// newMFACore composes the second factor itself. It runs before the session
// flows, because the login paths consult the gate it produces; the passkey
// ceremonies come after them, because a completed ceremony mints a session.
func newMFACore(
	service *application.Service,
	operatorRecords *application.OperatorMFA,
	sessions *SessionDependencies,
	deps *MFADependencies,
) (mfaFlows, error) {
	if deps == nil {
		return mfaFlows{}, nil
	}
	if sessions == nil {
		return mfaFlows{}, errors.New("identity access compose: mfa requires the session dependencies")
	}
	switch {
	case deps.Records == nil, deps.Settings == nil, deps.Codec == nil, deps.Codes == nil,
		deps.Mail == nil, deps.OperatorAudit == nil, strings.TrimSpace(deps.JWTSecret) == "":
		return mfaFlows{}, errors.New("identity access compose: every mfa dependency is required")
	}
	runtime := mfaRuntime(sessions)
	trail := mfaAuditTrail{events: authAudit{sessions.Audit}, operator: deps.OperatorAudit}
	secret := domain.DeriveMFASecret(deps.JWTSecret)

	account, err := application.NewAccountMFAFlows(application.AccountMFAFlowDependencies{
		Records: accountMFARecords{source: deps.Records}, Settings: mfaSettings{deps.Settings},
		Codec: mfaChallengeCodec{deps.Codec}, Codes: deps.Codes, Mail: mfaMail{deps.Mail},
		Audit: trail, Runtime: runtime, Secret: secret, Logger: deps.Logger,
	})
	if err != nil {
		return mfaFlows{}, err
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
		return mfaFlows{}, err
	}
	return mfaFlows{account: account, capability: capability, operator: operator}, nil
}

// withPasskeyFlows composes both portals' ceremonies once the session flows
// exist and returns the second factor carrying them. A composition without
// the relying-party facts keeps them nil, and every ceremony then reports
// ErrPasskeyFlowsUnavailable.
func withPasskeyFlows(
	flows mfaFlows,
	service *application.Service,
	auth *application.AccountAuthentication,
	operatorAuth *application.OperatorAuthentication,
	accountPasskeys *application.AccountPasskey,
	operatorPasskeys *application.OperatorPasskey,
	sessions *SessionDependencies,
	deps *MFADependencies,
) (mfaFlows, error) {
	if deps == nil || deps.Passkeys == nil {
		return flows, nil
	}
	if auth == nil || operatorAuth == nil {
		return mfaFlows{}, errors.New("identity access compose: the passkey ceremonies require the session and operator flows")
	}
	runtime := mfaRuntime(sessions)
	accountFlows, err := application.NewAccountPasskeyFlows(application.AccountPasskeyFlowDependencies{
		Accounts: accountMFARecords{source: deps.Records}, Records: accountPasskeys, MFA: flows.account,
		Sessions: auth, Runtime: runtime, RPID: deps.Passkeys.RPID, RPName: deps.Passkeys.RPName,
		TenantDomain: deps.Passkeys.TenantDomain, Logger: deps.Logger,
	})
	if err != nil {
		return mfaFlows{}, err
	}
	operatorFlows, err := application.NewOperatorPasskeyFlows(service, application.OperatorPasskeyFlowDependencies{
		Records: operatorPasskeys, MFA: flows.operator, Sessions: operatorAuth, Runtime: runtime,
		RPID: deps.Passkeys.RPID, RPName: deps.Passkeys.RPName,
		OperatorFrontendURL: deps.Passkeys.OperatorFrontendURL, Logger: deps.Logger,
	})
	if err != nil {
		return mfaFlows{}, err
	}
	// The ceremonies are carried in a new value rather than written into the
	// composed one: the composition surface stays free of mutable wiring.
	return mfaFlows{
		account: flows.account, capability: flows.capability, operator: flows.operator,
		accountPasskey: accountFlows, operatorPasskey: operatorFlows,
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

type mfaSettings struct{ source MFASettings }

func (s mfaSettings) MFAMode(ctx context.Context, tenantID int64) (string, error) {
	return s.source.MFAMode(ctx, tenantID)
}

func (s mfaSettings) MFAModeInTx(ctx context.Context, tenantID int64) (string, error) {
	return s.source.MFAModeInTx(ctx, tenantID)
}

func (s mfaSettings) TrustedDeviceEnabled(ctx context.Context, tenantID int64) (bool, error) {
	return s.source.TrustedDeviceEnabled(ctx, tenantID)
}

func (s mfaSettings) TrustedDeviceDays(ctx context.Context, tenantID int64) (int, error) {
	return s.source.TrustedDeviceDays(ctx, tenantID)
}

func (s mfaSettings) LockoutThreshold(ctx context.Context, tenantID int64) (int, error) {
	return s.source.LockoutThreshold(ctx, tenantID)
}

func (s mfaSettings) LockoutDuration(ctx context.Context, tenantID int64) (time.Duration, error) {
	return s.source.LockoutDuration(ctx, tenantID)
}

type mfaChallengeCodec struct{ source MFAChallengeCodec }

func (c mfaChallengeCodec) IssueChallengeToken(claims domain.MFAChallengeClaims, ttl time.Duration) (string, error) {
	return c.source.IssueChallengeToken(identityaccess.MFAChallengeClaims(claims), ttl)
}

func (c mfaChallengeCodec) ParseChallengeToken(token string) (domain.MFAChallengeClaims, error) {
	claims, err := c.source.ParseChallengeToken(token)
	return domain.MFAChallengeClaims(claims), err
}

type mfaMail struct{ source MFAMail }

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
	operator OperatorActionLog
}

func (t mfaAuditTrail) RecordAuthEvent(ctx context.Context, event domain.AuthEvent) error {
	return t.events.RecordAuthEvent(ctx, event)
}

func (t mfaAuditTrail) RecordOperatorAction(entry domain.OperatorAuditEntry) {
	t.operator.RecordOperatorActionAsync(publicOperatorAuditEntry(entry))
}

// --- the account MFA records -------------------------------------------------

type accountMFARecords struct{ source AccountMFARecords }

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

func (e engine) accountMFA() (identityaccess.AccountMFA, error) {
	if e.mfaFlows.capability == nil {
		return nil, identityaccess.ErrAccountMFAUnavailable
	}
	return e.mfaFlows.capability, nil
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
	return mfa.OperatorDisableMFA(ctx, operatorID, schoolID, targetAccountID, reason)
}

func (e engine) OperatorSetMFAOverride(ctx context.Context, operatorID, schoolID, targetAccountID int64, override, reason string) error {
	mfa, err := e.accountMFA()
	if err != nil {
		return err
	}
	return mfa.OperatorSetMFAOverride(ctx, operatorID, schoolID, targetAccountID, override, reason)
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

// mfaError is the one translation the MFA flows use: the operation
// envelope keeps its text and every internal sentinel gains its public
// counterpart, so a caller switches on one identity across the boundary.
func mfaError(err error) error { return authenticationError(err) }
