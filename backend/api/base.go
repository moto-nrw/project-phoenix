package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	slogchi "github.com/samber/slog-chi"
	"github.com/spf13/viper"
	"github.com/uptrace/bun"

	activeAPI "github.com/moto-nrw/project-phoenix/api/active"
	activitiesAPI "github.com/moto-nrw/project-phoenix/api/activities"
	adminAPI "github.com/moto-nrw/project-phoenix/api/admin"
	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	configAPI "github.com/moto-nrw/project-phoenix/api/config"
	databaseAPI "github.com/moto-nrw/project-phoenix/api/database"
	feedbackAPI "github.com/moto-nrw/project-phoenix/api/feedback"
	groupsAPI "github.com/moto-nrw/project-phoenix/api/groups"
	guardiansAPI "github.com/moto-nrw/project-phoenix/api/guardians"
	importAPI "github.com/moto-nrw/project-phoenix/api/import"
	iotAPI "github.com/moto-nrw/project-phoenix/api/iot"
	roomsAPI "github.com/moto-nrw/project-phoenix/api/rooms"
	schedulesAPI "github.com/moto-nrw/project-phoenix/api/schedules"
	sseAPI "github.com/moto-nrw/project-phoenix/api/sse"
	staffAPI "github.com/moto-nrw/project-phoenix/api/staff"
	studentsAPI "github.com/moto-nrw/project-phoenix/api/students"
	substitutionsAPI "github.com/moto-nrw/project-phoenix/api/substitutions"
	suggestionsAPI "github.com/moto-nrw/project-phoenix/api/suggestions"
	timeTrackingAPI "github.com/moto-nrw/project-phoenix/api/time-tracking"
	timetableAPI "github.com/moto-nrw/project-phoenix/api/timetable"
	usercontextAPI "github.com/moto-nrw/project-phoenix/api/usercontext"
	usersAPI "github.com/moto-nrw/project-phoenix/api/users"

	operatorAPI "github.com/moto-nrw/project-phoenix/api/operator"
	platformAPI "github.com/moto-nrw/project-phoenix/api/platform"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	customMiddleware "github.com/moto-nrw/project-phoenix/middleware"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services"
)

// API represents the API structure
type API struct {
	Services *services.Factory
	Router   chi.Router
	db       *bun.DB
	repos    *repositories.Factory

	// API Resources
	Auth             *authAPI.Resource
	Rooms            *roomsAPI.Resource
	Students         *studentsAPI.Resource
	Groups           *groupsAPI.Resource
	Guardians        *guardiansAPI.Resource
	Import           *importAPI.Resource
	Activities       *activitiesAPI.Resource
	Staff            *staffAPI.Resource
	Feedback         *feedbackAPI.Resource
	Suggestions      *suggestionsAPI.Resource
	Schedules        *schedulesAPI.Resource
	Settings         *configAPI.SettingsResource
	Active           *activeAPI.Resource
	IoT              *iotAPI.Resource
	SSE              *sseAPI.Resource
	Users            *usersAPI.Resource
	UserContext      *usercontextAPI.Resource
	Substitutions    *substitutionsAPI.Resource
	Database         *databaseAPI.Resource
	GradeTransitions *adminAPI.GradeTransitionResource
	TimeTracking     *timeTrackingAPI.Resource
	Timetable        *timetableAPI.Resource

	// Operator Dashboard (platform domain)
	Operator *operatorAPI.Resource
	Platform *platformAPI.Resource
}

// New creates a new API instance
func New(enableCORS bool, logger *slog.Logger) (*API, error) {
	// Get database connection as phoenix_auth (least-privilege for serve)
	db, err := database.DBConnForServe()
	if err != nil {
		return nil, err
	}

	if viper.GetBool("db_debug") {
		db.AddQueryHook(database.NewQueryHook(logger.With("component", "database")))
	}

	// Initialize repository factory with DB connection
	repoFactory := repositories.NewFactory(db)

	// Initialize service factory with repository factory
	serviceFactory, err := services.NewFactory(repoFactory, db, logger)
	if err != nil {
		return nil, err
	}

	// Create API instance
	api := &API{
		Services: serviceFactory,
		Router:   chi.NewRouter(),
		db:       db,
		repos:    repoFactory,
	}

	// Setup router middleware
	setupBasicMiddleware(api.Router, logger)

	// Setup CORS, security logging, and rate limiting
	if enableCORS {
		setupCORS(api.Router)
	}
	securityLogger := setupSecurityLogging(api.Router)
	setupRateLimiting(api.Router, securityLogger)

	// Initialize API resources
	initializeAPIResources(api, repoFactory, db, logger)

	// Register routes with rate limiting
	api.registerRoutesWithRateLimiting()

	return api, nil
}

