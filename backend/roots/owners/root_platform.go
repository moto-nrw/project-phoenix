package owners

import (
	"context"
	"fmt"
	"strings"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	auditRecords "github.com/moto-nrw/project-phoenix/modules/auditlog/records"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
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
// query adapter bound to the caller's transaction and tenant, and the append
// store routed through the fail-closed command
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

// BuildAuditOwner builds the Audit Platform. The composition reads the
// tenant runtime's ambient transaction itself; writes are routed through the
// command before any service captures them.
func BuildAuditOwner(db transactionruntime.Database, env RootEnvironment) (AuditOwner, error) {
	audits, err := auditCompose.NewTenantPersistence(db.Pool())
	if err != nil {
		return AuditOwner{}, err
	}
	command, err := auditCompose.NewObservedCommand(audits.AppendStore, env.Logger.With("component", "audit-command"), env.Observe.AuditAppend)
	if err != nil {
		return AuditOwner{}, err
	}
	return AuditOwner{Persistence: audits.WithRoutedWrites(command), Command: command}, nil
}

// BuildMailTransport binds the process's realtime hub, builds the mail
// transport and the web push configuration. An invalid VAPID configuration fails startup.
func BuildMailTransport(env RootEnvironment) (email.Mailer, email.Dispatcher, email.Email, deliveryCompose.VAPIDConfig, error) {
	cfg, logger := env.Config, env.Logger
	mailer, err := deliveryCompose.NewMailTransport(email.MailerConfig{
		Host: cfg.EmailSMTPHost, Port: cfg.EmailSMTPPort, User: cfg.EmailSMTPUser, Password: cfg.EmailSMTPPassword,
		DefaultFrom: email.NewEmail(cfg.EmailFromName, cfg.EmailFromAddress), TemplateDir: "./templates",
		Logger: logger.With("service", "email"), AppEnv: cfg.AppEnv,
	})
	if err != nil {
		return nil, nil, email.Email{}, deliveryCompose.VAPIDConfig{}, fmt.Errorf("initialize email transport: %w", err)
	}
	if deliveryCompose.IsLoggingMailTransport(mailer) {
		logger.Warn("SMTP mailer not configured; using mock mailer (tokens will not be sent via SMTP)")
	}
	vapid := deliveryCompose.VAPIDConfig{
		PublicKey: strings.TrimSpace(cfg.VAPIDPublicKey), PrivateKey: strings.TrimSpace(cfg.VAPIDPrivateKey), Subscriber: strings.TrimSpace(cfg.VAPIDSubscriber),
	}
	if err := vapid.Validate(); err != nil {
		return nil, nil, email.Email{}, deliveryCompose.VAPIDConfig{}, fmt.Errorf("invalid VAPID configuration: %w", err)
	}
	return mailer,
		deliveryCompose.NewMailDispatcher(mailer, logger.With("component", "email"), email.DeliveryObserver(env.Observe.Delivery)),
		email.NewEmail(deliveryModule.SenderOrDefault(cfg.EmailFromName, cfg.EmailFromAddress)), vapid, nil
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

// TenantMailIdentity binds the reply-to of tenant-bound mail to the OGS, not
// to moto (#1936); one instance is shared by every tenant-bound sender.
func TenantMailIdentity(schools organizationtenancy.Query, settings settingsCompose.SettingsService, env RootEnvironment) email.ReplyToResolver {
	return deliveryCompose.NewTenantMailIdentity(SchoolContactDirectory{Schools: schools}, func(ctx context.Context, tenantID int64) (string, error) {
		return settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
	}, env.Logger)
}

// rootUnitOfWork is the unit of work a root composes its services over.
type rootUnitOfWork = tenant.UnitOfWork

// BuildSettingsOwner builds the Settings Platform complete: the tenant
// runtime, the start-page composition (#2875) inside the same tenant
// transaction as every other tenant-scoped setting, Enrollment's active
// phase restrictions as the collection guards (#1663) and the lookup
// observer. Only the service resolves through the root's unit of work; the
// rows keep the plain runtime they always ran over.
func BuildSettingsOwner(db transactionruntime.Database, unit rootUnitOfWork, env RootEnvironment, organizations organizationtenancy.Capability, hub *realtime.Hub, phases enrollmentPhaseRestrictions) (SettingsOwner, error) {
	rows := settingsCompose.NewTenantSettingsRuntime(db.Pool(), nil)
	homeLayouts := settingsCompose.NewHomeLayoutService(settingsCompose.NewHomeLayoutRepository(rows), rows, env.Logger, SettingsChangedNotifier(hub))
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

// BuildMealPlanSettings binds the Meal Plan's settings to the tenant
// settings it resolves per call.
func BuildMealPlanSettings(settings settingsCompose.BatchSettingsService) (*mealplanCompose.Settings, error) {
	return mealplanCompose.NewBoundSettings(
		func(ctx context.Context) (bool, error) {
			return settings.ResolveBool(ctx, configModels.KeyMealPlanEnabled)
		},
		func(ctx context.Context) (bool, error) {
			return settings.ResolveBool(ctx, configModels.KeyMealRegistrationEnabled)
		},
		func(ctx context.Context) (mealplanCompose.OrderingSettings, error) {
			return mealOrderingSettings(ctx, settings)
		},
	)
}

// mealOrderingKeys are the settings behind a school's meal ordering rules,
// read in one statement.
var mealOrderingKeys = []string{
	configModels.KeyMealRegistrationModel, configModels.KeyMealRegistrationDeadlineKind,
	configModels.KeyMealRegistrationLeadDays, configModels.KeyMealRegistrationDeadlineWeekday,
	configModels.KeyMealRegistrationCutoffTime, configModels.KeyMealCancellationDeadlineKind,
	configModels.KeyMealCancellationLeadDays, configModels.KeyMealCancellationDeadlineWeekday,
	configModels.KeyMealCancellationCutoffTime, configModels.KeyMealAnnualRegistrationDeadline,
	configModels.KeyMealAnnualChanges, configModels.KeyMealSickAfterDeadline,
	configModels.KeyMealOrderRhythm, configModels.KeyMealOrderDetail,
}

func mealOrderingSettings(ctx context.Context, settings settingsCompose.BatchSettingsService) (mealplanCompose.OrderingSettings, error) {
	snapshot, err := settings.ResolveMany(ctx, mealOrderingKeys)
	if err != nil {
		return mealplanCompose.OrderingSettings{}, err
	}
	var result mealplanCompose.OrderingSettings
	text := func(key string, target *string) {
		if err == nil {
			*target, err = snapshot.String(key)
		}
	}
	number := func(key string, target *int) {
		if err == nil {
			*target, err = snapshot.Int(key)
		}
	}
	text(configModels.KeyMealRegistrationModel, &result.Model)
	text(configModels.KeyMealRegistrationDeadlineKind, &result.RegistrationKind)
	number(configModels.KeyMealRegistrationLeadDays, &result.RegistrationLeadDays)
	text(configModels.KeyMealRegistrationDeadlineWeekday, &result.RegistrationWeekday)
	text(configModels.KeyMealRegistrationCutoffTime, &result.RegistrationTime)
	text(configModels.KeyMealCancellationDeadlineKind, &result.CancellationKind)
	number(configModels.KeyMealCancellationLeadDays, &result.CancellationLeadDays)
	text(configModels.KeyMealCancellationDeadlineWeekday, &result.CancellationWeekday)
	text(configModels.KeyMealCancellationCutoffTime, &result.CancellationTime)
	text(configModels.KeyMealAnnualRegistrationDeadline, &result.AnnualDeadline)
	text(configModels.KeyMealAnnualChanges, &result.AnnualChanges)
	text(configModels.KeyMealSickAfterDeadline, &result.SickAfterDeadline)
	text(configModels.KeyMealOrderRhythm, &result.OrderRhythm)
	text(configModels.KeyMealOrderDetail, &result.OrderDetail)
	return result, err
}

// BuildFeedbackSettings binds Feedback's settings to the tenant settings it
// resolves per call.
func BuildFeedbackSettings(settings settingsCompose.SettingsService) (*feedbackCompose.Settings, error) {
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

// BuildDeliveryRuntime composes Delivery's module before Identity & Access;
// the recipients' names come from the People Directory's guardian rows.
func BuildDeliveryRuntime(db transactionruntime.Database, env RootEnvironment, guardians GuardianDisplayQuery) (DeliveryRuntime, error) {
	observe := env.Observe.DurableDelivery
	runtime, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{
		DB: db.Pool(), People: GuardianDisplayResolver{Query: guardians},
		Observe: func(observation deliveryContract.Observation) {
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
	return OutboxEnqueuer{Outbox: r.Outbox, Runtime: r.Runtime}
}
