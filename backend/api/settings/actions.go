package settings

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/config"
)

// executeAction handles executing an action
func (rs *Resource) executeAction(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("action key is required")))
		return
	}

	// Build action audit context
	auditCtx := buildActionAuditContext(r)

	// Execute the action
	result, err := rs.SettingsService.ExecuteAction(r.Context(), key, auditCtx)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	if result == nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("action not found")))
		return
	}

	if !result.Success {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(result.Message)))
		return
	}

	common.Respond(w, r, http.StatusOK, result, result.Message)
}

// getActionHistory handles getting the execution history for an action
func (rs *Resource) getActionHistory(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("action key is required")))
		return
	}

	// Parse limit
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	history, err := rs.SettingsService.GetActionHistory(r.Context(), key, limit)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]interface{}{
		"actionKey": key,
		"entries":   history,
	}, "Action history retrieved successfully")
}

// getRecentActionExecutions handles getting recent action executions
func (rs *Resource) getRecentActionExecutions(w http.ResponseWriter, r *http.Request) {
	// Parse limit
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	entries, err := rs.SettingsService.GetRecentActionExecutions(r.Context(), limit)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, entries, "Recent action executions retrieved successfully")
}

// buildActionAuditContext creates an ActionAuditContext from the request
func buildActionAuditContext(r *http.Request) *config.ActionAuditContext {
	ctx := &config.ActionAuditContext{}

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
	} else {
		ctx.AccountName = "System"
	}

	// Get IP address
	ctx.IPAddress = r.RemoteAddr

	// Get user agent
	ctx.UserAgent = r.UserAgent()

	return ctx
}
