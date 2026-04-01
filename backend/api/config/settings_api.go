package config

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// SettingsResource defines the settings API resource.
type SettingsResource struct {
	settingsService configSvc.SettingsService
	db              *bun.DB
}

// NewSettingsResource creates a new settings resource.
func NewSettingsResource(svc configSvc.SettingsService, db *bun.DB) *SettingsResource {
	return &SettingsResource{
		settingsService: svc,
		db:              db,
	}
}

// SettingsRouter returns a configured router for the 3 settings endpoints.
func (rs *SettingsResource) SettingsRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	tokenAuth, _ := jwt.NewTokenAuth()

	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)
		withTx := tenant.TenantTxMiddleware(rs.db)

		settingsWrite := authorize.RequiresAnyPermission(permissions.ConfigUpdate, permissions.ConfigManage)

		r.With(authorize.RequiresPermission(permissions.ConfigRead), withTx).Get("/schema", rs.getSchema)
		r.With(settingsWrite, withTx).Put("/values/{key}", rs.setValue)
		r.With(settingsWrite, withTx).Delete("/values/{key}", rs.resetValue)
	})

	return r
}

// --- Request types ---

type setValueRequest struct {
	Value any `json:"value"`
}

// --- Handlers ---

func (rs *SettingsResource) getSchema(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())

	schema, err := rs.settingsService.GetSchema(r.Context(), claims.Permissions)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, schema, "Schema retrieved successfully")
}

func (rs *SettingsResource) setValue(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")

	var req setValueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	changedBy := int64(claims.ID)

	err := tenant.WithTenantTx(r.Context(), rs.db, tenant.FromContext(r.Context()), func(ctx context.Context, _ bun.Tx) error {
		return rs.settingsService.SetValue(ctx, key, req.Value, &changedBy, claims.Permissions)
	})
	if err != nil {
		renderSettingsError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Value updated successfully")
}

func (rs *SettingsResource) resetValue(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")

	claims := jwt.ClaimsFromCtx(r.Context())
	changedBy := int64(claims.ID)

	err := tenant.WithTenantTx(r.Context(), rs.db, tenant.FromContext(r.Context()), func(ctx context.Context, _ bun.Tx) error {
		return rs.settingsService.ResetValue(ctx, key, &changedBy, claims.Permissions)
	})
	if err != nil {
		renderSettingsError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusNoContent, nil, "")
}

// --- Error rendering ---

func renderSettingsError(w http.ResponseWriter, r *http.Request, err error) {
	var settingsErr *configSvc.SettingsError
	if !errors.As(err, &settingsErr) {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	inner := settingsErr.Unwrap()

	var defNotFound *configSvc.DefinitionNotFoundError
	var invalidValue *configSvc.InvalidValueError
	var permDenied *configSvc.PermissionDeniedError

	switch {
	case errors.As(inner, &defNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.As(inner, &invalidValue):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.As(inner, &permDenied):
		common.RenderError(w, r, common.ErrorForbidden(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
