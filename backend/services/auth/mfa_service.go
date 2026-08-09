package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/auth"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// MFA constants enforcing key policies. Tunable via Settings where appropriate.
const (
	MFAChallengeTTL                   = 10 * time.Minute // email code TTL == challenge JWT TTL
	MFALockoutThreshold               = 5                // failed verifies before cooldown
	MFALockoutDuration                = 15 * time.Minute // matches PIN lockout
	MFAEmailRateLimitWindow           = 15 * time.Minute // count of issued codes within this window
	MFAEmailRateLimitMaxSent          = 3                // max codes per account per window
	MFATrustedDeviceCookieDefaultDays = 90

	// mfaAuditFallbackIP is the sentinel address written to
	// audit.auth_events.ip_address when an MFA event has no real client
	// IP (lockout counter rollover, async verify cleanup, admin disable
	// from operator surface). The column is INET NOT NULL, and the rest
	// of the codebase already uses this convention — see
	// services/users/caregiver_capability.go caregiverCapabilityAuditIP.
	mfaAuditFallbackIP = "0.0.0.0"
)

// MFA admin-override value re-exports. The canonical declarations live
// in models/auth/mfa_override.go so the repository layer can reference
// them without importing the service package. Re-exporting here keeps
// existing callers (handlers, tests) source-compatible.
const (
	MFAAdminOverrideNone     = auth.MFAAdminOverrideNone
	MFAAdminOverrideForceOff = auth.MFAAdminOverrideForceOff
	MFAAdminOverrideForceOn  = auth.MFAAdminOverrideForceOn
)

// IsValidMFAAdminOverride is the allow-list guard used by handlers + service
// before writing the override row. Anything else MUST be rejected with
// ErrMFAInvalidOverride so the DB CHECK constraint isn't the last line of
// defence.
func IsValidMFAAdminOverride(v string) bool {
	switch v {
	case MFAAdminOverrideNone, MFAAdminOverrideForceOff, MFAAdminOverrideForceOn:
		return true
	}
	return false
}

// Errors surfaced by the MFA service. Generic messages — internal detail
// stays in slog at Debug level.
var (
	ErrMFAChallengeTokenInvalid = errors.New("invalid or expired challenge token")
	ErrMFACodeInvalid           = errors.New("invalid or expired code")
	ErrMFALocked                = errors.New("account locked due to too many failed attempts")
	ErrMFARateLimited           = errors.New("too many code requests, please wait")
	ErrMFANotEnrolled           = errors.New("mfa not enrolled for this account")
	ErrMFAAlreadyEnrolled       = errors.New("mfa already enrolled for this account")
	ErrMFAPermissionDenied      = errors.New("permission denied")
	ErrMFAInvalidOverride       = errors.New("invalid mfa override value")
	ErrMFAUnsupportedScope      = errors.New("operator-scope MFA is wired up in a separate phase")
	// ErrMFAStatusUnavailable surfaces when the MFA gate cannot determine
	// whether this login requires MFA — typically a Settings or
	// MFA-credentials lookup that failed with a non-"not found" error
	// (DB timeout, connection error, etc.). Reject *this* login with 503
	// so an infrastructure blip doesn't silently degrade the security
	// posture, but never block other accounts (no global fail-closed).
	ErrMFAStatusUnavailable = errors.New("mfa status unavailable, please retry")
)

// VerifiedChallenge is returned by VerifyChallenge so the login flow has
// everything it needs to mint the actual access/refresh token pair without
// re-decoding the challenge JWT.
type VerifiedChallenge struct {
	AccountID int64
	Scope     string
	TenantID  int64
}

// MFAService is the public interface consumed by the login flow + handlers.
type MFAService interface {
	// Inquiry — used by the login flow to decide which path to take.
	// tenantID must be the resolved tenant for this login attempt; passing
	// 0 falls back to the registry default. Login runs outside the tenant
	// transaction middleware, so the caller is responsible for plumbing
	// the tenant explicitly instead of relying on tenant.FromContext.
	IsRequired(ctx context.Context, account *auth.Account, tenantID int64) (bool, error)
	HasEnrollment(ctx context.Context, accountID int64) (bool, error)
	// AccountBelongsToTenant reports whether the account has a tenant mapping
	// for the given school — the operator MFA admin endpoints gate on this
	// before managing a staff account's MFA state (issue #584).
	AccountBelongsToTenant(ctx context.Context, accountID, tenantID int64) (bool, error)

	// Email-code challenge flow.
	StartChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error)
	VerifyChallenge(ctx context.Context, challengeToken, code string) (*VerifiedChallenge, error)
	// VerifyChallengeForScope is the scope-parameterized sibling of
	// VerifyChallenge (#2207). A challenge is only redeemable at the
	// portal surface it was started for: the tenant verify endpoint
	// passes the tenant scope, the school portal endpoint the school
	// scope. VerifyChallenge is a thin wrapper pinning the tenant scope.
	VerifyChallengeForScope(ctx context.Context, challengeToken, code, expectedScope string) (*VerifiedChallenge, error)
	// ResendChallenge re-issues an email code for the still-active
	// challenge and returns the *new* JWT token. The previous token
	// remains valid until its own expiry, but the frontend must
	// replace the in-flight token because the next verify must travel
	// with the renewed JWT (otherwise the user sees a fresh emailed
	// code that cannot be verified once the old JWT expires).
	ResendChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error)

	// VerifyCodeForAccount is the JWT-less sibling of VerifyChallenge used by
	// enrollment confirmation, where the user is already authenticated and a
	// challenge JWT would just be ceremony. Returns ErrMFACodeInvalid on a
	// mismatch and otherwise mirrors VerifyChallenge's audit + lockout
	// bookkeeping.
	VerifyCodeForAccount(ctx context.Context, accountID int64, code string) error

	// Enrollment / lifecycle.
	Enroll(ctx context.Context, accountID int64) error
	Disable(ctx context.Context, accountID int64) error

	// Trusted-device cookie. The handler is responsible for the actual Set-Cookie
	// header — the service only mints/verifies the value.
	// IssueTrustedDevice returns ("", zero time, nil) without persisting a
	// row when mfa_trusted_device_enabled is false for the tenant — callers
	// must check the empty cookie value and skip the Set-Cookie write in
	// that case. tenantID is used to resolve the per-tenant setting since
	// the verify handler runs outside TenantTxMiddleware.
	IssueTrustedDevice(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (cookieValue string, expiresAt time.Time, err error)
	// VerifyTrustedDevice short-circuits to (false, nil) when the tenant
	// has mfa_trusted_device_enabled set to false. This ensures that
	// flipping the setting off immediately invalidates any cookies
	// already issued, instead of waiting for natural expiry.
	VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, signedCookie string) (bool, error)
	// ListTrustedDevices returns the user's active (non-revoked,
	// non-expired) trusted-device rows for the GIVEN tenant. Trust is
	// per-(account, tenant) (#1430 review item #9), so the settings page
	// only ever displays the devices the user trusted while logged into
	// the tenant they're currently in. Cross-tenant device management is
	// not a feature today.
	ListTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]*auth.MFATrustedDevice, error)
	// RevokeTrustedDevice marks a single trusted-device row revoked. The
	// service verifies the device belongs to the calling (account,
	// tenant) so an IDOR can't revoke someone else's device or another
	// tenant's row.
	RevokeTrustedDevice(ctx context.Context, accountID, tenantID, deviceID int64) error
	// IsTrustedDeviceEnabled reports whether the tenant has the
	// security.mfa_trusted_device_enabled setting on. The login flow uses
	// this to tell the frontend whether to render the "remember this
	// device" checkbox on the MFA challenge screen.
	IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool
	// TrustedDeviceDays resolves the tenant's
	// security.mfa_trusted_device_days setting. Surfaced in the
	// mfa_required login response so the frontend can render the exact
	// day count ("Auf diesem Gerät N Tage merken") instead of hardcoding
	// it. Falls back to MFATrustedDeviceCookieDefaultDays on errors.
	TrustedDeviceDays(ctx context.Context, tenantID int64) int

	// AdminDisable wipes the target's MFA enrollment + trusted devices. In
	// addition to the users:manage permission check, the service verifies
	// the target account is a member of the actor's tenant — `auth.accounts`
	// has no tenant_id column and no RLS, so without this check a school
	// admin with users:manage could force-disable MFA on any account in any
	// other tenant by guessing its primary key (#1430 Item #2 IDOR fix).
	AdminDisable(ctx context.Context, actorID, actorTenantID, targetAccountID int64, reason string, actorPermissions []string) error
	// SetMFAOverride writes a tenant-scoped admin override
	// (force_off / force_on / none). Audit row + trusted-device revocation
	// (for force_off, scoped to (account, tenant)) are part of the same
	// call so callers don't forget the security hygiene step. Like
	// AdminDisable, the actor's tenant is required: cross-tenant calls
	// are rejected as ErrMFAPermissionDenied with a failure audit row.
	// "none" deletes any existing tenant-scoped row for the (account,
	// actorTenant) pair — it does NOT clear the platform-wide row, which
	// only the operator can write/delete.
	SetMFAOverride(ctx context.Context, actorID, actorTenantID, targetAccountID int64, override, reason string, actorPermissions []string) error
	// GetTenantMFAOverride returns the tenant-scoped override value for
	// the given (account, tenant) pair, or "none" if no row exists. Used
	// by the tenant admin "Manage MFA" modal — the operator-side global
	// row is intentionally NOT exposed on this path so a tenant admin
	// can never see whether the operator has opened the emergency
	// switch.
	GetTenantMFAOverride(ctx context.Context, accountID, tenantID int64) (string, error)
	// GetGlobalMFAOverride returns the platform-wide override row (or
	// "none" if none is set). Operator-only — exposed for the operator
	// admin surface so the Notfall-Schalter can be inspected and
	// cleared.
	GetGlobalMFAOverride(ctx context.Context, accountID int64) (string, error)
	// OperatorAdminDisable is the operator-side variant of AdminDisable.
	// Skips the users:manage check because operator routes are already
	// gated at the platform JWT layer, and writes audit metadata with
	// actor_type=operator so the audit log can distinguish operator vs.
	// tenant-admin actions on the same account. The service verifies the
	// target account belongs to the given school as defense-in-depth so
	// any future direct caller (CLI, scheduler) can't act cross-tenant.
	OperatorAdminDisable(ctx context.Context, operatorID, schoolID, targetAccountID int64, reason string) error
	// OperatorSetMFAOverride is the operator-side variant of
	// SetMFAOverride. Writes a tenant-scoped row (just like the tenant
	// admin path), not a platform-wide one — that's the next method.
	OperatorSetMFAOverride(ctx context.Context, operatorID, schoolID, targetAccountID int64, override, reason string) error
	// OperatorSetGlobalMFAOverride is the explicit "account-wide
	// emergency switch" entry point. Writes (or clears, for "none") the
	// platform-wide row in auth.mfa_overrides. Only the operator surface
	// can call this — the tenant admin surface intentionally cannot
	// reach across tenants. (#1430 review round 2.)
	OperatorSetGlobalMFAOverride(ctx context.Context, operatorID, targetAccountID int64, override, reason string) error
	// GetAdminState returns the read-side snapshot the tenant admin
	// "Manage MFA" modal needs (enrolled + current override). Bundles
	// the users:manage permission check and the school-membership check
	// in one call so the read path can't accidentally skip the gate
	// that the write paths (AdminDisable / SetMFAOverride) already
	// enforce. A cross-tenant probe returns ErrMFAPermissionDenied and
	// emits a failure audit row, mirroring the write-side defense-in-
	// depth. (#1430 review round 2 — closes the GetState IDOR.)
	GetAdminState(ctx context.Context, actorID, actorTenantID, targetAccountID int64, actorPermissions []string) (MFAAdminState, error)
}

