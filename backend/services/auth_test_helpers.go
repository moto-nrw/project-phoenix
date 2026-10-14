package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailoutbox"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	platformModels "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose/vocabulary"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
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
	// Repos and TokenAuth let a behaviour test read the rows the flows
	// wrote and mint the tokens they expect.
	Repos     MembershipTestRepositories
	TokenAuth *authjwt.TokenAuth
}

type InvitationTestModule struct {
	Persistence *InvitationPersistence
	Invitation  InvitationCapability
	// Auth is the composed Identity & Access module, for the invitation
	// HTTP tests that mount the whole auth router and therefore need the
	// account-lifecycle capability its unrelated routes bind (#3364).
	Auth *identityaccess.Module
}

func NewInvitationTestModule(db *bun.DB, unit tenant.UnitOfWork) (InvitationTestModule, error) {
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
type AccountMFARecords = identityaccessCompose.AccountMFARecords

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

func NewAuthTestModule(db *bun.DB, unit tenant.UnitOfWork, options ...AuthTestOption) (AuthTestModule, error) {
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
	audits := auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db})
	audits = audits.WithRoutedWrites(command)
	operatorAuditLog := NewOperatorAuditLogFromEntries(audits.OperatorAuditLog)
	cfg := CurrentFactoryConfig()
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
	tokenAuth, err := authjwt.NewTokenAuthWithDurations(cfg.JWTSecret, cfg.JWTExpiry, cfg.JWTRefreshExpiry)
	if err != nil {
		return AuthTestModule{}, err
	}
	codec, err := SignedIdentityTokensOf(tokenAuth)
	if err != nil {
		return AuthTestModule{}, err
	}
	// The delivery module is composed after the identity module, so the
	// guardian mail reads its outbox at call time, as the factory does.
	var deliveryModule DeliveryTestModule
	identity := emailoutbox.NewTenantMailIdentity(SchoolContactDirectory{Schools: organizations}, func(ctx context.Context, tenantID int64) (string, error) {
		return settings.Settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
	}, logger)
	sessionRepos := SessionRepositoriesOf(r, organizations, audits.AuthEvent, deliveryCompose.NewPushSubscriptionRepository(db))
	if settingsOverrides.staffCreateErr != nil {
		sessionRepos.Lifecycle.Staff = failingStaffDirectory{StaffRepository: r.Staff, err: settingsOverrides.staffCreateErr}
	}
	// The operator flows are composed here too: the operator second factor
	// and both portals' passkey ceremonies (#3331) mint their sessions
	// through them, so a test module without them would compose a module
	// production never builds.
	owners, err := newOwnerCapabilitiesForTests(db)
	if err != nil {
		return AuthTestModule{}, err
	}
	operators, err := NewOperatorDependencies(OperatorAuthenticationWiring{
		Repos:         OperatorRepositoriesOf(r, operatorAuditLog),
		Organizations: owners.organizations,
		Persons:       owners.persons,
		Membership:    owners.membership,
		Logger:        logger,
	})
	if err != nil {
		return AuthTestModule{}, err
	}
	identityAccess, err := NewIdentityAccessWithSessions(identityaccessCompose.Dependencies{DB: db}, AccountAuthenticationWiring{
		Repos: sessionRepos, Codec: codec, Settings: settings.Settings, Audit: command, Logger: logger,
		Operators: operators,
		DemoAccess: &DemoAccessWiring{
			Dispatcher: dispatcher, DefaultFrom: defaultFrom, FrontendURL: frontendURL,
			Logger: logger, Backoff: settingsOverrides.resetBackoff, MaxActiveSchools: settingsOverrides.demoMaxActiveSchools,
			OperatorWithoutSecondFactor: settingsOverrides.demoOperatorWithoutSecondFactor,
			NewToken:                    testRequestTokens.NewCapabilityToken, Fingerprint: testRequestTokens.CapabilityFingerprint,
		},
		DemoStandingSchool: settingsOverrides.standingDemo,
		Mfa: &MfaWiring{
			OperatorAuditLog: operatorAuditLog, Settings: settings.Settings,
			Dispatcher: dispatcher, DefaultFrom: defaultFrom, FrontendURL: frontendURL,
			JwtSecret: mfaTestSecret(), Logger: logger, Backoff: settingsOverrides.mfaBackoff,
			Decorate: settingsOverrides.mfaRecords, Capability: settingsOverrides.mfaCapability,
			Passkeys: &authTestRelyingParty,
		},
		Lifecycle: &LifecycleWiring{
			Settings: settings.Settings, Audit: command,
			Caregivers: CaregiverProfiles{Persons: owners.persons, Membership: owners.membership},
			GuardianMail: &GuardianInvitationWiring{
				Settings: settings.Settings, Schools: organizations,
				Outbox:      func() platformModels.OutboxResultEnqueuer { return OutboxEnqueuer{Outbox: deliveryModule.EmailOutbox} },
				Enrollments: parentPortal.ParentEnrollmentRequest, ParentsURL: parentsURL,
				FallbackExpiry: time.Duration(inviteHours) * time.Hour, Logger: logger,
			},
		},
		Resets: &PasswordResetWiring{
			Dispatcher: dispatcher, DefaultFrom: defaultFrom,
			StaffURL: frontendURL, ParentsURL: parentsURL, SchoolURL: schoolURL,
			Expiry: time.Duration(resetMinutes) * time.Minute, RateLimitEnabled: settingsOverrides.rateLimitEnabled,
			Backoff: settingsOverrides.resetBackoff,
		},
		Invitations: &InvitationWiring{
			Dispatcher: dispatcher, DefaultFrom: defaultFrom, StaffURL: frontendURL, SchoolURL: schoolURL,
			MailIdentity: identity, Expiry: time.Duration(inviteHours) * time.Hour,
			Backoff: settingsOverrides.resetBackoff, Settings: settings.Settings,
			Outbox:       func() platformModels.OutboxEnqueuer { return OutboxEnqueuer{Outbox: deliveryModule.EmailOutbox} },
			WelcomeDelay: StaffWelcomeDelay,
		},
	})
	if err != nil {
		return AuthTestModule{}, err
	}
	if err := identityAccess.SetTenantRuntime(unit); err != nil {
		return AuthTestModule{}, err
	}
	invitation := InvitationCapability(identityAccess)
	deliveryModule, err = NewDeliveryTestModule(db, unit)
	if err != nil {
		return AuthTestModule{}, err
	}
	guardian := GuardianInvitationCapability(identityAccess)
	return AuthTestModule{
		Auth: identityAccess, AccountAuthentication: identityAccess,
		DemoAccess: identityAccess.DemoAccess(),
		MFA:        identityAccess, Passkeys: identityAccess,
		OperatorMFA: identityAccess, OperatorPasskeys: identityAccess,
		Repos: members, TokenAuth: tokenAuth,
		Invitation: invitation, GuardianInvitation: guardian,
		Schools: organizations, Settings: settings.Settings,
	}, nil
}

