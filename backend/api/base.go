package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	jwxjwt "github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/lestrrat-go/jwx/v3/transform"
	slogchi "github.com/samber/slog-chi"
	"github.com/spf13/viper"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/analytics"
	absencetypesAPI "github.com/moto-nrw/project-phoenix/api/absence-types"
	activeAPI "github.com/moto-nrw/project-phoenix/api/active"
	activitiesAPI "github.com/moto-nrw/project-phoenix/api/activities"
	adminAPI "github.com/moto-nrw/project-phoenix/api/admin"
	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	birthdaysAPI "github.com/moto-nrw/project-phoenix/api/birthdays"
	calendarAPI "github.com/moto-nrw/project-phoenix/api/calendar"
	classdayAPI "github.com/moto-nrw/project-phoenix/api/classday"
	classlistentriesAPI "github.com/moto-nrw/project-phoenix/api/classlistentries"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	configAPI "github.com/moto-nrw/project-phoenix/api/config"
	displayAPI "github.com/moto-nrw/project-phoenix/api/display"
	emergencyAPI "github.com/moto-nrw/project-phoenix/api/emergency"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/api/enrollment"
	groupsAPI "github.com/moto-nrw/project-phoenix/api/groups"

	importAPI "github.com/moto-nrw/project-phoenix/api/import"
	iotAPI "github.com/moto-nrw/project-phoenix/api/iot"
	remindersAPI "github.com/moto-nrw/project-phoenix/api/reminders"
	roomsAPI "github.com/moto-nrw/project-phoenix/api/rooms"
	schedulesAPI "github.com/moto-nrw/project-phoenix/api/schedules"
	schoolAPI "github.com/moto-nrw/project-phoenix/api/school"
	shifttypesAPI "github.com/moto-nrw/project-phoenix/api/shift-types"
	staffshiftsAPI "github.com/moto-nrw/project-phoenix/api/staff-shifts"
	statisticsAPI "github.com/moto-nrw/project-phoenix/api/statistics"
	studentsAPI "github.com/moto-nrw/project-phoenix/api/students"
	substitutionsAPI "github.com/moto-nrw/project-phoenix/api/substitutions"
	timeTrackingAPI "github.com/moto-nrw/project-phoenix/api/time-tracking"
	timetableAPI "github.com/moto-nrw/project-phoenix/api/timetable"
	usercontextAPI "github.com/moto-nrw/project-phoenix/api/usercontext"
	worktimemodelsAPI "github.com/moto-nrw/project-phoenix/api/work-time-models"
	notificationsAPI "github.com/moto-nrw/project-phoenix/modules/delivery/http/notifications"
	sseAPI "github.com/moto-nrw/project-phoenix/modules/delivery/http/sse"
	calendarService "github.com/moto-nrw/project-phoenix/services/calendar"

	announcementAPI "github.com/moto-nrw/project-phoenix/api/announcement"
	filestoreAPI "github.com/moto-nrw/project-phoenix/api/filestore"
	messagingAPI "github.com/moto-nrw/project-phoenix/api/messaging"
	operatorAPI "github.com/moto-nrw/project-phoenix/api/operator"
	parentAPI "github.com/moto-nrw/project-phoenix/api/parent"
	platformAPI "github.com/moto-nrw/project-phoenix/api/platform"
	staffMessagingAPI "github.com/moto-nrw/project-phoenix/api/staffmessaging"

	projectJWT "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	customMiddleware "github.com/moto-nrw/project-phoenix/middleware"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	feedbackCompose "github.com/moto-nrw/project-phoenix/modules/feedback/compose"
	feedbackAPI "github.com/moto-nrw/project-phoenix/modules/feedback/http"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	mealplanCompose "github.com/moto-nrw/project-phoenix/modules/mealplan/compose"
	mealplanAPI "github.com/moto-nrw/project-phoenix/modules/mealplan/http"
	organizationModule "github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	usersAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/http"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	staffHTTP "github.com/moto-nrw/project-phoenix/modules/schoolmembership/http"
	schoolStructureModule "github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/services"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// offeringSourceOptions narrows the enrollment decision service to the
// timetable editor's offering-source view (#2137). Returns nil when the
// concrete service does not implement it (partial test wiring) — the handler
// then responds 500 instead of panicking, matching the resource's nil-dep
// contract.
func offeringSourceOptions(svc enrollmentSvc.DecisionService) enrollmentSvc.OfferingSourceOptionLister {
	lister, ok := svc.(enrollmentSvc.OfferingSourceOptionLister)
	if !ok {
		return nil
	}
	return lister
}

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
	services *services.Factory
	mealPlan *mealplanModule.Module
	feedback *feedbackModule.Module
	persons  *peopleModule.Module
	// membership owns users.staff, users.teachers and users.guests (#2667).
	membership *schoolMembershipModule.Module
}

func initializeModuleServices(repoFactory *repositories.Factory, db *bun.DB, logger *slog.Logger) (moduleServices, error) {
	organizations, err := organizationCompose.New(organizationCompose.Dependencies{
		DB: db,
		Observe: func(observation organizationCompose.Observation) {
			observability.ObserveOrganizationTenancyOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, organizationModule.ErrorCode(observation.Err), observation.Err)
		},
	})
	if err != nil {
		return moduleServices{}, err
	}
	persons, err := peopleCompose.New(peopleCompose.Dependencies{
		DB: db,
		Observe: func(observation peopleCompose.Observation) {
			observability.ObservePeopleDirectoryOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, peopleModule.ErrorCode(observation.Err), observation.Err)
		},
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
	membership, err := schoolMembershipCompose.New(schoolMembershipCompose.Dependencies{
		DB: db,
		Observe: func(observation schoolMembershipCompose.Observation) {
			observability.ObserveSchoolMembershipOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, schoolMembershipModule.ErrorCode(observation.Err), observation.Err)
		},
	})
	if err != nil {
		return moduleServices{}, err
	}
	mealPlanSettings := mealplanCompose.NewSettings()
	mealPlan, err := mealplanCompose.New(mealplanCompose.Dependencies{
		DB:       db,
		Settings: mealPlanSettings,
		Observe: func(observation mealplanCompose.Observation) {
			observability.ObserveMealPlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, observation.Err)
		},
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
	factory, err := services.NewFactoryWithModules(
		repoFactory, db, logger,
		organizations, persons, groups, membership,
		mealPlan, mealPlanSettings.Bind,
		feedbackCapability, feedbackSettings.Bind,
		observability.ObserveAuditAppend,
		observability.ObserveSynchronousDelivery,
		observability.ObserveDurableDelivery,
	)
	if err != nil {
		return moduleServices{}, err
	}
	return moduleServices{services: factory, mealPlan: mealPlan, feedback: feedbackCapability, persons: persons, membership: membership}, nil
}

