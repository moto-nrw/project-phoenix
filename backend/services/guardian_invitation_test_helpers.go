package services

import (
	"context"
	"log/slog"
	"time"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// GuardianEnrollmentClaims claims the enrollment requests a guardian filed
// before they had an account.
type GuardianEnrollmentClaims interface {
	BackfillGuardianAccountID(ctx context.Context, accountID int64, email string) (int, error)
}

// GuardianInvitationTestConfig is what a guardian invitation test varies:
// the outbox it reads the queued mail from, the audit command it inspects,
// the enrollment claims an acceptance runs and the token lifetime.
type GuardianInvitationTestConfig struct {
	Audit       auditModels.Command
	Outbox      platformModels.OutboxEnqueuer
	Enrollments GuardianEnrollmentClaims
	Expiry      time.Duration
	Logger      *slog.Logger
}

// lifecycleTestModule composes the Identity & Access module with the account
// lifecycle and the guardian invitation flows bound (#2722/#3225), the way
// the factory does it.
func lifecycleTestModule(db *bun.DB, unit tenant.UnitOfWork, cfg GuardianInvitationTestConfig) (*identityaccess.Module, *accountSessions, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	audit := cfg.Audit
	if audit == nil {
		command, err := auditSvc.NewCommand(repositories.NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
		if err != nil {
			return nil, nil, err
		}
		audit = command
	}
	repos, err := repositories.NewAuthTestRepositories(db, audit)
	if err != nil {
		return nil, nil, err
	}
	signer, err := authjwt.NewTokenAuth()
	if err != nil {
		return nil, nil, err
	}
	expiry := cfg.Expiry
	if expiry <= 0 {
		expiry = auth.GuardianTokenExpiryFallback
	}
	outbox := cfg.Outbox
	if outbox == nil {
		outbox = discardingOutbox{}
	}
	var service *auth.Service
	identityAccess, err := newIdentityAccessWithSessions(db, accountAuthenticationWiring{
		repos: sessionRepositoriesOf(repos, repos.School), tokenAuth: signer, audit: audit, logger: logger,
		tenantRuntime: func(ctx context.Context) context.Context { return service.WithTenantRuntime(ctx) },
		lifecycle: &lifecycleWiring{
			audit: audit,
			admin: func() *auth.Service { return service },
			guardianMail: &guardianInvitationWiring{
				schools: repos.School,
				outbox:  func() platformModels.OutboxEnqueuer { return outbox },
				enrollments: func() guardianEnrollmentClaims {
					if cfg.Enrollments == nil {
						return nil
					}
					return cfg.Enrollments
				}(),
				parentsURL: "http://localhost:3000", fallbackExpiry: expiry, logger: logger,
			},
		},
	})
	if err != nil {
		return nil, nil, err
	}
	port := newAccountSessions(identityAccess)
	serviceConfig, err := auth.NewServiceConfig(nil, email.Email{}, "http://localhost:3000", time.Hour)
	if err != nil {
		return nil, nil, err
	}
	serviceConfig.TokenAuth = signer
	serviceConfig.Audit = audit
	serviceConfig.Sessions = port
	serviceConfig.Lifecycle = port
	service, err = auth.NewService(repos, serviceConfig, db, logger)
	if err != nil {
		return nil, nil, err
	}
	service.SetTenantRuntime(unit)
	return identityAccess, port, nil
}

// discardingOutbox stands in for the e-mail outbox where a test does not
// look at the queued mail.
type discardingOutbox struct{}

func (discardingOutbox) EnqueueOutbox(context.Context, platformModels.OutboxEnqueueRequest) error {
	return nil
}

// NewSchoolIdentityForTests returns the Identity & Access school identity
// provisioning over the test database, for tests that compose a retained
// flow (staff invitation, registration) without the whole factory.
func NewSchoolIdentityForTests(db *bun.DB, unit tenant.UnitOfWork) (auth.SchoolIdentityProvisioning, error) {
	_, port, err := lifecycleTestModule(db, unit, GuardianInvitationTestConfig{})
	return port, err
}

// NewGuardianInvitationServiceForTests composes the retained guardian
// invitation service over the owner module, the way the factory does it: the
// invitation flows, the related-accounts flows and the mail all run against
// the test database and the outbox the config names.
func NewGuardianInvitationServiceForTests(db *bun.DB, unit tenant.UnitOfWork, cfg GuardianInvitationTestConfig) (auth.GuardianInvitationService, error) {
	module, port, err := lifecycleTestModule(db, unit, cfg)
	if err != nil {
		return nil, err
	}
	return auth.NewGuardianInvitationService(newGuardianInvitations(module), port), nil
}
