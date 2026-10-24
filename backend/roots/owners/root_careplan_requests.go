package owners

import (
	"context"
	"fmt"

	realtime "github.com/moto-nrw/project-phoenix/realtime"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// CarePlanRequests are the parent request queues of Care Plan: the
// care-schedule change requests (#1803), the post-enrollment offering
// changes (#1665, #3561), the pickup adjustments, the excused-absence
// requests (#1845, #3093), the Stammdaten decisions (#3354) and the
// cross-kind bulk approval and conflict resolution (#2267, stories 6-10);
// one append-only ledger keeps every parent request's history (#2267).
type CarePlanRequests struct {
	CareRequests    carerequests.Service
	OfferingChanges careplan.OfferingChangeCapability
	CareOfferings   careplan.CareOfferingCapability
	Excused         *carePlanCompose.ExcusedAbsenceRequests
	MasterData      *carePlanCompose.MasterDataDecisions
	ParentRequests  parentrequests.Coordinator
	Documents       careplan.StudentDocuments
}

// NewRequestShareVisibility builds People Directory's sharing rules over the
// sharing ledger and the family protection alone, so every request queue
// receives them at construction (#2267, story 47).
func NewRequestShareVisibility(db transactionruntime.Database) (eventContract.ShareVisibilityResolver, error) {
	return peopleCompose.NewRequestShareVisibility(
		peopleCompose.NewParentRequestShareEventRepository(db.Pool()),
		peopleCompose.NewFamilyProtectionEventRepository(db.Pool()),
	)
}

// BuildCarePlanRequests binds the parent request queues over the schedules,
// the offerings, the caller context, the review policy and the effect sinks.
func BuildCarePlanRequests(
	db transactionruntime.Database, env RootEnvironment, hub *realtime.Hub, carePlan careplan.Capability,
	persons peopledirectory.Capability, people PeopleRows, schedules CarePlanSchedules, offerings CarePlanOfferings,
	enrollment EnrollmentRows, timetableOwner timetable.Capability, settings settingsCompose.BatchSettingsService,
	audit AuditOwner, services PeopleServices, emitter eventContract.Emitter, caller *CallerRows,
	reviews *ParentRequestReviewPolicy, permissions func(context.Context) []string, absences deliveryCompose.AbsenceNotifier, documentPermissions carePlanCompose.StudentDocumentPermissions,
) (CarePlanRequests, error) {
	shares, err := NewRequestShareVisibility(db)
	if err != nil {
		return CarePlanRequests{}, err
	}
	events := peopleCompose.NewParentRequestEventRepository(db.Pool())
	pickupPermissions, err := carePlanCompose.NewGuardianPickupPermissions(db.Pool(), env.Observe.CarePlan)
	if err != nil {
		return CarePlanRequests{}, fmt.Errorf("compose care plan pickup permissions: %w", err)
	}
	requests := CarePlanRequests{
		CareRequests: NewCareScheduleRequestServiceWithPickupChangesAndPolicy(
			carePlan, persons, schedules.Arrivals, schedules.Pickups, NewLoggedStudentPresence(db, env.Logger),
			schedules.AutoExcusal, carePlan, caller, emitter, hub,
			reviews, permissions, events, env.Logger.With("service", "care-requests"), services.StudentAudit,
			WithRequestShareVisibility(shares), WithCareRequestToday(env.Today), WithPickupPermissions(pickupPermissions),
		),
	}
	if requests.OfferingChanges, err = buildOfferingChanges(db, env, carePlan, people, enrollment, timetableOwner, settings, offerings, reviews, permissions, emitter, shares); err != nil {
		return CarePlanRequests{}, err
	}
	adjustments, err := NewPickupAdjustments(
		carePlan, schedules.Pickups, schedules.Arrivals, schedules.PickupBaselines, requests.OfferingChanges,
		settings, services.StudentAudit, people.Students, env.Today,
	)
	if err != nil {
		return CarePlanRequests{}, fmt.Errorf("compose care plan pickup adjustments: %w", err)
	}
	requests.CareOfferings = carePlanCompose.NewCareOfferings(offerings.Catalog, offerings.Bookings, requests.OfferingChanges, adjustments)
	// The workflow is owned by Care Plan (#3093); this root adapts the
	// retained directories, the review policy and the effect sinks.
	if requests.Excused, err = NewExcusedAbsenceRequestService(
		carePlan, people.Students, people.Persons, ParentRequestReviewScope(reviews, permissions),
		emitter, hub, events, absences, shares, env.Observe.CarePlan, env.Logger.With("service", "excused-requests"), nil,
	); err != nil {
		return CarePlanRequests{}, fmt.Errorf("compose excused absence requests: %w", err)
	}
	return buildCarePlanDecisions(db, env, hub, carePlan, persons, people, services, emitter, caller, reviews, permissions, shares, audit, requests, documentPermissions)
}

// buildOfferingChanges binds the offering-change review over Enrollment's
// manual planning and course groups.
func buildOfferingChanges(
	db transactionruntime.Database, env RootEnvironment, carePlan careplan.Capability, people PeopleRows, enrollment EnrollmentRows,
	timetableOwner timetable.Capability, settings settingsCompose.BatchSettingsService, offerings CarePlanOfferings,
	reviews *ParentRequestReviewPolicy, permissions func(context.Context) []string, emitter eventContract.Emitter,
	shares eventContract.ShareVisibilityResolver,
) (careplan.OfferingChangeCapability, error) {
	deps, err := OfferingChangeOwners(
		carePlan, nil, enrollment.Bookings, people.Students, NewCareWithdrawalCompletionRepository(carePlan),
		settings, reviews, permissions, emitter, peopleCompose.NewParentRequestEventRepository(db.Pool()),
	)
	if err != nil {
		return nil, fmt.Errorf("compose care plan offering changes: %w", err)
	}
	deps.Planning = ManualPlanningReader{Query: PlanningQuery{DB: db}, CourseGroups: timetableOwner}
	deps.Bookings = offerings.Bookings
	deps.Shares = shares
	deps.Today = env.Today
	deps.Logger = env.Logger.With("service", "offering-change-requests")
	changes, err := ComposeOfferingChanges(deps)
	if err != nil {
		return nil, fmt.Errorf("compose care plan offering changes: %w", err)
	}
	return changes, nil
}

// buildCarePlanDecisions binds the child documents (#777), the Stammdaten
// decisions and the cross-kind coordinator. People Directory keeps the
// child's record and its change history behind the decision's records port;
// the review scope is the one the Stammdaten queue is listed with. Every
// queue is bound at construction, or the resolve route would answer
// conflict_kind_unsupported for the missing one.
func buildCarePlanDecisions(
	db transactionruntime.Database, env RootEnvironment, hub *realtime.Hub, carePlan careplan.Capability,
	persons peopledirectory.Capability, people PeopleRows, services PeopleServices, emitter eventContract.Emitter,
	caller *CallerRows, reviews *ParentRequestReviewPolicy, permissions func(context.Context) []string,
	shares eventContract.ShareVisibilityResolver, audit AuditOwner, requests CarePlanRequests, documentPermissions carePlanCompose.StudentDocumentPermissions,
) (CarePlanRequests, error) {
	deps, err := StudentDocumentDependencies(carePlan, people.Students, caller, audit.Persistence.StudentFieldEdit, audit.Persistence.DataAccessLog, documentPermissions)
	if err != nil {
		return CarePlanRequests{}, fmt.Errorf("compose care plan documents: %w", err)
	}
	deps.DB = db.Pool()
	if requests.Documents, err = carePlanCompose.NewStudentDocuments(deps); err != nil {
		return CarePlanRequests{}, fmt.Errorf("compose care plan documents: %w", err)
	}
	events := peopleCompose.NewParentRequestEventRepository(db.Pool())
	if requests.MasterData, err = NewMasterDataDecisions(
		carePlan, persons, people.Students, people.Persons,
		services.StudentAudit, ParentRequestWriteScope(reviews, permissions),
		emitter, hub, events, shares, env.Logger.With("service", "master-data-review"), env.Today,
	); err != nil {
		return CarePlanRequests{}, fmt.Errorf("compose master data decisions: %w", err)
	}
	if requests.ParentRequests, err = NewParentRequestCoordinator(
		requests.MasterData, requests.Excused, requests.CareRequests, NewOfferingChangeConflicts(requests.OfferingChanges),
		events, permissions,
	); err != nil {
		return CarePlanRequests{}, fmt.Errorf("compose parent request coordinator: %w", err)
	}
	return requests, nil
}