// MFAAdminState is the read-side payload for the admin "Manage MFA"
// modal. Mirrors the handler's response DTO so the service layer owns
// the shape. Override defaults to "none" when no override is set.
type MFAAdminState struct {
	Enrolled bool
	Override string
}

// MFAServiceConfig groups dependencies for NewMFAService. Fields without zero
// values are required; zero-valued duration fields fall back to package
// constants (declared above).
type MFAServiceConfig struct {
	Repos       *repositories.Factory
	TokenAuth   *authjwt.TokenAuth
	Settings    config.SettingsService
	Dispatcher  *email.Dispatcher
	DefaultFrom email.Email
	FrontendURL string
	JWTSecret   string // used to derive the trusted-device HMAC key
	DB          *bun.DB
	Logger      *slog.Logger
}

// MFAService implementation.
type mfaService struct {
	MFAServiceConfig
	mfaSecret []byte
}

// Compile-time assertion that mfaService satisfies MFAService.
var _ MFAService = (*mfaService)(nil)

// NewMFAService wires the MFA service. Returns an error rather than panicking
// so wiring problems surface early at startup.
func NewMFAService(cfg MFAServiceConfig) (MFAService, error) {
	if cfg.Repos == nil {
		return nil, errors.New("MFAServiceConfig.Repos is required")
	}
	if cfg.TokenAuth == nil {
		return nil, errors.New("MFAServiceConfig.TokenAuth is required")
	}
	if cfg.DB == nil {
		return nil, errors.New("MFAServiceConfig.DB is required")
	}
	if cfg.JWTSecret == "" {
		return nil, errors.New("MFAServiceConfig.JWTSecret is required (for trusted-device HMAC)")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	cfg.Logger = cfg.Logger.With("service", "mfa")
	return &mfaService{
		MFAServiceConfig: cfg,
		mfaSecret:        DeriveMFASecret(cfg.JWTSecret),
	}, nil
}

// ===== Inquiry =====

// IsRequired evaluates the tenant's security.mfa_mode setting against the
// account's roles. Operator (platform-scope) sessions are handled by the
// platform service in a later phase — this implementation rejects them.
//
// Override resolution (#1430 review round 2 — closes cross-tenant
// override bleed):
//
//  1. Platform-wide row in auth.mfa_overrides (tenant_id IS NULL).
//     Set by the operator as the "account lost mailbox access" escape
//     hatch; applies to every login regardless of tenant.
//  2. Tenant-scoped row in auth.mfa_overrides (tenant_id = login
//     tenant). Set by a tenant admin; only affects logins to that
//     tenant. Force_off in tenant A does NOT bypass MFA in tenant B.
//  3. No override row → tenant policy (security.mfa_mode) decides.
//
// Override lookup is allowed to fail closed (refuse this login on
// infra error) for the same reason the enrollment-lookup is: if we
// can't tell whether the override is force_off, we must not silently
// honor it.
func (s *mfaService) IsRequired(ctx context.Context, account *auth.Account, tenantID int64) (bool, error) {
	if account == nil {
		return false, errors.New("account is required")
	}
	// Both override lookups must run as phoenix_admin (BYPASSRLS).
	// The login flow has no tenant transaction yet, so app.current_tenant_id
	// is unset and the RLS policy on auth.mfa_overrides hides every
	// row whose tenant_id is NOT NULL — including the tenant-scoped
	// row we need for resolution. The lookup is safe to read across
	// tenants here because the account is authenticated and the
	// tenantID arg is the resolved login tenant from a trusted source
	// (slug → school FK already validated upstream).
	var (
		global         *auth.MFAOverride
		tenantOverride *auth.MFAOverride
	)
	txErr := tenant.WithAdminTx(ctx, s.DB, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		global, err = s.Repos.MFAOverride.FindGlobal(txCtx, account.ID)
		if err != nil {
			return err
		}
		if tenantID > 0 {
			tenantOverride, err = s.Repos.MFAOverride.FindByAccountAndTenant(txCtx, account.ID, tenantID)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		s.Logger.Warn("mfa override lookup failed; refusing login",
			slog.Int64("account_id", account.ID),
			slog.Int64("tenant_id", tenantID),
			slog.String("error", txErr.Error()))
		return false, ErrMFAStatusUnavailable
	}
	// Step 1: platform-wide override (operator emergency switch) wins.
	if global != nil {
		switch global.Override {
		case MFAAdminOverrideForceOff:
			return false, nil
		case MFAAdminOverrideForceOn:
			return true, nil
		}
	}
	// Step 2: tenant-scoped override (tenant admin per-school).
	if tenantOverride != nil {
		switch tenantOverride.Override {
		case MFAAdminOverrideForceOff:
			return false, nil
		case MFAAdminOverrideForceOn:
			return true, nil
		}
	}
	mode := configModel.MFAModeOff
	if s.Settings != nil {
		// Login runs outside the tenant-transaction middleware, so
		// tenant.FromContext(ctx) is 0. Use ResolveStringForTenant when
		// the caller passed the resolved tenant explicitly; fall back to
		// ResolveString (registry default) otherwise.
		var (
			val string
			err error
		)
		if tenantID > 0 {
			val, err = s.Settings.ResolveStringForTenant(ctx, tenantID, configModel.KeyMFAMode)
		} else {
			val, err = s.Settings.ResolveString(ctx, configModel.KeyMFAMode)
		}
		if err != nil {
			// Settings infra error (DB timeout, connection drop). We can't
			// tell whether the tenant has opted in or out, so failing-open
			// to "off" would silently downgrade security; fail-closed to
			// "required" would lock everyone out on a registry hiccup.
			// Refuse THIS login instead — caller maps to 503.
			s.Logger.Warn("mfa_mode resolve failed; refusing login",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()))
			return false, ErrMFAStatusUnavailable
		} else if val != "" {
			mode = val
		}
	}
	switch mode {
	case configModel.MFAModeOff:
		return false, nil
	case configModel.MFAModeRequiredAll:
		return true, nil
	case configModel.MFAModeRequiredAdmins:
		return authorize.AccountHasRole(account, "admin"), nil
	default:
		s.Logger.Warn("unknown mfa_mode value; treating as off", slog.String("value", mode))
		return false, nil
	}
}

func (s *mfaService) HasEnrollment(ctx context.Context, accountID int64) (bool, error) {
	cred, err := s.Repos.MFACredential.FindByAccountID(ctx, accountID)
	if err != nil {
		// sql.ErrNoRows is the legitimate "not enrolled" signal — every
		// fresh account hits it. Anything else (DB timeout, connection
		// drop) means we can't actually answer the question; refuse this
		// login instead of fail-open returning false. errors.Is walks
		// through DatabaseError.Unwrap() so the wrapped sentinel matches.
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		s.Logger.Warn("mfa enrollment lookup failed; refusing login",
			slog.Int64("account_id", accountID),
			slog.String("error", err.Error()))
		return false, ErrMFAStatusUnavailable
	}
	return cred != nil && cred.ID > 0, nil
}

// ===== Challenge / verify =====

func (s *mfaService) StartChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error) {
	if scope != authjwt.MFAChallengeScopeTenant && scope != authjwt.MFAChallengeScopeSchool {
		return "", ErrMFAUnsupportedScope
	}

	account, err := s.Repos.Account.FindByID(ctx, accountID)
	if err != nil {
		return "", fmt.Errorf("look up account: %w", err)
	}
	if s.isMFALocked(account, time.Now()) {
		return "", ErrMFALocked
	}

	// Rate-limit code issuance. Cooldown is per-tenant configurable but the
	// hard cap (3 codes / 15 min) stays in code — it's an abuse defense, not
	// a UX knob.
	since := time.Now().Add(-MFAEmailRateLimitWindow)
	count, err := s.Repos.MFAEmailChallenge.CountRecentByAccountID(ctx, accountID, since)
	if err == nil && count >= MFAEmailRateLimitMaxSent {
		return "", ErrMFARateLimited
	}

	plainCode, err := GenerateEmailCode()
	if err != nil {
		return "", fmt.Errorf("generate email code: %w", err)
	}
	codeHash, err := HashShortCode(plainCode)
	if err != nil {
		return "", fmt.Errorf("hash email code: %w", err)
	}

	challenge := &auth.MFAEmailChallenge{
		AccountID: accountID,
		CodeHash:  codeHash,
		ExpiresAt: time.Now().Add(MFAChallengeTTL),
		IPAddress: ip,
	}
	if err := s.Repos.MFAEmailChallenge.Create(ctx, challenge); err != nil {
		return "", fmt.Errorf("persist email challenge: %w", err)
	}

	s.dispatchChallengeEmail(ctx, account, tenantID, plainCode, ip)
	s.recordAuthEvent(ctx, accountID, tenantID, audit.EventTypeMFAEmailSent, true, ip, "", map[string]any{
		"challenge_id": challenge.ID,
	})

	tokenString, err := s.TokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
		AccountID: accountID,
		Scope:     scope,
		TenantID:  tenantID,
	}, MFAChallengeTTL)
	if err != nil {
		return "", fmt.Errorf("mint challenge jwt: %w", err)
	}
	return tokenString, nil
}

