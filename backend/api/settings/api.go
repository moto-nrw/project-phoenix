package settings

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
)

// Resource defines the hierarchical settings API resource
type Resource struct {
	SettingsService configSvc.HierarchicalSettingsService
}

// NewResource creates a new hierarchical settings resource
func NewResource(settingsService configSvc.HierarchicalSettingsService) *Resource {
	return &Resource{
		SettingsService: settingsService,
	}
}

// Router returns a configured router for hierarchical settings endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// All routes require authentication
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Tab-based routes (available to authenticated users)
		r.Get("/tabs", rs.listTabs)
		r.Get("/tabs/{tab}", rs.getTabSettings)

		// Definition management (admin only)
		r.With(authorize.RequiresPermission(permissions.ConfigManage)).
			Get("/definitions", rs.listDefinitions)
		r.With(authorize.RequiresPermission(permissions.ConfigManage)).
			Post("/sync", rs.syncDefinitions)

		// Value operations
		r.Get("/values/{key}", rs.getValue)
		r.With(authorize.RequiresPermission(permissions.ConfigUpdate)).
			Put("/values/{key}", rs.setValue)
		r.With(authorize.RequiresPermission(permissions.ConfigUpdate)).
			Delete("/values/{key}", rs.deleteValue)

		// Object reference options
		r.Get("/values/{key}/options", rs.getObjectRefOptions)

		// Audit history
		r.With(authorize.RequiresPermission(permissions.ConfigRead)).
			Get("/values/{key}/history", rs.getSettingHistory)
		r.With(authorize.RequiresPermission(permissions.ConfigManage)).
			Get("/audit/recent", rs.getRecentChanges)

		// Soft delete management (admin only)
		r.With(authorize.RequiresPermission(permissions.ConfigManage)).
			Post("/values/{key}/restore", rs.restoreValue)
		r.With(authorize.RequiresPermission(permissions.ConfigManage)).
			Post("/purge", rs.purgeDeleted)

		// Action endpoints
		r.With(authorize.RequiresPermission(permissions.ConfigManage)).
			Post("/actions/{key}/execute", rs.executeAction)
		r.With(authorize.RequiresPermission(permissions.ConfigRead)).
			Get("/actions/{key}/history", rs.getActionHistory)
		r.With(authorize.RequiresPermission(permissions.ConfigManage)).
			Get("/actions/recent", rs.getRecentActionExecutions)
	})

	return r
}
