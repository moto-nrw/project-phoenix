package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	iotAPI "github.com/moto-nrw/project-phoenix/api/iot/compose"
	customMiddleware "github.com/moto-nrw/project-phoenix/middleware"
	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	careStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/careplan/http"
	requestFeedCompose "github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/compose"
	requestFeedHTTP "github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/http"
	classdayHTTP "github.com/moto-nrw/project-phoenix/modules/classday/http"
	announcementAPI "github.com/moto-nrw/project-phoenix/modules/communication/http/parentannouncements"
	messagingAPI "github.com/moto-nrw/project-phoenix/modules/communication/http/parentmessages"
	platformAnnouncementsHTTP "github.com/moto-nrw/project-phoenix/modules/communication/http/platformannouncements"
	staffMessagingAPI "github.com/moto-nrw/project-phoenix/modules/communication/http/staffmessages"
	dataImportCompose "github.com/moto-nrw/project-phoenix/modules/dataimport/compose"
	importAPI "github.com/moto-nrw/project-phoenix/modules/dataimport/http"
	sseAPI "github.com/moto-nrw/project-phoenix/modules/delivery/http/sse"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"
	tagScanOperatorAPI "github.com/moto-nrw/project-phoenix/modules/devicefleet/inbound/operator"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	emergencyAPI "github.com/moto-nrw/project-phoenix/modules/emergencysnapshot/http"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	roomsHTTPAdapter "github.com/moto-nrw/project-phoenix/modules/facilities/compose/httpadapter"
	filestoreAPI "github.com/moto-nrw/project-phoenix/modules/filestorage/http/files"
	groupLiveHTTP "github.com/moto-nrw/project-phoenix/modules/grouplive/http"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	meAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/me"
	identityOperatorAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	provisioningOperatorAPI "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	birthdaysAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/birthdays"
	studentsAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
	projectionStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/presenceprojection/http"
	requestreviewcompose "github.com/moto-nrw/project-phoenix/modules/requestreview/compose"
	reviewStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/requestreview/http"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	calendarAPI "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/http"
	schoolSetupCompose "github.com/moto-nrw/project-phoenix/modules/schoolsetup/compose"
	schoolStructureModule "github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	groupsHTTP "github.com/moto-nrw/project-phoenix/modules/schoolstructure/http"
	settingsFacts "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	settingsHTTP "github.com/moto-nrw/project-phoenix/modules/settings/http"
	settingsOperatorAPI "github.com/moto-nrw/project-phoenix/modules/settings/inbound/operator"
	reviewsettings "github.com/moto-nrw/project-phoenix/modules/settings/review"
	statisticsAPI "github.com/moto-nrw/project-phoenix/modules/statistics/http"
	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	presenceStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/studentpresence/http"
	presenceAPI "github.com/moto-nrw/project-phoenix/modules/studentpresence/inbound/presence"
	supervisionDashboardHTTP "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/http"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetableHTTPAdapter "github.com/moto-nrw/project-phoenix/modules/timetable/compose/httpadapter"
	timetableAPI "github.com/moto-nrw/project-phoenix/modules/timetable/http"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/workflows/careexit"
	gradeTransitionHTTP "github.com/moto-nrw/project-phoenix/workflows/gradetransition/http"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove"
	openRoomMoveHTTP "github.com/moto-nrw/project-phoenix/workflows/openroommove/http"
	parentAPI "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
	remindersHTTP "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/http"
	deleteStudentsHTTP "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/http"
	slogchi "github.com/samber/slog-chi"
	"github.com/spf13/viper"
)

// mountRoutes builds every route resource and mounts it where it is built:
// the public and portal routes at the root, the tenant routes under /api. No
// resource outlives its mount; the Serve graph keeps only the Worker's
// document sweeps. It refuses to start while a writing route has no
// core-action classification.
func mountRoutes(root chi.Router, in routeInputs, limiters authRateLimiters) (documentSweeps, error) {
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
	presence := NewLoggedStudentPresence(in.db, in.logger)
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
		DB: in.db.Pool(), Feed: newParentRequestFeed, FrontendURL: in.frontendURL, Now: time.Now,
		NewToken: sessiontoken.NewOpaqueCapabilityToken, HashToken: sessiontoken.OpaqueCapabilityFingerprint,
	})
	if err != nil {
		return err
	}
	resource := requestFeedHTTP.NewResource(requestFeed, requestFeedHTTP.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, requestFeedHTTP.Middleware)) {
			ProtectedTenantRoutes(router, register)
		},
		CurrentTenantID:  func(r *http.Request) int64 { return sessiontoken.ClaimsFromCtx(r.Context()).TenantID },
		CurrentAccountID: func(r *http.Request) int64 { return int64(sessiontoken.ClaimsFromCtx(r.Context()).ID) },
		Logger:           in.logger.With("handler", "request-feed"),
	})
	root.Mount("/public/request-feed", resource.PublicRouter())
	tenant.Mount("/students/change-requests/rss-feed", resource.TenantRouter())
	return nil
}

