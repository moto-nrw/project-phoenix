package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	groupsHTTP "github.com/moto-nrw/project-phoenix/modules/schoolstructure/http"

	"github.com/moto-nrw/project-phoenix/modules/dataimport/fileformat"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	schoolSetupCompose "github.com/moto-nrw/project-phoenix/modules/schoolsetup/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	jwxjwt "github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/lestrrat-go/jwx/v3/transform"
	slogchi "github.com/samber/slog-chi"
	"github.com/spf13/viper"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/analytics"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	configAPI "github.com/moto-nrw/project-phoenix/api/config"
	iotAPI "github.com/moto-nrw/project-phoenix/api/iot/compose"
	operatorAPI "github.com/moto-nrw/project-phoenix/api/operator"
	platformAPI "github.com/moto-nrw/project-phoenix/api/platform"
	remindersAPI "github.com/moto-nrw/project-phoenix/api/reminders"
	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	customMiddleware "github.com/moto-nrw/project-phoenix/middleware"
	appointmentsModule "github.com/moto-nrw/project-phoenix/modules/appointments"
	appointmentsCompose "github.com/moto-nrw/project-phoenix/modules/appointments/compose"
	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	parentAPI "github.com/moto-nrw/project-phoenix/modules/careplan/inbound/parent"
	requestFeedCompose "github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/compose"
	requestFeedHTTP "github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/http"
	classdayHTTP "github.com/moto-nrw/project-phoenix/modules/classday/http"
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	announcementAPI "github.com/moto-nrw/project-phoenix/modules/communication/http/parentannouncements"
	messagingAPI "github.com/moto-nrw/project-phoenix/modules/communication/http/parentmessages"
	staffMessagingAPI "github.com/moto-nrw/project-phoenix/modules/communication/http/staffmessages"
	importAPI "github.com/moto-nrw/project-phoenix/modules/dataimport/inbound"
	importCompose "github.com/moto-nrw/project-phoenix/modules/dataimport/inbound/compose"
	notificationsAPI "github.com/moto-nrw/project-phoenix/modules/delivery/http/notifications"
	sseAPI "github.com/moto-nrw/project-phoenix/modules/delivery/http/sse"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	displayHTTPAdapter "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/httpadapter"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"
	tagScanOperatorAPI "github.com/moto-nrw/project-phoenix/modules/devicefleet/inbound/operator"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	emergencyAPI "github.com/moto-nrw/project-phoenix/modules/emergencysnapshot/http"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	roomsHTTPAdapter "github.com/moto-nrw/project-phoenix/modules/facilities/compose/httpadapter"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	feedbackCompose "github.com/moto-nrw/project-phoenix/modules/feedback/compose"
	feedbackAPI "github.com/moto-nrw/project-phoenix/modules/feedback/http"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	filestoreAPI "github.com/moto-nrw/project-phoenix/modules/filestorage/http/files"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	meAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/me"
	identityOperatorAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
	projectJWT "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	mealplanCompose "github.com/moto-nrw/project-phoenix/modules/mealplan/compose"
	mealplanAPI "github.com/moto-nrw/project-phoenix/modules/mealplan/http"
	organizationModule "github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	usersAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/http"
	birthdaysAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/birthdays"
	studentsAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
	requestreviewcompose "github.com/moto-nrw/project-phoenix/modules/requestreview/compose"
	schoolCalendarModule "github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	calendarService "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	schoolPortal "github.com/moto-nrw/project-phoenix/modules/schoolportal"
	schoolStructureModule "github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	reviewsettings "github.com/moto-nrw/project-phoenix/modules/settings/review"
	calendarAPI "github.com/moto-nrw/project-phoenix/modules/staffcalendar/http"
	statisticsAPI "github.com/moto-nrw/project-phoenix/modules/statistics/http"
	presenceAPI "github.com/moto-nrw/project-phoenix/modules/studentpresence/inbound/presence"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetableHTTPAdapter "github.com/moto-nrw/project-phoenix/modules/timetable/compose/httpadapter"
	timetableAPI "github.com/moto-nrw/project-phoenix/modules/timetable/http"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	worktimemodelsHTTPAdapter "github.com/moto-nrw/project-phoenix/modules/workforce/compose/httpadapter"
	workforceInbound "github.com/moto-nrw/project-phoenix/modules/workforce/inbound"
	workforceShiftPlanning "github.com/moto-nrw/project-phoenix/modules/workforce/inbound/shiftplanning"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/scheduler"
	gradeTransitionHTTP "github.com/moto-nrw/project-phoenix/workflows/gradetransition/http"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove"
	reminderCompose "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/compose"
)

// recordHTTPRuntimeEvent turns one runtime event into a metric and, for
// failures, one ERROR record carrying method, route, path and status. A
// rolled-back transaction is only a failure when the client got a 5xx: the
// tenant middleware and service-owned transactions also roll back behind
// 401/404/410 responses, and those must not feed the error-spike alert
// (#2953). The slog-chi request line already records every 4xx at WARN.
type httpRuntimeObservation = apiCommon.TenantRuntimeObservation

func recordHTTPRuntimeEvent(tracer *observability.Tracer, observation httpRuntimeObservation) {
	event, r, status := observation.Event, observation.Request, observation.Status
	observability.RecordUnitOfWorkEvent(
		"http",
		string(event.Kind),
		string(event.Result),
		event.Duration,
		event.Retries,
	)
	ctx := r.Context()
	attrs := []slog.Attr{
		slog.String("method", r.Method),
		slog.String("route", observation.Route),
		slog.String("path", customMiddleware.RedactFeedToken(r.URL.Path)),
		slog.Int("status", status),
	}
	switch {
	case event.Kind == apiCommon.TenantRuntimeMissingTenant:
		tracer.Failure(ctx, "http", string(event.Kind), "missing_tenant", event.Err, attrs...)
	case event.Kind == apiCommon.TenantRuntimeTransaction && event.Err != nil && status >= http.StatusInternalServerError:
		attrs = append(attrs, slog.String("result", string(event.Result)))
		tracer.Failure(ctx, "http", string(event.Kind), "transaction_failure", event.Err, attrs...)
	case event.Kind == apiCommon.TenantRuntimeResponseWrite && event.Err != nil:
		tracer.Failure(ctx, "http", string(event.Kind), "response_write_failure", event.Err, attrs...)
	}
}

type moduleServices struct {
	repositories  *repositories.Factory
	services      *services.Factory
	demoAccess    authAPI.DemoAccesses
	communication *communicationModule.Module
	mealPlan      *mealplanModule.Module
	feedback      *feedbackModule.Module
	persons       *peopleModule.Module
	rooms         *facilitiesModule.Module
	timetable     *timetableModule.Module
	// calendar owns schedule.calendar_periods, schedule.closing_days and
	// schedule.dateframes and answers the tenant's non-working days.
	calendar *schoolCalendarModule.Module
	// membership owns users.staff, users.teachers and users.guests (#2667).
	membership *schoolMembershipModule.Module
	// workforce owns the work-time templates and staff schedule versions
	// in config.work_time_models, config.work_time_model_entries and
	// config.staff_work_schedules (#2687).
	workforce *workforceModule.Module
	// studentPhotoRuntime is the slot the People Directory photo lifecycle
	// resolves from; EnableStudentPhotos fills it once the factory exists.
	studentPhotoRuntime *peopleCompose.StudentPhotoRuntime
}

// NewCleanupTimetable composes the unobserved Timetable owner for CLI roots.
// The serving root uses initializeModuleServices so it can attach metrics.
func NewCleanupTimetable(db *bun.DB) (timetableModule.Capability, error) {
	students, err := peopleCompose.New(peopleCompose.Dependencies{DB: db, Observe: func(peopleCompose.Observation) {}})
	if err != nil {
		return nil, err
	}
	rooms, err := repositories.NewFacilities(db)
	if err != nil {
		return nil, err
	}
	return repositories.NewTimetable(db, students, rooms)
}

func composeTimetable(db *bun.DB, persons *peopleModule.Module, rooms *facilitiesModule.Module, membership *schoolMembershipModule.Module) (*timetableModule.Module, error) {
	return timetableCompose.New(timetableCompose.Dependencies{
		LockStaffAssignment: func(ctx context.Context, staffID int64) error {
			_, err := membership.FindStaffForMutation(ctx, staffID)
			return err
		},
		DB: db, Students: timetableStudents(persons), Rooms: timetableRooms(rooms), Sessions: repositories.NewPresenceFacts(db),
		Observe: func(observation timetableCompose.Observation) {
			observability.ObserveTimetableActivitiesOperation(
				observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows,
				observation.Stats.DuplicatePreventionConflicts, observation.Stats.StatementDuration,
				timetableModule.ErrorCode(observation.Err), observation.Err,
			)
		},
	})
}

