package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/config"
)

// Identity & Access owns the account and operator second factor and both
// portals' passkey ceremonies (#3331). This file binds the seams those flows
// need to the material the root still holds: the account MFA rows in the
// retained repositories (they move under the module with #3226), the school
// settings that parameterise the gate, the challenge JWT codec, the Argon2id
// code hasher, the two mails and the operator action log.

// mfaEmailBackoff spaces the retries of the MFA mails.
var mfaEmailBackoff = []time.Duration{
	time.Second,
	5 * time.Second,
	15 * time.Second,
}

// mfaWiring is the retained material the MFA seams bind to. A nil wiring
// composes the module without MFA, which the cleanup and fixture roots do.
type mfaWiring struct {
	repos       *repositories.Factory
	settings    config.SettingsService
	tokenAuth   *authjwt.TokenAuth
	dispatcher  *email.Dispatcher
	defaultFrom email.Email
	frontendURL string
	jwtSecret   string
	logger      *slog.Logger
	// backoff spaces the send retries; nil uses the production spacing.
	backoff []time.Duration
	// decorate wraps the record port before the module is composed, so a
	// behaviour test can make one statement fail and prove the flow refuses
	// rather than degrades. Nil in production.
	decorate func(identityaccessCompose.AccountMFARecords) identityaccessCompose.AccountMFARecords
	// capability serves the account second factor instead of composing it
	// over the records. Nil in production.
	capability identityaccess.AccountMFA
	// passkeys carries the relying-party facts; nil leaves the ceremonies
	// unavailable.
	passkeys *identityaccessCompose.PasskeyDependencies
}

func mfaDependencies(wiring *mfaWiring) *identityaccessCompose.MFADependencies {
	if wiring == nil || wiring.repos == nil || wiring.tokenAuth == nil {
		return nil
	}
	logger := wiring.logger
	if logger == nil {
		logger = slog.Default()
	}
	backoff := wiring.backoff
	if backoff == nil {
		backoff = mfaEmailBackoff
	}
	records := repositories.NewAccountMFARecords(wiring.repos)
	if wiring.decorate != nil {
		records = wiring.decorate(records)
	}
	return &identityaccessCompose.MFADependencies{
		Records:  records,
		Settings: mfaSettings{settings: wiring.settings},
		Codec:    mfaChallengeCodec{tokenAuth: wiring.tokenAuth},
		Codes:    shortCodeHasher{},
		Mail: mfaMailer{
			dispatcher: wiring.dispatcher, from: wiring.defaultFrom,
			frontendURL: wiring.frontendURL, logger: logger, backoff: backoff,
		},
		OperatorAudit: operatorActionLog{ledger: wiring.repos.OperatorAuditLog, logger: logger},
		JWTSecret:     wiring.jwtSecret,
		Passkeys:      wiring.passkeys,
		Capability:    wiring.capability,
		Logger:        logger,
	}
}

// --- the school settings -----------------------------------------------------

// mfaSettings resolves the gate's parameters. A root composed without a
// settings service answers the registry defaults, which is what the retained
// service did with a nil one.
type mfaSettings struct{ settings config.SettingsService }

func (s mfaSettings) MFAMode(ctx context.Context, tenantID int64) (string, error) {
	if s.settings == nil {
		return configModel.MFAModeOff, nil
	}
	if tenantID > 0 {
		return s.settings.ResolveStringForTenant(ctx, tenantID, configModel.KeyMFAMode)
	}
	return s.settings.ResolveString(ctx, configModel.KeyMFAMode)
}

func (s mfaSettings) MFAModeInTx(ctx context.Context, tenantID int64) (string, error) {
	if s.settings == nil {
		return configModel.MFAModeOff, nil
	}
	if tenantID > 0 {
		return s.settings.ResolveStringForTenantInTx(ctx, tenantID, configModel.KeyMFAMode)
	}
	return s.settings.ResolveString(ctx, configModel.KeyMFAMode)
}

func (s mfaSettings) TrustedDeviceEnabled(ctx context.Context, tenantID int64) (bool, error) {
	if s.settings == nil {
		return true, nil
	}
	if tenantID > 0 {
		return s.settings.ResolveBoolForTenant(ctx, tenantID, configModel.KeyMFATrustedDeviceEnabled)
	}
	return s.settings.ResolveBool(ctx, configModel.KeyMFATrustedDeviceEnabled)
}