// mountModuleRoutes mounts the route groups modules serve themselves.
func mountModuleRoutes(tenant chi.Router, in routeInputs) error {
	schoolSetup, err := newSchoolSetupRoute(schoolSetupCompose.Dependencies{
		Settings: SchoolSetupSettings{Settings: in.modules.services.Settings},
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
	router.Use(ServerErrorReporting)
	router.Use(customMiddleware.SecurityHeaders)
	// Request-scoped settings memo cache (issue #2065). Router-wide so routes
	// outside ProtectedTenantGroup (/auth incl. /auth/tenant/resolve,
	// /operator, /parent, public enrollment) dedupe their settings lookups
	// too. Idempotent with the group-wide attachment in api/common.
	router.Use(RequestSettingsCacheMiddleware)
	// Request-scoped identity memo cache (issue #2099). Router-wide so routes
	// outside ProtectedTenantGroup (notably /api/sse, which builds its own JWT
	// chain) dedupe their identity-chain lookups too.
	router.Use(RequestIdentityCacheMiddleware)
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
	return IsDemoEnvironment(viper.GetString("app_env"))
}

// setupRateLimiting configures rate limiting middleware if enabled
func setupRateLimiting(router chi.Router, securityLogger *customMiddleware.SecurityLogger, tokenAuth *sessiontoken.TokenAuth, exemptLoopback bool) {
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
	generalRateLimiter.SetKeyFunc(tokenAuth.RateLimitKey)
	if exemptLoopback {
		generalRateLimiter.ExemptLoopback()
	}
	if securityLogger != nil {
		generalRateLimiter.SetLogger(securityLogger)
	}
	router.Use(generalRateLimiter.Middleware())
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
func requestReviewDependencies(modules moduleServices, db transactionruntime.Database) (requestreviewcompose.ProjectionDependencies, carePlanModule.CareScheduleReviewQuery, error) {
	pool := db.Pool()
	svc := modules.services
	reviewGroups, err := schoolStructureCompose.New(schoolStructureCompose.Dependencies{DB: pool, Observe: func(observation schoolStructureCompose.Observation) {
		observability.ObserveSchoolStructureOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, schoolStructureModule.ErrorCode(observation.Err), observation.Err)
	}})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("request review groups: %w", err)
	}
	reviewStudents, err := requestreviewcompose.NewStudentDirectory(modules.persons, reviewGroups)
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("request review student directory: %w", err)
	}
	// The staff projection lists exactly what the decisions accept: one
	// Identity & Access review policy serves both (#3804).
	reviewScope := func(ctx context.Context) (carePlanCompose.ReviewScope, error) {
		schoolWide, groupIDs, err := modules.reviewPolicy.Scope(ctx, sessiontoken.PermissionsFromCtx(ctx))
		return carePlanCompose.ReviewScope{SchoolWide: schoolWide, GroupIDs: groupIDs}, err
	}
	// Report the union of ordinary and absence-review rights, as the
	// navigation capability does. Each native queue keeps its own scope.
	reviewAccess, err := requestreviewcompose.NewAccess(reviewStudentsHTTP.RequestReviewPrincipal, func(ctx context.Context) (string, error) {
		return modules.reviewPolicy.AccessLevel(ctx, sessiontoken.PermissionsFromCtx(ctx))
	})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("request review access: %w", err)
	}
	masterDataReviews, err := requestreviewcompose.NewMasterDataReviews(pool, studentDirectoryView.NewReader(), modules.persons,
		reviewScope, carePlanCompose.Today, func(observation requestreviewcompose.CareObservation) {
			observability.ObserveCarePlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.Conflicts, observation.Stats.StatementDuration, carePlanModule.ErrorCode(observation.Err), observation.Err)
		})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("master data review queue: %w", err)
	}
	careReviews, err := requestreviewcompose.NewScheduleReviews(pool, studentDirectoryView.NewReader(), requestreviewcompose.ScheduleReviewDependencies{
		People: modules.persons,
		Scope:  reviewScope,
		BookingsAuthoritative: func(ctx context.Context) (bool, error) {
			return svc.Settings.ResolveBool(ctx, reviewsettings.BookingsAuthoritative)
		},
		Today:  carePlanCompose.Today,
		Blocks: NewPickupReviewBlocks(db),
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
	offeringReviews, err := requestreviewcompose.NewOfferingReviews(pool, requestreviewcompose.OfferingReviewDependencies{
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
	correctionQueries, err := auditCompose.NewCorrectionQueries(pool, func(auditlog.Observation) {})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("request review corrections: %w", err)
	}
	corrections, err := requestreviewcompose.NewCorrectionLog(correctionQueries, modules.persons, func(ctx context.Context) bool {
		return reviewStudentsHTTP.RequestReviewCorrectionAccess(ctx, modules.userContext.HasCurrentStaff)
	})
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
		Devices:      svc.IoT.Fleet(),
		Schools:      deviceSchoolDirectory{schools: modules.organizations},
		SchoolErrors: devicefleetCompose.NewSchoolReadErrors(),
		Settings:     svc.Settings,
		FallbackPIN:  os.Getenv("OGS_DEVICE_PIN"),
	})
}

