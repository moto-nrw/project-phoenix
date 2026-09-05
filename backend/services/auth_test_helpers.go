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
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type AuthTestModule struct {
	Auth               auth.AuthService
	StaffPINAuth       auth.StaffPINAuthenticator
	Invitation         auth.InvitationService
	GuardianInvitation auth.GuardianInvitationService
	Schools            platform.SchoolService
	Settings           config.SettingsService
	MFA                auth.MFAService
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
	service, err := auth.NewService(r, authConfig, db, logger)
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
	identity := platform.NewTenantMailIdentityService(r.School, func(ctx context.Context, tenantID int64) (string, error) {
		return settings.Settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
	}, logger)
	invitation := auth.NewInvitationService(auth.InvitationServiceConfig{
		InvitationRepo: r.InvitationToken, AccountRepo: r.Account, AccountTenantRepo: r.AccountTenant,
		RoleRepo: r.Role, PermissionRepo: r.Permission, AccountRoleRepo: r.AccountRole,
		PersonRepo: r.Person, StaffRepo: r.Staff, TeacherRepo: r.Teacher, StudentRepo: r.Student, SchoolRepo: r.School,
		Mailer: mailer, Dispatcher: dispatcher, FrontendURL: frontendURL, SchoolURL: schoolURL,
		DefaultFrom: defaultFrom, InvitationExpiry: time.Duration(inviteHours) * time.Hour, MailIdentity: identity, DB: db, Logger: logger,
	})
	invitation.(tenantRuntimeSetter).SetTenantRuntime(unit)
	delivery, err := NewDeliveryTestModule(db, unit)
	if err != nil {
		return AuthTestModule{}, err
	}
	guardian := auth.NewGuardianInvitationService(auth.GuardianInvitationServiceConfig{
		InvitationRepo: r.GuardianInvitation, AccountRepo: r.Account, AccountTenantRepo: r.AccountTenant,
		AccountRoleRepo: r.AccountRole, RoleRepo: r.Role, PersonRepo: r.Person, GuardianProfileRepo: r.GuardianProfile,
		StudentGuardianRepo: r.StudentGuardian, Audit: command, StudentRepo: r.Student, SchoolRepo: r.School,
		EnrollmentBackfiller: r.ParentEnrollmentRequest, SettingsResolver: settings.Settings, OutboxEnqueuer: delivery.EmailOutbox,
		FrontendURL: parentsURL, FallbackExpiry: time.Duration(inviteHours) * time.Hour, DB: db, Logger: logger,
	})
	guardian.(tenantRuntimeSetter).SetTenantRuntime(unit)
	return AuthTestModule{Auth: service, StaffPINAuth: service, MFA: mfa, Invitation: invitation, GuardianInvitation: guardian,
		Schools: platform.NewSchoolService(r.School), Settings: settings.Settings}, nil
}