func initializeModuleServices(db *bun.DB, publicAPIURL string, logger *slog.Logger, tenantRuntime apiCommon.TenantRuntime) (moduleServices, error) {
	organizations, err := organizationCompose.New(organizationCompose.Dependencies{
		DB: db,
		Observe: func(observation organizationCompose.Observation) {
			observability.ObserveOrganizationTenancyOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, organizationModule.ErrorCode(observation.Err), observation.Err)
		},
	})
	if err != nil {
		return moduleServices{}, err
	}
	// The photo lifecycle's runtime (feature gate, caller access, file
	// cleanup, live refresh, consent trail) only exists once the services
	// factory and the HTTP layer are up, so the owner resolves it from a slot
	// that EnableStudentPhotos fills.
	persons, studentPhotoRuntime, err := repositories.NewPeopleDirectoryWithPhotosAndObserver(db, func(observation peopleCompose.Observation) {
		observability.ObservePeopleDirectoryOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, peopleModule.ErrorCode(observation.Err), observation.Err)
	})
	if err != nil {
		return moduleServices{}, err
	}
	groups, err := schoolStructureCompose.New(schoolStructureCompose.Dependencies{
		DB: db,
		Observe: func(observation schoolStructureCompose.Observation) {
			observability.ObserveSchoolStructureOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, schoolStructureModule.ErrorCode(observation.Err), observation.Err)
		},
	})
	if err != nil {
		return moduleServices{}, err
	}
	var legacyFacilities interface {
		ValidateRoomDeletion(context.Context, int64) error
	}
	rooms, err := composeFacilities(db, &legacyFacilities)
	if err != nil {
		return moduleServices{}, err
	}
	// A staff member is School Membership's membership row plus Workforce's
	// employment profile (#2753); the membership owner composes the two.
	staffEmployment, err := workforceCompose.NewStaffEmployment(db, func(observation workforceCompose.Observation) {
		observability.ObserveWorkforceOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, workforceModule.ErrorCode(observation.Err), observation.Err)
	})
	if err != nil {
		return moduleServices{}, err
	}
	membership, err := schoolMembershipCompose.New(schoolMembershipCompose.Dependencies{
		DB: db,
		Observe: func(observation schoolMembershipCompose.Observation) {
			observability.ObserveSchoolMembershipOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, schoolMembershipModule.ErrorCode(observation.Err), observation.Err)
		},
		Employment: repositories.MembershipStaffEmployment(staffEmployment),
		// Every counting membership write is checked against the school's
		// Kinderkontingent, which Organisation & Tenancy owns (#3567).
		ChildQuota: organizationCompose.NewChildQuotaLimits(),
	})
	if err != nil {
		return moduleServices{}, err
	}
	// The period administration resolves the recurrence gate, the
	// care-offering guard and the federal-state setting on every call from
	// the runtime the services factory answers with below; the guard is an
	// Enrollment service that itself reads this calendar, so it cannot exist
	// before the factory does.
	var calendarAdministration schoolCalendarCompose.AdministrationRuntime
	calendar, err := schoolCalendarCompose.New(schoolCalendarCompose.Dependencies{
		DB: db,
		Observe: func(observation schoolCalendarCompose.Observation) {
			observability.ObserveSchoolCalendarOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, schoolCalendarModule.ErrorCode(observation.Err), observation.Err)
		},
		Administration: func() schoolCalendarCompose.AdministrationRuntime { return calendarAdministration },
	})
	if err != nil {
		return moduleServices{}, err
	}
	timetableCapability, err := composeTimetable(db, persons, rooms, membership)
	if err != nil {
		return moduleServices{}, err
	}
	workTime, err := workforceCompose.New(workforceCompose.Dependencies{
		LockStaffAssignment: func(ctx context.Context, staffID int64) error {
			_, err := membership.FindStaffForMutation(ctx, staffID)
			return err
		},
		DB:           db,
		LiveStaffIDs: repositories.WorkforceLiveStaffIDs(membership),
		// A Sonderarbeitszeit never sets a target on a statutory holiday.
		StatutoryHolidays: calendar.TenantHolidayDates,
		Observe: func(observation workforceCompose.Observation) {
			observability.ObserveWorkforceOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, workforceModule.ErrorCode(observation.Err), observation.Err)
		},
	})
	if err != nil {
		return moduleServices{}, err
	}
	repoFactory := repositories.NewFactory(db, repositories.TimetableDependencies{
		Capability: timetableCapability, Students: persons, Groups: groups, Rooms: rooms, Calendar: calendar, Membership: membership, Workforce: workTime,
		ObserveIdentityAccess: observability.ObserveIdentityAccessOperation,
	})
	appointmentCapability, err := appointmentsCompose.New(appointmentsCompose.Dependencies{
		DB: db,
		Observe: func(observation appointmentsCompose.Observation) {
			observability.ObserveAppointmentsOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.DuplicatePreventionConflicts, observation.Stats.StatementDuration, appointmentsModule.ErrorCode(observation.Err), observation.Err)
		},
	})
	if err != nil {
		return moduleServices{}, err
	}
	communicationCapability, err := communicationCompose.New(communicationCompose.Dependencies{
		DB:            db,
		Organizations: organizations,
		People:        persons,
		Audit:         communicationCompose.NewOperatorAnnouncementAudit(repoFactory.OperatorAuditLog.Create),
		Observe: func(observation communicationCompose.Observation) {
			observability.ObserveCommunicationOperation(
				observation.Operation,
				observation.Duration,
				int64(observation.Stats.Queries),
				observation.Stats.Rows,
				int64(observation.Stats.DuplicatePreventionConflicts),
				observation.Stats.StatementDuration,
				communicationModule.ErrorCode(observation.Err),
				observation.Err,
			)
		},
	})
	if err != nil {
		return moduleServices{}, err
	}
	carePlan, err := composeCarePlan(db, persons, repositories.CarePlanStatusSlots(repoFactory.InstanceStudent))
	if err != nil {
		return moduleServices{}, err
	}
	repoFactory.BindCarePlan(carePlan)
	mealPlanSettings := mealplanCompose.NewSettings()
	mealPlan, err := mealplanCompose.New(mealplanCompose.Dependencies{
		DB:       db,
		Settings: mealPlanSettings,
		Observe: func(observation mealplanCompose.Observation) {
			observability.ObserveMealPlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, observation.Err)
		},
		Now:          time.Now,
		Participants: mealPlanParticipantFinder(repoFactory, time.Now),
	})
	if err != nil {
		return moduleServices{}, err
	}
	feedbackSettings := feedbackCompose.NewSettings()
	feedbackCapability, err := feedbackCompose.New(feedbackCompose.Dependencies{
		DB:       db,
		Settings: feedbackSettings,
		Today:    feedbackModule.Today,
		Observe: func(observation feedbackCompose.Observation) {
			observability.ObserveFeedbackOperation(
				observation.Operation,
				observation.Duration,
				observation.Stats.Queries,
				observation.Stats.Rows,
				observation.Stats.StatementDuration,
				feedbackModule.ErrorCode(observation.Err),
				observation.Err,
			)
		},
	})
	if err != nil {
		return moduleServices{}, err
	}
	factory, err := withFileStorageWiring(func(fileStorageWiring services.FileStorageWiring) (*services.Factory, error) {
		return services.NewFactoryWithModules(
			repoFactory, db, logger, publicAPIURL, tenantRuntime,
			organizations, persons, groups, rooms, membership, calendar, timetableCapability, appointmentCapability,
			communicationCapability,
			func(observation communicationCompose.Observation) {
				observability.ObserveCommunicationOperation(
					observation.Operation,
					observation.Duration,
					int64(observation.Stats.Queries),
					observation.Stats.Rows,
					int64(observation.Stats.DuplicatePreventionConflicts),
					observation.Stats.StatementDuration,
					communicationModule.ErrorCode(observation.Err),
					observation.Err,
				)
			},
			func(observation carePlanCompose.Observation) {
				observability.ObserveCarePlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.Conflicts, observation.Stats.StatementDuration, carePlanModule.ErrorCode(observation.Err), observation.Err)
			},
			mealPlan, mealPlanSettings.Bind,
			feedbackCapability, feedbackSettings.Bind,
			observability.ObserveAuditAppend,
			observability.ObserveSynchronousDelivery,
			observability.ObserveDurableDelivery,
			observability.ObserveDeviceFleetOperation,
			observability.ObserveIdentityAccessOperation,
			workTime,
			observeDataImport,
			fileStorageWiring,
		)
	})
	if err != nil {
		return moduleServices{}, err
	}
	legacyFacilities = factory.Facilities
	calendarAdministration = factory.SchoolCalendarAdministration()
	return moduleServices{repositories: repoFactory, services: factory, demoAccess: authAPI.ComposedDemoAccess(factory.AccountAuthentication().DemoAccess()), communication: communicationCapability, mealPlan: mealPlan, feedback: feedbackCapability, persons: persons, rooms: rooms, timetable: timetableCapability, calendar: calendar, membership: membership, workforce: workTime, studentPhotoRuntime: studentPhotoRuntime}, nil
}

// withFileStorageWiring resolves what the File Storage module needs from the
// root and cannot compose itself (#2707): the uploads object store, rooted at
// the uploads directory every document feature shares, and the metrics sink.
// It hands both to the service factory, which composes the module where the
// announcement ports live.
func withFileStorageWiring(build func(services.FileStorageWiring) (*services.Factory, error)) (*services.Factory, error) {
	uploads, err := apiCommon.PrivateUploadsBackend()
	if err != nil {
		return nil, fmt.Errorf("resolve uploads backend: %w", err)
	}
	return build(services.FileStorageWiring{Objects: uploads, Observe: observeFileStorage})
}

func observeFileStorage(observation filestorageCompose.Observation) {
	observability.ObserveFileStorageOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, filestorageModule.ErrorCode(observation.Err), observation.Err)
}

func observeDataImport(observation services.DataImportObservation) {
	observability.ObserveDataImport(observation.Entity, observation.DryRun, observation.Rows, observation.Accepted, observation.Rejected, observation.Created, observation.Updated, observation.Duration)
	if observation.DryRun {
		return
	}
	observability.ObserveDataImportRuntime(observation.Entity, observation.BatchesCommitted, observation.BatchesRetried, observation.CheckpointLag, observation.Deadlocks, observation.PoolWait, observation.LockWait)
	for _, command := range observation.Commands {
		observability.ObserveDataImportCommand(observation.Entity, command.Owner, command.Operation, command.Duration, command.Failed)
	}
}

func composeFacilities(db *bun.DB, legacyFacilities *interface {
	ValidateRoomDeletion(context.Context, int64) error
}) (*facilitiesModule.Module, error) {
	recurrenceLock, err := repositories.NewTimetableRecurrenceLock(db)
	if err != nil {
		return nil, err
	}
	return facilitiesCompose.New(facilitiesCompose.Dependencies{
		DB:           db,
		DeletionLock: recurrenceLock.LockRecurrenceWrites,
		DeletionGuard: func(ctx context.Context, roomID int64) error {
			if *legacyFacilities == nil {
				return facilitiesModule.ErrRoomDeletionGuardUnavailable
			}
			return mapRoomDeletionError((*legacyFacilities).ValidateRoomDeletion(ctx, roomID))
		},
		Observe: func(observation facilitiesCompose.Observation) {
			observability.ObserveFacilitiesOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, facilitiesModule.ErrorCode(observation.Err), observation.Err)
		},
	})
}

func composeCarePlan(db *bun.DB, persons *peopleModule.Module, slots carePlanCompose.StatusSlotDirectory) (*carePlanModule.Module, error) {
	statusStudents, err := repositories.CarePlanStatusStudents(db, persons)
	if err != nil {
		return nil, err
	}
	return carePlanCompose.New(carePlanCompose.Dependencies{
		DB: db, AmbientDB: carePlanCompose.TenantAmbientDatabase(db),
		StatusStudents: statusStudents, StatusSlots: slots,
		People: carePlanCompose.StudentNameFinderFunc(func(ctx context.Context, ids []int64) ([]carePlanCompose.StudentName, error) {
			values, err := persons.ListStudentNamesByID(ctx, ids)
			if err != nil {
				return nil, err
			}
			result := make([]carePlanCompose.StudentName, 0, len(values))
			for _, value := range values {
				result = append(result, carePlanCompose.StudentName{StudentID: value.StudentID, FirstName: value.FirstName, LastName: value.LastName})
			}
			return result, nil
		}),
		StudentLock: persons.LockStudent, StudentNotFound: peopleModule.ErrStudentNotFound,
		Observe: func(observation carePlanCompose.Observation) {
			observability.ObserveCarePlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.Conflicts, observation.Stats.StatementDuration, carePlanModule.ErrorCode(observation.Err), observation.Err)
		},
	})
}

func mapRoomDeletionError(err error) error {
	var coded interface{ Code() string }
	if !errors.As(err, &coded) {
		return err
	}
	switch coded.Code() {
	case "room_in_use":
		return facilitiesModule.ErrRoomInUse
	case "room_required_by_offering":
		return facilitiesModule.ErrRoomRequiredByOffering
	default:
		return err
	}
}

func mealPlanParticipantFinder(repoFactory *repositories.Factory, now func() time.Time) mealplanCompose.ParticipantFinder {
	return func(ctx context.Context, date string) ([]mealplanCompose.ParticipantCandidate, error) {
		students, err := repoFactory.Student.FindOverlappingWithGroupsOnDate(ctx, date, now())
		if err != nil {
			return nil, err
		}
		candidates := make([]mealplanCompose.ParticipantCandidate, 0, len(students))
		for _, row := range students {
			if row == nil || row.Student == nil || row.Person == nil {
				continue
			}
			candidates = append(candidates, mealplanCompose.ParticipantCandidate{
				StudentID: row.ID, FirstName: row.Person.FirstName, LastName: row.Person.LastName, SchoolClass: row.SchoolClass,
			})
		}
		sort.SliceStable(candidates, func(left, right int) bool {
			return mealPlanParticipantLess(candidates[left], candidates[right])
		})
		return candidates, nil
	}
}

func mealPlanParticipantLess(left, right mealplanCompose.ParticipantCandidate) bool {
	if left.SchoolClass != right.SchoolClass {
		return left.SchoolClass < right.SchoolClass
	}
	if left.LastName != right.LastName {
		return left.LastName < right.LastName
	}
	if left.FirstName != right.FirstName {
		return left.FirstName < right.FirstName
	}
	return left.StudentID < right.StudentID
}

var mealPlanErrorRules = []apiCommon.ErrorRule{
	{Target: mealplanModule.ErrDisabled, Render: apiCommon.FixedRenderer(apiCommon.ErrorForbidden, errors.New("feature_disabled"))},
	{Target: mealplanModule.ErrInvalidMealDate, Render: apiCommon.FixedRenderer(apiCommon.ErrorInvalidRequest, errors.New("meal plan covers weekdays only (Monday-Friday)"))},
	{Target: mealplanModule.ErrInvalidDishes, Render: apiCommon.ErrorInvalidRequest},
	{Target: mealplanModule.ErrRegistrationDisabled, Render: apiCommon.FixedRenderer(apiCommon.ErrorForbidden, errors.New("meal_registration_disabled"))},
	{Target: mealplanModule.ErrInvalidParticipation, Render: apiCommon.ErrorInvalidRequest},
	{Target: mealplanModule.ErrParticipationCutoff, Render: apiCommon.FixedRenderer(apiCommon.ErrorConflict, errors.New("meal_participation_cutoff_passed"))},
}

func renderMealPlanFailure(w http.ResponseWriter, r *http.Request, err error, internalMessage string) {
	renderer := apiCommon.RulesRenderer(mealPlanErrorRules, apiCommon.ErrorInternalServerRenderer(internalMessage))
	apiCommon.RenderError(w, r, renderer(err))
}

func newMealPlanResource(module *mealplanModule.Module, renderer services.SimpleListRenderer) *mealplanAPI.Resource {
	return mealplanAPI.NewResource(module, mealplanAPI.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, mealplanAPI.Middleware)) {
			apiCommon.ProtectedTenantRoutes(router, register)
		},
		Permission: func(access mealplanAPI.Access) mealplanAPI.Middleware {
			if access == mealplanAPI.AccessParticipants {
				return apiCommon.RequireMealParticipantsRead()
			}
			if access == mealplanAPI.AccessRead {
				return apiCommon.RequireConfigRead()
			}
			return apiCommon.RequireConfigUpdate()
		},
		Success: apiCommon.Respond,
		InvalidRequest: func(w http.ResponseWriter, r *http.Request, err error) {
			apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
		},
		ModuleFailure: func(w http.ResponseWriter, r *http.Request, err error, internalMessage string) {
			renderMealPlanFailure(w, r, err, internalMessage)
		},
		ExportDailyList: func(list mealplanModule.DailyList, format string) (mealplanAPI.ExportFile, error) {
			return renderMealParticipationExport(renderer, list, format)
		},
	})
}