// setupBasicMiddleware configures basic router middleware
func setupBasicMiddleware(router chi.Router, logger *slog.Logger) {
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(slogchi.NewWithConfig(logger, slogchi.Config{
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
		},
	}))
	router.Use(middleware.Recoverer)
	sentryMiddleware := sentryhttp.New(sentryhttp.Options{Repanic: true})
	router.Use(sentryMiddleware.Handle)
	router.Use(customMiddleware.SecurityHeaders)
}

// setupCORS configures CORS middleware with allowed origins from environment.
// Supports wildcard subdomain patterns like "*.example.com" via AllowOriginFunc.
func setupCORS(router chi.Router) {
	exactOrigins, wildcardSuffixes := parseAllowedOrigins()

	opts := cors.Options{
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Staff-PIN", "X-Staff-ID", "X-Device-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}

	if len(wildcardSuffixes) > 0 {
		// Build a set for O(1) exact-match lookups
		exactSet := make(map[string]bool, len(exactOrigins))
		for _, o := range exactOrigins {
			exactSet[o] = true
		}
		opts.AllowOriginFunc = func(_ *http.Request, origin string) bool {
			if exactSet[origin] {
				return true
			}
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
	} else {
		opts.AllowedOrigins = exactOrigins
	}

	router.Use(cors.Handler(opts))
}

// parseAllowedOrigins parses CORS_ALLOWED_ORIGINS and splits entries into
// exact-match origins and wildcard subdomain suffixes (e.g. "*.example.com"
// becomes suffix ".example.com").
func parseAllowedOrigins() (exact []string, wildcardSuffixes []string) {
	originsEnv := os.Getenv("CORS_ALLOWED_ORIGINS")
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

	generalLimit := parsePositiveInt("RATE_LIMIT_REQUESTS_PER_MINUTE", 60)
	generalBurst := parsePositiveInt("RATE_LIMIT_BURST", 10)

	generalRateLimiter := customMiddleware.NewRateLimiter(generalLimit, generalBurst)
	if securityLogger != nil {
		generalRateLimiter.SetLogger(securityLogger)
	}
	router.Use(generalRateLimiter.Middleware())
}

// parsePositiveInt parses a positive integer from environment variable with a default value
func parsePositiveInt(envVar string, defaultValue int) int {
	valueStr := os.Getenv(envVar)
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
	api.Auth = authAPI.NewResource(api.Services.Auth, api.Services.Invitation, repoFactory.School, db)
	api.Auth.CaregiverCapabilityService = api.Services.CaregiverCapability
	api.Auth.SettingsService = api.Services.Settings
	api.Rooms = roomsAPI.NewResource(api.Services.Facilities, db)
	api.Students = studentsAPI.NewResource(studentsAPI.ResourceConfig{
		PersonService:          api.Services.Users,
		StudentRepo:            repoFactory.Student,
		EducationService:       api.Services.Education,
		UserContextService:     api.Services.UserContext,
		ActiveService:          api.Services.Active,
		IoTService:             api.Services.IoT,
		PrivacyConsentRepo:     repoFactory.PrivacyConsent,
		PickupScheduleService:  api.Services.PickupSchedule,
		ArrivalScheduleService: api.Services.ArrivalSchedule,
		SchoolRepo:             repoFactory.School,
		SettingsService:        api.Services.Settings,
		AttendanceRepo:         repoFactory.Attendance,
		StudentStatusDayRepo:   repoFactory.StudentStatusDay,
		VisitRepo:              repoFactory.ActiveVisit,
		DataAccessLogRepo:      repoFactory.DataAccessLog,
		Broadcaster:            api.Services.RealtimeHub,
		Logger:                 logger.With("handler", "students"),
		DB:                     db,
	})
	api.Groups = groupsAPI.NewResource(api.Services.Education, api.Services.Active, api.Services.Users, api.Services.UserContext, repoFactory.Student, repoFactory.GroupSubstitution, db)
	api.Guardians = guardiansAPI.NewResource(api.Services.Guardian, api.Services.Users, api.Services.Education, api.Services.UserContext, repoFactory.Student, db)
	api.Import = importAPI.NewResource(api.Services.Import, repoFactory.DataImport, api.Services.Users, db)
	api.Activities = activitiesAPI.NewResource(api.Services.Activities, api.Services.Schedule, api.Services.Users, api.Services.UserContext, db)
	api.Staff = staffAPI.NewResource(api.Services.Users, api.Services.Education, api.Services.Auth, repoFactory.GroupSupervisor, api.Services.WorkSession, repoFactory.StaffAbsence, db, logger.With("handler", "staff"))
	api.Feedback = feedbackAPI.NewResource(api.Services.Feedback, api.Services.Settings, db)
	api.Suggestions = suggestionsAPI.NewResource(api.Services.Suggestions, db)
	api.Schedules = schedulesAPI.NewResource(api.Services.Schedule, db)
	api.Settings = configAPI.NewSettingsResource(api.Services.Settings, db)
	// Shared side-effect hook: when checkout.schulhof_enabled or
	// checkout.wc_enabled flips on (regardless of who flipped it), make sure
	// the corresponding system room exists. Also runs on disable transitions
	// for operations.student_photos_enabled to purge already-stored photos
	// — admin-confirmed in the UI before reaching here. Registered on BOTH
	// the tenant-admin resource and the operator resource so that either
	// writer triggers the side effect.
	//
	// The hook fires on PUT and DELETE (reset). DELETE forwards the registry
	// default as the post-reset effective value, so resetting
	// student_photos_enabled (default: false) purges photos exactly like
	// PUT value=false would.
	//
	// Two-phase contract: the closure returned alongside (postCommit, err) is
	// invoked ONLY after the surrounding tenant tx commits. Non-transactional
	// work that must not happen on rollback (file unlinks, external calls)
	// belongs there. In-tx work (DB writes, infrastructure provisioning that
	// participates in the tx) goes in the synchronous body.
	// broadcastStudentDataChanged tells the AFFECTED tenant's clients to
	// drop their cached student data and refetch. Used for tenant-wide
	// changes that affect what /api/students/* returns for every student
	// at once — disabling photos clears photo_url across the board,
	// re-enabling brings photo_url + consent metadata back even though
	// the rows themselves didn't change. Both cases require a cache-bust
	// because students-* SWR caches store the response shape, not the
	// underlying row, so the response shape changing is the trigger.
	//
	// Scoping: routed via BroadcastToTenant so a photo-toggle on school A
	// does not invalidate every ogs-students-*, search-students-*,
	// room-students-* and student-detail-* SWR cache on every other open
	// tenant tab on the platform — the on-receive handler in
	// use-global-sse.ts treats any student_updated as "blow away every
	// student cache", which fans out the cost of one toggle into N
	// unrelated tenants' refetch storms.
	//
	// `student_updated` is the existing event the global SSE handler
	// (use-global-sse.ts) maps to a bulk invalidation of student-detail-*,
	// ogs-students-*, search-students-*, room-students-*, etc., so we
	// reuse it instead of inventing a new event type. Source labels make
	// the trigger visible in log review.
	broadcastStudentDataChanged := func(tenantID int64, source string) {
		if api.Services.RealtimeHub == nil {
			return
		}
		s := source
		event := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{
			Source: &s,
		})
		if err := api.Services.RealtimeHub.BroadcastToTenant(tenantID, event); err != nil {
			logger.Warn("student_updated broadcast failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("source", s),
				slog.String("error", err.Error()),
			)
		}
	}

	purgeStudentPhotosInTx := func(ctx context.Context, tenantID int64) (func(), error) {
		urls, err := repoFactory.Student.PurgeAllPhotos(ctx)
		if err != nil {
			return nil, err
		}
		// Capture the URL list and unlink the files only after the tenant
		// tx commits. If the commit later fails (lock timeout, pool drop,
		// postgres restart mid-flight), the photo_path UPDATEs roll back —
		// running RemoveImage inside the tx would have orphaned the rows
		// (DB still pointing at /uploads/.../p_xxx.jpg, bytes already gone
		// → permanent 404 on every avatar until manual recovery).
		return func() {
			for _, url := range urls {
				path, resolveErr := apiCommon.ResolveStoredPath("public", url, apiCommon.StudentPhotoStoredURLPrefix)
				if resolveErr != nil {
					logger.Warn("could not resolve student photo path during feature-disable purge",
						slog.String("stored_url", url),
						slog.String("error", resolveErr.Error()),
					)
					continue
				}
				apiCommon.RemoveImage(path)
			}
			if len(urls) > 0 {
				logger.Info("student photos purged after feature disable",
					slog.Int("cleared_count", len(urls)),
				)
			}
			// Broadcast unconditionally — even if `urls` was empty. Other
			// open tabs may still hold render-state derived from a *prior*
			// enabled cycle (a re-disable after re-enable inside one
			// session): a stale `photos_enabled=true` signal in cached
			// schema responses, render decisions made off it, etc. The
			// SSE event is cheap and idempotent on the receive side.
			broadcastStudentDataChanged(tenantID, "tenant_photos_disabled")
		}, nil
	}
	// notifyTenantSettingsChanged returns a post-commit closure that fans out
	// a tenant_settings_changed SSE event for keys whose value travels through
	// /auth/tenant/resolve. Tenant tabs (including ones on a different origin
	// from the writer — i.e. an operator at operator.<domain> flipping a
	// setting that affects <slug>.<domain> tabs) re-resolve their tenant
	// context on receipt, which keeps feature gates (student_photos_enabled)
	// in sync without forcing a manual reload. The in-browser BroadcastChannel
	// in lib/settings-broadcast.ts is same-origin only and does not cover this.
	//
	// Scoping: routed via BroadcastToTenant so only clients connected to
	// the AFFECTED school receive the event. A previous implementation used
	// BroadcastToAll, which made every connected tab on the platform
	// re-resolve its own tenant on every settings change — a multi-school
	// deployment would see one operator toggle fan out into N
	// /auth/tenant/resolve calls for unrelated tenants. School A's clients
	// have Client.TenantID = A, so they receive A's events; school B sees
	// nothing for an A-side change.
	notifyTenantSettingsChanged := func(tenantID int64, key string) func() {
		return func() {
			if api.Services.RealtimeHub == nil {
				return
			}
			source := key
			event := realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{
				Source: &source,
			})
			if err := api.Services.RealtimeHub.BroadcastToTenant(tenantID, event); err != nil {
				logger.Warn("tenant_settings_changed broadcast failed",
					slog.Int64("tenant_id", tenantID),
					slog.String("key", key),
					slog.String("error", err.Error()),
				)
			}
		}
	}

	onSettingValueSet := func(ctx context.Context, tenantID int64, key string, value any) (func(), error) {
		switch key {
		case configModel.KeyCheckoutSchulhofEnabled:
			boolVal, ok := value.(bool)
			if !ok || !boolVal {
				return nil, nil
			}
			_, err := api.Services.Schulhof.EnsureInfrastructure(ctx, 0)
			return nil, err
		case configModel.KeyCheckoutWCEnabled:
			boolVal, ok := value.(bool)
			if !ok || !boolVal {
				return nil, nil
			}
			_, err := api.Services.WC.EnsureInfrastructure(ctx)
			return nil, err
		case configModel.KeyStudentPhotosEnabled:
			// Both directions need three signals to other tabs:
			//   - tenant_settings_changed → re-resolve TenantContext so
			//     feature gates flip without a manual reload.
			//   - student_updated → bulk-invalidate student-* SWR caches
			//     because the JSON shape returned by /api/students/*
			//     changes on every flip: disabling strips photo_url,
			//     enabling brings photo_url + preserved consent metadata
			//     back. Without this on the enable path, tabs that
			//     fetched students while photos were off keep showing
			//     them photo-less even after re-enable.
			//   - (disable only) post-commit file unlinks via the purge
			//     closure.
			notify := notifyTenantSettingsChanged(tenantID, key)
			boolVal, ok := value.(bool)
			if !ok || boolVal {
				return func() {
					broadcastStudentDataChanged(tenantID, "tenant_photos_enabled")
					notify()
				}, nil
			}
			purge, err := purgeStudentPhotosInTx(ctx, tenantID)
			if err != nil {
				return nil, err
			}
			return func() {
				if purge != nil {
					purge()
				}
				notify()
			}, nil
		default:
			return nil, nil
		}
	}
	api.Settings.OnValueSet(onSettingValueSet)
	api.Active = activeAPI.NewResource(api.Services.Active, api.Services.Users, api.Services.Schulhof, api.Services.UserContext, api.Services.Settings, db, logger.With("handler", "active"))
	api.IoT = iotAPI.NewResource(iotAPI.ServiceDependencies{
		IoTService:            api.Services.IoT,
		UsersService:          api.Services.Users,
		ActiveService:         api.Services.Active,
		ActivitiesService:     api.Services.Activities,
		SettingsService:       api.Services.Settings,
		FacilityService:       api.Services.Facilities,
		EducationService:      api.Services.Education,
		FeedbackService:       api.Services.Feedback,
		PickupScheduleService: api.Services.PickupSchedule,
		SchoolRepo:            repoFactory.School,
		Logger:                logger.With("handler", "iot"),
		DB:                    db,
	})
	api.SSE = sseAPI.NewResource(api.Services.RealtimeHub, api.Services.Active, api.Services.Users, api.Services.UserContext, api.Services.Settings, db, logger.With("handler", "sse"))
	api.Users = usersAPI.NewResource(api.Services.Users, db)
	api.UserContext = usercontextAPI.NewResource(api.Services.UserContext, db)
	api.Substitutions = substitutionsAPI.NewResource(api.Services.Education, db)
	api.Database = databaseAPI.NewResource(api.Services.Database, db)
	api.GradeTransitions = adminAPI.NewGradeTransitionResource(api.Services.GradeTransition, db)
	api.TimeTracking = timeTrackingAPI.NewResource(api.Services.WorkSession, api.Services.StaffAbsence, api.Services.Users, db)
	api.Timetable = timetableAPI.NewResource(timetableAPI.Dependencies{
		CalendarPeriodService:  api.Services.CalendarPeriod,
		MaterializationService: api.Services.Materialization,
		InstanceService:        api.Services.Instance,
		PersonService:          api.Services.Users,
		InstanceStudentRepo:    repoFactory.InstanceStudent,
		ActivityInstanceRepo:   repoFactory.ActivityInstance,
		ActivityExceptionRepo:  repoFactory.ActivityException,
		ActivityScheduleRepo:   repoFactory.ActivitySchedule,
		InstanceStaffRepo:      repoFactory.InstanceStaff,
		SupervisorRepo:         repoFactory.GroupSupervisor,
		ArrivalScheduleRepo:    repoFactory.StudentArrivalSchedule,
		ArrivalExceptionRepo:   repoFactory.StudentArrivalException,
		PickupScheduleRepo:     repoFactory.StudentPickupSchedule,
		PickupExceptionRepo:    repoFactory.StudentPickupException,
		VisitRepo:              repoFactory.ActiveVisit,
		StudentRepo:            repoFactory.Student,
		StaffRepo:              repoFactory.Staff,
		UserContextService:     api.Services.UserContext,
		SettingsService:        api.Services.Settings,
		Broadcaster:            api.Services.RealtimeHub,
		Logger:                 logger.With("handler", "timetable"),
		DB:                     db,
	})

	// Initialize operator dashboard resources
	api.Operator = operatorAPI.NewResource(operatorAPI.ResourceConfig{
		AuthService:                api.Services.OperatorAuth,
		InvitationService:          api.Services.OperatorInvitation,
		ProvisioningService:        api.Services.OperatorProvisioning,
		CaregiverCapabilityService: api.Services.CaregiverCapability,
		SuggestionsService:         api.Services.OperatorSuggestions,
		AnnouncementsService:       api.Services.Announcement,
		SettingsService:            api.Services.Settings,
		SchoolRepo:                 repoFactory.School,
		TokenAuth:                  nil, // Created internally by operator API
		DB:                         db,
	})
	// Mirror the tenant-side OnValueSet hook so operator writes also trigger
	// side effects (e.g. auto-creating the Schulhof/WC rooms when the
	// corresponding checkout toggle flips on).
	api.Operator.OnSettingValueSet(onSettingValueSet)
	api.Platform = platformAPI.NewResource(platformAPI.ResourceConfig{
		AnnouncementsService: api.Services.Announcement,
		TokenAuth:            nil, // Uses tenant auth middleware
	})
}

