package config

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// ScopedSettingsResource defines the scoped settings API resource
type ScopedSettingsResource struct {
	Service       configSvc.ScopedSettingsService
	PersonService users.PersonService
}

// NewScopedSettingsResource creates a new scoped settings resource
func NewScopedSettingsResource(service configSvc.ScopedSettingsService, personService users.PersonService) *ScopedSettingsResource {
	return &ScopedSettingsResource{
		Service:       service,
		PersonService: personService,
	}
}

// Router returns a configured router for scoped settings endpoints
func (rs *ScopedSettingsResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	tokenAuth, _ := jwt.NewTokenAuth()

	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// === Definitions (read-only) ===
		r.With(authorize.RequiresPermission(permissions.SettingsRead)).
			Get("/definitions", rs.listDefinitions)
		r.With(authorize.RequiresPermission(permissions.SettingsRead)).
			Get("/definitions/{key}", rs.getDefinition)

		// === System settings (admin) ===
		r.With(authorize.RequiresPermission(permissions.SettingsManage)).
			Get("/system", rs.getSystemSettings)
		r.With(authorize.RequiresPermission(permissions.SettingsManage)).
			Put("/system/{key}", rs.updateSystemSetting)

		// === OG settings ===
		r.With(authorize.RequiresPermission(permissions.SettingsRead)).
			Get("/og/{ogId}", rs.getOGSettings)
		r.With(authorize.RequiresPermission(permissions.SettingsUpdate)).
			Put("/og/{ogId}/{key}", rs.updateOGSetting)
		r.With(authorize.RequiresPermission(permissions.SettingsUpdate)).
			Delete("/og/{ogId}/{key}", rs.resetOGSetting)

		// === User settings (personal preferences) ===
		r.Get("/user/me", rs.getUserSettings)
		r.Put("/user/me/{key}", rs.updateUserSetting)

		// === History ===
		r.With(authorize.RequiresPermission(permissions.SettingsRead)).
			Get("/history", rs.getHistory)
		r.With(authorize.RequiresPermission(permissions.SettingsRead)).
			Get("/og/{ogId}/history", rs.getOGHistory)
		r.With(authorize.RequiresPermission(permissions.SettingsRead)).
			Get("/og/{ogId}/{key}/history", rs.getOGKeyHistory)

		// === Initialization ===
		r.With(authorize.RequiresPermission(permissions.SettingsManage)).
			Post("/initialize", rs.initializeDefinitions)
	})

	return r
}

// === Response Types ===

// DefinitionResponse represents a setting definition API response
type DefinitionResponse struct {
	ID               int64                     `json:"id"`
	Key              string                    `json:"key"`
	Type             string                    `json:"type"`
	DefaultValue     any                       `json:"default_value"`
	Category         string                    `json:"category"`
	Description      string                    `json:"description,omitempty"`
	Validation       *config.Validation        `json:"validation,omitempty"`
	AllowedScopes    []string                  `json:"allowed_scopes"`
	ScopePermissions map[string]string         `json:"scope_permissions"`
	DependsOn        *config.SettingDependency `json:"depends_on,omitempty"`
	GroupName        string                    `json:"group_name,omitempty"`
	SortOrder        int                       `json:"sort_order"`
}

// ResolvedSettingResponse represents a resolved setting API response
type ResolvedSettingResponse struct {
	Key         string                    `json:"key"`
	Value       any                       `json:"value"`
	Type        string                    `json:"type"`
	Category    string                    `json:"category"`
	Description string                    `json:"description,omitempty"`
	GroupName   string                    `json:"group_name,omitempty"`
	Source      *ScopeRefResponse         `json:"source,omitempty"`
	IsDefault   bool                      `json:"is_default"`
	IsActive    bool                      `json:"is_active"`
	CanModify   bool                      `json:"can_modify"`
	DependsOn   *config.SettingDependency `json:"depends_on,omitempty"`
	Validation  *config.Validation        `json:"validation,omitempty"`
}

// ScopeRefResponse represents a scope reference
type ScopeRefResponse struct {
	Type string `json:"type"`
	ID   *int64 `json:"id,omitempty"`
}

// UpdateSettingRequest represents a setting update request
type UpdateSettingRequest struct {
	Value  any    `json:"value"`
	Reason string `json:"reason,omitempty"`
}

