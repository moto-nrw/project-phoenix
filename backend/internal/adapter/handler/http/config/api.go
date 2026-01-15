package config

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	configSvc "github.com/moto-nrw/project-phoenix/internal/core/service/config"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/config"
	"github.com/moto-nrw/project-phoenix/internal/core/service/active"
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

// SettingResponse represents a setting API response
type SettingResponse struct {
	ID              int64     `json:"id"`
	Key             string    `json:"key"`
	Value           string    `json:"value"`
	Category        string    `json:"category"`
	Description     string    `json:"description,omitempty"`
	RequiresRestart bool      `json:"requires_restart"`
	RequiresDBReset bool      `json:"requires_db_reset"`
	IsSystemSetting bool      `json:"is_system_setting"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// SettingRequest represents a setting creation/update request
type SettingRequest struct {
	Key             string `json:"key"`
	Value           string `json:"value"`
	Category        string `json:"category"`
	Description     string `json:"description,omitempty"`
	RequiresRestart bool   `json:"requires_restart"`
	RequiresDBReset bool   `json:"requires_db_reset"`
}

// Bind validates the setting request
func (req *SettingRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Key, validation.Required),
		validation.Field(&req.Value, validation.Required),
		validation.Field(&req.Category, validation.Required),
	)
}

// SettingValueRequest represents a setting value update request
type SettingValueRequest struct {
	Value string `json:"value"`
}

// Bind validates the setting value request
func (req *SettingValueRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Value, validation.Required),
	)
}

// ImportSettingsRequest represents a settings import request
type ImportSettingsRequest struct {
	Settings []SettingRequest `json:"settings"`
}

// Bind validates the import settings request
func (req *ImportSettingsRequest) Bind(r *http.Request) error {
	if len(req.Settings) == 0 {
		return errors.New("at least one setting is required")
	}

	// Validate each setting
	for i, setting := range req.Settings {
		if err := (&setting).Bind(r); err != nil {
			return errors.New("invalid setting at index " + strconv.Itoa(i) + ": " + err.Error())
		}
	}

	return nil
}

// SystemStatusResponse represents the system status response
type SystemStatusResponse struct {
	RequiresRestart bool `json:"requires_restart"`
	RequiresDBReset bool `json:"requires_db_reset"`
}

// newSettingResponse converts a setting model to a response object
func newSettingResponse(setting *config.Setting) SettingResponse {
	return SettingResponse{
		ID:              setting.ID,
		Key:             setting.Key,
		Value:           setting.Value,
		Category:        setting.Category,
		Description:     setting.Description,
		RequiresRestart: setting.RequiresRestart,
		RequiresDBReset: setting.RequiresDBReset,
		IsSystemSetting: setting.IsSystemSetting(),
		CreatedAt:       setting.CreatedAt,
		UpdatedAt:       setting.UpdatedAt,
	}
}

// listSettings handles listing all settings with optional filtering
func (rs *Resource) listSettings(w http.ResponseWriter, r *http.Request) {
	// Get filter parameters
	category := r.URL.Query().Get("category")
	search := r.URL.Query().Get("search")

	// Create filters map
	filters := make(map[string]interface{})

	// Apply filters
	if category != "" {
		filters["category"] = category
	}

	if search != "" {
		// This would need repository support for keyword search
		filters["search"] = search
	}

	// Get settings
	settings, err := rs.ConfigService.ListSettings(r.Context(), filters)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	responses := make([]SettingResponse, 0, len(settings))
	for _, setting := range settings {
		responses = append(responses, newSettingResponse(setting))
	}

	common.Respond(w, r, http.StatusOK, responses, "Settings retrieved successfully")
}

// getSetting handles getting a setting by ID
func (rs *Resource) getSetting(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidSettingID)))
		return
	}

	// Get setting
	setting, err := rs.ConfigService.GetSettingByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newSettingResponse(setting), "Setting retrieved successfully")
}

// getSettingByKey handles getting a setting by key
func (rs *Resource) getSettingByKey(w http.ResponseWriter, r *http.Request) {
	// Get key from URL
	key := chi.URLParam(r, "key")
	if key == "" {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("key is required")))
		return
	}

	// Get setting
	setting, err := rs.ConfigService.GetSettingByKey(r.Context(), key)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newSettingResponse(setting), "Setting retrieved successfully")
}

// getSettingsByCategory handles getting settings by category
func (rs *Resource) getSettingsByCategory(w http.ResponseWriter, r *http.Request) {
	// Get category from URL
	category := chi.URLParam(r, "category")
	if category == "" {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("category is required")))
		return
	}

	// Get settings
	settings, err := rs.ConfigService.GetSettingsByCategory(r.Context(), category)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]SettingResponse, 0, len(settings))
	for _, setting := range settings {
		responses = append(responses, newSettingResponse(setting))
	}

	common.Respond(w, r, http.StatusOK, responses, "Settings retrieved successfully")
}

// createSetting handles creating a new setting
func (rs *Resource) createSetting(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &SettingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Create setting
	setting := &config.Setting{
		Key:             req.Key,
		Value:           req.Value,
		Category:        req.Category,
		Description:     req.Description,
		RequiresRestart: req.RequiresRestart,
		RequiresDBReset: req.RequiresDBReset,
	}

	if err := rs.ConfigService.CreateSetting(r.Context(), setting); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newSettingResponse(setting), "Setting created successfully")
}

// updateSetting handles updating an existing setting
func (rs *Resource) updateSetting(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidSettingID)))
		return
	}

	// Parse request
	req := &SettingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get existing setting
	setting, err := rs.ConfigService.GetSettingByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Update fields
	setting.Key = req.Key
	setting.Value = req.Value
	setting.Category = req.Category
	setting.Description = req.Description
	setting.RequiresRestart = req.RequiresRestart
	setting.RequiresDBReset = req.RequiresDBReset

	// Update setting
	if err := rs.ConfigService.UpdateSetting(r.Context(), setting); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newSettingResponse(setting), "Setting updated successfully")
}

// updateSettingValue handles updating only the value of a setting by key
func (rs *Resource) updateSettingValue(w http.ResponseWriter, r *http.Request) {
	// Get key from URL
	key := chi.URLParam(r, "key")
	if key == "" {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("key is required")))
		return
	}

	// Parse request
	req := &SettingValueRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Update setting value
	if err := rs.ConfigService.UpdateSettingValue(r.Context(), key, req.Value); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get updated setting to return
	setting, err := rs.ConfigService.GetSettingByKey(r.Context(), key)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newSettingResponse(setting), "Setting value updated successfully")
}

// deleteSetting handles deleting a setting
func (rs *Resource) deleteSetting(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidSettingID)))
		return
	}

	// Delete setting
	if err := rs.ConfigService.DeleteSetting(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Setting deleted successfully")
}

// importSettings handles importing multiple settings
func (rs *Resource) importSettings(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &ImportSettingsRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Convert requests to models
	settings := make([]*config.Setting, 0, len(req.Settings))
	for _, settingReq := range req.Settings {
		settings = append(settings, &config.Setting{
			Key:             settingReq.Key,
			Value:           settingReq.Value,
			Category:        settingReq.Category,
			Description:     settingReq.Description,
			RequiresRestart: settingReq.RequiresRestart,
			RequiresDBReset: settingReq.RequiresDBReset,
		})
	}

	// Import settings
	errors, err := rs.ConfigService.ImportSettings(r.Context(), settings)
	if err != nil {
		// If we have individual errors, include them in the response
		if len(errors) > 0 {
			errorMessages := make([]string, 0, len(errors))
			for _, e := range errors {
				errorMessages = append(errorMessages, e.Error())
			}
			common.Respond(w, r, http.StatusPartialContent, map[string]interface{}{
				"errors": errorMessages,
			}, "Some settings could not be imported")
			return
		}

		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]interface{}{
		"count": len(settings),
	}, "Settings imported successfully")
}

// initializeDefaults handles initializing default settings
func (rs *Resource) initializeDefaults(w http.ResponseWriter, r *http.Request) {
	// Initialize default settings
	if err := rs.ConfigService.InitializeDefaultSettings(r.Context()); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Default settings initialized successfully")
}

// getSystemStatus handles getting system status
func (rs *Resource) getSystemStatus(w http.ResponseWriter, r *http.Request) {
	// Check if restart is required
	requiresRestart, err := rs.ConfigService.RequiresRestart(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Check if database reset is required
	requiresDBReset, err := rs.ConfigService.RequiresDatabaseReset(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	response := SystemStatusResponse{
		RequiresRestart: requiresRestart,
		RequiresDBReset: requiresDBReset,
	}

	common.Respond(w, r, http.StatusOK, response, "System status retrieved successfully")
}

// getDefaultSettings returns the list of default settings that would be initialized
func (rs *Resource) getDefaultSettings(w http.ResponseWriter, r *http.Request) {
	// This is a read-only endpoint to show what default settings would be created
	defaultSettings := []SettingResponse{
		{
			Key:             "app_name",
			Value:           "Project Phoenix",
			Category:        "system",
			Description:     "The name of the application",
			RequiresRestart: false,
			RequiresDBReset: false,
		},
		{
			Key:             "version",
			Value:           "1.0.0",
			Category:        "system",
			Description:     "The version of the application",
			RequiresRestart: false,
			RequiresDBReset: false,
		},
		{
			Key:             "debug_mode",
			Value:           "false",
			Category:        "system",
			Description:     "Enable debug mode",
			RequiresRestart: true,
			RequiresDBReset: false,
		},
		// Add more default settings as needed
	}

	common.Respond(w, r, http.StatusOK, defaultSettings, "Default settings retrieved successfully")
}
