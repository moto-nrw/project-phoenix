package enrollmenthttp_test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/api"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
)

// factorySchools reads the seeded schools through the repository factory's
// Organisation & Tenancy capability, the owner the serving root binds.
// Callers pass a factory built on the pool from testpkg.SetupTestDB.
type factorySchools struct {
	repos *api.OrganizationTestRows
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
	return s.settings.ResolveString(ctx, settingKeyNotifyPerDecision)
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
	return s.settings.ResolveBool(ctx, settingKeyCollectGradeLevel)
}

func (s collectionSettings) CollectSchoolClass(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, settingKeyCollectSchoolClass)
}

func (s collectionSettings) GradeLevelMax(ctx context.Context) (int, error) {
	return s.settings.ResolveInt(ctx, settingKeyGradeLevelMax)
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

// newTestDecisions composes Enrollment's decision flow over deps and the
// suite's settings; the optional outbox records its parent mails.
func newTestDecisions(deps enrollmentCompose.DecisionDependencies, settings api.EnrollmentDecisionSettings, outboxes ...testMailOutbox) *TestDecisions {
	deps.Settings = api.EnrollmentDecisionSettingValues(settings)
	if deps.Notifications == nil {
		var outbox testMailOutbox
		if len(outboxes) > 0 {
			outbox = outboxes[0]
		}
		modes, _ := deps.Requests.(TestNotificationModePin)
		deps.Notifications = testNotifications(modes, settings, outbox, nil)
	}
	return api.NewEnrollmentDecisions(deps)
}

func newTestDecisionService(deps enrollmentCompose.DecisionDependencies, settings api.EnrollmentDecisionSettings, outboxes ...testMailOutbox) enrollmentAPI.DecisionService {
	return newFlowDecisionService(newTestDecisions(deps, settings, outboxes...))
}

// newTestRequestService composes the intake; without an explicit gate it
// binds the owner's capacity gate over the suite's offerings, children and
// settings, the way the root binds it.
func newTestRequestService(cfg api.EnrollmentIntakeSources) enrollmentAPI.RequestService {
	if cfg.Notifications == nil {
		cfg.Notifications = testNotifications(cfg.Requests, cfg.Settings, cfg.OutboxEnqueuer, cfg.SchoolRepo)
	}
	if cfg.Capacity == nil {
		offerings, _ := cfg.CareOfferingRepo.(capacityOfferings)
		cfg.Capacity = api.NewEnrollmentOfferingCapacity(offerings, cfg.Children, cfg.Settings)
	}
	return enrollmentAPI.NewRequestService(api.NewEnrollmentIntake(cfg))
}

func newTestChangeRequestService(cfg api.EnrollmentChangeRequestSources) enrollmentAPI.ChangeRequestService {
	if cfg.Notifications == nil {
		cfg.Notifications = testNotifications(cfg.Requests, cfg.Settings, cfg.OutboxEnqueuer, nil)
	}
	if cfg.Capacity == nil {
		offerings, _ := cfg.CareOfferingRepo.(capacityOfferings)
		cfg.Capacity = api.NewEnrollmentOfferingCapacity(offerings, cfg.Children, cfg.Settings)
	}
	return enrollmentAPI.NewChangeRequestService(api.NewEnrollmentChangeRequests(cfg))
}

// capacityOfferings locks the offerings a capacity check claims.
type capacityOfferings interface {
	ListByIDsForUpdate(ctx context.Context, ids []int64) ([]*enrollmentModels.CareOffering, error)
}

func newTestRolloverService(src api.EnrollmentRolloverSources) enrollmentAPI.RolloverService {
	if src.Notifications == nil {
		src.Notifications = testNotifications(nil, nil, nil, nil)
	}
	if src.PhaseEligibility == nil && src.Settings != nil {
		src.PhaseEligibility = NewTestPhases(TestPhaseDependencies{Settings: collectionSettings{settings: src.Settings}})
	}
	return enrollmentAPI.NewRolloverService(api.NewEnrollmentRollovers(src))
}

// testNotifications composes Enrollment's parent mails for a flow under
// test: the owner pins the mode, the suite's settings choose it, and the
// suite's outbox records the mails.
func testNotifications(modes TestNotificationModePin, settings interface {
	ResolveString(ctx context.Context, key string) (string, error)
}, outbox testMailOutbox, schools enrollmentOwner.SchoolDirectory) enrollmentOwner.Notifications {
	return NewTestFlowNotifications(modes, notificationModeSettings{settings: settings}, outbox, schools)
}