func renderMealParticipationExport(renderer services.SimpleListRenderer, list mealplanModule.DailyList, format string) (mealplanAPI.ExportFile, error) {
	rows := make([][]string, 0, len(list.Participants))
	for _, participant := range list.Participants {
		rows = append(rows, []string{participant.LastName + ", " + participant.FirstName, participant.SchoolClass})
	}
	date, err := list.Date.German()
	if err != nil {
		return mealplanAPI.ExportFile{}, err
	}
	file, err := renderer(services.SimpleListDocument{
		Title:       "Mittagessen – Tagesliste",
		Subtitle:    "Tagesliste für den " + date,
		GeneratedAt: time.Now(),
		Filters:     []string{"Änderungsfrist: " + list.CutoffTime + " Uhr", fmt.Sprintf("%d Kinder", len(rows))},
		Columns: []services.SimpleListColumn{
			{ID: "student_name", Label: "Kind"},
			{ID: "student_class", Label: "Klasse"},
		},
		Rows:   rows,
		Footer: "Vertraulich behandeln und nach dem Küchendienst sicher entsorgen.",
	}, format, "mittagessen-"+string(list.Date))
	if err != nil {
		return mealplanAPI.ExportFile{}, err
	}
	return mealplanAPI.ExportFile{Data: file.Data, ContentType: file.ContentType, Filename: file.Filename}, nil
}

func newUsersResource(module peopleModule.Capability, accountEmails func(context.Context, []int64) (map[int64]string, error), tagExists func(context.Context, string) (bool, error)) *usersAPI.Resource {
	return usersAPI.NewResource(module, usersAPI.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, usersAPI.Middleware)) {
			apiCommon.ProtectedTenantRoutes(router, register)
		},
		Permission: func(permission string) usersAPI.Middleware {
			return apiCommon.RequiresPermission(permission)
		},
		ParsePagination: apiCommon.ParsePagination,
		Success:         apiCommon.Respond,
		SuccessPaginated: func(w http.ResponseWriter, r *http.Request, status int, data any, pagination usersAPI.Pagination, message string) {
			apiCommon.RespondPaginated(w, r, status, data, apiCommon.PaginationParams{Page: pagination.Page, PageSize: pagination.PageSize, Total: pagination.Total}, message)
		},
		NoContent: apiCommon.RespondNoContent,
		Failure: func(w http.ResponseWriter, r *http.Request, kind usersAPI.FailureKind, err error) {
			switch kind {
			case usersAPI.FailureInvalidRequest:
				apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
			case usersAPI.FailureNotFound:
				apiCommon.RenderError(w, r, apiCommon.ErrorNotFound(err))
			case usersAPI.FailureConflict:
				apiCommon.RenderError(w, r, apiCommon.ErrorConflict(err))
			default:
				apiCommon.RenderError(w, r, apiCommon.ErrorInternalServerWrap("Internal server error", err))
			}
		},
		AccountEmails: accountEmails,
		TagExists:     tagExists,
		ObserveResponse: func(status int, code string) {
			observability.ObservePeopleDirectoryHTTPResponse(status, code)
		},
	})
}

func newFeedbackResource(module *feedbackModule.Module) *feedbackAPI.Resource {
	return feedbackAPI.NewResource(module, feedbackAPI.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, feedbackAPI.Middleware)) {
			apiCommon.ProtectedTenantRoutes(router, register)
		},
		Permission: func(permission string) feedbackAPI.Middleware {
			return apiCommon.RequiresPermission(permission)
		},
		Success: apiCommon.Respond,
		Failure: func(w http.ResponseWriter, r *http.Request, failure feedbackAPI.Failure) {
			apiCommon.RenderError(w, r, &apiCommon.ErrResponse{
				Err: failure.Err, HTTPStatusCode: failure.Status,
				Status: "error", ErrorText: failure.Err.Error(),
			})
		},
		ObserveResponse: func(status int, code string) {
			observability.ObserveFeedbackHTTPResponse("staff", status, code)
		},
	})
}

// serveGraph is the composed Serve root: its router, the process-scoped pool
// and the retained composition the embedded Worker still reads (#2749). Each
// route resource is mounted where it is built (#2745); only the two document
// sweeps the Worker runs outlive their mount.
type serveGraph struct {
	router             chi.Router
	db                 *bun.DB
	services           *services.Factory
	repos              *repositories.Factory
	tenantRuntime      apiCommon.TenantRuntime
	metrics            *httpMetrics
	tracer             *observability.Tracer
	metricsBearerToken string
	feedback           *feedbackModule.Module
	documents          documentSweeps
}

// documentSweeps are the Worker's file sweeps that route resources serve.
type documentSweeps struct {
	staff   scheduler.StaffDocumentFileCleaner
	student scheduler.StudentDocumentFileCleaner
}

// routeInputs is what the route composition reads. It hands back none of the
// resources it builds.
type routeInputs struct {
	modules        moduleServices
	db             *bun.DB
	logger         *slog.Logger
	sessionAuth    *projectJWT.TokenAuth
	frontendURL    string
	errorReportDSN string
	metricsToken   string
}

type apiBuildResources struct {
	pool     *bun.DB
	tracker  analytics.Tracker
	released bool
}

func (resources *apiBuildResources) close() error {
	if resources.released {
		return nil
	}
	var err error
	if resources.tracker != nil {
		err = resources.tracker.Close()
	}
	return errors.Join(err, database.ClosePool(resources.pool))
}

// newServeGraph composes the production HTTP graph of one Serve root.
func newServeGraph(config ServeConfig) (result *serveGraph, resultErr error) {
	logger := config.Logger
	metricsBearerToken, err := observability.MetricsBearerTokenFromEnv(os.Getenv)
	if err != nil {
		return nil, err
	}

	// Get database connection as phoenix_auth (least-privilege for serve)
	db, err := database.DBConnForServe()
	if err != nil {
		return nil, err
	}
	buildResources := apiBuildResources{pool: db}
	defer func() {
		resultErr = errors.Join(resultErr, buildResources.close())
	}()
	db.AddQueryHook(database.NewLockWaitQueryHook(services.ObserveUnitOfWorkLockWait)) //nolint:staticcheck // SA1019: hooks must reach the shared *bun.DB other components already hold; WithQueryHook returns a clone.
	postgresUnitOfWork, err := database.NewPostgresUnitOfWork(db, services.ObserveUnitOfWorkPoolWait)
	if err != nil {
		return nil, err
	}
	tenantRuntime, err := services.BindTenantRuntime(
		postgresUnitOfWork.WithinTenant,
		postgresUnitOfWork.WithinAdmin,
		postgresUnitOfWork,
		database.IsRetryableTransactionError,
	)
	if err != nil {
		return nil, err
	}

	// Fail fast on a partially migrated schema: the student repository selects
	// and writes its departure columns unconditionally (no per-request
	// information_schema probes, #2059), so the server must only start against
	// a fully migrated database.
	if err := usersRepo.VerifyStudentSchema(context.Background(), db); err != nil {
		return nil, err
	}

	if viper.GetBool("db_debug") {
		db.AddQueryHook(database.NewQueryHook(logger.With("component", "database"))) //nolint:staticcheck // SA1019: hooks must reach the shared *bun.DB other components already hold; WithQueryHook returns a clone.
	}

	// Compose one authoritative instance of each migrated module.
	modules, err := initializeModuleServices(db, config.PublicAPIURL, logger, tenantRuntime)
	if err != nil {
		return nil, err
	}
	serviceFactory := modules.services
	buildResources.tracker = serviceFactory.Tracker
	if err := serviceFactory.SetTenantRuntime(tenantRuntime); err != nil {
		return nil, err
	}
	serviceFactory.SetSettingsObservers(
		observability.ObserveSettingsLookup,
		observability.RecordSettingsSideEffectFailure,
	)
	registerRuntimeStatsProviders(db, modules)

	httpMetrics := newHTTPMetrics()
	tracer := newRuntimeTracer(logger)
	graph := &serveGraph{
		router:             chi.NewRouter(),
		db:                 db,
		services:           serviceFactory,
		repos:              modules.repositories,
		tenantRuntime:      tenantRuntime,
		metrics:            httpMetrics,
		tracer:             tracer,
		metricsBearerToken: metricsBearerToken,
		feedback:           modules.feedback,
	}

	// Setup router middleware
	graph.router.Use(requestIDs(tracer))
	graph.router.Use(apiCommon.ProblemResponseMiddleware)
	graph.router.Use(apiCommon.TenantRuntimeMiddleware(tenantRuntime))
	graph.router.Use(apiCommon.AuthorizationObserverMiddleware(func(event apiCommon.AuthorizationEvent) {
		observability.RecordAuthorizationEvent(event.Outcome, event.Reason, event.Elapsed)
	}))
	graph.router.Use(apiCommon.TenantRuntimeObserverMiddleware(func(observation apiCommon.TenantRuntimeObservation) {
		recordHTTPRuntimeEvent(tracer, observation)
	}))
	graph.router.Use(apiCommon.TenantRequestObserverMiddleware(func(event apiCommon.TenantRequestEvent) {
		observability.ObserveTenantRequest(
			event.TenantID,
			event.Scope,
			event.Request.Method,
			apiCommon.RoutePattern(event.Request),
			event.Status,
			event.Duration,
			event.Outcome,
		)
	}))
	setupBasicMiddleware(graph.router, logger, httpMetrics)

	// Setup CORS, security logging, and rate limiting
	setupCORSIfEnabled(graph.router, config.EnableCORS)
	securityLogger := setupSecurityLogging(graph.router)
	sessionAuth, err := newSessionTokenAuth()
	if err != nil {
		return nil, err
	}
	setupRateLimiting(graph.router, securityLogger, sessionAuth, demoLoopbackExempt())
	// One verifier serves every route: it only parses the presented token, and
	// each protected group still rejects through the Authenticator and its
	// scope gate. A group mounted without it fails closed.
	graph.router.Use(sessionAuth.Verifier())
	// Sentry events name the session they affect (#3643).
	graph.router.Use(apiCommon.SentrySessionContext)
	// Core actions of the portals reach the usage analytics once their
	// response is 2xx (#3602). After the verifier, which names the session.
	graph.router.Use(coreActionAnalytics(serviceFactory.Tracker, sessionAuth, settingsCompose.NewAnalyseFreigabe(serviceFactory.Settings, logger)))

	documents, err := mountRoutes(graph.router, routeInputs{
		modules:        modules,
		db:             db,
		logger:         logger,
		sessionAuth:    sessionAuth,
		frontendURL:    config.FrontendURL,
		errorReportDSN: config.SentryPyrePortalDSN,
		metricsToken:   metricsBearerToken,
	}, authRateLimitersFromEnv())
	if err != nil {
		return nil, err
	}
	graph.documents = documents

	buildResources.released = true
	return graph, nil
}

// registerRuntimeStatsProviders hands the pool, the live streams and the PWA
// usage to the metrics endpoint.
func registerRuntimeStatsProviders(db *bun.DB, modules moduleServices) {
	serviceFactory := modules.services
	observability.RegisterDBStatsProvider(func() observability.DBStats {
		stats := database.SnapshotCapacity(db)
		return observability.DBStats{
			OpenConnections:   stats.OpenConnections,
			InUse:             stats.InUse,
			Idle:              stats.Idle,
			WaitCount:         stats.WaitCount,
			WaitDuration:      stats.WaitDuration,
			MaxIdleClosed:     stats.MaxIdleClosed,
			MaxLifetimeClosed: stats.MaxLifetimeClosed,
		}
	})
	observability.RegisterSSEStatsProvider(serviceFactory.RealtimeHub)
	observability.RegisterPWAUsageStatsProvider(observability.PWAUsageStatsProviderFunc(func() ([]observability.PWAUsageStat, error) {
		rows, err := serviceFactory.PWAUsage.SnapshotUsage()
		if err != nil {
			return nil, err
		}
		stats := make([]observability.PWAUsageStat, 0, len(rows))
		for _, row := range rows {
			stats = append(stats, observability.PWAUsageStat{
				TenantID:        row.TenantID,
				Portal:          row.Portal,
				StandaloneUsers: row.StandaloneUsers,
				EligibleUsers:   row.EligibleUsers,
			})
		}
		return stats, nil
	}))
}

// ServeHTTP routes a request through the composed graph. CalDAV extension
// methods reach their handler under a routable stand-in method.
func (graph *serveGraph) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	graph.router.ServeHTTP(w, normalizeCalDAVMethodForRouting(r))
}

