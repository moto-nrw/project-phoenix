package api

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	careStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/careplan/http"
	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	groupLiveHTTP "github.com/moto-nrw/project-phoenix/modules/grouplive/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	studentsAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
	projectionStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/presenceprojection/http"
	requestreviewcompose "github.com/moto-nrw/project-phoenix/modules/requestreview/compose"
	reviewStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/requestreview/http"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	reviewsettings "github.com/moto-nrw/project-phoenix/modules/settings/review"
	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/studentpresence/http"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/realtime"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	reviewbindings "github.com/moto-nrw/project-phoenix/roots/requestreviewbinding"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
	deleteStudentsHTTP "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/http"
)

// requestReviewDependencies binds native owner capabilities for the staff projection.
func requestReviewDependencies(
	db transactionruntime.Database, persons peopledirectory.Capability, reviewPolicy *owners.ParentRequestReviewPolicy, caller *owners.CallerRows,
	settings settingsCompose.SettingsService, excused *carePlanCompose.ExcusedAbsenceRequests,
) (requestreviewcompose.ProjectionDependencies, carePlanModule.CareScheduleReviewQuery, error) {
	pool := db.Pool()
	reviewGroups, err := schoolStructureCompose.New(schoolStructureCompose.Dependencies{DB: pool, Observe: func(observation schoolStructureCompose.Observation) {
		observability.ObserveSchoolStructureOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, schoolstructure.ErrorCode(observation.Err), observation.Err)
	}})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("request review groups: %w", err)
	}
	reviewStudents, err := reviewbindings.NewStudentDirectory(persons, reviewGroups)
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("request review student directory: %w", err)
	}
	// The staff projection lists exactly what the decisions accept: one
	// Identity & Access review policy serves both (#3804).
	reviewScope := func(ctx context.Context) (carePlanCompose.ReviewScope, error) {
		schoolWide, groupIDs, err := reviewPolicy.Scope(ctx, sessiontoken.PermissionsFromCtx(ctx))
		return carePlanCompose.ReviewScope{SchoolWide: schoolWide, GroupIDs: groupIDs}, err
	}
	// Report the union of ordinary and absence-review rights, as the
	// navigation capability does. Each native queue keeps its own scope.
	reviewAccess, err := reviewbindings.NewAccess(reviewStudentsHTTP.RequestReviewPrincipal, func(ctx context.Context) (string, error) {
		return reviewPolicy.AccessLevel(ctx, sessiontoken.PermissionsFromCtx(ctx))
	})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("request review access: %w", err)
	}
	masterDataReviews, err := NewRequestReviewMasterData(db,
		studentDirectoryView.NewReader(),
		persons,
		reviewScope,
		carePlanModule.Today,
		func(observation carePlanCompose.Observation) {
			observability.ObserveCarePlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.Conflicts, observation.Stats.StatementDuration, carePlanModule.ErrorCode(observation.Err), observation.Err)
		})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("master data review queue: %w", err)
	}
	careReviews, err := NewRequestReviewSchedules(db,
		studentDirectoryView.NewReader(),
		persons,
		reviewScope,
		func(ctx context.Context) (bool, error) {
			return settings.ResolveBool(ctx, reviewsettings.BookingsAuthoritative)
		},
		carePlanModule.Today,
		func(observation carePlanCompose.Observation) {
			observability.ObserveCarePlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.Conflicts, observation.Stats.StatementDuration, carePlanModule.ErrorCode(observation.Err), observation.Err)
		},
		func(observation timetableCompose.Observation) {
			observability.ObserveTimetableActivitiesOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.DuplicatePreventionConflicts, observation.Stats.StatementDuration, timetableModule.ErrorCode(observation.Err), observation.Err)
		},
		NewPickupReviewBlocks(db))
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("care schedule reviews: %w", err)
	}
	careQueue, err := reviewbindings.NewCareScheduleQueue(careReviews, carePlanModule.Today)
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("care schedule review queue: %w", err)
	}
	offeringReviews, err := NewRequestReviewOfferings(db,
		persons,
		reviewScope,
		carePlanModule.Today,
		func(observation carePlanCompose.Observation) {
			observability.ObserveCarePlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.Conflicts, observation.Stats.StatementDuration, carePlanModule.ErrorCode(observation.Err), observation.Err)
		},
		func(observation timetableCompose.Observation) {
			observability.ObserveTimetableActivitiesOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.DuplicatePreventionConflicts, observation.Stats.StatementDuration, timetableModule.ErrorCode(observation.Err), observation.Err)
		})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("offering reviews: %w", err)
	}
	offeringQueue, err := reviewbindings.NewOfferingQueue(offeringReviews, carePlanModule.Today)
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("offering review queue: %w", err)
	}
	correctionQueries, err := auditCompose.NewCorrectionQueries(pool, func(auditlog.Observation) {})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("request review corrections: %w", err)
	}
	corrections, err := reviewbindings.NewCorrectionLog(correctionQueries, persons, func(ctx context.Context) bool {
		return reviewStudentsHTTP.RequestReviewCorrectionAccess(ctx, caller.HasCurrentStaff)
	})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("direct correction history: %w", err)
	}
	masterQueue, err := reviewbindings.NewMasterDataQueue(masterDataReviews)
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("master data review queue: %w", err)
	}
	excusedQueue, err := reviewbindings.NewExcusedQueue(excused, carePlanModule.Today)
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("excused review queue: %w", err)
	}
	return requestreviewcompose.ProjectionDependencies{
		Queues:   requestreviewcompose.Queues{DirectCorrections: corrections, MasterData: masterQueue, CareSchedule: careQueue, Offering: offeringQueue, Excused: excusedQueue},
		Students: reviewStudents, FamilyProtection: reviewbindings.NewFamilyProtection(persons), Access: reviewAccess,
		Submitters: reviewbindings.NewSubmitterDirectory(persons),
	}, careReviews, nil
}

