package owners

import (
	"context"
	"fmt"
	"strings"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/spf13/viper"
)

// IdentityMailState reads Delivery's state of a queued mail; Identity's
// welcome mails wait for the mail they follow (#4006).
type IdentityMailState func(ctx context.Context, tenantID, id int64) (deliveryContract.State, bool, error)

// BuildIdentityAccess builds Identity & Access over the owners and the
// values the root built before it (#4006): sessions, the account lifecycle,
// the second factor and both portals' passkey ceremonies, the operator
// flows, the demo access and the caller context. Delivery is built first,
// so the guardian and staff mails enqueue into its outbox directly.
func BuildIdentityAccess(
	db transactionruntime.Database, unit rootUnitOfWork, env RootEnvironment, tokens TokenAdapter,
	organizations organizationtenancy.Capability, persons peopledirectory.Capability, membership schoolmembership.Capability,
	settings settingsCompose.BatchSettingsService, audit AuditOwner, operatorAuditLog OperatorAuditEntries,
	mailSender email.Mailer, mailDispatcher email.Dispatcher, mailFrom email.Email, deliveryRuntime *deliveryCompose.Runtime, mailIdentity email.ReplyToResolver, outbox MailResultOutbox,
	people PeopleRows, guardianEnrollments guardianEnrollmentClaims,
	groups EducationGroupRows, staffGroups schoolstructure.StaffGroupQuery, activityGroups timetablePorts.CallerActivityGroups,
	sessions *presenceCompose.SessionRecords, presence *studentpresence.Module,
) (*identityaccess.Module, *CallerRows, error) {
	durable, ok := outbox.(DurableMailOutbox)
	if !ok {
		return nil, nil, fmt.Errorf("identity access: transactional outbox is required")
	}
	cfg, links := env.Config, env.Links
	authLogger := env.Logger.With("service", "auth")
	platformLogger := env.Logger.With("service", "platform")
	tenantDomain := strings.TrimSpace(cfg.TenantDomain)
	if tenantDomain == "" {
		return nil, nil, fmt.Errorf("TENANT_DOMAIN is required")
	}
	operatorFrontendURL, err := operatorFrontendURLOf(cfg.OperatorHostname, links.FrontendURL)
	if err != nil {
		return nil, nil, err
	}
	identityChain := LifecycleRepositories{
		Persons: people.Persons, Staff: people.Staff, Teachers: people.Teachers, Students: people.Students,
		GuardianProfiles: people.GuardianProfiles, StudentGuardians: people.StudentGuardians,
		People: persons, Membership: membership,
	}
	operators, err := NewOperatorDependencies(OperatorRepositoriesOf(identityChain, operatorAuditLog), organizations, persons, membership, platformLogger)
	if err != nil {
		return nil, nil, err
	}
	mfaMail, err := NewMFAMailer(db, deliveryRuntime, mailSender, mailFrom, links.FrontendURL, authLogger, nil)
	if err != nil {
		return nil, nil, err
	}
	mfa, err := NewMFADependencies(tokens.SessionTokens, settings, mfaMail, operatorAuditLog, cfg.JWTSecret, &identityaccessCompose.PasskeyDependencies{
		RPID: tenantDomain, RPName: identityaccess.PasskeyRPName,
		TenantDomain: tenantDomain, OperatorFrontendURL: operatorFrontendURL,
	}, authLogger)
	if err != nil {
		return nil, nil, err
	}
	sessionRepos := SessionRepositoriesOf(identityChain, organizations, audit.Persistence.AuthEvent, deliveryCompose.NewPushSubscriptionRepository(db.Pool()))
	// The accept and login links go to the parents portal, never to the
	// staff frontend.
	guardianMail, enrollmentClaims, err := GuardianInvitationDependencies(settings, organizations, outbox, guardianEnrollments,
		links.ParentsURL, links.InvitationTokenExpiry, authLogger.With("flow", "guardian_invitation"))
	if err != nil {
		return nil, nil, err
	}
	// The lifecycle flows (#3225) read the role management of the module
	// they are composed into.
	catererMail, err := CatererInvitationDelivery(organizations, outbox, links.CatererURL)
	if err != nil {
		return nil, nil, err
	}
	lifecycle, err := LifecycleDependencies(sessionRepos.Lifecycle, audit.Command, CaregiverProfiles{Persons: persons, Membership: membership},
		guardianMail, catererMail, enrollmentClaims, authLogger)
	if err != nil {
		return nil, nil, err
	}
	demo, err := DemoDependencies(
		demoAccessWiringFor(strings.ToLower(cfg.AppEnv), mailDispatcher, mailFrom, links.FrontendURL, viper.GetInt("demo_max_active_schools"), tokens, authLogger, durable),
		organizationtenancy.StandingDemoSchool(viper.GetBool("demo_standing_school")), organizations)
	if err != nil {
		return nil, nil, err
	}
	// The deliveries record their outcome through the recorder the
	// composition hands them at construction.
	module, err := NewIdentityAccessWithSessions(identityaccessCompose.Dependencies{
		DB: db.Pool(), Observe: requiredIdentityAccessObservation(env.Observe.IdentityAccess), TenantRuntime: unit,
		MFA: mfa, Lifecycle: lifecycle, Operators: operators, Demo: demo,
		Resets: PasswordResetDependencies(&PasswordResetWiring{
			Dispatcher: mailDispatcher, DefaultFrom: mailFrom, Durable: durable,
			StaffURL: links.FrontendURL, ParentsURL: links.ParentsURL, SchoolURL: links.SchoolURL, CatererURL: links.CatererURL,
			Expiry: links.PasswordResetTokenExpiry, RateLimitEnabled: cfg.RateLimitEnabled,
		}, authLogger),
		Invitations: InvitationDependencies(&InvitationWiring{
			Dispatcher: mailDispatcher, DefaultFrom: mailFrom, Durable: durable, StaffURL: links.FrontendURL, SchoolURL: links.SchoolURL,
			MailIdentity: mailIdentity, Expiry: links.InvitationTokenExpiry,
			Settings:     settings,
			Outbox:       outbox,
			WelcomeDelay: identityaccess.StaffWelcomeDelay,
		}, tokens.SessionTokens, authLogger),
		OperatorProvisioning: operatorProvisioningDependencies(&operatorLinkWiring{
			dispatcher: mailDispatcher, defaultFrom: mailFrom, durable: durable,
			frontendURL: links.FrontendURL, operatorFrontendURL: operatorFrontendURL,
			invitationExpiry: links.InvitationTokenExpiry, emailChangeExpiry: emailChangeExpiry(links.PasswordResetTokenExpiry, env.Logger),
			logger: platformLogger,
		}, authLogger),
	}, sessionRepos, tokens.SessionTokens, settings, audit.Command, authLogger)
	if err != nil {
		return nil, nil, err
	}
	// The caller context (#3501) resolves the caller's chain through the
	// owners of each link; the rows serve the consumers that still speak the
	// retained records.
	usercontextLogger := env.Logger.With("service", "usercontext")
	caller, err := NewCallerContext(tokens, module, CallerPersons{Rows: people.Persons, People: persons}, membership, staffGroups,
		activityGroups, presence, settings, usercontextLogger)
	if err != nil {
		return nil, nil, err
	}
	rows, err := NewCallerRows(caller, groups, people.Staff, people.Teachers, people.Students, activityGroups, sessions, usercontextLogger)
	if err != nil {
		return nil, nil, err
	}
	return module, rows, nil
}

