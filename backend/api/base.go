package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	activitiesAPI "github.com/moto-nrw/project-phoenix/api/activities"
	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	groupsAPI "github.com/moto-nrw/project-phoenix/api/groups"
	roomsAPI "github.com/moto-nrw/project-phoenix/api/rooms"
	studentsAPI "github.com/moto-nrw/project-phoenix/api/students"
	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
)

// API represents the API structure
type API struct {
	Services *services.Factory
	Router   chi.Router

	// API Resources
	Auth       *authAPI.Resource
	Rooms      *roomsAPI.Resource
	Students   *studentsAPI.Resource
	Groups     *groupsAPI.Resource
	Activities *activitiesAPI.Resource
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
		api.Router.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{"*"},
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
	api.Students = studentsAPI.NewResource(api.Services.Users, repoFactory.Student)
	api.Groups = groupsAPI.NewResource(api.Services.Education)
	api.Activities = activitiesAPI.NewResource(api.Services.Activities)

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
		w.Write([]byte("MOTO API - Phoenix Project"))
	})

	a.Router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
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

		// Add other resource routes here as they are implemented
	})
}
