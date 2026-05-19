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

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/platform"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/uptrace/bun"
)

// Operator MFA constants. The cooldown / lockout / TTL values are not
// configurable for operators by design — moto-Operators always face the
// same security posture.
const (
	OperatorMFAChallengeTTL          = 10 * time.Minute
	OperatorMFALockoutThreshold      = 5
	OperatorMFALockoutDuration       = 15 * time.Minute
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
	ErrOperatorMFANotEnrolled           = authService.ErrMFANotEnrolled
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
	ResendChallenge(ctx context.Context, challengeToken string, ip net.IP) error
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
	repos       *repositories.Factory
	tokenAuth   *authjwt.TokenAuth
	dispatcher  *email.Dispatcher
	defaultFrom email.Email
	frontendURL string
	mfaSecret   []byte
	db          *bun.DB
	logger      *slog.Logger
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
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &operatorMFAService{
		repos:       cfg.Repos,
		tokenAuth:   cfg.TokenAuth,
		dispatcher:  cfg.Dispatcher,
		defaultFrom: cfg.DefaultFrom,
		frontendURL: cfg.FrontendURL,
		mfaSecret:   authService.DeriveMFASecret(cfg.JWTSecret),
		db:          cfg.DB,
		logger:      logger.With("service", "operator-mfa"),
	}, nil
}

// ===== Inquiry =====

func (s *operatorMFAService) HasEnrollment(ctx context.Context, operatorID int64) (bool, error) {
	cred, err := s.repos.OperatorMFACredential.FindByOperatorID(ctx, operatorID)
	if err != nil {
		// sql.ErrNoRows is the legitimate "not enrolled" signal — every
		// fresh operator hits it on the first login. Anything else is
		// infrastructure: refuse this login rather than fail-open with
		// false. errors.Is walks through DatabaseError.Unwrap().
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		s.logger.Warn("operator mfa enrollment lookup failed; refusing login",
			slog.Int64("operator_id", operatorID),
			slog.String("error", err.Error()))
		return false, authService.ErrMFAStatusUnavailable
	}
	return cred != nil && cred.ID > 0, nil
}

// ===== Challenge / verify =====

func (s *operatorMFAService) StartChallenge(ctx context.Context, operatorID int64, ip net.IP) (string, error) {
	op, err := s.repos.Operator.FindByID(ctx, operatorID)
	if err != nil || op == nil {
		return "", fmt.Errorf("look up operator: %w", err)
	}
	if op.IsMFALocked() {
		return "", ErrOperatorMFALocked
	}

	since := time.Now().Add(-OperatorMFARateLimitWindow)
	count, err := s.repos.OperatorMFAEmailChallenge.CountRecentByOperatorID(ctx, operatorID, since)
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
	if err := s.repos.OperatorMFAEmailChallenge.Create(ctx, challenge); err != nil {
		return "", fmt.Errorf("persist email challenge: %w", err)
	}

	s.dispatchChallengeEmail(ctx, op, plainCode, ip)
	s.recordAudit(ctx, operatorID, platform.ActionMFAEmailSent, ip, &challenge.ID, nil)

	tokenString, err := s.tokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
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

	op, err := s.repos.Operator.FindByID(ctx, claims.AccountID)
	if err != nil || op == nil {
		return nil, ErrOperatorMFAChallengeTokenInvalid
	}
	if op.IsMFALocked() {
		return nil, ErrOperatorMFALocked
	}

	active, err := s.repos.OperatorMFAEmailChallenge.FindActiveByOperatorID(ctx, op.ID)
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
	if err := s.repos.OperatorMFAEmailChallenge.MarkConsumed(ctx, active.ID, now); err != nil {
		s.logger.Warn("failed to mark operator challenge consumed; refusing verify",
			slog.Int64("operator_id", op.ID),
			slog.Int64("challenge_id", active.ID),
			slog.String("error", err.Error()))
		s.recordAudit(ctx, op.ID, platform.ActionMFAFailed, nil, &active.ID, map[string]any{"reason": "consume race"})
		return nil, ErrOperatorMFACodeInvalid
	}
	op.ResetMFAAttempts()
	if err := s.repos.Operator.Update(ctx, op); err != nil {
		s.logger.Warn("failed to reset operator MFA attempts", slog.String("error", err.Error()))
	}
	cred, _ := s.repos.OperatorMFACredential.FindByOperatorID(ctx, op.ID)
	if cred != nil && cred.ID > 0 {
		_ = s.repos.OperatorMFACredential.UpdateLastUsedAt(ctx, cred.ID, now)
	}
	s.recordAudit(ctx, op.ID, platform.ActionMFAVerified, nil, &active.ID, nil)
	return &OperatorVerifiedChallenge{OperatorID: op.ID}, nil
}