// HistoryEntryResponse represents a history entry
type HistoryEntryResponse struct {
	ID         int64  `json:"id"`
	SettingKey string `json:"setting_key"`
	ChangeType string `json:"change_type"`
	OldValue   any    `json:"old_value,omitempty"`
	NewValue   any    `json:"new_value,omitempty"`
	ChangedBy  string `json:"changed_by,omitempty"`
	ChangedAt  string `json:"changed_at"`
	Reason     string `json:"reason,omitempty"`
}

// === Handlers ===

// listDefinitions returns all setting definitions
func (rs *ScopedSettingsResource) listDefinitions(w http.ResponseWriter, r *http.Request) {
	filters := map[string]interface{}{}

	if scope := r.URL.Query().Get("scope"); scope != "" {
		filters["scope"] = scope
	}
	if category := r.URL.Query().Get("category"); category != "" {
		filters["category"] = category
	}
	if group := r.URL.Query().Get("group"); group != "" {
		filters["group"] = group
	}

	defs, err := rs.Service.ListDefinitions(r.Context(), filters)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	responses := make([]DefinitionResponse, len(defs))
	for i, def := range defs {
		responses[i] = mapDefinitionToResponse(def)
	}

	common.Respond(w, r, http.StatusOK, responses, "Definitions retrieved successfully")
}

// getDefinition returns a single definition by key
func (rs *ScopedSettingsResource) getDefinition(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")

	def, err := rs.Service.GetDefinition(r.Context(), key)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}
	if def == nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("definition not found")))
		return
	}

	common.Respond(w, r, http.StatusOK, mapDefinitionToResponse(def), "Definition retrieved successfully")
}

// getSystemSettings returns all system-level settings
func (rs *ScopedSettingsResource) getSystemSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := rs.Service.GetAll(r.Context(), config.NewSystemScope())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	actor := rs.getActorFromContext(r)
	responses := make([]ResolvedSettingResponse, len(settings))
	for i, s := range settings {
		canModify, _ := rs.Service.CanModify(r.Context(), s.Key, config.NewSystemScope(), actor)
		responses[i] = mapResolvedSettingToResponse(s, canModify)
	}

	common.Respond(w, r, http.StatusOK, responses, "System settings retrieved successfully")
}

// updateSystemSetting updates a system-level setting
func (rs *ScopedSettingsResource) updateSystemSetting(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")

	var req UpdateSettingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	actor := rs.getActorFromContext(r)
	if err := rs.Service.Set(r.Context(), key, config.NewSystemScope(), req.Value, actor, r); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Setting updated successfully")
}

// getOGSettings returns all settings for an OG
func (rs *ScopedSettingsResource) getOGSettings(w http.ResponseWriter, r *http.Request) {
	ogID, err := strconv.ParseInt(chi.URLParam(r, "ogId"), 10, 64)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid OG ID")))
		return
	}

	settings, err := rs.Service.GetAll(r.Context(), config.NewOGScope(ogID))
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	actor := rs.getActorFromContext(r)
	responses := make([]ResolvedSettingResponse, len(settings))
	for i, s := range settings {
		canModify, _ := rs.Service.CanModify(r.Context(), s.Key, config.NewOGScope(ogID), actor)
		responses[i] = mapResolvedSettingToResponse(s, canModify)
	}

	common.Respond(w, r, http.StatusOK, responses, "OG settings retrieved successfully")
}

// updateOGSetting updates an OG-level setting
func (rs *ScopedSettingsResource) updateOGSetting(w http.ResponseWriter, r *http.Request) {
	ogID, err := strconv.ParseInt(chi.URLParam(r, "ogId"), 10, 64)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid OG ID")))
		return
	}
	key := chi.URLParam(r, "key")

	var req UpdateSettingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	actor := rs.getActorFromContext(r)

	// Check if user can modify (owner check for OG)
	canModify, err := rs.Service.CanModify(r.Context(), key, config.NewOGScope(ogID), actor)
	if err != nil || !canModify {
		common.RenderError(w, r, ErrorForbidden(errors.New("not authorized to modify this setting")))
		return
	}

	if err := rs.Service.Set(r.Context(), key, config.NewOGScope(ogID), req.Value, actor, r); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Setting updated successfully")
}

// resetOGSetting resets an OG-level setting to inherit from parent
func (rs *ScopedSettingsResource) resetOGSetting(w http.ResponseWriter, r *http.Request) {
	ogID, err := strconv.ParseInt(chi.URLParam(r, "ogId"), 10, 64)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid OG ID")))
		return
	}
	key := chi.URLParam(r, "key")

	actor := rs.getActorFromContext(r)

	// Check if user can modify
	canModify, err := rs.Service.CanModify(r.Context(), key, config.NewOGScope(ogID), actor)
	if err != nil || !canModify {
		common.RenderError(w, r, ErrorForbidden(errors.New("not authorized to modify this setting")))
		return
	}

	if err := rs.Service.Reset(r.Context(), key, config.NewOGScope(ogID), actor, r); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Setting reset successfully")
}

