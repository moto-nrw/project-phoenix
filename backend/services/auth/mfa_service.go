package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/uptrace/bun"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/auth"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
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
)

// MFA admin-override values. Stored as text in auth.accounts.mfa_admin_override
// (see migration 1.15.59). Used by IsRequired to short-circuit the tenant-mode
// decision for individual accounts when an admin has explicitly opted them in
// or out — typically because a user lost mailbox access.
const (
	MFAAdminOverrideNone     = "none"
	MFAAdminOverrideForceOff = "force_off"
	MFAAdminOverrideForceOn  = "force_on"
)

// IsValidMFAAdminOverride is the allow-list guard used by handlers + service
// before writing the override column. Anything else MUST be rejected with
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

	// Email-code challenge flow.
	StartChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error)
	VerifyChallenge(ctx context.Context, challengeToken, code string) (*VerifiedChallenge, error)
	ResendChallenge(ctx context.Context, challengeToken string, ip net.IP) error

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
	// ListTrustedDevices returns all active (non-revoked, non-expired)
	// trusted-device rows for the given account. Used by the self-service
	// "Meine vertrauten Geräte" section in the admin Sicherheit tab.
	ListTrustedDevices(ctx context.Context, accountID int64) ([]*auth.MFATrustedDevice, error)
	// RevokeTrustedDevice marks a single trusted-device row revoked. The
	// service verifies the device belongs to the calling account so an
	// IDOR can't revoke someone else's device.
	RevokeTrustedDevice(ctx context.Context, accountID, deviceID int64) error
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
	// SetMFAOverride writes the per-account admin override
	// (force_off / force_on / none). Audit row + trusted-device revocation
	// (for force_off) are part of the same call so callers don't forget
	// the security hygiene step. Like AdminDisable, the actor's tenant is
	// required: cross-tenant calls are rejected as ErrMFAPermissionDenied
	// with a failure audit row.
	SetMFAOverride(ctx context.Context, actorID, actorTenantID, targetAccountID int64, override, reason string, actorPermissions []string) error
	// GetMFAOverride returns the current per-account override value.
	// Read-side helper used by admin/operator surfaces to drive the
	// settings modal — returns "none" when no row is present.
	GetMFAOverride(ctx context.Context, accountID int64) (string, error)
	// OperatorAdminDisable is the operator-side variant of AdminDisable.
	// Skips the users:manage check because operator routes are already
	// gated at the platform JWT layer, and writes audit metadata with
	// actor_type=operator so the audit log can distinguish operator vs.
	// tenant-admin actions on the same account. The service verifies the
	// target account belongs to the given school as defense-in-depth so
	// any future direct caller (CLI, scheduler) can't act cross-tenant.
	OperatorAdminDisable(ctx context.Context, operatorID, schoolID, targetAccountID int64, reason string) error
	// OperatorSetMFAOverride is the operator-side variant of
	// SetMFAOverride. Same permission/audit/membership treatment as
	// OperatorAdminDisable.
	OperatorSetMFAOverride(ctx context.Context, operatorID, schoolID, targetAccountID int64, override, reason string) error
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
	repos       *repositories.Factory
	tokenAuth   *authjwt.TokenAuth
	settings    config.SettingsService
	dispatcher  *email.Dispatcher
	defaultFrom email.Email
	frontendURL string
	mfaSecret   []byte
	db          *bun.DB
	logger      *slog.Logger
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
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &mfaService{
		repos:       cfg.Repos,
		tokenAuth:   cfg.TokenAuth,
		settings:    cfg.Settings,
		dispatcher:  cfg.Dispatcher,
		defaultFrom: cfg.DefaultFrom,
		frontendURL: cfg.FrontendURL,
		mfaSecret:   DeriveMFASecret(cfg.JWTSecret),
		db:          cfg.DB,
		logger:      logger.With("service", "mfa"),
	}, nil
}

// ===== Inquiry =====