// mountRoutes builds every route resource and mounts it where it is built:
// the public and portal routes at the root, the tenant routes under /api. No
// resource outlives its mount; the Serve graph keeps only the Worker's
// document sweeps. It refuses to start while a writing route has no
// core-action classification.
func mountRoutes(root chi.Router, in routeInputs, limiters authRateLimiters) (documentSweeps, error) {
	svc := in.modules.services
	svc.EnableStudentPhotos(services.StudentPhotoBootstrap{
		Unlinker:     studentsAPI.NewPhotoUnlinker(in.logger.With("component", "student-photo-unlinker"), "public"),
		PhotoRuntime: in.modules.studentPhotoRuntime,
		Logger:       in.logger.With("service", "student-photo"),
	})
	if err := mountDemoAccess(root, in.modules.demoAccess, viper.GetString("app_env"), in.frontendURL, viper.GetString("tenant_domain")); err != nil {
		return documentSweeps{}, err
	}
	mountPublicRoutes(root, in.metricsToken)
	tenant := chi.NewRouter()
	if err := mountRequestFeed(root, tenant, in); err != nil {
		return documentSweeps{}, err
	}
	if err := mountModuleRoutes(tenant, in); err != nil {
		return documentSweeps{}, err
	}
	documents, err := mountRouteGroups(root, tenant, in, limiters)
	if err != nil {
		return documentSweeps{}, err
	}
	root.Mount("/api", tenant)
	return documents, requireCoreActionClassification(root)
}

// mountRouteGroups mounts the resources of each route group. One device
// authentication composition serves every kiosk route group, so the IoT and
// students resources share its last-seen debouncer; one Student Presence owner
// serves the students, attendance and kiosk routes (#3349).
func mountRouteGroups(root, tenant chi.Router, in routeInputs, limiters authRateLimiters) (documentSweeps, error) {
	deviceAuth := newDeviceAuthentication(in.modules)
	presence := newStudentPresence(in.db, in.logger)
	student, err := mountStudentRoutes(tenant, in, deviceAuth, presence)
	if err != nil {
		return documentSweeps{}, err
	}
	if err := mountPresenceRoutes(tenant, in, deviceAuth, presence); err != nil {
		return documentSweeps{}, err
	}
	staff, err := mountStaffRoutes(tenant, in)
	if err != nil {
		return documentSweeps{}, err
	}
	if err := mountSchoolPortalRoutes(root, tenant, in, presence, limiters); err != nil {
		return documentSweeps{}, err
	}
	mountSchoolRoutes(tenant, in)
	mountPeopleRoutes(tenant, in)
	mountCommunicationRoutes(tenant, in)
	mountCalendarRoutes(root, tenant, in)
	mountDeliveryRoutes(root, tenant, in)
	if err := mountPortalRoutes(root, in, limiters); err != nil {
		return documentSweeps{}, err
	}
	return documentSweeps{staff: staff, student: student}, nil
}

// mountRequestFeed serves the change-request feed: the public feed behind its
// token and the staff's feed administration under /api.
func mountRequestFeed(root, tenant chi.Router, in routeInputs) error {
	requestFeed, err := requestFeedCompose.New(requestFeedCompose.Dependencies{
		DB: in.db, FrontendURL: in.frontendURL, Now: time.Now,
		NewToken: projectJWT.NewOpaqueCapabilityToken, HashToken: projectJWT.OpaqueCapabilityFingerprint,
	})
	if err != nil {
		return err
	}
	resource := requestFeedHTTP.NewResource(requestFeed, requestFeedHTTP.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, requestFeedHTTP.Middleware)) {
			apiCommon.ProtectedTenantRoutes(router, register)
		},
		CurrentTenantID:  func(r *http.Request) int64 { return projectJWT.ClaimsFromCtx(r.Context()).TenantID },
		CurrentAccountID: func(r *http.Request) int64 { return int64(projectJWT.ClaimsFromCtx(r.Context()).ID) },
		Logger:           in.logger.With("handler", "request-feed"),
	})
	root.Mount("/public/request-feed", resource.PublicRouter())
	tenant.Mount("/students/change-requests/rss-feed", resource.TenantRouter())
	return nil
}

// mountModuleRoutes mounts the route groups modules serve themselves.
func mountModuleRoutes(tenant chi.Router, in routeInputs) error {
	schoolSetup, err := newSchoolSetupRoute(schoolSetupCompose.Dependencies{
		Settings: in.modules.services.Settings,
	})
	if err != nil {
		return err
	}
	staffOnboarding, err := newStaffOnboardingRoute()
	if err != nil {
		return err
	}
	for _, module := range []moduleRoute{schoolSetup, staffOnboarding} {
		tenant.Mount(module.pattern, module.router)
	}
	return nil
}

func newRuntimeTracer(logger *slog.Logger) *observability.Tracer {
	return observability.NewTracer(logger, func(entryPoint, _ string, outcome string) {
		observability.RecordTenantRuntimeEvent(entryPoint, outcome)
	})
}

// setupBasicMiddleware configures basic router middleware
func setupBasicMiddleware(router chi.Router, logger *slog.Logger, httpMetrics *httpMetrics) {
	router.Use(middleware.ClientIPFromXFF())
	router.Use(syncClientIPToRemoteAddr)
	if httpMetrics != nil {
		router.Use(httpMetrics.middleware)
	}
	// Redact public calendar- and request-feed tokens (the sole credential for
	// those feeds) from the per-request "path" attribute, and
	// strip query-string values (staff-UI searches carry student names and
	// e-mail addresses as query parameters, issue #2105) so neither lands in
	// access logs.
	requestLogger := slog.New(customMiddleware.NewQueryValueRedactor(
		customMiddleware.NewFeedTokenRedactor(logger.Handler())))
	router.Use(slogchi.NewWithConfig(requestLogger, slogchi.Config{
		DefaultLevel:     slog.LevelInfo,
		ClientErrorLevel: slog.LevelWarn,
		ServerErrorLevel: slog.LevelError,
		WithRequestID:    true,
		WithRequestBody:  false,
		WithResponseBody: false,
		WithSpanID:       false,
		WithTraceID:      false,
		Filters: []slogchi.Filter{
			slogchi.IgnorePath("/health"),
			// Bot/scanner probes use HTTP methods our API never serves
			// (CONNECT tunneling, TRACE/XST, PRI HTTP/2 preface). They get a
			// 404/405 but add only WARN-level log noise (issue #850). Drop the
			// log lines; the Prometheus HTTP middleware runs earlier in the
			// chain and still counts them, so volume-based scan alerting is
			// unaffected. Legitimate 4xx on GET/POST/... routes keep logging.
			slogchi.IgnoreMethod(http.MethodConnect, http.MethodTrace, "PRI"),
		},
	}))
	router.Use(middleware.Recoverer)
	// Inside the Recoverer: sentryhttp reports a panic and repanics to it, and
	// every 5xx answer becomes one Sentry event (#3639).
	router.Use(apiCommon.ServerErrorReporting)
	router.Use(customMiddleware.SecurityHeaders)
	// Request-scoped settings memo cache (issue #2065). Router-wide so routes
	// outside ProtectedTenantGroup (/auth incl. /auth/tenant/resolve,
	// /operator, /parent, public enrollment) dedupe their settings lookups
	// too. Idempotent with the group-wide attachment in api/common.
	router.Use(apiCommon.RequestSettingsCacheMiddleware)
	// Request-scoped identity memo cache (issue #2099). Router-wide so routes
	// outside ProtectedTenantGroup (notably /api/sse, which builds its own JWT
	// chain) dedupe their identity-chain lookups too.
	router.Use(apiCommon.RequestIdentityCacheMiddleware)
}

func syncClientIPToRemoteAddr(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := middleware.GetClientIP(r.Context()); ip != "" {
			r.RemoteAddr = ip
		}
		next.ServeHTTP(w, r)
	})
}

func setupCORSIfEnabled(router chi.Router, enabled bool) {
	if enabled {
		setupCORS(router)
	}
}

// setupCORS configures CORS middleware with allowed origins from environment.
// Supports wildcard subdomain patterns like "*.example.com" via AllowOriginFunc.
func setupCORS(router chi.Router) {
	router.Use(corsHandler(os.Getenv("CORS_ALLOWED_ORIGINS")))
}

// corsHandler builds the CORS middleware for a CORS_ALLOWED_ORIGINS value.
func corsHandler(allowedOrigins string) func(http.Handler) http.Handler {
	exactOrigins, wildcardSuffixes := parseAllowedOrigins(allowedOrigins)

	opts := cors.Options{
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Staff-PIN", "X-Staff-ID", "X-Device-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}

	if len(wildcardSuffixes) > 0 {
		opts.AllowOriginFunc = buildCORSOriginFunc(exactOrigins, wildcardSuffixes)
	} else {
		opts.AllowedOrigins = exactOrigins
	}

	return cors.Handler(opts)
}

// buildCORSOriginFunc returns a CORS origin matcher that accepts any exact
// origin or any origin whose host ends in one of the wildcard suffixes
// (e.g. ".example.com" matches "https://school-a.example.com").
func buildCORSOriginFunc(exactOrigins, wildcardSuffixes []string) func(*http.Request, string) bool {
	// Build a set for O(1) exact-match lookups
	exactSet := make(map[string]bool, len(exactOrigins))
	for _, o := range exactOrigins {
		exactSet[o] = true
	}
	return func(_ *http.Request, origin string) bool {
		if exactSet[origin] {
			return true
		}
		return matchesWildcardSuffix(origin, wildcardSuffixes)
	}
}

// matchesWildcardSuffix reports whether the host portion of origin ends in one
// of the given suffixes (with at least one leading subdomain label).
func matchesWildcardSuffix(origin string, wildcardSuffixes []string) bool {
	// Extract the host portion of the origin (e.g. "https://school-a.example.com" → "school-a.example.com")
	host := origin
	if idx := strings.Index(origin, "://"); idx >= 0 {
		host = origin[idx+3:]
	}
	for _, suffix := range wildcardSuffixes {
		if strings.HasSuffix(host, suffix) && len(host) > len(suffix) {
			return true
		}
	}
	return false
}

// parseAllowedOrigins parses CORS_ALLOWED_ORIGINS and splits entries into
// exact-match origins and wildcard subdomain suffixes (e.g. "*.example.com"
// becomes suffix ".example.com").
func parseAllowedOrigins(originsEnv string) (exact []string, wildcardSuffixes []string) {
	if originsEnv == "" {
		return []string{"*"}, nil
	}

	for _, raw := range strings.Split(originsEnv, ",") {
		origin := strings.TrimSpace(raw)
		if origin == "" {
			continue
		}
		if strings.HasPrefix(origin, "*.") {
			// "*.example.com" → match any subdomain of ".example.com"
			wildcardSuffixes = append(wildcardSuffixes, origin[1:]) // ".example.com"
		} else {
			exact = append(exact, origin)
		}
	}

	if len(exact) == 0 && len(wildcardSuffixes) == 0 {
		return []string{"*"}, nil
	}
	return exact, wildcardSuffixes
}

// setupSecurityLogging configures security logging middleware if enabled
func setupSecurityLogging(router chi.Router) *customMiddleware.SecurityLogger {
	if os.Getenv("SECURITY_LOGGING_ENABLED") != "true" {
		return nil
	}

	securityLogger := customMiddleware.NewSecurityLogger()
	router.Use(customMiddleware.SecurityLoggingMiddleware(securityLogger))
	return securityLogger
}

// demoLoopbackExempt reports whether the rate limiters let loopback peers
// through. Only the public demo does: its demo process shares the server
// container's network namespace and drives more than a thousand API calls per
// seeded school under one account. Public requests never arrive from
// loopback (see customMiddleware.RateLimiter.ExemptLoopback).
func demoLoopbackExempt() bool {
	return services.IsDemoEnvironment(viper.GetString("app_env"))
}

// setupRateLimiting configures rate limiting middleware if enabled
func setupRateLimiting(router chi.Router, securityLogger *customMiddleware.SecurityLogger, tokenAuth *projectJWT.TokenAuth, exemptLoopback bool) {
	if os.Getenv("RATE_LIMIT_ENABLED") != "true" {
		return
	}

	generalLimit := parsePositiveInt(os.Getenv("RATE_LIMIT_REQUESTS_PER_MINUTE"), 60)
	generalBurst := parsePositiveInt(os.Getenv("RATE_LIMIT_BURST"), 10)

	generalRateLimiter := customMiddleware.NewRateLimiter(generalLimit, generalBurst)
	generalRateLimiter.SetBucketFunc(func(r *http.Request) string {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			return "read"
		default:
			return "write"
		}
	})
	generalRateLimiter.SetRejectObserver(observability.RecordRateLimitRejection)
	generalRateLimiter.SetKeyFunc(identityRateLimitKey(tokenAuth))
	if exemptLoopback {
		generalRateLimiter.ExemptLoopback()
	}
	if securityLogger != nil {
		generalRateLimiter.SetLogger(securityLogger)
	}
	router.Use(generalRateLimiter.Middleware())
}