var mealPlanErrorRules = []apiCommon.ErrorRule{
	{Target: mealplanModule.ErrDisabled, Render: apiCommon.FixedRenderer(apiCommon.ErrorForbidden, errors.New("feature_disabled"))},
	{Target: mealplanModule.ErrInvalidMealDate, Render: apiCommon.FixedRenderer(apiCommon.ErrorInvalidRequest, errors.New("meal plan covers weekdays only (Monday-Friday)"))},
	{Target: mealplanModule.ErrInvalidDishes, Render: apiCommon.ErrorInvalidRequest},
}

func renderMealPlanFailure(w http.ResponseWriter, r *http.Request, err error, internalMessage string) {
	renderer := apiCommon.RulesRenderer(mealPlanErrorRules, apiCommon.ErrorInternalServerRenderer(internalMessage))
	apiCommon.RenderError(w, r, renderer(err))
}

func newMealPlanResource(module *mealplanModule.Module, db *bun.DB) *mealplanAPI.Resource {
	return mealplanAPI.NewResource(module, mealplanAPI.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, mealplanAPI.Middleware)) {
			apiCommon.ProtectedTenantGroup(router, db, register)
		},
		Permission: func(access mealplanAPI.Access) mealplanAPI.Middleware {
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
	})
}

func newUsersResource(module peopleModule.Capability, repoFactory *repositories.Factory, db *bun.DB) *usersAPI.Resource {
	return usersAPI.NewResource(module, usersAPI.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, usersAPI.Middleware)) {
			apiCommon.ProtectedTenantGroup(router, db, register)
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
		AccountEmails: repoFactory.Account.FindEmailsByAccountIDs,
		TagExists: func(ctx context.Context, tagID string) (bool, error) {
			cards, err := repoFactory.RFIDCard.List(ctx, map[string]any{"id": tagID})
			if err != nil {
				return false, err
			}
			return len(cards) > 0, nil
		},
		ObserveResponse: func(status int, code string) {
			observability.ObservePeopleDirectoryHTTPResponse(status, code)
		},
	})
}

func newFeedbackResource(module *feedbackModule.Module, db *bun.DB) *feedbackAPI.Resource {
	return feedbackAPI.NewResource(module, feedbackAPI.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, feedbackAPI.Middleware)) {
			apiCommon.ProtectedTenantGroup(router, db, register)
		},
		Permission: func(permission string) feedbackAPI.Middleware {
			return apiCommon.RequiresPermission(permission)
		},
		Success: apiCommon.Respond,
		Failure: func(w http.ResponseWriter, r *http.Request, failure feedbackAPI.Failure) {
			apiCommon.RenderError(w, r, &apiCommon.ErrResponse{
				Err: failure.Err, HTTPStatusCode: failure.Status,
				Status: failure.Classification, ErrorText: failure.Err.Error(),
			})
		},
		ObserveResponse: func(status int, code string) {
			observability.ObserveFeedbackHTTPResponse("staff", status, code)
		},
	})
}