// mountStudentRoutes serves /api/students and hands its resource back as the
// Worker's student document sweep.
func mountStudentRoutes(tenant chi.Router, in routeInputs, deviceAuth *deviceauth.Authenticators, presence *studentpresence.Module) (schedulerPorts.StudentDocumentFileCleaner, error) {
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
	students, careStudents, projectionStudents, presenceStudents, deletionStudents, requestStudents := NewStudentHTTPResources(
		studentsAPI.ResourceConfig{
			ActiveEnrollments:      studentActiveEnrollments{enrollments: svc.Activities},
			AuthenticatedDevice:    deviceauth.DeviceID,
			ChildQuota:             childQuotaStudentsReader{usages: modules.membership},
			ClassListEntries:       classListEntryStudentsReader{entries: modules.membership},
			DeviceAuthenticator:    deviceAuth.Device(),
			FamilyProtection:       svc.PeopleDirectory,
			Logger:                 in.logger.With("handler", "students"),
			PeopleDirectory:        svc.PeopleDirectory,
			Persons:                peopleCompose.NewStudentRoutePersons(svc.Users),
			StudentAuditService:    svc.PeopleDirectory,
			StudentDocumentService: svc.StudentDocuments,
			StudentNotes:           svc.PeopleDirectory,
			StudentPhotos:          svc.PeopleDirectory,
			Uploads:                nativeUploads.UploadFiles{},
			UserContextService:     in.modules.userContext,
		},
		careStudentsHTTP.ResourceConfig{
			ArrivalScheduleService:       svc.ArrivalSchedule,
			Broadcaster:                  svc.RealtimeHub,
			CareLifecycleService:         svc.CareLifecycle,
			CareRequestReviews:           careReviews,
			CareRequestService:           svc.CareRequests,
			CompanionService:             modules.companions,
			DocumentStorage:              NewStudentDocumentStorage(),
			ExcusedRequestService:        svc.ExcusedRequests,
			LockTemplateRecurrence:       svc.TimetableData.RecurrenceLock.LockRecurrenceWrites,
			Logger:                       in.logger.With("handler", "students"),
			MasterDataReviewService:      svc.MasterDataReview,
			OfferingChangeService:        svc.EnrollmentCareOffering,
			OfferingPickupTimes:          svc.EnrollmentCareOffering,
			OfferingSourceResyncer:       studentClassResyncer,
			ParentEventEmitter:           svc.ParentEventEmitter,
			ParentRequestBulkService:     svc.ParentRequests,
			ParentRequestConflictService: svc.ParentRequests,
			PartialAbsenceService:        svc.PartialAbsence,
			PeopleDirectory:              svc.PeopleDirectory,
			Persons:                      peopleCompose.NewStudentRoutePersons(svc.Users),
			PickupAdjustmentService:      svc.EnrollmentCareOffering,
			PickupScheduleService:        svc.PickupSchedule,
			SettingsService:              svc.Settings,
			StudentAuditService:          svc.PeopleDirectory,
			StudentConsents:              svc.StudentConsents,
			StudentDocumentService:       svc.StudentDocuments,
			StudentPhotos:                svc.PeopleDirectory,
			UserContextService:           in.modules.userContext,
			WeekdayPickupNotes:           modules.carePlan,
		},
		projectionStudentsHTTP.ResourceConfig{
			AbsenceOverview:         svc.AbsenceOverview,
			ActiveEnrollments:       studentActiveEnrollments{enrollments: svc.Activities},
			ActiveService:           svc.Active,
			ArrivalScheduleService:  svc.ArrivalSchedule,
			CareDayService:          svc.CareDay,
			CareLifecycleService:    svc.CareLifecycle,
			ClassListEntries:        classListEntryStudentsReader{entries: modules.membership},
			CompanionService:        modules.companions,
			ExcusedRequestService:   svc.ExcusedRequests,
			InstanceService:         svc.Instance,
			ListExportService:       lists.NewRenderer(),
			Logger:                  in.logger.With("handler", "students"),
			PeopleDirectory:         svc.PeopleDirectory,
			Persons:                 peopleCompose.NewStudentRoutePersons(svc.Users),
			PickupScheduleService:   svc.PickupSchedule,
			SchoolGroups:            studentSchoolGroups{GroupManagement: svc.Education},
			SchoolService:           studentSchoolDirectory{schools: modules.organizations},
			SettingsService:         svc.Settings,
			StudentConsents:         svc.StudentConsents,
			StudentHistoryService:   svc.StudentHistory,
			StudentStatusDayService: modules.presence.StatusDays,
			UserContextService:      in.modules.userContext,
		},
		presenceStudentsHTTP.ResourceConfig{
			AbsenceNotifier:         studentAbsenceNotifier{notifier: svc.AbsenceNotifier},
			ActiveService:           svc.Active,
			Broadcaster:             svc.RealtimeHub,
			Logger:                  in.logger.With("handler", "students"),
			ParentEventEmitter:      svc.ParentEventEmitter,
			PeopleDirectory:         svc.PeopleDirectory,
			Persons:                 peopleCompose.NewStudentRoutePersons(svc.Users),
			PrivacyConsents:         presence,
			SettingsService:         svc.Settings,
			StudentStatusDayService: modules.presence.StatusDays,
			UserContextService:      in.modules.userContext,
		},
		deleteStudentsHTTP.ResourceConfig{
			CareLifecycleService: svc.CareLifecycle,
			ConstraintViolation:  transactionFailures.IsConstraintViolation,
			PeopleDirectory:      svc.PeopleDirectory,
			StudentDeletion:      svc.StudentDeletion,
			UserContextService:   in.modules.userContext,
		},
		reviewStudentsHTTP.ResourceConfig{
			PeopleDirectory:     svc.PeopleDirectory,
			RequestReview:       requestReview,
			RequestReviewAccess: modules.reviewPolicy,
		},
	)
	tenant.Mount("/students", students.Router(ProtectedTenantRoutesWith(
		groupLiveHTTP.NewResource(svc.OGSGroupLive).Register,
		careStudents.Register, projectionStudents.Register, presenceStudents.Register, deletionStudents.Register, requestStudents.Register,
		(&enrollmentAPI.StudentResource{EnrollmentDecision: svc.EnrollmentDecision, EnrollmentFormSchema: newEnrollmentFormSchemas(svc.Settings, in.logger), StudentReadAccess: students.StudentReadAccess, Logger: in.logger.With("handler", "students")}).Register,
	)))
	return careStudents, nil
}

