package database

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/tenant"
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
// Note: Authentication is handled by tenant middleware in base.go when TENANT_AUTH_ENABLED=true
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Stats endpoint - admin only (no permission constant for system:manage)
	// Admin check is done in handler
	r.Get("/stats", rs.getStats)

	return r
}

// getStats returns database statistics based on user permissions
// Only admin users can access this endpoint
func (rs *Resource) getStats(w http.ResponseWriter, r *http.Request) {
	// Admin-only check: system:manage doesn't exist in BetterAuth permissions
	// Per WP6 spec: use tenant.IsAdmin(ctx) instead
	if !tenant.IsAdmin(r.Context()) {
		common.RenderError(w, r, common.ErrorForbidden(nil))
		return
	}

	stats, err := rs.DatabaseService.GetStats(r.Context())
	if err != nil {
		log.Printf("Error getting database stats: %v", err)
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Return the stats response directly - it already includes permissions
	render.JSON(w, r, stats)
}

// =============================================================================
// HANDLER ACCESSOR METHODS (for testing)
// =============================================================================

// GetStatsHandler returns the getStats handler
func (rs *Resource) GetStatsHandler() http.HandlerFunc { return rs.getStats }
