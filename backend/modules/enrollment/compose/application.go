package compose

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/turnstile"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/application"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Ports the composition root binds for Enrollment's application services.
type (
	PhaseRecords             = application.PhaseRecords
	PhaseOfferings           = application.PhaseOfferings
	CalendarPeriods          = application.CalendarPeriods
	CollectionSettings       = application.CollectionSettings
	FormSchemaRecords        = application.FormSchemaRecords
	CaptchaSettings          = application.CaptchaSettings
	NotificationSettings     = application.NotificationSettings
	NotificationModePin      = application.NotificationModePin
	MailOutbox               = application.MailOutbox
	Mail                     = application.Mail
	ExpirySnapshots          = application.ExpirySnapshots
	ApprovedSelections       = application.ApprovedSelections
	OfferingStudent          = application.OfferingStudent
	OfferingStudentDirectory = application.OfferingStudentDirectory
)

// Application services the composition root hands to its consumers beyond
// their public contract.
type (
	// Phases is the phase administration; the rollover also uses its
	// eligibility guard.
	Phases = application.Phases
	// PhaseExpirySnapshots reads the owner's phase-expiry report for the
	// tenant in context.
	PhaseExpirySnapshots = application.PhaseExpiryProjection
	// ApprovedOfferingProjection resolves approved offering selections to
	// still-enrolled children, also for Care Plan's baselines.
	ApprovedOfferingProjection = application.ApprovedOfferingProjection
)

func tenantRuntime() application.Runtime {
	return application.Runtime{
		Transactions: tenant.NewTransactionRunner(),
		TenantID:     tenant.FromContext,
		MarkRollback: tenant.MarkRollback,
		NotFound:     func(err error) bool { return errors.Is(err, sql.ErrNoRows) },
	}
}

// PhaseDependencies bind the phase administration to its owners. Calendar,
// Settings and the three response ports are optional in focused tests.
type PhaseDependencies struct {
	Records                         PhaseRecords
	Offerings                       PhaseOfferings
	Calendar                        CalendarPeriods
	LockTemplateRecurrence          func(context.Context) error
	ValidateCareOfferingPhaseChange func(context.Context, int64, *enrollment.Phase) error
	// SourcedTemplates resolves the decision flow's sourced-template
	// resyncer on every edit, so the root can bind it after the phase
	// administration exists.
	SourcedTemplates func() enrollment.SourcedTemplateResyncer
	Settings         CollectionSettings
	Roster           enrollment.PhaseResponseRoster
	CareExits        enrollment.PhaseResponseCareExits
	PortalAccounts   enrollment.PhaseResponsePortalAccounts
	Logger           *slog.Logger
	Today            func() calendar.Date
}

// NewPhases composes the phase administration in the ambient tenant
// transaction.
func NewPhases(deps PhaseDependencies) *Phases {
	var responses *application.PhaseResponseSources
	if deps.Roster != nil || deps.CareExits != nil || deps.PortalAccounts != nil {
		responses = &application.PhaseResponseSources{Roster: deps.Roster, CareExits: deps.CareExits, PortalAccounts: deps.PortalAccounts}
	}
	return application.NewPhases(application.PhaseDependencies{
		Records: deps.Records, Offerings: deps.Offerings, Calendar: deps.Calendar,
		LockTemplateRecurrence:          deps.LockTemplateRecurrence,
		ValidateCareOfferingPhaseChange: deps.ValidateCareOfferingPhaseChange,
		SourcedTemplates:                deps.SourcedTemplates,
		Settings:                        deps.Settings,
		Responses:                       responses,
		Runtime:                         tenantRuntime(),
		Logger:                          deps.Logger,
		Today:                           deps.Today,
	})
}

// NewFormSchemas composes form-schema publishing. Every write runs in the
// caller's tenant transaction, or opens one for a standalone caller.
func NewFormSchemas(records FormSchemaRecords, settings CollectionSettings, logger *slog.Logger) enrollment.FormSchemaAdministration {
	runtime := tenantRuntime()
	return application.NewTransactionalFormSchemas(application.NewFormSchemas(application.FormSchemaDependencies{
		Records: records, Settings: settings, Runtime: runtime, Logger: logger,
	}), runtime.Transactions)
}

// NewPhaseExpirySnapshots composes the phase-expiry report inputs for the
// tenant in context from People Directory and Care Plan reads.
func NewPhaseExpirySnapshots(owner ExpirySnapshots, students enrollment.PhaseExpiryStudents, carePlan enrollment.PhaseExpiryOfferings, bookings enrollment.PhaseExpiryBookings) *PhaseExpirySnapshots {
	return application.NewPhaseExpiryProjection(owner, students, carePlan, bookings, tenant.FromContext)
}

// NewPhaseExpiryWarnings composes the administrator-facing phase-expiry
// warnings over a snapshot source.
func NewPhaseExpiryWarnings(snapshots application.PhaseExpirySnapshots) enrollment.PhaseExpiryWarnings {
	return application.NewPhaseExpiryWarnings(snapshots)
}

// NewCaptcha composes captcha verification against Cloudflare Turnstile. An
// empty verifyURL uses Turnstile's siteverify endpoint; tests point it at a
// local provider.
func NewCaptcha(settings CaptchaSettings, verifyURL string, logger *slog.Logger) enrollment.CaptchaVerifier {
	return application.NewCaptcha(settings, turnstile.New(nil, verifyURL), logger)
}

// NotificationDependencies bind the parent mails to their owners.
type NotificationDependencies struct {
	Modes    NotificationModePin
	Settings NotificationSettings
	Outbox   MailOutbox
	Schools  enrollment.SchoolDirectory
	// Fingerprint identifies a decision state for the outbox idempotency
	// keys.
	Fingerprint func(content []byte) string
}

// NewNotifications composes the mail branding and decision notifications.
func NewNotifications(deps NotificationDependencies) enrollment.Notifications {
	return application.NewNotifications(application.NotificationDependencies{
		Modes: deps.Modes, Settings: deps.Settings, Outbox: deps.Outbox, Schools: deps.Schools,
		Fingerprint: deps.Fingerprint,
	})
}

// NewMailRenderers returns Enrollment's mail renderers.
func NewMailRenderers() enrollment.MailRenderers {
	return application.NewMailRenderers()
}

// NewApprovedOfferingProjection composes the approved-offering projection.
func NewApprovedOfferingProjection(selections ApprovedSelections, students OfferingStudentDirectory) *ApprovedOfferingProjection {
	return application.NewApprovedOfferingProjection(selections, students)
}