// mountPresenceRoutes serves the attendance routes and the kiosk API. Both
// move children through one open-room move over the Student Presence owner.
func mountPresenceRoutes(tenant chi.Router, in routeInputs, deviceAuth *deviceauth.Authenticators, presence *studentpresence.Module) error {
	svc, logger := in.modules.services, in.logger
	openRoomMove, err := newOpenRoomMove(in.modules, svc.Active, logger)
	if err != nil {
		return err
	}
	operations, err := presenceservice.NewPresenceOperations(svc.Active, svc.OGSGroupLive, logger.With("service", "presence-operations"))
	if err != nil {
		return err
	}
	active := presenceAPI.NewResource(operations, activePeople{source: NewAttendanceRoutePeople(svc.Users)}, teacherGroupIDs(in.modules), activeStaffAccess{source: peopleCompose.NewAttendanceRouteStaff(in.modules.userContext)}, svc.Settings, ProtectedTenantRoutesWith(supervisionDashboardHTTP.NewResource(svc.SupervisionDashboard).Register), logger.With("handler", "active"), presence, activeRequestRuntime(), activeAuthorization(), settingsFacts.NewDisplaySettings(svc.Settings))
	iot, err := newIoTResource(in, deviceAuth, presence, openRoomMove)
	if err != nil {
		return err
	}
	move := &openRoomMoveHTTP.Resource{
		OpenRoomMove: openRoomMove, Authorize: active.BulkStudentMoveAuthorization,
		MarkRollback: activeRequestRuntime().MarkRollback, RenderPresenceError: presenceAPI.ErrorRenderer,
		WebAttendance: settingsFacts.NewDisplaySettings(svc.Settings).WebAttendanceEnabledReader(),
	}
	yard := supervisionDashboardHTTP.NewSchulhofResource(newSupervisionYard(svc.Schulhof), func(ctx context.Context) (int64, bool, error) {
		staff, err := active.UserContextService.GetCurrentStaff(ctx)
		if staff == nil {
			return 0, false, err
		}
		return staff.ID, true, err
	})
	tenant.Mount("/active", active.RouterWithOwnerRoutes(move.Register, yard.Register))
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
	errorReports, err := NewIoTErrorReportRelay(in.errorReportDSN)
	if err != nil {
		return nil, err
	}
	deviceScan := devicescanCompose.New(devicescanCompose.Dependencies{
		Fleet:      svc.IoT.Fleet(),
		Presence:   presence,
		Rooms:      modules.rooms,
		Active:     svc.Active,
		Users:      NewDeviceScanPeople(svc.Users),
		Activities: DeviceActivityCatalog(svc.Activities),
		Rosters:    modules.timetable,
		Education:  svc.Education,
		Pickups:    svc.PickupSchedule,
		// A destination chosen at the kiosk runs the phone's open-room
		// move (#3067).
		OpenRooms: newDeviceOpenRoomMover(openRoomMove),
		Settings:  newDeviceScanSettings(svc.Settings),
		Logger:    logger.With("service", "device-scan"),
	})
	return iotAPI.NewResource(iotAPI.ServiceDependencies{
		Administration:   NewIoTAdministration(devicefleetCompose.NewAdministration(svc.IoT.Fleet())),
		Runtime:          NewIoTHTTPRuntime(),
		DeviceScan:       deviceScan,
		OpenRooms:        deviceScan,
		StaffClock:       newStaffClockService(svc.Users, NewPeopleIdentityAccess(in.db, observability.ObserveIdentityAccessOperation), svc.WorkSession.TimeClock),
		Configuration:    devicescanCompose.NewConfiguration(DeviceConfiguration{Settings: svc.Settings}),
		Rooms:            devicescanCompose.NewRoomAvailability(modules.presence.RoomService),
		Directory:        devicescanCompose.NewDirectory(NewDeviceScanPeople(svc.Users), DeviceActivityCatalog(svc.Activities), logger),
		TagAssignments:   devicescanCompose.NewTagAssignments(NewDeviceScanPeople(svc.Users), logger),
		FeedbackStudents: devicescanCompose.NewFeedbackStudents(NewDeviceScanPeople(svc.Users)),
		FeedbackService:  modules.feedback,
		FeedbackResponseObserver: func(status int, code string) {
			observability.ObserveFeedbackHTTPResponse("iot", status, code)
		},
		SchoolName:              devicescanCompose.NewSchoolName(schoolName(modules.organizations)),
		ErrorReports:            NewIoTErrorReporter(errorReports),
		SessionEnd:              bindIoTHostSessionEnd(BindIoTSessionEnd(sessionEnd)),
		SessionLifecycle:        devicescanCompose.NewSessionLifecycle(svc.Active, devicescanCompose.NewSupervisionQuery(presence), NewDeviceScanPeople(svc.Users), svc.IoT, NewKioskSessionMirror(modules.timetableRepositories, svc.Activities, KioskMirrorPublisher(svc.RealtimeHub, logger), logger), logger),
		Logger:                  logger.With("handler", "iot"),
		DeviceAuthenticator:     deviceAuth.Device(),
		DeviceOnlyAuthenticator: deviceAuth.DeviceOnly(),
	}), nil
}

