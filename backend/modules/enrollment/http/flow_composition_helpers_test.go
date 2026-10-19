package enrollmenthttp_test

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"

	"github.com/moto-nrw/project-phoenix/api"
	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
)

// factorySchools reads the seeded schools through the repository factory's
// Organisation & Tenancy capability, the owner the serving root binds.
// Callers pass a factory built on the pool from testpkg.SetupTestDB.
type factorySchools struct {
	repos *api.OrganizationTestRows
}

func (s factorySchools) FindSchool(ctx context.Context, id int64) (*enrollmentNative.School, error) {
	school, err := s.repos.School.FindSchool(ctx, id)
	if err != nil {
		return nil, err
	}
	return &enrollmentNative.School{
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
// unusedPhaseRecords stands in for the phase rows of a phase administration
// whose flow under test never reads them.
type unusedPhaseRecords struct{ TestPhaseRecords }

type phaseOfferings struct {
	rows interface {
		ListByPhase(ctx context.Context, phaseID int64) ([]*offeringContract.CareOfferingRecord, error)
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
func newTestDecisions(deps enrollmentCompose.DecisionDependencies, settings owners.EnrollmentDecisionSettings, outboxes ...testMailOutbox) *TestDecisions {
	deps.Settings = owners.EnrollmentDecisionSettingValues(settings)
	if deps.Notifications == nil {
		var outbox testMailOutbox
		if len(outboxes) > 0 {
			outbox = outboxes[0]
		}
		modes, _ := deps.Requests.(TestNotificationModePin)
		deps.Notifications = testNotifications(modes, settings, outbox, nil)
	}
	return owners.NewEnrollmentDecisions(deps)
}

func newTestDecisionService(deps enrollmentCompose.DecisionDependencies, settings owners.EnrollmentDecisionSettings, outboxes ...testMailOutbox) enrollmentAPI.DecisionService {
	return newFlowDecisionService(newTestDecisions(deps, settings, outboxes...))
}

// newTestRequestService composes the intake; without an explicit gate it
// binds the owner's capacity gate over the suite's offerings, children and
// settings, the way the root binds it.
func newTestRequestService(cfg owners.EnrollmentIntakeSources) enrollmentAPI.RequestService {
	if cfg.Notifications == nil {
		cfg.Notifications = testNotifications(cfg.Requests, cfg.Settings, cfg.OutboxEnqueuer, cfg.SchoolRepo)
	}
	if cfg.Capacity == nil {
		offerings, _ := cfg.CareOfferingRepo.(capacityOfferings)
		cfg.Capacity = owners.NewEnrollmentOfferingCapacity(offerings, cfg.Children, cfg.Settings)
	}
	return requestFixtureRoot{enrollmentAPI.NewRequestService(owners.NewEnrollmentIntake(cfg))}
}

func newTestChangeRequestService(cfg owners.EnrollmentChangeRequestSources) enrollmentAPI.ChangeRequestService {
	if cfg.Notifications == nil {
		cfg.Notifications = testNotifications(cfg.Requests, cfg.Settings, cfg.OutboxEnqueuer, nil)
	}
	if cfg.Capacity == nil {
		offerings, _ := cfg.CareOfferingRepo.(capacityOfferings)
		cfg.Capacity = owners.NewEnrollmentOfferingCapacity(offerings, cfg.Children, cfg.Settings)
	}
	return enrollmentAPI.NewChangeRequestService(owners.NewEnrollmentChangeRequests(cfg))
}

// capacityOfferings locks the offerings a capacity check claims.
type capacityOfferings interface {
	ListByIDsForUpdate(ctx context.Context, ids []int64) ([]*offeringContract.CareOfferingRecord, error)
}

func newTestRolloverService(src owners.EnrollmentRolloverSources) enrollmentAPI.RolloverService {
	if src.Notifications == nil {
		src.Notifications = testNotifications(nil, nil, nil, nil)
	}
	if src.PhaseEligibility == nil && src.Settings != nil {
		src.PhaseEligibility = NewTestPhases(TestPhaseDependencies{
			Records: unusedPhaseRecords{}, Offerings: phaseOfferings{}, LockTemplateRecurrence: func(context.Context) error { return nil },
			Resyncer: &recordingSourcedTemplateResyncer{}, Settings: collectionSettings{settings: src.Settings},
		})
	}
	return rolloverFixtureRoot{enrollmentAPI.NewRolloverService(owners.NewEnrollmentRollovers(src))}
}

// testNotifications composes Enrollment's parent mails for a flow under
// test: the owner pins the mode, the suite's settings choose it, and the
// suite's outbox records the mails.
func testNotifications(modes TestNotificationModePin, settings interface {
	ResolveString(ctx context.Context, key string) (string, error)
}, outbox testMailOutbox, schools enrollmentNative.SchoolDirectory) enrollmentNative.Notifications {
	return NewTestFlowNotifications(modes, notificationModeSettings{settings: settings}, outbox, schools)
}

// Fixture roots supply the same outer units as the HTTP/worker roots. The
// native owner and persistence tests exercise missing-transaction rejection.
type requestFixtureRoot struct{ enrollmentAPI.RequestService }

func (r requestFixtureRoot) Submit(ctx context.Context, req enrollmentAPI.SubmitRequest) (value *enrollmentAPI.SubmitResult, err error) {
	id, err := tenant.TenantFromContext(ctx)
	if err != nil {
		id, err = tenant.NewTenantID(req.TenantID)
	}
	if err != nil {
		return nil, err
	}
	err = tenant.WithinTenant(ctx, id, func(txCtx context.Context) error { value, err = r.RequestService.Submit(txCtx, req); return err })
	return value, err
}
func (r requestFixtureRoot) GetByStatusToken(ctx context.Context, token string) (value *enrollmentAPI.Request, children []*enrollmentAPI.RequestChild, err error) {
	err = fixturePublicTokenRoot(ctx, func(txCtx context.Context) error {
		value, children, err = r.RequestService.GetByStatusToken(txCtx, token)
		return err
	})
	return value, children, err
}
func (r requestFixtureRoot) Edit(ctx context.Context, token string, patch enrollmentAPI.EditPatch) error {
	return fixturePublicTokenRoot(ctx, func(txCtx context.Context) error { return r.RequestService.Edit(txCtx, token, patch) })
}
func (r requestFixtureRoot) ConfirmRenewal(ctx context.Context, token string) (value int, err error) {
	err = fixturePublicTokenRoot(ctx, func(txCtx context.Context) error {
		value, err = r.RequestService.ConfirmRenewal(txCtx, token)
		return err
	})
	return value, err
}
func fixturePublicTokenRoot(ctx context.Context, fn func(context.Context) error) error {
	if _, active := tenant.TransactionFromContext(ctx); active {
		return fn(ctx)
	}
	return tenant.WithinAdmin(ctx, fn)
}

type rolloverFixtureRoot struct{ enrollmentAPI.RolloverService }

func (r rolloverFixtureRoot) CreatePhaseFromSource(ctx context.Context, req enrollmentAPI.CreatePhaseFromSourceRequest) (value *enrollmentAPI.RolloverResult, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value, err = r.RolloverService.CreatePhaseFromSource(txCtx, req)
		return err
	})
	return value, err
}
func (r rolloverFixtureRoot) PreviewPhaseFromSource(ctx context.Context, id int64, bumps bool) (value *enrollmentAPI.RolloverPreview, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value, err = r.RolloverService.PreviewPhaseFromSource(txCtx, id, bumps)
		return err
	})
	return value, err
}
func (r rolloverFixtureRoot) ListReviewQueue(ctx context.Context, id int64) (value []*enrollmentAPI.ReviewQueueItem, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value, err = r.RolloverService.ListReviewQueue(txCtx, id)
		return err
	})
	return value, err
}
func (r rolloverFixtureRoot) DecideReview(ctx context.Context, req enrollmentAPI.DecideReviewRequest) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error { return r.RolloverService.DecideReview(txCtx, req) })
}
func (r rolloverFixtureRoot) RunDeadlineWorker(ctx context.Context, at time.Time) (value *enrollmentAPI.DeadlineWorkerSummary, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value, err = r.RolloverService.RunDeadlineWorker(txCtx, at)
		return err
	})
	return value, err
}

func newFixtureRequestRoot(owner enrollmentAPI.IntakeOwner) enrollmentAPI.RequestService {
	return requestFixtureRoot{enrollmentAPI.NewRequestService(owner)}
}

func (r requestFixtureRoot) CreateLateInvite(ctx context.Context, input enrollmentAPI.CreateLateInviteInput) (value *enrollmentAPI.CreateLateInviteResult, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value, err = r.RequestService.CreateLateInvite(txCtx, input)
		return err
	})
	return value, err
}
