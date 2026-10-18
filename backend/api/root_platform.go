package api

import (
	"context"
	"fmt"
	"strings"

	auditRecords "github.com/moto-nrw/project-phoenix/modules/auditlog/records"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	feedbackCompose "github.com/moto-nrw/project-phoenix/modules/feedback/compose"
	mealplanCompose "github.com/moto-nrw/project-phoenix/modules/mealplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// AuditOwner is the Audit Platform as the roots compose it: every retained
// query adapter bound to the caller's transaction, tenant and read-only root
// DB fallback, and the append store routed through the fail-closed command
// (ADR 0049, #4013).
type auditCommand interface {
	auditRecords.Command
	retentionAuditLedgers
	retentionDeletionLog
	DeleteExpiredOperatorAuditLog(context.Context) (int, error)
}

type AuditOwner struct {
	Persistence *auditCompose.Persistence
	Command     auditCommand
}

// buildAuditOwner builds the Audit Platform. The composition reads the
// tenant runtime's ambient transaction itself; writes are routed through the
// command before any service captures them.
func buildAuditOwner(db transactionruntime.Database, env RootEnvironment) (AuditOwner, error) {
	audits, err := auditCompose.NewTenantPersistence(db.Pool(), env.AllowAuditRootWrites)
	if err != nil {
		return AuditOwner{}, err
	}
	command, err := auditCompose.NewObservedCommand(audits.AppendStore, env.Logger.With("component", "audit-command"), env.Observe.AuditAppend)
	if err != nil {
		return AuditOwner{}, err
	}
	return AuditOwner{Persistence: audits.WithRoutedWrites(command), Command: command}, nil
}

// MailTransport is Delivery's synchronous side: the realtime hub every live
// view shares, the SMTP mailer and its dispatcher, the default sender and the
// validated web push keys.
type MailTransport struct {
	Hub         *realtime.Hub
	Mailer      email.Mailer
	Dispatcher  *email.Dispatcher
	DefaultFrom email.Email
	VAPID       deliveryCompose.VAPIDConfig
}

// buildMailTransport binds the process's realtime hub, builds the mail
// transport and the web push configuration. An invalid VAPID configuration fails startup.
func buildMailTransport(env RootEnvironment, hub *realtime.Hub) (MailTransport, error) {
	cfg, logger := env.Config, env.Logger
	mailer, err := email.NewMailer(email.MailerConfig{
		Host: cfg.EmailSMTPHost, Port: cfg.EmailSMTPPort, User: cfg.EmailSMTPUser, Password: cfg.EmailSMTPPassword,
		DefaultFrom: email.NewEmail(cfg.EmailFromName, cfg.EmailFromAddress), TemplateDir: "./templates",
		Logger: logger.With("service", "email"), AppEnv: cfg.AppEnv,
	})
	if err != nil {
		return MailTransport{}, fmt.Errorf("initialize email transport: %w", err)
	}
	if _, ok := mailer.(*email.MockMailer); ok {
		logger.Warn("SMTP mailer not configured; using mock mailer (tokens will not be sent via SMTP)")
	}
	vapid := deliveryCompose.VAPIDConfig{
		PublicKey: strings.TrimSpace(cfg.VAPIDPublicKey), PrivateKey: strings.TrimSpace(cfg.VAPIDPrivateKey), Subscriber: strings.TrimSpace(cfg.VAPIDSubscriber),
	}
	if err := vapid.Validate(); err != nil {
		return MailTransport{}, fmt.Errorf("invalid VAPID configuration: %w", err)
	}
	return MailTransport{
		Hub:         hub,
		Mailer:      mailer,
		Dispatcher:  email.NewDispatcher(mailer, logger.With("component", "email"), email.DeliveryObserver(env.Observe.Delivery)),
		DefaultFrom: email.NewEmail(deliveryModule.SenderOrDefault(cfg.EmailFromName, cfg.EmailFromAddress)),
		VAPID:       vapid,
	}, nil
}

// SettingsOwner is the Settings Platform: the tenant settings runtime its
// rows and the tenant operations run over, and the settings service with
// the start-page composition.
type SettingsOwner struct {
	// Rows is the runtime the setting, audit and home-layout rows and the
	// tenant operations run over; Service resolves through the root's unit
	// of work.
	Rows    settingsCompose.TenantSettingsRuntime
	Service settingsCompose.BatchSettingsService
}

// tenantMailIdentity binds the reply-to of tenant-bound mail to the OGS, not
// to moto (#1936); one instance is shared by every tenant-bound sender.
func tenantMailIdentity(schools organizationtenancy.Query, settings settingsCompose.SettingsService, env RootEnvironment) email.ReplyToResolver {
	return deliveryCompose.NewTenantMailIdentity(SchoolContactDirectory{Schools: schools}, func(ctx context.Context, tenantID int64) (string, error) {
		return settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
	}, env.Logger)
}

// rootUnitOfWork is the unit of work a root composes its services over.
type rootUnitOfWork = tenant.UnitOfWork

// buildSettingsOwner builds the Settings Platform complete: the tenant
// runtime, the start-page composition (#2875) inside the same tenant
// transaction as every other tenant-scoped setting, Enrollment's active
// phase restrictions as the collection guards (#1663) and the lookup
// observer. Only the service resolves through the root's unit of work; the
// rows keep the plain runtime they always ran over.
func buildSettingsOwner(db transactionruntime.Database, unit rootUnitOfWork, env RootEnvironment, organizations organizationtenancy.Capability, hub *realtime.Hub, phases enrollmentPhaseRestrictions) (SettingsOwner, error) {
	rows := settingsCompose.NewTenantSettingsRuntime(db.Pool(), nil)
	homeLayouts := settingsCompose.NewHomeLayoutService(settingsCompose.NewHomeLayoutRepository(rows), rows, env.Logger, settingsChangedNotifier(hub))
	service, err := settingsCompose.NewSettings(settingsCompose.SettingsDependencies{
		Values: settingsCompose.NewSettingValueRepository(rows), Audit: settingsCompose.NewSettingAuditRepository(rows),
		Schools: NewSchoolSettingsStore(organizations), Runtime: settingsCompose.NewTenantSettingsRuntime(db.Pool(), &unit),
		Logger: env.Logger, HomeLayouts: homeLayouts, ObserveLookup: env.Observe.SettingsLookup,
		ClassRestrictionGuard: phases.HasActiveClassRestrictedPhase,
		GradeRestrictionGuard: phases.HasActiveGradeRestrictedPhase,
		GradeCapGuard:         phases.MaxActivePhaseGrade,
	})
	if err != nil {
		return SettingsOwner{}, err
	}
	return SettingsOwner{Rows: rows, Service: service}, nil
}

// buildMealPlanSettings binds the Meal Plan's settings to the tenant
// settings it resolves per call.
func buildMealPlanSettings(settings settingsCompose.SettingsService) (*mealplanCompose.Settings, error) {
	return mealplanCompose.NewBoundSettings(
		func(ctx context.Context) (bool, error) {
			return settings.ResolveBool(ctx, configModels.KeyMealPlanEnabled)
		},
		func(ctx context.Context) (bool, error) {
			return settings.ResolveBool(ctx, configModels.KeyMealRegistrationEnabled)
		},
		func(ctx context.Context) (string, error) {
			return settings.ResolveString(ctx, configModels.KeyMealRegistrationCutoffTime)
		},
	)
}

// buildFeedbackSettings binds Feedback's settings to the tenant settings it
// resolves per call.
func buildFeedbackSettings(settings settingsCompose.SettingsService) (*feedbackCompose.Settings, error) {
	return feedbackCompose.NewBoundSettings(
		func(ctx context.Context) (bool, error) {
			return settings.ResolveBool(ctx, configModels.KeyFeedbackEnabled)
		},
		func(ctx context.Context) (int, error) {
			return settings.ResolveInt(ctx, configModels.KeyFeedbackDataRetentionDays)
		},
	)
}

// DeliveryRuntime is Delivery's durable side: the module with the outbox
// and the delivery state the mails of Identity & Access and Enrollment wait
// on. The Worker that drains it is composed last, once every mail renderer
// exists.
type DeliveryRuntime struct {
	Runtime *deliveryCompose.Runtime
	Outbox  *deliveryCompose.EmailOutbox
}

// buildDeliveryRuntime composes Delivery's module before Identity & Access;
// the recipients' names come from the People Directory's guardian rows.
func buildDeliveryRuntime(db transactionruntime.Database, env RootEnvironment, guardians GuardianDisplayQuery) (DeliveryRuntime, error) {
	observe := env.Observe.DurableDelivery
	runtime, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{
		DB: db.Pool(), People: GuardianDisplayResolver{Query: guardians},
		Observe: func(observation deliveryModule.Observation) {
			observe(string(observation.Transport), observation.Template, observation.Operation, observation.Duration, observation.Count, observation.Err)
		},
	})
	if err != nil {
		return DeliveryRuntime{}, fmt.Errorf("initialize delivery module: %w", err)
	}
	return DeliveryRuntime{Runtime: runtime, Outbox: deliveryCompose.NewEmailOutbox(deliveryCompose.NewDurableEmails(runtime.Module))}, nil
}

// Enqueuer serves the root's mail outbox contract over Delivery's enqueue
// facade.
func (r DeliveryRuntime) Enqueuer() OutboxEnqueuer {
	return OutboxEnqueuer{Outbox: r.Outbox, Module: r.Runtime.Module}
}
