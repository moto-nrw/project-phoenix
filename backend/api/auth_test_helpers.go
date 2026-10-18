//go:build testhooks

package api

import (
	"context"
	"log/slog"
	"strings"
	"time"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type AuthTestModule struct {
	// Auth and AccountAuthentication are the same composed Identity &
	// Access module: the surfaces the behaviour tests drive consume it
	// through the capabilities they were bound to (#3364).
	Auth                  *identityaccess.Module
	AccountAuthentication *identityaccess.Module
	DemoAccess            *identityaccess.DemoAccess
	Invitation            InvitationCapability
	GuardianInvitation    GuardianInvitationCapability
	Schools               organizationtenancy.Capability
	Settings              settingsCompose.SettingsService
	// MFA, Passkeys, OperatorMFA and OperatorPasskeys are the Identity &
	// Access second factor and both portals' WebAuthn ceremonies (#3331),
	// composed over the same repositories the root uses.
	MFA              identityaccess.AccountMFA
	Passkeys         identityaccess.AccountPasskeyFlows
	OperatorMFA      identityaccess.OperatorMFAFlows
	OperatorPasskeys identityaccess.OperatorPasskeyFlows
	// Repos and SessionSigning let a behaviour test read the rows the flows
	// wrote and mint the tokens they expect.
	Repos          MembershipTestRepositories
	SessionSigning AuthTestSessionSigning
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
	Persistence *InvitationPersistence
	Invitation  InvitationCapability
	// Auth is the composed Identity & Access module, for the invitation
	// HTTP tests that mount the whole auth router and therefore need the
	// account-lifecycle capability its unrelated routes bind (#3364).
	Auth *identityaccess.Module
}

func NewInvitationTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (InvitationTestModule, error) {
	repos, err := NewInvitationPersistence(db)
	if err != nil {
		return InvitationTestModule{}, err
	}
	auth, err := NewAuthTestModule(db, unit)
	if err != nil {
		return InvitationTestModule{}, err
	}
	return InvitationTestModule{Persistence: repos, Invitation: auth.Invitation, Auth: auth.Auth}, nil
}

// AccountMFARecords is the account MFA record port the composed module
// consumes, named here so a behaviour test can decorate it without reaching
// into the module's composition package.
type AccountMFARecords = identityPorts.AccountMFARecords

// AuthTestOption overrides a default of the composed test module.
type AuthTestOption func(*authTestSettings)

type authTestSettings struct {
	mailer           email.Mailer
	rateLimitEnabled bool
	resetBackoff     []time.Duration
	mfaBackoff       []time.Duration
	mfaRecords       func(AccountMFARecords) AccountMFARecords
	mfaCapability    identityaccess.AccountMFA
	staffCreateErr   error
	standingDemo     string
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
	return func(settings *authTestSettings) { settings.demoOperatorWithoutSecondFactor = true }
}

// WithDemoMaxActiveSchools composes the demo access with this capacity.
func WithDemoMaxActiveSchools(capacity int) AuthTestOption {
	return func(settings *authTestSettings) { settings.demoMaxActiveSchools = capacity }
}

// WithStandingDemoSchool composes the demo access with the fallback of #3463:
// every access enters the school with this slug instead of its own.
func WithStandingDemoSchool(slug string) AuthTestOption {
	return func(settings *authTestSettings) { settings.standingDemo = slug }
}

// WithAuthTestMailer composes the module on the given mailer, so a test can
// read the mails the flows send.
func WithAuthTestMailer(mailer email.Mailer) AuthTestOption {
	return func(s *authTestSettings) { s.mailer = mailer }
}

// WithAuthTestPasswordResetRateLimit enables the per-address reset rate
// limit, which the test configuration leaves off.
func WithAuthTestPasswordResetRateLimit(enabled bool) AuthTestOption {
	return func(s *authTestSettings) { s.rateLimitEnabled = enabled }
}

// WithAuthTestStaffCreateFailure composes the module with a staff directory
// whose insert fails, so a test can prove that a failure inside the identity
// chain rolls the whole acceptance back.
func WithAuthTestStaffCreateFailure(err error) AuthTestOption {
	return func(s *authTestSettings) { s.staffCreateErr = err }
}

// WithAuthTestPasswordResetBackoff shortens the retry spacing of the reset
// mail, so a test that asserts the recorded failure waits milliseconds
// instead of the production twenty seconds.
func WithAuthTestPasswordResetBackoff(backoff ...time.Duration) AuthTestOption {
	return func(s *authTestSettings) { s.resetBackoff = backoff }
}

// WithAuthTestMFABackoff shortens the retry spacing of the MFA mails, so a
// test that asserts a failed synchronous send waits milliseconds instead of
// the production twenty seconds.
func WithAuthTestMFABackoff(backoff ...time.Duration) AuthTestOption {
	return func(s *authTestSettings) { s.mfaBackoff = backoff }
}

// WithAuthTestMFARecords wraps the account MFA record port, so a test can
// make one statement fail and prove the flow refuses instead of degrading.
func WithAuthTestMFARecords(decorate func(AccountMFARecords) AccountMFARecords) AuthTestOption {
	return func(s *authTestSettings) { s.mfaRecords = decorate }
}

// WithAuthTestMFACapability composes the module over the given account
// second factor instead of the one over the repositories, so a test can
// drive the login branches the gate decides between.
func WithAuthTestMFACapability(capability identityaccess.AccountMFA) AuthTestOption {
	return func(s *authTestSettings) { s.mfaCapability = capability }
}

// authTestRelyingParty is the passkey relying party the composed test module
// runs its ceremonies under. The localhost value makes the module follow the
// request's own origin, so every school subdomain works.
var authTestRelyingParty = identityaccessCompose.PasskeyDependencies{
	RPID: "localhost", RPName: "moto",
	TenantDomain: "localhost", OperatorFrontendURL: "http://operator.localhost",
}

func NewAuthTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, options ...AuthTestOption) (AuthTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return AuthTestModule{}, err
	}
	logger := slog.Default()
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return AuthTestModule{}, err
	}
	members, err := NewMembershipTestRepositories(db)
	if err != nil {
		return AuthTestModule{}, err
	}
	r := identityChainOf(db, members)
	parentPortal, err := NewAuthTestParentPortal(db)
	if err != nil {
		return AuthTestModule{}, err
	}
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return AuthTestModule{}, err
	}
	// The auth events append through the command; the operator second factor
	// appends to the operator ledger (#3331), so the module needs it bound.
	audits := auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db.Pool()})
	audits = audits.WithRoutedWrites(command)
	operatorAuditLog := NewOperatorAuditLogFromEntries(audits.OperatorAuditLog)
	cfg := CurrentServiceConfig()
	settingsOverrides := authTestSettings{mailer: email.NewMockMailer(), rateLimitEnabled: cfg.RateLimitEnabled, demoMaxActiveSchools: 300}
	for _, option := range options {
		option(&settingsOverrides)
	}
	mailer := settingsOverrides.mailer
	dispatcher := email.NewDispatcher(mailer, logger)
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
	identity := deliveryCompose.NewTenantMailIdentity(SchoolContactDirectory{Schools: organizations}, func(ctx context.Context, tenantID int64) (string, error) {
		return settings.Settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
	}, logger)
	sessionRepos := SessionRepositoriesOf(r, organizations, audits.AuthEvent, deliveryCompose.NewPushSubscriptionRepository(db.Pool()))
	if settingsOverrides.staffCreateErr != nil {
		sessionRepos.Lifecycle.Membership = failingStaffMembership{lifecycleMembership: r.Membership, err: settingsOverrides.staffCreateErr}
	}
	// The operator flows are composed here too: the operator second factor
	// and both portals' passkey ceremonies (#3331) mint their sessions
	// through them, so a test module without them would compose a module
	// production never builds.
	operatorOrganizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return AuthTestModule{}, err
	}
	operatorPersons, err := NewPeopleDirectory(db)
	if err != nil {
		return AuthTestModule{}, err
	}
	operatorMembership, err := NewSchoolMembership(db)
	if err != nil {
		return AuthTestModule{}, err
	}
	operators, err := NewOperatorDependencies(OperatorRepositoriesOf(r, operatorAuditLog), operatorOrganizations, operatorPersons, operatorMembership, logger)
	if err != nil {
		return AuthTestModule{}, err
	}
	mfa, err := newMFADependencies(codec, settings.Settings, mfaMailer{
		dispatcher: dispatcher, from: defaultFrom, frontendURL: frontendURL, logger: logger, backoff: settingsOverrides.mfaBackoff,
	}, operatorAuditLog, mfaTestSecret(), &authTestRelyingParty, logger)
	if err != nil {
		return AuthTestModule{}, err
	}
	mfa.DecorateRecords, mfa.Capability = settingsOverrides.mfaRecords, settingsOverrides.mfaCapability
	guardianMail, enrollmentClaims, err := guardianInvitationDependencies(settings.Settings, organizations, OutboxEnqueuer{Outbox: deliveryModule.EmailOutbox},
		parentPortal.ParentEnrollmentRequest, parentsURL, time.Duration(inviteHours)*time.Hour, logger)
	if err != nil {
		return AuthTestModule{}, err
	}
	lifecycle, err := lifecycleDependencies(sessionRepos.Lifecycle, command, CaregiverProfiles{Persons: operatorPersons, Membership: operatorMembership},
		guardianMail, enrollmentClaims, logger)
	if err != nil {
		return AuthTestModule{}, err
	}
	demo, err := demoDependencies(&DemoAccessWiring{
		Dispatcher: dispatcher, DefaultFrom: defaultFrom, FrontendURL: frontendURL,
		Logger: logger, Backoff: settingsOverrides.resetBackoff, MaxActiveSchools: settingsOverrides.demoMaxActiveSchools,
		OperatorWithoutSecondFactor: settingsOverrides.demoOperatorWithoutSecondFactor,
		NewToken:                    requestTokenReads.NewCapabilityToken, Fingerprint: requestTokenReads.CapabilityFingerprint,
	}, settingsOverrides.standingDemo, organizations)
	if err != nil {
		return AuthTestModule{}, err
	}
	identityAccess, err := NewIdentityAccessWithSessions(identityaccessCompose.Dependencies{
		DB: db.Pool(), Observe: func(identityaccessCompose.Observation) {},
		MFA: mfa, Lifecycle: lifecycle, Operators: operators, Demo: demo,
		Resets: passwordResetDependencies(&PasswordResetWiring{
			Dispatcher: dispatcher, DefaultFrom: defaultFrom,
			StaffURL: frontendURL, ParentsURL: parentsURL, SchoolURL: schoolURL,
			Expiry: time.Duration(resetMinutes) * time.Minute, RateLimitEnabled: settingsOverrides.rateLimitEnabled,
			Backoff: settingsOverrides.resetBackoff,
		}, logger),
		Invitations: invitationDependencies(&InvitationWiring{
			Dispatcher: dispatcher, DefaultFrom: defaultFrom, StaffURL: frontendURL, SchoolURL: schoolURL,
			MailIdentity: identity, Expiry: time.Duration(inviteHours) * time.Hour,
			Backoff: settingsOverrides.resetBackoff, Settings: settings.Settings,
			Outbox:       OutboxEnqueuer{Outbox: deliveryModule.EmailOutbox},
			WelcomeDelay: identityaccess.StaffWelcomeDelay,
		}, codec, logger),
	}, sessionRepos, codec, settings.Settings, command, logger)
	if err != nil {
		return AuthTestModule{}, err
	}
	if err := identityAccess.SetTenantRuntime(unit); err != nil {
		return AuthTestModule{}, err
	}
	invitation := InvitationCapability(identityAccess)
	guardian := GuardianInvitationCapability(identityAccess)
	return AuthTestModule{
		Auth: identityAccess, AccountAuthentication: identityAccess,
		DemoAccess: identityAccess.DemoAccess(),
		MFA:        identityAccess, Passkeys: identityAccess,
		OperatorMFA: identityAccess, OperatorPasskeys: identityAccess,
		Repos:          members,
		SessionSigning: AuthTestSessionSigning{Secret: cfg.JWTSecret, AccessExpiry: cfg.JWTExpiry, RefreshExpiry: cfg.JWTRefreshExpiry},
		Invitation:     invitation, GuardianInvitation: guardian,
		Schools: organizations, Settings: settings.Settings,
	}, nil
}

