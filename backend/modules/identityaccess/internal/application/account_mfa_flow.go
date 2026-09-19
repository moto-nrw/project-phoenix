package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// AccountMFAFlows is the account's second factor (#3331): the gate that
// decides whether a login needs one, the e-mail challenge and its
// verification, the enrollment, the remember-device cookies and the admin
// overrides that force MFA on or off for one account.
//
// Every read the gate needs may fail, and none of them may fail open: an
// unreadable override, mode or enrollment refuses this one login with
// ErrMFAStatusUnavailable instead of silently dropping the second factor.
type AccountMFAFlows struct {
	records  ports.AccountMFARecords
	settings ports.MFASettings
	codec    ports.MFAChallengeCodec
	codes    ports.ShortCodeHasher
	mail     ports.MFAMail
	audit    ports.MFAAuditTrail
	runtime  ports.Runtime
	secret   []byte
	logger   *slog.Logger
	now      func() time.Time
}

// AccountMFAFlowDependencies are the ports the account MFA flows consume.
type AccountMFAFlowDependencies struct {
	Records  ports.AccountMFARecords
	Settings ports.MFASettings
	Codec    ports.MFAChallengeCodec
	Codes    ports.ShortCodeHasher
	Mail     ports.MFAMail
	Audit    ports.MFAAuditTrail
	Runtime  ports.Runtime
	// Secret is the trusted-device HMAC key derived from the JWT secret.
	Secret []byte
	Logger *slog.Logger
}

// NewAccountMFAFlows composes the flows.
func NewAccountMFAFlows(deps AccountMFAFlowDependencies) (*AccountMFAFlows, error) {
	switch {
	case deps.Records == nil, deps.Settings == nil, deps.Codec == nil, deps.Codes == nil,
		deps.Mail == nil, deps.Audit == nil, deps.Runtime == nil, len(deps.Secret) == 0:
		return nil, errors.New("identity access account mfa: all dependencies are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &AccountMFAFlows{
		records: deps.Records, settings: deps.Settings, codec: deps.Codec, codes: deps.Codes,
		mail: deps.Mail, audit: deps.Audit, runtime: deps.Runtime, secret: deps.Secret,
		logger: logger.With("component", "account-mfa"), now: time.Now,
	}, nil
}

// ===== The gate =====

// IsRequired applies the resolved policy to the account's roles. The caller
// passes the school explicitly: login runs outside the tenant middleware, so
// the context carries no school of its own.
func (f *AccountMFAFlows) IsRequired(ctx context.Context, accountID int64, roleNames []string, tenantID int64) (bool, error) {
	policy, err := f.ResolvePolicy(ctx, accountID, tenantID)
	if err != nil {
		return false, err
	}
	return policy.RequiredFor(roleNames), nil
}

// ResolvePolicy performs every read the gate needs and returns the verdict
// with the role predicate unapplied.
//
// Resolution order: the operator's platform-wide override wins, then the
// admin's school-scoped override, then the school's security.mfa_mode. A
// force_off at one school never bypasses MFA at another.
func (f *AccountMFAFlows) ResolvePolicy(ctx context.Context, accountID, tenantID int64) (domain.MFAPolicy, error) {
	return f.resolvePolicy(ctx, accountID, tenantID, false)
}

// ResolvePolicyInTx re-resolves the policy on the caller's transaction, past
// the request-scoped settings cache. The school-portal mint guard uses it:
// an admin switching security.mfa_mode from off to required while a login is
// in flight must not leave the guard evaluating the stale verdict.
func (f *AccountMFAFlows) ResolvePolicyInTx(ctx context.Context, accountID, tenantID int64) (domain.MFAPolicy, error) {
	return f.resolvePolicy(ctx, accountID, tenantID, true)
}

func (f *AccountMFAFlows) resolvePolicy(ctx context.Context, accountID, tenantID int64, inTx bool) (domain.MFAPolicy, error) {
	// Both override lookups run administratively. The login flow has no
	// school transaction yet, so the RLS policy on auth.mfa_overrides would
	// hide exactly the school-scoped row the resolution needs. Reading
	// across schools is safe here: the account is authenticated and the
	// school came from the already-validated slug.
	var (
		global, tenantOverride   domain.AccountMFAOverride
		globalFound, tenantFound bool
		overrideErr              error
	)
	load := func(txCtx context.Context) error {
		global, globalFound, overrideErr = f.records.FindGlobalOverride(txCtx, accountID)
		if overrideErr != nil {
			return overrideErr
		}
		if tenantID > 0 {
			tenantOverride, tenantFound, overrideErr = f.records.FindTenantOverride(txCtx, accountID, tenantID)
			if overrideErr != nil {
				return overrideErr
			}
		}
		return nil
	}
	// The in-transaction resolution joins the caller's administrative
	// transaction rather than opening a second one: taking another pooled
	// connection while the mint transaction holds one risks exhausting the
	// pool, and the caller's own transaction already sees everything
	// committed up to now, which is the freshness the re-read is for.
	if err := f.runtime.WithAdminTx(ctx, load); err != nil {
		f.logger.Warn("mfa override lookup failed; refusing login",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()))
		return domain.MFAPolicy{}, domain.ErrMFAStatusUnavailable
	}
	if globalFound {
		if policy, decided := domain.OverrideVerdict(global.Override); decided {
			return policy, nil
		}
	}
	if tenantFound {
		if policy, decided := domain.OverrideVerdict(tenantOverride.Override); decided {
			return policy, nil
		}
	}
	mode, err := f.resolveMode(ctx, tenantID, inTx)
	if err != nil {
		// A settings failure tells us neither that the school opted in nor
		// that it opted out. Failing open would downgrade security, failing
		// closed would lock everyone out on a registry hiccup: refuse this
		// one login, which the caller maps to 503.
		f.logger.Warn("mfa_mode resolve failed; refusing login",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()))
		return domain.MFAPolicy{}, domain.ErrMFAStatusUnavailable
	}
	if !domain.KnownMFAMode(mode) {
		f.logger.Warn("unknown mfa_mode value; treating as off", slog.String("value", mode))
	}
	return domain.MFAPolicyForMode(mode), nil
}

