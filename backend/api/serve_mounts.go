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

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

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
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	announcementAPI "github.com/moto-nrw/project-phoenix/modules/communication/http/parentannouncements"
	messagingAPI "github.com/moto-nrw/project-phoenix/modules/communication/http/parentmessages"
	platformAnnouncementsHTTP "github.com/moto-nrw/project-phoenix/modules/communication/http/platformannouncements"
	staffMessagingAPI "github.com/moto-nrw/project-phoenix/modules/communication/http/staffmessages"
	dataImportCompose "github.com/moto-nrw/project-phoenix/modules/dataimport/compose"
	importAPI "github.com/moto-nrw/project-phoenix/modules/dataimport/http"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	sseAPI "github.com/moto-nrw/project-phoenix/modules/delivery/http/sse"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"
	tagScanOperatorAPI "github.com/moto-nrw/project-phoenix/modules/devicefleet/inbound/operator"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/emergencysnapshot"
	emergencyAPI "github.com/moto-nrw/project-phoenix/modules/emergencysnapshot/http"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	roomsHTTPAdapter "github.com/moto-nrw/project-phoenix/modules/facilities/http"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	filestoreAPI "github.com/moto-nrw/project-phoenix/modules/filestorage/http/files"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	groupLiveHTTP "github.com/moto-nrw/project-phoenix/modules/grouplive/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	meAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/me"
	identityOperatorAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	provisioningOperatorAPI "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	birthdaysAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/birthdays"
	studentsAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	projectionStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/presenceprojection/http"
	requestreviewcompose "github.com/moto-nrw/project-phoenix/modules/requestreview/compose"
	reviewStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/requestreview/http"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	calendarAPI "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/http"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	schoolSetupCompose "github.com/moto-nrw/project-phoenix/modules/schoolsetup/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
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
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	supervisionDashboardHTTP "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/http"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetableAPI "github.com/moto-nrw/project-phoenix/modules/timetable/http"
	timetableHTTPAdapter "github.com/moto-nrw/project-phoenix/modules/timetable/http/activities"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/workflows/careexit"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	gradeTransitionHTTP "github.com/moto-nrw/project-phoenix/workflows/gradetransition/http"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove"
	openRoomMoveHTTP "github.com/moto-nrw/project-phoenix/workflows/openroommove/http"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentAPI "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	remindersHTTP "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/http"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
	deleteStudentsHTTP "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/http"
	slogchi "github.com/samber/slog-chi"
	"github.com/spf13/viper"
)

// mountRequestFeed serves the change-request feed: the public feed behind its
// token and the staff's feed administration under /api.
func mountRequestFeed(root, tenant chi.Router, db transactionruntime.Database, logger *slog.Logger, frontendURL string) error {
	requestFeed, err := requestFeedCompose.New(requestFeedCompose.Dependencies{
		DB: db.Pool(), Feed: newParentRequestFeed, FrontendURL: frontendURL, Now: time.Now,
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
		Logger:           logger.With("handler", "request-feed"),
	})
	root.Mount("/public/request-feed", resource.PublicRouter())
	tenant.Mount("/students/change-requests/rss-feed", resource.TenantRouter())
	return nil
}

// mountModuleRoutes mounts the route groups modules serve themselves.
func mountModuleRoutes(tenant chi.Router, settings settingsCompose.SettingsService) error {
	schoolSetup, err := newSchoolSetupRoute(schoolSetupCompose.Dependencies{
		Settings: SchoolSetupSettings{Settings: settings},
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
func requestReviewDependencies(
	db transactionruntime.Database, persons peopledirectory.Capability, reviewPolicy *ParentRequestReviewPolicy, caller *CallerRows,
	settings settingsCompose.SettingsService, excused *carePlanCompose.ExcusedAbsenceRequests,
) (requestreviewcompose.ProjectionDependencies, carePlanModule.CareScheduleReviewQuery, error) {
	pool := db.Pool()
	reviewGroups, err := schoolStructureCompose.New(schoolStructureCompose.Dependencies{DB: pool, Observe: func(observation schoolStructureCompose.Observation) {
		observability.ObserveSchoolStructureOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, schoolstructure.ErrorCode(observation.Err), observation.Err)
	}})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("request review groups: %w", err)
	}
	reviewStudents, err := requestreviewcompose.NewStudentDirectory(persons, reviewGroups)
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
	reviewAccess, err := requestreviewcompose.NewAccess(reviewStudentsHTTP.RequestReviewPrincipal, func(ctx context.Context) (string, error) {
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
	careQueue, err := requestreviewcompose.NewCareScheduleQueue(careReviews, carePlanModule.Today)
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
	offeringQueue, err := requestreviewcompose.NewOfferingQueue(offeringReviews, carePlanModule.Today)
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("offering review queue: %w", err)
	}
	correctionQueries, err := auditCompose.NewCorrectionQueries(pool, func(auditlog.Observation) {})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("request review corrections: %w", err)
	}
	corrections, err := requestreviewcompose.NewCorrectionLog(correctionQueries, persons, func(ctx context.Context) bool {
		return reviewStudentsHTTP.RequestReviewCorrectionAccess(ctx, caller.HasCurrentStaff)
	})
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("direct correction history: %w", err)
	}
	masterQueue, err := requestreviewcompose.NewMasterDataQueue(masterDataReviews)
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("master data review queue: %w", err)
	}
	excusedQueue, err := requestreviewcompose.NewExcusedQueue(excused, carePlanModule.Today)
	if err != nil {
		return requestreviewcompose.ProjectionDependencies{}, nil, fmt.Errorf("excused review queue: %w", err)
	}
	return requestreviewcompose.ProjectionDependencies{
		Queues:   requestreviewcompose.Queues{DirectCorrections: corrections, MasterData: masterQueue, CareSchedule: careQueue, Offering: offeringQueue, Excused: excusedQueue},
		Students: reviewStudents, FamilyProtection: requestreviewcompose.NewFamilyProtection(persons), Access: reviewAccess,
	}, careReviews, nil
}

