package config

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	"github.com/moto-nrw/project-phoenix/internal/core/service/active"
	configSvc "github.com/moto-nrw/project-phoenix/internal/core/service/config"
)

// Resource defines the config API resource
type Resource struct {
	ConfigService  configSvc.Service
	CleanupService active.CleanupService
}

// NewResource creates a new config resource
func NewResource(configService configSvc.Service, cleanupService active.CleanupService) *Resource {
	return &Resource{
		ConfigService:  configService,
		CleanupService: cleanupService,
	}
}

// Router returns a configured router for config endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Read operations require config:read permission
		r.With(authorize.RequiresPermission(permissions.ConfigRead)).Get("/", rs.listSettings)
		r.With(authorize.RequiresPermission(permissions.ConfigRead)).Get("/{id}", rs.getSetting)
		r.With(authorize.RequiresPermission(permissions.ConfigRead)).Get("/key/{key}", rs.getSettingByKey)
		r.With(authorize.RequiresPermission(permissions.ConfigRead)).Get("/category/{category}", rs.getSettingsByCategory)
		r.With(authorize.RequiresPermission(permissions.ConfigRead)).Get("/system-status", rs.getSystemStatus)
		r.With(authorize.RequiresPermission(permissions.ConfigRead)).Get("/defaults", rs.getDefaultSettings)

		// Write operations require config:update or config:manage permission
		r.With(authorize.RequiresPermission(permissions.ConfigUpdate)).Post("/", rs.createSetting)
		r.With(authorize.RequiresPermission(permissions.ConfigUpdate)).Put("/{id}", rs.updateSetting)
		r.With(authorize.RequiresPermission(permissions.ConfigUpdate)).Patch("/key/{key}", rs.updateSettingValue)
		r.With(authorize.RequiresPermission(permissions.ConfigManage)).Delete("/{id}", rs.deleteSetting)

		// Bulk and system operations require config:manage permission
		r.With(authorize.RequiresPermission(permissions.ConfigManage)).Post("/import", rs.importSettings)
		r.With(authorize.RequiresPermission(permissions.ConfigManage)).Post("/initialize-defaults", rs.initializeDefaults)

		// Data retention settings
		r.With(authorize.RequiresPermission(permissions.ConfigRead)).Get("/retention", rs.getRetentionSettings)
		r.With(authorize.RequiresPermission(permissions.ConfigUpdate)).Put("/retention", rs.updateRetentionSettings)
		r.With(authorize.RequiresPermission(permissions.ConfigManage)).Post("/retention/cleanup", rs.triggerRetentionCleanup)
		r.With(authorize.RequiresPermission(permissions.ConfigRead)).Get("/retention/stats", rs.getRetentionStats)
	})

	return r
}
