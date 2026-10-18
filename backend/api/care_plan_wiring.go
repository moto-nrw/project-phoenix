package api

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/observability"
)

// carePlanWiringOwners are the owners the root has composed when it builds
// the Care Plan wiring of the service graph (#4011).
type carePlanWiringOwners struct {
	DB         transactionruntime.Database
	Logger     *slog.Logger
	CarePlan   careplan.Capability
	Persons    peopledirectory.Capability
	Membership schoolmembership.Capability
	Timetable  timetable.Capability
	Calendar   schoolcalendar.Query
	// People are the retained People Directory repositories the root built
	// for the graph; AuditActor names the actor of a child's change history.
	People     PeopleWiring
	AuditActor peoplePorts.RequestAuditActor
	// ObserveCarePlan records the excused-absence workflow as Care Plan
	// operations; nil leaves it unobserved.
	ObserveCarePlan func(carePlanCompose.Observation)
}

// carePlanRoot is the Care Plan wiring the graph takes, and the companion
// graph the root mounts on the student routes.
type carePlanRoot struct {
	wiring     CarePlanWiring
	companions careplan.StudentCompanions
}

// newCarePlanWiring binds Care Plan to the owners the root composed, and
// hands the graph the bindings over the values only it composes. The graph
// takes the result in place of building it itself (ADR 0053 point 8).
func newCarePlanWiring(owners carePlanWiringOwners) (carePlanRoot, error) {
	if owners.CarePlan == nil || owners.Persons == nil || owners.Membership == nil || owners.Timetable == nil ||
		owners.Calendar == nil || owners.People.Students == nil || owners.People.Persons == nil || owners.People.StudentAudit == nil ||
		owners.AuditActor == nil {
		return carePlanRoot{}, errors.New("care plan wiring: the Care Plan, People Directory, School Membership, Timetable and School Calendar owners and the audit actor are required")
	}
	logger := owners.Logger
	if logger == nil {
		logger = slog.Default()
	}
	pool := owners.DB.Pool()
	carePlan, persons, people := owners.CarePlan, owners.Persons, owners.People

	// The arrival mirror: the class timetable supplies the regular arrival
	// time of the arrival baselines and schedules (#2414, ADR 0005).
	classArrivals, err := timetableCompose.NewClassArrivals(pool, func(observation timetableCompose.Observation) {
		logger.Debug("class arrival query",
			"operation", observation.Operation,
			"duration", observation.Duration,
			"queries", observation.Stats.Queries,
			"rows", observation.Stats.Rows,
			"error", observation.Err)
	})
	if err != nil {
		return carePlanRoot{}, err
	}
	classExceptions, err := NewClassArrivalExceptions(classArrivals, persons)
	if err != nil {
		return carePlanRoot{}, err
	}
	arrivalRules, arrivalDependencies, err := ArrivalScheduleBinding(persons, ClassArrivalPlans(classArrivals), classExceptions, logger.With("service", "arrival-schedule"))
	if err != nil {
		return carePlanRoot{}, err
	}
	pickupStudents, err := PickupPlanStudents(persons)
	if err != nil {
		return carePlanRoot{}, err
	}
	studentAudit := peopleCompose.NewStudentAuditService(owners.AuditActor, people.StudentAudit)
	// The enrollment change requests' multi-child approval takes the
	// companion lock order through the same companion graph.
	companions, err := carePlanCompose.NewCompanions(carePlan, NewCompanionStudents(people.Students, persons, studentAudit))
	if err != nil {
		return carePlanRoot{}, err
	}
	lifecycleOwners := NewCareLifecycleOwners(
		people.Students, people.Persons, owners.Membership, persons, owners.Timetable, owners.Calendar,
		NewUnloggedStudentPresence(owners.DB), newPresenceProjectionReads(owners.DB),
		NewEnrollmentBookingProjection(NewEnrollmentBookings()), NewCareEndRecorder(studentAudit),
	)

	wiring := CarePlanWiring{
		StatusDays:             NewStudentStatusDayRepository(carePlan),
		PickupStudents:         pickupStudents,
		PickupStudentLocks:     pickupStudentLocks{students: persons},
		PickupExcusalTimetable: NewPickupExcusalTimetable(owners.Timetable, NewPartialAbsencePreview(owners.DB)),
		ArrivalRules:           arrivalRules,
		ArrivalDependencies:    arrivalDependencies,
		LifecycleOwners:        lifecycleOwners,
		CompanionGraph:         companionGraphCoordinator{CompanionLocks: companions, strandings: people.Students},
		CareScheduleRequests:   NewCareScheduleChangeRequestRepository(carePlan),
		ExcusedRequestRows:     NewExcusedAbsenceRequestRepository(carePlan),

		CareParticipation: CareParticipationResolver,
		ArrivalBaselines: func(approved careplan.ApprovedBookingReader, bookingsAuthoritative func(context.Context) (bool, error)) (careplan.ArrivalBaselineReader, error) {
			return NewArrivalBaselines(carePlan, persons, classArrivals, approved, bookingsAuthoritative)
		},
		OfferingCatalog: func(sources CareOfferingCatalogSources) (careplan.CareOfferingCatalogCapability, error) {
			sourcedTemplates, pickup := lateCareOfferingResync(sources.Resync)
			return newCareOfferingCatalog(careOfferingCatalogInputs{
				Records: carePlan, Phases: sources.Enrollment, Bookings: sources.Enrollment,
				Timetable: owners.Timetable, Calendar: owners.Calendar, Settings: sources.Settings,
				SourcedTemplates: sourcedTemplates, Pickup: pickup,
				LockRecurrence: sources.LockRecurrence, Today: sources.Today, Logger: sources.Logger,
			})
		},
		OfferingPhaseGuard: careOfferingPhaseGuard,
		BookingMaterialization: func(sources BookingMaterializationSources) (careplan.BookingMaterializationCapability, error) {
			deps, err := bookingMaterializationOwners(
				owners.Timetable, sources.Rosters, persons, owners.Calendar, sources.Enrollment, sources.Approved,
				sources.Adjustments, people.Persons, sources.Accounts, sources.Pickup, carePlan,
			)
			if err != nil {
				return nil, err
			}
			deps.Catalog = sources.Catalog
			deps.Settings = settingsModule.NewCareBookingValues(sources.Settings)
			deps.Bookings = carePlan
			deps.Withdrawals = sources.Withdrawals
			deps.LockTemplateRecurrence = sources.LockRecurrence
			deps.ResyncPickupAutoExcusals = sources.ResyncPickupAutoExcusals
			deps.ClearPickupWeekdayExtension = owners.Timetable.ClearPickupWeekdayExtension
			deps.PickupGuardians = sources.GuardianNotifier
			deps.Today = sources.Today
			deps.Logger = sources.Logger
			return newBookingMaterialization(deps, sources.Broadcaster)
		},
		OfferingRosterResync: timetableOfferingRosterResync,
		StudentDocuments: func(sources StudentDocumentSources) (careplan.StudentDocuments, error) {
			deps, err := StudentDocumentDependencies(carePlan, people.Students, sources.Caller, sources.Edits, sources.AccessLog)
			if err != nil {
				return nil, err
			}
			deps.DB = pool
			return carePlanCompose.NewStudentDocuments(deps)
		},
		ReviewPolicy: func(reviews identityaccess.ParentRequestReviews) CarePlanReviewPolicy {
			return NewParentRequestReviewPolicy(reviews)
		},
		CareRequests: func(sources CareRequestSources) carerequests.Service {
			return NewCareScheduleRequestServiceWithPickupChangesAndPolicy(
				carePlan, persons, sources.Arrivals, sources.Pickups, NewLoggedStudentPresence(owners.DB, logger),
				sources.AutoExcusal, carePlan, sources.Staff, sources.Emitter, sources.Broadcaster,
				sources.Reviews, sources.Permissions, sources.Events, sources.Logger, sources.Audit,
				WithRequestShareVisibility(sources.Shares), WithCareRequestToday(sources.Today),
			)
		},
		PickupAdjustments: func(sources PickupAdjustmentSources) (careplan.PickupAdjustments, error) {
			return NewPickupAdjustments(
				carePlan, sources.PickupSchedules, sources.ArrivalSchedules, sources.Baselines, sources.Offerings,
				sources.Settings, sources.Audit, people.Students, sources.Today,
			)
		},
		ExcusedRequests: func(sources ExcusedRequestSources) (*carePlanCompose.ExcusedAbsenceRequests, error) {
			return NewExcusedAbsenceRequestService(
				carePlan, people.Students, people.Persons, ParentRequestReviewScope(sources.Reviews, sources.Permissions),
				sources.Emitter, sources.Broadcaster, sources.Events, sources.Notifier, sources.Shares,
				owners.ObserveCarePlan, sources.Logger, nil,
			)
		},
		SettingsSideEffects: RegisterCareWithdrawalSettingsSideEffects,
	}
	return carePlanRoot{wiring: wiring, companions: companions}, nil
}

// pickupStudentLocks takes the People Directory student lock a weekly pickup
// approval holds. A student that no longer exists reports not found.
type pickupStudentLocks struct {
	students interface {
		LockStudent(ctx context.Context, studentID int64) error
	}
}

func (l pickupStudentLocks) LockStudent(ctx context.Context, studentID int64) (bool, error) {
	err := l.students.LockStudent(ctx, studentID)
	if errors.Is(err, peopledirectory.ErrStudentNotFound) {
		return false, nil
	}
	return err == nil, err
}

// observeCarePlan records the excused-absence workflow as Care Plan
// operations.
func observeCarePlan(observation carePlanCompose.Observation) {
	observability.ObserveCarePlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.Conflicts, observation.Stats.StatementDuration, careplan.ErrorCode(observation.Err), observation.Err)
}