// mountStaffRoutes serves the staff administration, shift planning,
// substitutions and time tracking, and hands the staff administration back
// as the Worker's staff document sweep.
func mountStaffRoutes(tenant chi.Router, in routeInputs) (schedulerPorts.StaffDocumentFileCleaner, error) {
	svc, workforce := in.modules.services, in.modules.workforce
	staff, staffAdmin, err := newStaffComposition(in.modules.membership, workforce, svc, in.modules.identity, in.db, in.logger.With("handler", "staff"))
	if err != nil {
		return nil, err
	}
	staffShifts := newStaffShiftsResource(staffShiftsDependencies{
		Planning:       svc.StaffShifts,
		ResolveStaffID: currentStaffID(in.modules),
		ActorAccountID: sessiontoken.ActorAccountIDFromCtx,
	})
	absenceTypes := newAbsenceTypesResource(workforceCompose.NewAbsenceTypeAdministration(workforce, in.logger.With("service", "active")), currentStaffID(in.modules)).Router()
	tenant.Mount("/staff", staff.Router())
	tenant.Mount("/work-time-models", newWorkTimeModelsResource(workforce, staffTimeTrackingNotifier(svc.RealtimeHub)).Router())
	tenant.Mount("/staff-shifts", staffShifts.Router())
	tenant.Mount("/shift-types", newShiftTypesResource(svc.ShiftTypes).Router())
	tenant.Mount("/absence-types", absenceTypes)
	tenant.Mount("/substitutions", newSubstitutionsResource(SubstitutionCapability(svc.Substitution)).Router())
	tenant.Mount("/time-tracking", newTimeTrackingResource(svc, in.modules.calendar).Router(ProtectedTenantRoutes))
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
	pickupExtensions, err := careexit.NewPickupExtensions(in.modules.timetable, presence)
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
	staffNotices := timetableAPI.NewStaffNoticeResource(timetableCompose.NewStaffNotices(timetableCompose.StaffNoticeDependencies{
		DB: in.db.Pool(), Periods: timetableCompose.NewCalendarPeriodRepository(in.modules.calendar, newCalendarPeriodUsage(in.modules.timetable)), Names: staffNoticeNames(svc.PeopleDirectory),
		Logger: in.logger.With("service", "staffnotice"),
	}), func(ctx context.Context) int64 { return timeTrackingIdentity(ctx).AccountID })
	// Notification abstraction (#1624).
	notifications := NewNotificationHTTP(svc.Notifications, svc.PushSubscriptions, svc.NotificationPreferences)
	// The class-day surface reads the class-day projection (#2701): the day
	// report over Enrollment's day roster and the arrival-exception write
	// seam (#2970) behind its one public capability.
	classDay := classdayHTTP.NewResource(svc.ClassDayArrivalExceptions, in.logger.With("handler", "class-day"))
	school := NewSchoolPortalHTTP(SchoolPortalAuth(in.modules.identity), SchoolPortalMFA(in.modules.identity), schoolPasswordResets(in.modules.identity), classDay, timetable, staffMessaging, staffNotices, notifications)
	tenant.Mount("/staff-messages", staffMessaging.Router(ProtectedTenantRoutes))
	tenant.Mount("/staff-notices", staffNotices.Router(ProtectedTenantRoutes))
	tenant.Mount("/timetable", timetable.Router(ProtectedTenantRoutesWith(classdayHTTP.NewSlotListResource(svc.SlotLists).Register)))
	tenant.Mount("/notifications", notifications.Router())
	root.Mount("/school", school.RouterWithAuthRateLimiter(limiters.authMiddleware(), SchoolSessionChain, ProtectedSchoolRoutes))
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
		CalendarPeriodUsage:     calendarPeriodUsage{usage: newCalendarPeriodUsage(modules.timetable)},
		MaterializationService:  svc.Materialization,
		InstanceService:         svc.Instance,
		InstanceSeriesConverter: svc.InstanceSeriesConverter,
		OperationsService:       svc.TimetableOperations,
		People:                  NewTimetablePeople(svc.Users),
		Templates:               svc.TimetableData.Templates,
		RecurrenceLock:          svc.TimetableData.RecurrenceLock,
		AttendanceCorrections:   svc.TimetableData.AttendanceCorrections,
		Deviations:              svc.TimetableData.Deviations,
		TimetableData:           svc.TimetableData.Data,
		ConflictDetection:       svc.TimetableData.ConflictDetection,
		PlanningTracks:          modules.planningTracks,
		CareDayService:          svc.CareDay,
		UserContextService:      modules.userContext,
		SettingsService:         svc.Settings,
		OfferingSourceOptions:   NewTimetableOfferingSources(svc.EnrollmentCareOffering),
		SupervisionSheets:       NewTimetableSupervisionSheets(svc.ClassDayArrivalExceptions),
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
	svc, modules, pool, logger := in.modules.services, in.modules, in.db.Pool(), in.logger
	tenant.Mount("/rooms", roomsHTTPAdapter.NewResource(modules.rooms, roomsHTTPAdapter.Dependencies{
		Facilities:          modules.presence.RoomService,
		RoomHistorySettings: NewRoomHistorySettings(svc.Settings, logger.With("handler", "rooms"), settingsFacts.PrefetchSettings),
		UserContext:         modules.userContext, Active: svc.Active,
		Users: peopleCompose.NewRoomSnapshotPeople(svc.Users), Education: svc.Education,
		ListExport: modules.listExport,
	}, logger.With("handler", "rooms"), ProtectedTenantRoutes, transactionFailures.IsConstraintViolation).Router())
	tenant.Mount("/groups", groupsHTTP.NewResource(svc.Education, svc.Active, peopleCompose.NewGroupRoutePeople(svc.Users), modules.userContext, transactionFailures.IsConstraintViolation).Router(ProtectedTenantRoutes))
	tenant.Mount("/activities", timetableHTTPAdapter.NewResource(svc.Activities, modules.timetable, svc.Users, modules.userContext, pool).Router(ProtectedTenantRoutes))
	// Parent enrollment (PR 5+).
	tenant.Mount("/enrollment", newEnrollmentResource(modules, logger).Router(TenantSessionChain))
	// Info-point displays (#1325). One Device Fleet owner serves every entry
	// point: the service graph composes it and the IoT service hands it
	// back here (#2676).
	tenant.Mount("/display", NewDisplayHTTPRouter(svc.IoT.Fleet(), DisplayFeatureEnabled(svc.Settings)))
	// Dateframes belong to the School Calendar, timeframes and recurrence
	// rules to the Timetable owner; timeframe changes stay guarded by the
	// recurrence gate and the Care Plan catalog's care-offering check.
	tenant.Mount("/schedules", timetableHTTPAdapter.NewSchedulesResource(modules.calendar, modules.timetable, TimeframeChangeGuard(
		svc.TimetableData.RecurrenceLock.LockRecurrenceWrites,
		svc.EnrollmentCareOffering.ValidateTimeframeChange,
	), pool).Router(ProtectedTenantRoutes))
	// The schema-driven settings system.
	homeLayouts := requireHomeLayoutOperations(svc.Settings)
	tenant.Mount("/settings", newSettingsResource(svc.TenantSettings, homeLayouts, NewEnrollmentBookings().SchemaReferencesLegalDocument).SettingsRouter())
	// Class-list-only entries (#2382).
	tenant.Mount("/class-list-entries", newClassListEntriesResource(modules.membership, logger.With("handler", "class-list-entries")).Router())
	tenant.Mount("/database", databaseStatsRouter(in))
	// CSV/Excel import endpoints.
	tenant.Mount("/import", importAPI.NewResource(importAPI.Dependencies{
		Students: modules.imports.Student, Staff: modules.imports.Staff, ClassList: modules.imports.ClassList,
		Files: dataImportCompose.NewFileDecoder(), Runtime: dataImportHTTPRuntime(svc.PeopleDirectory, modules.membership, modules.imports.OpeningBalance),
	}).Router())
	tenant.Mount("/admin/grade-transitions", gradeTransitionHTTP.NewGradeTransitionResource(svc.GradeTransition).Router(ProtectedTenantRoutes))
}