// IsRequired evaluates the tenant's security.mfa_mode setting against the
// account's roles. Operator (platform-scope) sessions are handled by the
// platform service in a later phase — this implementation rejects them.
func (s *mfaService) IsRequired(ctx context.Context, account *auth.Account, tenantID int64) (bool, error) {
	if account == nil {
		return false, errors.New("account is required")
	}
	// Per-account admin override wins over the tenant-mode decision. This
	// is the "user lost mailbox access" escape hatch and the inverse
	// "this admin must always 2FA even if the school is off" hardening.
	switch account.MFAAdminOverride {
	case MFAAdminOverrideForceOff:
		return false, nil
	case MFAAdminOverrideForceOn:
		return true, nil
	}
	mode := configModel.MFAModeOff
	if s.settings != nil {
		// Login runs outside the tenant-transaction middleware, so
		// tenant.FromContext(ctx) is 0. Use ResolveStringForTenant when
		// the caller passed the resolved tenant explicitly; fall back to
		// ResolveString (registry default) otherwise.
		var (
			val string
			err error
		)
		if tenantID > 0 {
			val, err = s.settings.ResolveStringForTenant(ctx, tenantID, configModel.KeyMFAMode)
		} else {
			val, err = s.settings.ResolveString(ctx, configModel.KeyMFAMode)
		}
		if err != nil {
			// Settings infra error (DB timeout, connection drop). We can't
			// tell whether the tenant has opted in or out, so failing-open
			// to "off" would silently downgrade security; fail-closed to
			// "required" would lock everyone out on a registry hiccup.
			// Refuse THIS login instead — caller maps to 503.
			s.logger.Warn("mfa_mode resolve failed; refusing login",
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
		return account.HasRole("admin"), nil
	default:
		s.logger.Warn("unknown mfa_mode value; treating as off", slog.String("value", mode))
		return false, nil
	}
}

func (s *mfaService) HasEnrollment(ctx context.Context, accountID int64) (bool, error) {
	cred, err := s.repos.MFACredential.FindByAccountID(ctx, accountID)
	if err != nil {
		// sql.ErrNoRows is the legitimate "not enrolled" signal — every
		// fresh account hits it. Anything else (DB timeout, connection
		// drop) means we can't actually answer the question; refuse this
		// login instead of fail-open returning false. errors.Is walks
		// through DatabaseError.Unwrap() so the wrapped sentinel matches.
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		s.logger.Warn("mfa enrollment lookup failed; refusing login",
			slog.Int64("account_id", accountID),
			slog.String("error", err.Error()))
		return false, ErrMFAStatusUnavailable
	}
	return cred != nil && cred.ID > 0, nil
}

// ===== Challenge / verify =====

func (s *mfaService) StartChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error) {
	if scope != authjwt.MFAChallengeScopeTenant {
		return "", ErrMFAUnsupportedScope
	}

	account, err := s.repos.Account.FindByID(ctx, accountID)
	if err != nil {
		return "", fmt.Errorf("look up account: %w", err)
	}
	if account.IsMFALocked() {
		return "", ErrMFALocked
	}

	// Rate-limit code issuance. Cooldown is per-tenant configurable but the
	// hard cap (3 codes / 15 min) stays in code — it's an abuse defense, not
	// a UX knob.
	since := time.Now().Add(-MFAEmailRateLimitWindow)
	count, err := s.repos.MFAEmailChallenge.CountRecentByAccountID(ctx, accountID, since)
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
	if err := s.repos.MFAEmailChallenge.Create(ctx, challenge); err != nil {
		return "", fmt.Errorf("persist email challenge: %w", err)
	}

	s.dispatchChallengeEmail(ctx, account, tenantID, plainCode, ip)
	s.recordAuthEvent(ctx, accountID, audit.EventTypeMFAEmailSent, true, ip, "", map[string]any{
		"challenge_id": challenge.ID,
	})

	tokenString, err := s.tokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
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
	claims, err := s.parseChallengeToken(challengeToken)
	if err != nil {
		return nil, ErrMFAChallengeTokenInvalid
	}
	if claims.Scope != authjwt.MFAChallengeScopeTenant {
		return nil, ErrMFAUnsupportedScope
	}

	account, err := s.repos.Account.FindByID(ctx, claims.AccountID)
	if err != nil {
		return nil, ErrMFAChallengeTokenInvalid
	}
	if account.IsMFALocked() {
		return nil, ErrMFALocked
	}

	active, err := s.repos.MFAEmailChallenge.FindActiveByAccountID(ctx, claims.AccountID)
	if err != nil || active == nil {
		s.recordAuthEvent(ctx, claims.AccountID, audit.EventTypeMFAFailed, false, nil, "no active challenge", nil)
		return nil, ErrMFACodeInvalid
	}

	ok, verifyErr := VerifyShortCode(code, active.CodeHash)
	if verifyErr != nil || !ok {
		s.handleFailedAttempt(ctx, account)
		s.recordAuthEvent(ctx, claims.AccountID, audit.EventTypeMFAFailed, false, nil, "code mismatch", nil)
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
	if err := s.repos.MFAEmailChallenge.MarkConsumed(ctx, active.ID, now); err != nil {
		s.logger.Warn("failed to mark challenge consumed; refusing verify",
			slog.Int64("account_id", claims.AccountID),
			slog.Int64("challenge_id", active.ID),
			slog.String("error", err.Error()))
		s.recordAuthEvent(ctx, claims.AccountID, audit.EventTypeMFAFailed, false, nil, "consume race", nil)
		return nil, ErrMFACodeInvalid
	}
	account.ResetMFAAttempts()
	if err := s.repos.Account.Update(ctx, account); err != nil {
		s.logger.Warn("failed to reset MFA attempts", slog.String("error", err.Error()))
	}

	cred, _ := s.repos.MFACredential.FindByAccountID(ctx, claims.AccountID)
	if cred != nil && cred.ID > 0 {
		_ = s.repos.MFACredential.UpdateLastUsedAt(ctx, cred.ID, now)
	}

	s.recordAuthEvent(ctx, claims.AccountID, audit.EventTypeMFAVerified, true, nil, "", nil)
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
	account, err := s.repos.Account.FindByID(ctx, accountID)
	if err != nil {
		return ErrMFACodeInvalid
	}
	if account.IsMFALocked() {
		return ErrMFALocked
	}
	active, err := s.repos.MFAEmailChallenge.FindActiveByAccountID(ctx, accountID)
	if err != nil || active == nil {
		s.recordAuthEvent(ctx, accountID, audit.EventTypeMFAFailed, false, nil, "no active challenge", nil)
		return ErrMFACodeInvalid
	}
	ok, vErr := VerifyShortCode(code, active.CodeHash)
	if vErr != nil || !ok {
		s.handleFailedAttempt(ctx, account)
		s.recordAuthEvent(ctx, accountID, audit.EventTypeMFAFailed, false, nil, "code mismatch", nil)
		return ErrMFACodeInvalid
	}
	now := time.Now()
	if err := s.repos.MFAEmailChallenge.MarkConsumed(ctx, active.ID, now); err != nil {
		// Same single-use guard as VerifyChallenge — if the atomic UPDATE
		// reports 0 rows affected, another concurrent caller already
		// consumed this code. Refuse this caller so the code remains
		// single-use across racing requests.
		s.logger.Warn("failed to mark challenge consumed; refusing verify",
			slog.Int64("account_id", accountID),
			slog.Int64("challenge_id", active.ID),
			slog.String("error", err.Error()))
		s.recordAuthEvent(ctx, accountID, audit.EventTypeMFAFailed, false, nil, "consume race", nil)
		return ErrMFACodeInvalid
	}
	account.ResetMFAAttempts()
	_ = s.repos.Account.Update(ctx, account)
	s.recordAuthEvent(ctx, accountID, audit.EventTypeMFAVerified, true, nil, "", nil)
	return nil
}

func (s *mfaService) ResendChallenge(ctx context.Context, challengeToken string, ip net.IP) error {
	claims, err := s.parseChallengeToken(challengeToken)
	if err != nil {
		return ErrMFAChallengeTokenInvalid
	}

	// No per-resend cooldown gate — the sliding-window cap inside
	// StartChallenge (3 codes / 15 min) remains as the abuse defense.
	if _, err := s.StartChallenge(ctx, claims.AccountID, claims.TenantID, claims.Scope, ip); err != nil {
		return err
	}
	return nil
}

// handleFailedAttempt increments the lockout counter and emits an mfa_locked
// event when the threshold is hit. Errors here are logged but never bubble
// up — VerifyChallenge already returns a generic "invalid code" message and
// must keep timing consistent.
func (s *mfaService) handleFailedAttempt(ctx context.Context, account *auth.Account) {
	wasLocked := account.IsMFALocked()
	account.IncrementMFAAttempts()
	if err := s.repos.Account.Update(ctx, account); err != nil {
		s.logger.Warn("failed to persist MFA attempt counter", slog.String("error", err.Error()))
		return
	}
	if !wasLocked && account.IsMFALocked() {
		s.recordAuthEvent(ctx, account.ID, audit.EventTypeMFALocked, false, nil, "", map[string]any{
			"locked_until": account.MFALockedUntil,
		})
	}
}

// ===== Enrollment / lifecycle =====

func (s *mfaService) Enroll(ctx context.Context, accountID int64) error {
	existing, _ := s.repos.MFACredential.FindByAccountID(ctx, accountID)
	if existing != nil && existing.ID > 0 {
		return ErrMFAAlreadyEnrolled
	}
	cred := &auth.MFACredential{
		AccountID:  accountID,
		Method:     auth.MFAMethodEmail,
		EnrolledAt: time.Now(),
	}
	if err := s.repos.MFACredential.Create(ctx, cred); err != nil {
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
	err := tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		if err := s.repos.MFACredential.DeleteByAccountID(txCtx, accountID); err != nil {
			return fmt.Errorf("delete credential: %w", err)
		}
		if err := s.repos.MFATrustedDevice.RevokeAllByAccountID(txCtx, accountID, time.Now()); err != nil {
			return fmt.Errorf("revoke trusted devices: %w", err)
		}
		if account, err := s.repos.Account.FindByID(txCtx, accountID); err == nil && account != nil {
			account.ResetMFAAttempts()
			if err := s.repos.Account.Update(txCtx, account); err != nil {
				return fmt.Errorf("reset mfa attempts: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.recordAuthEvent(ctx, accountID, audit.EventTypeMFADisabled, true, nil, "", nil)
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
	if s.settings == nil {
		return true
	}
	if tenantID <= 0 {
		enabled, err := s.settings.ResolveBool(ctx, configModel.KeyMFATrustedDeviceEnabled)
		if err != nil {
			s.logger.Warn("trusted_device_enabled resolve failed; disabling feature",
				slog.String("error", err.Error()))
			return false
		}
		return enabled
	}
	enabled, err := s.settings.ResolveBoolForTenant(ctx, tenantID, configModel.KeyMFATrustedDeviceEnabled)
	if err != nil {
		s.logger.Warn("trusted_device_enabled resolve failed; disabling feature",
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
		TokenHash: HashTrustedDeviceToken(rawToken),
		IPAddress: ip,
		ExpiresAt: expiresAt,
	}
	if userAgent != "" {
		ua := userAgent
		device.UserAgent = &ua
	}
	if err := s.repos.MFATrustedDevice.Create(ctx, device); err != nil {
		return "", time.Time{}, fmt.Errorf("persist trusted device: %w", err)
	}
	signed := SignTrustedDeviceToken(rawToken, s.mfaSecret)
	s.recordAuthEvent(ctx, accountID, audit.EventTypeMFATrustedDeviceAdded, true, ip, "", map[string]any{
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
	device, err := s.repos.MFATrustedDevice.FindActiveByAccountIDAndTokenHash(ctx, accountID, tokenHash)
	if err != nil || device == nil {
		return false, nil
	}
	_ = s.repos.MFATrustedDevice.UpdateLastUsedAt(ctx, device.ID, time.Now())
	return true, nil
}

func (s *mfaService) ListTrustedDevices(ctx context.Context, accountID int64) ([]*auth.MFATrustedDevice, error) {
	return s.repos.MFATrustedDevice.ListActiveByAccountID(ctx, accountID)
}

func (s *mfaService) RevokeTrustedDevice(ctx context.Context, accountID, deviceID int64) error {
	// Validate ownership before revoke — a device row can only be revoked by
	// its own account so an attacker with a stolen access token for account A
	// can't revoke account B's devices via id-guessing.
	devices, err := s.repos.MFATrustedDevice.ListActiveByAccountID(ctx, accountID)
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
	return s.repos.MFATrustedDevice.Revoke(ctx, deviceID, time.Now())
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
	return s.adminDisableCore(ctx, "account", actorID, targetAccountID, reason)
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
	return s.adminDisableCore(ctx, "operator", operatorID, targetAccountID, reason)
}

func (s *mfaService) GetMFAOverride(ctx context.Context, accountID int64) (string, error) {
	account, err := s.repos.Account.FindByID(ctx, accountID)
	if err != nil || account == nil {
		return "", fmt.Errorf("load account: %w", err)
	}
	if account.MFAAdminOverride == "" {
		return MFAAdminOverrideNone, nil
	}
	return account.MFAAdminOverride, nil
}

// adminDisableCore is the shared cascade used by both tenant-admin and
// operator paths. actorType becomes audit metadata so the audit log can
// tell which surface initiated the disable. Both rejected attempts and
// successful disables produce an audit row so brute-force / scanning
// behaviour is forensically visible.
func (s *mfaService) adminDisableCore(ctx context.Context, actorType string, actorID, targetAccountID int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		s.recordAdminOverrideFailure(ctx, adminOverrideFailureEvent{
			ActorType:       actorType,
			ActorID:         actorID,
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
			TargetAccountID: targetAccountID,
			Action:          "disable",
			Reason:          reason,
			ErrMsg:          err.Error(),
		})
		return err
	}
	s.recordAuthEvent(ctx, targetAccountID, audit.EventTypeMFAAdminOverride, true, nil, "", map[string]any{
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
	return s.setMFAOverrideCore(ctx, "account", actorID, targetAccountID, override, reason)
}

// OperatorSetMFAOverride mirrors SetMFAOverride for the operator path.
// Skips the users:manage check (route layer guards this) and writes
// audit metadata tagged actor_type=operator. Verifies the target account
// belongs to the given school as defense-in-depth.
func (s *mfaService) OperatorSetMFAOverride(ctx context.Context, operatorID, schoolID, targetAccountID int64, override, reason string) error {
	if err := s.requireSchoolMembership(ctx, "operator", operatorID, schoolID, targetAccountID, "set_override"); err != nil {
		return err
	}
	return s.setMFAOverrideCore(ctx, "operator", operatorID, targetAccountID, override, reason)
}

// setMFAOverrideCore is the shared write + trusted-device-revoke +
// audit-record pipeline used by both tenant-admin and operator flows.
// The account update and trusted-device revoke run in a single tx so a
// failed revoke rolls back the override flip — otherwise a force_off
// could leave the account flagged "no MFA" while stale trusted-device
// cookies remain valid. Rejected attempts (bad value, empty reason,
// missing target) produce a failure audit row so abuse is observable.
func (s *mfaService) setMFAOverrideCore(ctx context.Context, actorType string, actorID, targetAccountID int64, override, reason string) error {
	if !IsValidMFAAdminOverride(override) {
		s.recordAdminOverrideFailure(ctx, adminOverrideFailureEvent{
			ActorType:       actorType,
			ActorID:         actorID,
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
			TargetAccountID: targetAccountID,
			Action:          "set_override",
			Override:        override,
			ErrMsg:          "reason is required",
		})
		return errors.New("reason is required for admin override")
	}

	var previous string
	txErr := tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		account, err := s.repos.Account.FindByID(txCtx, targetAccountID)
		if err != nil || account == nil {
			return fmt.Errorf("load target account: %w", err)
		}
		previous = account.MFAAdminOverride
		if previous == "" {
			previous = MFAAdminOverrideNone
		}
		account.MFAAdminOverride = override
		if err := s.repos.Account.Update(txCtx, account); err != nil {
			return fmt.Errorf("persist mfa override: %w", err)
		}
		// Force-off must revoke every existing trusted-device cookie in
		// the same tx — a partial success (override flipped but devices
		// kept) is a security regression, not a transient warning.
		if override == MFAAdminOverrideForceOff {
			if err := s.repos.MFATrustedDevice.RevokeAllByAccountID(txCtx, targetAccountID, time.Now()); err != nil {
				return fmt.Errorf("revoke trusted devices: %w", err)
			}
		}
		return nil
	})
	if txErr != nil {
		s.recordAdminOverrideFailure(ctx, adminOverrideFailureEvent{
			ActorType:       actorType,
			ActorID:         actorID,
			TargetAccountID: targetAccountID,
			Action:          "set_override",
			Override:        override,
			Reason:          reason,
			ErrMsg:          txErr.Error(),
		})
		return txErr
	}

	s.recordAuthEvent(ctx, targetAccountID, audit.EventTypeMFAAdminOverride, true, nil, "", map[string]any{
		"actor_type":        actorType,
		"actor_account_id":  actorID,
		"action":            "set_override",
		"override":          override,
		"previous_override": previous,
		"reason":            reason,
	})
	return nil
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
	exists, err := s.repos.AccountTenant.ExistsByAccountAndTenant(ctx, targetAccountID, schoolID)
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
	Action          string
	Override        string
	Reason          string
	ErrMsg          string
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
	s.recordAuthEvent(ctx, ev.TargetAccountID, audit.EventTypeMFAAdminOverride, false, nil, ev.ErrMsg, metadata)
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
	return s.tokenAuth.ParseMFAChallengeJWT(tokenString)
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
	if s.settings == nil {
		return MFATrustedDeviceCookieDefaultDays
	}
	if tenantID <= 0 {
		val, err := s.settings.ResolveInt(ctx, configModel.KeyMFATrustedDeviceDays)
		if err != nil || val <= 0 {
			return MFATrustedDeviceCookieDefaultDays
		}
		return val
	}
	val, err := s.settings.ResolveIntForTenant(ctx, tenantID, configModel.KeyMFATrustedDeviceDays)
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
	if s.dispatcher == nil {
		s.logger.Warn("email dispatcher unavailable; mfa code not sent",
			slog.Int64("account_id", account.ID))
		return
	}

	frontendURL := strings.TrimRight(s.frontendURL, "/")
	logoURL := fmt.Sprintf("%s/images/moto_transparent.png", frontendURL)

	requestIP := ""
	if ip != nil && !ip.IsUnspecified() {
		requestIP = ip.String()
	}

	trustedDeviceEnabled, trustedDeviceDays := s.resolveTrustedDeviceHint(ctx, tenantID)

	message := email.Message{
		From:     s.defaultFrom,
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
	s.dispatcher.Dispatch(ctx, email.DeliveryRequest{
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
	if s.dispatcher == nil {
		s.logger.Warn("email dispatcher unavailable; trusted-device-added mail skipped",
			slog.Int64("account_id", accountID))
		return
	}

	account, err := s.repos.Account.FindByID(ctx, accountID)
	if err != nil || account == nil || strings.TrimSpace(account.Email) == "" {
		s.logger.Warn("could not load account for trusted-device-added mail",
			slog.Int64("account_id", accountID),
			slog.Any("error", err))
		return
	}

	frontendURL := strings.TrimRight(s.frontendURL, "/")
	logoURL := fmt.Sprintf("%s/images/moto_transparent.png", frontendURL)

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
		From:     s.defaultFrom,
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
	s.dispatcher.Dispatch(ctx, email.DeliveryRequest{
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
	if s.settings == nil {
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
func (s *mfaService) recordAuthEvent(ctx context.Context, accountID int64, eventType string, success bool, ip net.IP, errorMessage string, metadata map[string]any) {
	tenantID := tenant.FromContext(ctx)

	ipStr := ""
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
				s.logger.Error("goroutine panic recovered", slog.String("error", err.Error()))
				sentry.CurrentHub().Recover(r)
				sentry.Flush(2 * time.Second)
			}
		}()
		if tenantID == 0 {
			s.logger.Warn("skipping mfa audit event: no tenant context",
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
		err := tenant.WithTenantTx(logCtx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
			return s.repos.AuthEvent.Create(ctx, event)
		})
		if err != nil {
			s.logger.Error("failed to log mfa audit event",
				slog.String("event_type", eventType),
				slog.String("error", err.Error()),
			)
		}
	}()
}
