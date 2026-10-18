package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	enqueue     func(context.Context, DurableMail) (int64, error)
	deliver     func(context.Context, int64) error
	from        email.Email
	frontendURL string
	logger      *slog.Logger
	backoff     []time.Duration
}

func newMFAMailer(db transactionruntime.Database, runtime *deliveryCompose.Runtime, transport email.Mailer, from email.Email, frontendURL string, logger *slog.Logger, backoff []time.Duration) (mfaMailer, error) {
	mail := mfaMailer{from: from, frontendURL: frontendURL, logger: logger, backoff: backoff}
	if transport == nil {
		return mail, nil
	}
	if runtime == nil {
		return mail, errors.New("mfa mail requires the delivery runtime")
	}
	outbox := OutboxEnqueuer{Runtime: runtime}
	provider := &deliveryCompose.MailPushProvider{
		Registry: deliveryCompose.NewEmailTemplateRegistry(map[string]deliveryCompose.MailRenderer{transactionalMailKind: transactionalMailRenderer{logger: logger}}),
		Mailer:   transport, Logger: logger, Tenants: deliveryCompose.NewTenantTransactions(db.Pool()),
	}
	worker, err := deliveryCompose.NewWorker(runtime, provider)
	if err != nil {
		return mail, err
	}
	mail.enqueue = func(ctx context.Context, message DurableMail) (int64, error) {
		queued, err := outbox.enqueueTransactionalMail(ctx, message)
		return queued.ID, err
	}
	mail.deliver = func(ctx context.Context, id int64) error {
		return worker.DeliverEmail(tenant.WithUnitOfWork(ctx, db.UnitOfWork()), 0, id)
	}
	return mail, nil
}

// EnqueueCode returns a continuation that may run only after its transaction
// commits. A failed synchronous send is terminal, as for the original mailer.
func (m mfaMailer) EnqueueCode(ctx context.Context, message identityaccess.MFACodeMail, challengeID int64, expiresAt time.Time) (func(context.Context) error, error) {
	if m.deliver == nil || m.enqueue == nil {
		return nil, email.ErrDeliveryUnavailable
	}
	deliveryType := "mfa_email_code"
	if message.Operator {
		deliveryType = "operator_mfa_email_code"
	}
	id, err := m.enqueue(ctx, DurableMail{
		IdempotencyKey: fmt.Sprintf("%s:%d", deliveryType, challengeID),
		Message:        *identityMessage(m.from, email.NewEmail(message.RecipientName, message.Recipient), identityaccess.MFACodeMailContent(message, m.frontendURL)),
		Type:           deliveryType, ReferenceID: message.ReferenceID, ExpiresAt: expiresAt, Backoff: m.backoff,
	})
	if err != nil {
		return nil, err
	}
	return func(sendCtx context.Context) error {
		err := m.deliver(sendCtx, id)
		if err != nil {
			m.logger.Warn("mfa challenge delivery failed",
				slog.Int64("reference_id", message.ReferenceID),
				slog.Bool("operator", message.Operator),
				slog.String("error", err.Error()),
			)
		}
		return err
	}, nil
}

func (m mfaMailer) NotifyTrustedDeviceAdded(ctx context.Context, message identityaccess.TrustedDeviceMail, deviceID int64) error {
	if m.enqueue == nil {
		return email.ErrDeliveryUnavailable
	}
	deliveryType := "mfa_trusted_device_added"
	if message.Operator {
		deliveryType = "operator_mfa_trusted_device_added"
	}
	_, err := m.enqueue(ctx, DurableMail{
		IdempotencyKey: fmt.Sprintf("%s:%d", deliveryType, deviceID),
		Message:        *identityMessage(m.from, email.NewEmail(message.RecipientName, message.Recipient), identityaccess.TrustedDeviceAddedMailContent(message, m.frontendURL)),
		Type:           deliveryType, ReferenceID: message.ReferenceID, Backoff: m.backoff,
	})
	return err
}

// --- the operator action log -------------------------------------------------

// operatorActionLog appends before returning, joining the triggering transaction.
// A driver panic remains contained, as for the former detached writer.
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

func (l operatorActionLog) RecordOperatorAction(ctx context.Context, entry identityaccess.OperatorAuditEntry) (err error) {
	if l.ledger == nil {
		return errors.New("operator audit ledger is required")
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic in operator mfa audit logging: %v", recovered)
			l.logger.Error("operator audit write panic recovered",
				slog.String("action", entry.Action),
				slog.String("error", err.Error()),
			)
			sentry.WithScope(func(scope *sentry.Scope) {
				scope.SetTag("job", "operator-audit-log")
				sentry.CurrentHub().Recover(recovered)
			})
			sentry.Flush(2 * time.Second)
		}
	}()
	row := &auditModels.OperatorAuditEntry{
		OperatorID: entry.OperatorID, Action: entry.Action, ResourceType: entry.ResourceType,
		ResourceID: entry.ResourceID, RequestIP: securityruntime.ParseClientAddress(entry.IPAddress), CreatedAt: time.Now(),
	}
	if changes, encodeErr := entry.MFAAuditChanges(); encodeErr != nil {
		l.logger.Warn("failed to encode operator mfa audit changes",
			slog.String("action", entry.Action),
			slog.String("error", encodeErr.Error()),
		)
	} else {
		row.Changes = changes
	}
	logCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err = l.ledger.Create(logCtx, row)
	if err != nil {
		l.logger.Error("failed to log operator mfa audit event",
			slog.String("action", entry.Action),
			slog.String("error", err.Error()),
		)
	}
	return err
}