// IdentityAccessForTests composes the Identity & Access module over the
// identity chain rows of db the way the factory does it, for behaviour tests
// that exercise sessions, the account lifecycle or the role administration
// without the whole factory (#3364).
func IdentityAccessForTests(cfg IdentityAccessTestConfig, db *bun.DB, logger *slog.Logger) (*identityaccess.Module, error) {
	if logger == nil {
		logger = slog.Default()
	}
	signer := cfg.TokenAuth
	if signer == nil {
		created, err := ConfiguredTokenAuth()
		if err != nil {
			return nil, err
		}
		signer = created
	}
	codec, err := SignedIdentityTokensOf(signer)
	if err != nil {
		return nil, err
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
	audits := auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db})
	module, err := NewIdentityAccessWithSessions(identityaccessCompose.Dependencies{DB: db}, AccountAuthenticationWiring{
		Repos: SessionRepositoriesOf(repos, organizations, audits.AuthEvent, deliveryCompose.NewPushSubscriptionRepository(db)), Codec: codec, Settings: cfg.Settings,
		Audit: cfg.Audit, Logger: logger,
		Mfa: &MfaWiring{
			OperatorAuditLog: NewOperatorAuditLogFromEntries(audits.OperatorAuditLog), Settings: cfg.Settings,
			Dispatcher: cfg.Dispatcher, DefaultFrom: cfg.DefaultFrom, FrontendURL: cfg.FrontendURL,
			JwtSecret: mfaTestSecret(), Logger: logger,
		},
		Lifecycle: &LifecycleWiring{Settings: cfg.Settings, Audit: cfg.Audit, Caregivers: caregivers},
		Resets: &PasswordResetWiring{
			Dispatcher: cfg.Dispatcher, DefaultFrom: cfg.DefaultFrom,
			StaffURL: cfg.FrontendURL, ParentsURL: cfg.ParentsURL, SchoolURL: cfg.SchoolURL,
			Expiry: cfg.PasswordResetExpiry, RateLimitEnabled: cfg.RateLimitEnabled,
		},
	})
	if err != nil {
		return nil, err
	}
	if err := module.SetTenantRuntime(cfg.TenantRuntime); err != nil {
		return nil, err
	}
	return module, nil
}

