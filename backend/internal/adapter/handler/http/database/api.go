package database

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	databaseSvc "github.com/moto-nrw/project-phoenix/services/database"
)

// Resource defines the database API resource
type Resource struct {
	DatabaseService databaseSvc.DatabaseService
}

// NewResource creates a new database resource
func NewResource(databaseService databaseSvc.DatabaseService) *Resource {
	return &Resource{
		DatabaseService: databaseService,
	}
}

// Router returns a configured router for database endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustTokenAuth()

	// Protected routes that require authentication and admin permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Stats endpoint - requires system:manage permission (admin only)
		r.With(authorize.RequiresPermission("system:manage")).Get("/stats", rs.getStats)
	})

	return r
}

// getStats returns database statistics based on user permissions
func (rs *Resource) getStats(w http.ResponseWriter, r *http.Request) {
	stats, err := rs.DatabaseService.GetStats(r.Context())
	if err != nil {
		logger.Logger.WithError(err).Error("Error getting database stats")
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Return the stats response directly - it already includes permissions
	render.JSON(w, r, stats)
}
