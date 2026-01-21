package api

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/uptrace/bun"

	activeAPI "github.com/moto-nrw/project-phoenix/api/active"
	activitiesAPI "github.com/moto-nrw/project-phoenix/api/activities"
	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	configAPI "github.com/moto-nrw/project-phoenix/api/config"
	databaseAPI "github.com/moto-nrw/project-phoenix/api/database"
	feedbackAPI "github.com/moto-nrw/project-phoenix/api/feedback"
	groupsAPI "github.com/moto-nrw/project-phoenix/api/groups"
	guardiansAPI "github.com/moto-nrw/project-phoenix/api/guardians"
	importAPI "github.com/moto-nrw/project-phoenix/api/import"
	internalAPI "github.com/moto-nrw/project-phoenix/api/internal"
	iotAPI "github.com/moto-nrw/project-phoenix/api/iot"
	roomsAPI "github.com/moto-nrw/project-phoenix/api/rooms"
	schedulesAPI "github.com/moto-nrw/project-phoenix/api/schedules"
	sseAPI "github.com/moto-nrw/project-phoenix/api/sse"
	staffAPI "github.com/moto-nrw/project-phoenix/api/staff"
	studentsAPI "github.com/moto-nrw/project-phoenix/api/students"
	substitutionsAPI "github.com/moto-nrw/project-phoenix/api/substitutions"
	usercontextAPI "github.com/moto-nrw/project-phoenix/api/usercontext"
	usersAPI "github.com/moto-nrw/project-phoenix/api/users"
	"github.com/moto-nrw/project-phoenix/auth/betterauth"
	"github.com/moto-nrw/project-phoenix/auth/tenant"
	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/logging"
	customMiddleware "github.com/moto-nrw/project-phoenix/middleware"
	"github.com/moto-nrw/project-phoenix/services"
)

// API represents the API structure
type API struct {
	Services *services.Factory
	Router   chi.Router

	// Infrastructure for multi-tenancy (WP5)
	db               *bun.DB
	betterAuthClient *betterauth.Client

	// API Resources
	Auth          *authAPI.Resource
	Rooms         *roomsAPI.Resource
	Students      *studentsAPI.Resource
	Groups        *groupsAPI.Resource
	Guardians     *guardiansAPI.Resource
	Import        *importAPI.Resource
	Activities    *activitiesAPI.Resource
	Staff         *staffAPI.Resource
	Feedback      *feedbackAPI.Resource
	Schedules     *schedulesAPI.Resource
	Config        *configAPI.Resource
	Active        *activeAPI.Resource
	IoT           *iotAPI.Resource
	SSE           *sseAPI.Resource
	Users         *usersAPI.Resource
	UserContext   *usercontextAPI.Resource
	Substitutions *substitutionsAPI.Resource
	Database      *databaseAPI.Resource
	Internal      *internalAPI.Resource // Internal API (not exposed externally)
}

// New creates a new API instance
func New(enableCORS bool) (*API, error) {
	// Get database connection
	db, err := database.DBConn()
	if err != nil {
		return nil, err
	}

	// Initialize repository factory with DB connection
	repoFactory := repositories.NewFactory(db)

	// Initialize service factory with repository factory
	serviceFactory, err := services.NewFactory(repoFactory, db)
	if err != nil {
		return nil, err
	}

	// Create BetterAuth client for multi-tenancy (WP5)
	// This client is used by tenant middleware to validate sessions
	betterAuthClient := betterauth.NewClient()

	// Create API instance
	api := &API{
		Services:         serviceFactory,
		Router:           chi.NewRouter(),
		db:               db,
		betterAuthClient: betterAuthClient,
	}

	// Setup router middleware
	setupBasicMiddleware(api.Router)

	// Setup CORS, security logging, and rate limiting
	if enableCORS {
		setupCORS(api.Router)
	}
	securityLogger := setupSecurityLogging(api.Router)
	setupRateLimiting(api.Router, securityLogger)

	// Initialize API resources
	initializeAPIResources(api, repoFactory, db)

	// Register routes with rate limiting
	api.registerRoutesWithRateLimiting()

	return api, nil
}

// setupBasicMiddleware configures basic router middleware
func setupBasicMiddleware(router chi.Router) {
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(customMiddleware.SecurityHeaders)
}

// setupCORS configures CORS middleware with allowed origins from environment
func setupCORS(router chi.Router) {
	allowedOrigins := parseAllowedOrigins()
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Staff-PIN", "X-Staff-ID", "X-Device-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
}

