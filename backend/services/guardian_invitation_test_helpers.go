package services

import (
	"context"
	"log/slog"
	"time"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// lifecycleTestPort composes the Identity & Access module with the account
// lifecycle bound (#3225), the way the factory does it, and returns the
// retained auth service's port over it. delivery is read at call time so a
// guardian invitation service composed afterwards can serve it.
func lifecycleTestPort(db *bun.DB, unit tenant.UnitOfWork, audit auditModels.Command, logger *slog.Logger, delivery func() auth.GuardianInvitationDelivery) (*accountSessions, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if audit == nil {
		command, err := auditSvc.NewCommand(repositories.NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
		if err != nil {
			return nil, err
		}
		audit = command
	}
	if delivery == nil {
		delivery = func() auth.GuardianInvitationDelivery { return nil }
	}
	repos, err := repositories.NewAuthTestRepositories(db, audit)
	if err != nil {
		return nil, err
	}
	signer, err := authjwt.NewTokenAuth()
	if err != nil {
		return nil, err
	}
	var service *auth.Service
	identityAccess, err := newIdentityAccessWithSessions(db, accountAuthenticationWiring{
		repos: sessionRepositoriesOf(repos, repos.School), tokenAuth: signer, audit: audit, logger: logger,
		tenantRuntime: func(ctx context.Context) context.Context { return service.WithTenantRuntime(ctx) },
		lifecycle: &lifecycleWiring{
			audit:    audit,
			admin:    func() *auth.Service { return service },
			delivery: delivery,
		},
	})
	if err != nil {
		return nil, err
	}
	port := newAccountSessions(identityAccess)
	serviceConfig, err := auth.NewServiceConfig(nil, email.Email{}, "http://localhost:3000", time.Hour)
	if err != nil {
		return nil, err
	}
	serviceConfig.TokenAuth = signer
	serviceConfig.Audit = audit
	serviceConfig.Sessions = port
	serviceConfig.Lifecycle = port
	service, err = auth.NewService(repos, serviceConfig, db, logger)
	if err != nil {
		return nil, err
	}
	service.SetTenantRuntime(unit)
	return port, nil
}

// NewSchoolIdentityForTests returns the Identity & Access school identity
// provisioning over the test database, for tests that compose a retained
// flow (staff invitation, registration) without the whole factory.
func NewSchoolIdentityForTests(db *bun.DB, unit tenant.UnitOfWork) (auth.SchoolIdentityProvisioning, error) {
	return lifecycleTestPort(db, unit, nil, nil, nil)
}

// NewGuardianInvitationServiceForTests composes the guardian invitation
// service over cfg with its related-accounts flows bound to Identity &
// Access (#3225), the way the factory does it: the module's invitation
// delivery and financial audit are the service's own outbox and audit
// command, so a test that captures either keeps seeing what it enqueued.
func NewGuardianInvitationServiceForTests(db *bun.DB, unit tenant.UnitOfWork, cfg auth.GuardianInvitationServiceConfig) (auth.GuardianInvitationService, error) {
	var guardian auth.GuardianInvitationService
	port, err := lifecycleTestPort(db, unit, cfg.Audit, cfg.Logger, func() auth.GuardianInvitationDelivery {
		delivery, _ := guardian.(auth.GuardianInvitationDelivery)
		return delivery
	})
	if err != nil {
		return nil, err
	}
	cfg.RelativeAccess = port
	guardian = auth.NewGuardianInvitationService(cfg)
	guardian.(tenantRuntimeSetter).SetTenantRuntime(unit)
	return guardian, nil
}