// API represents the API structure
type API struct {
	Services           *services.Factory
	Router             chi.Router
	db                 *bun.DB
	repos              *repositories.Factory
	tenantRuntime      apiCommon.TenantRuntime
	metrics            *httpMetrics
	tracer             *observability.Tracer
	metricsBearerToken string
	databaseLogger     *slog.Logger
	feedback           *feedbackModule.Module
	membership         *schoolMembershipModule.Module
	securityLogging    bool
	rateLimiting       bool
	authRateLimit      string

	// API Resources
	Auth             *authAPI.Resource
	Rooms            *roomsAPI.Resource
	Students         *studentsAPI.Resource
	Statistics       *statisticsAPI.Resource
	Groups           *groupsAPI.Resource
	Guardians        *usersAPI.GuardianResource
	Import           *importAPI.Resource
	Activities       *activitiesAPI.Resource
	Staff            *staffHTTP.Resource
	StaffAdmin       *timeTrackingAPI.StaffAdminResource
	WorkTimeModels   *worktimemodelsAPI.Resource
	StaffShifts      *staffshiftsAPI.Resource
	ShiftTypes       *shifttypesAPI.Resource
	AbsenceTypes     *absencetypesAPI.Resource
	Feedback         *feedbackAPI.Resource
	MealPlan         *mealplanAPI.Resource
	Enrollment       *enrollmentAPI.Resource
	Display          *displayAPI.Resource
	Schedules        *schedulesAPI.Resource
	Settings         *configAPI.SettingsResource
	Active           *activeAPI.Resource
	IoT              *iotAPI.Resource
	SSE              *sseAPI.Resource
	Users            *usersAPI.Resource
	Birthdays        *birthdaysAPI.Resource
	ClassDay         *classdayAPI.Resource
	ClassListEntries *classlistentriesAPI.Resource
	School           *schoolAPI.Resource
	UserContext      *usercontextAPI.Resource
	Substitutions    *substitutionsAPI.Resource
	GradeTransitions *adminAPI.GradeTransitionResource
	TimeTracking     *timeTrackingAPI.Resource
	Timetable        *timetableAPI.Resource
	Emergency        *emergencyAPI.Resource
	Messaging        *messagingAPI.Resource
	StaffMessaging   *staffMessagingAPI.Resource
	Calendar         *calendarAPI.Resource
	Announcements    *announcementAPI.Resource
	StaffNotices     *timeTrackingAPI.StaffNoticeResource
	FileStore        *filestoreAPI.Resource
	Reminders        *remindersAPI.Resource
	Notifications    *notificationsAPI.Resource

	// Operator Dashboard (platform domain)
	Operator *operatorAPI.Resource
	Parent   *parentAPI.Resource
	Platform *platformAPI.Resource
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

// New creates a new API instance
func New(enableCORS bool, logger *slog.Logger) (result *API, resultErr error) {
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
	db.AddQueryHook(database.NewLockWaitQueryHook(services.ObserveUnitOfWorkLockWait))
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
		db.AddQueryHook(database.NewQueryHook(logger.With("component", "database")))
	}

	// Initialize repository factory with DB connection
	repoFactory := repositories.NewFactory(db)

	// Compose one authoritative instance of each migrated module.
	modules, err := initializeModuleServices(repoFactory, db, logger)
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

	// Create API instance
	httpMetrics := newHTTPMetrics()
	tracer := newRuntimeTracer(logger)
	api := &API{
		Services:           serviceFactory,
		Router:             chi.NewRouter(),
		db:                 db,
		repos:              repoFactory,
		tenantRuntime:      tenantRuntime,
		metrics:            httpMetrics,
		tracer:             tracer,
		metricsBearerToken: metricsBearerToken,
		databaseLogger:     logger.With("handler", "database"),
		feedback:           modules.feedback,
		membership:         modules.membership,
	}

	// Setup router middleware
	api.Router.Use(func(next http.Handler) http.Handler { return requestIDMiddleware(tracer, next) })
	api.Router.Use(apiCommon.TenantRuntimeMiddleware(tenantRuntime))
	api.Router.Use(apiCommon.AuthorizationObserverMiddleware(func(event apiCommon.AuthorizationEvent) {
		observability.RecordAuthorizationEvent(event.Outcome, event.Reason, event.Elapsed)
	}))
	api.Router.Use(apiCommon.TenantRuntimeObserverMiddleware(func(observation apiCommon.TenantRuntimeObservation) {
		recordHTTPRuntimeEvent(tracer, observation)
	}))
	api.Router.Use(apiCommon.TenantRequestObserverMiddleware(func(event apiCommon.TenantRequestEvent) {
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
	setupBasicMiddleware(api.Router, logger, httpMetrics)

	// Setup CORS, security logging, and rate limiting
	if enableCORS {
		setupCORS(api.Router)
	}
	securityLogger := setupSecurityLogging(api.Router)
	setupRateLimiting(api.Router, securityLogger)

	// Initialize API resources
	initializeAPIResources(api, repoFactory, db, logger)
	api.MealPlan = newMealPlanResource(modules.mealPlan, db)
	api.Feedback = newFeedbackResource(modules.feedback, db)
	api.Users = newUsersResource(modules.persons, repoFactory, db)

	// Register routes with rate limiting
	api.securityLogging = os.Getenv("SECURITY_LOGGING_ENABLED") == "true"
	api.rateLimiting = os.Getenv("RATE_LIMIT_ENABLED") == "true"
	api.authRateLimit = os.Getenv("RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE")
	api.registerRoutesWithRateLimiting()

	buildResources.released = true
	return api, nil
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
	// Redact calendar-feed tokens (the sole credential for the public
	// /public/calendar/{token} feed) from the per-request "path" attribute, and
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
	sentryMiddleware := sentryhttp.New(sentryhttp.Options{Repanic: true})
	router.Use(sentryMiddleware.Handle)
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

// setupCORS configures CORS middleware with allowed origins from environment.
// Supports wildcard subdomain patterns like "*.example.com" via AllowOriginFunc.
func setupCORS(router chi.Router) {
	exactOrigins, wildcardSuffixes := parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))

	opts := cors.Options{
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Staff-PIN", "X-Staff-ID", "X-Staff-Auth-PIN", "X-Device-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}

	if len(wildcardSuffixes) > 0 {
		opts.AllowOriginFunc = buildCORSOriginFunc(exactOrigins, wildcardSuffixes)
	} else {
		opts.AllowedOrigins = exactOrigins
	}

	router.Use(cors.Handler(opts))
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

// setupRateLimiting configures rate limiting middleware if enabled
func setupRateLimiting(router chi.Router, securityLogger *customMiddleware.SecurityLogger) {
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
	if tokenAuth, err := projectJWT.NewTokenAuth(); err == nil {
		generalRateLimiter.SetKeyFunc(identityRateLimitKey(tokenAuth))
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

// initializeAPIResources initializes all API resource instances
func initializeAPIResources(api *API, repoFactory *repositories.Factory, db *bun.DB, logger *slog.Logger) {
	deviceLastSeenDebouncer := iotAPI.NewDeviceLastSeenDebouncer()
	api.Auth = authAPI.NewResource(api.Services.Auth, api.Services.Invitation, api.Services.Schools, db)
	api.Auth.CaregiverCapabilityService = api.Services.CaregiverCapability
	api.Auth.SettingsService = api.Services.Settings
	api.Auth.SetMFAService(api.Services.MFA)
	api.Auth.SetPasskeyService(api.Services.Passkey)
	api.Auth.SetGuardianInvitationService(api.Services.GuardianInvitation)
	api.Rooms = roomsAPI.NewResource(roomsAPI.ResourceConfig{
		FacilityService:    api.Services.Facilities,
		SettingsService:    api.Services.Settings,
		UserContextService: api.Services.UserContext,
		Logger:             logger.With("handler", "rooms"),
		DB:                 db,
	})
	api.Rooms.ActiveService = api.Services.Active
	api.Rooms.PersonService = api.Services.Users
	api.Rooms.EducationService = api.Services.Education
	api.Rooms.ListExportService = api.Services.ListExport
	api.Services.EnableStudentPhotos(services.StudentPhotoBootstrap{
		Unlinker:    studentsAPI.NewPhotoUnlinker(logger.With("component", "student-photo-unlinker"), "public"),
		StudentRepo: repoFactory.Student,
		DB:          db,
		Logger:      logger.With("service", "student-photo"),
	})
	// A direct school_class edit must resync Jahrgang-filtered offering-sourced
	// Regeltermine like a grade transition does (#2147 review round 10). The
	// factory already fails startup when the decision service stops
	// implementing the resync, so the assertion cannot silently miss here.
	studentClassResyncer, _ := api.Services.EnrollmentDecision.(educationSvc.OfferingSourceResyncer)
	api.Students = studentsAPI.NewResource(studentsAPI.ResourceConfig{
		PersonService:                api.Services.Users,
		PeopleDirectory:              api.Services.PeopleDirectory,
		StudentService:               api.Services.Students,
		ClassListEntryService:        api.Services.ClassListEntries,
		StudentDeletionService:       api.Services.StudentDeletion,
		CareLifecycleService:         api.Services.CareLifecycle,
		StudentAuditService:          api.Services.StudentAudit,
		EducationService:             api.Services.Education,
		GradeTransitionService:       api.Services.GradeTransition,
		UserContextService:           api.Services.UserContext,
		ActiveService:                api.Services.Active,
		IoTService:                   api.Services.IoT,
		StaffPINAuthenticator:        api.Services.StaffPINAuth,
		DevicePINFallback:            os.Getenv("OGS_DEVICE_PIN"),
		DeviceLastSeenDebouncer:      deviceLastSeenDebouncer,
		PickupScheduleService:        api.Services.PickupSchedule,
		PartialAbsenceService:        api.Services.PartialAbsence,
		ArrivalScheduleService:       api.Services.ArrivalSchedule,
		InstanceService:              api.Services.Instance,
		CareDayService:               api.Services.CareDay,
		SchoolService:                api.Services.Schools,
		SettingsService:              api.Services.Settings,
		MasterDataReviewService:      api.Services.MasterDataReview,
		CareRequestService:           api.Services.CareRequests,
		OfferingChangeService:        api.Services.OfferingChanges,
		PickupAdjustmentService:      api.Services.PickupAdjustments,
		ExcusedRequestService:        api.Services.ExcusedRequests,
		ParentRequestBulkService:     api.Services.ParentRequests,
		ParentRequestConflictService: api.Services.ParentRequests,
		FamilyProtectionService:      api.Services.FamilyProtection,
		RequestReviewAccess:          api.Services.RequestReviewPolicy,
		StudentStatusDayService:      api.Services.StudentStatusDays,
		AbsenceOverview:              api.Services.AbsenceOverview,
		StudentHistoryService:        api.Services.StudentHistory,
		OGSGroupLiveService:          api.Services.OGSGroupLive,
		ActivityService:              api.Services.Activities,
		EnrollmentDecision:           api.Services.EnrollmentDecision,
		EnrollmentFormSchema:         api.Services.EnrollmentFormSchema,
		OfferingSourceResyncer:       studentClassResyncer,
		LockTemplateRecurrence: func(ctx context.Context) error {
			return scheduleSvc.LockTenantRecurrenceWrites(ctx, db)
		},
		Broadcaster:            api.Services.RealtimeHub,
		ParentEventEmitter:     api.Services.ParentEventEmitter,
		AbsenceNotifier:        api.Services.AbsenceNotifier,
		StudentPhotos:          api.Services.StudentPhotos,
		StudentConsents:        api.Services.StudentConsents,
		StudentDocumentService: api.Services.StudentDocuments,
		ListExportService:      api.Services.ListExport,
		Logger:                 logger.With("handler", "students"),
		DB:                     db,
	})
	api.Statistics = statisticsAPI.NewResource(api.Services.Statistics, api.Services.ListExport, db, logger.With("handler", "statistics"))
	api.Messaging = messagingAPI.NewResource(api.Services.Messaging, db)
	api.StaffMessaging = staffMessagingAPI.NewResource(api.Services.StaffMessaging, db)
	api.Calendar = calendarAPI.NewResource(api.Services.Calendar, db, logger.With("handler", "calendar"))
	api.Announcements = announcementAPI.NewResource(api.Services.ParentAnnouncement, db)
	api.StaffNotices = timeTrackingAPI.NewStaffNoticeResource(api.Services.StaffNotice, db)
	api.FileStore = filestoreAPI.NewResource(api.Services.FileStore, db, logger.With("handler", "filestore"))
	api.Groups = groupsAPI.NewResource(api.Services.Education, api.Services.Active, api.Services.Users, api.Services.UserContext, db)
	api.Guardians = newGuardiansResource(api.Services.PeopleDirectory, api.Services, db, viper.GetString("app_env"), logger.With("handler", "guardians"))
	api.Import = importAPI.NewResource(api.Services.Import, api.Services.StaffImport, api.Services.ClassListImport, api.Services.Users, db)
	api.Import.SetOpeningBalanceImportFactory(api.Services.OpeningBalanceImport)
	api.Activities = activitiesAPI.NewResource(api.Services.Activities, api.Services.Schedule, api.Services.Users, api.Services.UserContext, db)
	api.Staff, api.StaffAdmin = newStaffComposition(api.membership, api.Services, db, logger.With("handler", "staff"))
	api.WorkTimeModels = worktimemodelsAPI.NewResource(api.Services.WorkTimeModels, db, logger.With("handler", "work-time-models"))
	api.StaffShifts = staffshiftsAPI.NewResource(api.Services.StaffShifts, api.Services.StaffShiftSeries, api.Services.StaffScheduleOverview, api.Services.Users, api.Services.PlanExport, db, logger.With("handler", "staff-shifts"))
	api.ShiftTypes = shifttypesAPI.NewResource(api.Services.ShiftTypes, api.Services.Activities, db, logger.With("handler", "shift-types"))
	api.AbsenceTypes = absencetypesAPI.NewResource(api.Services.StaffAbsenceType, db, logger.With("handler", "absence-types"))
	api.AbsenceTypes.SetActorResolver(api.currentStaffID)
	api.Enrollment = enrollmentAPI.NewResource(
		api.Services.EnrollmentFormSchema,
		api.Services.EnrollmentCareOffering,
		api.Services.EnrollmentRequest,
		api.Services.EnrollmentCaptcha,
		api.Services.EnrollmentPhase,
		api.Services.EnrollmentDecision,
		api.Services.EnrollmentReport,
		api.Services.EnrollmentRollover,
		api.Services.EnrollmentChangeRequest,
		api.Services.EnrollmentDeletion,
		api.Services.GuardianInvitation,
		api.Services.GuardianProfileLoader,
		api.Services.Schools,
		db,
		repoFactory.FormSchema,
	)
	api.Enrollment.ListExportService = api.Services.ListExport
	api.Enrollment.PhaseExpiryService = api.Services.EnrollmentPhaseExpiry
	api.Display = displayAPI.NewResource(api.Services.Display, api.Services.Settings, db)
	api.Schedules = schedulesAPI.NewResource(api.Services.Schedule, db)
	settingsRuntime := configAPI.NewRuntime(configAPI.RuntimeDependencies{
		Protected: func(r chi.Router, fn func(chi.Router, configAPI.Middleware)) {
			apiCommon.ProtectedTenantGroup(r, db, fn)
		},
		Permission: func(access configAPI.Access) configAPI.Middleware {
			switch access {
			case configAPI.AccessRead:
				return apiCommon.RequireConfigRead()
			case configAPI.AccessManage:
				return apiCommon.RequireConfigManage()
			case configAPI.AccessReadOrWrite:
				return apiCommon.RequireConfigReadOrWrite()
			default:
				return apiCommon.RequireConfigWrite()
			}
		},
		TenantGuard: apiCommon.TenantOperationMiddleware,
		RequestActor: func(ctx context.Context) (int64, int64, []string) {
			principal, err := apiCommon.CurrentPrincipal(ctx)
			if err != nil {
				return 0, 0, nil
			}
			return principal.TenantID(), principal.AccountID(), principal.Permissions()
		},
		Editable:  apiCommon.CanEditConfig,
		Success:   apiCommon.Respond,
		NoContent: apiCommon.RespondNoContent,
		Failure: func(w http.ResponseWriter, r *http.Request, status int, err error) {
			switch status {
			case http.StatusBadRequest:
				apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
			case http.StatusForbidden:
				apiCommon.RenderError(w, r, apiCommon.ErrorForbidden(err))
			case http.StatusNotFound:
				apiCommon.RenderError(w, r, apiCommon.ErrorNotFound(err))
			default:
				apiCommon.RenderError(w, r, apiCommon.ErrorInternalServer(err))
			}
		},
		ImageUpload: func(w http.ResponseWriter, r *http.Request, field string, maxBody int64) (*configAPI.UploadedFile, error) {
			file, err := apiCommon.ParseImage(w, r, field, maxBody)
			if err != nil {
				return nil, err
			}
			return &configAPI.UploadedFile{File: file.File, ContentType: file.ContentType}, nil
		},
		PDFUpload: func(w http.ResponseWriter, r *http.Request, field string, maxFile, maxBody int64) (*configAPI.UploadedFile, error) {
			file, err := apiCommon.ParsePDFWithLimits(w, r, field, maxFile, maxBody)
			if err != nil {
				return nil, err
			}
			return &configAPI.UploadedFile{File: file.File, ContentType: file.ContentType}, nil
		},
		ImageSave:  apiCommon.SaveImage,
		PDFSave:    apiCommon.SavePDF,
		FileRemove: apiCommon.RemoveImage,
		StoredPath: apiCommon.ResolveStoredPath,
		LegalDocumentReference: func(ctx context.Context, storedURL string) (bool, error) {
			publicURL := enrollmentSvc.PublicEnrollmentLegalDocumentURL(storedURL)
			referenced, err := repoFactory.FormSchema.HasLegalDocumentReference(ctx, storedURL, publicURL)
			if err != nil {
				return false, fmt.Errorf("check AGB document references: %w", err)
			}
			return referenced, nil
		},
	})
	api.Settings = configAPI.NewSettingsResource(api.Services.TenantSettings, settingsRuntime)
	api.Active = activeAPI.NewResource(api.Services.Active, api.Services.Users, api.Services.Education, api.Services.Schulhof, api.Services.UserContext, api.Services.Settings, db, logger.With("handler", "active"))
	api.Active.SupervisionDashboardService = api.Services.SupervisionDashboard
	api.IoT = iotAPI.NewResource(iotAPI.ServiceDependencies{
		IoTService:            api.Services.IoT,
		StaffPINAuthenticator: api.Services.StaffPINAuth,
		CheckinService:        api.Services.Checkin,
		StaffClockService:     api.Services.StaffClock,
		UsersService:          api.Services.Users,
		ActiveService:         api.Services.Active,
		ActivitiesService:     api.Services.Activities,
		SettingsService:       api.Services.Settings,
		FacilityService:       api.Services.Facilities,
		EducationService:      api.Services.Education,
		FeedbackService:       api.feedback,
		FeedbackResponseObserver: func(status int, code string) {
			observability.ObserveFeedbackHTTPResponse("iot", status, code)
		},
		PickupScheduleService:   api.Services.PickupSchedule,
		SchoolService:           api.Services.Schools,
		TimetableDataService:    api.Services.TimetableData,
		TimetableBridge:         api.Services.TimetableBridge,
		UnregisteredTagScans:    api.Services.UnregisteredTagScans,
		Broadcaster:             api.Services.RealtimeHub,
		Logger:                  logger.With("handler", "iot"),
		DailyCheckoutFallback:   os.Getenv("STUDENT_DAILY_CHECKOUT_TIME"),
		DevicePINFallback:       os.Getenv("OGS_DEVICE_PIN"),
		DB:                      db,
		DeviceLastSeenDebouncer: deviceLastSeenDebouncer,
	})
	api.SSE = sseAPI.NewResource(api.Services.RealtimeHub, api.Services.UserContext, db, logger.With("handler", "sse"))
	api.SSE.SetSchoolAccess(api.Services.Auth)
	api.Birthdays = birthdaysAPI.NewResource(api.Services.Birthdays, api.Services.ListExport, api.Services.UserContext, api.Services.Settings, db, logger.With("handler", "birthdays"))
	api.UserContext = usercontextAPI.NewResource(api.Services.UserContext, db)
	api.ClassDay = classdayAPI.NewResource(api.Services.EnrollmentReport, api.Services.UserContext, db, logger.With("handler", "class-day"))
	api.ClassListEntries = classlistentriesAPI.NewResource(api.Services.ClassListEntries, db, logger.With("handler", "class-list-entries"))
	api.Substitutions = substitutionsAPI.NewResource(api.Services.Substitution, db)
	api.GradeTransitions = adminAPI.NewGradeTransitionResource(api.Services.GradeTransition, db)
	api.TimeTracking = timeTrackingAPI.NewResource(api.Services.WorkSession, api.Services.StaffAbsence, api.Services.Users, api.Services.Settings, api.Services.StaffShifts, api.Services.StaffAssignments, api.Services.WorkTimeMonth, db)
	api.TimeTracking.HolidayService = api.Services.Holidays
	api.TimeTracking.ClosingDayService = api.Services.ClosingDays
	api.Timetable = timetableAPI.NewResource(timetableAPI.Dependencies{
		CalendarPeriodService:   api.Services.CalendarPeriod,
		ClosingDayService:       api.Services.ClosingDays,
		MaterializationService:  api.Services.Materialization,
		InstanceService:         api.Services.Instance,
		InstanceSeriesConverter: api.Services.InstanceSeriesConverter,
		OperationsService:       api.Services.TimetableOperations,
		TemplateSplitService:    api.Services.TemplateSplit,
		PersonService:           api.Services.Users,
		TimetableData:           api.Services.TimetableData,
		PlanningTrackService:    api.Services.PlanningTracks,
		CareDayService:          api.Services.CareDay,
		UserContextService:      api.Services.UserContext,
		SettingsService:         api.Services.Settings,
		SlotListsService:        api.Services.SlotLists,
		OfferingSourceOptions:   offeringSourceOptions(api.Services.EnrollmentDecision),
		ReportService:           api.Services.EnrollmentReport,
		PlanExportService:       api.Services.PlanExport,
		Broadcaster:             api.Services.RealtimeHub,
		Logger:                  logger.With("handler", "timetable"),
		DB:                      db,
	})
	// The school portal reuses the class-day and the timetable resources, so
	// it is built after both (#2207, #2527).
	api.Notifications = notificationsAPI.NewResource(api.Services.Notifications, api.Services.PushSubscriptions, api.Services.NotificationPreferences, db)
	api.School = schoolAPI.NewResource(api.Services.Auth, api.Services.MFA, api.ClassDay, api.Timetable, api.StaffMessaging, api.Notifications)
	api.Emergency = emergencyAPI.NewResource(api.Services.Emergency, db)
	api.Reminders = remindersAPI.NewResource(api.Services.Reminders, api.Services.UserContext, db)

	// Initialize operator dashboard resources
	api.Operator = operatorAPI.NewResource(operatorAPI.ResourceConfig{
		AppEnv:                     viper.GetString("app_env"),
		AuthService:                api.Services.OperatorAuth,
		PasskeyService:             api.Services.OperatorPasskey,
		MFAService:                 api.Services.OperatorMFA,
		InvitationService:          api.Services.OperatorInvitation,
		ProvisioningService:        api.Services.OperatorProvisioning,
		CaregiverCapabilityService: api.Services.CaregiverCapability,
		AnnouncementsService:       api.Services.Announcement,
		UnregisteredTagScanService: api.Services.UnregisteredTagScans,
		SettingsService:            api.Services.Settings,
		Broadcaster:                api.Services.RealtimeHub,
		SchoolService:              api.Services.Schools,
		ActiveService:              api.Services.Active,
		CareLifecycle:              api.Services.CareLifecycle,
		TenantMFAService:           api.Services.MFA,
		TokenAuth:                  nil, // Created internally by operator API
		DB:                         db,
	})
	// Mirror the tenant-side OnValueSet hook so operator writes also trigger
	// side effects (e.g. auto-creating the Schulhof/WC rooms when the
	// corresponding checkout toggle flips on).
	api.Operator.OnSettingValueSet(api.Services.SettingsSideEffects.Dispatch)
	api.Parent = parentAPI.NewResource(
		api.Services.Auth,
		api.Services.Parent,
		api.Services.EnrollmentRequest,
		api.Services.GuardianProfileLoader,
		api.Services.Schools,
		db,
	)
	api.Parent.SetCalendarService(api.Services.Calendar)
	api.Parent.SetPushService(api.Services.PushSubscriptions)
	api.Parent.SetPWAUsageService(api.Services.PWAUsage)
	api.Parent.SetPreferenceService(api.Services.NotificationPreferences)
	api.Platform = platformAPI.NewResource(platformAPI.ResourceConfig{
		AnnouncementsService: api.Services.Announcement,
		TokenAuth:            nil, // Uses tenant auth middleware
	})
}

func (a *API) currentStaffID(ctx context.Context) (int64, error) {
	staff, err := a.Services.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return 0, err
	}
	if staff == nil {
		return 0, errors.New("current staff member not found")
	}
	return staff.ID, nil
}

// ServeHTTP implements the http.Handler interface for the API
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.Router.ServeHTTP(w, r)
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
// logger when present.
func buildAuthRateLimiters(securityLogger *customMiddleware.SecurityLogger, configuredLimit string) authRateLimiters {
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
	if securityLogger != nil {
		limiters.auth.SetLogger(securityLogger)
		limiters.emailConfirm.SetLogger(securityLogger)
		limiters.invitation.SetLogger(securityLogger)
	}
	return limiters
}

// registerRoutesWithRateLimiting registers all API routes with appropriate rate limiting
func (a *API) registerRoutesWithRateLimiting() {
	// Get security logger if it exists
	var securityLogger *customMiddleware.SecurityLogger
	if a.securityLogging {
		securityLogger = customMiddleware.NewSecurityLogger()
	}

	// Configure auth-specific rate limiting if enabled. When disabled, the
	// zero-value limiters carry nil fields and the setters below are skipped.
	var limiters authRateLimiters
	if a.rateLimiting {
		limiters = buildAuthRateLimiters(securityLogger, a.authRateLimit)
	}

	a.registerPublicRoutes()
	a.registerTenantRoutes()
	a.registerPortalRoutes(limiters)
}

// registerPublicRoutes registers unauthenticated root-level routes: the
// landing/health probes, the public image/legal-document servers, and the
// bearer-protected metrics endpoint.
func (a *API) registerPublicRoutes() {
	a.Router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("MOTO API - Phoenix Project"))
	})

	a.Router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})

	// Note: Avatar files are served through authenticated endpoints, not as static files
	// This prevents unauthorized access to user avatars

	// Public login image serving (no auth - displayed on the login page before authentication)
	a.Router.Get("/public/login-image/{filename}", func(w http.ResponseWriter, r *http.Request) {
		filename := chi.URLParam(r, "filename")
		apiCommon.ServeImage(w, r, "public/uploads/login-images", filename, "public, max-age=86400")
	})

	// Public enrollment legal document serving (no auth - parents read it before submitting).
	a.Router.Get("/public/enrollment-legal-documents/{filename}", func(w http.ResponseWriter, r *http.Request) {
		filename := chi.URLParam(r, "filename")
		apiCommon.ServeFile(w, r, "public/uploads/enrollment-legal-documents", filename, "public, max-age=86400")
	})
	a.Router.Get("/public/enrollment-form-legal-documents/{filename}", func(w http.ResponseWriter, r *http.Request) {
		filename := chi.URLParam(r, "filename")
		apiCommon.ServeFile(w, r, "public/uploads/enrollment-form-legal-documents", filename, "public, max-age=86400")
	})

	// Public parent calendar subscription feed (no auth — the token in the URL
	// is the capability). Calendar apps (Apple/Google/Outlook) poll this to keep
	// the parent's Termine in sync.
	a.Router.Get("/public/calendar/{token}", a.servePublicCalendarFeed)

	a.Router.With(metricsAuthMiddleware(a.metricsBearerToken)).Handle("/internal/metrics", metricsHandler())
}