// identityRateLimitKey buckets authenticated requests by their stable quota
// identity — account ID, scope, and tenant ID from the verified JWT claims —
// instead of a per-token hash (#2064). Every valid session of the same
// identity shares one budget, so a re-login or token refresh no longer
// resets the quota. The key carries only numeric IDs and the scope label,
// never the raw token, email, or name.
//
// Quota-identity rules:
//   - Same account, same scope, same tenant → one shared budget across all
//     sessions (browser tabs, refreshed tokens, re-logins).
//   - A tenant switch mints a JWT with a different tenant_id and gets its
//     own budget. Different portal scopes ("", "org", "platform", "parent")
//     are separate budgets too — scope must be in the key because operator
//     IDs (platform.operators) and account IDs (auth.accounts) are
//     different ID spaces.
//   - Requests without a verified identity — missing, expired, or
//     manipulated JWTs, non-JWT bearer values such as IoT device API keys,
//     and MFA challenge/enrollment tokens (no "id" claim) — return "" and
//     the limiter falls back to the trusted client IP. Unverified bearer
//     values must never produce token-derived buckets: an attacker could
//     mint arbitrary values and sidestep IP limiting entirely.
func identityRateLimitKey(tokenAuth *projectJWT.TokenAuth) func(*http.Request) string {
	return func(r *http.Request) string {
		tokenString := extractBearerToken(r.Header.Get("Authorization"))
		if tokenString == "" || tokenAuth == nil || tokenAuth.JwtAuth == nil {
			return ""
		}

		token, err := tokenAuth.JwtAuth.Decode(tokenString)
		if err != nil {
			return ""
		}
		if err := jwxjwt.Validate(token); err != nil {
			return ""
		}

		claims := map[string]any{}
		if err := transform.AsMap(token, claims); err != nil {
			return ""
		}
		// JSON numbers decode as float64 (same contract as AppClaims.ParseClaims).
		accountID, ok := claims["id"].(float64)
		if !ok || accountID <= 0 {
			return ""
		}
		scope, _ := claims["scope"].(string)
		tenantID, _ := claims["tenant_id"].(float64)

		return fmt.Sprintf("acct:%d:%s:%d", int64(accountID), scope, int64(tenantID))
	}
}

func extractBearerToken(authHeader string) string {
	const bearerPrefix = "Bearer "
	if len(authHeader) <= len(bearerPrefix) || !strings.HasPrefix(authHeader, bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(authHeader[len(bearerPrefix):])
}

// parsePositiveInt parses a positive integer from environment variable with a default value
func parsePositiveInt(valueStr string, defaultValue int) int {
	if valueStr == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(valueStr)
	if err != nil || parsed <= 0 {
		return defaultValue
	}
	return parsed
}

// requestReviewDependencies binds native owner capabilities for the staff projection.
func requestReviewDependencies(modules moduleServices, db *bun.DB) (requestreviewcompose.ProjectionDependencies, carePlanModule.CareScheduleReviewQuery, error) {
	svc := modules.services
	reviewStudents, err := requestreviewcompose.NewStudentDirectory(db, modules.persons, func(observation requestreviewcompose.DirectoryObservation) {
		observability.ObserveSchoolStructureOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, schoolStructureModule.ErrorCode(observation.Err), observation.Err)
	})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("request review student directory: %w", err)
	}
	// The staff projection lists exactly what the decisions accept: one
	// Identity & Access review policy serves both (#3804).
	reviewScope := func(ctx context.Context) (carePlanCompose.ReviewScope, error) {
		schoolWide, groupIDs, err := svc.RequestReviewPolicy.Scope(ctx, projectJWT.PermissionsFromCtx(ctx))
		return carePlanCompose.ReviewScope{SchoolWide: schoolWide, GroupIDs: groupIDs}, err
	}
	// Report the union of ordinary and absence-review rights, as the
	// navigation capability does. Each native queue keeps its own scope.
	reviewAccess, err := requestreviewcompose.NewAccess(studentsAPI.RequestReviewPrincipal, func(ctx context.Context) (string, error) {
		return svc.RequestReviewPolicy.AccessLevel(ctx, projectJWT.PermissionsFromCtx(ctx))
	})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("request review access: %w", err)
	}
	masterDataReviews, err := requestreviewcompose.NewMasterDataReviews(db, modules.persons,
		reviewScope, carePlanCompose.Today, func(observation requestreviewcompose.CareObservation) {
			observability.ObserveCarePlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.Conflicts, observation.Stats.StatementDuration, carePlanModule.ErrorCode(observation.Err), observation.Err)
		})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("master data review queue: %w", err)
	}
	careReviews, err := requestreviewcompose.NewScheduleReviews(db, requestreviewcompose.ScheduleReviewDependencies{
		People: modules.persons,
		Scope:  reviewScope,
		BookingsAuthoritative: func(ctx context.Context) (bool, error) {
			return svc.Settings.ResolveBool(ctx, reviewsettings.BookingsAuthoritative)
		},
		Today:  carePlanCompose.Today,
		Blocks: repositories.NewPickupReviewBlocks(db),
		ObserveCare: func(observation requestreviewcompose.CareObservation) {
			observability.ObserveCarePlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.Conflicts, observation.Stats.StatementDuration, carePlanModule.ErrorCode(observation.Err), observation.Err)
		},
		ObserveTimetable: func(observation requestreviewcompose.TimetableObservation) {
			observability.ObserveTimetableActivitiesOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.DuplicatePreventionConflicts, observation.Stats.StatementDuration, timetableModule.ErrorCode(observation.Err), observation.Err)
		},
	})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("care schedule reviews: %w", err)
	}
	careQueue, err := requestreviewcompose.NewCareScheduleQueue(careReviews, carePlanCompose.Today)
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("care schedule review queue: %w", err)
	}
	offeringReviews, err := requestreviewcompose.NewOfferingReviews(db, requestreviewcompose.OfferingReviewDependencies{
		People: modules.persons,
		Scope:  reviewScope,
		Today:  carePlanCompose.Today,
		ObserveCare: func(observation requestreviewcompose.CareObservation) {
			observability.ObserveCarePlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.Conflicts, observation.Stats.StatementDuration, carePlanModule.ErrorCode(observation.Err), observation.Err)
		},
		ObserveTimetable: func(observation requestreviewcompose.TimetableObservation) {
			observability.ObserveTimetableActivitiesOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.DuplicatePreventionConflicts, observation.Stats.StatementDuration, timetableModule.ErrorCode(observation.Err), observation.Err)
		},
	})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("offering reviews: %w", err)
	}
	offeringQueue, err := requestreviewcompose.NewOfferingQueue(offeringReviews, carePlanCompose.Today)
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("offering review queue: %w", err)
	}
	corrections, err := requestreviewcompose.NewCorrectionLog(db, modules.persons, func(ctx context.Context) bool {
		return studentsAPI.RequestReviewCorrectionAccess(ctx, svc.UserContext.HasCurrentStaff)
	}, func(requestreviewcompose.AuditObservation) {})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("direct correction history: %w", err)
	}
	masterQueue, err := requestreviewcompose.NewMasterDataQueue(masterDataReviews)
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("master data review queue: %w", err)
	}
	excusedQueue, err := requestreviewcompose.NewExcusedQueue(svc.ExcusedRequests, carePlanCompose.Today)
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("excused review queue: %w", err)
	}
	return requestreviewcompose.ProjectionDependencies{
		Queues:   requestreviewcompose.Queues{DirectCorrections: corrections, MasterData: masterQueue, CareSchedule: careQueue, Offering: offeringQueue, Excused: excusedQueue},
		Students: reviewStudents, FamilyProtection: requestreviewcompose.NewFamilyProtection(svc.PeopleDirectory), Access: reviewAccess,
	}, careReviews, nil
}

func newDeviceAuthentication(modules moduleServices) *deviceauth.Authenticators {
	svc := modules.services
	return deviceauth.New(deviceauth.Dependencies{
		Devices:     svc.IoT.Fleet(),
		Schools:     deviceSchoolDirectory{schools: svc.Schools},
		Settings:    svc.Settings,
		FallbackPIN: os.Getenv("OGS_DEVICE_PIN"),
	})
}

// mountStudentRoutes serves /api/students and hands its resource back as the
// Worker's student document sweep.
func mountStudentRoutes(tenant chi.Router, in routeInputs, deviceAuth *deviceauth.Authenticators, presence *studentpresence.Module) (scheduler.StudentDocumentFileCleaner, error) {
	svc, modules := in.modules.services, in.modules
	// A direct school_class edit must resync Jahrgang-filtered offering-sourced
	// Regeltermine like a grade transition does (#2147 review round 10); Care
	// Plan's booking materialization provides the resync (#3560).
	var studentClassResyncer schoolStructureCompose.OfferingSourceResyncer = svc.EnrollmentCareOffering
	reviewDependencies, careReviews, err := requestReviewDependencies(modules, in.db)
	if err != nil {
		return nil, err
	}
	requestReview, err := requestreviewcompose.NewProjection(reviewDependencies)
	if err != nil {
		return nil, fmt.Errorf("request review projection: %w", err)
	}
	students := studentsAPI.NewResource(studentsAPI.ResourceConfig{
		PeopleDirectory:              svc.PeopleDirectory,
		Persons:                      services.NewStudentRoutePersons(svc.Users),
		CompanionService:             svc.Students.Companions,
		ClassListEntries:             classListEntryStudentsReader{entries: modules.membership},
		ChildQuota:                   childQuotaStudentsReader{usages: modules.membership},
		StudentDeletion:              svc.StudentDeletion,
		CareLifecycleService:         svc.CareLifecycle,
		StudentAuditService:          svc.PeopleDirectory,
		SchoolGroups:                 studentSchoolGroups{GroupManagement: svc.Education},
		UserContextService:           svc.UserContext,
		ActiveService:                svc.Active,
		DeviceAuthenticator:          deviceAuth.Device(),
		AuthenticatedDevice:          deviceauth.DeviceID,
		PickupScheduleService:        svc.PickupSchedule,
		WeekdayPickupNotes:           modules.repositories.CarePlan(),
		PartialAbsenceService:        svc.PartialAbsence,
		ArrivalScheduleService:       svc.ArrivalSchedule,
		InstanceService:              svc.Instance,
		CareDayService:               svc.CareDay,
		SchoolService:                studentSchoolDirectory{schools: svc.Schools},
		SettingsService:              svc.Settings,
		MasterDataReviewService:      svc.MasterDataReview,
		CareRequestService:           svc.CareRequests,
		CareRequestReviews:           careReviews,
		OfferingChangeService:        svc.EnrollmentCareOffering,
		PickupAdjustmentService:      svc.EnrollmentCareOffering,
		ExcusedRequestService:        svc.ExcusedRequests,
		ParentRequestBulkService:     svc.ParentRequests,
		ParentRequestConflictService: svc.ParentRequests,
		FamilyProtection:             svc.PeopleDirectory,
		StudentNotes:                 svc.PeopleDirectory,
		RequestReviewAccess:          svc.RequestReviewPolicy,
		RequestReview:                requestReview,
		StudentStatusDayService:      svc.StudentStatusDays,
		AbsenceOverview:              svc.AbsenceOverview,
		StudentHistoryService:        svc.StudentHistory,
		OGSGroupLiveService:          svc.OGSGroupLive,
		ActiveEnrollments:            studentActiveEnrollments{enrollments: svc.Activities},
		EnrollmentDecision:           svc.EnrollmentDecision,
		OfferingPickupTimes:          svc.EnrollmentCareOffering,
		EnrollmentFormSchema:         svc.EnrollmentFormSchema,
		OfferingSourceResyncer:       studentClassResyncer,
		LockTemplateRecurrence:       svc.TimetableData.RecurrenceLock.LockRecurrenceWrites,
		Broadcaster:                  svc.RealtimeHub,
		ParentEventEmitter:           svc.ParentEventEmitter,
		AbsenceNotifier:              svc.AbsenceNotifier,
		StudentPhotos:                svc.PeopleDirectory,
		StudentConsents:              svc.StudentConsents,
		PrivacyConsents:              presence,
		StudentDocumentService:       svc.StudentDocuments,
		ListExportService:            lists.NewRenderer(),
		Logger:                       in.logger.With("handler", "students"),
	})
	tenant.Mount("/students", students.Router())
	return students, nil
}

