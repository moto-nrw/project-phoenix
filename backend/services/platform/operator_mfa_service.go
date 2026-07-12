package platform

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
	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/platform"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Operator MFA constants. The cooldown / lockout / TTL values are not
// configurable for operators by design — moto-Operators always face the
// same security posture. The lockout threshold/duration reference the
// auth-side constants so the 5-attempt / 15-minute brute-force policy has a
// single source of truth across the platform (issue #586 — no second copy of
// the literals; the per-tenant security.account_lockout_* settings only apply
// to tenant accounts, which operators are not).
const (
	OperatorMFAChallengeTTL          = 10 * time.Minute
	OperatorMFALockoutThreshold      = authService.MFALockoutThreshold
	OperatorMFALockoutDuration       = authService.MFALockoutDuration
	OperatorMFARateLimitWindow       = 15 * time.Minute
	OperatorMFARateLimitMaxSent      = 3
	OperatorMFATrustedDeviceDuration = 90 * 24 * time.Hour
)

// Errors mirror the auth-side surface so callers can switch on error
// identity. We reuse the auth-package errors where the meaning is
// identical to keep the API surface lean.
var (
	ErrOperatorMFAChallengeTokenInvalid = authService.ErrMFAChallengeTokenInvalid
	ErrOperatorMFACodeInvalid           = authService.ErrMFACodeInvalid
	ErrOperatorMFALocked                = authService.ErrMFALocked
	ErrOperatorMFARateLimited           = authService.ErrMFARateLimited
	ErrOperatorMFAAlreadyEnrolled       = authService.ErrMFAAlreadyEnrolled
	ErrOperatorMFAPermissionDenied      = authService.ErrMFAPermissionDenied
)

// OperatorVerifiedChallenge mirrors authService.VerifiedChallenge for the
// platform scope. Returned by VerifyChallenge so the operator login flow
// can mint a token pair without re-decoding the challenge JWT.
type OperatorVerifiedChallenge struct {
	OperatorID int64
}

// OperatorMFAService is the public interface consumed by the operator
// login flow + (Phase 7b-4) operator HTTP handlers.
type OperatorMFAService interface {
	HasEnrollment(ctx context.Context, operatorID int64) (bool, error)

	StartChallenge(ctx context.Context, operatorID int64, ip net.IP) (string, error)
	VerifyChallenge(ctx context.Context, challengeToken, code string) (*OperatorVerifiedChallenge, error)
	// ResendChallenge — see tenant counterpart. Returns the renewed
	// challenge JWT so the operator frontend can replace its in-flight
	// token. Returning only an error like the previous shape created a
	// dead-end where the freshly emailed code couldn't be verified once
	// the original JWT expired.
	ResendChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error)
	VerifyCodeForOperator(ctx context.Context, operatorID int64, code string) error

	Enroll(ctx context.Context, operatorID int64) error
	Disable(ctx context.Context, operatorID int64) error

	IssueTrustedDevice(ctx context.Context, operatorID int64, userAgent string, ip net.IP) (cookieValue string, expiresAt time.Time, err error)
	VerifyTrustedDevice(ctx context.Context, operatorID int64, signedCookie string) (bool, error)
	// ListTrustedDevices returns all active (non-revoked, non-expired)
	// trusted-device rows for the given operator. Used by the self-service
	// "Meine vertrauten Geräte" section in the operator settings page.
	ListTrustedDevices(ctx context.Context, operatorID int64) ([]*platform.OperatorMFATrustedDevice, error)
	// RevokeTrustedDevice marks a single trusted-device row revoked. The
	// service verifies the device belongs to the calling operator so an
	// IDOR can't revoke someone else's device.
	RevokeTrustedDevice(ctx context.Context, operatorID, deviceID int64) error
}

// OperatorMFAServiceConfig groups dependencies for NewOperatorMFAService.
type OperatorMFAServiceConfig struct {
	Repos       *repositories.Factory
	TokenAuth   *authjwt.TokenAuth
	Dispatcher  *email.Dispatcher
	DefaultFrom email.Email
	FrontendURL string
	JWTSecret   string
	DB          *bun.DB
	Logger      *slog.Logger
}

type operatorMFAService struct {
	OperatorMFAServiceConfig
	mfaSecret []byte
}

var _ OperatorMFAService = (*operatorMFAService)(nil)