func (s *operatorMFAService) ResendChallenge(ctx context.Context, challengeToken string, ip net.IP) error {
	claims, err := s.parseChallengeToken(challengeToken)
	if err != nil {
		return ErrOperatorMFAChallengeTokenInvalid
	}
	if claims.Scope != authjwt.MFAChallengeScopePlatform {
		return ErrOperatorMFAChallengeTokenInvalid
	}
	if _, err := s.StartChallenge(ctx, claims.AccountID, ip); err != nil {
		return err
	}
	return nil
}

func (s *operatorMFAService) VerifyCodeForOperator(ctx context.Context, operatorID int64, code string) error {
	op, err := s.repos.Operator.FindByID(ctx, operatorID)
	if err != nil || op == nil {
		return ErrOperatorMFACodeInvalid
	}
	if op.IsMFALocked() {
		return ErrOperatorMFALocked
	}
	active, err := s.repos.OperatorMFAEmailChallenge.FindActiveByOperatorID(ctx, operatorID)
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
	if err := s.repos.OperatorMFAEmailChallenge.MarkConsumed(ctx, active.ID, now); err != nil {
		// Same race-loser refusal as VerifyChallenge (operator scope).
		s.logger.Warn("failed to mark operator challenge consumed; refusing verify",
			slog.Int64("operator_id", operatorID),
			slog.Int64("challenge_id", active.ID),
			slog.String("error", err.Error()))
		s.recordAudit(ctx, operatorID, platform.ActionMFAFailed, nil, &active.ID, map[string]any{"reason": "consume race"})
		return ErrOperatorMFACodeInvalid
	}
	op.ResetMFAAttempts()
	_ = s.repos.Operator.Update(ctx, op)
	s.recordAudit(ctx, operatorID, platform.ActionMFAVerified, nil, &active.ID, nil)
	return nil
}

func (s *operatorMFAService) handleFailedAttempt(ctx context.Context, op *platform.Operator) {
	wasLocked := op.IsMFALocked()
	op.IncrementMFAAttempts()
	if err := s.repos.Operator.Update(ctx, op); err != nil {
		s.logger.Warn("failed to persist operator MFA attempt counter", slog.String("error", err.Error()))
		return
	}
	if !wasLocked && op.IsMFALocked() {
		s.recordAudit(ctx, op.ID, platform.ActionMFALocked, nil, nil, map[string]any{"locked_until": op.MFALockedUntil})
	}
}

// ===== Enrollment =====

func (s *operatorMFAService) Enroll(ctx context.Context, operatorID int64) error {
	existing, _ := s.repos.OperatorMFACredential.FindByOperatorID(ctx, operatorID)
	if existing != nil && existing.ID > 0 {
		return ErrOperatorMFAAlreadyEnrolled
	}
	cred := &platform.OperatorMFACredential{
		OperatorID: operatorID,
		Method:     platform.OperatorMFAMethodEmail,
		EnrolledAt: time.Now(),
	}
	if err := s.repos.OperatorMFACredential.Create(ctx, cred); err != nil {
		return fmt.Errorf("persist operator mfa credential: %w", err)
	}
	s.recordAudit(ctx, operatorID, platform.ActionMFAEnrolled, nil, nil, nil)
	return nil
}

