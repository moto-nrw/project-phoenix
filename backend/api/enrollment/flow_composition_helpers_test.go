package enrollment_test

import (
	"context"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/api/enrollment"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// factorySchools reads the seeded schools through the repository factory's
// Organisation & Tenancy capability, the owner the serving root binds.
// Callers pass a factory built on the pool from testpkg.SetupTestDB.
type factorySchools struct {
	repos *repositories.EnrollmentFlowTestRepositories
}

func (s factorySchools) FindSchool(ctx context.Context, id int64) (*enrollmentOwner.School, error) {
	school, err := s.repos.School.FindSchool(ctx, id)
	if err != nil {
		return nil, err
	}
	return &enrollmentOwner.School{
		Name: school.Name, Subdomain: school.Subdomain, Settings: school.Settings, Deleted: school.IsDeleted(),
	}, nil
}

// notificationModeSettings reads the decision notification mode from a
// suite's settings double.
type notificationModeSettings struct {
	settings interface {
		ResolveString(ctx context.Context, key string) (string, error)
	}
}

func (s notificationModeSettings) NotifyPerDecision(ctx context.Context) (string, error) {
	if s.settings == nil {
		return "", nil
	}
	return s.settings.ResolveString(ctx, configModel.KeyEnrollmentNotifyPerDecision)
}

// collectionSettings reads the grade and class collection toggles from a
// suite's settings double; a double without the class-collection lock
// skips it.
type collectionSettings struct {
	settings interface {
		ResolveBool(ctx context.Context, key string) (bool, error)
		ResolveInt(ctx context.Context, key string) (int, error)
	}
}

func (s collectionSettings) CollectGradeLevel(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, configModel.KeyEnrollmentCollectGradeLevel)
}

func (s collectionSettings) CollectSchoolClass(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, configModel.KeyEnrollmentCollectSchoolClass)
}

func (s collectionSettings) GradeLevelMax(ctx context.Context) (int, error) {
	return s.settings.ResolveInt(ctx, configModel.KeyEnrollmentGradeLevelMax)
}

func (s collectionSettings) LockClassCollectionPair(ctx context.Context) error {
	if locker, ok := s.settings.(interface {
		LockClassCollectionPair(context.Context) error
	}); ok {
		return locker.LockClassCollectionPair(ctx)
	}
	return nil
}

// phaseOfferings reads a phase's Care Plan offerings through the retained
// offering rows, the way the root binds the phase administration.
type phaseOfferings struct {
	rows interface {
		ListByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error)
		CountByPhaseID(ctx context.Context, phaseID int64) (int, error)
	}
}

func (o phaseOfferings) OfferingIDsForPhase(ctx context.Context, phaseID int64) ([]int64, error) {
	offerings, err := o.rows.ListByPhase(ctx, phaseID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		if offering != nil {
			ids = append(ids, offering.ID)
		}
	}
	return ids, nil
}

func (o phaseOfferings) CountOfferingsForPhase(ctx context.Context, phaseID int64) (int, error) {
	return o.rows.CountByPhaseID(ctx, phaseID)
}

// The constructors below compose a retained service the way the root does:
// Enrollment's parent mails over the suite's owner, settings and outbox, and
// the rollover's eligibility guard over the suite's settings.

// newTestDecisions composes Enrollment's decision flow over the sources; the
// optional outbox records its parent mails.
func newTestDecisions(src testutil.EnrollmentDecisionSources, outboxes ...platformModels.OutboxEnqueuer) *enrollmentAPI.TestDecisions {
	if src.Notifications == nil {
		var outbox platformModels.OutboxEnqueuer
		if len(outboxes) > 0 {
			outbox = outboxes[0]
		}
		modes, _ := src.Requests.(enrollmentAPI.TestNotificationModePin)
		src.Notifications = testNotifications(modes, src.Settings, outbox, nil)
	}
	return testutil.NewEnrollmentDecisions(src)
}

func newTestDecisionService(src testutil.EnrollmentDecisionSources, outboxes ...platformModels.OutboxEnqueuer) enrollmentAPI.DecisionService {
	return newFlowDecisionService(newTestDecisions(src, outboxes...))
}

// newTestRequestService composes the intake; without an explicit gate it
// binds the owner's capacity gate over the suite's offerings, children and
// settings, the way the root binds it.
func newTestRequestService(cfg testutil.EnrollmentIntakeSources) enrollmentAPI.RequestService {
	if cfg.Notifications == nil {
		cfg.Notifications = testNotifications(cfg.Requests, cfg.Settings, cfg.OutboxEnqueuer, cfg.SchoolRepo)
	}
	if cfg.Capacity == nil {
		offerings, _ := cfg.CareOfferingRepo.(capacityOfferings)
		cfg.Capacity = testutil.NewEnrollmentOfferingCapacity(offerings, cfg.Children, cfg.Settings)
	}
	return enrollmentAPI.NewRequestService(testutil.NewEnrollmentIntake(cfg))
}

func newTestChangeRequestService(cfg testutil.EnrollmentChangeRequestSources) enrollmentAPI.ChangeRequestService {
	if cfg.Notifications == nil {
		cfg.Notifications = testNotifications(cfg.Requests, cfg.Settings, cfg.OutboxEnqueuer, nil)
	}
	if cfg.Capacity == nil {
		offerings, _ := cfg.CareOfferingRepo.(capacityOfferings)
		cfg.Capacity = testutil.NewEnrollmentOfferingCapacity(offerings, cfg.Children, cfg.Settings)
	}
	return enrollmentAPI.NewChangeRequestService(testutil.NewEnrollmentChangeRequests(cfg))
}

// capacityOfferings locks the offerings a capacity check claims.
type capacityOfferings interface {
	ListByIDsForUpdate(ctx context.Context, ids []int64) ([]*enrollmentModels.CareOffering, error)
}

func newTestRolloverService(src testutil.EnrollmentRolloverSources) enrollmentAPI.RolloverService {
	if src.Notifications == nil {
		src.Notifications = testNotifications(nil, nil, nil, nil)
	}
	if src.PhaseEligibility == nil && src.Settings != nil {
		src.PhaseEligibility = enrollmentAPI.NewTestPhases(enrollmentAPI.TestPhaseDependencies{Settings: collectionSettings{settings: src.Settings}})
	}
	return enrollmentAPI.NewRolloverService(testutil.NewEnrollmentRollovers(src))
}

// testNotifications composes Enrollment's parent mails for a flow under
// test: the owner pins the mode, the suite's settings choose it, and the
// suite's outbox records the mails.
func testNotifications(modes enrollmentAPI.TestNotificationModePin, settings interface {
	ResolveString(ctx context.Context, key string) (string, error)
}, outbox platformModels.OutboxEnqueuer, schools enrollmentOwner.SchoolDirectory) enrollmentOwner.Notifications {
	return enrollmentAPI.NewTestFlowNotifications(modes, notificationModeSettings{settings: settings}, outbox, schools)
}