func (s *mfaService) VerifyChallenge(ctx context.Context, challengeToken, code string) (*VerifiedChallenge, error) {
	return s.VerifyChallengeForScope(ctx, challengeToken, code, authjwt.MFAChallengeScopeTenant)
}

// VerifyChallengeForScope verifies an email-code challenge and enforces that
// the challenge was started for the expected portal scope. Scope mismatch is
// refused before any code comparison so a school challenge can never be
// burned (or redeemed) at the tenant endpoint and vice versa.
func (s *mfaService) VerifyChallengeForScope(ctx context.Context, challengeToken, code, expectedScope string) (*VerifiedChallenge, error) {
	claims, err := s.parseChallengeToken(challengeToken)
	if err != nil {
		return nil, ErrMFAChallengeTokenInvalid
	}
	if claims.Scope != expectedScope {
		return nil, ErrMFAUnsupportedScope
	}

	account, err := s.Repos.Account.FindByID(ctx, claims.AccountID)
	if err != nil {
		return nil, ErrMFAChallengeTokenInvalid
	}
	if s.isMFALocked(account, time.Now()) {
		return nil, ErrMFALocked
	}

	active, err := s.Repos.MFAEmailChallenge.FindActiveByAccountID(ctx, claims.AccountID)
	if err != nil || active == nil {
		s.recordAuthEvent(ctx, claims.AccountID, claims.TenantID, audit.EventTypeMFAFailed, false, nil, "no active challenge", nil)
		return nil, ErrMFACodeInvalid
	}

	ok, verifyErr := VerifyShortCode(code, active.CodeHash)
	if verifyErr != nil || !ok {
		s.handleFailedAttempt(ctx, account, claims.TenantID)
		s.recordAuthEvent(ctx, claims.AccountID, claims.TenantID, audit.EventTypeMFAFailed, false, nil, "code mismatch", nil)
		return nil, ErrMFACodeInvalid
	}

	// Single-use enforcement. MarkConsumed uses
	//   UPDATE ... SET consumed_at=? WHERE id=? AND consumed_at IS NULL
	// plus AssertRowsAffected(1), so two concurrent verifies on the same
	// code race on the UPDATE: the winner flips consumed_at; the loser
	// observes 0 rows affected and returns a DatabaseError. We MUST refuse
	// the loser — the previous code only logged the error and continued to
	// mint a VerifiedChallenge, which let both racers complete the login
	// with the same single-use code. Treat any consume failure the same
	// way (DB outage included — we can't prove single-use, so we refuse).
	now := time.Now()
	if err := s.Repos.MFAEmailChallenge.MarkConsumed(ctx, active.ID, now); err != nil {
		s.Logger.Warn("failed to mark challenge consumed; refusing verify",
			slog.Int64("account_id", claims.AccountID),
			slog.Int64("challenge_id", active.ID),
			slog.String("error", err.Error()))
		s.recordAuthEvent(ctx, claims.AccountID, claims.TenantID, audit.EventTypeMFAFailed, false, nil, "consume race", nil)
		return nil, ErrMFACodeInvalid
	}
	// Atomic reset (UPDATE SET mfa_attempts=0, mfa_locked_until=NULL).
	// Prevents a successful verify from clobbering a concurrent failed
	// verify's increment via a full-row Update of stale in-memory state.
	if err := s.Repos.Account.ResetMFAAttempts(ctx, account.ID); err != nil {
		s.Logger.Warn("failed to reset MFA attempts", slog.String("error", err.Error()))
	}
	account.MFAAttempts = 0
	account.MFALockedUntil = nil

	cred, _ := s.Repos.MFACredential.FindByAccountID(ctx, claims.AccountID)
	if cred != nil && cred.ID > 0 {
		_ = s.Repos.MFACredential.UpdateLastUsedAt(ctx, cred.ID, now)
	}

	s.recordAuthEvent(ctx, claims.AccountID, claims.TenantID, audit.EventTypeMFAVerified, true, nil, "", nil)
	return &VerifiedChallenge{
		AccountID: claims.AccountID,
		Scope:     claims.Scope,
		TenantID:  claims.TenantID,
	}, nil
}

// VerifyCodeForAccount runs the same verify pipeline as VerifyChallenge but
// without the JWT round-trip — the caller has already authenticated the user
// out-of-band (typically a regular access token from /auth/mfa/enroll/confirm).
func (s *mfaService) VerifyCodeForAccount(ctx context.Context, accountID int64, code string) error {
	account, err := s.Repos.Account.FindByID(ctx, accountID)
	if err != nil {
		return ErrMFACodeInvalid
	}
	if s.isMFALocked(account, time.Now()) {
		return ErrMFALocked
	}
	// VerifyCodeForAccount runs INSIDE TenantTxMiddleware (called from
	// authenticated /auth/mfa/enroll/confirm), so tenant.FromContext(ctx)
	// is set. Pass 0 below; recordAuthEvent + handleFailedAttempt fall
	// back to the context tenant.
	active, err := s.Repos.MFAEmailChallenge.FindActiveByAccountID(ctx, accountID)
	if err != nil || active == nil {
		s.recordAuthEvent(ctx, accountID, 0, audit.EventTypeMFAFailed, false, nil, "no active challenge", nil)
		return ErrMFACodeInvalid
	}
	ok, vErr := VerifyShortCode(code, active.CodeHash)
	if vErr != nil || !ok {
		s.handleFailedAttempt(ctx, account, 0)
		s.recordAuthEvent(ctx, accountID, 0, audit.EventTypeMFAFailed, false, nil, "code mismatch", nil)
		return ErrMFACodeInvalid
	}
	now := time.Now()
	if err := s.Repos.MFAEmailChallenge.MarkConsumed(ctx, active.ID, now); err != nil {
		// Same single-use guard as VerifyChallenge — if the atomic UPDATE
		// reports 0 rows affected, another concurrent caller already
		// consumed this code. Refuse this caller so the code remains
		// single-use across racing requests.
		s.Logger.Warn("failed to mark challenge consumed; refusing verify",
			slog.Int64("account_id", accountID),
			slog.Int64("challenge_id", active.ID),
			slog.String("error", err.Error()))
		s.recordAuthEvent(ctx, accountID, 0, audit.EventTypeMFAFailed, false, nil, "consume race", nil)
		return ErrMFACodeInvalid
	}
	// Atomic reset matches VerifyChallenge — single UPDATE so a concurrent
	// failed verify's increment is never silently overwritten.
	_ = s.Repos.Account.ResetMFAAttempts(ctx, accountID)
	account.MFAAttempts = 0
	account.MFALockedUntil = nil
	s.recordAuthEvent(ctx, accountID, 0, audit.EventTypeMFAVerified, true, nil, "", nil)
	return nil
}

