package api

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"

	"github.com/spf13/viper"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// Identity & Access is composed in the root (#4006, ADR 0053 points 1, 4 and
// 8). The service graph consumes the module, so it hands the root's
// composition the values it builds itself: the settings, the audit ledger,
// the mail transport, the identity chain rows. Every seam that binds
// Identity to another owner lives in the root; the module is built through
// modules/identityaccess/compose, and the session codec comes from the root's
// session token adapter.

// identityRoot composes Identity & Access for the service graph.
type identityRoot struct {
	db     transactionruntime.Database
	tokens TokenAdapter
}

// composedIdentity is what the root mounts once the service graph composed
// Identity & Access: the module and its caller context with the rows
// the retained routes read.
type composedIdentity func() (*identityaccess.Module, *CallerRows)

// newIdentityRoot returns the composition the service graph receives and a
// read of what it composed.
func newIdentityRoot(db transactionruntime.Database, tokens TokenAdapter) (IdentityRoot, composedIdentity) {
	root := identityRoot{db: db, tokens: tokens}
	var module *identityaccess.Module
	var userContext *CallerRows
	compose := func(m IdentityMaterial) (IdentityComposition, error) {
		composition, rows, err := root.compose(m)
		if err != nil {
			return IdentityComposition{}, err
		}
		module, userContext = composition.Access, rows
		return composition, nil
	}
	wiring := IdentityRoot{
		Compose:     compose,
		Permissions: tokens.Permissions,
		ActorScope:  tokens.actorScope,
		AuditActor:  tokens.AuditActor,
	}
	return wiring, func() (*identityaccess.Module, *CallerRows) { return module, userContext }
}

