package api

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	activeAPI "github.com/moto-nrw/project-phoenix/api/active"
	activitiesAPI "github.com/moto-nrw/project-phoenix/api/activities"
	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
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
	usercontextAPI "github.com/moto-nrw/project-phoenix/api/usercontext"
	usersAPI "github.com/moto-nrw/project-phoenix/api/users"
	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/adapter/realtime"
	"github.com/moto-nrw/project-phoenix/internal/adapter/storage"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	"github.com/moto-nrw/project-phoenix/logging"
	customMiddleware "github.com/moto-nrw/project-phoenix/internal/adapter/middleware"
	"github.com/moto-nrw/project-phoenix/services"
)

// API represents the API structure
type API struct {
	Services    *services.Factory
	Router      chi.Router
	RealtimeHub *realtime.Hub // SSE hub for real-time client management

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

	// Initialize file storage adapter for avatars (Hexagonal Architecture: adapter created here, injected into services)
	var fileStorage port.FileStorage
	avatarStorage, err := storage.NewLocalStorage(port.StorageConfig{
		BasePath:        "public/uploads",
		PublicURLPrefix: "/uploads",
	}, logging.Logger)
	if err != nil {
		logging.Logger.WithError(err).Warn("storage: failed to initialize local storage for avatars")
	} else {
		fileStorage = avatarStorage
	}

	// Create realtime hub for SSE broadcasting (Hexagonal Architecture: adapter created here, injected into services)
	realtimeHub := realtime.NewHub()

	// Initialize service factory with repository factory, file storage, and broadcaster
	serviceFactory, err := services.NewFactory(repoFactory, db, fileStorage, realtimeHub)
	if err != nil {
		return nil, err
	}

	// Create API instance
	api := &API{
		Services:    serviceFactory,
		Router:      chi.NewRouter(),
		RealtimeHub: realtimeHub, // Store hub for SSE resource access
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
	initializeAPIResources(api)

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

	securityLogger := customMiddleware.NewSecurityLogger(nil)
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
func initializeAPIResources(api *API) {
	api.Auth = authAPI.NewResource(api.Services.Auth, api.Services.Invitation)
	api.Rooms = roomsAPI.NewResource(api.Services.Facilities)
	api.Students = studentsAPI.NewResource(api.Services.Users, api.Services.Student, api.Services.Education, api.Services.UserContext, api.Services.Active, api.Services.IoT)
	api.Groups = groupsAPI.NewResource(api.Services.Education, api.Services.Active, api.Services.Users, api.Services.UserContext, api.Services.Student)
	api.Guardians = guardiansAPI.NewResource(api.Services.Guardian, api.Services.Users, api.Services.Education, api.Services.UserContext, api.Services.Student)
	api.Import = importAPI.NewResource(api.Services.Import)
	api.Activities = activitiesAPI.NewResource(api.Services.Activities, api.Services.Schedule, api.Services.Users, api.Services.UserContext)
	api.Staff = staffAPI.NewResource(api.Services.Users, api.Services.Education, api.Services.Auth)
	api.Feedback = feedbackAPI.NewResource(api.Services.Feedback)
	api.Schedules = schedulesAPI.NewResource(api.Services.Schedule)
	api.Config = configAPI.NewResource(api.Services.Config, api.Services.ActiveCleanup)
	api.Active = activeAPI.NewResource(api.Services.Active, api.Services.Users, api.Services.Facilities)
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
	api.SSE = sseAPI.NewResource(api.RealtimeHub, api.Services.Active, api.Services.Users, api.Services.UserContext)
	api.Users = usersAPI.NewResource(api.Services.Users)
	api.UserContext = usercontextAPI.NewResource(api.Services.UserContext)
	api.Substitutions = substitutionsAPI.NewResource(api.Services.Education)
	api.Database = databaseAPI.NewResource(api.Services.Database)
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
		securityLogger = customMiddleware.NewSecurityLogger(nil)
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

		// Mount schedule resources
		r.Mount("/schedules", a.Schedules.Router())

		// Mount config resources
		r.Mount("/config", a.Config.Router())

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

		// Add other resource routes here as they are implemented
	})
}