// registerPortalRoutes mounts the root-level portal routers (tenant auth,
// operator, parent) and applies the auth rate limiters when present.
func (a *API) registerPortalRoutes(limiters authRateLimiters) {
	// Auth routes mounted at root level to match frontend expectations
	// Rate limiting is applied per-route inside Auth.Router() (only login, register, password-reset)
	if limiters.auth != nil {
		a.Auth.SetAuthRateLimiter(limiters.auth.Middleware())
	}
	a.Router.Mount("/auth", a.Auth.Router())

	// Mount operator dashboard routes at root level (separate from tenant API)
	// Apply the same auth rate limiter to operator login for brute-force protection
	if limiters.auth != nil {
		a.Operator.SetAuthRateLimiter(limiters.auth.Middleware())
	}
	if limiters.emailConfirm != nil {
		a.Operator.SetEmailConfirmRateLimiter(limiters.emailConfirm.Middleware())
	}
	if limiters.invitation != nil {
		a.Operator.SetInvitationRateLimiter(limiters.invitation.Middleware())
	}
	a.Router.Mount("/operator", a.Operator.Router())

	// Parent (cross-tenant guardian portal). Mounted at the root level
	// like /auth and /operator. Public /parent/auth/login + protected
	// /parent/* routes (the protected ones get added in commit 5).
	// Reuse the shared authRateLimiter so guardian login gets the same
	// brute-force protection as tenant and operator login.
	if limiters.auth != nil {
		a.Parent.SetAuthRateLimiter(limiters.auth.Middleware())
	}
	a.Router.Mount("/parent", a.Parent.Router())

	// School portal ("moto schule", #2207). Mounted at the root level like
	// /parent. Public /school/auth/* (login + school-scope MFA exchange)
	// plus the school-scope class-day surface. Token refresh and logout go
	// through the shared scope-preserving /auth/refresh and /auth/logout.
	if limiters.auth != nil {
		a.School.SetAuthRateLimiter(limiters.auth.Middleware())
	}
	a.Router.Mount("/school", a.School.Router())

	// Parent-portal SSE stream. Mounted at root (not under /parent, which is a
	// catch-all mount) and authenticated with ParentMiddleware. Delivers only
	// whitelisted triggers (parent_message) for the tenants of the guardian's
	// children.
	a.Router.Mount("/parent-sse", a.SSE.ParentRouter())

	// Anhänge, die Eltern zu einer Mitteilung herunterladen (#2890). Root
	// gemountet wie /parent-sse, weil /parent ein Catch-all-Mount ist, und mit
	// ParentMiddleware authentifiziert. Der Empfängerkreis der Mitteilung
	// entscheidet; wer nicht dazugehört, bekommt 404.
	a.Router.Mount("/parent-news-attachments", a.FileStore.ParentAnnouncementAttachmentRouter())

	// School-portal SSE stream (#2208): account-addressed triggers only
	// (Team-Chat), authenticated with SchoolMiddleware. Root-mounted for the
	// same reason as /parent-sse.
	a.Router.Mount("/school-sse", a.SSE.SchoolRouter())
}