// compose builds Identity & Access over the material: sessions, the account
// lifecycle, the second factor and both portals' passkey ceremonies, the
// operator flows, the demo access, the caller context and the mails.
func (r identityRoot) compose(m IdentityMaterial) (IdentityComposition, *CallerRows, error) {
	cfg := m.Config
	authLogger := m.Logger.With("service", "auth")
	platformLogger := m.Logger.With("service", "platform")
	tenantDomain := strings.TrimSpace(cfg.TenantDomain)
	if tenantDomain == "" {
		return IdentityComposition{}, nil, fmt.Errorf("TENANT_DOMAIN is required")
	}
	operatorFrontendURL, err := operatorFrontendURLOf(cfg.OperatorHostname, m.FrontendURL)
	if err != nil {
		return IdentityComposition{}, nil, err
	}
	identityChain := LifecycleRepositories{
		Persons: m.PersonRows, Staff: m.StaffRows, Teachers: m.TeacherRows, Students: m.StudentRows,
		GuardianProfiles: m.GuardianProfiles, StudentGuardians: m.StudentGuardians,
		People: m.Persons, Membership: m.Membership,
	}
	operators, err := NewOperatorDependencies(OperatorRepositoriesOf(identityChain, m.OperatorAuditLog), m.Organizations, m.Persons, m.Membership, platformLogger)
	if err != nil {
		return IdentityComposition{}, nil, err
	}
	mfa, err := newMFADependencies(r.tokens.SessionTokens, m.Settings, mfaMailer{
		dispatcher: m.Dispatcher, from: m.DefaultFrom, frontendURL: m.FrontendURL, logger: authLogger,
	}, m.OperatorAuditLog, cfg.JWTSecret, &identityaccessCompose.PasskeyDependencies{
		RPID: tenantDomain, RPName: identityaccess.PasskeyRPName,
		TenantDomain: tenantDomain, OperatorFrontendURL: operatorFrontendURL,
	}, authLogger)
	if err != nil {
		return IdentityComposition{}, nil, err
	}
	sessionRepos := SessionRepositoriesOf(identityChain, m.Organizations, m.AuthEvents, m.PushSubscriptions)
	// The accept and login links go to the parents portal, never to the
	// staff frontend.
	guardianMail, enrollmentClaims, err := guardianInvitationDependencies(m.Settings, m.Organizations, lateMailOutbox{m.Outbox}, m.GuardianEnrollments,
		m.ParentsURL, m.InvitationExpiry, authLogger.With("flow", "guardian_invitation"))
	if err != nil {
		return IdentityComposition{}, nil, err
	}
	// The lifecycle flows (#3225) read the role management of the module
	// they are composed into.
	lifecycle, err := lifecycleDependencies(sessionRepos.Lifecycle, m.Audit, CaregiverProfiles{Persons: m.Persons, Membership: m.Membership},
		guardianMail, enrollmentClaims, authLogger)
	if err != nil {
		return IdentityComposition{}, nil, err
	}
	demo, err := demoDependencies(
		demoAccessWiringFor(strings.ToLower(cfg.AppEnv), m.Dispatcher, m.DefaultFrom, m.FrontendURL, viper.GetInt("demo_max_active_schools"), r.tokens, authLogger),
		organizationtenancy.StandingDemoSchool(viper.GetBool("demo_standing_school")), m.Organizations)
	if err != nil {
		return IdentityComposition{}, nil, err
	}
	module, err := NewIdentityAccessWithSessions(identityaccessCompose.Dependencies{
		DB: r.db.Pool(), Observe: requiredIdentityAccessObservation(m.Observe),
		MFA: mfa, Lifecycle: lifecycle, Operators: operators, Demo: demo,
		Resets: passwordResetDependencies(&PasswordResetWiring{
			Dispatcher: m.Dispatcher, DefaultFrom: m.DefaultFrom,
			StaffURL: m.FrontendURL, ParentsURL: m.ParentsURL, SchoolURL: m.SchoolURL,
			Expiry: m.PasswordResetExpiry, RateLimitEnabled: cfg.RateLimitEnabled,
		}, authLogger),
		Invitations: invitationDependencies(&InvitationWiring{
			Dispatcher: m.Dispatcher, DefaultFrom: m.DefaultFrom, StaffURL: m.FrontendURL, SchoolURL: m.SchoolURL,
			MailIdentity: m.MailIdentity, Expiry: m.InvitationExpiry,
			Settings:     m.Settings,
			Outbox:       lateMailOutbox{m.Outbox},
			WelcomeDelay: identityaccess.StaffWelcomeDelay,
		}, r.tokens.SessionTokens, authLogger),
		OperatorProvisioning: operatorProvisioningDependencies(&operatorLinkWiring{
			dispatcher: m.Dispatcher, defaultFrom: m.DefaultFrom,
			frontendURL: m.FrontendURL, operatorFrontendURL: operatorFrontendURL,
			invitationExpiry: m.InvitationExpiry, emailChangeExpiry: emailChangeExpiry(m.PasswordResetExpiry, m.Logger),
			logger: platformLogger,
		}, authLogger),
	}, sessionRepos, r.tokens.SessionTokens, m.Settings, m.Audit, authLogger)
	if err != nil {
		return IdentityComposition{}, nil, err
	}
	// The caller context (#3501) resolves the caller's chain through the
	// owners of each link; the rows serve the consumers that still speak the
	// retained records.
	usercontextLogger := m.Logger.With("service", "usercontext")
	caller, err := newCallerContext(r.tokens, module, callerPersons{rows: m.PersonRows, people: m.Persons}, m.Membership, m.StaffGroups,
		m.ActivityGroups, m.Presence, m.Settings, usercontextLogger)
	if err != nil {
		return IdentityComposition{}, nil, err
	}
	rows, err := NewCallerRows(caller, m.Groups, m.StaffRows, m.TeacherRows, m.StudentRows, m.ActivityGroups, m.Sessions, usercontextLogger)
	if err != nil {
		return IdentityComposition{}, nil, err
	}
	return IdentityComposition{
		Access:               module,
		Caller:               rows,
		GroupLiveCaller:      GroupLiveCaller{rows},
		SupervisionCaller:    SupervisionCaller{rows},
		GuardianInvitations:  NewGuardianInvitationReads(module),
		ParentGuardianAccess: NewParentGuardianAccess(module),
		MailRenderers:        identityMailRenderers(module, m, authLogger),
	}, rows, nil
}