// NewOperatorMFAService constructs the service. Returns an error rather
// than panicking so wiring problems surface at startup.
func NewOperatorMFAService(cfg OperatorMFAServiceConfig) (OperatorMFAService, error) {
	if cfg.Repos == nil {
		return nil, errors.New("OperatorMFAServiceConfig.Repos is required")
	}
	if cfg.TokenAuth == nil {
		return nil, errors.New("OperatorMFAServiceConfig.TokenAuth is required")
	}
	if cfg.DB == nil {
		return nil, errors.New("OperatorMFAServiceConfig.DB is required")
	}
	if cfg.JWTSecret == "" {
		return nil, errors.New("OperatorMFAServiceConfig.JWTSecret is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	cfg.Logger = cfg.Logger.With("service", "operator-mfa")
	return &operatorMFAService{
		OperatorMFAServiceConfig: cfg,
		mfaSecret:                authService.DeriveMFASecret(cfg.JWTSecret),
	}, nil
}

// ===== Inquiry =====

func (s *operatorMFAService) HasEnrollment(ctx context.Context, operatorID int64) (bool, error) {
	cred, err := s.Repos.OperatorMFACredential.FindByOperatorID(ctx, operatorID)
	if err != nil {
		// sql.ErrNoRows is the legitimate "not enrolled" signal — every
		// fresh operator hits it on the first login. Anything else is
		// infrastructure: refuse this login rather than fail-open with
		// false. errors.Is walks through DatabaseError.Unwrap().
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		s.Logger.Warn("operator mfa enrollment lookup failed; refusing login",
			slog.Int64("operator_id", operatorID),
			slog.String("error", err.Error()))
		return false, authService.ErrMFAStatusUnavailable
	}
	return cred != nil && cred.ID > 0, nil
}

// ===== Challenge / verify =====

func (s *operatorMFAService) StartChallenge(ctx context.Context, operatorID int64, ip net.IP) (string, error) {
	op, err := s.Repos.Operator.FindByID(ctx, operatorID)
	if err != nil || op == nil {
		return "", fmt.Errorf("look up operator: %w", err)
	}
	if s.isMFALocked(op, time.Now()) {
		return "", ErrOperatorMFALocked
	}

	since := time.Now().Add(-OperatorMFARateLimitWindow)
	count, err := s.Repos.OperatorMFAEmailChallenge.CountRecentByOperatorID(ctx, operatorID, since)
	if err == nil && count >= OperatorMFARateLimitMaxSent {
		return "", ErrOperatorMFARateLimited
	}

	plainCode, err := authService.GenerateEmailCode()
	if err != nil {
		return "", fmt.Errorf("generate email code: %w", err)
	}
	codeHash, err := authService.HashShortCode(plainCode)
	if err != nil {
		return "", fmt.Errorf("hash email code: %w", err)
	}

	challenge := &platform.OperatorMFAEmailChallenge{
		OperatorID: operatorID,
		CodeHash:   codeHash,
		ExpiresAt:  time.Now().Add(OperatorMFAChallengeTTL),
		IPAddress:  ip,
	}
	if err := s.Repos.OperatorMFAEmailChallenge.Create(ctx, challenge); err != nil {
		return "", fmt.Errorf("persist email challenge: %w", err)
	}

	s.dispatchChallengeEmail(ctx, op, plainCode, ip)
	s.recordAudit(ctx, operatorID, platform.ActionMFAEmailSent, ip, &challenge.ID, nil)

	tokenString, err := s.TokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
		AccountID: operatorID, // operator id reuses the AccountID slot, scope distinguishes
		Scope:     authjwt.MFAChallengeScopePlatform,
	}, OperatorMFAChallengeTTL)
	if err != nil {
		return "", fmt.Errorf("mint operator challenge jwt: %w", err)
	}
	return tokenString, nil
}

