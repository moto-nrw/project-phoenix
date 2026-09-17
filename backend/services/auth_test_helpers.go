package services

import (
	"context"
	"log/slog"
	"strings"
	"time"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailoutbox"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type AuthTestModule struct {
	Auth auth.AuthService
	// AccountAuthentication is the Identity & Access module the auth
	// service delegates its session flows to (#3251); the auth HTTP tests
	// hand it to the routes that call the public contract directly.
	AccountAuthentication *identityaccess.Module
	StaffPINAuth          StaffPINAuthenticator
	Invitation            auth.InvitationService
	GuardianInvitation    auth.GuardianInvitationService
	Schools               organizationtenancy.Capability
	Settings              config.SettingsService
	// MFA, Passkeys, OperatorMFA and OperatorPasskeys are the Identity &
	// Access second factor and both portals' WebAuthn ceremonies (#3331),
	// composed over the same repositories the root uses.
	MFA              auth.MFAService
	Passkeys         auth.PasskeyService
	OperatorMFA      platform.OperatorMFAService
	OperatorPasskeys platform.OperatorPasskeyService
	// Repos and TokenAuth let a behaviour test read the rows the flows
	// wrote and mint the tokens they expect.
	Repos     *repositories.Factory
	TokenAuth *authjwt.TokenAuth
}

type InvitationTestModule struct {
	Persistence *repositories.InvitationPersistence
	Invitation  auth.InvitationService
}