func (s *operatorMFAService) Disable(ctx context.Context, operatorID int64) error {
	if err := s.repos.OperatorMFACredential.DeleteByOperatorID(ctx, operatorID); err != nil {
		return fmt.Errorf("delete operator credential: %w", err)
	}
	if err := s.repos.OperatorMFATrustedDevice.RevokeAllByOperatorID(ctx, operatorID, time.Now()); err != nil {
		return fmt.Errorf("revoke operator trusted devices: %w", err)
	}
	if op, err := s.repos.Operator.FindByID(ctx, operatorID); err == nil && op != nil {
		op.ResetMFAAttempts()
		_ = s.repos.Operator.Update(ctx, op)
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
	if err := s.repos.OperatorMFATrustedDevice.Create(ctx, device); err != nil {
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
	device, err := s.repos.OperatorMFATrustedDevice.FindActiveByOperatorIDAndTokenHash(ctx, operatorID, tokenHash)
	if err != nil || device == nil {
		return false, nil
	}
	_ = s.repos.OperatorMFATrustedDevice.UpdateLastUsedAt(ctx, device.ID, time.Now())
	return true, nil
}

func (s *operatorMFAService) ListTrustedDevices(ctx context.Context, operatorID int64) ([]*platform.OperatorMFATrustedDevice, error) {
	return s.repos.OperatorMFATrustedDevice.ListActiveByOperatorID(ctx, operatorID)
}

func (s *operatorMFAService) RevokeTrustedDevice(ctx context.Context, operatorID, deviceID int64) error {
	// Validate ownership before revoke — a device row can only be revoked
	// by its own operator account.
	devices, err := s.repos.OperatorMFATrustedDevice.ListActiveByOperatorID(ctx, operatorID)
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
	return s.repos.OperatorMFATrustedDevice.Revoke(ctx, deviceID, time.Now())
}

// ===== Internal helpers =====

func (s *operatorMFAService) parseChallengeToken(tokenString string) (*authjwt.MFAChallengeClaims, error) {
	return s.tokenAuth.ParseMFAChallengeJWT(tokenString)
}

func (s *operatorMFAService) dispatchChallengeEmail(ctx context.Context, op *platform.Operator, plainCode string, ip net.IP) {
	if s.dispatcher == nil {
		s.logger.Warn("email dispatcher unavailable; operator mfa code not sent",
			slog.Int64("operator_id", op.ID))
		return
	}
	frontendURL := strings.TrimRight(s.frontendURL, "/")
	logoURL := fmt.Sprintf("%s/images/moto_transparent.png", frontendURL)

	requestIP := ""
	if ip != nil && !ip.IsUnspecified() {
		requestIP = ip.String()
	}

	message := email.Message{
		From:     s.defaultFrom,
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
	s.dispatcher.Dispatch(ctx, email.DeliveryRequest{
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
	if s.dispatcher == nil {
		s.logger.Warn("email dispatcher unavailable; operator trusted-device-added mail skipped",
			slog.Int64("operator_id", operatorID))
		return
	}
	op, err := s.repos.Operator.FindByID(ctx, operatorID)
	if err != nil || op == nil || strings.TrimSpace(op.Email) == "" {
		s.logger.Warn("could not load operator for trusted-device-added mail",
			slog.Int64("operator_id", operatorID),
			slog.Any("error", err))
		return
	}

	frontendURL := strings.TrimRight(s.frontendURL, "/")
	logoURL := fmt.Sprintf("%s/images/moto_transparent.png", frontendURL)

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
		From:     s.defaultFrom,
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
	s.dispatcher.Dispatch(ctx, email.DeliveryRequest{
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
		logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.repos.OperatorAuditLog.Create(logCtx, entry); err != nil {
			s.logger.Error("failed to log operator mfa audit event",
				slog.String("action", action),
				slog.String("error", err.Error()),
			)
		}
	}()
	_ = ctx // outer ctx isn't used for the async path, but keeping the param shape consistent
}