func (s *mfaService) ResendChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	claims, err := s.parseChallengeToken(challengeToken)
	if err != nil {
		return "", ErrMFAChallengeTokenInvalid
	}

	// No per-resend cooldown gate — the sliding-window cap inside
	// StartChallenge (3 codes / 15 min) remains as the abuse defense.
	// Return the renewed JWT so the caller's frontend can replace its
	// in-flight challenge_token; otherwise the new email code becomes
	// unverifiable as soon as the original token expires (the resend
	// loses-its-token dead end #1430 review round 2 flagged).
	renewed, err := s.StartChallenge(ctx, claims.AccountID, claims.TenantID, claims.Scope, ip)
	if err != nil {
		return "", err
	}
	return renewed, nil
}

// isMFALocked reports whether the account is inside its MFA-failure cooldown
// window relative to now. The decision lives in the service (clock injected),
// not on the model (issue #586, Rule 12); the account row only holds the
// mfa_locked_until fact.
func (s *mfaService) isMFALocked(account *auth.Account, now time.Time) bool {
	return account != nil && account.MFALockedUntil != nil && now.Before(*account.MFALockedUntil)
}

// handleFailedAttempt atomically bumps the lockout counter via the
// repository and emits an mfa_locked event when *this* increment was the
// one that crossed the threshold. Errors here are logged but never bubble
// up — VerifyChallenge already returns a generic "invalid code" message
// and must keep timing consistent.
//
// The previous read-modify-write pattern was racy: two concurrent failed
// verifies both read mfa_attempts=N, both wrote N+1, and only one of N
// failures actually counted (an attacker could double their attempt
// budget). The atomic UPDATE here closes that. (#1430 review item #6)
//
// tenantID is forwarded to recordAuthEvent so the mfa_locked row lands in
// the audit table during login (where tenant.FromContext(ctx) is 0). Pass
// 0 from authenticated callers (post-login enrollment confirm) — the
// recorder falls back to the context tenant.
func (s *mfaService) handleFailedAttempt(ctx context.Context, account *auth.Account, tenantID int64) {
	threshold := s.resolveLockoutThreshold(ctx, tenantID)
	duration := s.resolveLockoutDuration(ctx, tenantID)
	result, err := s.Repos.Account.IncrementMFAAttempts(ctx, account.ID, threshold, duration)
	if err != nil {
		s.Logger.Warn("failed to persist MFA attempt counter", slog.String("error", err.Error()))
		return
	}
	// Keep the in-memory account in sync so callers higher up that still
	// inspect account.MFAAttempts / account.MFALockedUntil see the new
	// values without re-reading.
	account.MFAAttempts = result.Attempts
	account.MFALockedUntil = result.LockedUntil

	// Emit mfa_locked exactly when this attempt crossed the threshold.
	// `result.Attempts == threshold` means: my increment was the N-th and
	// only I see N == threshold. Later attempts past the threshold only push
	// locked_until forward and don't re-emit — matches the original
	// "!wasLocked && IsMFALocked()" semantic without the TOCTOU window.
	if result.Attempts == threshold {
		s.recordAuthEvent(ctx, account.ID, tenantID, audit.EventTypeMFALocked, false, nil, "", map[string]any{
			"locked_until": result.LockedUntil,
		})
	}
}

// resolveLockoutThreshold resolves the tenant's
// security.account_lockout_threshold, falling back to MFALockoutThreshold on
// any error or missing override (issue #586). Follows the HasTenantOverride →
// Resolve → fallback contract from .claude/rules/settings-system.md.
func (s *mfaService) resolveLockoutThreshold(ctx context.Context, tenantID int64) int {
	if s.Settings == nil {
		return MFALockoutThreshold
	}
	var (
		val int
		err error
	)
	if tenantID > 0 {
		val, err = s.Settings.ResolveIntForTenant(ctx, tenantID, configModel.KeyAccountLockoutThreshold)
	} else {
		val, err = s.Settings.ResolveInt(ctx, configModel.KeyAccountLockoutThreshold)
	}
	if err != nil || val <= 0 {
		return MFALockoutThreshold
	}
	return val
}

// resolveLockoutDuration resolves the tenant's
// security.account_lockout_duration_minutes, falling back to MFALockoutDuration
// on any error or missing override (issue #586).
func (s *mfaService) resolveLockoutDuration(ctx context.Context, tenantID int64) time.Duration {
	if s.Settings == nil {
		return MFALockoutDuration
	}
	var (
		minutes int
		err     error
	)
	if tenantID > 0 {
		minutes, err = s.Settings.ResolveIntForTenant(ctx, tenantID, configModel.KeyAccountLockoutDurationMinutes)
	} else {
		minutes, err = s.Settings.ResolveInt(ctx, configModel.KeyAccountLockoutDurationMinutes)
	}
	if err != nil || minutes <= 0 {
		return MFALockoutDuration
	}
	return time.Duration(minutes) * time.Minute
}

// ===== Enrollment / lifecycle =====

func (s *mfaService) Enroll(ctx context.Context, accountID int64) error {
	existing, _ := s.Repos.MFACredential.FindByAccountID(ctx, accountID)
	if existing != nil && existing.ID > 0 {
		return ErrMFAAlreadyEnrolled
	}
	cred := &auth.MFACredential{
		AccountID:  accountID,
		Method:     auth.MFAMethodEmail,
		EnrolledAt: time.Now(),
	}
	if err := s.Repos.MFACredential.Create(ctx, cred); err != nil {
		return fmt.Errorf("persist mfa credential: %w", err)
	}
	return nil
}

// Disable cascades: credential -> trusted devices revoked. Account-level
// lockout fields are reset so a future re-enrollment starts clean. The
// three writes happen in a single tx so partial-failure can't leave the
// account in a half-disabled state (e.g. credential gone but devices
// still trusted).
func (s *mfaService) Disable(ctx context.Context, accountID int64) error {
	err := tenant.WithAdminTx(ctx, s.DB, func(txCtx context.Context, _ bun.Tx) error {
		if err := s.Repos.MFACredential.DeleteByAccountID(txCtx, accountID); err != nil {
			return fmt.Errorf("delete credential: %w", err)
		}
		if err := s.Repos.MFATrustedDevice.RevokeAllByAccountID(txCtx, accountID, time.Now()); err != nil {
			return fmt.Errorf("revoke trusted devices: %w", err)
		}
		// Atomic reset — replaces the previous fetch + Update with a single
		// UPDATE so the disable cascade isn't racing concurrent failed
		// verifies on the same account.
		if err := s.Repos.Account.ResetMFAAttempts(txCtx, accountID); err != nil {
			return fmt.Errorf("reset mfa attempts: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Disable runs from authenticated endpoints (self-service or admin) with
	// an active TenantTxMiddleware, so context already carries the tenant.
	s.recordAuthEvent(ctx, accountID, 0, audit.EventTypeMFADisabled, true, nil, "", nil)
	return nil
}

// ===== Trusted device =====

// isTrustedDeviceEnabled reads security.mfa_trusted_device_enabled for the
// given tenant. Login + verify both run outside the TenantTxMiddleware, so
// we resolve via the explicit-tenant variant; otherwise the helper would
// fall back to the registry default and ignore an admin's opt-out.
//
// Conservative defaults: when the settings service or the tenant ID are
// missing we honor the registry default (true). When the lookup itself
// fails we log and return false — better to surprise the user with a
// missing checkbox than to ignore the admin's opt-out.
// IsTrustedDeviceEnabled is the public-interface entry point. Internally it
// delegates to the same helper used by Issue/VerifyTrustedDevice so callers
// see exactly the same answer the gating logic uses.
func (s *mfaService) IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool {
	return s.isTrustedDeviceEnabled(ctx, tenantID)
}

func (s *mfaService) isTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool {
	if s.Settings == nil {
		return true
	}
	if tenantID <= 0 {
		enabled, err := s.Settings.ResolveBool(ctx, configModel.KeyMFATrustedDeviceEnabled)
		if err != nil {
			s.Logger.Warn("trusted_device_enabled resolve failed; disabling feature",
				slog.String("error", err.Error()))
			return false
		}
		return enabled
	}
	enabled, err := s.Settings.ResolveBoolForTenant(ctx, tenantID, configModel.KeyMFATrustedDeviceEnabled)
	if err != nil {
		s.Logger.Warn("trusted_device_enabled resolve failed; disabling feature",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()))
		return false
	}
	return enabled
}

func (s *mfaService) IssueTrustedDevice(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (string, time.Time, error) {
	if !s.isTrustedDeviceEnabled(ctx, tenantID) {
		// Setting is off for this tenant — caller treats empty value as
		// "skip cookie".
		return "", time.Time{}, nil
	}
	days := s.resolveTrustedDeviceDays(ctx, tenantID)
	expiresAt := time.Now().Add(time.Duration(days) * 24 * time.Hour)

	rawToken, err := GenerateTrustedDeviceToken()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate trusted-device token: %w", err)
	}
	device := &auth.MFATrustedDevice{
		AccountID: accountID,
		// Bind the row to the resolved tenant so a cookie issued for
		// tenant A cannot ever bypass MFA on tenant B (#1430 review #9).
		TenantID:  tenantID,
		TokenHash: HashTrustedDeviceToken(rawToken),
		IPAddress: ip,
		ExpiresAt: expiresAt,
	}
	if userAgent != "" {
		ua := userAgent
		device.UserAgent = &ua
	}
	if err := s.Repos.MFATrustedDevice.Create(ctx, device); err != nil {
		return "", time.Time{}, fmt.Errorf("persist trusted device: %w", err)
	}
	signed := SignTrustedDeviceToken(rawToken, s.mfaSecret)
	s.recordAuthEvent(ctx, accountID, tenantID, audit.EventTypeMFATrustedDeviceAdded, true, ip, "", map[string]any{
		"device_id": device.ID,
	})
	// Fire-and-forget security notification — the user gets a mail so
	// "remember this device" never happens silently. If the mail dispatch
	// fails the trusted device is still issued (the security signal is
	// nice-to-have, the login flow must not break).
	s.dispatchTrustedDeviceAddedEmail(ctx, accountID, userAgent, ip, days)
	return signed, expiresAt, nil
}

func (s *mfaService) VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, signedCookie string) (bool, error) {
	if !s.isTrustedDeviceEnabled(ctx, tenantID) {
		// Tenant has flipped the feature off after a cookie was issued —
		// reject it immediately instead of waiting for natural expiry.
		return false, nil
	}
	rawToken, ok := VerifyTrustedDeviceToken(signedCookie, s.mfaSecret)
	if !ok {
		return false, nil
	}
	tokenHash := HashTrustedDeviceToken(rawToken)
	// Tenant-scoped lookup: the repo refuses to match against a row that
	// belongs to a different tenant, even if the (account_id, token_hash)
	// pair happens to collide. (#1430 review #9)
	device, err := s.Repos.MFATrustedDevice.FindActiveByAccountTenantAndTokenHash(ctx, accountID, tenantID, tokenHash)
	if err != nil || device == nil {
		return false, nil
	}
	_ = s.Repos.MFATrustedDevice.UpdateLastUsedAt(ctx, device.ID, time.Now())
	return true, nil
}

func (s *mfaService) ListTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]*auth.MFATrustedDevice, error) {
	return s.Repos.MFATrustedDevice.ListActiveByAccountTenant(ctx, accountID, tenantID)
}