// IdentityAccessTestConfig is what a behaviour test supplies to compose the
// module: the signer, the mail transport, the portal hosts and the audit
// ledger. Every zero value composes the module without that seam.
type IdentityAccessTestConfig struct {
	TokenAuth           *authjwt.TokenAuth
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

// failingStaffDirectory fails every staff insert; every other operation is
// the real repository.
type failingStaffDirectory struct {
	userModels.StaffRepository
	err error
}

func (d failingStaffDirectory) Create(context.Context, *userModels.Staff) error { return d.err }

// mfaTestSecret is the signing secret the composed test module derives its
// trusted-device HMAC key from: the process configuration, exactly as the
// production root reads it, so no key is stored in source.
func mfaTestSecret() string { return CurrentFactoryConfig().JWTSecret }

// testRequestTokens are the request reads and capability tokens of the
// retained token adapter, bound the way the serving root binds them (api).
// The test compositions keep them here until the test hooks leave the legacy
// composition (#2747).
var testRequestTokens = TokenAdapter{
	Permissions:           authjwt.PermissionsFromCtx,
	Claims:                testRequestClaims,
	Memo:                  testRequestMemo,
	NewCapabilityToken:    authjwt.NewOpaqueCapabilityToken,
	CapabilityFingerprint: authjwt.OpaqueCapabilityFingerprint,
}

// testTokensFor binds testRequestTokens to the session signer cfg names.
func testTokensFor(cfg FactoryConfig) (TokenAdapter, error) {
	signer, err := authjwt.NewTokenAuthWithDurations(cfg.JWTSecret, cfg.JWTExpiry, cfg.JWTRefreshExpiry)
	if err != nil {
		return TokenAdapter{}, fmt.Errorf("invalid auth JWT configuration: %w", err)
	}
	codec, err := SignedIdentityTokensOf(signer)
	if err != nil {
		return TokenAdapter{}, err
	}
	tokens := testRequestTokens
	tokens.SessionTokens = codec
	return tokens, nil
}

func testRequestClaims(ctx context.Context) (RequestClaims, bool) {
	claims, ok := ctx.Value(authjwt.CtxClaims).(authjwt.AppClaims)
	if !ok {
		return RequestClaims{}, false
	}
	return RequestClaims{
		AccountID: int64(claims.ID), TenantID: claims.TenantID, Scope: claims.Scope,
		SchoolScope: claims.IsSchoolScope(), Admin: claims.IsAdmin,
		FirstName: claims.FirstName, LastName: claims.LastName, Username: claims.Username,
	}, true
}

func testRequestMemo(ctx context.Context) RequestMemo {
	if cache := authjwt.RequestIdentityCacheFrom(ctx); cache != nil {
		return cache
	}
	return nil
}
