package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
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
func lifecycleTestModule(db *bun.DB, unit tenant.UnitOfWork, cfg GuardianInvitationTestConfig) (*identityaccess.Module, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	audit := cfg.Audit
	if audit == nil {
		command, err := auditSvc.NewCommand(repositories.NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
		if err != nil {
			return nil, err
		}
		audit = command
	}
	repos, err := repositories.NewAuthTestRepositories(db, audit)
	if err != nil {
		return nil, err
	}
	signer, err := authjwt.NewTokenAuth()
	if err != nil {
		return nil, err
	}
	expiry := cfg.Expiry
	if expiry <= 0 {
		expiry = GuardianTokenExpiryFallback
	}
	outbox := cfg.Outbox
	if outbox == nil {
		outbox = discardingOutbox{}
	}
	claims := guardianEnrollmentClaims(unclaimedEnrollments{})
	if cfg.Enrollments != nil {
		claims = cfg.Enrollments
	}
	codec, err := signedIdentityTokensOf(signer)
	if err != nil {
		return nil, err
	}
	identityAccess, err := newIdentityAccessWithSessions(db, accountAuthenticationWiring{
		repos: sessionRepositoriesOf(repos, repos.School), codec: codec, audit: audit, logger: logger,
		lifecycle: &lifecycleWiring{
			audit: audit,
			guardianMail: &guardianInvitationWiring{
				schools:     repos.School,
				outbox:      func() platformModels.OutboxEnqueuer { return outbox },
				enrollments: claims,
				parentsURL:  "http://localhost:3000", fallbackExpiry: expiry, logger: logger,
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if err := identityAccess.SetTenantRuntime(unit); err != nil {
		return nil, err
	}
	return identityAccess, nil
}

// unclaimedEnrollments stands in for the enrollment claim where a test has
// no pre-account requests to claim; the serving root always binds the real
// one.
type unclaimedEnrollments struct{}

func (unclaimedEnrollments) BackfillGuardianAccountID(context.Context, int64, string) (int, error) {
	return 0, nil
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
func NewSchoolIdentityForTests(db *bun.DB, unit tenant.UnitOfWork) (identityaccess.SchoolIdentityProvisioning, error) {
	return lifecycleTestModule(db, unit, GuardianInvitationTestConfig{})
}

// NewGuardianInvitationServiceForTests composes the retained guardian
// invitation service over the owner module, the way the factory does it: the
// invitation flows, the related-accounts flows and the mail all run against
// the test database and the outbox the config names.
func NewGuardianInvitationServiceForTests(db *bun.DB, unit tenant.UnitOfWork, cfg GuardianInvitationTestConfig) (GuardianInvitationCapability, error) {
	module, err := lifecycleTestModule(db, unit, cfg)
	if err != nil {
		return nil, err
	}
	return module, nil
}