// mountPresenceRoutes serves the attendance routes and the kiosk API. Both
// move children through one open-room move over the Student Presence owner.
func mountPresenceRoutes(tenant chi.Router, in routeInputs, deviceAuth *deviceauth.Authenticators, presence *studentpresence.Module) error {
	svc, logger := in.modules.services, in.logger
	openRoomMove, err := newOpenRoomMove(in.modules, svc.Active, logger)
	if err != nil {
		return err
	}
	active := presenceAPI.NewResource(services.NewPresenceOperations(svc.Active, svc.OGSGroupLive, logger.With("service", "presence-operations")), activePeople{source: services.NewAttendanceRoutePeople(svc.Users)}, teacherGroupIDs(in.modules), services.NewSchulhofProjection(svc.Schulhof), activeStaffAccess{source: services.NewAttendanceRouteStaff(svc.UserContext)}, svc.Settings, apiCommon.ProtectedTenantRoutes, logger.With("handler", "active"), presence, activeRequestRuntime(), activeAuthorization(), openRoomMove)
	active.SupervisionDashboardService = svc.SupervisionDashboard
	iot, err := newIoTResource(in, deviceAuth, presence, openRoomMove)
	if err != nil {
		return err
	}
	tenant.Mount("/active", active.Router())
	tenant.Mount("/iot", iot.Router())
	return nil
}

func teacherGroupIDs(modules moduleServices) func(context.Context, int64) ([]int64, error) {
	svc := modules.services
	return func(ctx context.Context, teacherID int64) ([]int64, error) {
		groups, err := svc.Education.GetTeacherGroups(ctx, teacherID)
		if err != nil {
			return nil, err
		}
		ids := make([]int64, 0, len(groups))
		for _, group := range groups {
			if group != nil {
				ids = append(ids, group.ID)
			}
		}
		return ids, nil
	}
}

// newIoTResource composes the kiosk API. The device-scan workflow runs every
// kiosk scan through one orchestrator over the public Device Fleet, Student
// Presence, Facilities and Timetable & Activities capabilities; the retained
// services behind its ports are compatibility bindings (#2698).
func newIoTResource(in routeInputs, deviceAuth *deviceauth.Authenticators, presence *studentpresence.Module, openRoomMove openroommove.Command) (*iotAPI.Resource, error) {
	svc, modules, logger := in.modules.services, in.modules, in.logger
	sessionEnd, err := newSessionEnd(presence, modules, svc, logger)
	if err != nil {
		return nil, err
	}
	errorReports, err := iotAPI.NewErrorReportRelay(in.errorReportDSN)
	if err != nil {
		return nil, err
	}
	deviceScan := devicescanCompose.New(devicescanCompose.Dependencies{
		Fleet:      svc.IoT.Fleet(),
		Presence:   presence,
		Rooms:      modules.rooms,
		Active:     svc.Active,
		Users:      svc.Users,
		Activities: svc.Activities,
		Rosters:    repositories.SessionRosters{Sessions: presence, Roster: modules.timetable},
		Education:  svc.Education,
		Pickups:    svc.PickupSchedule,
		// A destination chosen at the kiosk runs the phone's open-room
		// move (#3067).
		OpenRooms: newDeviceOpenRoomMover(openRoomMove),
		Settings:  svc.Settings,
		Logger:    logger.With("service", "device-scan"),
	})
	repoFactory := modules.repositories
	return iotAPI.NewResource(iotAPI.ServiceDependencies{
		Administration:   devicefleetCompose.NewAdministration(svc.IoT.Fleet()),
		DeviceScan:       deviceScan,
		OpenRooms:        deviceScan,
		StaffClock:       svc.StaffClock,
		Configuration:    devicescanCompose.NewConfiguration(svc.Settings),
		Rooms:            devicescanCompose.NewRoomAvailability(svc.Facilities),
		Directory:        devicescanCompose.NewDirectory(svc.Users, svc.Activities, logger),
		TagAssignments:   devicescanCompose.NewTagAssignments(svc.Users, logger),
		FeedbackStudents: devicescanCompose.NewFeedbackStudents(svc.Users),
		FeedbackService:  modules.feedback,
		FeedbackResponseObserver: func(status int, code string) {
			observability.ObserveFeedbackHTTPResponse("iot", status, code)
		},
		SchoolName:              devicescanCompose.NewSchoolName(schoolName(svc.Schools)),
		ErrorReports:            errorReports,
		SessionEnd:              sessionEnd,
		SessionLifecycle:        devicescanCompose.NewSessionLifecycle(svc.Active, devicescanCompose.NewSupervisionQuery(presence), svc.Users, svc.IoT, devicescanCompose.NewSessionMirror(repoFactory.ActivityInstance, repoFactory.InstanceStaff, svc.Activities, services.KioskMirrorPublisher(svc.RealtimeHub, logger), logger), logger),
		Logger:                  logger.With("handler", "iot"),
		DeviceAuthenticator:     deviceAuth.Device(),
		DeviceOnlyAuthenticator: deviceAuth.DeviceOnly(),
	}), nil
}

// mountStaffRoutes serves the staff administration, shift planning,
// substitutions and time tracking, and hands the staff administration back
// as the Worker's staff document sweep.
func mountStaffRoutes(tenant chi.Router, in routeInputs) (scheduler.StaffDocumentFileCleaner, error) {
	svc, workforce := in.modules.services, in.modules.workforce
	staff, staffAdmin, err := newStaffComposition(in.modules.membership, workforce, svc, in.db, in.logger.With("handler", "staff"))
	if err != nil {
		return nil, err
	}
	staffShifts := workforceShiftPlanning.NewStaffShiftsResource(workforceShiftPlanning.StaffShiftsDependencies{
		Planning:       svc.StaffShifts,
		ResolveStaffID: currentStaffID(in.modules),
		ActorAccountID: projectJWT.ActorAccountIDFromCtx,
	})
	absenceTypes := workforceInbound.NewAbsenceTypesResource(services.AbsenceTypeAdministration(workforce, in.logger.With("service", "active")), currentStaffID(in.modules))
	tenant.Mount("/staff", staff.Router())
	tenant.Mount("/work-time-models", worktimemodelsHTTPAdapter.NewResource(workforce, services.StaffTimeTrackingNotifier(svc.RealtimeHub)).Router())
	tenant.Mount("/staff-shifts", staffShifts.Router())
	tenant.Mount("/shift-types", workforceShiftPlanning.NewShiftTypesResource(svc.ShiftTypes).Router())
	tenant.Mount("/absence-types", absenceTypes.Router())
	tenant.Mount("/substitutions", workforceInbound.NewSubstitutionsResource(services.SubstitutionCapability(svc.Substitution)).Router())
	tenant.Mount("/time-tracking", newTimeTrackingResource(svc, in.modules.calendar, in.db).Router())
	return staffAdmin, nil
}

// mountSchoolPortalRoutes serves the school portal ("moto schule", #2207) at
// the root: public /school/auth/* (login and the school-scope MFA exchange)
// plus the class-day surface. Token refresh and logout go through the shared
// scope-preserving /auth/refresh and /auth/logout. The portal reuses the
// timetable, colleague chat, staff notice and notification resources the
// staff reach under /api (#2207, #2527).
func mountSchoolPortalRoutes(root, tenant chi.Router, in routeInputs, presence *studentpresence.Module, limiters authRateLimiters) error {
	svc := in.modules.services
	pickupExtensions, err := newPickupExtensions(in.modules.timetable, presence)
	if err != nil {
		return err
	}
	timetable := newTimetableResource(in.modules, pickupExtensions, in.logger)
	// OGS-internal colleague chat (#2598) — staff-to-staff, deliberately a
	// separate surface from /messages (which is parent-facing).
	staffMessaging := staffMessagingAPI.NewResource(svc.StaffMessaging)
	// Tagesinformationen (#2180) are Timetable's: the owner composes the
	// service over its own repository, the calendar periods for the week
	// pattern and the People Directory names of the acknowledgement list
	// (#3418). All staff read them, admins write them.
	staffNotices := timetableHTTPAdapter.NewStaffNoticeResource(timetableCompose.NewStaffNotices(timetableCompose.StaffNoticeDependencies{
		DB: in.db, Periods: in.modules.repositories.CalendarPeriod, Names: services.StaffNoticeNames(svc.PeopleDirectory),
		Logger: in.logger.With("service", "staffnotice"),
	}), func(ctx context.Context) int64 { return timeTrackingIdentity(ctx).AccountID })
	// Notification abstraction (#1624).
	notifications := notificationsAPI.NewResource(svc.Notifications, svc.PushSubscriptions, svc.NotificationPreferences)
	// The class-day surface reads the class-day projection (#2701): the day
	// report over Enrollment's day roster and the arrival-exception write
	// seam (#2970) behind its one public capability.
	classDay := classdayHTTP.NewResource(svc.ClassDayArrivalExceptions, in.logger.With("handler", "class-day"))
	school := schoolPortal.NewResource(schoolPortalAuth(svc.SchoolPortalAuthentication()), schoolPortalMFA(svc.SchoolPortalMFA()), schoolPasswordResets(svc.SchoolPasswordResetRuntime()), classDay, timetable, staffMessaging, staffNotices, notifications)
	tenant.Mount("/staff-messages", staffMessaging.Router())
	tenant.Mount("/staff-notices", staffNotices.Router())
	tenant.Mount("/timetable", timetable.Router())
	tenant.Mount("/notifications", notifications.Router())
	root.Mount("/school", school.RouterWithAuthRateLimiter(limiters.authMiddleware()))
	return nil
}

// newTimetableResource composes the timetable routes. Dateframes belong to
// the School Calendar; the conflict detection is the one the instance
// lifecycle uses.
func newTimetableResource(modules moduleServices, pickupExtensions timetableModule.PickupExtensionCapability, logger *slog.Logger) *timetableAPI.Resource {
	svc := modules.services
	return timetableAPI.NewResource(timetableAPI.Dependencies{
		CalendarPeriods:         modules.calendar,
		ClosingDays:             modules.calendar,
		CalendarPeriodUsage:     calendarPeriodUsage{usage: modules.repositories.CalendarPeriodUsage()},
		MaterializationService:  svc.Materialization,
		InstanceService:         svc.Instance,
		InstanceSeriesConverter: svc.InstanceSeriesConverter,
		OperationsService:       svc.TimetableOperations,
		People:                  services.NewTimetablePeople(svc.Users),
		Templates:               svc.TimetableData.Templates,
		RecurrenceLock:          svc.TimetableData.RecurrenceLock,
		AttendanceCorrections:   svc.TimetableData.AttendanceCorrections,
		Deviations:              svc.TimetableData.Deviations,
		TimetableData:           svc.TimetableData.Data,
		ConflictDetection:       svc.TimetableData.ConflictDetection,
		PlanningTracks:          svc.PlanningTracks,
		CareDayService:          svc.CareDay,
		UserContextService:      svc.UserContext,
		SettingsService:         svc.Settings,
		SlotListsService:        svc.SlotLists,
		OfferingSourceOptions:   services.NewTimetableOfferingSources(svc.EnrollmentCareOffering),
		SupervisionSheets:       services.NewTimetableSupervisionSheets(svc.ClassDayArrivalExceptions),
		PlanExportService:       svc.PlanExport,
		PickupExtensions:        pickupExtensions,
		Staffing:                timetableStaffingAnnouncer(svc.Instance),
		Logger:                  logger.With("handler", "timetable"),
	})
}

