package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	activeAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/active"
	activitiesAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/activities"
	authAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/auth"
	configAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/config"
	databaseAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/database"
	feedbackAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/feedback"
	groupsAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/groups"
	guardiansAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/guardians"
	importAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/import"
	iotAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot"
	roomsAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/rooms"
	schedulesAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/schedules"
	sseAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/sse"
	staffAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/staff"
	studentsAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/students"
	substitutionsAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/substitutions"
	usercontextAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/usercontext"
	usersAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/users"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	customMiddleware "github.com/moto-nrw/project-phoenix/internal/adapter/middleware"
	"github.com/moto-nrw/project-phoenix/internal/adapter/realtime"
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/database"
	"github.com/moto-nrw/project-phoenix/internal/adapter/services"
	"github.com/moto-nrw/project-phoenix/internal/adapter/storage"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	"github.com/spf13/viper"
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
	fileStorage, err := initFileStorage()
	if err != nil {
		return nil, err
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
		if err := setupCORS(api.Router); err != nil {
			return nil, err
		}
	}
	securityLogger := setupSecurityLogging(api.Router)
	if err := setupRateLimiting(api.Router, securityLogger); err != nil {
		return nil, err
	}

	// Initialize API resources
	initializeAPIResources(api)

	// Register routes with rate limiting
	if err := api.registerRoutesWithRateLimiting(); err != nil {
		return nil, err
	}

	return api, nil
}

func initFileStorage() (port.FileStorage, error) {
	backend := strings.ToLower(strings.TrimSpace(os.Getenv("STORAGE_BACKEND")))
	if backend == "" {
		return nil, fmt.Errorf("STORAGE_BACKEND environment variable is required")
	}

	switch backend {
	case "disabled", "none", "off":
		logger.Logger.Info("storage: disabled by configuration")
		return nil, nil
	case "memory":
		appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
		if appEnv == "" {
			return nil, fmt.Errorf("APP_ENV environment variable is required for memory storage")
		}
		if appEnv == "production" {
			return nil, fmt.Errorf("memory storage is not allowed in production; configure a remote storage backend")
		}
		if appEnv != "development" && appEnv != "test" {
			return nil, fmt.Errorf("memory storage is only allowed in development or test (APP_ENV=%s)", appEnv)
		}
		if !strings.EqualFold(strings.TrimSpace(os.Getenv("STORAGE_ALLOW_MEMORY")), "true") {
			return nil, fmt.Errorf("STORAGE_ALLOW_MEMORY environment variable must be set to true to use memory storage")
		}

		publicPrefix := strings.TrimSpace(os.Getenv("STORAGE_PUBLIC_URL_PREFIX"))
		if publicPrefix == "" {
			return nil, fmt.Errorf("STORAGE_PUBLIC_URL_PREFIX environment variable is required for memory storage")
		}

		avatarStorage, err := storage.NewMemoryStorage(port.StorageConfig{
			PublicURLPrefix: publicPrefix,
		}, logger.Logger)
		if err != nil {
			return nil, err
		}
		return avatarStorage, nil
	case "s3", "minio":
		publicPrefix := strings.TrimSpace(os.Getenv("STORAGE_PUBLIC_URL_PREFIX"))
		if publicPrefix == "" {
			return nil, fmt.Errorf("STORAGE_PUBLIC_URL_PREFIX environment variable is required for S3 storage")
		}
		bucket := strings.TrimSpace(os.Getenv("STORAGE_S3_BUCKET"))
		if bucket == "" {
			return nil, fmt.Errorf("STORAGE_S3_BUCKET environment variable is required for S3 storage")
		}
		region := strings.TrimSpace(os.Getenv("STORAGE_S3_REGION"))
		if region == "" {
			return nil, fmt.Errorf("STORAGE_S3_REGION environment variable is required for S3 storage")
		}

		endpoint := strings.TrimSpace(os.Getenv("STORAGE_S3_ENDPOINT"))
		accessKeyID := strings.TrimSpace(os.Getenv("STORAGE_S3_ACCESS_KEY_ID"))
		secretAccessKey := strings.TrimSpace(os.Getenv("STORAGE_S3_SECRET_ACCESS_KEY"))
		sessionToken := strings.TrimSpace(os.Getenv("STORAGE_S3_SESSION_TOKEN"))
		keyPrefix := strings.TrimSpace(os.Getenv("STORAGE_S3_PREFIX"))
		forcePathStyle := strings.EqualFold(strings.TrimSpace(os.Getenv("STORAGE_S3_FORCE_PATH_STYLE")), "true")

		if backend == "minio" {
			if endpoint == "" {
				return nil, fmt.Errorf("STORAGE_S3_ENDPOINT environment variable is required for MinIO storage")
			}
			if accessKeyID == "" || secretAccessKey == "" {
				return nil, fmt.Errorf("STORAGE_S3_ACCESS_KEY_ID and STORAGE_S3_SECRET_ACCESS_KEY are required for MinIO storage")
			}
			forcePathStyle = true
		}

		avatarStorage, err := storage.NewS3Storage(context.Background(), storage.S3Config{
			Bucket:          bucket,
			Region:          region,
			Endpoint:        endpoint,
			PublicURLPrefix: publicPrefix,
			KeyPrefix:       keyPrefix,
			AccessKeyID:     accessKeyID,
			SecretAccessKey: secretAccessKey,
			SessionToken:    sessionToken,
			ForcePathStyle:  forcePathStyle,
		}, logger.Logger)
		if err != nil {
			return nil, err
		}
		return avatarStorage, nil
	default:
		return nil, fmt.Errorf("unsupported STORAGE_BACKEND %q", backend)
	}
}