func (s *mfaService) RevokeTrustedDevice(ctx context.Context, accountID, tenantID, deviceID int64) error {
	// Validate ownership before revoke — a device row can only be revoked by
	// its own account AND only from the tenant it was trusted in. An attacker
	// with a stolen access token for account A in tenant T1 can't revoke
	// account B's devices via id-guessing, and can't revoke A's tenant-T2
	// devices either.
	devices, err := s.Repos.MFATrustedDevice.ListActiveByAccountTenant(ctx, accountID, tenantID)
	if err != nil {
		return err
	}
	owned := false
	for _, d := range devices {
		if d.ID == deviceID {
			owned = true
			break
		}
	}
	if !owned {
		return ErrMFAPermissionDenied
	}
	return s.Repos.MFATrustedDevice.Revoke(ctx, deviceID, time.Now())
}

// ===== Admin override ("Godmode") =====

func (s *mfaService) AdminDisable(ctx context.Context, actorID, actorTenantID, targetAccountID int64, reason string, actorPermissions []string) error {
	if err := s.requireAdminPermission(actorPermissions); err != nil {
		s.recordAdminOverrideFailure(ctx, adminOverrideFailureEvent{
			ActorType:       "account",
			ActorID:         actorID,
			TargetAccountID: targetAccountID,
			Action:          "disable",
			Reason:          reason,
			ErrMsg:          "permission denied",
		})
		return err
	}
	// Defense-in-depth: refuse to act across tenants even if a misconfigured
	// role accidentally granted users:manage. Mirrors the operator-side
	// requireSchoolMembership check. (#1430 Item #2)
	if err := s.requireSchoolMembership(ctx, "account", actorID, actorTenantID, targetAccountID, "disable"); err != nil {
		return err
	}
	// targetTenantID == actorTenantID: membership check just proved the
	// target is in the actor's tenant. Pass it explicitly so audit rows
	// land even when the caller isn't inside TenantTxMiddleware.
	return s.adminDisableCore(ctx, "account", actorID, actorTenantID, targetAccountID, reason)
}

// OperatorAdminDisable mirrors AdminDisable but skips the users:manage
// check (operator routes carry their own platform-level gate) and tags
// the audit metadata with actor_type=operator. Verifies the target
// account belongs to the given school so direct service callers (CLI,
// scheduler) cannot reach across tenants.
func (s *mfaService) OperatorAdminDisable(ctx context.Context, operatorID, schoolID, targetAccountID int64, reason string) error {
	if err := s.requireSchoolMembership(ctx, "operator", operatorID, schoolID, targetAccountID, "disable"); err != nil {
		return err
	}
	// schoolID is the target tenant. Operator routes do not run inside
	// TenantTxMiddleware, so the audit row would otherwise lose its
	// tenant_id and be skipped entirely. (#1430 Item #5)
	return s.adminDisableCore(ctx, "operator", operatorID, schoolID, targetAccountID, reason)
}

// GetTenantMFAOverride returns the tenant-scoped override row's value
// for (account, tenant) or "none" if no row exists. Used by the tenant
// admin "Manage MFA" modal — the operator-global row is intentionally
// hidden on this path so tenant admins can't enumerate operator state.
func (s *mfaService) GetTenantMFAOverride(ctx context.Context, accountID, tenantID int64) (string, error) {
	if tenantID == 0 {
		return MFAAdminOverrideNone, nil
	}
	row, err := s.Repos.MFAOverride.FindByAccountAndTenant(ctx, accountID, tenantID)
	if err != nil {
		return "", fmt.Errorf("load tenant mfa override: %w", err)
	}
	if row == nil {
		return MFAAdminOverrideNone, nil
	}
	return row.Override, nil
}

// GetGlobalMFAOverride returns the platform-wide override or "none" if
// the operator has not set one. Used by the operator surface only.
func (s *mfaService) GetGlobalMFAOverride(ctx context.Context, accountID int64) (string, error) {
	row, err := s.Repos.MFAOverride.FindGlobal(ctx, accountID)
	if err != nil {
		return "", fmt.Errorf("load global mfa override: %w", err)
	}
	if row == nil {
		return MFAAdminOverrideNone, nil
	}
	return row.Override, nil
}

// GetAdminState bundles the permission + school-membership gates that
// the write paths (AdminDisable / SetMFAOverride) already enforce so
// the read path can't be the soft underbelly. A cross-tenant probe is
// indistinguishable from "no such account" — both return
// ErrMFAPermissionDenied — and lands a failure audit row so scanning
// attempts are forensically visible.
//
// (#1430 review round 2 — without this gate, a tenant admin with
// users:manage could enumerate which account_ids in *other* tenants
// are MFA-enrolled and which carry which override, purely by probing
// integer IDs.)
func (s *mfaService) GetAdminState(ctx context.Context, actorID, actorTenantID, targetAccountID int64, actorPermissions []string) (MFAAdminState, error) {
	if err := s.requireAdminPermission(actorPermissions); err != nil {
		s.recordAdminOverrideFailure(ctx, adminOverrideFailureEvent{
			ActorType:       "account",
			ActorID:         actorID,
			TargetTenantID:  actorTenantID,
			TargetAccountID: targetAccountID,
			Action:          "get_state",
			ErrMsg:          "permission denied",
		})
		return MFAAdminState{}, err
	}
	if err := s.requireSchoolMembership(ctx, "account", actorID, actorTenantID, targetAccountID, "get_state"); err != nil {
		return MFAAdminState{}, err
	}
	enrolled, err := s.HasEnrollment(ctx, targetAccountID)
	if err != nil {
		return MFAAdminState{}, err
	}
	// Tenant admin reads only see their own tenant's override row.
	// The operator's account-wide row is intentionally hidden from
	// this path (tenant admins must not learn whether the operator
	// has opened the emergency switch on shared accounts).
	override, err := s.GetTenantMFAOverride(ctx, targetAccountID, actorTenantID)
	if err != nil {
		return MFAAdminState{}, err
	}
	return MFAAdminState{Enrolled: enrolled, Override: override}, nil
}