// operatorFrontendURLOf is the operator portal host the invitation mails
// link to. The operator subdomain is separate from FRONTEND_URL, so the link
// goes straight to the operator host: a cross-origin redirect hop is what
// mail content scanners treat as a phishing signal.
func operatorFrontendURLOf(operatorHostname, frontendURL string) (string, error) {
	if operatorHostname == "" {
		return "", fmt.Errorf("NEXT_PUBLIC_OPERATOR_HOSTNAME is required")
	}
	protocol := "http"
	if strings.HasPrefix(frontendURL, "https://") {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s", protocol, strings.TrimRight(operatorHostname, "/")), nil
}

// emailChangeExpiry is the owner's lifetime of an operator e-mail change
// link; the root reports when the owner raised the configured value.
func emailChangeExpiry(passwordResetExpiry time.Duration, logger *slog.Logger) time.Duration {
	expiry := identityaccess.OperatorEmailChangeExpiry(passwordResetExpiry)
	if expiry != passwordResetExpiry {
		logger.Warn("email change token expiry bumped to minimum 15 minutes",
			slog.Int("configured_minutes", int(passwordResetExpiry.Minutes())),
			slog.Int("effective_minutes", int(expiry.Minutes())),
		)
	}
	return expiry
}

// identityMailRenderers render the Identity & Access mail kinds of the
// Delivery outbox: the guardian invitation, the guardian welcome and the
// staff welcome. A welcome waits for the mail it follows; whether it may go
// out is Identity's rule, the root only reports Delivery's state.
func identityMailRenderers(module *identityaccess.Module, m IdentityMaterial, logger *slog.Logger) map[string]deliveryCompose.MailRenderer {
	return map[string]deliveryCompose.MailRenderer{
		identityaccess.MailKindGuardianInvitation: payloadRenderer(NewGuardianInvitationRenderer(GuardianInvitationRendererConfig{
			DefaultFrom: m.DefaultFrom,
		})),
		identityaccess.MailKindGuardianWelcome: payloadRenderer(NewGuardianWelcomeRenderer(GuardianInvitationRendererConfig{
			DefaultFrom:          m.DefaultFrom,
			PrecedingEmailStatus: identityaccessCompose.NewGuardianWelcomeOrder(precedingMails{status: m.EmailStatus}),
		})),
		identityaccess.MailKindStaffWelcome: NewStaffWelcomeRenderer(StaffWelcomeRendererConfig{
			DefaultFrom: m.DefaultFrom, MailIdentity: m.MailIdentity, Logger: logger,
			InvitationDeliverySent: module.SchoolInvitationDeliverySent,
		}),
	}
}

// payloadRenderer renders a queued mail from its payload alone.
func payloadRenderer(render func(context.Context, map[string]any) (*email.Message, error)) deliveryCompose.MailRendererFunc {
	return func(ctx context.Context, intent *deliveryCompose.MailIntent) (*email.Message, error) {
		return render(ctx, intent.Payload)
	}
}

// precedingMails reports Delivery's state of a queued mail in Identity's
// terms (#4006).
type precedingMails struct {
	status func(ctx context.Context, tenantID, id int64) (deliveryModule.State, bool, error)
}

func (p precedingMails) PrecedingMailState(ctx context.Context, tenantID, outboxID int64) (identityaccessCompose.PrecedingMailState, bool, error) {
	state, found, err := p.status(ctx, tenantID, outboxID)
	if err != nil || !found {
		return identityaccessCompose.PrecedingMailQueued, found, err
	}
	switch state {
	case deliveryModule.StateSent:
		return identityaccessCompose.PrecedingMailSent, true, nil
	case deliveryModule.StateCancelled:
		return identityaccessCompose.PrecedingMailCancelled, true, nil
	case deliveryModule.StateDeadLetter:
		return identityaccessCompose.PrecedingMailUndeliverable, true, nil
	default:
		return identityaccessCompose.PrecedingMailQueued, true, nil
	}
}

// NewAuthCleanupService composes the expired session sweep and the password
// reset maintenance the cleanup CLI runs (#3251, #3332). The sweep never
// signs or verifies a session, so it is composed without a signer.
func NewAuthCleanupService(db transactionruntime.Database, logger *slog.Logger, command auditModels.Command) (*identityaccessCompose.SessionMaintenance, error) {
	// The sweep resolves the school and person facts through their owners
	// and clears the push subscriptions through Delivery (#3251, #4013).
	schools, err := NewOrganizationTenancy(db)
	if err != nil {
		return nil, fmt.Errorf("auth cleanup: compose organization tenancy: %w", err)
	}
	events := auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db.Pool()}).WithRoutedWrites(command).AuthEvent
	return identityaccessCompose.NewSessionMaintenance(identityaccessCompose.SessionMaintenanceDependencies{
		DB: db.Pool(), Runtime: db.UnitOfWork(),
		Schools: schoolDirectory{schools: schools}, Persons: personDirectory{persons: NewPersonRepository(db)},
		Audit: authAudit{command: command, events: events}, Push: pushSubscriptionCleanup{subscriptions: deliveryCompose.NewPushSubscriptionRepository(db.Pool())},
		// The cleanup root only removes spent links and stale windows; it
		// never issues a link.
		ResetExpiry: cleanupResetExpiry, Logger: logger,
	})
}

// lateMailOutbox reads the outbox of the retained graph at call time, which
// composes Delivery after Identity & Access. It goes with that graph.
type lateMailOutbox struct{ current func() MailResultOutbox }

func (o lateMailOutbox) EnqueueOutbox(ctx context.Context, req deliveryCompose.MailEnqueueRequest) error {
	return o.current().EnqueueOutbox(ctx, req)
}

func (o lateMailOutbox) EnqueueOutboxWithResult(ctx context.Context, req deliveryCompose.MailEnqueueRequest) (deliveryCompose.MailEnqueued, error) {
	return o.current().EnqueueOutboxWithResult(ctx, req)
}