func NewInvitationTestModule(db *bun.DB, unit tenant.UnitOfWork) (InvitationTestModule, error) {
	repos, err := repositories.NewInvitationPersistence(db)
	if err != nil {
		return InvitationTestModule{}, err
	}
	auth, err := NewAuthTestModule(db, unit)
	if err != nil {
		return InvitationTestModule{}, err
	}
	return InvitationTestModule{Persistence: repos, Invitation: auth.Invitation}, nil
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
	mfaCapability    auth.MFAService
	mfaSettings      config.SettingsService
	staffCreateErr   error
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

// WithAuthTestMFASettings resolves the MFA gate's school settings through
// the given service, so a test can drive security.mfa_mode and the
// trusted-device values without touching the rest of the composition.
func WithAuthTestMFASettings(settings config.SettingsService) AuthTestOption {
	return func(s *authTestSettings) { s.mfaSettings = settings }
}

// WithAuthTestMFACapability composes the module over the given account
// second factor instead of the one over the repositories, so a test can
// drive the login branches the gate decides between.
func WithAuthTestMFACapability(capability auth.MFAService) AuthTestOption {
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
	command, err := auditSvc.NewCommand(repositories.NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
	if err != nil {
		return AuthTestModule{}, err
	}
	r, err := repositories.NewAuthTestRepositories(db, command)
	if err != nil {
		return AuthTestModule{}, err
	}
	cfg := currentFactoryConfig()
	settingsOverrides := authTestSettings{mailer: email.NewMockMailer(), rateLimitEnabled: cfg.RateLimitEnabled}
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
	authConfig, err := auth.NewServiceConfig(dispatcher, defaultFrom, frontendURL, time.Duration(resetMinutes)*time.Minute)
	if err != nil {
		return AuthTestModule{}, err
	}
	authConfig.ParentsURL = parentsURL
	authConfig.SchoolURL = schoolURL
	authConfig.RateLimitEnabled = settingsOverrides.rateLimitEnabled
	authConfig.Settings = settings.Settings
	authConfig.Audit = command
	authConfig.TokenAuth, err = authjwt.NewTokenAuthWithDurations(cfg.JWTSecret, cfg.JWTExpiry, cfg.JWTRefreshExpiry)
	if err != nil {
		return AuthTestModule{}, err
	}
	var service *auth.Service
	// The delivery module is composed after the identity module, so the
	// guardian mail reads its outbox at call time, as the factory does.
	var deliveryModule DeliveryTestModule
	identity := emailoutbox.NewTenantMailIdentity(schoolContactDirectory{schools: r.School}, func(ctx context.Context, tenantID int64) (string, error) {
		return settings.Settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
	}, logger)
	sessionRepos := sessionRepositoriesOf(r, r.School)
	if settingsOverrides.staffCreateErr != nil {
		sessionRepos.lifecycle.staff = failingStaffDirectory{StaffRepository: r.Staff, err: settingsOverrides.staffCreateErr}
	}
	// The operator flows are composed here too: the operator second factor
	// and both portals' passkey ceremonies (#3331) mint their sessions
	// through them, so a test module without them would compose a module
	// production never builds.
	owners, err := newOwnerCapabilitiesForTests(db)
	if err != nil {
		return AuthTestModule{}, err
	}
	operators, err := newOperatorDependencies(operatorAuthenticationWiring{
		repos:         operatorRepositoriesOf(r),
		organizations: owners.organizations,
		persons:       owners.persons,
		membership:    owners.membership,
		logger:        logger,
	})
	if err != nil {
		return AuthTestModule{}, err
	}
	identityAccess, err := newIdentityAccessWithSessions(db, accountAuthenticationWiring{
		repos: sessionRepos, tokenAuth: authConfig.TokenAuth, settings: settings.Settings, audit: command, logger: logger,
		operators:     operators,
		tenantRuntime: func(ctx context.Context) context.Context { return service.WithTenantRuntime(ctx) },
		mfa: &mfaWiring{
			repos: r, settings: mfaSettingsService(settings.Settings, settingsOverrides), tokenAuth: authConfig.TokenAuth,
			dispatcher: dispatcher, defaultFrom: defaultFrom, frontendURL: frontendURL,
			jwtSecret: mfaTestSecret(), logger: logger, backoff: settingsOverrides.mfaBackoff,
			decorate: settingsOverrides.mfaRecords, capability: newModuleAccountMFA(settingsOverrides.mfaCapability),
			passkeys: &authTestRelyingParty,
		},
		lifecycle: &lifecycleWiring{
			settings: settings.Settings, audit: command,
			admin: func() *auth.Service { return service },
			guardianMail: &guardianInvitationWiring{
				settings: settings.Settings, schools: r.School,
				outbox:      func() platformModels.OutboxEnqueuer { return outboxEnqueuer{outbox: deliveryModule.EmailOutbox} },
				enrollments: r.ParentEnrollmentRequest, parentsURL: parentsURL,
				fallbackExpiry: time.Duration(inviteHours) * time.Hour, logger: logger,
			},
		},
		resets: &passwordResetWiring{
			dispatcher: dispatcher, defaultFrom: defaultFrom,
			staffURL: frontendURL, parentsURL: parentsURL, schoolURL: schoolURL,
			expiry: time.Duration(resetMinutes) * time.Minute, rateLimitEnabled: settingsOverrides.rateLimitEnabled,
			backoff: settingsOverrides.resetBackoff,
		},
		invitations: &invitationWiring{
			dispatcher: dispatcher, defaultFrom: defaultFrom, staffURL: frontendURL, schoolURL: schoolURL,
			mailIdentity: identity, tokenAuth: authConfig.TokenAuth, expiry: time.Duration(inviteHours) * time.Hour,
			backoff: settingsOverrides.resetBackoff,
		},
	})
	if err != nil {
		return AuthTestModule{}, err
	}
	accountSessionsPort := newAccountSessions(identityAccess)
	authConfig.Sessions = accountSessionsPort
	authConfig.Lifecycle = accountSessionsPort
	authConfig.Resets = accountSessionsPort
	service, err = auth.NewService(r, authConfig, db, logger)
	if err != nil {
		return AuthTestModule{}, err
	}
	service.SetTenantRuntime(unit)
	invitation := NewInvitationService(identityAccess)
	deliveryModule, err = NewDeliveryTestModule(db, unit)
	if err != nil {
		return AuthTestModule{}, err
	}
	guardian := auth.NewGuardianInvitationService(newGuardianInvitations(identityAccess), accountSessionsPort)
	return AuthTestModule{
		Auth: service, AccountAuthentication: identityAccess,
		StaffPINAuth: NewStaffPINAuthenticator(identityAccess),
		MFA:          newAccountMFAPort(identityAccess), Passkeys: newAccountPasskeyPort(identityAccess),
		OperatorMFA: newOperatorMFAPort(identityAccess), OperatorPasskeys: newOperatorPasskeyPort(identityAccess),
		Repos: r, TokenAuth: authConfig.TokenAuth,
		Invitation: invitation, GuardianInvitation: guardian,
		Schools: r.School, Settings: settings.Settings,
	}, nil
}

// NewAuthServiceForTests composes the retained auth service over the given
// repositories with its session flows bound to Identity & Access (#3251),
// the way the factory does it. Tests that used to build the service alone
// use it where login, refresh or revocation is exercised.
func NewAuthServiceForTests(repos *repositories.Factory, base auth.ServiceConfig, db *bun.DB, logger *slog.Logger) (*auth.Service, error) {
	cfg := base
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.TokenAuth == nil {
		signer, err := authjwt.NewTokenAuth()
		if err != nil {
			return nil, err
		}
		cfg.TokenAuth = signer
	}
	var service *auth.Service
	identityAccess, err := newIdentityAccessWithSessions(db, accountAuthenticationWiring{
		repos: sessionRepositoriesOf(repos, repos.School), tokenAuth: cfg.TokenAuth, settings: cfg.Settings, audit: cfg.Audit, logger: logger,
		tenantRuntime: func(ctx context.Context) context.Context { return service.WithTenantRuntime(ctx) },
		mfa: &mfaWiring{
			repos: repos, settings: cfg.Settings, tokenAuth: cfg.TokenAuth,
			dispatcher: cfg.Dispatcher, defaultFrom: cfg.DefaultFrom, frontendURL: cfg.FrontendURL,
			jwtSecret: mfaTestSecret(), logger: logger,
		},
		lifecycle: &lifecycleWiring{
			settings: cfg.Settings, audit: cfg.Audit,
			admin: func() *auth.Service { return service },
		},
		resets: &passwordResetWiring{
			dispatcher: cfg.Dispatcher, defaultFrom: cfg.DefaultFrom,
			staffURL: cfg.FrontendURL, parentsURL: cfg.ParentsURL, schoolURL: cfg.SchoolURL,
			expiry: cfg.PasswordResetExpiry, rateLimitEnabled: cfg.RateLimitEnabled,
		},
	})
	if err != nil {
		return nil, err
	}
	port := newAccountSessions(identityAccess)
	cfg.Sessions = port
	cfg.Lifecycle = port
	cfg.Resets = port
	service, err = auth.NewService(repos, &cfg, db, logger)
	return service, err
}

// IdentityAccessForTests returns the Identity & Access module a retained
// auth service composed by NewAuthServiceForTests delegates to, so behaviour
// tests reach the role administration (#3314) through the public contract.
func IdentityAccessForTests(service auth.AuthService) *identityaccess.Module {
	return identityAccessOf(service)
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
func mfaTestSecret() string { return currentFactoryConfig().JWTSecret }

// mfaSettingsService is the settings service the MFA gate resolves through:
// the composed one, or the one a test supplied.
func mfaSettingsService(composed config.SettingsService, overrides authTestSettings) config.SettingsService {
	if overrides.mfaSettings != nil {
		return overrides.mfaSettings
	}
	return composed
}