func newDeviceAuthentication(iot devicefleetCompose.IoTService, organizations organizationtenancy.Query, settings settingsCompose.SettingsService) *deviceauth.Authenticators {
	return deviceauth.New(deviceauth.Dependencies{
		Devices:      iot.Fleet(),
		Schools:      deviceSchoolDirectory{schools: organizations},
		SchoolErrors: devicefleetCompose.NewSchoolReadErrors(),
		Settings:     settings,
		FallbackPIN:  os.Getenv("OGS_DEVICE_PIN"),
	})
}

// mountStudentRoutes serves /api/students.
func mountStudentRoutes(
	tenant chi.Router, db transactionruntime.Database, logger *slog.Logger, deviceAuth *deviceauth.Authenticators, presence *studentpresence.Module,
	persons peopledirectory.Capability, services PeopleServices, membership *schoolMembershipModule.Module, caller *CallerRows,
	reviewPolicy *ParentRequestReviewPolicy, settings settingsCompose.SettingsService, hub *realtime.Hub, carePlan carePlanModule.Capability,
	companions carePlanModule.StudentCompanions, schedules CarePlanSchedules, requests CarePlanRequests, planning TimetablePlanning,
	emitter eventContract.Emitter, activities activityContract.ActivityService, lifecycle TimetableLifecycle,
	reports PresenceReports, structureViews StructureServices, organizations organizationtenancy.Query,
	statusDays studentpresence.StatusDays, absences deliveryCompose.AbsenceNotifier, deletion *studentdeletion.Workflow,
	groupLive grouplive.Query, decisions enrollmentOwner.Decisions, active studentpresence.Presence,
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
			ArrivalScheduleService:       schedules.Arrivals,
			Broadcaster:                  hub,
			CareLifecycleService:         schedules.Lifecycle,
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
			AbsenceOverview:         reports.AbsenceOverview,
			ActiveEnrollments:       studentActiveEnrollments{enrollments: activities},
			ActiveService:           active,
			ArrivalScheduleService:  schedules.Arrivals,
			CareDayService:          schedules.CareDay,
			CareLifecycleService:    schedules.Lifecycle,
			ClassListEntries:        classListEntryStudentsReader{entries: membership},
			CompanionService:        companions,
			ExcusedRequestService:   requests.Excused,
			InstanceService:         lifecycle.Instances,
			ListExportService:       lists.NewRenderer(),
			Logger:                  logger.With("handler", "students"),
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

// mountPresenceRoutes serves the attendance routes and the kiosk API. Both
// move children through one open-room move over the Student Presence owner.
func mountPresenceRoutes(
	tenant chi.Router, db transactionruntime.Database, logger *slog.Logger, errorReportDSN string,
	deviceAuth *deviceauth.Authenticators, presence *studentpresence.Module, active studentpresence.Presence,
	facilities *facilitiesModule.Module, timetableOwner timetableModule.Capability, timetableRepos timetableCompose.Repositories,
	days TimetableDays, persons peopledirectory.Capability, users peoplePorts.PersonDirectory, caller *CallerRows,
	settings settingsCompose.SettingsService, hub *realtime.Hub, emitter eventContract.Emitter,
	groupLive grouplive.Query, dashboard supervisiondashboard.Query, yard FacilitiesYard, iot devicefleetCompose.IoTService,
	activities activityContract.ActivityService, education schoolstructure.GroupManagement, pickups carePlanModule.PickupScheduleService,
	timeTracking *workforceCompose.TimeTracking, rooms FacilitiesRooms, feedback *feedbackModule.Module,
	organizations organizationtenancy.Query,
) error {
	openRoomMove, err := newOpenRoomMove(facilities, timetableOwner, active, logger)
	if err != nil {
		return err
	}
	operations, err := presenceservice.NewPresenceOperations(active, groupLive, logger.With("service", "presence-operations"))
	if err != nil {
		return err
	}
	activeResource := presenceAPI.NewResource(operations, activePeople{source: NewAttendanceRoutePeople(users)}, teacherGroupIDs(education), activeStaffAccess{source: peopleCompose.NewAttendanceRouteStaff(caller)}, settings, ProtectedTenantRoutesWith(supervisionDashboardHTTP.NewResource(dashboard).Register), logger.With("handler", "active"), presence, activeRequestRuntime(), activeAuthorization(), settingsFacts.NewDisplaySettings(settings))
	sessionEnd, err := newSessionEnd(presence, timetableOwner, days.Bridge, persons, facilities, hub, emitter, logger)
	if err != nil {
		return err
	}
	iotResource, err := newIoTResource(db, logger, errorReportDSN, deviceAuth, presence, openRoomMove, sessionEnd, iot, facilities, active, users,
		activities, timetableOwner, timetableRepos, education, pickups, settings, timeTracking, rooms, feedback, organizations, hub)
	if err != nil {
		return err
	}
	move := &openRoomMoveHTTP.Resource{
		OpenRoomMove: openRoomMove, Authorize: activeResource.BulkStudentMoveAuthorization,
		MarkRollback: activeRequestRuntime().MarkRollback, RenderPresenceError: presenceAPI.ErrorRenderer,
		WebAttendance: settingsFacts.NewDisplaySettings(settings).WebAttendanceEnabledReader(),
	}
	yardResource := supervisionDashboardHTTP.NewSchulhofResource(newSupervisionYard(yard.Schulhof), func(ctx context.Context) (int64, bool, error) {
		staff, err := activeResource.UserContextService.GetCurrentStaff(ctx)
		if staff == nil {
			return 0, false, err
		}
		return staff.ID, true, err
	})
	tenant.Mount("/active", activeResource.RouterWithOwnerRoutes(move.Register, yardResource.Register))
	tenant.Mount("/iot", iotResource.Router())
	return nil
}

func teacherGroupIDs(education schoolstructure.GroupManagement) func(context.Context, int64) ([]int64, error) {
	return func(ctx context.Context, teacherID int64) ([]int64, error) {
		groups, err := education.GetTeacherGroups(ctx, teacherID)
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
func newIoTResource(
	db transactionruntime.Database, logger *slog.Logger, errorReportDSN string, deviceAuth *deviceauth.Authenticators,
	presence *studentpresence.Module, openRoomMove openroommove.Command, sessionEnd sessionend.Command,
	iot devicefleetCompose.IoTService, facilities *facilitiesModule.Module, active studentpresence.Presence,
	users peoplePorts.PersonDirectory, activities activityContract.ActivityService, timetableOwner timetableModule.Capability,
	timetableRepos timetableCompose.Repositories, education schoolstructure.GroupManagement, pickups carePlanModule.PickupScheduleService,
	settings settingsCompose.SettingsService, timeTracking *workforceCompose.TimeTracking, rooms FacilitiesRooms,
	feedback *feedbackModule.Module, organizations organizationtenancy.Query, hub *realtime.Hub,
) (*iotAPI.Resource, error) {
	errorReports, err := NewIoTErrorReportRelay(errorReportDSN)
	if err != nil {
		return nil, err
	}
	deviceScan := devicescanCompose.New(devicescanCompose.Dependencies{Principal: DeviceScanPrincipal,
		Fleet:      iot.Fleet(),
		Presence:   presence,
		Rooms:      facilities,
		Active:     active,
		Users:      NewDeviceScanPeople(users),
		Activities: DeviceActivityCatalog(activities),
		Rosters:    timetableOwner,
		Education:  education,
		Pickups:    pickups,
		// A destination chosen at the kiosk runs the phone's open-room
		// move (#3067).
		OpenRooms: newDeviceOpenRoomMover(openRoomMove),
		Settings:  newDeviceScanSettings(settings),
		Logger:    logger.With("service", "device-scan"),
	})
	return iotAPI.NewResource(iotAPI.ServiceDependencies{
		Administration:   NewIoTAdministration(devicefleetCompose.NewAdministration(iot.Fleet())),
		Runtime:          NewIoTHTTPRuntime(),
		DeviceScan:       deviceScan,
		OpenRooms:        deviceScan,
		StaffClock:       newStaffClockService(users, NewPeopleIdentityAccess(db, observability.ObserveIdentityAccessOperation), timeTracking.TimeClock),
		Configuration:    devicescanCompose.NewConfiguration(DeviceConfiguration{Settings: settings}, DeviceScanPrincipal),
		Rooms:            devicescanCompose.NewRoomAvailability(rooms.RoomService, DeviceScanPrincipal),
		Directory:        devicescanCompose.NewDirectory(NewDeviceScanPeople(users), DeviceActivityCatalog(activities), logger, DeviceScanPrincipal),
		TagAssignments:   devicescanCompose.NewTagAssignments(NewDeviceScanPeople(users), logger, DeviceScanPrincipal),
		FeedbackStudents: devicescanCompose.NewFeedbackStudents(NewDeviceScanPeople(users)),
		FeedbackService:  feedback,
		FeedbackResponseObserver: func(status int, code string) {
			observability.ObserveFeedbackHTTPResponse("iot", status, code)
		},
		SchoolName:              devicescanCompose.NewSchoolName(schoolName(organizations), DeviceScanPrincipal),
		ErrorReports:            NewIoTErrorReporter(errorReports),
		SessionEnd:              bindIoTHostSessionEnd(BindIoTSessionEnd(sessionEnd)),
		SessionLifecycle:        devicescanCompose.NewSessionLifecycle(active, devicescanCompose.NewSupervisionQuery(presence), NewDeviceScanPeople(users), iot, NewKioskSessionMirror(timetableRepos, activities, KioskMirrorPublisher(hub, logger), logger), logger, DeviceScanPrincipal),
		Logger:                  logger.With("handler", "iot"),
		DeviceAuthenticator:     deviceAuth.Device(),
		DeviceOnlyAuthenticator: deviceAuth.DeviceOnly(),
	}), nil
}

// mountStaffRoutes serves the staff administration, shift planning,
// substitutions and time tracking.
func mountStaffRoutes(
	tenant chi.Router, db transactionruntime.Database, logger *slog.Logger, membership schoolMembershipModule.Capability,
	workTime workforceModule.Capability, users peoplePorts.PersonDirectory, staffAdmin *workforceModule.StaffAdmin,
	workforce WorkforcePlanning, shifts *workforceCompose.ShiftPlanning, education schoolstructure.GroupManagement,
	substitution schoolStructureCompose.SubstitutionModule, persons peopledirectory.Capability, settings settingsCompose.SettingsService,
	hub *realtime.Hub, identity *identityaccess.Module, caller *CallerRows, calendar schoolcalendar.Calendar,
) error {
	staff, _, err := newStaffComposition(membership, workTime, users, staffAdmin, workforce.TimeTracking, education, persons, settings, hub, identity, db, logger.With("handler", "staff"))
	if err != nil {
		return err
	}
	staffShifts := newStaffShiftsResource(staffShiftsDependencies{
		Planning:       workforce.StaffShifts,
		ResolveStaffID: currentStaffID(caller),
		ActorAccountID: sessiontoken.ActorAccountIDFromCtx,
	})
	absenceTypes := newAbsenceTypesResource(workforceCompose.NewAbsenceTypeAdministration(workTime, logger.With("service", "active")), currentStaffID(caller)).Router()
	tenant.Mount("/staff", staff.Router())
	tenant.Mount("/work-time-models", newWorkTimeModelsResource(workTime, staffTimeTrackingNotifier(hub)).Router())
	tenant.Mount("/staff-shifts", staffShifts.Router())
	tenant.Mount("/shift-types", newShiftTypesResource(shifts.ShiftTypes).Router())
	tenant.Mount("/absence-types", absenceTypes)
	tenant.Mount("/substitutions", newSubstitutionsResource(SubstitutionCapability(substitution)).Router())
	tenant.Mount("/time-tracking", newTimeTrackingResource(users, staffAdmin, workforce.TimeTracking, workforce.StaffShifts, shifts.Assignments, calendar, settings).Router(ProtectedTenantRoutes))
	return nil
}

// mountSchoolPortalRoutes serves the school portal ("moto schule", #2207) at
// the root: public /school/auth/* (login and the school-scope MFA exchange)
// plus the class-day surface. Token refresh and logout go through the shared
// scope-preserving /auth/refresh and /auth/logout. The portal reuses the
// timetable, colleague chat, staff notice and notification resources the
// staff reach under /api (#2207, #2527).
func mountSchoolPortalRoutes(
	root, tenant chi.Router, db transactionruntime.Database, logger *slog.Logger, limiters authRateLimiters,
	presence *studentpresence.Module, timetableOwner timetableModule.Capability, calendar schoolcalendar.Calendar,
	materialization timetableModule.MaterializationCapability, lifecycle TimetableLifecycle, planning TimetablePlanning,
	planningTracks timetableModule.PlanningTrackAdministration, users peoplePorts.PersonDirectory, schedules CarePlanSchedules,
	caller *CallerRows, settings settingsCompose.SettingsService, careOfferings carePlanModule.CareOfferingCapability,
	planExport planexport.Service, chat CommunicationMessaging, persons peopledirectory.Capability, notifications DeliveryNotifications,
	devices DeliveryDevices, classDay ClassDayViews, identity *identityaccess.Module,
) error {
	pickupExtensions, err := careexit.NewPickupExtensions(timetableOwner, presence)
	if err != nil {
		return err
	}
	timetableResource := newTimetableResource(calendar, timetableOwner, materialization, lifecycle, planning, planningTracks, users, schedules,
		caller, settings, careOfferings, classDay, planExport, pickupExtensions, logger)
	// OGS-internal colleague chat (#2598) — staff-to-staff, deliberately a
	// separate surface from /messages (which is parent-facing).
	staffMessaging := staffMessagingAPI.NewResource(chat.StaffMessaging)
	// Tagesinformationen (#2180) are Timetable's: the owner composes the
	// service over its own repository, the calendar periods for the week
	// pattern and the People Directory names of the acknowledgement list
	// (#3418). All staff read them, admins write them.
	staffNotices := timetableAPI.NewStaffNoticeResource(timetableCompose.NewStaffNotices(timetableCompose.StaffNoticeDependencies{
		DB: db.Pool(), Periods: timetableCompose.NewCalendarPeriodRepository(calendar, newCalendarPeriodUsage(timetableOwner)), Names: staffNoticeNames(persons),
		Logger: logger.With("service", "staffnotice"),
	}), func(ctx context.Context) int64 { return timeTrackingIdentity(ctx).AccountID })
	// Notification abstraction (#1624).
	notificationRoutes := NewNotificationHTTP(notifications.Notifier, devices.PushSubscriptions, notifications.Preferences)
	// The class-day surface reads the class-day projection (#2701): the day
	// report over Enrollment's day roster and the arrival-exception write
	// seam (#2970) behind its one public capability.
	classDayResource := classdayHTTP.NewResource(classDay.ClassDay, logger.With("handler", "class-day"))
	school := NewSchoolPortalHTTP(SchoolPortalAuth(identity), SchoolPortalMFA(identity), schoolPasswordResets(identity), classDayResource, timetableResource, staffMessaging, staffNotices, notificationRoutes)
	tenant.Mount("/staff-messages", staffMessaging.Router(ProtectedTenantRoutes))
	tenant.Mount("/staff-notices", staffNotices.Router(ProtectedTenantRoutes))
	tenant.Mount("/timetable", timetableResource.Router(ProtectedTenantRoutesWith(classdayHTTP.NewSlotListResource(classDay.SlotLists).Register)))
	tenant.Mount("/notifications", notificationRoutes.Router())
	root.Mount("/school", school.RouterWithAuthRateLimiter(limiters.authMiddleware(), SchoolSessionChain, ProtectedSchoolRoutes))
	return nil
}

// newTimetableResource composes the timetable routes. Dateframes belong to
// the School Calendar; the conflict detection is the one the instance
// lifecycle uses.
func newTimetableResource(
	calendar schoolcalendar.Calendar, timetableOwner timetableModule.Capability, materialization timetableModule.MaterializationCapability,
	lifecycle TimetableLifecycle, planning TimetablePlanning, planningTracks timetableModule.PlanningTrackAdministration,
	users peoplePorts.PersonDirectory, schedules CarePlanSchedules, caller *CallerRows, settings settingsCompose.SettingsService,
	careOfferings carePlanModule.CareOfferingCapability, classDay ClassDayViews, planExport planexport.Service,
	pickupExtensions timetableModule.PickupExtensionCapability, logger *slog.Logger,
) *timetableAPI.Resource {
	return timetableAPI.NewResource(timetableAPI.Dependencies{
		CalendarPeriods:         calendar,
		ClosingDays:             calendar,
		CalendarPeriodUsage:     calendarPeriodUsage{usage: newCalendarPeriodUsage(timetableOwner)},
		MaterializationService:  materialization,
		InstanceService:         lifecycle.Instances,
		InstanceSeriesConverter: planning.SeriesConverter,
		OperationsService:       lifecycle.Operations,
		People:                  NewTimetablePeople(users),
		Templates:               planning.Planning.Templates,
		RecurrenceLock:          planning.Planning.RecurrenceLock,
		AttendanceCorrections:   planning.Planning.AttendanceCorrections,
		Deviations:              planning.Planning.Deviations,
		TimetableData:           planning.Planning.Data,
		ConflictDetection:       planning.Planning.ConflictDetection,
		PlanningTracks:          planningTracks,
		CareDayService:          schedules.CareDay,
		UserContextService:      caller,
		SettingsService:         settings,
		OfferingSourceOptions:   NewTimetableOfferingSources(careOfferings),
		SupervisionSheets:       NewTimetableSupervisionSheets(classDay.ClassDay),
		PlanExportService:       planExport,
		PickupExtensions:        pickupExtensions,
		Staffing:                timetableStaffingAnnouncer(lifecycle.Instances),
		Logger:                  logger.With("handler", "timetable"),
	})
}

// mountSchoolRoutes serves the school's rooms, groups, activities, schedules,
// enrollment, settings, info-point displays and data administration under
// /api.
func mountSchoolRoutes(
	tenant chi.Router, db transactionruntime.Database, logger *slog.Logger, facilities *facilitiesModule.Module, rooms FacilitiesRooms,
	settings settingsCompose.SettingsService, caller *CallerRows, active studentpresence.Presence, users peoplePorts.PersonDirectory,
	education schoolstructure.GroupManagement, renderer *lists.RendererService, activities activityContract.ActivityService,
	timetableOwner timetableModule.Capability, iot devicefleetCompose.IoTService, calendar schoolcalendar.Calendar,
	planning TimetablePlanning, careOfferings carePlanModule.CareOfferingCapability, settingsWrites SettingsWrites,
	membership schoolMembershipModule.Capability, classListEntries schoolMembershipModule.ClassListEntries, databaseStats DatabaseStatsReader,
	dataImports imports, persons peopledirectory.Capability, gradeTransition *gradetransition.Workflow, enrollment *enrollmentAPI.Resource,
) {
	tenant.Mount("/rooms", roomsHTTPAdapter.NewResource(facilities, roomsHTTPAdapter.Dependencies{
		Facilities:          rooms.RoomService,
		RoomHistorySettings: NewRoomHistorySettings(settings, logger.With("handler", "rooms"), settingsFacts.PrefetchSettings),
		UserContext:         caller, Active: active,
		Users: peopleCompose.NewRoomSnapshotPeople(users), Education: education,
		ListExport: renderer,
	}, logger.With("handler", "rooms"), ProtectedTenantRoutes, transactionFailures.IsConstraintViolation).Router())
	tenant.Mount("/groups", groupsHTTP.NewResource(education, active, peopleCompose.NewGroupRoutePeople(users), caller, transactionFailures.IsConstraintViolation).Router(ProtectedTenantRoutes))
	tenant.Mount("/activities", timetableHTTPAdapter.NewResource(activities, timetableOwner, NewTimetableSupervisorDirectory(users), caller, TimetableHTTPTransactions).Router(ProtectedTenantRoutes))
	// Parent enrollment (PR 5+).
	tenant.Mount("/enrollment", enrollment.Router(TenantSessionChain))
	// Info-point displays (#1325). One Device Fleet owner serves every entry
	// point: the root composes it and the IoT service hands it back here
	// (#2676).
	tenant.Mount("/display", NewDisplayHTTPRouter(iot.Fleet(), DisplayFeatureEnabled(settings)))
	// Dateframes belong to the School Calendar, timeframes and recurrence
	// rules to the Timetable owner; timeframe changes stay guarded by the
	// recurrence gate and the Care Plan catalog's care-offering check.
	tenant.Mount("/schedules", timetableHTTPAdapter.NewSchedulesResource(calendar, timetableOwner, TimeframeChangeGuard(
		planning.Planning.RecurrenceLock.LockRecurrenceWrites,
		careOfferings.ValidateTimeframeChange,
	), TimetableHTTPTransactions).Router(ProtectedTenantRoutes))
	// The schema-driven settings system.
	homeLayouts := requireHomeLayoutOperations(settings)
	tenant.Mount("/settings", newSettingsResource(settingsWrites.Tenant, homeLayouts, NewEnrollmentBookings().SchemaReferencesLegalDocument).SettingsRouter())
	// Class-list-only entries (#2382).
	tenant.Mount("/class-list-entries", newClassListEntriesResource(classListEntries, logger.With("handler", "class-list-entries")).Router())
	tenant.Mount("/database", newDatabaseStatsRouter(databaseStats, logger.With("handler", "database")))
	// CSV/Excel import endpoints.
	tenant.Mount("/import", importAPI.NewResource(importAPI.Dependencies{
		Students: dataImports.Student, Staff: dataImports.Staff, ClassList: dataImports.ClassList,
		Files: dataImportCompose.NewFileDecoder(), Runtime: dataImportHTTPRuntime(persons, membership, dataImports.OpeningBalance),
	}).Router())
	tenant.Mount("/admin/grade-transitions", gradeTransitionHTTP.NewGradeTransitionResource(gradeTransition).Router(ProtectedTenantRoutes))
}

// newEnrollmentResource builds Enrollment's staff and public routes. The
// root builds the form schemas, the captcha and the phase expiry warnings
// here, over the settings and the owners it composed (#4012).
func newEnrollmentResource(
	logger *slog.Logger, settings settingsCompose.SettingsService, careOfferings carePlanModule.CareOfferingCapability,
	flows EnrollmentFlows, phases enrollmentOwner.PhaseAdministration, decisions enrollmentOwner.Decisions,
	deletions enrollmentOwner.EnrollmentDeletions, identity *identityaccess.Module,
	guardianProfiles *peopleCompose.GuardianProfileLoader, organizations organizationtenancy.Query,
	renderer *lists.RendererService, persons peopledirectory.Capability, carePlan carePlanModule.Capability,
) *enrollmentAPI.Resource {
	resource := enrollmentAPI.NewResource(
		newEnrollmentFormSchemas(settings, logger),
		enrollmentAPI.NewCareOfferingCatalog(NewEnrollmentCareOfferingValues(careOfferings)),
		enrollmentAPI.NewRequestService(flows.Intake),
		newEnrollmentCaptcha(settings, logger),
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
		NewEnrollmentBookings(),
	)
	resource.ListExportService = renderer
	resource.PhaseExpiryService = newEnrollmentPhaseExpiry(persons, carePlan)
	return resource
}

// mountPeopleRoutes serves the people directory, guardians, statistics,
// birthdays, the caller's own context, the emergency snapshot, feedback and
// the meal plan under /api.
func mountPeopleRoutes(
	tenant chi.Router, db transactionruntime.Database, logger *slog.Logger, reports PresenceReports, renderer *lists.RendererService,
	persons *peopledirectory.Module, guardians peoplePorts.GuardianService, identity *identityaccess.Module, caller *CallerRows,
	feedback *feedbackModule.Module, mealPlan *mealplanModule.Module, birthdays peopledirectory.Birthdays,
	emergency emergencysnapshot.Query,
) {
	tenant.Mount("/statistics", statisticsAPI.NewResource(reports.Statistics, statisticsHTTPRuntime(renderer), logger.With("handler", "statistics")).Router(ProtectedTenantRoutes))
	tenant.Mount("/guardians", newGuardiansResource(persons, NewGuardianDirectoryRuntime(guardians, renderer, identity, caller.HasCurrentStaff, peopleCompose.NewLegacyRepositoryRuntime(db.Pool())), viper.GetString("app_env"), logger.With("handler", "guardians")).Router())
	tenant.Mount("/feedback", newFeedbackResource(feedback).Router())
	tenant.Mount("/meal-plan", newMealPlanResource(mealPlan, newMealPlanExportRenderer()).Router())
	tenant.Mount("/users", newUsersResource(persons, identity.ListAccountEmails, func(ctx context.Context, tagID string) (bool, error) {
		_, _, found, err := identity.LookupRFIDCard(ctx, tagID)
		return found, err
	}).Router())
	// Birthday display and staff birthday list (#1542).
	tenant.Mount("/birthdays", birthdaysAPI.NewResource(birthdays, renderer, caller, logger.With("handler", "birthdays")).Router(ProtectedTenantRoutes))
	tenant.Mount("/me", meAPI.NewResource(caller.Caller(), caller, TenantSessionChain, nativeUploads.UploadFiles{}).Router())
	tenant.Mount("/emergency", emergencyAPI.NewResource(emergency).Router(ProtectedTenantRoutes))
}

// mountCommunicationRoutes serves parent messages and announcements, staff
// reminders, the PWA usage report and the platform announcements under /api.
func mountCommunicationRoutes(
	tenant chi.Router, chat CommunicationMessaging, announcements communicationModule.ParentAnnouncementCapability,
	reminders reminder.Capability, devices DeliveryDevices, communication *communicationModule.Module,
) {
	tenant.Mount("/messages", messagingAPI.NewResource(chat.Messaging).Router(ProtectedTenantRoutes))
	tenant.Mount("/parent-announcements", announcementAPI.NewResource(announcements, newDeclarationReports()).Router(ProtectedTenantRoutes))
	// Visual-only staff reminders (#1457).
	tenant.Mount("/reminders", remindersHTTP.NewResource(reminders, ReminderHTTPRuntime()).Router())
	// PWA standalone-usage reporting (#2189).
	tenant.Mount("/pwa", pwaUsageRouter(devices.PWAUsage))
	// User-facing platform announcements.
	tenant.Mount("/platform", platformAnnouncementsHTTP.NewResource(platformAnnouncementsHTTP.ResourceConfig{
		AnnouncementsService: communication,
		Runtime:              newPlatformRuntime(),
	}).Router())
}

// mountCalendarRoutes serves the personal calendars under /api, the
// read-only staff CalDAV and the public subscription feeds.
func mountCalendarRoutes(root, tenant chi.Router, logger *slog.Logger, calendarApp calendarCompose.Application) {
	calendar := calendarAPI.NewResource(calendarApp, logger.With("handler", "calendar"))
	tenant.Mount("/calendar", calendar.Router(TenantSessionChain))

	// Public parent calendar subscription feed (no auth — the token in the URL
	// is the capability). Calendar apps (Apple/Google/Outlook) poll this to keep
	// the parent's Termine in sync.
	root.Get("/public/calendar/{token}", calendarAPI.PublicFeed(calendarApp))

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
func mountDeliveryRoutes(root, tenant chi.Router, logger *slog.Logger, fileStore *filestorageModule.Module, hub *realtime.Hub, caller *CallerRows, identity *identityaccess.Module) error {
	files := filestoreAPI.NewResource(fileStore, logger.With("handler", "filestore"))
	sse, err := sseAPI.NewResourceWithSchoolAccess(hub, caller, NewSSEHTTPRuntime(), identity, logger.With("handler", "sse"))
	if err != nil {
		return err
	}
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
	return nil
}

// mountPortalRoutes mounts the root-level session portals: tenant auth,
// operator dashboard (separate from the tenant API) and the cross-tenant
// guardian portal. The auth rate limiter guards each login.
func mountPortalRoutes(root chi.Router, limiters authRateLimiters, account *authAPI.Resource, operator chi.Router, parent *parentAPI.Resource) {
	// RouterWithAuthRateLimiter applies the limiter only to the public login,
	// password-reset, MFA and passkey-login routes.
	root.Mount("/auth", account.RouterWithAuthRateLimiter(limiters.authMiddleware(), TenantSessionChain))
	root.Mount("/operator", operator)
	root.Mount("/parent", parent.RouterWithAuthRateLimiter(limiters.authMiddleware(), ParentSessionChain))
}

// caregiverCapabilityViews binds People Directory's caregiver capability for
// the account and operator routes that toggle it (#2736); db opens the
// administrative transaction of the operator routes.
func caregiverCapabilityViews(db transactionruntime.Database, caregivers peoplePorts.CaregiverCapabilityService) peopleCompose.CaregiverCapabilityViews {
	return peopleCompose.NewCaregiverCapabilityViews(caregivers, db.Pool())
}

// newAccountResource composes the tenant account routes. Their caregiver
// capability is the one the operator routes provision with.
func newAccountResource(db transactionruntime.Database, identity *identityaccess.Module, organizations organizationtenancy.Query, caregivers peoplePorts.CaregiverCapabilityService, settings settingsCompose.SettingsService) *authAPI.Resource {
	authSchools := authSchoolDirectory{schools: organizations, memberships: identity.ListActiveAccountSchoolIDs}
	resource := authAPI.NewResource(identity, identity, authSchools, identity, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation)
	resource.CaregiverCapabilityService = caregiverCapabilityViews(db, caregivers)
	resource.SettingsService = settings
	resource.RoleGrants = RoleGrantPolicy{}
	resource.MFAService = identity
	resource.PasskeyService = identity
	return resource
}

// newOperatorRouter composes the operator dashboard. The Device Fleet review
// of unregistered RFID scans answers in the operator surface's format (#3232).
func newOperatorRouter(
	db transactionruntime.Database, logger *slog.Logger, sessionAuth *sessiontoken.TokenAuth, limiters authRateLimiters,
	iot devicefleetCompose.IoTService, organizations organizationtenancy.Query, settings settingsCompose.SettingsService,
	hub *realtime.Hub, active studentpresence.Presence, settingsWrites SettingsWrites, identity *identityaccess.Module,
	provisioning organizationtenancy.Provisioning, caregivers peoplePorts.CaregiverCapabilityService,
	lifecycle carePlanModule.CareLifecycle, communication *communicationModule.Module,
) (chi.Router, error) {
	tagScanReview := tagScanOperatorAPI.NewResource(tagScanOperatorAPI.Config{
		Scans:     iot.Fleet(),
		Directory: tagScanSchoolDirectory{schools: organizations},
		Surface: tagScanOperatorAPI.Surface{
			InvalidRequest: apiCommon.OperatorInvalidRequest,
			Internal:       apiCommon.OperatorInternal,
			RenderError:    apiCommon.RenderError,
			Respond:        apiCommon.Respond,
			OperatorID:     operatorIDFromSession,
		},
	})
	billing, err := newOperatorBilling(logger)
	if err != nil {
		return nil, fmt.Errorf("compose operator billing: %w", err)
	}
	schoolSettings := settingsCompose.NewOperatorSchoolSettings(settingsCompose.OperatorDependencies{
		Settings:       settings,
		DB:             db.Pool(),
		Notify:         settingsChangedNotifier(hub),
		OpenAttendance: OpenAttendanceChecker{Service: active},
		// Mirror the tenant-side OnValueSet hook so operator writes also
		// trigger side effects (e.g. auto-creating the Schulhof/WC rooms when
		// the corresponding checkout toggle flips on).
		OnValueSet: settingsWrites.SideEffects.Dispatch,
	})
	operatorIdentity := operatorIdentity{
		sessions:    identityOperatorAPI.NewSessions(sessionAuth, identity),
		account:     identityOperatorAPI.NewResource(identity, operatorIdentityResponses()),
		access:      identity,
		passkeys:    identity,
		mfa:         identity,
		invitations: identity,
		accountMFA:  identity,
	}
	provisioningConfig := provisioningOperatorAPI.ProvisioningConfig{
		Service:             provisioning,
		CaregiverCapability: caregiverCapabilityViews(db, caregivers),
		AppEnv:              viper.GetString("app_env"),
	}
	isLocalSeedRequest := func(r *http.Request) bool {
		return apiCommon.IsLocalSeedRequest(r, viper.GetString("app_env"))
	}
	return operatorRouter(
		operatorIdentity,
		provisioningConfig,
		provisioningOperatorAPI.NewBillingResource(billing, nil, isLocalSeedRequest),
		&operatorSchoolSettings{
			values: settingsOperatorAPI.NewSettingsResource(settingsOperatorAPI.SettingsConfig{
				Settings:   schoolSettings,
				Schools:    operatorSettingsSchools{schools: organizations},
				OperatorID: operatorIDFromSession,
			}),
			bookingAuthorityImpact: newBookingAuthorityImpact(lifecycle),
		},
		communication,
		tagScanReview.Router(),
		limiters,
	), nil
}

// newParentResource composes the cross-tenant guardian portal: public
// /parent/auth/login and the protected /parent/* routes.
func newParentResource(
	identity *identityaccess.Module, parent *parentportal.Portal, calendarApp calendarCompose.Application, flows EnrollmentFlows,
	guardianProfiles *peopleCompose.GuardianProfileLoader, organizations organizationtenancy.Query,
	devices DeliveryDevices, notifications DeliveryNotifications,
) *parentAPI.Resource {
	return parentAPI.NewResource(parentAPI.ResourceConfig{
		Auth:                  parentPortalLogin(identity),
		Resets:                parentPasswordResets(identity),
		Parent:                parent,
		Calendar:              calendarApp,
		Enrollment:            parentEnrollmentForms(enrollmentAPI.NewRequestService(flows.Intake)),
		GuardianProfileLoader: newParentGuardianProfiles(guardianProfiles),
		Schools:               parentSchoolDirectory{schools: organizations},
		Push:                  bindParentPushSubscriptions(devices.PushSubscriptions),
		Preferences:           bindParentPreferences(notifications.Preferences),
		PWAUsage:              devices.PWAUsage,
		Reports:               newDeclarationReports(),
		SupportedLocale:       bindParentLocale(notifications.Preferences),
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
func currentStaffID(caller *CallerRows) func(context.Context) (int64, error) {
	return func(ctx context.Context) (int64, error) {
		return caller.Caller().StaffID(ctx)
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
