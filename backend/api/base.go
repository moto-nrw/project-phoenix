package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	activeAPI "github.com/moto-nrw/project-phoenix/api/active"
	activitiesAPI "github.com/moto-nrw/project-phoenix/api/activities"
	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	configAPI "github.com/moto-nrw/project-phoenix/api/config"
	feedbackAPI "github.com/moto-nrw/project-phoenix/api/feedback"
	groupsAPI "github.com/moto-nrw/project-phoenix/api/groups"
	iotAPI "github.com/moto-nrw/project-phoenix/api/iot"
	roomsAPI "github.com/moto-nrw/project-phoenix/api/rooms"
	schedulesAPI "github.com/moto-nrw/project-phoenix/api/schedules"
	staffAPI "github.com/moto-nrw/project-phoenix/api/staff"
	studentsAPI "github.com/moto-nrw/project-phoenix/api/students"
	usercontextAPI "github.com/moto-nrw/project-phoenix/api/usercontext"
	usersAPI "github.com/moto-nrw/project-phoenix/api/users"
	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
)

// API represents the API structure
type API struct {
	Services *services.Factory
	Router   chi.Router

	// API Resources
	Auth        *authAPI.Resource
	Rooms       *roomsAPI.Resource
	Students    *studentsAPI.Resource
	Groups      *groupsAPI.Resource
	Activities  *activitiesAPI.Resource
	Staff       *staffAPI.Resource
	Feedback    *feedbackAPI.Resource
	Schedules   *schedulesAPI.Resource
	Config      *configAPI.Resource
	Active      *activeAPI.Resource
	IoT         *iotAPI.Resource
	Users       *usersAPI.Resource
	UserContext *usercontextAPI.Resource
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
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: true,
			MaxAge:           300,
		}))
	}

	// Initialize API resources
	api.Auth = authAPI.NewResource(api.Services.Auth)
	api.Rooms = roomsAPI.NewResource(api.Services.Facilities)
	api.Students = studentsAPI.NewResource(api.Services.Users, repoFactory.Student, api.Services.Education, api.Services.UserContext)
	api.Groups = groupsAPI.NewResource(api.Services.Education)
	api.Activities = activitiesAPI.NewResource(api.Services.Activities, api.Services.Schedule, api.Services.Users)
	api.Staff = staffAPI.NewResource(api.Services.Users, api.Services.Education, api.Services.Auth)
	api.Feedback = feedbackAPI.NewResource(api.Services.Feedback)
	api.Schedules = schedulesAPI.NewResource(api.Services.Schedule)
	api.Config = configAPI.NewResource(api.Services.Config)
	api.Active = activeAPI.NewResource(api.Services.Active)
	api.IoT = iotAPI.NewResource(api.Services.IoT)
	api.Users = usersAPI.NewResource(api.Services.Users)
	api.UserContext = usercontextAPI.NewResource(api.Services.UserContext)

	// Register routes
	api.registerRoutes()

	return api, nil
}

// ServeHTTP implements the http.Handler interface for the API
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.Router.ServeHTTP(w, r)
}

// registerRoutes registers all API routes
func (a *API) registerRoutes() {
	a.Router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("MOTO API - Phoenix Project"))
	})

	a.Router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})

	// Mount API resources
	// Auth routes mounted at root level to match frontend expectations
	a.Router.Mount("/auth", a.Auth.Router())

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

		// Add other resource routes here as they are implemented
	})
}