// getUserSettings returns the current user's settings
func (rs *ScopedSettingsResource) getUserSettings(w http.ResponseWriter, r *http.Request) {
	actor := rs.getActorFromContext(r)
	if actor == nil {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("not authenticated")))
		return
	}

	settings, err := rs.Service.GetAll(r.Context(), config.NewUserScope(actor.PersonID))
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	responses := make([]ResolvedSettingResponse, len(settings))
	for i, s := range settings {
		// Users can always modify their own settings
		responses[i] = mapResolvedSettingToResponse(s, true)
	}

	common.Respond(w, r, http.StatusOK, responses, "User settings retrieved successfully")
}

// updateUserSetting updates a user's personal setting
func (rs *ScopedSettingsResource) updateUserSetting(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")

	var req UpdateSettingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	actor := rs.getActorFromContext(r)
	if actor == nil {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("not authenticated")))
		return
	}

	if err := rs.Service.Set(r.Context(), key, config.NewUserScope(actor.PersonID), req.Value, actor, r); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Setting updated successfully")
}

// getHistory returns setting change history with filters
func (rs *ScopedSettingsResource) getHistory(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	scopeType := r.URL.Query().Get("scope_type")
	if scopeType == "" {
		scopeType = string(config.ScopeSystem)
	}

	var scopeID *int64
	if id := r.URL.Query().Get("scope_id"); id != "" {
		if parsed, err := strconv.ParseInt(id, 10, 64); err == nil {
			scopeID = &parsed
		}
	}

	history, err := rs.Service.GetHistoryForScope(r.Context(), config.ScopeRef{
		Type: config.ScopeType(scopeType),
		ID:   scopeID,
	}, limit)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	responses := mapHistoryToResponse(history)
	common.Respond(w, r, http.StatusOK, responses, "History retrieved successfully")
}

// getOGHistory returns history for an OG
func (rs *ScopedSettingsResource) getOGHistory(w http.ResponseWriter, r *http.Request) {
	ogID, err := strconv.ParseInt(chi.URLParam(r, "ogId"), 10, 64)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid OG ID")))
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	history, err := rs.Service.GetHistoryForScope(r.Context(), config.NewOGScope(ogID), limit)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	responses := mapHistoryToResponse(history)
	common.Respond(w, r, http.StatusOK, responses, "History retrieved successfully")
}

// getOGKeyHistory returns history for a specific setting in an OG
func (rs *ScopedSettingsResource) getOGKeyHistory(w http.ResponseWriter, r *http.Request) {
	ogID, err := strconv.ParseInt(chi.URLParam(r, "ogId"), 10, 64)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid OG ID")))
		return
	}
	key := chi.URLParam(r, "key")

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	history, err := rs.Service.GetHistory(r.Context(), key, config.NewOGScope(ogID), limit)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	responses := mapHistoryToResponse(history)
	common.Respond(w, r, http.StatusOK, responses, "History retrieved successfully")
}

// initializeDefinitions syncs code-defined settings to the database
func (rs *ScopedSettingsResource) initializeDefinitions(w http.ResponseWriter, r *http.Request) {
	if err := rs.Service.InitializeDefinitions(r.Context()); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Definitions initialized successfully")
}

// === Helper Functions ===

func (rs *ScopedSettingsResource) getActorFromContext(r *http.Request) *configSvc.Actor {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		return nil
	}

	perms := jwt.PermissionsFromCtx(r.Context())

	actor := &configSvc.Actor{
		AccountID:   int64(claims.ID),
		Permissions: perms,
	}

	// Try to get PersonID from account
	if rs.PersonService != nil {
		person, err := rs.PersonService.FindByAccountID(r.Context(), int64(claims.ID))
		if err == nil && person != nil {
			actor.PersonID = person.ID
		}
	}

	return actor
}