// adminDisableCore is the shared cascade used by both tenant-admin and
// operator paths. actorType becomes audit metadata so the audit log can
// tell which surface initiated the disable. Both rejected attempts and
// successful disables produce an audit row so brute-force / scanning
// behaviour is forensically visible.
func (s *mfaService) adminDisableCore(ctx context.Context, actorType string, actorID, targetTenantID, targetAccountID int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		s.recordAdminOverrideFailure(ctx, adminOverrideFailureEvent{
			ActorType:       actorType,
			ActorID:         actorID,
			TargetTenantID:  targetTenantID,
			TargetAccountID: targetAccountID,
			Action:          "disable",
			ErrMsg:          "reason is required",
		})
		return errors.New("reason is required for admin override")
	}
	if err := s.Disable(ctx, targetAccountID); err != nil {
		s.recordAdminOverrideFailure(ctx, adminOverrideFailureEvent{
			ActorType:       actorType,
			ActorID:         actorID,
			TargetTenantID:  targetTenantID,
			TargetAccountID: targetAccountID,
			Action:          "disable",
			Reason:          reason,
			ErrMsg:          err.Error(),
		})
		return err
	}
	s.recordAuthEvent(ctx, targetAccountID, targetTenantID, audit.EventTypeMFAAdminOverride, true, nil, "", map[string]any{
		"actor_type":       actorType,
		"actor_account_id": actorID,
		"action":           "disable",
		"reason":           reason,
	})
	return nil
}

func (s *mfaService) SetMFAOverride(ctx context.Context, actorID, actorTenantID, targetAccountID int64, override, reason string, actorPermissions []string) error {
	if err := s.requireAdminPermission(actorPermissions); err != nil {
		// Audit denied attempts too so abuse / misconfigured roles
		// surface in the same stream as successful overrides.
		s.recordAdminOverrideFailure(ctx, adminOverrideFailureEvent{
			ActorType:       "account",
			ActorID:         actorID,
			TargetAccountID: targetAccountID,
			Action:          "set_override",
			Override:        override,
			Reason:          reason,
			ErrMsg:          "permission denied",
		})
		return err
	}
	// Defense-in-depth: refuse cross-tenant writes even when the actor has
	// users:manage. Without this an admin in tenant A with a guessed
	// account ID could flip force_off on a user in tenant B. (#1430 Item #2)
	if err := s.requireSchoolMembership(ctx, "account", actorID, actorTenantID, targetAccountID, "set_override"); err != nil {
		return err
	}
	return s.setMFAOverrideCore(ctx, "account", actorID, actorTenantID, targetAccountID, override, reason)
}

// OperatorSetMFAOverride mirrors SetMFAOverride for the operator path.
// Skips the users:manage check (route layer guards this) and writes
// audit metadata tagged actor_type=operator. Verifies the target account
// belongs to the given school as defense-in-depth.
func (s *mfaService) OperatorSetMFAOverride(ctx context.Context, operatorID, schoolID, targetAccountID int64, override, reason string) error {
	if err := s.requireSchoolMembership(ctx, "operator", operatorID, schoolID, targetAccountID, "set_override"); err != nil {
		return err
	}
	return s.setMFAOverrideCore(ctx, "operator", operatorID, schoolID, targetAccountID, override, reason)
}

// setMFAOverrideCore is the shared write + trusted-device-revoke +
// audit-record pipeline used by both tenant-admin and operator flows.
// The row mutation and the (account, tenant)-scoped trusted-device
// revoke run in a single tx so a failed revoke rolls back the override
// flip — otherwise a force_off could leave the account flagged "no
// MFA" for this tenant while stale trusted-device cookies remain
// valid.
//
// "none" deletes any existing tenant-scoped row for the (account,
// actorTenant) pair; it does NOT touch the platform-wide row (which
// only OperatorSetGlobalMFAOverride can write/clear). The audit row
// records the previous value so the audit trail shows the transition.
func (s *mfaService) setMFAOverrideCore(ctx context.Context, actorType string, actorID, targetTenantID, targetAccountID int64, override, reason string) error {
	if !IsValidMFAAdminOverride(override) {
		s.recordAdminOverrideFailure(ctx, adminOverrideFailureEvent{
			ActorType:       actorType,
			ActorID:         actorID,
			TargetTenantID:  targetTenantID,
			TargetAccountID: targetAccountID,
			Action:          "set_override",
			Override:        override,
			Reason:          reason,
			ErrMsg:          ErrMFAInvalidOverride.Error(),
		})
		return ErrMFAInvalidOverride
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		s.recordAdminOverrideFailure(ctx, adminOverrideFailureEvent{
			ActorType:       actorType,
			ActorID:         actorID,
			TargetTenantID:  targetTenantID,
			TargetAccountID: targetAccountID,
			Action:          "set_override",
			Override:        override,
			ErrMsg:          "reason is required",
		})
		return errors.New("reason is required for admin override")
	}
	if targetTenantID == 0 {
		// setMFAOverrideCore writes tenant-scoped rows — a zero tenant
		// would silently target the platform-wide slot, which only the
		// operator-global path is allowed to do.
		return errors.New("target tenant id is required for set_override")
	}

	previous := MFAAdminOverrideNone
	txErr := tenant.WithAdminTx(ctx, s.DB, func(txCtx context.Context, _ bun.Tx) error {
		existing, err := s.Repos.MFAOverride.FindByAccountAndTenant(txCtx, targetAccountID, targetTenantID)
		if err != nil {
			return fmt.Errorf("load existing tenant override: %w", err)
		}
		if existing != nil {
			previous = existing.Override
		}
		if override == MFAAdminOverrideNone {
			if err := s.Repos.MFAOverride.DeleteTenant(txCtx, targetAccountID, targetTenantID); err != nil {
				return fmt.Errorf("clear tenant override: %w", err)
			}
			return nil
		}
		tenantID := targetTenantID
		row := &auth.MFAOverride{
			AccountID: targetAccountID,
			TenantID:  &tenantID,
			Override:  override,
			SetBy:     actorID,
			SetByType: actorType,
			Reason:    reason,
		}
		if err := s.Repos.MFAOverride.UpsertTenant(txCtx, row); err != nil {
			return fmt.Errorf("persist tenant mfa override: %w", err)
		}
		// Force-off must revoke trusted devices for THIS tenant only —
		// the same account may legitimately keep trust in other tenants
		// where the admin hasn't (yet) flipped the switch.
		if override == MFAAdminOverrideForceOff {
			if err := s.Repos.MFATrustedDevice.RevokeAllByAccountTenant(txCtx, targetAccountID, targetTenantID, time.Now()); err != nil {
				return fmt.Errorf("revoke tenant-scoped trusted devices: %w", err)
			}
		}
		return nil
	})
	if txErr != nil {
		s.recordAdminOverrideFailure(ctx, adminOverrideFailureEvent{
			ActorType:       actorType,
			ActorID:         actorID,
			TargetTenantID:  targetTenantID,
			TargetAccountID: targetAccountID,
			Action:          "set_override",
			Override:        override,
			Reason:          reason,
			ErrMsg:          txErr.Error(),
		})
		return txErr
	}

	s.recordAuthEvent(ctx, targetAccountID, targetTenantID, audit.EventTypeMFAAdminOverride, true, nil, "", map[string]any{
		"actor_type":        actorType,
		"actor_account_id":  actorID,
		"action":            "set_override",
		"scope":             "tenant",
		"override":          override,
		"previous_override": previous,
		"reason":            reason,
	})
	return nil
}