// mountStudentRoutes serves /api/students.
func mountStudentRoutes(
	tenant chi.Router, db transactionruntime.Database, logger *slog.Logger, deviceAuth *deviceauth.Authenticators, presence studentpresence.Owner,
	persons peopledirectory.Capability, services owners.PeopleServices, membership schoolMembershipModule.Owner, caller *owners.CallerRows,
	reviewPolicy *owners.ParentRequestReviewPolicy, settings settingsCompose.SettingsService, hub *realtime.Hub, carePlan carePlanModule.Capability,
	companions carePlanModule.StudentCompanions, schedules owners.CarePlanSchedules, requests owners.CarePlanRequests, planning TimetablePlanning,
	emitter eventContract.Emitter, activities activityContract.ActivityService, lifecycle owners.TimetableLifecycle,
	reports owners.PresenceReports, structureViews StructureServices, organizations organizationtenancy.Query,
	statusDays studentpresence.StatusDays, absences deliveryCompose.AbsenceNotifier, deletion *studentdeletion.Workflow,
	groupLive grouplive.Query, decisions enrollmentOwner.Decisions, active studentpresence.Presence, resumption careStudentsHTTP.CareResumption,
) error {
	// A direct school_class edit must resync Jahrgang-filtered offering-sourced
	// Regeltermine like a grade transition does (#2147 review round 10); Care
	// Plan's booking materialization provides the resync (#3560).
	var studentClassResyncer schoolStructureCompose.OfferingSourceResyncer = requests.CareOfferings
	reviewDependencies, careReviews, err := requestReviewDependencies(db, persons, reviewPolicy, caller, settings, requests.Excused)
	if err != nil {
		return err
	}
	requestReview, err := requestreviewcompose.NewProjection(reviewDependencies)
	if err != nil {
		return fmt.Errorf("request review projection: %w", err)
	}
	students, careStudents, projectionStudents, presenceStudents, deletionStudents, requestStudents := NewStudentHTTPResources(
		studentsAPI.ResourceConfig{
			ActiveEnrollments:      studentActiveEnrollments{enrollments: activities},
			AuthenticatedDevice:    deviceauth.DeviceID,
			ChildQuota:             childQuotaStudentsReader{usages: membership},
			ClassListEntries:       classListEntryStudentsReader{entries: membership},
			DeviceAuthenticator:    deviceAuth.Device(),
			FamilyProtection:       persons,
			Logger:                 logger.With("handler", "students"),
			PeopleDirectory:        persons,
			Persons:                peopleCompose.NewStudentRoutePersons(services.Users),
			StudentAuditService:    persons,
			StudentDocumentService: requests.Documents,
			StudentNotes:           persons,
			StudentPhotos:          persons,
			Uploads:                nativeUploads.UploadFiles{},
			UserContextService:     caller,
		},
		careStudentsHTTP.ResourceConfig{
			ArrivalScheduleService: schedules.Arrivals,
			Broadcaster:            hub,
			CareLifecycleService:   schedules.Lifecycle, CareResumption: resumption,
			CareRequestReviews:           careReviews,
			CareRequestService:           requests.CareRequests,
			CompanionService:             companions,
			DocumentStorage:              NewStudentDocumentStorage(),
			ExcusedRequestService:        requests.Excused,
			LockTemplateRecurrence:       planning.Planning.RecurrenceLock.LockRecurrenceWrites,
			Logger:                       logger.With("handler", "students"),
			MasterDataReviewService:      requests.MasterData,
			OfferingChangeService:        requests.CareOfferings,
			OfferingPickupTimes:          requests.CareOfferings,
			OfferingSourceResyncer:       studentClassResyncer,
			ParentEventEmitter:           emitter,
			ParentRequestBulkService:     requests.ParentRequests,
			ParentRequestConflictService: requests.ParentRequests,
			PartialAbsenceService:        schedules.PartialAbsences,
			PeopleDirectory:              persons,
			Persons:                      peopleCompose.NewStudentRoutePersons(services.Users),
			PickupAdjustmentService:      requests.CareOfferings,
			PickupScheduleService:        schedules.Pickups,
			SettingsService:              settings,
			StudentAuditService:          persons,
			StudentConsents:              services.StudentConsents,
			StudentDocumentService:       requests.Documents,
			StudentPhotos:                persons,
			UserContextService:           caller,
			WeekdayPickupNotes:           carePlan,
		},
		projectionStudentsHTTP.ResourceConfig{
			AbsenceOverview:        reports.AbsenceOverview,
			ActiveEnrollments:      studentActiveEnrollments{enrollments: activities},
			ActiveService:          active,
			ArrivalScheduleService: schedules.Arrivals,
			CareDayService:         schedules.CareDay,
			CareLifecycleService:   schedules.Lifecycle, ClassHandovers: studentListHandovers{reads: classHandoverReads{handovers: presence, staff: NewTimetablePeople(services.Users)}},
			ClassListEntries:        classListEntryStudentsReader{entries: membership},
			CompanionService:        companions,
			ExcusedRequestService:   requests.Excused,
			InstanceService:         lifecycle.Instances,
			ListExportService:       lists.NewRenderer(),
			Logger:                  logger.With("handler", "students"),
			OfferingDayRoster:       schedules.OfferingDays,
			PeopleDirectory:         persons,
			Persons:                 peopleCompose.NewStudentRoutePersons(services.Users),
			PickupScheduleService:   schedules.Pickups,
			SchoolGroups:            studentSchoolGroups{GroupManagement: structureViews.Education},
			SchoolService:           studentSchoolDirectory{schools: organizations},
			SettingsService:         settings,
			StudentConsents:         services.StudentConsents,
			StudentHistoryService:   reports.StudentHistory,
			StudentStatusDayService: statusDays,
			UserContextService:      caller,
		},
		presenceStudentsHTTP.ResourceConfig{
			AbsenceNotifier:         studentAbsenceNotifier{notifier: absences},
			ActiveService:           active,
			Broadcaster:             hub,
			Logger:                  logger.With("handler", "students"),
			ParentEventEmitter:      emitter,
			PeopleDirectory:         persons,
			Persons:                 peopleCompose.NewStudentRoutePersons(services.Users),
			PrivacyConsents:         presence,
			SettingsService:         settings,
			StudentStatusDayService: statusDays,
			UserContextService:      caller,
		},
		deleteStudentsHTTP.ResourceConfig{
			CareLifecycleService: schedules.Lifecycle,
			ConstraintViolation:  transactionFailures.IsConstraintViolation,
			PeopleDirectory:      persons,
			StudentDeletion:      deletion,
			UserContextService:   caller,
		},
		reviewStudentsHTTP.ResourceConfig{
			PeopleDirectory:     persons,
			RequestReview:       requestReview,
			RequestReviewAccess: reviewPolicy,
		},
	)
	tenant.Mount("/students", students.Router(ProtectedTenantRoutesWith(
		groupLiveHTTP.NewResource(groupLive).Register,
		careStudents.Register, projectionStudents.Register, presenceStudents.Register, deletionStudents.Register, requestStudents.Register,
		(&enrollmentAPI.StudentResource{EnrollmentDecision: decisions, EnrollmentFormSchema: newEnrollmentFormSchemas(settings, logger), StudentReadAccess: students.StudentReadAccess, Logger: logger.With("handler", "students")}).Register,
	)))
	return nil
}

