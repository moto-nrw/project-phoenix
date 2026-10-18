//go:build testhooks

package api

import (
	"context"
	"log/slog"
	"time"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// GuardianInvitationTestConfig is what a guardian invitation test varies:
// the outbox it reads the queued mail from, the audit command it inspects,
// the enrollment claims an acceptance runs and the token lifetime.
type GuardianInvitationTestConfig struct {
	Audit       auditModels.Command
	Outbox      MailResultOutbox
	Enrollments GuardianEnrollmentClaims
	Expiry      time.Duration
	Logger      *slog.Logger
}

// lifecycleTestModule composes the Identity & Access module with the account
// lifecycle and the guardian invitation flows bound (#2722/#3225), the way
// the service graph does it.
func lifecycleTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, cfg GuardianInvitationTestConfig) (*identityaccess.Module, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	audit := cfg.Audit
	if audit == nil {
		command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
		if err != nil {
			return nil, err
		}
		audit = command
	}
	repos, err := authTestIdentityChain(db)
	if err != nil {
		return nil, err
	}
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return nil, err
	}
	audits := auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db.Pool()})
	audits = audits.WithRoutedWrites(audit)
	tokens, err := newLegacyTokenAdapter()
	if err != nil {
		return nil, err
	}
	expiry := cfg.Expiry
	if expiry <= 0 {
		expiry = identityaccess.GuardianInvitationExpiryFallback
	}
	outbox := cfg.Outbox
	if outbox == nil {
		outbox = discardingOutbox{}
	}
	claims := GuardianEnrollmentClaims(unclaimedEnrollments{})
	if cfg.Enrollments != nil {
		claims = cfg.Enrollments
	}
	codec := tokens.SessionTokens
	caregivers, err := caregiverProfilesForTests(db)
	if err != nil {
		return nil, err
	}
	sessionRepos := SessionRepositoriesOf(repos, organizations, audits.AuthEvent, deliveryCompose.NewPushSubscriptionRepository(db.Pool()))
	guardianMail, enrollmentClaims, err := guardianInvitationDependencies(nil, organizations, outbox, claims, "http://localhost:3000", expiry, logger)
	if err != nil {
		return nil, err
	}
	lifecycle, err := lifecycleDependencies(sessionRepos.Lifecycle, audit, caregivers, guardianMail, enrollmentClaims, logger)
	if err != nil {
		return nil, err
	}
	identityAccess, err := NewIdentityAccessWithSessions(identityaccessCompose.Dependencies{
		DB: db.Pool(), Observe: func(identityaccessCompose.Observation) {}, Lifecycle: lifecycle,
	}, sessionRepos, codec, nil, audit, logger)
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

func (discardingOutbox) EnqueueOutbox(context.Context, deliveryCompose.MailEnqueueRequest) error {
	return nil
}

func (discardingOutbox) EnqueueOutboxWithResult(context.Context, deliveryCompose.MailEnqueueRequest) (deliveryCompose.MailEnqueued, error) {
	return deliveryCompose.MailEnqueued{ID: 1}, nil
}

// NewSchoolIdentityForTests returns the Identity & Access school identity
// provisioning over the test database, for tests that compose a retained
// flow (staff invitation, registration) without the whole factory.
func NewSchoolIdentityForTests(db transactionruntime.Database, unit tenant.UnitOfWork) (identityaccess.SchoolIdentityProvisioning, error) {
	return lifecycleTestModule(db, unit, GuardianInvitationTestConfig{})
}

// NewGuardianInvitationServiceForTests composes the retained guardian
// invitation service over the owner module, the way the service graph does it: the
// invitation flows, the related-accounts flows and the mail all run against
// the test database and the outbox the config names.
func NewGuardianInvitationServiceForTests(db transactionruntime.Database, unit tenant.UnitOfWork, cfg GuardianInvitationTestConfig) (GuardianInvitationCapability, error) {
	module, err := lifecycleTestModule(db, unit, cfg)
	if err != nil {
		return nil, err
	}
	return module, nil
}