// newEnrollmentResource mounts Enrollment's staff and public routes. The
// root builds the form schemas, the captcha and the phase expiry warnings
// here, over the settings and the owners it composed (#4012).
func newEnrollmentResource(modules moduleServices, logger *slog.Logger) *enrollmentAPI.Resource {
	svc := modules.services
	resource := enrollmentAPI.NewResource(
		newEnrollmentFormSchemas(svc.Settings, logger),
		enrollmentAPI.NewCareOfferingCatalog(NewEnrollmentCareOfferingValues(svc.EnrollmentCareOffering)),
		enrollmentAPI.NewRequestService(svc.EnrollmentRequest),
		newEnrollmentCaptcha(svc.Settings, logger),
		svc.EnrollmentPhase,
		enrollmentAPI.NewDecisionService(svc.EnrollmentDecision),
		svc.EnrollmentReport,
		enrollmentAPI.NewRolloverService(svc.EnrollmentRollover),
		enrollmentAPI.NewChangeRequestService(svc.EnrollmentChangeRequest),
		svc.EnrollmentDeletion,
		enrollmentGuardianInvitations(modules.identity),
		NewEnrollmentGuardianAutofill(modules.guardianProfiles),
		enrollmentSchoolDirectory{schools: modules.organizations},
		nativeUploads.UploadFiles{},
		enrollmentAPI.DatabaseFailures{Transient: transactionFailures.IsTransientDatabaseError, Constraint: transactionFailures.IsConstraintViolation},
		NewEnrollmentBookings(),
	)
	resource.ListExportService = modules.listExport
	resource.PhaseExpiryService = newEnrollmentPhaseExpiry(modules.persons, modules.carePlan)
	return resource
}

// mountPeopleRoutes serves the people directory, guardians, statistics,
// birthdays, the caller's own context, the emergency snapshot, feedback and
// the meal plan under /api.
func mountPeopleRoutes(tenant chi.Router, in routeInputs) {
	svc, modules, logger := in.modules.services, in.modules, in.logger
	tenant.Mount("/statistics", statisticsAPI.NewResource(svc.Statistics, statisticsHTTPRuntime(modules.listExport), logger.With("handler", "statistics")).Router(ProtectedTenantRoutes))
	tenant.Mount("/guardians", newGuardiansResource(svc.PeopleDirectory, NewGuardianDirectoryRuntime(svc, modules.listExport, modules.identity, modules.userContext.HasCurrentStaff, peopleCompose.NewLegacyRepositoryRuntime(in.db.Pool())), viper.GetString("app_env"), logger.With("handler", "guardians")).Router())
	tenant.Mount("/feedback", newFeedbackResource(modules.feedback).Router())
	tenant.Mount("/meal-plan", newMealPlanResource(modules.mealPlan, newMealPlanExportRenderer()).Router())
	tenant.Mount("/users", newUsersResource(modules.persons, modules.identity.ListAccountEmails, func(ctx context.Context, tagID string) (bool, error) {
		_, _, found, err := modules.identity.LookupRFIDCard(ctx, tagID)
		return found, err
	}).Router())
	// Birthday display and staff birthday list (#1542).
	tenant.Mount("/birthdays", birthdaysAPI.NewResource(modules.birthdays, modules.listExport, modules.userContext, logger.With("handler", "birthdays")).Router(ProtectedTenantRoutes))
	tenant.Mount("/me", meAPI.NewResource(modules.userContext.Caller(), modules.userContext, TenantSessionChain, nativeUploads.UploadFiles{}).Router())
	tenant.Mount("/emergency", emergencyAPI.NewResource(svc.Emergency).Router(ProtectedTenantRoutes))
}

// mountCommunicationRoutes serves parent messages and announcements, staff
// reminders, the PWA usage report and the platform announcements under /api.
func mountCommunicationRoutes(tenant chi.Router, in routeInputs) {
	svc := in.modules.services
	tenant.Mount("/messages", messagingAPI.NewResource(svc.Messaging).Router(ProtectedTenantRoutes))
	tenant.Mount("/parent-announcements", announcementAPI.NewResource(svc.ParentAnnouncement, newDeclarationReports()).Router(ProtectedTenantRoutes))
	// Visual-only staff reminders (#1457).
	tenant.Mount("/reminders", remindersHTTP.NewResource(svc.Reminders, ReminderHTTPRuntime()).Router())
	// PWA standalone-usage reporting (#2189).
	tenant.Mount("/pwa", pwaUsageRouter(svc.PWAUsage))
	// User-facing platform announcements.
	tenant.Mount("/platform", platformAnnouncementsHTTP.NewResource(platformAnnouncementsHTTP.ResourceConfig{
		AnnouncementsService: in.modules.communication,
		Runtime:              newPlatformRuntime(),
	}).Router())
}

