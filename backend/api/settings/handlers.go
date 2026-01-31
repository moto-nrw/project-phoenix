package settings

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/settings"
)

// listTabs handles listing available settings tabs
func (rs *Resource) listTabs(w http.ResponseWriter, r *http.Request) {
	// Get user permissions from context
	userPerms := getUserPermissions(r)

	tabs, err := rs.SettingsService.GetTabs(r.Context(), userPerms)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, tabs, "Tabs retrieved successfully")
}

// getTabSettings handles getting settings for a specific tab
func (rs *Resource) getTabSettings(w http.ResponseWriter, r *http.Request) {
	tab := chi.URLParam(r, "tab")
	if tab == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tab is required")))
		return
	}

	// Build scope context from request
	scopeCtx := buildScopeContext(r)

	// Get user permissions from context
	userPerms := getUserPermissions(r)

	response, err := rs.SettingsService.GetTabSettings(r.Context(), tab, scopeCtx, userPerms)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, response, "Tab settings retrieved successfully")
}

// listDefinitions handles listing all setting definitions (admin only)
func (rs *Resource) listDefinitions(w http.ResponseWriter, r *http.Request) {
	// Get all registered definitions from code
	defs := settings.All()

	response := make([]*config.SettingDefinitionDTO, len(defs))
	for i, def := range defs {
		dbDef := def.ToSettingDefinition()
		response[i] = dbDef.ToDTO()
	}

	common.Respond(w, r, http.StatusOK, response, "Definitions retrieved successfully")
}

// syncDefinitions handles syncing code definitions and tabs to database
func (rs *Resource) syncDefinitions(w http.ResponseWriter, r *http.Request) {
	if err := rs.SettingsService.SyncAll(r.Context()); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]int{
		"definitions_synced": settings.Count(),
		"tabs_synced":        settings.TabCount(),
	}, "Settings and tabs synced successfully")
}

// getValue handles getting a setting's effective value
func (rs *Resource) getValue(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("key is required")))
		return
	}

	// Build scope context from request
	scopeCtx := buildScopeContext(r)

	resolved, err := rs.SettingsService.GetEffective(r.Context(), key, scopeCtx)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(err))
		return
	}

	common.Respond(w, r, http.StatusOK, resolved, "Setting value retrieved successfully")
}

// setValue handles setting a value at a specific scope
func (rs *Resource) setValue(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("key is required")))
		return
	}

	// Parse request
	req := &SetValueRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Build audit context
	auditCtx := buildAuditContext(r)

	// Set the value
	if err := rs.SettingsService.SetValue(r.Context(), key, req.Value, req.Scope, req.ScopeID, auditCtx); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Setting value updated successfully")
}

// deleteValue handles soft deleting a scope override
func (rs *Resource) deleteValue(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("key is required")))
		return
	}

	// Parse request
	req := &DeleteValueRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Build audit context
	auditCtx := buildAuditContext(r)

	// Delete the override
	if err := rs.SettingsService.DeleteOverride(r.Context(), key, req.Scope, req.ScopeID, auditCtx); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Setting override deleted successfully")
}

// getObjectRefOptions handles getting available options for object reference settings
func (rs *Resource) getObjectRefOptions(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("key is required")))
		return
	}

	// Build scope context from request
	scopeCtx := buildScopeContext(r)

	options, err := rs.SettingsService.GetObjectRefOptions(r.Context(), key, scopeCtx)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, options, "Options retrieved successfully")
}

// getSettingHistory handles getting the change history for a setting
func (rs *Resource) getSettingHistory(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("key is required")))
		return
	}

	// Parse limit
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	history, err := rs.SettingsService.GetSettingHistory(r.Context(), key, limit)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]interface{}{
		"key":     key,
		"entries": history,
	}, "History retrieved successfully")
}

// getRecentChanges handles getting recent audit entries
func (rs *Resource) getRecentChanges(w http.ResponseWriter, r *http.Request) {
	// Parse limit
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	entries, err := rs.SettingsService.GetRecentChanges(r.Context(), limit)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, entries, "Recent changes retrieved successfully")
}

// restoreValue handles restoring a soft-deleted value
func (rs *Resource) restoreValue(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("key is required")))
		return
	}

	// Parse request
	req := &RestoreValueRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Build audit context
	auditCtx := buildAuditContext(r)

	// Restore the value
	if err := rs.SettingsService.RestoreValue(r.Context(), key, req.Scope, req.ScopeID, auditCtx); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Setting value restored successfully")
}

// purgeDeleted handles permanently removing old soft-deleted records
func (rs *Resource) purgeDeleted(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &PurgeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Default to 30 days
	days := 30
	if req.Days > 0 {
		days = req.Days
	}

	count, err := rs.SettingsService.PurgeDeletedOlderThan(r.Context(), days)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]interface{}{
		"purged": count,
		"days":   days,
	}, "Deleted records purged successfully")
}

// buildScopeContext creates a ScopeContext from the request
func buildScopeContext(r *http.Request) *config.ScopeContext {
	scopeCtx := &config.ScopeContext{}

	// Get account ID from JWT claims
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID > 0 {
		accountID := int64(claims.ID)
		scopeCtx.AccountID = &accountID
	}

	// Get device ID from query param or header
	if deviceIDStr := r.URL.Query().Get("device_id"); deviceIDStr != "" {
		if deviceID, err := strconv.ParseInt(deviceIDStr, 10, 64); err == nil {
			scopeCtx.DeviceID = &deviceID
		}
	}

	return scopeCtx
}

// buildAuditContext creates an AuditContext from the request
func buildAuditContext(r *http.Request) *config.AuditContext {
	ctx := &config.AuditContext{}

	// Get account info from JWT claims
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID > 0 {
		ctx.AccountID = int64(claims.ID)
		// Use name from claims if available, fall back to username
		if claims.FirstName != "" || claims.LastName != "" {
			ctx.AccountName = claims.FirstName + " " + claims.LastName
		} else {
			ctx.AccountName = claims.Username
		}
	}

	// Get IP address
	ctx.IPAddress = r.RemoteAddr

	// Get user agent
	ctx.UserAgent = r.UserAgent()

	return ctx
}

// getUserPermissions extracts user permissions from the request context
func getUserPermissions(r *http.Request) []string {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		return []string{}
	}
	return claims.Permissions
}