func (s *operatorMFAService) VerifyChallenge(ctx context.Context, challengeToken, code string) (*OperatorVerifiedChallenge, error) {
	claims, err := s.parseChallengeToken(challengeToken)
	if err != nil {
		return nil, ErrOperatorMFAChallengeTokenInvalid
	}
	if claims.Scope != authjwt.MFAChallengeScopePlatform {
		return nil, ErrOperatorMFAChallengeTokenInvalid
	}

	op, err := s.Repos.Operator.FindByID(ctx, claims.AccountID)
	if err != nil || op == nil {
		return nil, ErrOperatorMFAChallengeTokenInvalid
	}
	if s.isMFALocked(op, time.Now()) {
		return nil, ErrOperatorMFALocked
	}

	active, err := s.Repos.OperatorMFAEmailChallenge.FindActiveByOperatorID(ctx, op.ID)
	if err != nil || active == nil {
		s.recordAudit(ctx, op.ID, platform.ActionMFAFailed, nil, nil, map[string]any{"reason": "no active challenge"})
		return nil, ErrOperatorMFACodeInvalid
	}

	ok, vErr := authService.VerifyShortCode(code, active.CodeHash)
	if vErr != nil || !ok {
		s.handleFailedAttempt(ctx, op)
		s.recordAudit(ctx, op.ID, platform.ActionMFAFailed, nil, &active.ID, map[string]any{"reason": "code mismatch"})
		return nil, ErrOperatorMFACodeInvalid
	}

	// Single-use enforcement — mirrors the tenant-side fix. MarkConsumed
	// returns a DatabaseError when 0 rows are affected (another concurrent
	// verify already consumed this code). The previous code logged-and-
	// continued, which let two racing requests both mint a session from a
	// single-use code. Refuse the loser instead.
	now := time.Now()
	if err := s.Repos.OperatorMFAEmailChallenge.MarkConsumed(ctx, active.ID, now); err != nil {
		s.Logger.Warn("failed to mark operator challenge consumed; refusing verify",
			slog.Int64("operator_id", op.ID),
			slog.Int64("challenge_id", active.ID),
			slog.String("error", err.Error()))
		s.recordAudit(ctx, op.ID, platform.ActionMFAFailed, nil, &active.ID, map[string]any{"reason": "consume race"})
		return nil, ErrOperatorMFACodeInvalid
	}
	// Atomic reset — single UPDATE so a concurrent failed verify's
	// increment can't be silently overwritten by a stale full-row Update.
	if err := s.Repos.Operator.ResetMFAAttempts(ctx, op.ID); err != nil {
		s.Logger.Warn("failed to reset operator MFA attempts", slog.String("error", err.Error()))
	}
	op.MFAAttempts = 0
	op.MFALockedUntil = nil
	cred, _ := s.Repos.OperatorMFACredential.FindByOperatorID(ctx, op.ID)
	if cred != nil && cred.ID > 0 {
		_ = s.Repos.OperatorMFACredential.UpdateLastUsedAt(ctx, cred.ID, now)
	}
	s.recordAudit(ctx, op.ID, platform.ActionMFAVerified, nil, &active.ID, nil)
	return &OperatorVerifiedChallenge{OperatorID: op.ID}, nil
}

func (s *operatorMFAService) ResendChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	claims, err := s.parseChallengeToken(challengeToken)
	if err != nil {
		return "", ErrOperatorMFAChallengeTokenInvalid
	}
	if claims.Scope != authjwt.MFAChallengeScopePlatform {
		return "", ErrOperatorMFAChallengeTokenInvalid
	}
	renewed, err := s.StartChallenge(ctx, claims.AccountID, ip)
	if err != nil {
		return "", err
	}
	return renewed, nil
}

func (s *operatorMFAService) VerifyCodeForOperator(ctx context.Context, operatorID int64, code string) error {
	op, err := s.Repos.Operator.FindByID(ctx, operatorID)
	if err != nil || op == nil {
		return ErrOperatorMFACodeInvalid
	}
	if s.isMFALocked(op, time.Now()) {
		return ErrOperatorMFALocked
	}
	active, err := s.Repos.OperatorMFAEmailChallenge.FindActiveByOperatorID(ctx, operatorID)
	if err != nil || active == nil {
		s.recordAudit(ctx, operatorID, platform.ActionMFAFailed, nil, nil, map[string]any{"reason": "no active challenge"})
		return ErrOperatorMFACodeInvalid
	}
	ok, vErr := authService.VerifyShortCode(code, active.CodeHash)
	if vErr != nil || !ok {
		s.handleFailedAttempt(ctx, op)
		s.recordAudit(ctx, operatorID, platform.ActionMFAFailed, nil, &active.ID, map[string]any{"reason": "code mismatch"})
		return ErrOperatorMFACodeInvalid
	}
	now := time.Now()
	if err := s.Repos.OperatorMFAEmailChallenge.MarkConsumed(ctx, active.ID, now); err != nil {
		// Same race-loser refusal as VerifyChallenge (operator scope).
		s.Logger.Warn("failed to mark operator challenge consumed; refusing verify",
			slog.Int64("operator_id", operatorID),
			slog.Int64("challenge_id", active.ID),
			slog.String("error", err.Error()))
		s.recordAudit(ctx, operatorID, platform.ActionMFAFailed, nil, &active.ID, map[string]any{"reason": "consume race"})
		return ErrOperatorMFACodeInvalid
	}
	// Atomic reset matches VerifyChallenge — single UPDATE so a concurrent
	// failed verify's increment isn't silently overwritten.
	_ = s.Repos.Operator.ResetMFAAttempts(ctx, operatorID)
	op.MFAAttempts = 0
	op.MFALockedUntil = nil
	s.recordAudit(ctx, operatorID, platform.ActionMFAVerified, nil, &active.ID, nil)
	return nil
}

