package database

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	databaseSvc "github.com/moto-nrw/project-phoenix/services/database"
)

// Resource defines the database API resource
type Resource struct {
	DatabaseService databaseSvc.DatabaseService
	db              *bun.DB
	logger          *slog.Logger
}

// NewResource creates a new database resource
func NewResource(databaseService databaseSvc.DatabaseService, db *bun.DB, logger *slog.Logger) *Resource {
	return &Resource{
		DatabaseService: databaseService,
		db:              db,
		logger:          logger,
	}
}

func (rs *Resource) getLogger() *slog.Logger {
	if rs.logger != nil {
		return rs.logger
	}
	return slog.Default()
}

// Router returns a configured router for database endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and admin permissions
	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		// Stats endpoint - requires system:manage permission (admin only)
		r.With(common.RequiresPermission("system:manage"), withTx).Get("/stats", rs.getStats)
	})

	return r
}

// getStats returns database statistics based on user permissions
func (rs *Resource) getStats(w http.ResponseWriter, r *http.Request) {
	stats, err := rs.DatabaseService.GetStats(r.Context())
	if err != nil {
		rs.getLogger().Error("failed to get database stats",
			"error", err,
		)
		common.RenderError(w, r, common.ErrorInternalServerWrap("Internal server error", err))
		return
	}

	// Return the stats response directly - it already includes permissions
	render.JSON(w, r, stats)
}