func (f *AccountMFAFlows) resolveMode(ctx context.Context, tenantID int64, inTx bool) (string, error) {
	var (
		mode string
		err  error
	)
	if inTx {
		mode, err = f.settings.MFAModeInTx(ctx, tenantID)
	} else {
		mode, err = f.settings.MFAMode(ctx, tenantID)
	}
	if err != nil {
		return "", err
	}
	if mode == "" {
		return domain.MFAModeOff, nil
	}
	return mode, nil
}

// HasEnrollment reports whether the account enrolled in e-mail MFA. Only a
// missing row is (false, nil); anything else refuses this login.
func (f *AccountMFAFlows) HasEnrollment(ctx context.Context, accountID int64) (bool, error) {
	credential, found, err := f.records.FindCredential(ctx, accountID)
	if err != nil {
		f.logger.Warn("mfa enrollment lookup failed; refusing login",
			slog.Int64("account_id", accountID),
			slog.String("error", err.Error()))
		return false, domain.ErrMFAStatusUnavailable
	}
	return found && credential.ID > 0, nil
}

// AccountBelongsToTenant reports the school mapping the operator MFA admin
// endpoints gate on.
func (f *AccountMFAFlows) AccountBelongsToTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	return f.records.AccountBelongsToTenant(ctx, accountID, tenantID)
}

// ===== Challenge and verification =====

