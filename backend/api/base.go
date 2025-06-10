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
	iotAPI "github.com/moto-nrw/project-phoenix/api/iot"
	roomsAPI "github.com/moto-nrw/project-phoenix/api/rooms"
	schedulesAPI "github.com/moto-nrw/project-phoenix/api/schedules"
	staffAPI "github.com/moto-nrw/project-phoenix/api/staff"
	studentsAPI "github.com/moto-nrw/project-phoenix/api/students"
	substitutionsAPI "github.com/moto-nrw/project-phoenix/api/substitutions"
	usercontextAPI "github.com/moto-nrw/project-phoenix/api/usercontext"
	usersAPI "github.com/moto-nrw/project-phoenix/api/users"
	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	customMiddleware "github.com/moto-nrw/project-phoenix/middleware"
	"github.com/moto-nrw/project-phoenix/services"
)

// API represents the API structure
type API struct {
	Services *services.Factory
	Router   chi.Router

	// API Resources
	Auth          *authAPI.Resource
	Rooms         *roomsAPI.Resource
	Students      *studentsAPI.Resource
	Groups        *groupsAPI.Resource
	Activities    *activitiesAPI.Resource
	Staff         *staffAPI.Resource
	Feedback      *feedbackAPI.Resource
	Schedules     *schedulesAPI.Resource
	Config        *configAPI.Resource
	Active        *activeAPI.Resource
	IoT           *iotAPI.Resource
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

	// Initialize service factory with repository factory
	serviceFactory, err := services.NewFactory(repoFactory, db)
	if err != nil {
		return nil, err
	}

	// Create API instance
	api := &API{
		Services: serviceFactory,
		Router:   chi.NewRouter(),
	}

	// Setup router middleware
	api.Router.Use(middleware.RequestID)
	api.Router.Use(middleware.RealIP)
	api.Router.Use(middleware.Logger)
	api.Router.Use(middleware.Recoverer)

	// Add security headers to all responses
	api.Router.Use(customMiddleware.SecurityHeaders)

	// Setup CORS if enabled
	if enableCORS {
		// Get allowed origins from environment variable
		// Default to "*" for backwards compatibility if not specified
		// This maintains current behavior while allowing restriction in production
		allowedOrigins := []string{"*"}
		if originsEnv := os.Getenv("CORS_ALLOWED_ORIGINS"); originsEnv != "" {
			// Parse comma-separated origins
			allowedOrigins = strings.Split(originsEnv, ",")
			for i := range allowedOrigins {
				allowedOrigins[i] = strings.TrimSpace(allowedOrigins[i])
			}
		}

		api.Router.Use(cors.Handler(cors.Options{
			AllowedOrigins:   allowedOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Staff-PIN"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: true,
			MaxAge:           300,
		}))
	}

	// Setup security logging if enabled
	var securityLogger *customMiddleware.SecurityLogger
	if securityLogging := os.Getenv("SECURITY_LOGGING_ENABLED"); securityLogging == "true" {
		securityLogger = customMiddleware.NewSecurityLogger()
		api.Router.Use(customMiddleware.SecurityLoggingMiddleware(securityLogger))
	}

	// Setup rate limiting if enabled
	if rateLimitEnabled := os.Getenv("RATE_LIMIT_ENABLED"); rateLimitEnabled == "true" {
		// Get rate limit configuration from environment
		generalLimit := 60 // default: 60 requests per minute
		if limit := os.Getenv("RATE_LIMIT_REQUESTS_PER_MINUTE"); limit != "" {
			if parsed, err := strconv.Atoi(limit); err == nil && parsed > 0 {
				generalLimit = parsed
			}
		}

		generalBurst := 10 // default burst
		if burst := os.Getenv("RATE_LIMIT_BURST"); burst != "" {
			if parsed, err := strconv.Atoi(burst); err == nil && parsed > 0 {
				generalBurst = parsed
			}
		}

		// Create general rate limiter for all endpoints
		generalRateLimiter := customMiddleware.NewRateLimiter(generalLimit, generalBurst)
		if securityLogger != nil {
			generalRateLimiter.SetLogger(securityLogger)
		}
		api.Router.Use(generalRateLimiter.Middleware())
	}

	// Initialize API resources
	api.Auth = authAPI.NewResource(api.Services.Auth)
	api.Rooms = roomsAPI.NewResource(api.Services.Facilities)
	api.Students = studentsAPI.NewResource(api.Services.Users, repoFactory.Student, api.Services.Education, api.Services.UserContext, api.Services.Active)
	api.Groups = groupsAPI.NewResource(api.Services.Education, api.Services.Active, api.Services.Users, api.Services.UserContext, repoFactory.Student)
	api.Activities = activitiesAPI.NewResource(api.Services.Activities, api.Services.Schedule, api.Services.Users, api.Services.UserContext)
	api.Staff = staffAPI.NewResource(api.Services.Users, api.Services.Education, api.Services.Auth)
	api.Feedback = feedbackAPI.NewResource(api.Services.Feedback)
	api.Schedules = schedulesAPI.NewResource(api.Services.Schedule)
	api.Config = configAPI.NewResource(api.Services.Config)
	api.Active = activeAPI.NewResource(api.Services.Active)
	api.IoT = iotAPI.NewResource(api.Services.IoT, api.Services.Users, api.Services.Active, api.Services.Activities, api.Services.Config, api.Services.Facilities)
	api.Users = usersAPI.NewResource(api.Services.Users)
	api.UserContext = usercontextAPI.NewResource(api.Services.UserContext)
	api.Substitutions = substitutionsAPI.NewResource(api.Services.Education)
	api.Database = databaseAPI.NewResource(api.Services.Database)

	// Register routes with rate limiting
	api.registerRoutesWithRateLimiting()

	return api, nil
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
	if securityLogging := os.Getenv("SECURITY_LOGGING_ENABLED"); securityLogging == "true" {
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

	// Other API routes under /api prefix for organization
	a.Router.Route("/api", func(r chi.Router) {
		// Mount room resources
		r.Mount("/rooms", a.Rooms.Router())

		// Mount student resources
		r.Mount("/students", a.Students.Router())

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

		// Add other resource routes here as they are implemented
	})
}
