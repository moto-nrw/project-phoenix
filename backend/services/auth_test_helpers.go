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
	Invitation            auth.InvitationService
	GuardianInvitation    auth.GuardianInvitationService
	Schools               organizationtenancy.Capability
	Settings              config.SettingsService
	MFA                   auth.MFAService
}

// InvitationSchoolsForTests binds the invitation school seam to the
// Organisation & Tenancy capability the way the factory does. Every read runs
// under runtime, as the serving root's request middleware provides it; the
// behaviour tests call the services without that middleware.
func InvitationSchoolsForTests(schools organizationtenancy.Query, runtime tenant.UnitOfWork) auth.SchoolDirectory {
	return runtimeInvitationSchools{directory: invitationSchoolDirectory{schools: schools}, runtime: runtime}
}

type runtimeInvitationSchools struct {
	directory invitationSchoolDirectory
	runtime   tenant.UnitOfWork
}

func (d runtimeInvitationSchools) FindSchool(ctx context.Context, id int64) (*auth.InvitationSchool, error) {
	return d.directory.FindSchool(tenant.WithUnitOfWork(ctx, d.runtime), id)
}

func (d runtimeInvitationSchools) FindSchoolForShare(ctx context.Context, id int64) (*auth.InvitationSchool, error) {
	return d.directory.FindSchoolForShare(tenant.WithUnitOfWork(ctx, d.runtime), id)
}

type InvitationTestModule struct {
	Persistence *repositories.InvitationPersistence
	Invitation  auth.InvitationService
}

func NewInvitationTestModule(db *bun.DB, unit tenant.UnitOfWork) (InvitationTestModule, error) {
	signer, err := authjwt.NewTokenAuth()
	if err != nil {
		return InvitationTestModule{}, err
	}
	repos, err := repositories.NewInvitationPersistence(db)
	if err != nil {
		return InvitationTestModule{}, err
	}
	schoolIdentity, err := NewSchoolIdentityForTests(db, unit)
	if err != nil {
		return InvitationTestModule{}, err
	}
	service := auth.NewInvitationService(auth.InvitationServiceConfig{
		TokenAuth: signer, InvitationRepo: repos.InvitationToken, AccountRepo: repos.Account,
		AccountTenantRepo: repos.AccountTenant, RoleRepo: repos.Role, PermissionRepo: repos.Permission,
		AccountRoleRepo: repos.AccountRole, SchoolIdentity: schoolIdentity, SchoolRepo: invitationSchoolDirectory{schools: repos.School}, DB: db,
	})
	service.(tenantRuntimeSetter).SetTenantRuntime(unit)
	return InvitationTestModule{Persistence: repos, Invitation: service}, nil
}

func NewAuthTestModule(db *bun.DB, unit tenant.UnitOfWork) (AuthTestModule, error) {
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
	mailer := email.NewMockMailer()
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
	authConfig.RateLimitEnabled = cfg.RateLimitEnabled
	authConfig.Settings = settings.Settings
	authConfig.Audit = command
	authConfig.TokenAuth, err = authjwt.NewTokenAuthWithDurations(cfg.JWTSecret, cfg.JWTExpiry, cfg.JWTRefreshExpiry)
	if err != nil {
		return AuthTestModule{}, err
	}
	var service *auth.Service
	var guardian auth.GuardianInvitationService
	identityAccess, err := newIdentityAccessWithSessions(db, accountAuthenticationWiring{
		repos: sessionRepositoriesOf(r, r.School), tokenAuth: authConfig.TokenAuth, settings: settings.Settings, audit: command, logger: logger,
		tenantRuntime: func(ctx context.Context) context.Context { return service.WithTenantRuntime(ctx) },
		mfa:           func() auth.MFAService { return service.CurrentMFAService() },
		lifecycle: &lifecycleWiring{
			settings: settings.Settings, audit: command,
			admin: func() *auth.Service { return service },
			delivery: func() auth.GuardianInvitationDelivery {
				delivery, _ := guardian.(auth.GuardianInvitationDelivery)
				return delivery
			},
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
	identity := emailoutbox.NewTenantMailIdentity(schoolContactDirectory{schools: r.School}, func(ctx context.Context, tenantID int64) (string, error) {
		return settings.Settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
	}, logger)
	invitation := auth.NewInvitationService(auth.InvitationServiceConfig{
		TokenAuth:      authConfig.TokenAuth,
		InvitationRepo: r.InvitationToken, AccountRepo: r.Account, AccountTenantRepo: r.AccountTenant,
		RoleRepo: r.Role, PermissionRepo: r.Permission, AccountRoleRepo: r.AccountRole, SchoolRepo: invitationSchoolDirectory{schools: r.School},
		Mailer: mailer, Dispatcher: dispatcher, FrontendURL: frontendURL, SchoolURL: schoolURL,
		DefaultFrom: defaultFrom, InvitationExpiry: time.Duration(inviteHours) * time.Hour, MailIdentity: identity,
		SchoolIdentity: accountSessionsPort, DB: db, Logger: logger,
	})
	invitation.(tenantRuntimeSetter).SetTenantRuntime(unit)
	delivery, err := NewDeliveryTestModule(db, unit)
	if err != nil {
		return AuthTestModule{}, err
	}
	guardian = auth.NewGuardianInvitationService(auth.GuardianInvitationServiceConfig{
		InvitationRepo: r.GuardianInvitation, AccountRepo: r.Account, AccountTenantRepo: r.AccountTenant,
		AccountRoleRepo: r.AccountRole, RoleRepo: r.Role, PersonRepo: r.Person, GuardianProfileRepo: r.GuardianProfile,
		StudentGuardianRepo: r.StudentGuardian, Audit: command, StudentRepo: r.Student, SchoolRepo: invitationSchoolDirectory{schools: r.School},
		EnrollmentBackfiller: r.ParentEnrollmentRequest, RelativeAccess: accountSessionsPort, SettingsResolver: settings.Settings, OutboxEnqueuer: outboxEnqueuer{outbox: delivery.EmailOutbox},
		FrontendURL: parentsURL, FallbackExpiry: time.Duration(inviteHours) * time.Hour, DB: db, Logger: logger,
	})
	guardian.(tenantRuntimeSetter).SetTenantRuntime(unit)
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
			admin:    func() *auth.Service { return service },
			delivery: func() auth.GuardianInvitationDelivery { return nil },
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
