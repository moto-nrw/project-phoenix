//go:build testhooks

package api

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type AuthTestModule struct {
	// Auth and AccountAuthentication are the same composed Identity &
	// Access module: the surfaces the behaviour tests drive consume it
	// through the capabilities they were bound to (#3364).
	Auth                  identityaccess.Owner
	AccountAuthentication identityaccess.Owner
	DemoAccess            *identityaccess.DemoAccess
	Invitation            InvitationCapability
	GuardianInvitation    GuardianInvitationCapability
	// MFA, Passkeys, OperatorMFA and OperatorPasskeys are the Identity &
	// Access second factor and both portals' WebAuthn ceremonies (#3331),
	// composed over the same repositories the root uses.
	MFA              identityaccess.AccountMFA
	Passkeys         identityaccess.AccountPasskeyFlows
	OperatorMFA      identityaccess.OperatorMFAFlows
	OperatorPasskeys identityaccess.OperatorPasskeyFlows
	// SessionSigning lets a behaviour test mint the tokens the flows expect;
	// the rows the flows wrote are read through
	// MustNewMembershipTestRepositories, the schools through
	// NewOrganizationTestRows and the settings through NewAuthTestSettings
	// over the same database (#4080).
	SessionSigning AuthTestSessionSigning
}

// NewAuthTestSettings are the tenant settings the composed module reads,
// over db and unit.
func NewAuthTestSettings(db transactionruntime.Database, unit tenant.UnitOfWork) settingsCompose.SettingsService {
	settings, err := owners.NewSettingsTestModule(db, unit)
	if err != nil {
		panic(fmt.Sprintf("auth test settings: %v", err))
	}
	return settings.Settings
}