// OperatorSetGlobalMFAOverride writes (or clears) the platform-wide
// override row. This is the explicit "account-wide emergency switch"
// surface — set force_off when the user has lost mailbox access in a
// way that affects every school they belong to (e.g. account-wide
// password reset failed). Tenant admins cannot reach this; the surface
// lives only on /operator/accounts/{id}/mfa/global-override.
//
// "none" deletes the row, returning the account to tenant-scoped
// resolution. Force_off additionally revokes trusted devices across
// EVERY tenant the account has trusted in — this is the only override
// path that touches RevokeAllByAccountID.
func (s *mfaService) OperatorSetGlobalMFAOverride(ctx context.Context, operatorID, targetAccountID int64, override, reason string) error {
	if !IsValidMFAAdminOverride(override) {
		s.recordAdminOverrideFailure(ctx, adminOverrideFailureEvent{
			ActorType:       auth.MFAOverrideSetByTypeOperator,
			ActorID:         operatorID,
			TargetAccountID: targetAccountID,
			Action:          "set_global_override",
			Override:        override,
			Reason:          reason,
			ErrMsg:          ErrMFAInvalidOverride.Error(),
		})
		return ErrMFAInvalidOverride
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		s.recordAdminOverrideFailure(ctx, adminOverrideFailureEvent{
			ActorType:       auth.MFAOverrideSetByTypeOperator,
			ActorID:         operatorID,
			TargetAccountID: targetAccountID,
			Action:          "set_global_override",
			Override:        override,
			ErrMsg:          "reason is required",
		})
		return errors.New("reason is required for global mfa override")
	}

	previous := MFAAdminOverrideNone
	txErr := tenant.WithAdminTx(ctx, s.DB, func(txCtx context.Context, _ bun.Tx) error {
		existing, err := s.Repos.MFAOverride.FindGlobal(txCtx, targetAccountID)
		if err != nil {
			return fmt.Errorf("load existing global override: %w", err)
		}
		if existing != nil {
			previous = existing.Override
		}
		if override == MFAAdminOverrideNone {
			if err := s.Repos.MFAOverride.DeleteGlobal(txCtx, targetAccountID); err != nil {
				return fmt.Errorf("clear global mfa override: %w", err)
			}
			return nil
		}
		row := &auth.MFAOverride{
			AccountID: targetAccountID,
			TenantID:  nil,
			Override:  override,
			SetBy:     operatorID,
			SetByType: auth.MFAOverrideSetByTypeOperator,
			Reason:    reason,
		}
		if err := s.Repos.MFAOverride.UpsertGlobal(txCtx, row); err != nil {
			return fmt.Errorf("persist global mfa override: %w", err)
		}
		if override == MFAAdminOverrideForceOff {
			// Force_off at platform scope yanks trust across every
			// tenant — that's the whole point of the emergency switch.
			if err := s.Repos.MFATrustedDevice.RevokeAllByAccountID(txCtx, targetAccountID, time.Now()); err != nil {
				return fmt.Errorf("revoke trusted devices: %w", err)
			}
		}
		return nil
	})
	if txErr != nil {
		s.recordAdminOverrideFailure(ctx, adminOverrideFailureEvent{
			ActorType:       auth.MFAOverrideSetByTypeOperator,
			ActorID:         operatorID,
			TargetAccountID: targetAccountID,
			Action:          "set_global_override",
			Override:        override,
			Reason:          reason,
			ErrMsg:          txErr.Error(),
		})
		return txErr
	}

	// Account-wide override is a platform-scope operator action with no
	// single tenant. auth.auth_events is tenant-scoped (NOT NULL) and
	// recordAuthEvent drops rows without a tenant, so this audit goes to
	// platform.operator_audit_log instead — same table the operator's
	// own MFA actions use. (#1430 review round 3, finding ③)
	s.recordOperatorAudit(operatorID, targetAccountID, map[string]any{
		"action":            "set_global_override",
		"scope":             "platform",
		"override":          override,
		"previous_override": previous,
		"reason":            reason,
	})
	return nil
}

// recordOperatorAudit writes a platform.operator_audit_log row for an
// operator's account-wide MFA action. Async + recover + sentry, mirroring
// recordAuthEvent / operatorMFAService.recordAudit. The write runs on a
// detached context.Background() so it survives the request ending. Best-
// effort: failures are logged but never bubble up to the caller.
func (s *mfaService) recordOperatorAudit(operatorID, targetAccountID int64, changes map[string]any) {
	target := targetAccountID
	entry := &platformModels.OperatorAuditLog{
		OperatorID:   operatorID,
		Action:       platformModels.ActionMFAAdminOverride,
		ResourceType: platformModels.ResourceAccount,
		ResourceID:   &target,
		CreatedAt:    time.Now(),
	}
	if changes != nil {
		if buf, err := json.Marshal(changes); err == nil {
			entry.Changes = buf
		}
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Errorf("panic in operator mfa-override audit logging: %v", r)
				s.Logger.Error("goroutine panic recovered", slog.String("error", err.Error()))
				sentry.CurrentHub().Recover(r)
				sentry.Flush(2 * time.Second)
			}
		}()
		logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.Repos.OperatorAuditLog.Create(logCtx, entry); err != nil {
			s.Logger.Error("failed to log operator mfa-override audit event",
				slog.Int64("operator_id", operatorID),
				slog.Int64("account_id", targetAccountID),
				slog.String("error", err.Error()),
			)
		}
	}()
}

// requireSchoolMembership is the defense-in-depth membership check on
// the operator admin paths. The HTTP handler already verifies this at
// the route level, but mirroring it in the service means any future
// non-HTTP caller (CLI, scheduler, internal batch job) cannot reach
// across tenants. A failure produces a permission-denied error and
// emits a failure audit row.
func (s *mfaService) requireSchoolMembership(ctx context.Context, actorType string, actorID, schoolID, targetAccountID int64, action string) error {
	if schoolID == 0 {
		s.recordAdminOverrideFailure(ctx, adminOverrideFailureEvent{
			ActorType:       actorType,
			ActorID:         actorID,
			TargetAccountID: targetAccountID,
			Action:          action,
			ErrMsg:          "missing school_id",
		})
		return ErrMFAPermissionDenied
	}
	exists, err := s.Repos.AccountTenant.ExistsByAccountAndTenant(ctx, targetAccountID, schoolID)
	if err != nil {
		s.recordAdminOverrideFailure(ctx, adminOverrideFailureEvent{
			ActorType:       actorType,
			ActorID:         actorID,
			TargetAccountID: targetAccountID,
			Action:          action,
			ErrMsg:          "membership lookup failed: " + err.Error(),
		})
		return fmt.Errorf("verify account membership: %w", err)
	}
	if !exists {
		s.recordAdminOverrideFailure(ctx, adminOverrideFailureEvent{
			ActorType:       actorType,
			ActorID:         actorID,
			TargetAccountID: targetAccountID,
			Action:          action,
			ErrMsg:          "account is not a member of school",
		})
		return ErrMFAPermissionDenied
	}
	return nil
}

// adminOverrideFailureEvent carries the variable shape of a rejected
// admin-override audit row. Grouped into a struct so the recording
// helper stays under the function-parameter cap (go:S107).
type adminOverrideFailureEvent struct {
	ActorType       string
	ActorID         int64
	TargetAccountID int64
	// TargetTenantID is the tenant the audit row should be filed under.
	// Required because the admin paths often run from contexts (operator
	// surface) where tenant.FromContext(ctx) is 0 and the row would be
	// silently dropped without it. (#1430 Item #5)
	TargetTenantID int64
	Action         string
	Override       string
	Reason         string
	ErrMsg         string
}

// recordAdminOverrideFailure emits an audit row for a rejected admin
// override attempt. Success rows go through recordAuthEvent directly;
// this helper centralizes the failure shape so every rejection path
// produces consistent metadata.
func (s *mfaService) recordAdminOverrideFailure(ctx context.Context, ev adminOverrideFailureEvent) {
	metadata := map[string]any{
		"actor_type":       ev.ActorType,
		"actor_account_id": ev.ActorID,
		"action":           ev.Action,
	}
	if ev.Override != "" {
		metadata["override"] = ev.Override
	}
	if ev.Reason != "" {
		metadata["reason"] = ev.Reason
	}
	s.recordAuthEvent(ctx, ev.TargetAccountID, ev.TargetTenantID, audit.EventTypeMFAAdminOverride, false, nil, ev.ErrMsg, metadata)
}

func (s *mfaService) requireAdminPermission(actorPermissions []string) error {
	const required = "users:manage"
	for _, p := range actorPermissions {
		if p == required || hasWildcardMatch(p, required) {
			return nil
		}
	}
	return ErrMFAPermissionDenied
}

// hasWildcardMatch checks "users:*" against "users:manage" etc.
func hasWildcardMatch(granted, required string) bool {
	if !strings.HasSuffix(granted, ":*") {
		return false
	}
	prefix := strings.TrimSuffix(granted, ":*") + ":"
	return strings.HasPrefix(required, prefix) || granted == "admin:*"
}

// ===== Internal helpers =====

func (s *mfaService) parseChallengeToken(tokenString string) (*authjwt.MFAChallengeClaims, error) {
	return s.TokenAuth.ParseMFAChallengeJWT(tokenString)
}

// TrustedDeviceDays is the exported entry point for callers that need the
// resolved per-tenant value (login response, cookie issuer). Delegates to
// the same helper IssueTrustedDevice already uses, so both surfaces always
// agree on the day count.
func (s *mfaService) TrustedDeviceDays(ctx context.Context, tenantID int64) int {
	return s.resolveTrustedDeviceDays(ctx, tenantID)
}

// resolveTrustedDeviceDays resolves the tenant's
// security.mfa_trusted_device_days setting outside the
// TenantTxMiddleware — login + verify both run pre-session, so we plumb
// the tenant explicitly. Falls back to MFATrustedDeviceCookieDefaultDays
// on any error / missing override so the cookie still issues with a
// reasonable lifetime.
func (s *mfaService) resolveTrustedDeviceDays(ctx context.Context, tenantID int64) int {
	if s.Settings == nil {
		return MFATrustedDeviceCookieDefaultDays
	}
	if tenantID <= 0 {
		val, err := s.Settings.ResolveInt(ctx, configModel.KeyMFATrustedDeviceDays)
		if err != nil || val <= 0 {
			return MFATrustedDeviceCookieDefaultDays
		}
		return val
	}
	val, err := s.Settings.ResolveIntForTenant(ctx, tenantID, configModel.KeyMFATrustedDeviceDays)
	if err != nil || val <= 0 {
		return MFATrustedDeviceCookieDefaultDays
	}
	return val
}

