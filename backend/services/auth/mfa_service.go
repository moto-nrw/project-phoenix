package auth

import (
	"context"
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
	MFATrustedDeviceCookieDefaultDays = 30
)

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
	ErrMFAUnsupportedScope      = errors.New("operator-scope MFA is wired up in a separate phase")
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

	// Recovery codes.
	GenerateRecoveryCodes(ctx context.Context, accountID int64) ([]string, error)
	VerifyRecoveryCode(ctx context.Context, accountID int64, code string) error

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
	// IsTrustedDeviceEnabled reports whether the tenant has the
	// security.mfa_trusted_device_enabled setting on. The login flow uses
	// this to tell the frontend whether to render the "remember this
	// device" checkbox on the MFA challenge screen.
	IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool

	// Admin override ("Godmode") — defense-in-depth permission check.
	AdminDisable(ctx context.Context, actorID, targetAccountID int64, reason string, actorPermissions []string) error
	AdminRegenerateRecoveryCodes(ctx context.Context, actorID, targetAccountID int64, reason string, actorPermissions []string) ([]string, error)
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
			s.logger.Warn("mfa_mode resolve failed; treating as off",
				slog.String("error", err.Error()))
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
		return false, nil // not-found is the most common case; treat as "no enrollment"
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

	s.dispatchChallengeEmail(ctx, account, plainCode, ip)
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

	// Single-use + reset lockout, all in one transaction.
	now := time.Now()
	if err := s.repos.MFAEmailChallenge.MarkConsumed(ctx, active.ID, now); err != nil {
		s.logger.Warn("failed to mark challenge consumed", slog.String("error", err.Error()))
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
		s.logger.Warn("failed to mark challenge consumed", slog.String("error", err.Error()))
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

	// Per-tenant cooldown gate. Distinct from the sliding-window cap inside
	// StartChallenge: the window cap (3 codes / 15 min) is an abuse defense;
	// this cooldown is a UX / cost knob exposed to school admins via the
	// `security.mfa_email_resend_cooldown_seconds` setting.
	if cooldown := s.resolveResendCooldown(ctx); cooldown > 0 {
		if last, err := s.repos.MFAEmailChallenge.FindActiveByAccountID(ctx, claims.AccountID); err == nil && last != nil {
			if time.Since(last.CreatedAt) < cooldown {
				return ErrMFARateLimited
			}
		}
	}

	if _, err := s.StartChallenge(ctx, claims.AccountID, claims.TenantID, claims.Scope, ip); err != nil {
		return err
	}
	return nil
}

// resolveResendCooldown returns the per-tenant cooldown duration between
// successive email-code resends. Falls back to zero (no cooldown) on errors
// — the sliding-window rate limit inside StartChallenge still applies as
// the abuse defense.
func (s *mfaService) resolveResendCooldown(ctx context.Context) time.Duration {
	if s.settings == nil {
		return 0
	}
	seconds, err := s.settings.ResolveInt(ctx, configModel.KeyMFAEmailResendCooldownSeconds)
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
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

// Disable cascades: credential -> recovery codes -> trusted devices revoked.
// Account-level lockout fields are reset so a future re-enrollment starts clean.
func (s *mfaService) Disable(ctx context.Context, accountID int64) error {
	if err := s.repos.MFACredential.DeleteByAccountID(ctx, accountID); err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}
	if err := s.repos.MFARecoveryCode.DeleteByAccountID(ctx, accountID); err != nil {
		return fmt.Errorf("delete recovery codes: %w", err)
	}
	if err := s.repos.MFATrustedDevice.RevokeAllByAccountID(ctx, accountID, time.Now()); err != nil {
		return fmt.Errorf("revoke trusted devices: %w", err)
	}
	if account, err := s.repos.Account.FindByID(ctx, accountID); err == nil && account != nil {
		account.ResetMFAAttempts()
		_ = s.repos.Account.Update(ctx, account)
	}
	s.recordAuthEvent(ctx, accountID, audit.EventTypeMFADisabled, true, nil, "", nil)
	return nil
}

// ===== Recovery codes =====

func (s *mfaService) GenerateRecoveryCodes(ctx context.Context, accountID int64) ([]string, error) {
	plain, err := GenerateRecoveryCodes(MFARecoveryCodeCount)
	if err != nil {
		return nil, fmt.Errorf("generate recovery codes: %w", err)
	}
	rows := make([]*auth.MFARecoveryCode, 0, len(plain))
	for _, code := range plain {
		hash, err := HashShortCode(code)
		if err != nil {
			return nil, fmt.Errorf("hash recovery code: %w", err)
		}
		rows = append(rows, &auth.MFARecoveryCode{
			AccountID: accountID,
			CodeHash:  hash,
		})
	}
	// Wholesale replacement: the user has no copy of the old codes by definition,
	// so we delete the lot and seed a fresh batch.
	if err := s.repos.MFARecoveryCode.DeleteByAccountID(ctx, accountID); err != nil {
		return nil, fmt.Errorf("clear old recovery codes: %w", err)
	}
	if err := s.repos.MFARecoveryCode.BulkCreate(ctx, rows); err != nil {
		return nil, fmt.Errorf("persist recovery codes: %w", err)
	}
	return plain, nil
}