// AuthTestSessionSigning is the signing configuration of the composed
// module's session tokens. A behaviour test builds Identity's session token
// adapter over it to mint and read the tokens the flows issue.
type AuthTestSessionSigning struct {
	Secret        string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

type InvitationTestModule struct {
	Invitation InvitationCapability
	// Auth is the composed Identity & Access module, for the invitation
	// HTTP tests that mount the whole auth router and therefore need the
	// account-lifecycle capability its unrelated routes bind (#3364).
	Auth identityaccess.Owner
}

func NewInvitationTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (InvitationTestModule, error) {
	auth, err := NewAuthTestModule(db, unit)
	if err != nil {
		return InvitationTestModule{}, err
	}
	return InvitationTestModule{Invitation: auth.Invitation, Auth: auth.Auth}, nil
}

// AuthTestOption overrides a default of the composed test module: the
// Identity & Access settings or the mail transport.
type AuthTestOption func(*authTestSettings, *authTestMail)

// authTestMail is the mail transport the composed module sends through.
type authTestMail struct {
	outbox owners.DurableMailOutbox
	mailer email.Mailer
}

type authTestSettings struct {
	invitationResetClock func() time.Time
	rateLimitEnabled     bool
	resetBackoff         []time.Duration
	mfaBackoff           []time.Duration
	mfaCapability        identityaccess.AccountMFA
	staffCreateErr       error
	standingDemo         string
	// demoMaxActiveSchools is the demo capacity; the default leaves room
	// for every demo school the tests of a package queue.
	demoMaxActiveSchools int
	// demoOperatorWithoutSecondFactor composes the operator login the way
	// APP_ENV=demo does (#3460).
	demoOperatorWithoutSecondFactor bool
}

// WithDemoOperatorWithoutSecondFactor composes the operator login of the
// demo environment, which asks for no second factor (#3460).
func WithDemoOperatorWithoutSecondFactor() AuthTestOption {
	return func(settings *authTestSettings, _ *authTestMail) { settings.demoOperatorWithoutSecondFactor = true }
}

// WithDemoMaxActiveSchools composes the demo access with this capacity.
func WithDemoMaxActiveSchools(capacity int) AuthTestOption {
	return func(settings *authTestSettings, _ *authTestMail) { settings.demoMaxActiveSchools = capacity }
}

// WithStandingDemoSchool composes the demo access with the fallback of #3463:
// every access enters the school with this slug instead of its own.
func WithStandingDemoSchool(slug string) AuthTestOption {
	return func(settings *authTestSettings, _ *authTestMail) { settings.standingDemo = slug }
}

// WithAuthTestMailer composes the module on the given mailer, so a test can
// read the mails the flows send.
func WithAuthTestMailer(mailer email.Mailer) AuthTestOption {
	return func(_ *authTestSettings, mail *authTestMail) { mail.mailer = mailer }
}

// WithAuthTestTransactionalOutbox lets transaction tests drive the real
// enqueuer without automatically running the fixture worker after commit.
func WithAuthTestTransactionalOutbox(outbox owners.DurableMailOutbox) AuthTestOption {
	return func(_ *authTestSettings, mail *authTestMail) { mail.outbox = outbox }
}

// WithAuthTestPasswordResetRateLimit enables the per-address reset rate
// limit, which the test configuration leaves off.
func WithAuthTestPasswordResetRateLimit(enabled bool) AuthTestOption {
	return func(s *authTestSettings, _ *authTestMail) { s.rateLimitEnabled = enabled }
}

// WithAuthTestStaffCreateFailure composes the module with a staff directory
// whose insert fails, so a test can prove that a failure inside the identity
// chain rolls the whole acceptance back.
func WithAuthTestStaffCreateFailure(err error) AuthTestOption {
	return func(s *authTestSettings, _ *authTestMail) { s.staffCreateErr = err }
}

// WithAuthTestPasswordResetBackoff shortens the retry spacing of the reset
// mail, so a test that asserts the recorded failure waits milliseconds
// instead of the production twenty seconds.
func WithAuthTestPasswordResetBackoff(backoff ...time.Duration) AuthTestOption {
	return func(s *authTestSettings, _ *authTestMail) { s.resetBackoff = backoff }
}

// WithAuthTestMFABackoff shortens the retry spacing of the MFA mails, so a
// test that asserts a failed synchronous send waits milliseconds instead of
// the production twenty seconds.
func WithAuthTestMFABackoff(backoff ...time.Duration) AuthTestOption {
	return func(s *authTestSettings, _ *authTestMail) { s.mfaBackoff = backoff }
}

// WithAuthTestMFACapability composes the module over the given account
// second factor instead of the one over the repositories, so a test can
// drive the login branches the gate decides between.
func WithAuthTestMFACapability(capability identityaccess.AccountMFA) AuthTestOption {
	return func(s *authTestSettings, _ *authTestMail) { s.mfaCapability = capability }
}

// authTestRelyingParty is the passkey relying party the composed test module
// runs its ceremonies under. The localhost value makes the module follow the
// request's own origin, so every school subdomain works.
var authTestRelyingParty = identityPorts.PasskeyDependencies{
	RPID: "localhost", RPName: "moto",
	TenantDomain: "localhost", OperatorFrontendURL: "http://operator.localhost",
}

func NewAuthTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, options ...AuthTestOption) (AuthTestModule, error) {
	settings, err := owners.NewSettingsTestModule(db, unit)
	if err != nil {
		return AuthTestModule{}, err
	}
	logger := slog.Default()
	command, err := auditCompose.NewCommand(owners.NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return AuthTestModule{}, err
	}
	members, err := owners.NewMembershipTestRepositories(db)
	if err != nil {
		return AuthTestModule{}, err
	}
	r := identityChainOf(db, members)
	parentPortal, err := NewAuthTestParentPortal(db)
	if err != nil {
		return AuthTestModule{}, err
	}
	organizations, err := owners.NewOrganizationTenancy(db)
	if err != nil {
		return AuthTestModule{}, err
	}
	// The auth events append through the command; the operator second factor
	// appends to the operator ledger (#3331), so the module needs it bound.
	audits := auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db.Pool()})
	audits = audits.WithRoutedWrites(command)
	operatorAuditLog := owners.NewOperatorAuditLogFromEntries(audits.OperatorAuditLog)
	cfg := CurrentServiceConfig()
	settingsOverrides := authTestSettings{rateLimitEnabled: cfg.RateLimitEnabled, demoMaxActiveSchools: 300}
	fallbackMailer, err := deliveryCompose.NewMailTransport(email.MailerConfig{AppEnv: "test"})
	if err != nil {
		return AuthTestModule{}, err
	}
	mail := authTestMail{mailer: fallbackMailer}
	for _, option := range options {
		option(&settingsOverrides, &mail)
	}
	mailer := mail.mailer
	dispatcher := deliveryCompose.NewMailDispatcher(mailer, logger)
	defaultFrom := email.NewEmail(cfg.EmailFromName, cfg.EmailFromAddress)
	if defaultFrom.Address == "" {
		defaultFrom = email.NewEmail("moto", "no-reply@moto.local")
	}
	frontendURL := strings.TrimRight(cfg.FrontendURL, "/")
	parentsURL := strings.TrimRight(cfg.ParentsURL, "/")
	schoolURL := strings.TrimRight(cfg.SchoolURL, "/")
	resetMinutes := cfg.PasswordResetExpiryMinutes
	if resetMinutes <= 0 {
		resetMinutes = 30
	} else if resetMinutes > 1440 {
		resetMinutes = 1440
	}
	inviteHours := cfg.InvitationTokenExpiryHours
	if inviteHours <= 0 {
		inviteHours = 48
	} else if inviteHours > 168 {
		inviteHours = 168
	}
	codec, err := NewConfiguredSessionTokens(cfg.JWTSecret, cfg.JWTExpiry, cfg.JWTRefreshExpiry)
	if err != nil {
		return AuthTestModule{}, err
	}
	// Delivery is composed before Identity & Access, whose mails enqueue
	// into its outbox (ADR 0059).
	deliveryModule, err := NewDeliveryTestModule(db, unit)
	if err != nil {
		return AuthTestModule{}, err
	}
	var identityAccess identityaccess.Owner
	durable := newAuthFixtureOutbox(owners.OutboxEnqueuer{Outbox: deliveryModule.EmailOutbox, Runtime: deliveryModule.Runtime}, deliveryModule.Runtime, mailer, logger, func() identityaccess.Owner { return identityAccess })
	if mail.outbox != nil {
		durable = mail.outbox
	}
	identity := owners.TenantMailIdentity(organizations, settings.Settings, owners.RootEnvironment{Logger: logger})
	sessionRepos := owners.SessionRepositoriesOf(r, organizations, audits.AuthEvent, deliveryCompose.NewPushSubscriptionRepository(db.Pool()))
	if settingsOverrides.staffCreateErr != nil {
		sessionRepos.Lifecycle.Membership = failingStaffMembership{LifecycleMembership: r.Membership, err: settingsOverrides.staffCreateErr}
	}
	// The operator flows are composed here too: the operator second factor
	// and both portals' passkey ceremonies (#3331) mint their sessions
	// through them, so a test module without them would compose a module
	// production never builds.
	operatorOrganizations, err := owners.NewOrganizationTenancy(db)
	if err != nil {
		return AuthTestModule{}, err
	}
	operatorPersons, err := owners.NewPeopleDirectory(db)
	if err != nil {
		return AuthTestModule{}, err
	}
	operatorMembership, err := owners.NewSchoolMembership(db)
	if err != nil {
		return AuthTestModule{}, err
	}
	operators, err := owners.NewOperatorDependencies(owners.OperatorRepositoriesOf(r, operatorAuditLog), operatorOrganizations, operatorPersons, operatorMembership, logger)
	if err != nil {
		return AuthTestModule{}, err
	}
	mfaMail, err := owners.NewMFAMailer(db, deliveryModule.Runtime, mailer, defaultFrom, frontendURL, logger, settingsOverrides.mfaBackoff)
	if err != nil {
		return AuthTestModule{}, err
	}
	mfa, err := owners.NewMFADependencies(codec, settings.Settings, mfaMail, operatorAuditLog, mfaTestSecret(), &authTestRelyingParty, logger)
	if err != nil {
		return AuthTestModule{}, err
	}
	if mail.outbox == nil {
		source := mfa.Mail.(owners.MFAMailer)
		mfa.Mail = fixtureMFAMail(func() (owners.MFAMailer, *deliveryCompose.Runtime, *slog.Logger) {
			return source, deliveryModule.Runtime, logger
		})
	}
	mfa.Capability = settingsOverrides.mfaCapability
	guardianMail, enrollmentClaims, err := owners.GuardianInvitationDependencies(settings.Settings, organizations, owners.OutboxEnqueuer{Outbox: deliveryModule.EmailOutbox},
		parentPortal.ParentEnrollmentRequest, parentsURL, logger)
	if err != nil {
		return AuthTestModule{}, err
	}
	lifecycle, err := owners.LifecycleDependencies(sessionRepos.Lifecycle, command, owners.CaregiverProfiles{Persons: operatorPersons, Membership: operatorMembership},
		guardianMail, enrollmentClaims, logger)
	if err != nil {
		return AuthTestModule{}, err
	}
	demo, err := owners.DemoDependencies(&owners.DemoAccessWiring{
		Dispatcher: dispatcher, DefaultFrom: defaultFrom, Durable: durable, FrontendURL: frontendURL,
		Logger: logger, Backoff: settingsOverrides.resetBackoff, MaxActiveSchools: settingsOverrides.demoMaxActiveSchools,
		OperatorWithoutSecondFactor: settingsOverrides.demoOperatorWithoutSecondFactor,
		NewToken:                    requestTokenReads.NewCapabilityToken, Fingerprint: requestTokenReads.CapabilityFingerprint,
	}, settingsOverrides.standingDemo, organizations)
	if err != nil {
		return AuthTestModule{}, err
	}
	invitationDependencies := owners.InvitationDependencies(&owners.InvitationWiring{
		Dispatcher: dispatcher, DefaultFrom: defaultFrom, Durable: durable, StaffURL: frontendURL, SchoolURL: schoolURL,
		MailIdentity: identity, Expiry: time.Duration(inviteHours) * time.Hour,
		Backoff: settingsOverrides.resetBackoff, Settings: settings.Settings,
		Outbox:       owners.OutboxEnqueuer{Outbox: deliveryModule.EmailOutbox},
		WelcomeDelay: identityaccess.StaffWelcomeDelay,
	}, codec, logger)
	invitationDependencies.EmploymentReset, err = authTestInvitationReset(db, members, operatorPersons, operatorMembership, audits, settings.Settings, settingsOverrides.invitationResetClock)
	if err != nil {
		return AuthTestModule{}, err
	}
	identityAccess, err = owners.NewIdentityAccessWithSessions(identityaccessCompose.Dependencies{
		DB: db.Pool(), Observe: func(identityaccessCompose.Observation) {},
		MFA: mfa, Lifecycle: lifecycle, Operators: operators, Demo: demo, TenantRuntime: unit,
		Resets: owners.PasswordResetDependencies(&owners.PasswordResetWiring{
			Dispatcher: dispatcher, DefaultFrom: defaultFrom, Durable: durable,
			StaffURL: frontendURL, ParentsURL: parentsURL, SchoolURL: schoolURL,
			Expiry: time.Duration(resetMinutes) * time.Minute, RateLimitEnabled: settingsOverrides.rateLimitEnabled,
			Backoff: settingsOverrides.resetBackoff,
		}, logger),
		Invitations: invitationDependencies,
	}, sessionRepos, codec, settings.Settings, command, logger)
	if err != nil {
		return AuthTestModule{}, err
	}
	invitation := InvitationCapability(identityAccess)
	guardian := GuardianInvitationCapability(identityAccess)
	return AuthTestModule{
		Auth: identityAccess, AccountAuthentication: identityAccess,
		DemoAccess: identityAccess.DemoAccess(),
		MFA:        identityAccess, Passkeys: identityAccess,
		OperatorMFA: identityAccess, OperatorPasskeys: identityAccess,
		SessionSigning: AuthTestSessionSigning{Secret: cfg.JWTSecret, AccessExpiry: cfg.JWTExpiry, RefreshExpiry: cfg.JWTRefreshExpiry},
		Invitation:     invitation, GuardianInvitation: guardian,
	}, nil
}