// newEnrollmentResource builds Enrollment's staff and public routes. The
// root builds the form schemas, the captcha and the phase expiry warnings
// here, over the settings and the owners it composed (#4012).
func newEnrollmentResource(
	logger *slog.Logger, settings settingsCompose.SettingsService, turnstile TurnstileKeys, careOfferings carePlanModule.CareOfferingCapability,
	flows owners.EnrollmentFlows, phases enrollmentOwner.PhaseAdministration, decisions enrollmentOwner.Decisions,
	deletions enrollmentOwner.EnrollmentDeletions, identity identityaccess.Owner,
	guardianProfiles *peopleCompose.GuardianProfileLoader, organizations organizationtenancy.Query,
	renderer *lists.RendererService, persons peopledirectory.Capability, carePlan carePlanModule.Capability,
) *enrollmentAPI.Resource {
	resource := enrollmentAPI.NewResource(
		newEnrollmentFormSchemas(settings, logger),
		enrollmentAPI.NewCareOfferingCatalog(NewEnrollmentCareOfferingValues(careOfferings)),
		enrollmentAPI.NewRequestService(flows.Intake),
		newEnrollmentCaptcha(settings, turnstile, logger),
		phases,
		enrollmentAPI.NewDecisionService(decisions),
		flows.Reports,
		enrollmentAPI.NewRolloverService(flows.Rollovers),
		enrollmentAPI.NewChangeRequestService(flows.ChangeRequests),
		deletions,
		enrollmentGuardianInvitations(identity),
		NewEnrollmentGuardianAutofill(guardianProfiles),
		enrollmentSchoolDirectory{schools: organizations},
		nativeUploads.UploadFiles{},
		enrollmentAPI.DatabaseFailures{Transient: transactionFailures.IsTransientDatabaseError, Constraint: transactionFailures.IsConstraintViolation},
		owners.NewEnrollmentBookings(),
	)
	resource.ListExportService = renderer
	resource.PhaseExpiryService = newEnrollmentPhaseExpiry(persons, carePlan)
	return resource
}