// mountCalendarRoutes serves the personal calendars under /api, the
// read-only staff CalDAV and the public subscription feeds.
func mountCalendarRoutes(root, tenant chi.Router, in routeInputs) {
	svc := in.modules.services
	calendar := calendarAPI.NewResource(svc.Calendar, in.logger.With("handler", "calendar"))
	tenant.Mount("/calendar", calendar.Router(TenantSessionChain))

	// Public parent calendar subscription feed (no auth — the token in the URL
	// is the capability). Calendar apps (Apple/Google/Outlook) poll this to keep
	// the parent's Termine in sync.
	root.Get("/public/calendar/{token}", calendarAPI.PublicFeed(in.modules.services.Calendar))

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
	sse := sseAPI.NewResource(svc.RealtimeHub, in.modules.userContext, NewSSEHTTPRuntime(), in.logger.With("handler", "sse"))
	sse.SetSchoolAccess(in.modules.identity)
	tenant.Mount("/files", files.Router(ProtectedTenantRoutes))
	// Anhänge an Elternmitteilungen (#2890). Eigener Pfad statt einer
	// Route unter /parent-announcements: die Bytes gehören der
	// Dateiablage, die Mitteilung steuert nur den Empfängerkreis bei.
	tenant.Mount("/announcement-attachments", files.AnnouncementAttachmentRouter(ProtectedTenantRoutes))
	tenant.Mount("/sse", NewSSEHTTPRouter(sse))
	// Parent-portal SSE stream: only whitelisted triggers (parent_message)
	// for the tenants of the guardian's children.
	root.Mount("/parent-sse", newParentSSERouter(sse))
	// Anhänge, die Eltern zu einer Mitteilung herunterladen (#2890). Der
	// Empfängerkreis der Mitteilung entscheidet; wer nicht dazugehört,
	// bekommt 404.
	root.Mount("/parent-news-attachments", files.ParentAnnouncementAttachmentRouter(ProtectedParentGroup))
	// School-portal SSE stream (#2208): account-addressed triggers only
	// (Team-Chat).
	root.Mount("/school-sse", newSchoolSSERouter(sse))
}

// mountPortalRoutes mounts the root-level session portals: tenant auth,
// operator dashboard (separate from the tenant API) and the cross-tenant
// guardian portal. The auth rate limiter guards each login.
func mountPortalRoutes(root chi.Router, in routeInputs, limiters authRateLimiters) error {
	// RouterWithAuthRateLimiter applies the limiter only to the public login,
	// password-reset, MFA and passkey-login routes.
	root.Mount("/auth", newAccountResource(in).RouterWithAuthRateLimiter(limiters.authMiddleware(), TenantSessionChain))
	operator, err := newOperatorRouter(in, limiters)
	if err != nil {
		return err
	}
	root.Mount("/operator", operator)
	root.Mount("/parent", newParentResource(in).RouterWithAuthRateLimiter(limiters.authMiddleware(), ParentSessionChain))
	return nil
}

// caregiverCapabilityViews binds People Directory's caregiver capability for
// the account and operator routes that toggle it (#2736); in.db opens the
// administrative transaction of the operator routes.
func caregiverCapabilityViews(in routeInputs) peopleCompose.CaregiverCapabilityViews {
	return peopleCompose.NewCaregiverCapabilityViews(in.modules.services.CaregiverCapability, in.db.Pool())
}

// newAccountResource composes the tenant account routes. Their caregiver
// capability is the one the operator routes provision with.
func newAccountResource(in routeInputs) *authAPI.Resource {
	svc, identity := in.modules.services, in.modules.identity
	authSchools := authSchoolDirectory{schools: in.modules.organizations, memberships: identity.ListActiveAccountSchoolIDs}
	resource := authAPI.NewResource(identity, identity, authSchools, identity, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation)
	resource.CaregiverCapabilityService = caregiverCapabilityViews(in)
	resource.SettingsService = svc.Settings
	resource.RoleGrants = RoleGrantPolicy{}
	resource.MFAService = identity
	resource.PasskeyService = identity
	return resource
}

// newOperatorRouter composes the operator dashboard. The Device Fleet review
// of unregistered RFID scans answers in the operator surface's format (#3232).
func newOperatorRouter(in routeInputs, limiters authRateLimiters) (chi.Router, error) {
	svc := in.modules.services
	tagScanReview := tagScanOperatorAPI.NewResource(tagScanOperatorAPI.Config{
		Scans:     svc.IoT.Fleet(),
		Directory: tagScanSchoolDirectory{schools: in.modules.organizations},
		Surface: tagScanOperatorAPI.Surface{
			InvalidRequest: apiCommon.OperatorInvalidRequest,
			Internal:       apiCommon.OperatorInternal,
			RenderError:    apiCommon.RenderError,
			Respond:        apiCommon.Respond,
			OperatorID:     operatorIDFromSession,
		},
	})
	billing, err := newOperatorBilling(in.logger)
	if err != nil {
		return nil, fmt.Errorf("compose operator billing: %w", err)
	}
	schoolSettings := settingsCompose.NewOperatorSchoolSettings(settingsCompose.OperatorDependencies{
		Settings:       svc.Settings,
		DB:             in.db.Pool(),
		Notify:         settingsChangedNotifier(svc.RealtimeHub),
		OpenAttendance: OpenAttendanceChecker{Service: svc.Active},
		// Mirror the tenant-side OnValueSet hook so operator writes also
		// trigger side effects (e.g. auto-creating the Schulhof/WC rooms when
		// the corresponding checkout toggle flips on).
		OnValueSet: svc.SettingsSideEffects.Dispatch,
	})
	identity := operatorIdentity{
		sessions:    identityOperatorAPI.NewSessions(in.sessionAuth, in.modules.identity),
		account:     identityOperatorAPI.NewResource(in.modules.identity, operatorIdentityResponses()),
		access:      in.modules.identity,
		passkeys:    in.modules.identity,
		mfa:         in.modules.identity,
		invitations: in.modules.identity,
		accountMFA:  in.modules.identity,
	}
	provisioning := provisioningOperatorAPI.ProvisioningConfig{
		Service:             svc.OperatorProvisioning,
		CaregiverCapability: caregiverCapabilityViews(in),
		AppEnv:              viper.GetString("app_env"),
	}
	isLocalSeedRequest := func(r *http.Request) bool {
		return apiCommon.IsLocalSeedRequest(r, viper.GetString("app_env"))
	}
	return operatorRouter(
		identity,
		provisioning,
		provisioningOperatorAPI.NewBillingResource(billing, nil, isLocalSeedRequest),
		&operatorSchoolSettings{
			values: settingsOperatorAPI.NewSettingsResource(settingsOperatorAPI.SettingsConfig{
				Settings:   schoolSettings,
				Schools:    operatorSettingsSchools{schools: in.modules.organizations},
				OperatorID: operatorIDFromSession,
			}),
			bookingAuthorityImpact: newBookingAuthorityImpact(svc.CareLifecycle),
		},
		in.modules.communication,
		tagScanReview.Router(),
		limiters,
	), nil
}