func (s mfaSettings) TrustedDeviceDays(ctx context.Context, tenantID int64) (int, error) {
	if s.settings == nil {
		return 0, nil
	}
	if tenantID > 0 {
		return s.settings.ResolveIntForTenant(ctx, tenantID, configModel.KeyMFATrustedDeviceDays)
	}
	return s.settings.ResolveInt(ctx, configModel.KeyMFATrustedDeviceDays)
}

func (s mfaSettings) LockoutThreshold(ctx context.Context, tenantID int64) (int, error) {
	if s.settings == nil {
		return 0, nil
	}
	if tenantID > 0 {
		return s.settings.ResolveIntForTenant(ctx, tenantID, configModel.KeyAccountLockoutThreshold)
	}
	return s.settings.ResolveInt(ctx, configModel.KeyAccountLockoutThreshold)
}

func (s mfaSettings) LockoutDuration(ctx context.Context, tenantID int64) (time.Duration, error) {
	if s.settings == nil {
		return 0, nil
	}
	var (
		minutes int
		err     error
	)
	if tenantID > 0 {
		minutes, err = s.settings.ResolveIntForTenant(ctx, tenantID, configModel.KeyAccountLockoutDurationMinutes)
	} else {
		minutes, err = s.settings.ResolveInt(ctx, configModel.KeyAccountLockoutDurationMinutes)
	}
	if err != nil {
		return 0, err
	}
	return time.Duration(minutes) * time.Minute, nil
}

// --- the challenge codec and the code hasher ---------------------------------

type mfaChallengeCodec struct{ tokenAuth *authjwt.TokenAuth }

func (c mfaChallengeCodec) IssueChallengeToken(claims identityaccess.MFAChallengeClaims, ttl time.Duration) (string, error) {
	return c.tokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
		AccountID: claims.AccountID, Scope: claims.Scope,
		TenantID: claims.TenantID, ChallengeID: claims.ChallengeID,
	}, ttl)
}

func (c mfaChallengeCodec) ParseChallengeToken(token string) (identityaccess.MFAChallengeClaims, error) {
	claims, err := c.tokenAuth.ParseMFAChallengeJWT(token)
	if err != nil {
		return identityaccess.MFAChallengeClaims{}, err
	}
	return identityaccess.MFAChallengeClaims{
		AccountID: claims.AccountID, Scope: claims.Scope,
		TenantID: claims.TenantID, ChallengeID: claims.ChallengeID,
	}, nil
}

// shortCodeHasher routes the e-mail codes through the project-wide Argon2id
// helper, so tuning its parameters reaches MFA codes too.
type shortCodeHasher struct{}

func (shortCodeHasher) HashShortCode(plain string) (string, error) {
	return auth.HashPassword(plain)
}

func (shortCodeHasher) VerifyShortCode(plain, encodedHash string) (bool, error) {
	return auth.VerifyPassword(plain, encodedHash)
}

// --- the two mails -----------------------------------------------------------

// mfaMailer sends the code through synchronous delivery and the
// device-added notice asynchronously. The code template carries no link or
// button on purpose: the user pastes the code into the moto login, which
// kills the "click here to confirm" phishing pattern.
type mfaMailer struct {
	dispatcher  *email.Dispatcher
	from        email.Email
	frontendURL string
	logger      *slog.Logger
	backoff     []time.Duration
}

func (m mfaMailer) DeliverCode(ctx context.Context, message identityaccess.MFACodeMail) error {
	if m.dispatcher == nil {
		return email.ErrDeliveryUnavailable
	}
	deliveryType := "mfa_email_code"
	if message.Operator {
		deliveryType = "operator_mfa_email_code"
	}
	err := m.dispatcher.Deliver(ctx, email.DeliveryRequest{
		Message: email.Message{
			From:     m.from,
			To:       email.NewEmail(message.RecipientName, message.Recipient),
			Subject:  "Ihr moto-Anmeldecode",
			Template: "mfa-email-code.html",
			Content: map[string]any{
				"LogoURL":              motoLogoURL(m.frontendURL),
				"Code":                 message.Code,
				"ExpiryMinutes":        message.ExpiryMinutes,
				"RequestIP":            message.RequestIP,
				"TrustedDeviceEnabled": message.TrustedDeviceEnabled,
				"TrustedDeviceDays":    message.TrustedDeviceDays,
			},
		},
		Metadata: email.DeliveryMetadata{
			Type: deliveryType, ReferenceID: message.ReferenceID, Recipient: message.Recipient,
		},
		BackoffPolicy: m.backoff,
		MaxAttempts:   3,
	})
	if err != nil {
		m.logger.Warn("mfa challenge delivery failed",
			slog.Int64("reference_id", message.ReferenceID),
			slog.Bool("operator", message.Operator),
			slog.String("error", err.Error()))
		return err
	}
	return nil
}