// StartChallenge mails a fresh code for one portal and returns the challenge
// JWT that names its row. The row is stored consumed and only activated once
// the transport accepted the message, so a failed send leaves no redeemable
// code behind.
func (f *AccountMFAFlows) StartChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error) {
	if scope != domain.MFAChallengeScopeTenant && scope != domain.MFAChallengeScopeSchool {
		return "", domain.ErrMFAUnsupportedScope
	}
	account, found, err := f.records.FindAccountIdentity(ctx, accountID)
	if err != nil {
		return "", fmt.Errorf("look up account: %w", err)
	}
	if !found {
		return "", fmt.Errorf("look up account: %w", domain.ErrAccountNotFound)
	}
	if domain.MFALocked(account.MFALockedUntil, f.now()) {
		return "", domain.ErrMFALocked
	}

	// The hard cap of three codes per fifteen minutes is an abuse defense,
	// so a failed count refuses the send instead of waving it through:
	// ignoring the error meant the cap silently stopped existing exactly
	// when the database was least healthy.
	since := f.now().Add(-domain.MFAEmailRateLimitWindow)
	count, err := f.records.CountChallengesSince(ctx, accountID, since)
	if err != nil {
		f.logger.Warn("mfa code rate-limit lookup failed; refusing to issue a code",
			slog.Int64("account_id", accountID),
			slog.String("error", err.Error()))
		return "", domain.ErrMFAStatusUnavailable
	}
	if count >= domain.MFAEmailRateLimitMaxSent {
		return "", domain.ErrMFARateLimited
	}

	plainCode, err := domain.GenerateEmailCode()
	if err != nil {
		return "", fmt.Errorf("generate email code: %w", err)
	}
	codeHash, err := f.codes.HashShortCode(plainCode)
	if err != nil {
		return "", fmt.Errorf("hash email code: %w", err)
	}

	// The portal and the school travel with the row, not only with the JWT:
	// the enrollment-confirm and passkey-registration paths look a code up
	// by account, and only these columns keep that lookup inside its portal.
	createdAt := f.now()
	challenge, err := f.records.CreateChallenge(ctx, domain.AccountMFAChallenge{
		AccountID: accountID, Scope: scope, TenantID: tenantID, CodeHash: codeHash,
		ExpiresAt: createdAt.Add(domain.MFAChallengeTTL), ConsumedAt: &createdAt, IPAddress: ip,
		CreatedAt: createdAt, UpdatedAt: createdAt,
	})
	if err != nil {
		return "", fmt.Errorf("persist email challenge: %w", err)
	}

	enabled, days := f.trustedDeviceHint(ctx, tenantID)
	if err := f.mail.DeliverCode(ctx, domain.MFACodeMailFor(
		account.Email, "", account.ID, false, plainCode, domain.MFAChallengeTTL, ip, enabled, days,
	)); err != nil {
		f.logger.Warn("mfa challenge delivery failed",
			slog.Int64("account_id", account.ID),
			slog.String("error", err.Error()))
		return "", domain.ErrMFAStatusUnavailable
	}
	if err := f.records.ActivateChallenge(ctx, challenge.ID); err != nil {
		f.logger.Error("failed to activate delivered mfa challenge",
			slog.Int64("challenge_id", challenge.ID),
			slog.String("error", err.Error()))
		return "", domain.ErrMFAStatusUnavailable
	}
	f.recordAuthEvent(ctx, domain.AuthEvent{
		AccountID: accountID, TenantID: tenantID, Type: domain.AuthEventMFAEmailSent, Success: true,
		IPAddress: domain.MFAAuditIP(ip), MFA: &domain.MFAEvidence{ChallengeID: challenge.ID},
	})

	token, err := f.codec.IssueChallengeToken(domain.MFAChallengeClaims{
		AccountID: accountID, Scope: scope, TenantID: tenantID, ChallengeID: challenge.ID,
	}, domain.MFAChallengeTTL)
	if err != nil {
		return "", fmt.Errorf("mint challenge jwt: %w", err)
	}
	return token, nil
}

// challengeOwner pins a challenge to the identity the caller already
// authenticated out of band.
type challengeOwner struct {
	accountID int64
	tenantID  int64
}

// VerifyChallenge verifies a tenant-portal challenge.
func (f *AccountMFAFlows) VerifyChallenge(ctx context.Context, challengeToken, code string) (domain.VerifiedMFAChallenge, error) {
	return f.VerifyChallengeForScope(ctx, challengeToken, code, domain.MFAChallengeScopeTenant)
}

