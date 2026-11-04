package owners

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer"
	exportTransferCompose "github.com/moto-nrw/project-phoenix/modules/exporttransfer/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/tokenprimitives"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Export ans Lohnbüro (#4805): the root binds each port of the payroll
// dispatch to one owner: the Settings Platform, Export Transfer, Delivery,
// Organisation & Tenancy and the People Directory's staff contacts. Workforce
// writes the mails; Delivery carries them.

// BuildExportTransfer wires the Export Transfer capability (#3050) over the
// tenant settings. The manual SFTP route and the payroll dispatch each build
// it where they are composed.
func BuildExportTransfer(settings settingsCompose.SettingsService, db transactionruntime.Database, logger *slog.Logger) (exporttransfer.Capability, error) {
	// The setting keys stay in the settings layer next to their registry
	// definitions; the root only passes the binding on.
	resolvers, keys := settingsCompose.NewSFTPExportSettings(settings)
	return exportTransferCompose.New(exportTransferCompose.Dependencies{
		DB: db.Pool(),
		Settings: exportTransferCompose.Settings{
			Enabled: resolvers.Enabled, Host: resolvers.Host, Port: resolvers.Port, Username: resolvers.Username,
			Password: resolvers.Password, RemoteDirectory: resolvers.RemoteDirectory, HostKeyFingerprint: resolvers.HostKeyFingerprint,
		},
		Keys: exportTransferCompose.SettingKeys{
			Host: keys.Host, Port: keys.Port, Username: keys.Username, Password: keys.Password,
			RemoteDirectory: keys.RemoteDirectory, HostKeyFingerprint: keys.HostKeyFingerprint,
		},
		Logger: logger,
	})
}

// PayrollDispatchStaff lists the staff holding a permission, with contacts.
type PayrollDispatchStaff interface {
	ListStaffWithPermission(context.Context, string) ([]*peopleValues.StaffWithRoleInfo, error)
}

// BuildPayrollDispatch composes the automatic send of the month file over the
// clock's export.
func BuildPayrollDispatch(
	db transactionruntime.Database, settings settingsCompose.SettingsService, export workforce.PayrollExport,
	transfer exporttransfer.Capability, outbox OutboxEnqueuer, mailFrom email.Email, mailIdentity email.ReplyToResolver,
	schools organizationtenancy.Query, staff PayrollDispatchStaff, frontendURL string, logger *slog.Logger,
) (workforce.PayrollDispatch, error) {
	if settings == nil || transfer == nil || outbox.Runtime == nil || schools == nil || staff == nil {
		return nil, errors.New("payroll dispatch: settings, transfer, delivery, schools and staff are required")
	}
	logger = logger.With("service", "payroll_dispatch")
	return workforceCompose.NewPayrollDispatch(db.Pool(), workforce.PayrollDispatchPorts{
		Settings: payrollDispatchSettings{settings: settings}, Export: export,
		Transfer: payrollTransfer{module: transfer},
		Mail:     payrollMailOutbox{outbox: outbox, from: mailFrom, identity: mailIdentity, logger: logger},
		Schools:  payrollSchools{schools: schools}, Managers: payrollManagers{staff: staff},
		Tokens: payrollTokens{}, FrontendURL: frontendURL, Logger: logger,
	})
}

type payrollDispatchSettings struct {
	settings settingsCompose.SettingsService
}

func (s payrollDispatchSettings) PayrollDispatchConfig(ctx context.Context) (workforce.PayrollDispatchConfig, error) {
	var config workforce.PayrollDispatchConfig
	var err error
	if config.Auto, err = s.settings.ResolveBool(ctx, settingsValues.KeyPayrollDispatchAuto); err != nil {
		return config, err
	}
	if config.Channel, err = s.settings.ResolveString(ctx, settingsValues.KeyPayrollDispatchChannel); err != nil {
		return config, err
	}
	if config.Format, err = s.settings.ResolveString(ctx, settingsValues.KeyPayrollDispatchFormat); err != nil {
		return config, err
	}
	config.Email, err = s.settings.ResolveString(ctx, settingsValues.KeyPayrollDispatchEmail)
	return config, err
}

type payrollTransfer struct {
	module exporttransfer.Capability
}

func (t payrollTransfer) Transfer(ctx context.Context, request workforce.PayrollTransferRequest) (workforce.PayrollTransferOutcome, error) {
	outcome, err := t.module.Transfer(ctx, exporttransfer.Request{
		Kind: exporttransfer.KindStaffTimeTracking, Format: request.Format, Period: request.Period,
		PrepareFile: request.Prepare, ActorAccountID: request.ActorAccountID, ActorName: request.ActorName,
	})
	if err != nil {
		if errors.Is(err, exporttransfer.ErrUnknownSettlement) {
			return workforce.PayrollTransferOutcome{Uncertain: true}, nil
		}
		return workforce.PayrollTransferOutcome{}, err
	}
	return workforce.PayrollTransferOutcome{
		Transferred:   outcome.Transferred,
		NotConfigured: outcome.Reason == exporttransfer.ReasonNotConfigured,
		Reason:        outcome.Reason,
	}, nil
}

func (t payrollTransfer) RecordEmail(ctx context.Context, result workforce.PayrollEmailResult) error {
	record := exporttransfer.EmailRecord{
		Kind: exporttransfer.KindStaffTimeTracking, Format: result.Format, Period: result.Period,
		Filename: result.Filename, ByteSize: result.ByteSize, Recipient: result.Recipient,
		Delivered: result.Delivered, ActorAccountID: result.ActorAccountID, ActorName: result.ActorName,
	}
	if !result.Delivered {
		record.Reason = exporttransfer.ReasonMailFailed
	}
	return t.module.RecordEmail(ctx, record)
}

type payrollTokens struct{}

func (payrollTokens) NewToken() (string, string, error) {
	return tokenprimitives.NewOpaqueCapabilityToken()
}

func (payrollTokens) Fingerprint(raw string) string {
	return tokenprimitives.OpaqueCapabilityFingerprint(raw)
}

// payrollMailOutbox carries Workforce's mails through Delivery's durable
// outbox, with the school's reply-to address.
type payrollMailOutbox struct {
	outbox   OutboxEnqueuer
	from     email.Email
	identity email.ReplyToResolver
	logger   *slog.Logger
}

func (o payrollMailOutbox) Enqueue(ctx context.Context, message workforce.PayrollMailMessage) (int64, error) {
	mail := email.Message{
		From: o.from, To: email.NewEmail(message.ToName, message.ToAddress),
		Subject: message.Subject, Template: message.Template, Content: payrollTemplateData(message.Content),
	}
	identity := email.ResolveReplyToIdentity(ctx, o.identity, message.TenantID, o.logger)
	mail.ReplyTo = email.NewEmail(identity.Name, identity.Address)
	enqueued, err := o.outbox.EnqueueMailIntent(ctx, DurableMail{
		IdempotencyKey: message.IdempotencyKey, TenantID: message.TenantID, Message: mail,
		Type: message.Type, ReferenceID: message.ReferenceID, ExpiresAt: message.ExpiresAt,
		MaxAttempts: message.MaxAttempts, Backoff: message.Backoff,
	})
	return enqueued.ID, err
}

// payrollTemplateData is the template data of the payroll mails.
func payrollTemplateData(content workforce.PayrollMailContent) map[string]any {
	return map[string]any{
		"LogoURL": content.LogoURL, "SchoolName": content.SchoolName, "FirstName": content.FirstName,
		"MonthLabel": content.MonthLabel, "Correction": content.Correction, "LinkURL": content.LinkURL,
		"ExpiresAt": content.ExpiresAt, "ReasonText": content.ReasonText,
	}
}

// State reads Delivery's state of a mail. A mail Delivery gave up on or
// dropped counts as failed; so does a mail it no longer knows.
func (o payrollMailOutbox) State(ctx context.Context, outboxID int64) (string, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return "", err
	}
	state, found, err := o.outbox.Runtime.EmailStatus(ctx, tenantID.Int64(), outboxID)
	switch {
	case err != nil:
		return "", err
	case !found:
		return workforce.PayrollMailFailed, nil
	case state == deliveryContract.StateSent:
		return workforce.PayrollMailSent, nil
	case state == deliveryContract.StateDeadLetter || state == deliveryContract.StateCancelled:
		return workforce.PayrollMailFailed, nil
	default:
		return workforce.PayrollMailPending, nil
	}
}

type payrollSchools struct {
	schools organizationtenancy.Query
}

func (s payrollSchools) School(ctx context.Context, tenantID int64) (workforce.PayrollSchool, error) {
	school, _, err := LookupSchool(ctx, s.schools, tenantID)
	return workforce.PayrollSchool{Name: school.Name, Subdomain: school.Subdomain}, err
}

type payrollManagers struct {
	staff PayrollDispatchStaff
}

func (m payrollManagers) TimeTrackingManagers(ctx context.Context) ([]workforce.PayrollContact, error) {
	rows, err := m.staff.ListStaffWithPermission(ctx, permissions.TimeTrackingManage)
	if err != nil {
		return nil, err
	}
	contacts := make([]workforce.PayrollContact, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			contacts = append(contacts, workforce.PayrollContact{StaffID: row.StaffID, FirstName: row.FirstName, LastName: row.LastName, Email: row.Email})
		}
	}
	return contacts, nil
}