func (m mfaMailer) NotifyTrustedDeviceAdded(ctx context.Context, message identityaccess.TrustedDeviceMail) {
	if m.dispatcher == nil {
		m.logger.Warn("email dispatcher unavailable; trusted-device-added mail skipped",
			slog.Int64("reference_id", message.ReferenceID))
		return
	}
	deliveryType := "mfa_trusted_device_added"
	if message.Operator {
		deliveryType = "operator_mfa_trusted_device_added"
	}
	m.dispatcher.Dispatch(ctx, email.DeliveryRequest{
		Message: email.Message{
			From:     m.from,
			To:       email.NewEmail(message.RecipientName, message.Recipient),
			Subject:  "Neues vertrautes Gerät zu Ihrem moto-Konto hinzugefügt",
			Template: "trusted-device-added.html",
			Content: map[string]any{
				"LogoURL":     motoLogoURL(m.frontendURL),
				"DeviceLabel": message.DeviceLabel,
				"IPAddress":   message.RequestIP,
				"AddedAt":     message.AddedAt,
				"TrustedDays": message.TrustedDays,
			},
		},
		Metadata: email.DeliveryMetadata{
			Type: deliveryType, ReferenceID: message.ReferenceID, Recipient: message.Recipient,
		},
		BackoffPolicy: m.backoff,
		MaxAttempts:   3,
	})
}

// --- the operator action log -------------------------------------------------

// operatorActionLog appends an operator MFA entry on a detached context so
// the login pipeline never pays for the insert, and recovers from a panic in
// the write path: without the guard a nil pointer, a driver bug or any other
// surprise in the goroutine would take the whole server down.
type operatorActionLog struct {
	ledger operatorAuditWriter
	logger *slog.Logger
}

// The operator ledger's shapes, named here so the composition's own tests can
// drive the seam without reaching past it into the owner's rows.
type (
	operatorAuditWriter = platformModels.OperatorAuditLogRepository
	operatorAuditRow    = platformModels.OperatorAuditLog
	operatorAuditEntry  = identityaccess.OperatorAuditEntry
)

func (l operatorActionLog) RecordOperatorActionAsync(entry identityaccess.OperatorAuditEntry) {
	if l.ledger == nil {
		return
	}
	row := &platformModels.OperatorAuditLog{
		OperatorID: entry.OperatorID, Action: entry.Action, ResourceType: entry.ResourceType,
		ResourceID: entry.ResourceID, RequestIP: auth.ParseClientIP(entry.IPAddress), CreatedAt: time.Now(),
	}
	if changes := operatorMFAChanges(entry.MFA); len(changes) > 0 {
		// A marshal failure falls through with empty changes: losing the
		// metadata beats losing the whole audit row.
		if err := row.SetChanges(changes); err != nil {
			l.logger.Warn("failed to encode operator mfa audit changes",
				slog.String("action", entry.Action),
				slog.String("error", err.Error()))
		}
	}
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				l.logger.Error("operator audit goroutine panic recovered",
					slog.String("action", entry.Action),
					slog.String("error", fmt.Sprintf("panic in operator mfa audit logging: %v", recovered)))
				sentry.CurrentHub().Recover(recovered)
				sentry.Flush(2 * time.Second)
			}
		}()
		logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := l.ledger.Create(logCtx, row); err != nil {
			l.logger.Error("failed to log operator mfa audit event",
				slog.String("action", entry.Action),
				slog.String("error", err.Error()))
		}
	}()
}

func operatorMFAChanges(evidence *identityaccess.OperatorMFAEvidence) map[string]any {
	if evidence == nil {
		return nil
	}
	if override := evidence.Override; override != nil {
		return map[string]any{
			"action":            override.Action,
			"scope":             override.Scope,
			"override":          override.Override,
			"previous_override": override.PreviousOverride,
			"reason":            override.Reason,
		}
	}
	switch {
	case evidence.Reason != "":
		return map[string]any{"reason": evidence.Reason}
	case evidence.LockedUntil != nil:
		return map[string]any{"locked_until": evidence.LockedUntil}
	}
	return nil
}

func motoLogoURL(frontendURL string) string {
	return fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", strings.TrimRight(frontendURL, "/"))
}
