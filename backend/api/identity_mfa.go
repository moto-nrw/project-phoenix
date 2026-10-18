package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// Identity & Access owns the account and operator second factor and both
// portals' passkey ceremonies (#3331). This file binds the seams those flows
// need to the material the root still holds: the school
// settings that parameterise the gate, the challenge JWT codec, the Argon2id
// code hasher, the two mails and the operator action log.

// newMFADependencies binds the second factor and the passkey ceremonies to
// the challenge codec, the school settings, the mails and the operator
// ledger. passkeys carries the relying-party facts; nil leaves the
// ceremonies unavailable.
func newMFADependencies(codec identityPorts.MFAChallengeCodec, settings settingsCompose.SettingsService, mail mfaMailer, operatorAuditLog operatorAuditEntries, jwtSecret string, passkeys *identityaccessCompose.PasskeyDependencies, logger *slog.Logger) (*identityaccessCompose.MFADependencies, error) {
	if codec == nil || operatorAuditLog == nil || mail.logger == nil || logger == nil {
		return nil, errors.New("identity access composition: the second factor needs the challenge codec, the operator audit log and a logger")
	}
	return &identityaccessCompose.MFADependencies{
		Settings:      mfaSettingsFor(settings),
		Codec:         codec,
		Codes:         shortCodeHasher{},
		Mail:          mail,
		OperatorAudit: operatorActionLog{ledger: operatorAuditLog, logger: logger},
		JWTSecret:     jwtSecret,
		Passkeys:      passkeys,
		Logger:        logger,
	}, nil
}

// --- the school settings -----------------------------------------------------

// mfaSettingsFor binds the gate's parameters to the settings service; a root
// composed without one gets Identity & Access's answers for that case.
func mfaSettingsFor(settings settingsCompose.SettingsService) identityaccessCompose.MFASettings {
	if settings == nil {
		return identityaccess.MFASettingsWithoutService{}
	}
	return mfaSettings{settings: settings}
}

// mfaSettings resolves the gate's parameters from the school settings.
type mfaSettings struct {
	settings settingsCompose.SettingsService
}

func (s mfaSettings) MFAMode(ctx context.Context, tenantID int64) (string, error) {
	if tenantID > 0 {
		return s.settings.ResolveStringForTenant(ctx, tenantID, configModel.KeyMFAMode)
	}
	return s.settings.ResolveString(ctx, configModel.KeyMFAMode)
}

func (s mfaSettings) MFAModeInTx(ctx context.Context, tenantID int64) (string, error) {
	if tenantID > 0 {
		return s.settings.ResolveStringForTenantInTx(ctx, tenantID, configModel.KeyMFAMode)
	}
	return s.settings.ResolveString(ctx, configModel.KeyMFAMode)
}

func (s mfaSettings) TrustedDeviceEnabled(ctx context.Context, tenantID int64) (bool, error) {
	if tenantID > 0 {
		return s.settings.ResolveBoolForTenant(ctx, tenantID, configModel.KeyMFATrustedDeviceEnabled)
	}
	return s.settings.ResolveBool(ctx, configModel.KeyMFATrustedDeviceEnabled)
}

func (s mfaSettings) TrustedDeviceDays(ctx context.Context, tenantID int64) (int, error) {
	if tenantID > 0 {
		return s.settings.ResolveIntForTenant(ctx, tenantID, configModel.KeyMFATrustedDeviceDays)
	}
	return s.settings.ResolveInt(ctx, configModel.KeyMFATrustedDeviceDays)
}

func (s mfaSettings) LockoutThreshold(ctx context.Context, tenantID int64) (int, error) {
	if tenantID > 0 {
		return s.settings.ResolveIntForTenant(ctx, tenantID, configModel.KeyAccountLockoutThreshold)
	}
	return s.settings.ResolveInt(ctx, configModel.KeyAccountLockoutThreshold)
}

func (s mfaSettings) LockoutDuration(ctx context.Context, tenantID int64) (time.Duration, error) {
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

// shortCodeHasher routes the e-mail codes through the project-wide Argon2id
// helper, so tuning its parameters reaches MFA codes too.
type shortCodeHasher struct{}

func (shortCodeHasher) HashShortCode(plain string) (string, error) {
	return securityruntime.HashPassword(plain)
}

func (shortCodeHasher) VerifyShortCode(plain, encodedHash string) (bool, error) {
	return securityruntime.VerifyPassword(plain, encodedHash)
}

// --- the two mails -----------------------------------------------------------

// mfaMailer sends the code through synchronous delivery and the
// device-added notice asynchronously. A nil backoff uses the production
// spacing of the send retries.
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
	err := m.dispatcher.Deliver(ctx, email.WithInteractiveRetry(email.DeliveryRequest{
		Message: *identityMessage(m.from, email.NewEmail(message.RecipientName, message.Recipient),
			identityaccess.MFACodeMailContent(message, m.frontendURL)),
		Metadata: email.DeliveryMetadata{
			Type: deliveryType, ReferenceID: message.ReferenceID, Recipient: message.Recipient,
		},
	}, m.backoff))
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
	m.dispatcher.Dispatch(ctx, email.WithInteractiveRetry(email.DeliveryRequest{
		Message: *identityMessage(m.from, email.NewEmail(message.RecipientName, message.Recipient),
			identityaccess.TrustedDeviceAddedMailContent(message, m.frontendURL)),
		Metadata: email.DeliveryMetadata{
			Type: deliveryType, ReferenceID: message.ReferenceID, Recipient: message.Recipient,
		},
	}, m.backoff))
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
	operatorAuditWriter = operatorAuditEntries
	operatorAuditRow    = auditModels.OperatorAuditEntry
	operatorAuditEntry  = identityaccess.OperatorAuditEntry
)

func (l operatorActionLog) RecordOperatorActionAsync(entry identityaccess.OperatorAuditEntry) {
	if l.ledger == nil {
		return
	}
	row := &auditModels.OperatorAuditEntry{
		OperatorID: entry.OperatorID, Action: entry.Action, ResourceType: entry.ResourceType,
		ResourceID: entry.ResourceID, RequestIP: securityruntime.ParseClientAddress(entry.IPAddress), CreatedAt: time.Now(),
	}
	// A marshal failure falls through with empty changes: losing the
	// metadata beats losing the whole audit row.
	if changes, err := entry.MFAAuditChanges(); err != nil {
		l.logger.Warn("failed to encode operator mfa audit changes",
			slog.String("action", entry.Action),
			slog.String("error", err.Error()))
	} else {
		row.Changes = changes
	}
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				l.logger.Error("operator audit goroutine panic recovered",
					slog.String("action", entry.Action),
					slog.String("error", fmt.Sprintf("panic in operator mfa audit logging: %v", recovered)))
				sentry.WithScope(func(scope *sentry.Scope) {
					scope.SetTag("job", "operator-audit-log")
					sentry.CurrentHub().Recover(recovered)
				})
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