// setupBasicMiddleware configures basic router middleware
func setupBasicMiddleware(router chi.Router) {
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(customMiddleware.RequestLogger(logger.Logger))
	router.Use(middleware.Recoverer)
	router.Use(customMiddleware.SecurityHeaders)
}

// setupCORS configures CORS middleware with allowed origins from environment
func setupCORS(router chi.Router) error {
	allowedOrigins, err := parseAllowedOrigins()
	if err != nil {
		return err
	}
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Staff-PIN", "X-Staff-ID", "X-Device-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	return nil
}

// parseAllowedOrigins parses CORS_ALLOWED_ORIGINS environment variable
func parseAllowedOrigins() ([]string, error) {
	originsEnv := strings.TrimSpace(viper.GetString("cors_allowed_origins"))
	if originsEnv == "" {
		appEnv := strings.ToLower(strings.TrimSpace(viper.GetString("app_env")))
		if appEnv == "" {
			return nil, fmt.Errorf("APP_ENV environment variable is required to determine CORS defaults")
		}
		if appEnv == "production" {
			return nil, fmt.Errorf("CORS_ALLOWED_ORIGINS environment variable is required in production when CORS is enabled")
		}
		return []string{"*"}, nil
	}

	rawOrigins := strings.Split(originsEnv, ",")
	origins := make([]string, 0, len(rawOrigins))
	for _, origin := range rawOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins = append(origins, origin)
		}
	}

	if len(origins) == 0 {
		return nil, fmt.Errorf("CORS_ALLOWED_ORIGINS must contain at least one origin")
	}
	return origins, nil
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
func setupRateLimiting(router chi.Router, securityLogger *customMiddleware.SecurityLogger) error {
	if os.Getenv("RATE_LIMIT_ENABLED") != "true" {
		return nil
	}

	generalLimit, err := parseRequiredPositiveInt("RATE_LIMIT_REQUESTS_PER_MINUTE")
	if err != nil {
		return err
	}
	generalBurst, err := parseRequiredPositiveInt("RATE_LIMIT_BURST")
	if err != nil {
		return err
	}

	generalRateLimiter := customMiddleware.NewRateLimiter(generalLimit, generalBurst)
	if securityLogger != nil {
		generalRateLimiter.SetLogger(securityLogger)
	}
	router.Use(generalRateLimiter.Middleware())
	return nil
}

// parseRequiredPositiveInt parses a required positive integer from environment variables.
func parseRequiredPositiveInt(envVar string) (int, error) {
	valueStr := strings.TrimSpace(os.Getenv(envVar))
	if valueStr == "" {
		return 0, fmt.Errorf("%s environment variable is required", envVar)
	}

	parsed, err := strconv.Atoi(valueStr)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", envVar)
	}
	return parsed, nil
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
func (a *API) registerRoutesWithRateLimiting() error {
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
		authLimit, err := parseRequiredPositiveInt("RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE")
		if err != nil {
			return err
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

	return nil
}