func mapDefinitionToResponse(def *config.SettingDefinition) DefinitionResponse {
	defaultValue, _ := def.GetDefaultValueTyped()

	return DefinitionResponse{
		ID:               def.ID,
		Key:              def.Key,
		Type:             string(def.Type),
		DefaultValue:     defaultValue,
		Category:         def.Category,
		Description:      def.Description,
		Validation:       def.Validation,
		AllowedScopes:    def.AllowedScopes,
		ScopePermissions: def.ScopePermissions,
		DependsOn:        def.DependsOn,
		GroupName:        def.GroupName,
		SortOrder:        def.SortOrder,
	}
}

func mapResolvedSettingToResponse(s *config.ResolvedSetting, canModify bool) ResolvedSettingResponse {
	resp := ResolvedSettingResponse{
		Key:         s.Key,
		Value:       s.Value,
		Type:        string(s.Type),
		Category:    s.Category,
		Description: s.Description,
		GroupName:   s.GroupName,
		IsDefault:   s.IsDefault,
		IsActive:    s.IsActive,
		CanModify:   canModify,
		DependsOn:   s.DependsOn,
		Validation:  s.Validation,
	}

	if s.Source != nil {
		resp.Source = &ScopeRefResponse{
			Type: string(s.Source.Type),
			ID:   s.Source.ID,
		}
	}

	return resp
}

func mapHistoryToResponse(history []*configSvc.SettingHistoryEntry) []HistoryEntryResponse {
	responses := make([]HistoryEntryResponse, len(history))
	for i, h := range history {
		responses[i] = HistoryEntryResponse{
			ID:         h.ID,
			SettingKey: h.SettingKey,
			ChangeType: h.ChangeType,
			OldValue:   h.OldValue,
			NewValue:   h.NewValue,
			ChangedBy:  h.ChangedBy,
			ChangedAt:  h.ChangedAt,
			Reason:     h.Reason,
		}
	}
	return responses
}

// =============================================================================
// EXPORTED HANDLERS FOR TESTING
// =============================================================================

// InitializeDefinitionsHandler returns the initializeDefinitions handler for testing.
func (rs *ScopedSettingsResource) InitializeDefinitionsHandler() http.HandlerFunc {
	return rs.initializeDefinitions
}

// ListDefinitionsHandler returns the listDefinitions handler for testing.
func (rs *ScopedSettingsResource) ListDefinitionsHandler() http.HandlerFunc {
	return rs.listDefinitions
}

// GetDefinitionHandler returns the getDefinition handler for testing.
func (rs *ScopedSettingsResource) GetDefinitionHandler() http.HandlerFunc {
	return rs.getDefinition
}

// GetSystemSettingsHandler returns the getSystemSettings handler for testing.
func (rs *ScopedSettingsResource) GetSystemSettingsHandler() http.HandlerFunc {
	return rs.getSystemSettings
}

// UpdateSystemSettingHandler returns the updateSystemSetting handler for testing.
func (rs *ScopedSettingsResource) UpdateSystemSettingHandler() http.HandlerFunc {
	return rs.updateSystemSetting
}

// GetOGSettingsHandler returns the getOGSettings handler for testing.
func (rs *ScopedSettingsResource) GetOGSettingsHandler() http.HandlerFunc {
	return rs.getOGSettings
}

// UpdateOGSettingHandler returns the updateOGSetting handler for testing.
func (rs *ScopedSettingsResource) UpdateOGSettingHandler() http.HandlerFunc {
	return rs.updateOGSetting
}

// ResetOGSettingHandler returns the resetOGSetting handler for testing.
func (rs *ScopedSettingsResource) ResetOGSettingHandler() http.HandlerFunc {
	return rs.resetOGSetting
}

// GetUserSettingsHandler returns the getUserSettings handler for testing.
func (rs *ScopedSettingsResource) GetUserSettingsHandler() http.HandlerFunc {
	return rs.getUserSettings
}

// UpdateUserSettingHandler returns the updateUserSetting handler for testing.
func (rs *ScopedSettingsResource) UpdateUserSettingHandler() http.HandlerFunc {
	return rs.updateUserSetting
}

// GetHistoryHandler returns the getHistory handler for testing.
func (rs *ScopedSettingsResource) GetHistoryHandler() http.HandlerFunc {
	return rs.getHistory
}

// GetOGHistoryHandler returns the getOGHistory handler for testing.
func (rs *ScopedSettingsResource) GetOGHistoryHandler() http.HandlerFunc {
	return rs.getOGHistory
}

// GetOGKeyHistoryHandler returns the getOGKeyHistory handler for testing.
func (rs *ScopedSettingsResource) GetOGKeyHistoryHandler() http.HandlerFunc {
	return rs.getOGKeyHistory
}