// mountSchoolRoutes serves the school's rooms, groups, activities, schedules,
// enrollment, settings, info-point displays and data administration under
// /api.
func mountSchoolRoutes(tenant chi.Router, in routeInputs) {
	svc, modules, db, logger := in.modules.services, in.modules, in.db, in.logger
	tenant.Mount("/rooms", roomsHTTPAdapter.NewResource(modules.rooms, roomsHTTPAdapter.Dependencies{
		Facilities: svc.Facilities, Settings: svc.Settings,
		UserContext: svc.UserContext, Active: svc.Active,
		Users: services.NewRoomSnapshotPeople(svc.Users), Education: svc.Education,
		ListExport: svc.ListExport,
	}, logger.With("handler", "rooms")).Router())
	tenant.Mount("/groups", groupsHTTP.NewResource(svc.Education, svc.Active, services.NewGroupRoutePeople(svc.Users), svc.UserContext).Router())
	tenant.Mount("/activities", timetableHTTPAdapter.NewResource(svc.Activities, modules.timetable, svc.Users, svc.UserContext, db).Router())
	// Parent enrollment (PR 5+).
	tenant.Mount("/enrollment", newEnrollmentResource(modules).Router())
	// Info-point displays (#1325). One Device Fleet owner serves every entry
	// point: the services factory composes it and the IoT service hands it
	// back here (#2676).
	tenant.Mount("/display", displayHTTPAdapter.NewResource(svc.IoT.Fleet(), svc.Settings).Router())
	// Dateframes belong to the School Calendar, timeframes and recurrence
	// rules to the Timetable owner; timeframe changes stay guarded by the
	// recurrence gate and the Care Plan catalog's care-offering check.
	tenant.Mount("/schedules", timetableHTTPAdapter.NewSchedulesResource(modules.calendar, modules.timetable, services.TimeframeChangeGuard(
		svc.TimetableData.RecurrenceLock.LockRecurrenceWrites,
		svc.EnrollmentCareOffering.ValidateTimeframeChange,
	), db).Router())
	// The schema-driven settings system.
	homeLayouts := requireHomeLayoutOperations(svc.Settings)
	tenant.Mount("/settings", newSettingsResource(svc.TenantSettings, homeLayouts, modules.repositories.Enrollment().SchemaReferencesLegalDocument).SettingsRouter())
	// Class-list-only entries (#2382).
	tenant.Mount("/class-list-entries", newClassListEntriesResource(modules.membership, logger.With("handler", "class-list-entries")).Router())
	tenant.Mount("/database", databaseStatsRouter(in))
	// CSV/Excel import endpoints.
	tenant.Mount("/import", importAPI.NewResource(importAPI.Dependencies{
		Students: svc.Import, Staff: svc.StaffImport, ClassList: svc.ClassListImport,
		Files: fileformat.Decoder{}, Runtime: importCompose.HTTPRuntime(db, svc.PeopleDirectory, modules.membership, svc.OpeningBalanceImport),
	}).Router())
	tenant.Mount("/admin/grade-transitions", gradeTransitionHTTP.NewGradeTransitionResource(svc.GradeTransition).Router())
}

func newEnrollmentResource(modules moduleServices) *enrollmentAPI.Resource {
	svc := modules.services
	resource := enrollmentAPI.NewResource(
		svc.EnrollmentFormSchema,
		enrollmentAPI.NewCareOfferingCatalog(services.NewEnrollmentCareOfferingValues(svc.EnrollmentCareOffering)),
		enrollmentAPI.NewRequestService(svc.EnrollmentRequest),
		svc.EnrollmentCaptcha,
		svc.EnrollmentPhase,
		enrollmentAPI.NewDecisionService(svc.EnrollmentDecision),
		svc.EnrollmentReport,
		enrollmentAPI.NewRolloverService(svc.EnrollmentRollover),
		enrollmentAPI.NewChangeRequestService(svc.EnrollmentChangeRequest),
		svc.EnrollmentDeletion,
		enrollmentGuardianInvitations(svc.GuardianInvitation),
		services.NewEnrollmentGuardianAutofill(svc.GuardianProfileLoader),
		enrollmentSchoolDirectory{schools: svc.Schools},
		modules.repositories.Enrollment(),
	)
	resource.ListExportService = svc.ListExport
	resource.PhaseExpiryService = svc.EnrollmentPhaseExpiry
	return resource
}

// mountPeopleRoutes serves the people directory, guardians, statistics,
// birthdays, the caller's own context, the emergency snapshot, feedback and
// the meal plan under /api.
func mountPeopleRoutes(tenant chi.Router, in routeInputs) {
	svc, modules, logger := in.modules.services, in.modules, in.logger
	tenant.Mount("/statistics", statisticsAPI.NewResource(svc.Statistics, svc.ListExport, logger.With("handler", "statistics")).Router())
	tenant.Mount("/guardians", newGuardiansResource(svc.PeopleDirectory, svc.NewGuardianDirectoryRuntime(in.db), viper.GetString("app_env"), logger.With("handler", "guardians")).Router())
	tenant.Mount("/feedback", newFeedbackResource(modules.feedback).Router())
	tenant.Mount("/meal-plan", newMealPlanResource(modules.mealPlan, newMealPlanExportRenderer()).Router())
	tenant.Mount("/users", newUsersResource(modules.persons, svc.Auth.ListAccountEmails, func(ctx context.Context, tagID string) (bool, error) {
		_, _, found, err := modules.repositories.RFIDCard.LookupRFIDCard(ctx, tagID)
		return found, err
	}).Router())
	// Birthday display and staff birthday list (#1542).
	tenant.Mount("/birthdays", birthdaysAPI.NewResource(svc.Birthdays, svc.ListExport, svc.UserContext, logger.With("handler", "birthdays")).Router())
	tenant.Mount("/me", meAPI.NewResource(svc.UserContext.Caller(), svc.UserContext).Router())
	tenant.Mount("/emergency", emergencyAPI.NewResource(svc.Emergency).Router())
}

// mountCommunicationRoutes serves parent messages and announcements, staff
// reminders, the PWA usage report and the platform announcements under /api.
func mountCommunicationRoutes(tenant chi.Router, in routeInputs) {
	svc := in.modules.services
	tenant.Mount("/messages", messagingAPI.NewResource(svc.Messaging).Router())
	tenant.Mount("/parent-announcements", announcementAPI.NewResource(svc.ParentAnnouncement, newDeclarationReports()).Router())
	// Visual-only staff reminders (#1457).
	tenant.Mount("/reminders", remindersAPI.NewResource(svc.Reminders, reminderCompose.HTTPRuntime()).Router())
	// PWA standalone-usage reporting (#2189).
	tenant.Mount("/pwa", pwaUsageRouter(svc.PWAUsage))
	// User-facing platform announcements.
	tenant.Mount("/platform", platformAPI.NewResource(platformAPI.ResourceConfig{
		AnnouncementsService: svc.Announcement,
		Runtime:              newPlatformRuntime(),
	}).Router())
}

// mountCalendarRoutes serves the personal calendars under /api, the
// read-only staff CalDAV and the public subscription feeds.
func mountCalendarRoutes(root, tenant chi.Router, in routeInputs) {
	svc := in.modules.services
	calendar := calendarAPI.NewResource(svc.Calendar, in.logger.With("handler", "calendar"))
	tenant.Mount("/calendar", calendar.Router())

	// Public parent calendar subscription feed (no auth — the token in the URL
	// is the capability). Calendar apps (Apple/Google/Outlook) poll this to keep
	// the parent's Termine in sync.
	root.Get("/public/calendar/{token}", publicCalendarFeed(in.modules))

	// Read-only staff CalDAV. Authentication happens inside the protocol
	// handler with the tenant-bound calendar app password, before a tenant is
	// known, so these routes intentionally sit outside the JWT tenant group.
	calDAVHandler := restoreCalDAVMethod(http.HandlerFunc(calendar.ServeCalDAV))
	root.Handle("/.well-known/caldav", calDAVHandler)
	root.Handle("/api/caldav", calDAVHandler)
	root.Handle("/api/caldav/*", calDAVHandler)
}

// mountDeliveryRoutes serves the live streams and the file storage: the staff
// routes under /api and the portal routes at the root. The portal routes sit
// at the root because /parent is a catch-all mount; ParentMiddleware and
// SchoolMiddleware authenticate them.
func mountDeliveryRoutes(root, tenant chi.Router, in routeInputs) {
	svc := in.modules.services
	files := filestoreAPI.NewResource(svc.FileStore, in.logger.With("handler", "filestore"))
	sse := sseAPI.NewResource(svc.RealtimeHub, svc.UserContext, in.db, in.logger.With("handler", "sse"))
	sse.SetSchoolAccess(svc.Auth)
	tenant.Mount("/files", files.Router())
	// Anhänge an Elternmitteilungen (#2890). Eigener Pfad statt einer
	// Route unter /parent-announcements: die Bytes gehören der
	// Dateiablage, die Mitteilung steuert nur den Empfängerkreis bei.
	tenant.Mount("/announcement-attachments", files.AnnouncementAttachmentRouter())
	tenant.Mount("/sse", sse.Router())
	// Parent-portal SSE stream: only whitelisted triggers (parent_message)
	// for the tenants of the guardian's children.
	root.Mount("/parent-sse", sse.ParentRouter())
	// Anhänge, die Eltern zu einer Mitteilung herunterladen (#2890). Der
	// Empfängerkreis der Mitteilung entscheidet; wer nicht dazugehört,
	// bekommt 404.
	root.Mount("/parent-news-attachments", files.ParentAnnouncementAttachmentRouter())
	// School-portal SSE stream (#2208): account-addressed triggers only
	// (Team-Chat).
	root.Mount("/school-sse", sse.SchoolRouter())
}

// mountPortalRoutes mounts the root-level session portals: tenant auth,
// operator dashboard (separate from the tenant API) and the cross-tenant
// guardian portal. The auth rate limiter guards each login.
func mountPortalRoutes(root chi.Router, in routeInputs, limiters authRateLimiters) error {
	// RouterWithAuthRateLimiter applies the limiter only to the public login,
	// password-reset, MFA and passkey-login routes.
	root.Mount("/auth", newAccountResource(in).RouterWithAuthRateLimiter(limiters.authMiddleware()))
	operator, err := newOperatorResource(in)
	if err != nil {
		return err
	}
	root.Mount("/operator", operatorRouter(operator, limiters))
	root.Mount("/parent", newParentResource(in).RouterWithAuthRateLimiter(limiters.authMiddleware()))
	return nil
}

// newAccountResource composes the tenant account routes. Their caregiver
// capability is the one the operator routes provision with.
func newAccountResource(in routeInputs) *authAPI.Resource {
	svc := in.modules.services
	authSchools := authSchoolDirectory{schools: svc.Schools, memberships: svc.Auth.ListActiveAccountSchoolIDs}
	resource := authAPI.NewResource(svc.Auth, svc.Invitation, authSchools, svc.AccountAuthentication(), services.AccountRouteTenantRuntime())
	resource.CaregiverCapabilityService = svc.CaregiverCapabilityViews(in.db)
	resource.SettingsService = svc.Settings
	resource.RoleGrants = services.RoleGrantPolicy{}
	resource.MFAService = svc.MFA
	resource.PasskeyService = svc.Passkey
	return resource
}

// newOperatorResource composes the operator dashboard. The Device Fleet review
// of unregistered RFID scans answers in the operator surface's format (#3232).
func newOperatorResource(in routeInputs) (*operatorAPI.Resource, error) {
	svc := in.modules.services
	tagScanReview := tagScanOperatorAPI.NewResource(tagScanOperatorAPI.Config{
		Scans:     svc.IoT.Fleet(),
		Directory: tagScanSchoolDirectory{schools: svc.Schools},
		Surface: tagScanOperatorAPI.Surface{
			InvalidRequest: apiCommon.OperatorInvalidRequest,
			Internal:       apiCommon.OperatorInternal,
			RenderError:    apiCommon.RenderError,
			Respond:        apiCommon.Respond,
			OperatorID: func(ctx context.Context) int64 {
				return int64(projectJWT.ClaimsFromCtx(ctx).ID)
			},
		},
	})
	billing, err := newOperatorBilling(in.logger)
	if err != nil {
		return nil, fmt.Errorf("compose operator billing: %w", err)
	}
	schoolSettings := settingsCompose.NewOperatorSchoolSettings(settingsCompose.OperatorDependencies{
		Settings:       svc.Settings,
		DB:             in.db,
		Notify:         svc.SettingsChangedNotifier(),
		OpenAttendance: svc.OpenAttendanceChecker(),
		CareLifecycle:  svc.CareLifecycle,
		// Mirror the tenant-side OnValueSet hook so operator writes also
		// trigger side effects (e.g. auto-creating the Schulhof/WC rooms when
		// the corresponding checkout toggle flips on).
		OnValueSet: svc.SettingsSideEffects.Dispatch,
	})
	return operatorAPI.NewResource(operatorAPI.ResourceConfig{
		AppEnv:      viper.GetString("app_env"),
		AuthService: svc.OperatorAuth,
		IsLocalSeedRequest: func(r *http.Request) bool {
			return apiCommon.IsLocalSeedRequest(r, viper.GetString("app_env"))
		},
		Identity:             identityOperatorAPI.NewResource(svc.AccountAuthentication(), operatorAPI.IdentityResponses()),
		PasskeyService:       svc.OperatorPasskey,
		MFAService:           svc.OperatorMFA,
		InvitationService:    svc.OperatorInvitation,
		ProvisioningService:  svc.OperatorProvisioning,
		Caregivers:           svc.CaregiverCapabilityViews(in.db),
		AnnouncementsService: svc.Announcement,
		UnregisteredTagScans: tagScanReview.Router(),
		SchoolSettings:       schoolSettings,
		SchoolService:        svc.Schools,
		Billing:              billing,
		TenantMFAService:     svc.MFA,
		Sessions:             identityOperatorAPI.NewSessions(in.sessionAuth, svc.OperatorAuth),
	}), nil
}