// guardianEnrollmentClaims are the enrollment requests an accepted guardian
// invitation claims.
type guardianEnrollmentClaims interface {
	BackfillGuardianAccountID(ctx context.Context, accountID int64, email string) (int, error)
}

// IdentityDeliveryRenderers render the Identity & Access mail kinds of the
// Delivery outbox: the guardian invitation, the guardian welcome and the
// staff welcome.
func IdentityDeliveryRenderers(module *identityaccess.Module, defaultFrom email.Email, mailIdentity email.ReplyToResolver, mailState IdentityMailState, env RootEnvironment) map[string]deliveryCompose.MailRenderer {
	logger := env.Logger.With("service", "auth")
	return map[string]deliveryCompose.MailRenderer{
		TransactionalMailKind: TransactionalMailRenderer{Identity: module, Logger: logger},
		identityaccess.MailKindGuardianInvitation: payloadRenderer(NewGuardianInvitationRenderer(GuardianInvitationRendererConfig{
			DefaultFrom: defaultFrom,
		})),
		identityaccess.MailKindCatererInvitation: payloadRenderer(NewCatererInvitationRenderer(defaultFrom)),
		identityaccess.MailKindGuardianWelcome: payloadRenderer(NewGuardianWelcomeRenderer(GuardianInvitationRendererConfig{
			DefaultFrom:          defaultFrom,
			PrecedingEmailStatus: identityaccessCompose.NewGuardianWelcomeOrder(precedingMails{status: mailState}),
		})),
		identityaccess.MailKindStaffWelcome: NewStaffWelcomeRenderer(StaffWelcomeRendererConfig{
			DefaultFrom: defaultFrom, MailIdentity: mailIdentity, Logger: logger,
			InvitationDeliverySent: module.SchoolInvitationDeliverySent,
		}),
	}
}