// dispatchChallengeEmail fires the branded MFA code email asynchronously
// via the existing dispatcher. The HTML template
// (templates/email/mfa-email-code.html) handles formatting; html2text
// generates the plain-text alternative automatically.
//
// We deliberately do NOT include any links or buttons in this template —
// the user must paste the code into the moto login UI on their own.
// That kills the most common phishing pattern (a malicious mail with
// "click here to confirm").
func (s *mfaService) dispatchChallengeEmail(ctx context.Context, account *auth.Account, tenantID int64, plainCode string, ip net.IP) {
	if s.Dispatcher == nil {
		s.Logger.Warn("email dispatcher unavailable; mfa code not sent",
			slog.Int64("account_id", account.ID))
		return
	}

	frontendURL := strings.TrimRight(s.FrontendURL, "/")
	logoURL := fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", frontendURL)

	requestIP := ""
	if ip != nil && !ip.IsUnspecified() {
		requestIP = ip.String()
	}

	trustedDeviceEnabled, trustedDeviceDays := s.resolveTrustedDeviceHint(ctx, tenantID)

	message := email.Message{
		From:     s.DefaultFrom,
		To:       email.NewEmail("", account.Email),
		Subject:  "Ihr moto-Anmeldecode",
		Template: "mfa-email-code.html",
		Content: map[string]any{
			"LogoURL":              logoURL,
			"Code":                 plainCode,
			"ExpiryMinutes":        int(MFAChallengeTTL.Minutes()),
			"RequestIP":            requestIP,
			"TrustedDeviceEnabled": trustedDeviceEnabled,
			"TrustedDeviceDays":    trustedDeviceDays,
		},
	}
	meta := email.DeliveryMetadata{
		Type:        "mfa_email_code",
		ReferenceID: account.ID,
		Recipient:   account.Email,
	}
	s.Dispatcher.Dispatch(ctx, email.DeliveryRequest{
		Message:       message,
		Metadata:      meta,
		BackoffPolicy: passwordResetEmailBackoff,
		MaxAttempts:   3,
	})
}

// dispatchTrustedDeviceAddedEmail notifies the account holder by mail when a
// new trusted-device cookie has been issued. The user gets an actionable
// security signal: "this device was just added — wasn't you? remove it in
// settings." Fires asynchronously and never blocks the login flow.
func (s *mfaService) dispatchTrustedDeviceAddedEmail(ctx context.Context, accountID int64, userAgent string, ip net.IP, days int) {
	if s.Dispatcher == nil {
		s.Logger.Warn("email dispatcher unavailable; trusted-device-added mail skipped",
			slog.Int64("account_id", accountID))
		return
	}

	account, err := s.Repos.Account.FindByID(ctx, accountID)
	if err != nil || account == nil || strings.TrimSpace(account.Email) == "" {
		s.Logger.Warn("could not load account for trusted-device-added mail",
			slog.Int64("account_id", accountID),
			slog.Any("error", err))
		return
	}

	frontendURL := strings.TrimRight(s.FrontendURL, "/")
	logoURL := fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", frontendURL)

	requestIP := ""
	if ip != nil && !ip.IsUnspecified() {
		requestIP = ip.String()
	}
	deviceLabel := ShortenUserAgent(userAgent)
	addedAt := time.Now().Format("02.01.2006 15:04")

	// No deep-link to the self-service section: only admins can see it.
	// The mail now points users without that access to contact their
	// school's administration instead.
	message := email.Message{
		From:     s.DefaultFrom,
		To:       email.NewEmail("", account.Email),
		Subject:  "Neues vertrautes Gerät zu Ihrem moto-Konto hinzugefügt",
		Template: "trusted-device-added.html",
		Content: map[string]any{
			"LogoURL":     logoURL,
			"DeviceLabel": deviceLabel,
			"IPAddress":   requestIP,
			"AddedAt":     addedAt,
			"TrustedDays": days,
		},
	}
	meta := email.DeliveryMetadata{
		Type:        "mfa_trusted_device_added",
		ReferenceID: accountID,
		Recipient:   account.Email,
	}
	s.Dispatcher.Dispatch(ctx, email.DeliveryRequest{
		Message:       message,
		Metadata:      meta,
		BackoffPolicy: passwordResetEmailBackoff,
		MaxAttempts:   3,
	})
}

// ShortenUserAgent collapses a full User-Agent string to a friendly
// "Browser auf OS" label. Mirrors the frontend helper so the mail and the
// "Meine vertrauten Geräte" list show the same identity for one device.
// Exported so the operator MFA service can reuse it for its notification
// mail without duplicating the parser.
func ShortenUserAgent(ua string) string {
	if strings.TrimSpace(ua) == "" {
		return "Unbekanntes Gerät"
	}
	lower := strings.ToLower(ua)
	browser := "Browser"
	switch {
	case strings.Contains(lower, "edg/"):
		browser = "Edge"
	case strings.Contains(lower, "firefox"):
		browser = "Firefox"
	case strings.Contains(lower, "chrome") && !strings.Contains(lower, "edg/"):
		browser = "Chrome"
	case strings.Contains(lower, "safari") && !strings.Contains(lower, "chrome"):
		browser = "Safari"
	}
	osName := ""
	switch {
	case strings.Contains(lower, "iphone"):
		osName = "iPhone"
	case strings.Contains(lower, "ipad"):
		osName = "iPad"
	case strings.Contains(lower, "android"):
		osName = "Android"
	case strings.Contains(lower, "mac os"):
		osName = "macOS"
	case strings.Contains(lower, "windows"):
		osName = "Windows"
	case strings.Contains(lower, "linux"):
		osName = "Linux"
	}
	if osName == "" {
		return browser
	}
	return browser + " auf " + osName
}

// resolveTrustedDeviceHint returns (enabled, days) for the email template.
// Uses the tenant-aware resolution so a school admin's override is honoured
// instead of the registry default. On any error or missing settings service,
// falls back to (false, 0) so a misconfigured deployment skips the hint
// instead of advertising a feature that may not actually work.
func (s *mfaService) resolveTrustedDeviceHint(ctx context.Context, tenantID int64) (bool, int) {
	if s.Settings == nil {
		return false, 0
	}
	if !s.isTrustedDeviceEnabled(ctx, tenantID) {
		return false, 0
	}
	return true, s.resolveTrustedDeviceDays(ctx, tenantID)
}

// recordAuthEvent writes to audit.auth_events asynchronously, in a tenant-scoped
// transaction. Failures are logged but never bubble up to the caller — auditing
// is best-effort by design.
//
// tenantID is required so the login flow (which runs OUTSIDE TenantTxMiddleware
// and therefore has tenant.FromContext(ctx) == 0) can still produce audit rows.
// Callers that already have a tenant tx in flight may pass 0 to fall back to
// tenant.FromContext(ctx); pre-login callers MUST pass the resolved tenant
// explicitly or the audit row is silently dropped. Item #5 (PR #1430 review)
// added the parameter — previously the login-flow MFA events (email sent,
// code mismatch, verified, locked) were all skipped because no tenant context
// was set, leaving a blind spot in the audit trail.
func (s *mfaService) recordAuthEvent(ctx context.Context, accountID, tenantID int64, eventType string, success bool, ip net.IP, errorMessage string, metadata map[string]any) {
	if tenantID == 0 {
		tenantID = tenant.FromContext(ctx)
	}

	// audit.auth_events.ip_address is INET NOT NULL. MFA events from
	// internal state transitions (lockout, admin disable, async verify)
	// have no client IP; substitute the project's established sentinel
	// (see caregiverCapabilityAuditIP) so the row still lands instead of
	// being rejected by Validate() and lost.
	ipStr := mfaAuditFallbackIP
	if ip != nil {
		ipStr = ip.String()
	}
	event := audit.NewAuthEvent(accountID, eventType, success, ipStr)
	if errorMessage != "" {
		event.ErrorMessage = errorMessage
	}
	for k, v := range metadata {
		event.Metadata[k] = v
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Errorf("panic in mfa audit logging: %v", r)
				s.Logger.Error("goroutine panic recovered", slog.String("error", err.Error()))
				sentry.CurrentHub().Recover(r)
				sentry.Flush(2 * time.Second)
			}
		}()
		if tenantID == 0 {
			s.Logger.Warn("skipping mfa audit event: no tenant context",
				slog.Int64("account_id", accountID),
				slog.String("event_type", eventType),
			)
			return
		}
		event.SetTenantID(tenantID)

		logCtx, cancel := context.WithTimeout(
			tenant.WithTenantID(context.Background(), tenantID),
			5*time.Second,
		)
		defer cancel()
		err := tenant.WithTenantTx(logCtx, s.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
			return s.Repos.AuthEvent.Create(ctx, event)
		})
		if err != nil {
			s.Logger.Error("failed to log mfa audit event",
				slog.String("event_type", eventType),
				slog.String("error", err.Error()),
			)
		}
	}()
}

// AccountBelongsToTenant reports whether the account has a tenant mapping for
// the given school (issue #584; repository result verbatim).
func (s *mfaService) AccountBelongsToTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	return s.Repos.AccountTenant.ExistsByAccountAndTenant(ctx, accountID, tenantID)
}