// IdentityAccessForTests composes the Identity & Access module over the
// identity chain rows of db the way the service graph does it, for behaviour tests
// that exercise sessions, the account lifecycle or the role administration
// without the whole factory (#3364).
func IdentityAccessForTests(cfg IdentityAccessTestConfig, db transactionruntime.Database, logger *slog.Logger) (*identityaccess.Module, error) {
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
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return nil, err
	}
	repos, err := authTestIdentityChain(db)
	if err != nil {
		return nil, err
	}
	audits := auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db.Pool()})
	mfa, err := newMFADependencies(codec, cfg.Settings, mfaMailer{
		dispatcher: cfg.Dispatcher, from: cfg.DefaultFrom, frontendURL: cfg.FrontendURL, logger: logger,
	}, NewOperatorAuditLogFromEntries(audits.OperatorAuditLog), mfaTestSecret(), nil, logger)
	if err != nil {
		return nil, err
	}
	sessionRepos := SessionRepositoriesOf(repos, organizations, audits.AuthEvent, deliveryCompose.NewPushSubscriptionRepository(db.Pool()))
	lifecycle, err := lifecycleDependencies(sessionRepos.Lifecycle, cfg.Audit, caregivers, unsentGuardianMail{}, nil, logger)
	if err != nil {
		return nil, err
	}
	module, err := NewIdentityAccessWithSessions(identityaccessCompose.Dependencies{
		DB: db.Pool(), Observe: func(identityaccessCompose.Observation) {}, MFA: mfa, Lifecycle: lifecycle,
		Resets: passwordResetDependencies(&PasswordResetWiring{
			Dispatcher: cfg.Dispatcher, DefaultFrom: cfg.DefaultFrom,
			StaffURL: cfg.FrontendURL, ParentsURL: cfg.ParentsURL, SchoolURL: cfg.SchoolURL,
			Expiry: cfg.PasswordResetExpiry, RateLimitEnabled: cfg.RateLimitEnabled,
		}, logger),
	}, sessionRepos, codec, cfg.Settings, cfg.Audit, logger)
	if err != nil {
		return nil, err
	}
	if err := module.SetTenantRuntime(cfg.TenantRuntime); err != nil {
		return nil, err
	}
	return module, nil
}