// newParentResource composes the cross-tenant guardian portal: public
// /parent/auth/login and the protected /parent/* routes.
func newParentResource(in routeInputs) *parentAPI.Resource {
	svc := in.modules.services
	return parentAPI.NewResource(parentAPI.ResourceConfig{
		Auth:                  parentPortalLogin(in.modules.identity),
		Resets:                parentPasswordResets(in.modules.identity),
		Parent:                svc.Parent,
		Calendar:              svc.Calendar,
		Enrollment:            parentEnrollmentForms(enrollmentAPI.NewRequestService(svc.EnrollmentRequest)),
		GuardianProfileLoader: newParentGuardianProfiles(in.modules.guardianProfiles),
		Schools:               parentSchoolDirectory{schools: in.modules.organizations},
		Push:                  bindParentPushSubscriptions(svc.PushSubscriptions),
		Preferences:           bindParentPreferences(svc.NotificationPreferences),
		PWAUsage:              svc.PWAUsage,
		Reports:               newDeclarationReports(),
		SupportedLocale:       bindParentLocale(svc.NotificationPreferences),
	})
}

func newPlatformRuntime() platformAnnouncementsHTTP.Runtime {
	return platformAnnouncementsHTTP.Runtime{
		// TenantMiddleware supplies the scope guard: parent- and school-scope
		// tokens are rejected with 401 (#2207) — before it, this group was the
		// only /api mount without a scope check, so a portal-bound token could
		// read tenant announcements here. Tenant, org, and platform tokens pass
		// unchanged.
		Protected: func(router chi.Router, register func(chi.Router)) {
			router.Group(func(r chi.Router) {
				r.Use(sessiontoken.Authenticator)
				r.Use(ReadOnlyPreviewMiddleware)
				r.Use(TenantScopeMiddleware)
				r.Use(SecurityPrincipalMiddleware)
				register(r)
			})
		},
		Viewer: func(r *http.Request) platformAnnouncementsHTTP.Viewer {
			claims := sessiontoken.ClaimsFromCtx(r.Context())
			return platformAnnouncementsHTTP.Viewer{AccountID: int64(claims.ID), Roles: claims.Roles, TenantID: claims.TenantID, OrgID: claims.OrgID}
		},
		IDParam: apiCommon.ParseInt64IDWithError,
		Success: apiCommon.Respond,
		Failure: func(w http.ResponseWriter, r *http.Request, failure platformAnnouncementsHTTP.Failure) {
			apiCommon.RenderError(w, r, apiCommon.ErrorInternalServerWrap(failure.Message, failure.Err))
		},
	}
}

func requireHomeLayoutOperations(settings any) settingsHTTP.HomeLayoutOperations {
	homeLayouts, ok := settings.(settingsHTTP.HomeLayoutOperations)
	if !ok {
		panic("settings platform does not provide home layout operations")
	}
	return homeLayouts
}

// currentStaffID resolves the caller's staff ID at request time.
func currentStaffID(modules moduleServices) func(context.Context) (int64, error) {
	return func(ctx context.Context) (int64, error) {
		return modules.userContext.Caller().StaffID(ctx)
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
		apiCommon.ServeImage(w, r, "public/uploads/login-images", filename, "public, max-age=86400", nativeUploads.UploadFiles{})
	})

	// Public enrollment legal document serving (no auth - parents read it before submitting).
	root.Get("/public/enrollment-legal-documents/{filename}", func(w http.ResponseWriter, r *http.Request) {
		filename := chi.URLParam(r, "filename")
		apiCommon.ServeFile(w, r, "public/uploads/enrollment-legal-documents", filename, "public, max-age=86400", nativeUploads.UploadFiles{})
	})
	root.Get("/public/enrollment-form-legal-documents/{filename}", func(w http.ResponseWriter, r *http.Request) {
		filename := chi.URLParam(r, "filename")
		apiCommon.ServeFile(w, r, "public/uploads/enrollment-form-legal-documents", filename, "public, max-age=86400", nativeUploads.UploadFiles{})
	})

	root.With(metricsAuthMiddleware(metricsToken)).Handle("/internal/metrics", metricsHandler())
}

func databaseStatsRouter(in routeInputs) chi.Router {
	return newDatabaseStatsRouter(in.modules.databaseStats, in.logger.With("handler", "database"))
}

func newDatabaseStatsRouter(read DatabaseStatsReader, logger *slog.Logger) chi.Router {
	router := chi.NewRouter()
	ProtectedTenantRoutes(router, func(router chi.Router, withTx apiCommon.Middleware) {
		// The reader redacts every count the caller may not read, so the
		// route opens for any permission authorize.NewDatabaseStatsCapabilities
		// honours: the Datenverwaltung hub shows its counts to a lead role a
		// school defines itself, not only to system administrators (#3469).
		router.With(apiCommon.RequiresAnyPermission(DatabaseStatsPermissions()...), withTx).
			Get("/stats", func(w http.ResponseWriter, r *http.Request) { serveDatabaseStats(w, r, read, logger) })
	})
	return router
}

func serveDatabaseStats(w http.ResponseWriter, r *http.Request, read DatabaseStatsReader, logger *slog.Logger) {
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

// newMealPlanExportRenderer keeps the export adapter construction in the API
// composition layer so both production and route tests receive a real renderer.
func newMealPlanExportRenderer() SimpleListRenderer {
	return NewSimpleListRenderer()
}

// newSessionTokenAuth hands the signing configuration to Identity's
// composition, which builds the HTTP session runtime (ADR 0053 point 6).
func newSessionTokenAuth() (*sessiontoken.TokenAuth, error) {
	return NewConfiguredSessionAuth(
		viper.GetString("auth_jwt_secret"),
		viper.GetDuration("auth_jwt_expiry"),
		viper.GetDuration("auth_jwt_refresh_expiry"),
	)
}