// newParentResource composes the cross-tenant guardian portal: public
// /parent/auth/login and the protected /parent/* routes.
func newParentResource(in routeInputs) *parentAPI.Resource {
	svc := in.modules.services
	return parentAPI.NewResource(parentAPI.ResourceConfig{
		Auth:                  parentPortalLogin(svc.ParentPortalLogin()),
		Resets:                parentPasswordResets(svc.ParentPasswordResetRuntime()),
		Parent:                svc.Parent,
		Calendar:              svc.Calendar,
		Enrollment:            parentEnrollmentForms(enrollmentAPI.NewRequestService(svc.EnrollmentRequest)),
		GuardianProfileLoader: svc.GuardianProfileLoader,
		Schools:               parentSchoolDirectory{schools: svc.Schools},
		Push:                  svc.PushSubscriptions,
		Preferences:           svc.NotificationPreferences,
		PWAUsage:              svc.PWAUsage,
		Reports:               newDeclarationReports(),
		DB:                    in.db,
	})
}

func newPlatformRuntime() platformAPI.Runtime {
	return platformAPI.Runtime{
		// TenantMiddleware supplies the scope guard: parent- and school-scope
		// tokens are rejected with 401 (#2207) — before it, this group was the
		// only /api mount without a scope check, so a portal-bound token could
		// read tenant announcements here. Tenant, org, and platform tokens pass
		// unchanged.
		Protected: func(router chi.Router, register func(chi.Router)) {
			router.Group(func(r chi.Router) {
				r.Use(projectJWT.Authenticator)
				r.Use(apiCommon.ReadOnlyPreviewMiddleware)
				r.Use(apiCommon.TenantScopeMiddleware)
				r.Use(apiCommon.SecurityPrincipalMiddleware)
				register(r)
			})
		},
		Viewer: func(r *http.Request) platformAPI.Viewer {
			claims := projectJWT.ClaimsFromCtx(r.Context())
			return platformAPI.Viewer{AccountID: int64(claims.ID), Roles: claims.Roles, TenantID: claims.TenantID, OrgID: claims.OrgID}
		},
		IDParam: apiCommon.ParseInt64IDWithError,
		Success: apiCommon.Respond,
		Failure: func(w http.ResponseWriter, r *http.Request, failure platformAPI.Failure) {
			apiCommon.RenderError(w, r, apiCommon.ErrorInternalServerWrap(failure.Message, failure.Err))
		},
	}
}

func requireHomeLayoutOperations(settings any) configAPI.HomeLayoutOperations {
	homeLayouts, ok := settings.(configAPI.HomeLayoutOperations)
	if !ok {
		panic("settings platform does not provide home layout operations")
	}
	return homeLayouts
}

// currentStaffID resolves the caller's staff ID at request time.
func currentStaffID(modules moduleServices) func(context.Context) (int64, error) {
	return func(ctx context.Context) (int64, error) {
		return modules.services.UserContext.Caller().StaffID(ctx)
	}
}

type calDAVOriginalMethodKey struct{}

var calDAVExtensionMethods = map[string]struct{}{
	"ACL": {}, "COPY": {}, "LOCK": {}, "MKCALENDAR": {}, "MKCOL": {},
	"MOVE": {}, "PROPFIND": {}, "PROPPATCH": {}, "REPORT": {}, "UNLOCK": {},
}

func normalizeCalDAVMethodForRouting(r *http.Request) *http.Request {
	isCalDAVPath := r.URL.Path == "/.well-known/caldav" || r.URL.Path == "/api/caldav" || strings.HasPrefix(r.URL.Path, "/api/caldav/")
	if _, ok := calDAVExtensionMethods[r.Method]; !ok || !isCalDAVPath {
		return r
	}
	ctx := context.WithValue(r.Context(), calDAVOriginalMethodKey{}, r.Method)
	routed := r.Clone(ctx)
	routed.Method = "QUERY"
	return routed
}

func restoreCalDAVMethod(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		original, _ := r.Context().Value(calDAVOriginalMethodKey{}).(string)
		if original == "" {
			handler.ServeHTTP(w, r)
			return
		}
		restored := r.Clone(r.Context())
		restored.Method = original
		handler.ServeHTTP(w, restored)
	})
}

// authRateLimiters bundles the auth-endpoint rate limiters. Each field is nil
// when rate limiting is disabled; separate instances give email-confirm and
// invitation endpoints independent per-IP counters so they can't exhaust each
// other's budget.
type authRateLimiters struct {
	auth         *customMiddleware.RateLimiter
	emailConfirm *customMiddleware.RateLimiter
	invitation   *customMiddleware.RateLimiter
}

// buildAuthRateLimiters constructs the stricter auth-endpoint rate limiters
// from RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE (default 5), wiring the security
// logger when present. exemptLoopback lets loopback peers through (demo only).
func buildAuthRateLimiters(securityLogger *customMiddleware.SecurityLogger, configuredLimit string, exemptLoopback bool) authRateLimiters {
	authLimit := 5 // default: 5 requests per minute for auth
	if limit := configuredLimit; limit != "" {
		if parsed, err := strconv.Atoi(limit); err == nil && parsed > 0 {
			authLimit = parsed
		}
	}
	limiters := authRateLimiters{
		auth:         customMiddleware.NewRateLimiter(authLimit, 10), // allow reasonable burst for login attempts
		emailConfirm: customMiddleware.NewRateLimiter(authLimit, 10),
		invitation:   customMiddleware.NewRateLimiter(authLimit, 10),
	}
	if exemptLoopback {
		limiters.auth.ExemptLoopback()
		limiters.emailConfirm.ExemptLoopback()
		limiters.invitation.ExemptLoopback()
	}
	if securityLogger != nil {
		limiters.auth.SetLogger(securityLogger)
		limiters.emailConfirm.SetLogger(securityLogger)
		limiters.invitation.SetLogger(securityLogger)
	}
	return limiters
}

// authRateLimitersFromEnv builds the auth-endpoint limiters while
// RATE_LIMIT_ENABLED is set. When it is not, the zero-value limiters carry
// nil fields and the portals mount without them.
func authRateLimitersFromEnv() authRateLimiters {
	if os.Getenv("RATE_LIMIT_ENABLED") != "true" {
		return authRateLimiters{}
	}
	var securityLogger *customMiddleware.SecurityLogger
	if os.Getenv("SECURITY_LOGGING_ENABLED") == "true" {
		securityLogger = customMiddleware.NewSecurityLogger()
	}
	return buildAuthRateLimiters(securityLogger, os.Getenv("RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE"), demoLoopbackExempt())
}

// authMiddleware is the auth limiter's middleware, nil while rate limiting is
// disabled. Tenant, operator, parent and school login share its budget.
func (limiters authRateLimiters) authMiddleware() func(http.Handler) http.Handler {
	if limiters.auth == nil {
		return nil
	}
	return limiters.auth.Middleware()
}

// operatorRouter applies the auth limiter to operator login for brute-force
// protection and the dedicated limiters to e-mail confirmation and
// invitations, then builds the operator routes.
func operatorRouter(operator *operatorAPI.Resource, limiters authRateLimiters) chi.Router {
	if limiters.auth != nil {
		operator.SetAuthRateLimiter(limiters.auth.Middleware())
	}
	if limiters.emailConfirm != nil {
		operator.SetEmailConfirmRateLimiter(limiters.emailConfirm.Middleware())
	}
	if limiters.invitation != nil {
		operator.SetInvitationRateLimiter(limiters.invitation.Middleware())
	}
	return operator.Router()
}

// mountPublicRoutes registers unauthenticated root-level routes: the
// landing/health probes, the public image/legal-document servers, and the
// bearer-protected metrics endpoint.
func mountPublicRoutes(root chi.Router, metricsToken string) {
	root.Get("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("MOTO API - Phoenix Project"))
	})

	root.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})

	// Note: Avatar files are served through authenticated endpoints, not as static files
	// This prevents unauthorized access to user avatars

	// Public login image serving (no auth - displayed on the login page before authentication)
	root.Get("/public/login-image/{filename}", func(w http.ResponseWriter, r *http.Request) {
		filename := chi.URLParam(r, "filename")
		apiCommon.ServeImage(w, r, "public/uploads/login-images", filename, "public, max-age=86400")
	})

	// Public enrollment legal document serving (no auth - parents read it before submitting).
	root.Get("/public/enrollment-legal-documents/{filename}", func(w http.ResponseWriter, r *http.Request) {
		filename := chi.URLParam(r, "filename")
		apiCommon.ServeFile(w, r, "public/uploads/enrollment-legal-documents", filename, "public, max-age=86400")
	})
	root.Get("/public/enrollment-form-legal-documents/{filename}", func(w http.ResponseWriter, r *http.Request) {
		filename := chi.URLParam(r, "filename")
		apiCommon.ServeFile(w, r, "public/uploads/enrollment-form-legal-documents", filename, "public, max-age=86400")
	})

	root.With(metricsAuthMiddleware(metricsToken)).Handle("/internal/metrics", metricsHandler())
}

func databaseStatsRouter(in routeInputs) chi.Router {
	svc := in.modules.services
	return newDatabaseStatsRouter(services.NewDatabaseStatsReader(svc.Database, svc.DatabaseStatsCapabilities), in.logger.With("handler", "database"))
}

func newDatabaseStatsRouter(read services.DatabaseStatsReader, logger *slog.Logger) chi.Router {
	router := chi.NewRouter()
	apiCommon.ProtectedTenantRoutes(router, func(router chi.Router, withTx apiCommon.Middleware) {
		// The reader redacts every count the caller may not read, so the
		// route opens for any permission authorize.NewDatabaseStatsCapabilities
		// honours: the Datenverwaltung hub shows its counts to a lead role a
		// school defines itself, not only to system administrators (#3469).
		router.With(apiCommon.RequiresAnyPermission(services.DatabaseStatsPermissions()...), withTx).
			Get("/stats", func(w http.ResponseWriter, r *http.Request) { serveDatabaseStats(w, r, read, logger) })
	})
	return router
}

func serveDatabaseStats(w http.ResponseWriter, r *http.Request, read services.DatabaseStatsReader, logger *slog.Logger) {
	stats, err := read(r.Context())
	if err != nil {
		logger.Error("failed to get database stats",
			"error", err,
		)
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServerWrap("Internal server error", err))
		return
	}
	apiCommon.RespondWithJSON(w, r, http.StatusOK, stats)
}

// publicCalendarFeed serves parent and staff iCalendar subscription feeds.
// There is no auth — the token in the URL is the capability.
func publicCalendarFeed(modules moduleServices) http.HandlerFunc {
	svc := modules.services
	return func(w http.ResponseWriter, r *http.Request) {
		if svc.Calendar == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		token := chi.URLParam(r, "token")
		filename, content, err := svc.Calendar.ParentCalendarFeedByToken(r.Context(), token)
		if errors.Is(err, calendarService.ErrNotFound) {
			filename, content, err = svc.Calendar.StaffCalendarFeedByToken(r.Context(), token)
		}
		if err != nil {
			if errors.Is(err, calendarService.ErrNotFound) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			slog.Error("calendar feed failed",
				"error", err.Error(),
			)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Content-Disposition", "inline; filename=\""+filename+"\"")
		w.Header().Set("Cache-Control", "private, max-age=3600")
		_, _ = w.Write([]byte(content))
	}
}

// newMealPlanExportRenderer keeps the export adapter construction in the API
// composition layer so both production and route tests receive a real renderer.
func newMealPlanExportRenderer() services.SimpleListRenderer {
	return services.NewSimpleListRenderer()
}

// newSessionTokenAuth resolves the signing configuration once for the API root.
func newSessionTokenAuth() (*projectJWT.TokenAuth, error) {
	tokenAuth, err := projectJWT.NewTokenAuthWithDurations(
		viper.GetString("auth_jwt_secret"),
		viper.GetDuration("auth_jwt_expiry"),
		viper.GetDuration("auth_jwt_refresh_expiry"),
	)
	if err != nil {
		return nil, fmt.Errorf("invalid auth JWT configuration: %w", err)
	}
	return tokenAuth, nil
}

// newStudentPresence composes the Student Presence owner for the serving
// root with its attendance rules bound to the Timetable planned roster and
// the Care Plan reads (#2762).
func newStudentPresence(db *bun.DB, logger *slog.Logger) *studentpresence.Module {
	module, err := repositories.NewStudentPresence(db, func(o presenceCompose.Observation) {
		logger.Debug("student presence operation",
			"operation", o.Operation,
			"duration", o.Duration,
			"queries", o.Queries,
			"rows", o.Rows,
			"error", o.Err,
		)
	})
	if err != nil {
		panic(err)
	}
	return module
}
