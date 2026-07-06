package database

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	databaseSvc "github.com/moto-nrw/project-phoenix/services/database"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Resource defines the database API resource
type Resource struct {
	DatabaseService databaseSvc.DatabaseService
	db              *bun.DB
}

// NewResource creates a new database resource
func NewResource(databaseService databaseSvc.DatabaseService, db *bun.DB) *Resource {
	return &Resource{
		DatabaseService: databaseService,
		db:              db,
	}
}

// Router returns a configured router for database endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustNewTokenAuth()

	// Protected routes that require authentication and admin permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)

		// Stats endpoint - requires system:manage permission (admin only)
		withTx := tenant.TenantTxMiddleware(rs.db)
		r.With(authorize.RequiresPermission("system:manage"), withTx).Get("/stats", rs.getStats)
	})

	return r
}

// getStats returns database statistics based on user permissions
func (rs *Resource) getStats(w http.ResponseWriter, r *http.Request) {
	stats, err := rs.DatabaseService.GetStats(r.Context())
	if err != nil {
		slog.Default().Error("failed to get database stats", slog.String("error", err.Error()))
		common.RenderError(w, r, common.ErrorInternalServerWrap("Internal server error", err))
		return
	}

	// Return the stats response directly - it already includes permissions
	render.JSON(w, r, stats)
}