// VerifyChallengeForScope refuses a challenge started for another portal
// before any code is compared, so a school code can never be burned — or
// redeemed — at the tenant endpoint and the other way round.
func (f *AccountMFAFlows) VerifyChallengeForScope(ctx context.Context, challengeToken, code, expectedScope string) (domain.VerifiedMFAChallenge, error) {
	return f.verifyChallengeBound(ctx, challengeToken, code, expectedScope, nil)
}

// VerifyChallengeForOwner additionally pins the challenge to an account and
// a school the caller authenticated by other means. The ownership check runs
// before the code comparison and therefore before the single-use consume:
// checking the result afterwards burns a foreign challenge on its way to the
// 401, which kills the rightful owner's in-flight login.
func (f *AccountMFAFlows) VerifyChallengeForOwner(ctx context.Context, challengeToken, code, expectedScope string, accountID, tenantID int64) (domain.VerifiedMFAChallenge, error) {
	return f.verifyChallengeBound(ctx, challengeToken, code, expectedScope, &challengeOwner{accountID: accountID, tenantID: tenantID})
}

func (f *AccountMFAFlows) verifyChallengeBound(ctx context.Context, challengeToken, code, expectedScope string, owner *challengeOwner) (domain.VerifiedMFAChallenge, error) {
	claims, err := f.codec.ParseChallengeToken(challengeToken)
	if err != nil {
		return domain.VerifiedMFAChallenge{}, domain.ErrMFAChallengeTokenInvalid
	}
	if claims.Scope != expectedScope {
		return domain.VerifiedMFAChallenge{}, domain.ErrMFAUnsupportedScope
	}
	// A foreign challenge is refused while it is still redeemable by
	// whoever it was minted for.
	if owner != nil && (claims.AccountID != owner.accountID || claims.TenantID != owner.tenantID) {
		return domain.VerifiedMFAChallenge{}, domain.ErrMFAChallengeTokenInvalid
	}
	account, found, err := f.records.FindAccountIdentity(ctx, claims.AccountID)
	if err != nil || !found {
		return domain.VerifiedMFAChallenge{}, domain.ErrMFAChallengeTokenInvalid
	}
	if domain.MFALocked(account.MFALockedUntil, f.now()) {
		return domain.VerifiedMFAChallenge{}, domain.ErrMFALocked
	}
	// The token names the exact row it was minted for. "Newest active code
	// for this account" would let two concurrent challenges — a tenant login
	// and a school login of the same person — redeem each other's code. A
	// token without the claim predates it and is refused rather than
	// downgraded to the ambiguous lookup.
	if claims.ChallengeID == 0 {
		return domain.VerifiedMFAChallenge{}, domain.ErrMFAChallengeTokenInvalid
	}
	active, activeFound, err := f.records.FindActiveChallengeForAccount(ctx, claims.ChallengeID, claims.AccountID)
	if err != nil || !activeFound {
		f.recordFailure(ctx, claims.AccountID, claims.TenantID, "no active challenge")
		return domain.VerifiedMFAChallenge{}, domain.ErrMFACodeInvalid
	}
	// The scope check above trusts the JWT alone. Re-check it against the
	// stored row so a forged or mixed-up pairing is refused before the code
	// comparison rather than after it.
	if active.Scope != expectedScope || (claims.TenantID > 0 && active.TenantID != claims.TenantID) {
		f.recordFailure(ctx, claims.AccountID, claims.TenantID, "challenge portal mismatch")
		return domain.VerifiedMFAChallenge{}, domain.ErrMFAUnsupportedScope
	}

	ok, verifyErr := f.codes.VerifyShortCode(code, active.CodeHash)
	if verifyErr != nil || !ok {
		f.handleFailedAttempt(ctx, account, claims.TenantID)
		f.recordFailure(ctx, claims.AccountID, claims.TenantID, "code mismatch")
		return domain.VerifiedMFAChallenge{}, domain.ErrMFACodeInvalid
	}

	// Single use. The consume is one conditional UPDATE, so two concurrent
	// verifications race on it: the loser must be refused, or both mint a
	// session from one code. Any consume failure is treated the same way —
	// without proof of single use we refuse.
	now := f.now()
	if err := f.records.ConsumeChallenge(ctx, active.ID, now); err != nil {
		f.logger.Warn("failed to mark challenge consumed; refusing verify",
			slog.Int64("account_id", claims.AccountID),
			slog.Int64("challenge_id", active.ID),
			slog.String("error", err.Error()))
		f.recordFailure(ctx, claims.AccountID, claims.TenantID, "consume race")
		return domain.VerifiedMFAChallenge{}, domain.ErrMFACodeInvalid
	}
	// One UPDATE, so a successful verify cannot clobber a concurrent failed
	// verify's increment with stale in-memory state.
	if err := f.records.ResetMFAAttempts(ctx, account.ID); err != nil {
		f.logger.Warn("failed to reset MFA attempts", slog.String("error", err.Error()))
	}
	if credential, credFound, credErr := f.records.FindCredential(ctx, claims.AccountID); credErr == nil && credFound && credential.ID > 0 {
		_ = f.records.TouchCredential(ctx, credential.ID, now)
	}
	f.recordAuthEvent(ctx, domain.AuthEvent{
		AccountID: claims.AccountID, TenantID: claims.TenantID, Type: domain.AuthEventMFAVerified,
		Success: true, IPAddress: domain.MFAAuditFallbackIP,
	})
	return domain.VerifiedMFAChallenge{AccountID: claims.AccountID, Scope: claims.Scope, TenantID: claims.TenantID}, nil
}