// IdentityAccessForTests composes the Identity & Access module over the
// identity chain rows of db the way the service graph does it, for behaviour tests
// that exercise sessions, the account lifecycle or the role administration
// without the whole factory (#3364).
func IdentityAccessForTests(cfg IdentityAccessTestConfig, unit tenant.UnitOfWork, audit auditModels.Command, db transactionruntime.Database, logger *slog.Logger) (identityaccess.Owner, error) {
	if logger == nil {
		logger = slog.Default()
	}
	codec := cfg.SessionTokens
	if codec == nil {
		configured, err := newLegacyTokenAdapter()
		if err != nil {
			return nil, err
		}
		codec = configured.SessionTokens
	}
	caregivers, err := caregiverProfilesForTests(db)
	if err != nil {
		return nil, err
	}
	organizations, err := owners.NewOrganizationTenancy(db)
	if err != nil {
		return nil, err
	}
	repos, err := authTestIdentityChain(db)
	if err != nil {
		return nil, err
	}
	audits := auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db.Pool()})
	settings, err := owners.NewSettingsTestModule(db, unit)
	if err != nil {
		return nil, err
	}
	mfa, err := owners.NewMFADependencies(codec, settings.Settings, owners.MFAMailer{
		FrontendURL: cfg.FrontendURL, Logger: logger,
	}, owners.NewOperatorAuditLogFromEntries(audits.OperatorAuditLog), mfaTestSecret(), nil, logger)
	if err != nil {
		return nil, err
	}
	sessionRepos := owners.SessionRepositoriesOf(repos, organizations, audits.AuthEvent, deliveryCompose.NewPushSubscriptionRepository(db.Pool()))
	lifecycle, err := owners.LifecycleDependencies(sessionRepos.Lifecycle, audit, caregivers, owners.UnsentGuardianMail{}, nil, logger)
	if err != nil {
		return nil, err
	}
	module, err := owners.NewIdentityAccessWithSessions(identityaccessCompose.Dependencies{
		DB: db.Pool(), Observe: func(identityaccessCompose.Observation) {}, MFA: mfa, Lifecycle: lifecycle, TenantRuntime: unit,
		Resets: owners.PasswordResetDependencies(&owners.PasswordResetWiring{
			StaffURL: cfg.FrontendURL, ParentsURL: cfg.ParentsURL, SchoolURL: cfg.SchoolURL,
			Expiry: cfg.PasswordResetExpiry, RateLimitEnabled: cfg.RateLimitEnabled,
		}, logger),
	}, sessionRepos, codec, nil, audit, logger)
	if err != nil {
		return nil, err
	}
	return module, nil
}

// IdentityAccessTestConfig is what a behaviour test supplies to compose the
// module beside the tenant runtime and the audit ledger: the session tokens
// and the portal hosts. The module uses real registry-backed settings without
// a mail transport. Zero values leave optional seams unconfigured;
// nil session tokens sign with the configured secret.
type IdentityAccessTestConfig struct {
	SessionTokens       identityaccessCompose.SignedIdentityTokens
	FrontendURL         string
	ParentsURL          string
	SchoolURL           string
	PasswordResetExpiry time.Duration
	RateLimitEnabled    bool
}

// failingStaffMembership fails every staff insert; the caregiver profile
// insert is the real owner command.
type failingStaffMembership struct {
	owners.LifecycleMembership
	err error
}

func (d failingStaffMembership) CreateStaff(context.Context, schoolmembership.CreateStaff) (schoolmembership.Staff, error) {
	return schoolmembership.Staff{}, d.err
}

// mfaTestSecret is the signing secret the composed test module derives its
// trusted-device HMAC key from: the process configuration, exactly as the
// production root reads it, so no key is stored in source.
func mfaTestSecret() string { return CurrentServiceConfig().JWTSecret }