// isMFALocked reports whether the operator is inside its MFA-failure cooldown
// window relative to now. The decision lives in the service (clock injected),
// not on the model (Rule 12); the operator row only holds mfa_locked_until.
func (s *operatorMFAService) isMFALocked(op *platform.Operator, now time.Time) bool {
	return op != nil && op.MFALockedUntil != nil && now.Before(*op.MFALockedUntil)
}

// handleFailedAttempt atomically bumps the operator's lockout counter via
// the repository and emits an mfa_locked audit entry when *this* increment
// crossed the threshold. Mirrors the tenant-side fix from #1430 review
// item #6 — the previous read-modify-write let two concurrent failed
// verifies collapse into a single counted attempt.
func (s *operatorMFAService) handleFailedAttempt(ctx context.Context, op *platform.Operator) {
	result, err := s.Repos.Operator.IncrementMFAAttempts(ctx, op.ID, OperatorMFALockoutThreshold, OperatorMFALockoutDuration)
	if err != nil {
		s.Logger.Warn("failed to persist operator MFA attempt counter", slog.String("error", err.Error()))
		return
	}
	op.MFAAttempts = result.Attempts
	op.MFALockedUntil = result.LockedUntil
	if result.Attempts == OperatorMFALockoutThreshold {
		s.recordAudit(ctx, op.ID, platform.ActionMFALocked, nil, nil, map[string]any{"locked_until": result.LockedUntil})
	}
}

// ===== Enrollment =====

func (s *operatorMFAService) Enroll(ctx context.Context, operatorID int64) error {
	existing, _ := s.Repos.OperatorMFACredential.FindByOperatorID(ctx, operatorID)
	if existing != nil && existing.ID > 0 {
		return ErrOperatorMFAAlreadyEnrolled
	}
	cred := &platform.OperatorMFACredential{
		OperatorID: operatorID,
		Method:     platform.OperatorMFAMethodEmail,
		EnrolledAt: time.Now(),
	}
	if err := s.Repos.OperatorMFACredential.Create(ctx, cred); err != nil {
		return fmt.Errorf("persist operator mfa credential: %w", err)
	}
	s.recordAudit(ctx, operatorID, platform.ActionMFAEnrolled, nil, nil, nil)
	return nil
}

