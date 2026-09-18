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
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/config"
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
	Invitation            InvitationCapability
	GuardianInvitation    GuardianInvitationCapability
	Schools               organizationtenancy.Capability
	Settings              config.SettingsService
	MFA                   auth.MFAService
}

type InvitationTestModule struct {
	Persistence *repositories.InvitationPersistence
	Invitation  InvitationCapability
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

// AuthTestOption overrides a default of the composed test module.
type AuthTestOption func(*authTestSettings)

type authTestSettings struct {
	mailer           email.Mailer
	rateLimitEnabled bool
	resetBackoff     []time.Duration
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
	identityAccess, err := newIdentityAccessWithSessions(db, accountAuthenticationWiring{
		repos: sessionRepos, tokenAuth: authConfig.TokenAuth, settings: settings.Settings, audit: command, logger: logger,
		tenantRuntime: func(ctx context.Context) context.Context { return service.WithTenantRuntime(ctx) },
		mfa:           func() auth.MFAService { return service.CurrentMFAService() },
		lifecycle: &lifecycleWiring{
			settings: settings.Settings, audit: command,
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
	service, err = auth.NewService(r, authConfig, db, logger)
	if err != nil {
		return AuthTestModule{}, err
	}
	service.SetTenantRuntime(unit)
	mfa, err := auth.NewMFAService(auth.MFAServiceConfig{
		Repos: r, TokenAuth: authConfig.TokenAuth, Settings: settings.Settings, Dispatcher: dispatcher,
		DefaultFrom: defaultFrom, FrontendURL: frontendURL, JWTSecret: cfg.JWTSecret, DB: db, Logger: logger, Audit: command,
	})
	if err != nil {
		return AuthTestModule{}, err
	}
	mfa.(tenantRuntimeSetter).SetTenantRuntime(unit)
	service.SetMFAService(mfa)
	invitation := InvitationCapability(identityAccess)
	deliveryModule, err = NewDeliveryTestModule(db, unit)
	if err != nil {
		return AuthTestModule{}, err
	}
	guardian := GuardianInvitationCapability(identityAccess)
	return AuthTestModule{Auth: service, AccountAuthentication: identityAccess, StaffPINAuth: NewStaffPINAuthenticator(identityAccess), MFA: mfa, Invitation: invitation, GuardianInvitation: guardian,
		Schools: r.School, Settings: settings.Settings}, nil
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
		mfa:           func() auth.MFAService { return service.CurrentMFAService() },
		lifecycle: &lifecycleWiring{
			settings: cfg.Settings, audit: cfg.Audit,
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