// registerTenantRoutes mounts all tenant API resources under the /api prefix.
func (a *API) registerTenantRoutes() {
	// Other API routes under /api prefix for organization
	a.Router.Route("/api", func(r chi.Router) {
		// Mount room resources
		r.Mount("/rooms", a.Rooms.Router())

		// Mount student resources
		r.Mount("/students", a.Students.Router())
		r.Mount("/statistics", a.Statistics.Router())
		r.Mount("/messages", a.Messaging.Router())
		// OGS-internal colleague chat (#2598) — staff-to-staff, deliberately a
		// separate surface from /messages (which is parent-facing).
		r.Mount("/staff-messages", a.StaffMessaging.Router())
		r.Mount("/parent-announcements", a.Announcements.Router())
		// Tagesinformationen (#2180): lesen alle Mitarbeitenden, schreiben Admins.
		r.Mount("/staff-notices", a.StaffNotices.Router())
		r.Mount("/files", a.FileStore.Router())

		// Anhänge an Elternmitteilungen (#2890). Eigener Pfad statt einer
		// Route unter /parent-announcements: die Bytes gehören der
		// Dateiablage, die Mitteilung steuert nur den Empfängerkreis bei.
		r.Mount("/announcement-attachments", a.FileStore.AnnouncementAttachmentRouter())

		// Mount guardian resources
		r.Mount("/guardians", a.Guardians.Router())

		// Mount group resources
		r.Mount("/groups", a.Groups.Router())

		// Mount activities resources
		r.Mount("/activities", a.Activities.Router())

		// Mount staff resources
		r.Mount("/staff", a.Staff.Router())
		r.Mount("/work-time-models", a.WorkTimeModels.Router())
		r.Mount("/staff-shifts", a.StaffShifts.Router())
		r.Mount("/shift-types", a.ShiftTypes.Router())
		r.Mount("/absence-types", a.AbsenceTypes.Router())

		// Mount personal calendar resources
		r.Mount("/calendar", a.Calendar.Router())

		// Mount feedback resources
		r.Mount("/feedback", a.Feedback.Router())

		// Mount meal plan resources
		r.Mount("/meal-plan", a.MealPlan.Router())

		// Mount enrollment resources (parent-enrollment PR 5+)
		r.Mount("/enrollment", a.Enrollment.Router())

		// Mount info-point display resources (issue #1325)
		r.Mount("/display", a.Display.Router())

		// Mount schedule resources
		r.Mount("/schedules", a.Schedules.Router())

		// Mount settings resources (new schema-driven settings system)
		r.Mount("/settings", a.Settings.SettingsRouter())

		// Mount active resources
		r.Mount("/active", a.Active.Router())

		// Mount IoT resources
		r.Mount("/iot", a.IoT.Router())

		// Mount users resources
		r.Mount("/users", a.Users.Router())

		// Birthday display + staff birthday list (#1542)
		r.Mount("/birthdays", a.Birthdays.Router())

		// Mount user context resources
		r.Mount("/me", a.UserContext.Router())

		// Mount class-list-only entries (#2382)
		r.Mount("/class-list-entries", a.ClassListEntries.Router())

		// Mount substitutions resources
		r.Mount("/substitutions", a.Substitutions.Router())

		// Mount database resources
		r.Mount("/database", a.databaseStatsRouter())

		// Mount import resources (CSV/Excel import endpoints)
		r.Mount("/import", a.Import.Router())

		// Mount SSE resources (Server-Sent Events for real-time updates)
		r.Mount("/sse", a.SSE.Router())

		// Mount time-tracking resources
		r.Mount("/time-tracking", a.TimeTracking.Router())

		// Mount timetable resources
		r.Mount("/timetable", a.Timetable.Router())

		// Mount emergency snapshot resources
		r.Mount("/emergency", a.Emergency.Router())

		// Mount reminders resources (visual-only staff reminders, issue #1457)
		r.Mount("/reminders", a.Reminders.Router())

		// Mount notification abstraction resources (issue #1624)
		r.Mount("/notifications", a.Notifications.Router())

		// Mount PWA standalone-usage reporting (issue #2189)
		r.Mount("/pwa", a.pwaUsageRouter())

		// Mount admin resources
		r.Mount("/admin/grade-transitions", a.GradeTransitions.Router())

		// Mount platform resources (user-facing announcements)
		r.Mount("/platform", a.Platform.Router())

		// Add other resource routes here as they are implemented
	})
}