// IdentityAccessTestConfig is what a behaviour test supplies to compose the
// module: the session tokens, the mail transport, the portal hosts and the
// audit ledger. Every zero value composes the module without that seam; nil
// session tokens sign with the configured secret.
type IdentityAccessTestConfig struct {
	SessionTokens       identityaccessCompose.SignedIdentityTokens
	TenantRuntime       tenant.UnitOfWork
	Settings            settingsCompose.SettingsService
	Audit               auditModels.Command
	Dispatcher          *email.Dispatcher
	DefaultFrom         email.Email
	FrontendURL         string
	ParentsURL          string
	SchoolURL           string
	PasswordResetExpiry time.Duration
	RateLimitEnabled    bool
}

// failingStaffMembership fails every staff insert; the caregiver profile
// insert is the real owner command.
type failingStaffMembership struct {
	lifecycleMembership
	err error
}

func (d failingStaffMembership) CreateStaff(context.Context, schoolmembership.CreateStaff) (schoolmembership.Staff, error) {
	return schoolmembership.Staff{}, d.err
}

// mfaTestSecret is the signing secret the composed test module derives its
// trusted-device HMAC key from: the process configuration, exactly as the
// production root reads it, so no key is stored in source.
func mfaTestSecret() string { return CurrentServiceConfig().JWTSecret }

// TokenAdapterForTests binds the root's request reads to the session tokens of the
// signer cfg names.
func TokenAdapterForTests(cfg ServiceConfig) (TokenAdapter, error) {
	return tokenAdapterFor(cfg.JWTSecret, cfg.JWTExpiry, cfg.JWTRefreshExpiry)
}

// IdentityRootForTests is the root's Identity & Access composition a test
// hands the service graph, signing with the secret cfg names (#4006).
func IdentityRootForTests(db transactionruntime.Database, cfg ServiceConfig) (IdentityRoot, error) {
	tokens, err := TokenAdapterForTests(cfg)
	if err != nil {
		return IdentityRoot{}, err
	}
	wiring, _ := newIdentityRoot(db, tokens)
	return wiring, nil
}