// ServeHTTP implements the http.Handler interface for the API
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.Router.ServeHTTP(w, r)
}

// registerRoutesWithRateLimiting registers all API routes with appropriate rate limiting
func (a *API) registerRoutesWithRateLimiting() {
	// Check if rate limiting is enabled
	rateLimitEnabled := os.Getenv("RATE_LIMIT_ENABLED") == "true"

	// Get security logger if it exists
	var securityLogger *customMiddleware.SecurityLogger
	if os.Getenv("SECURITY_LOGGING_ENABLED") == "true" {
		securityLogger = customMiddleware.NewSecurityLogger()
	}

	// Configure auth-specific rate limiting if enabled
	var authRateLimiter *customMiddleware.RateLimiter
	var emailConfirmLimiter *customMiddleware.RateLimiter
	var invitationLimiter *customMiddleware.RateLimiter
	if rateLimitEnabled {
		// Stricter rate limit for auth endpoints
		authLimit := 5 // default: 5 requests per minute for auth
		if limit := os.Getenv("RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE"); limit != "" {
			if parsed, err := strconv.Atoi(limit); err == nil && parsed > 0 {
				authLimit = parsed
			}
		}
		authRateLimiter = customMiddleware.NewRateLimiter(authLimit, 10) // allow reasonable burst for login attempts
		// Separate instances for email-confirm and invitations: same config,
		// independent per-IP counters. Prevents cross-endpoint budget exhaustion.
		emailConfirmLimiter = customMiddleware.NewRateLimiter(authLimit, 10)
		invitationLimiter = customMiddleware.NewRateLimiter(authLimit, 10)
		if securityLogger != nil {
			authRateLimiter.SetLogger(securityLogger)
			emailConfirmLimiter.SetLogger(securityLogger)
			invitationLimiter.SetLogger(securityLogger)
		}
	}
	a.Router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("MOTO API - Phoenix Project"))
	})

	a.Router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})

	// Note: Avatar files are served through authenticated endpoints, not as static files
	// This prevents unauthorized access to user avatars

	// Public login image serving (no auth — displayed on the login page before authentication)
	a.Router.Get("/public/login-image/{filename}", func(w http.ResponseWriter, r *http.Request) {
		filename := chi.URLParam(r, "filename")
		apiCommon.ServeImage(w, r, "public/uploads/login-images", filename, "public, max-age=86400")
	})

	// Mount API resources
	// Auth routes mounted at root level to match frontend expectations
	// Rate limiting is applied per-route inside Auth.Router() (only login, register, password-reset)
	if rateLimitEnabled && authRateLimiter != nil {
		a.Auth.SetAuthRateLimiter(authRateLimiter.Middleware())
	}
	a.Router.Mount("/auth", a.Auth.Router())

	// Other API routes under /api prefix for organization
	a.Router.Route("/api", func(r chi.Router) {
		// Mount room resources
		r.Mount("/rooms", a.Rooms.Router())

		// Mount student resources
		r.Mount("/students", a.Students.Router())

		// Mount guardian resources
		r.Mount("/guardians", a.Guardians.Router())

		// Mount group resources
		r.Mount("/groups", a.Groups.Router())

		// Mount activities resources
		r.Mount("/activities", a.Activities.Router())

		// Mount staff resources
		r.Mount("/staff", a.Staff.Router())

		// Mount feedback resources
		r.Mount("/feedback", a.Feedback.Router())

		// Mount suggestions resources
		r.Mount("/suggestions", a.Suggestions.Router())

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

		// Mount user context resources
		r.Mount("/me", a.UserContext.Router())

		// Mount substitutions resources
		r.Mount("/substitutions", a.Substitutions.Router())

		// Mount database resources
		r.Mount("/database", a.Database.Router())

		// Mount import resources (CSV/Excel import endpoints)
		r.Mount("/import", a.Import.Router())

		// Mount SSE resources (Server-Sent Events for real-time updates)
		r.Mount("/sse", a.SSE.Router())

		// Mount time-tracking resources
		r.Mount("/time-tracking", a.TimeTracking.Router())

		// Mount timetable resources
		r.Mount("/timetable", a.Timetable.Router())

		// Mount admin resources
		r.Mount("/admin/grade-transitions", a.GradeTransitions.Router())

		// Mount platform resources (user-facing announcements)
		r.Mount("/platform", a.Platform.Router())

		// Add other resource routes here as they are implemented
	})

	// Mount operator dashboard routes at root level (separate from tenant API)
	// Apply the same auth rate limiter to operator login for brute-force protection
	if rateLimitEnabled && authRateLimiter != nil {
		a.Operator.SetAuthRateLimiter(authRateLimiter.Middleware())
	}
	if rateLimitEnabled && emailConfirmLimiter != nil {
		a.Operator.SetEmailConfirmRateLimiter(emailConfirmLimiter.Middleware())
	}
	if rateLimitEnabled && invitationLimiter != nil {
		a.Operator.SetInvitationRateLimiter(invitationLimiter.Middleware())
	}
	a.Router.Mount("/operator", a.Operator.Router())
}