// VerifyCodeForAccount runs the same pipeline without the JWT round trip:
// the caller authenticated the user out of band (an enrollment token, a
// tenant session at passkey registration).
//
// Without a challenge id the row is resolved as "the newest active code of
// this account", which is only safe once narrowed to the portal asking.
// expectedScope and tenantID are therefore required, not decorative:
// without them a Lehrkraft with a school login challenge in one tab could
// have that code consumed here and a tenant session minted from a second
// factor issued for another portal.
func (f *AccountMFAFlows) VerifyCodeForAccount(ctx context.Context, accountID, tenantID int64, code, expectedScope string) error {
	account, found, err := f.records.FindAccountIdentity(ctx, accountID)
	if err != nil || !found {
		return domain.ErrMFACodeInvalid
	}
	if domain.MFALocked(account.MFALockedUntil, f.now()) {
		return domain.ErrMFALocked
	}
	// The callers run inside a school context, so the zero school below lets
	// the audit and the lockout fall back to the context. The tenantID
	// argument narrows the challenge lookup, which is a different job.
	active, activeFound, err := f.records.FindActiveChallengeInScope(ctx, accountID, tenantID, expectedScope)
	if err != nil || !activeFound {
		f.recordFailure(ctx, accountID, 0, "no active challenge")
		return domain.ErrMFACodeInvalid
	}
	ok, verifyErr := f.codes.VerifyShortCode(code, active.CodeHash)
	if verifyErr != nil || !ok {
		f.handleFailedAttempt(ctx, account, 0)
		f.recordFailure(ctx, accountID, 0, "code mismatch")
		return domain.ErrMFACodeInvalid
	}
	if err := f.records.ConsumeChallenge(ctx, active.ID, f.now()); err != nil {
		f.logger.Warn("failed to mark challenge consumed; refusing verify",
			slog.Int64("account_id", accountID),
			slog.Int64("challenge_id", active.ID),
			slog.String("error", err.Error()))
		f.recordFailure(ctx, accountID, 0, "consume race")
		return domain.ErrMFACodeInvalid
	}
	_ = f.records.ResetMFAAttempts(ctx, accountID)
	f.recordAuthEvent(ctx, domain.AuthEvent{
		AccountID: accountID, Type: domain.AuthEventMFAVerified, Success: true,
		IPAddress: domain.MFAAuditFallbackIP,
	})
	return nil
}