// Disable wipes the operator's MFA enrollment + revokes every trusted
// device + resets the lockout counter. The three writes happen in a
// single transaction so partial failure can't leave the operator in a
// half-disabled state (e.g. credential gone but trusted-device cookies
// still verifying, or attempts counter still stuck at the lockout
// threshold). Mirrors the tenant-side mfaService.Disable cascade.
// (#1430 review item #7)
func (s *operatorMFAService) Disable(ctx context.Context, operatorID int64) error {
	err := tenant.WithAdminTx(ctx, s.DB, func(txCtx context.Context, _ bun.Tx) error {
		if err := s.Repos.OperatorMFACredential.DeleteByOperatorID(txCtx, operatorID); err != nil {
			return fmt.Errorf("delete operator credential: %w", err)
		}
		if err := s.Repos.OperatorMFATrustedDevice.RevokeAllByOperatorID(txCtx, operatorID, time.Now()); err != nil {
			return fmt.Errorf("revoke operator trusted devices: %w", err)
		}
		// Atomic reset — replaces the previous fetch + full-row Update with a
		// single UPDATE so the disable cascade isn't racing concurrent failed
		// verifies on the same operator.
		if err := s.Repos.Operator.ResetMFAAttempts(txCtx, operatorID); err != nil {
			return fmt.Errorf("reset operator mfa attempts: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.recordAudit(ctx, operatorID, platform.ActionMFADisabled, nil, nil, nil)
	return nil
}

// ===== Trusted device =====

func (s *operatorMFAService) IssueTrustedDevice(ctx context.Context, operatorID int64, userAgent string, ip net.IP) (string, time.Time, error) {
	expiresAt := time.Now().Add(OperatorMFATrustedDeviceDuration)
	rawToken, err := authService.GenerateTrustedDeviceToken()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate operator trusted-device token: %w", err)
	}
	device := &platform.OperatorMFATrustedDevice{
		OperatorID: operatorID,
		TokenHash:  authService.HashTrustedDeviceToken(rawToken),
		IPAddress:  ip,
		ExpiresAt:  expiresAt,
	}
	if userAgent != "" {
		ua := userAgent
		device.UserAgent = &ua
	}
	if err := s.Repos.OperatorMFATrustedDevice.Create(ctx, device); err != nil {
		return "", time.Time{}, fmt.Errorf("persist operator trusted device: %w", err)
	}
	signed := authService.SignTrustedDeviceToken(rawToken, s.mfaSecret)
	s.recordAudit(ctx, operatorID, platform.ActionMFATrustedDeviceAdded, ip, &device.ID, nil)
	// Fire-and-forget notification mail — mirrors the tenant flow so an
	// operator gets the same "new device added" signal that tenant users do.
	s.dispatchTrustedDeviceAddedEmail(ctx, operatorID, userAgent, ip, int(OperatorMFATrustedDeviceDuration.Hours()/24))
	return signed, expiresAt, nil
}

func (s *operatorMFAService) VerifyTrustedDevice(ctx context.Context, operatorID int64, signedCookie string) (bool, error) {
	rawToken, ok := authService.VerifyTrustedDeviceToken(signedCookie, s.mfaSecret)
	if !ok {
		return false, nil
	}
	tokenHash := authService.HashTrustedDeviceToken(rawToken)
	device, err := s.Repos.OperatorMFATrustedDevice.FindActiveByOperatorIDAndTokenHash(ctx, operatorID, tokenHash)
	if err != nil || device == nil {
		return false, nil
	}
	_ = s.Repos.OperatorMFATrustedDevice.UpdateLastUsedAt(ctx, device.ID, time.Now())
	return true, nil
}

func (s *operatorMFAService) ListTrustedDevices(ctx context.Context, operatorID int64) ([]*platform.OperatorMFATrustedDevice, error) {
	return s.Repos.OperatorMFATrustedDevice.ListActiveByOperatorID(ctx, operatorID)
}

func (s *operatorMFAService) RevokeTrustedDevice(ctx context.Context, operatorID, deviceID int64) error {
	// Validate ownership before revoke — a device row can only be revoked
	// by its own operator account.
	devices, err := s.Repos.OperatorMFATrustedDevice.ListActiveByOperatorID(ctx, operatorID)
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
		return ErrOperatorMFAPermissionDenied
	}
	return s.Repos.OperatorMFATrustedDevice.Revoke(ctx, deviceID, time.Now())
}

// ===== Internal helpers =====

func (s *operatorMFAService) parseChallengeToken(tokenString string) (*authjwt.MFAChallengeClaims, error) {
	return s.TokenAuth.ParseMFAChallengeJWT(tokenString)
}

func (s *operatorMFAService) dispatchChallengeEmail(ctx context.Context, op *platform.Operator, plainCode string, ip net.IP) {
	if s.Dispatcher == nil {
		s.Logger.Warn("email dispatcher unavailable; operator mfa code not sent",
			slog.Int64("operator_id", op.ID))
		return
	}
	frontendURL := strings.TrimRight(s.FrontendURL, "/")
	logoURL := fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", frontendURL)

	requestIP := ""
	if ip != nil && !ip.IsUnspecified() {
		requestIP = ip.String()
	}

	message := email.Message{
		From:     s.DefaultFrom,
		To:       email.NewEmail(op.DisplayName, op.Email),
		Subject:  "Ihr moto-Anmeldecode",
		Template: "mfa-email-code.html",
		Content: map[string]any{
			"LogoURL":              logoURL,
			"Code":                 plainCode,
			"ExpiryMinutes":        int(OperatorMFAChallengeTTL.Minutes()),
			"RequestIP":            requestIP,
			"TrustedDeviceEnabled": true, // operators always have it on
			"TrustedDeviceDays":    int(OperatorMFATrustedDeviceDuration.Hours() / 24),
		},
	}
	meta := email.DeliveryMetadata{
		Type:        "operator_mfa_email_code",
		ReferenceID: op.ID,
		Recipient:   op.Email,
	}
	s.Dispatcher.Dispatch(ctx, email.DeliveryRequest{
		Message:       message,
		Metadata:      meta,
		BackoffPolicy: []time.Duration{time.Second, 5 * time.Second, 15 * time.Second},
		MaxAttempts:   3,
	})
}

// dispatchTrustedDeviceAddedEmail mirrors the tenant-side notification so
// an operator sees the same "new device was just added" mail after the
// remember-device cookie is issued. Fire-and-forget.
func (s *operatorMFAService) dispatchTrustedDeviceAddedEmail(ctx context.Context, operatorID int64, userAgent string, ip net.IP, days int) {
	if s.Dispatcher == nil {
		s.Logger.Warn("email dispatcher unavailable; operator trusted-device-added mail skipped",
			slog.Int64("operator_id", operatorID))
		return
	}
	op, err := s.Repos.Operator.FindByID(ctx, operatorID)
	if err != nil || op == nil || strings.TrimSpace(op.Email) == "" {
		s.Logger.Warn("could not load operator for trusted-device-added mail",
			slog.Int64("operator_id", operatorID),
			slog.Any("error", err))
		return
	}

	frontendURL := strings.TrimRight(s.FrontendURL, "/")
	logoURL := fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", frontendURL)

	requestIP := ""
	if ip != nil && !ip.IsUnspecified() {
		requestIP = ip.String()
	}
	deviceLabel := authService.ShortenUserAgent(userAgent)
	addedAt := time.Now().Format("02.01.2006 15:04")

	// Operators see the Sicherheit section by default — the template
	// text still works for them. We don't pass a deep-link any more
	// (tenant-side decision, mirrored here for template parity).
	message := email.Message{
		From:     s.DefaultFrom,
		To:       email.NewEmail(op.DisplayName, op.Email),
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
		Type:        "operator_mfa_trusted_device_added",
		ReferenceID: operatorID,
		Recipient:   op.Email,
	}
	s.Dispatcher.Dispatch(ctx, email.DeliveryRequest{
		Message:       message,
		Metadata:      meta,
		BackoffPolicy: []time.Duration{time.Second, 5 * time.Second, 15 * time.Second},
		MaxAttempts:   3,
	})
}

// recordAudit writes a row to platform.operator_audit_log. Async via
// goroutine so the login pipeline doesn't pay for the audit insert.
// Failures are logged but never bubble up — operator audit is best-effort
// by design (matches the auth-side mfa_service.recordAuthEvent shape).
func (s *operatorMFAService) recordAudit(ctx context.Context, operatorID int64, action string, ip net.IP, resourceID *int64, changes map[string]any) {
	entry := &platform.OperatorAuditLog{
		OperatorID:   operatorID,
		Action:       action,
		ResourceType: platform.ResourceOperatorMFA,
		ResourceID:   resourceID,
		RequestIP:    ip,
		CreatedAt:    time.Now(),
	}
	if changes != nil {
		// Marshal failures fall through with empty Changes — better to lose
		// the metadata than to skip the whole audit row.
		if buf, err := json.Marshal(changes); err == nil {
			entry.Changes = buf
		}
	}

	go func() {
		// Recover from any panic in the audit-write path. Without this
		// guard a nil-pointer in the repo, a bun driver bug, or any
		// other unexpected panic in the goroutine would crash the
		// entire server process. Mirrors the recovery + sentry pattern
		// in mfaService.recordAuthEvent. (#1430 review item #10)
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Errorf("panic in operator mfa audit logging: %v", r)
				s.Logger.Error("operator audit goroutine panic recovered",
					slog.String("action", action),
					slog.String("error", err.Error()),
				)
				sentry.CurrentHub().Recover(r)
				sentry.Flush(2 * time.Second)
			}
		}()
		logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.Repos.OperatorAuditLog.Create(logCtx, entry); err != nil {
			s.Logger.Error("failed to log operator mfa audit event",
				slog.String("action", action),
				slog.String("error", err.Error()),
			)
		}
	}()
	_ = ctx // outer ctx isn't used for the async path, but keeping the param shape consistent
}