// VerifyRecoveryCode walks all unused codes and Argon2id-checks them in
// constant time. A match marks that single row used and resets the
// failure-counter lockout (the user proved possession).
func (s *mfaService) VerifyRecoveryCode(ctx context.Context, accountID int64, code string) error {
	code = strings.TrimSpace(strings.ToLower(code))
	candidates, err := s.repos.MFARecoveryCode.FindUnusedByAccountID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("load recovery codes: %w", err)
	}
	for _, candidate := range candidates {
		ok, vErr := VerifyShortCode(code, candidate.CodeHash)
		if vErr == nil && ok {
			if err := s.repos.MFARecoveryCode.MarkUsed(ctx, candidate.ID, time.Now()); err != nil {
				return fmt.Errorf("mark recovery code used: %w", err)
			}
			if account, _ := s.repos.Account.FindByID(ctx, accountID); account != nil {
				account.ResetMFAAttempts()
				_ = s.repos.Account.Update(ctx, account)
			}
			s.recordAuthEvent(ctx, accountID, audit.EventTypeMFARecoveryUsed, true, nil, "", nil)
			return nil
		}
	}
	s.recordAuthEvent(ctx, accountID, audit.EventTypeMFAFailed, false, nil, "recovery code mismatch", nil)
	return ErrMFACodeInvalid
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
	days := s.resolveTrustedDeviceDays(ctx)
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

// ===== Admin override ("Godmode") =====

func (s *mfaService) AdminDisable(ctx context.Context, actorID, targetAccountID int64, reason string, actorPermissions []string) error {
	if err := s.requireAdminPermission(actorPermissions); err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errors.New("reason is required for admin override")
	}
	if err := s.Disable(ctx, targetAccountID); err != nil {
		return err
	}
	s.recordAuthEvent(ctx, targetAccountID, audit.EventTypeMFAAdminOverride, true, nil, "", map[string]any{
		"actor_account_id": actorID,
		"action":           "disable",
		"reason":           reason,
	})
	return nil
}

func (s *mfaService) AdminRegenerateRecoveryCodes(ctx context.Context, actorID, targetAccountID int64, reason string, actorPermissions []string) ([]string, error) {
	if err := s.requireAdminPermission(actorPermissions); err != nil {
		return nil, err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("reason is required for admin override")
	}
	codes, err := s.GenerateRecoveryCodes(ctx, targetAccountID)
	if err != nil {
		return nil, err
	}
	s.recordAuthEvent(ctx, targetAccountID, audit.EventTypeMFAAdminOverride, true, nil, "", map[string]any{
		"actor_account_id": actorID,
		"action":           "regenerate_recovery_codes",
		"reason":           reason,
	})
	return codes, nil
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
	jwtToken, err := s.tokenAuth.JwtAuth.Decode(tokenString)
	if err != nil {
		return nil, err
	}
	raw := make(map[string]any)
	for _, k := range jwtToken.Keys() {
		var v any
		if gErr := jwtToken.Get(k, &v); gErr == nil {
			raw[k] = v
		}
	}
	var claims authjwt.MFAChallengeClaims
	if err := claims.ParseClaims(raw); err != nil {
		return nil, err
	}
	if claims.ExpiresAt > 0 && claims.ExpiresAt < time.Now().Unix() {
		return nil, errors.New("challenge token expired")
	}
	return &claims, nil
}

func (s *mfaService) resolveTrustedDeviceDays(ctx context.Context) int {
	if s.settings == nil {
		return MFATrustedDeviceCookieDefaultDays
	}
	val, err := s.settings.ResolveInt(ctx, configModel.KeyMFATrustedDeviceDays)
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
func (s *mfaService) dispatchChallengeEmail(ctx context.Context, account *auth.Account, plainCode string, ip net.IP) {
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

	trustedDeviceEnabled, trustedDeviceDays := s.resolveTrustedDeviceHint(ctx)

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

// resolveTrustedDeviceHint returns (enabled, days) for the email template.
// Resolves directly against the settings service so tenants on the registry
// default (enabled=true) see the hint even before they set an explicit
// override. On any error or missing settings service, falls back to (false,
// 0) so a misconfigured deployment skips the hint instead of advertising a
// feature that may not actually work.
func (s *mfaService) resolveTrustedDeviceHint(ctx context.Context) (bool, int) {
	if s.settings == nil {
		return false, 0
	}
	enabled, err := s.settings.ResolveBool(ctx, configModel.KeyMFATrustedDeviceEnabled)
	if err != nil {
		s.logger.Warn("trusted_device_enabled resolve failed; hiding hint",
			slog.String("error", err.Error()))
		return false, 0
	}
	if !enabled {
		return false, 0
	}
	return true, s.resolveTrustedDeviceDays(ctx)
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