// ResendChallengeForScope is the variant every portal resend endpoint uses:
// no surface may re-drive a foreign portal's challenge and burn its budget.
func (f *AccountMFAFlows) ResendChallengeForScope(ctx context.Context, challengeToken string, ip net.IP, expectedScope string) (string, error) {
	claims, err := f.codec.ParseChallengeToken(challengeToken)
	if err != nil {
		return "", domain.ErrMFAChallengeTokenInvalid
	}
	if claims.Scope != expectedScope {
		return "", domain.ErrMFAUnsupportedScope
	}
	return f.resend(ctx, claims, ip)
}

// ResendChallenge accepts a challenge of any scope and is therefore not for
// HTTP surfaces; every portal endpoint uses ResendChallengeForScope.
func (f *AccountMFAFlows) ResendChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	claims, err := f.codec.ParseChallengeToken(challengeToken)
	if err != nil {
		return "", domain.ErrMFAChallengeTokenInvalid
	}
	return f.resend(ctx, claims, ip)
}

// resend has no cooldown of its own: the sliding window inside StartChallenge
// remains the abuse defense. It returns the renewed JWT so the frontend can
// replace its in-flight token — otherwise the fresh code becomes
// unverifiable the moment the original token expires.
func (f *AccountMFAFlows) resend(ctx context.Context, claims domain.MFAChallengeClaims, ip net.IP) (string, error) {
	return f.StartChallenge(ctx, claims.AccountID, claims.TenantID, claims.Scope, ip)
}

// handleFailedAttempt bumps the lockout counter atomically and records the
// cooldown exactly when this increment crossed the threshold. A read-modify-
// write would let two concurrent failed verifies collapse into one counted
// attempt and double an attacker's budget.
func (f *AccountMFAFlows) handleFailedAttempt(ctx context.Context, account domain.AccountIdentity, tenantID int64) {
	threshold := f.lockoutThreshold(ctx, tenantID)
	result, err := f.records.IncrementMFAAttempts(ctx, account.ID, threshold, f.lockoutDuration(ctx, tenantID))
	if err != nil {
		f.logger.Warn("failed to persist MFA attempt counter", slog.String("error", err.Error()))
		return
	}
	// result.Attempts == threshold means this increment was the N-th, and
	// only this caller sees it. Later attempts push the window forward
	// without re-emitting.
	if result.Attempts == threshold {
		f.recordAuthEvent(ctx, domain.AuthEvent{
			AccountID: account.ID, TenantID: tenantID, Type: domain.AuthEventMFALocked,
			IPAddress: domain.MFAAuditFallbackIP, MFA: &domain.MFAEvidence{LockedUntil: result.LockedUntil},
		})
	}
}

func (f *AccountMFAFlows) lockoutThreshold(ctx context.Context, tenantID int64) int {
	value, err := f.settings.LockoutThreshold(ctx, tenantID)
	if err != nil || value <= 0 {
		return domain.MFALockoutThreshold
	}
	return value
}

func (f *AccountMFAFlows) lockoutDuration(ctx context.Context, tenantID int64) time.Duration {
	value, err := f.settings.LockoutDuration(ctx, tenantID)
	if err != nil || value <= 0 {
		return domain.MFALockoutDuration
	}
	return value
}

// ===== Enrollment =====

// Enroll records the account's e-mail enrollment once.
func (f *AccountMFAFlows) Enroll(ctx context.Context, accountID int64) error {
	existing, found, _ := f.records.FindCredential(ctx, accountID)
	if found && existing.ID > 0 {
		return domain.ErrMFAAlreadyEnrolled
	}
	if err := f.records.CreateCredential(ctx, domain.AccountMFACredential{
		AccountID: accountID, Method: domain.MFAMethodEmail, EnrolledAt: f.now(),
	}); err != nil {
		return fmt.Errorf("persist mfa credential: %w", err)
	}
	return nil
}