func (a *API) databaseStatsRouter() chi.Router {
	router := chi.NewRouter()
	apiCommon.ProtectedTenantGroup(router, a.db, func(router chi.Router, withTx apiCommon.Middleware) {
		router.With(apiCommon.RequiresPermission("system:manage"), withTx).Get("/stats", a.getDatabaseStats)
	})
	return router
}

func (a *API) getDatabaseStats(w http.ResponseWriter, r *http.Request) {
	stats, err := a.Services.Database.GetStats(r.Context(), a.Services.DatabaseStatsCapabilities(r.Context()))
	if err != nil {
		a.getDatabaseLogger().Error("failed to get database stats",
			"error", err,
		)
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServerWrap("Internal server error", err))
		return
	}
	apiCommon.RespondWithJSON(w, r, http.StatusOK, stats)
}

func (a *API) getDatabaseLogger() *slog.Logger {
	if a.databaseLogger != nil {
		return a.databaseLogger
	}
	return slog.Default()
}

// servePublicCalendarFeed serves parent and staff iCalendar subscription feeds.
// There is no auth — the token in the URL is the capability.
func (a *API) servePublicCalendarFeed(w http.ResponseWriter, r *http.Request) {
	if a.Services.Calendar == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	token := chi.URLParam(r, "token")
	filename, content, err := a.Services.Calendar.ParentCalendarFeedByToken(r.Context(), token)
	if errors.Is(err, calendarService.ErrNotFound) {
		filename, content, err = a.Services.Calendar.StaffCalendarFeedByToken(r.Context(), token)
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