// parseAllowedOrigins parses CORS_ALLOWED_ORIGINS environment variable
func parseAllowedOrigins() []string {
	originsEnv := os.Getenv("CORS_ALLOWED_ORIGINS")
	if originsEnv == "" {
		return []string{"*"}
	}

	origins := strings.Split(originsEnv, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}
	return origins
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
func initializeAPIResources(api *API, repoFactory *repositories.Factory, db *bun.DB) {
	api.Auth = authAPI.NewResource(api.Services.Auth, api.Services.Invitation)
	api.Rooms = roomsAPI.NewResource(api.Services.Facilities)
	api.Students = studentsAPI.NewResource(api.Services.Users, repoFactory.Student, api.Services.Education, api.Services.UserContext, api.Services.Active, api.Services.IoT, repoFactory.PrivacyConsent)
	api.Groups = groupsAPI.NewResource(api.Services.Education, api.Services.Active, api.Services.Users, api.Services.UserContext, repoFactory.Student, repoFactory.GroupSubstitution)
	api.Guardians = guardiansAPI.NewResource(api.Services.Guardian, api.Services.Users, api.Services.Education, api.Services.UserContext, repoFactory.Student)
	api.Import = importAPI.NewResource(api.Services.Import, repoFactory.DataImport, api.Services.Auth)
	api.Activities = activitiesAPI.NewResource(api.Services.Activities, api.Services.Schedule, api.Services.Users, api.Services.UserContext)
	api.Staff = staffAPI.NewResource(api.Services.Users, api.Services.Education, api.Services.Auth, repoFactory.GroupSupervisor)
	api.Feedback = feedbackAPI.NewResource(api.Services.Feedback)
	api.Schedules = schedulesAPI.NewResource(api.Services.Schedule)
	api.Config = configAPI.NewResource(api.Services.Config, api.Services.ActiveCleanup)
	api.Active = activeAPI.NewResource(api.Services.Active, api.Services.Users, api.Services.Auth, db)
	api.IoT = iotAPI.NewResource(iotAPI.ServiceDependencies{
		IoTService:        api.Services.IoT,
		UsersService:      api.Services.Users,
		ActiveService:     api.Services.Active,
		ActivitiesService: api.Services.Activities,
		ConfigService:     api.Services.Config,
		FacilityService:   api.Services.Facilities,
		EducationService:  api.Services.Education,
		FeedbackService:   api.Services.Feedback,
	})
	api.SSE = sseAPI.NewResource(api.Services.RealtimeHub, api.Services.Active, api.Services.Users, api.Services.UserContext, api.Services.Auth)
	api.Users = usersAPI.NewResource(api.Services.Users)
	api.UserContext = usercontextAPI.NewResource(api.Services.UserContext, repoFactory.GroupSubstitution)
	api.Substitutions = substitutionsAPI.NewResource(api.Services.Education)
	api.Database = databaseAPI.NewResource(api.Services.Database)
	api.Internal = internalAPI.NewResource(api.Services.Mailer, api.Services.Dispatcher, api.Services.DefaultFrom)
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
	if rateLimitEnabled {
		// Stricter rate limit for auth endpoints
		authLimit := 5 // default: 5 requests per minute for auth
		if limit := os.Getenv("RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE"); limit != "" {
			if parsed, err := strconv.Atoi(limit); err == nil && parsed > 0 {
				authLimit = parsed
			}
		}
		authRateLimiter = customMiddleware.NewRateLimiter(authLimit, 2) // small burst for auth
		if securityLogger != nil {
			authRateLimiter.SetLogger(securityLogger)
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

	// Mount API resources
	// Auth routes mounted at root level to match frontend expectations
	// Apply stricter rate limiting to auth endpoints if enabled
	if rateLimitEnabled && authRateLimiter != nil {
		a.Router.Route("/auth", func(r chi.Router) {
			r.Use(authRateLimiter.Middleware())
			r.Mount("/", a.Auth.Router())
		})
	} else {
		a.Router.Mount("/auth", a.Auth.Router())
	}

	// Check if tenant auth (BetterAuth) is enabled
	// When enabled, routes use BetterAuth sessions instead of JWT
	// Set TENANT_AUTH_ENABLED=true to enable multi-tenancy auth
	tenantAuthEnabled := os.Getenv("TENANT_AUTH_ENABLED") == "true"

	if tenantAuthEnabled && logging.Logger != nil {
		logging.Logger.Info("Tenant authentication enabled (BetterAuth)")
	}

	// Internal API routes - NO AUTHENTICATION
	// These routes are only accessible from within the Docker network.
	// Security is enforced through Docker network isolation, not authentication.
	// WARNING: Never expose these routes to the public internet!
	a.Router.Mount("/api/internal", a.Internal.Router())

	// Other API routes under /api prefix for organization
	a.Router.Route("/api", func(r chi.Router) {
		// IoT routes use their own authentication (API key + PIN)
		// They MUST be mounted BEFORE tenant middleware is applied
		// This ensures device authentication path remains unchanged
		r.Mount("/iot", a.IoT.Router())

		// All other routes go through tenant middleware when enabled
		// This validates BetterAuth sessions and sets RLS context
		if tenantAuthEnabled {
			r.Group(func(r chi.Router) {
				// Apply tenant middleware to validate BetterAuth sessions
				r.Use(tenant.Middleware(a.betterAuthClient, a.db))

				// Mount resources that require tenant context
				a.mountAuthenticatedRoutes(r)
			})
		} else {
			// Without tenant auth, mount routes directly
			// They still use JWT auth configured in each resource's router
			a.mountAuthenticatedRoutes(r)
		}
	})
}

// mountAuthenticatedRoutes mounts all API routes that require authentication.
// This is called either with or without tenant middleware based on TENANT_AUTH_ENABLED.
func (a *API) mountAuthenticatedRoutes(r chi.Router) {
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

	// Mount schedule resources
	r.Mount("/schedules", a.Schedules.Router())

	// Mount config resources
	r.Mount("/config", a.Config.Router())

	// Mount active resources
	r.Mount("/active", a.Active.Router())

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
}