// Disable cascades: the credential goes, every trusted device is revoked and
// the lockout counter is reset. The three writes share one transaction so a
// partial failure cannot leave the account half-disabled — a credential gone
// while devices still verify, say.
func (f *AccountMFAFlows) Disable(ctx context.Context, accountID int64) error {
	err := f.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		if err := f.records.DeleteCredentials(txCtx, accountID); err != nil {
			return fmt.Errorf("delete credential: %w", err)
		}
		if err := f.records.RevokeAllTrustedDevices(txCtx, accountID, f.now()); err != nil {
			return fmt.Errorf("revoke trusted devices: %w", err)
		}
		if err := f.records.ResetMFAAttempts(txCtx, accountID); err != nil {
			return fmt.Errorf("reset mfa attempts: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Disable runs from authenticated endpoints, so the context already
	// carries the school.
	f.recordAuthEvent(ctx, domain.AuthEvent{
		AccountID: accountID, Type: domain.AuthEventMFADisabled, Success: true,
		IPAddress: domain.MFAAuditFallbackIP,
	})
	return nil
}

// ===== Trusted devices =====

// IsTrustedDeviceEnabled reports the school's
// security.mfa_trusted_device_enabled. A failed read disables the feature:
// surprising the user with a missing checkbox beats ignoring an admin's
// opt-out.
func (f *AccountMFAFlows) IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool {
	enabled, err := f.settings.TrustedDeviceEnabled(ctx, tenantID)
	if err != nil {
		f.logger.Warn("trusted_device_enabled resolve failed; disabling feature",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()))
		return false
	}
	return enabled
}

// TrustedDeviceDays resolves the school's remember-device lifetime, falling
// back to the default so the cookie still issues with a sane one.
func (f *AccountMFAFlows) TrustedDeviceDays(ctx context.Context, tenantID int64) int {
	days, err := f.settings.TrustedDeviceDays(ctx, tenantID)
	if err != nil || days <= 0 {
		return domain.MFATrustedDeviceCookieDefaultDays
	}
	return days
}

// IssueTrustedDevice mints a remember-device cookie for one school. It
// answers an empty value without persisting a row when the school turned the
// feature off, and the caller then skips the Set-Cookie write.
func (f *AccountMFAFlows) IssueTrustedDevice(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (string, time.Time, error) {
	if !f.IsTrustedDeviceEnabled(ctx, tenantID) {
		return "", time.Time{}, nil
	}
	days := f.TrustedDeviceDays(ctx, tenantID)
	expiresAt := f.now().Add(time.Duration(days) * 24 * time.Hour)
	rawToken, err := domain.GenerateTrustedDeviceToken()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate trusted-device token: %w", err)
	}
	device := domain.AccountTrustedDevice{
		AccountID: accountID,
		// The row carries the school, so a cookie issued at school A can
		// never bypass MFA at school B.
		TenantID:  tenantID,
		TokenHash: domain.HashTrustedDeviceToken(rawToken),
		IPAddress: ip,
		ExpiresAt: expiresAt,
	}
	if userAgent != "" {
		label := userAgent
		device.UserAgent = &label
	}
	stored, err := f.records.CreateTrustedDevice(ctx, device)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("persist trusted device: %w", err)
	}
	signed := domain.SignTrustedDeviceToken(rawToken, f.secret)
	f.recordAuthEvent(ctx, domain.AuthEvent{
		AccountID: accountID, TenantID: tenantID, Type: domain.AuthEventMFATrustedDeviceAdded,
		Success: true, IPAddress: domain.MFAAuditIP(ip), MFA: &domain.MFAEvidence{DeviceID: stored.ID},
	})
	// Fire-and-forget: remembering a device must never happen silently, but
	// a failed notification must not break the login either.
	f.notifyTrustedDeviceAdded(ctx, accountID, userAgent, ip, days)
	return signed, expiresAt, nil
}

// VerifyTrustedDevice reports whether the cookie names an active device of
// this account at this school. A school that turned the feature off after a
// cookie was issued rejects it immediately rather than waiting for expiry.
func (f *AccountMFAFlows) VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, signedCookie string) (bool, error) {
	if !f.IsTrustedDeviceEnabled(ctx, tenantID) {
		return false, nil
	}
	rawToken, ok := domain.VerifyTrustedDeviceToken(signedCookie, f.secret)
	if !ok {
		return false, nil
	}
	device, found, err := f.records.FindActiveTrustedDevice(ctx, accountID, tenantID, domain.HashTrustedDeviceToken(rawToken))
	if err != nil || !found {
		return false, nil
	}
	_ = f.records.TouchTrustedDevice(ctx, device.ID, f.now())
	return true, nil
}

// ListTrustedDevices returns the account's active devices at one school.
// Trust is per (account, school), so the settings page only shows the
// devices trusted while logged into the school the user is in.
func (f *AccountMFAFlows) ListTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]domain.AccountTrustedDevice, error) {
	return f.records.ListActiveTrustedDevices(ctx, accountID, tenantID)
}

// RevokeTrustedDevice revokes one device after proving it belongs to the
// calling account at the calling school, so neither an id guess nor a token
// from another school can revoke it.
func (f *AccountMFAFlows) RevokeTrustedDevice(ctx context.Context, accountID, tenantID, deviceID int64) error {
	devices, err := f.records.ListActiveTrustedDevices(ctx, accountID, tenantID)
	if err != nil {
		return err
	}
	for _, device := range devices {
		if device.ID == deviceID {
			return f.records.RevokeTrustedDevice(ctx, deviceID, f.now())
		}
	}
	return domain.ErrMFAPermissionDenied
}

func (f *AccountMFAFlows) trustedDeviceHint(ctx context.Context, tenantID int64) (bool, int) {
	if !f.IsTrustedDeviceEnabled(ctx, tenantID) {
		return false, 0
	}
	return true, f.TrustedDeviceDays(ctx, tenantID)
}

func (f *AccountMFAFlows) notifyTrustedDeviceAdded(ctx context.Context, accountID int64, userAgent string, ip net.IP, days int) {
	account, found, err := f.records.FindAccountIdentity(ctx, accountID)
	if err != nil || !found || account.Email == "" {
		f.logger.Warn("could not load account for trusted-device-added mail",
			slog.Int64("account_id", accountID),
			slog.Any("error", err))
		return
	}
	f.mail.NotifyTrustedDeviceAdded(ctx, domain.TrustedDeviceMail{
		Recipient: account.Email, ReferenceID: accountID,
		DeviceLabel: domain.ShortenUserAgent(userAgent), RequestIP: domain.AuditIPString(ip),
		AddedAt: f.now().Format("02.01.2006 15:04"), TrustedDays: days,
	})
}

// ===== Audit =====

func (f *AccountMFAFlows) recordFailure(ctx context.Context, accountID, tenantID int64, reason string) {
	f.recordAuthEvent(ctx, domain.AuthEvent{
		AccountID: accountID, TenantID: tenantID, Type: domain.AuthEventMFAFailed,
		IPAddress: domain.MFAAuditFallbackIP, ErrorMessage: reason,
	})
}

// recordAuthEvent appends to the authentication ledger. It joins the
// caller's transaction when one is active and otherwise opens the school's
// own, because the login flow runs outside the tenant middleware and its
// events would be dropped without an explicit school.
func (f *AccountMFAFlows) recordAuthEvent(ctx context.Context, event domain.AuthEvent) {
	if event.TenantID == 0 {
		event.TenantID = f.runtime.TenantID(ctx)
	}
	if event.TenantID == 0 {
		f.logger.Error("failed to audit mfa event",
			slog.String("event_type", event.Type),
			slog.String("error", "tenant is not configured"))
		return
	}
	if event.IPAddress == "" {
		event.IPAddress = domain.MFAAuditFallbackIP
	}
	appendEvent := func(txCtx context.Context) error { return f.audit.RecordAuthEvent(txCtx, event) }
	var err error
	if f.runtime.HasTransaction(ctx) {
		err = appendEvent(ctx)
	} else {
		// The event belongs to the school it names, which is not always the
		// school the request runs under: an operator override audits at the
		// target's school. Naming it on the context first keeps the write
		// from being refused as a mismatched nesting and silently dropping
		// the row.
		err = f.runtime.WithTenantTx(f.runtime.WithTenantID(ctx, event.TenantID), event.TenantID, appendEvent)
	}
	if err != nil {
		f.logger.Error("failed to audit mfa event",
			slog.String("event_type", event.Type),
			slog.String("error", err.Error()))
	}
}
