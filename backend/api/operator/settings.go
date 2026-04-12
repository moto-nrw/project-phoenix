package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// operatorWildcardPermissions grants full access to every setting. Used to
// bypass the schema builder's per-setting ReadPermission filter when an
// operator calls GetSchema for a school. Operator access is already gated
// at the route level by RequiresOperatorScope middleware.
var operatorWildcardPermissions = []string{"*:*"}

// SettingsResource handles operator-level settings management for schools.
type SettingsResource struct {
	settingsService configSvc.SettingsService
	db              *bun.DB
}

// NewSettingsResource creates a new operator settings resource.
func NewSettingsResource(svc configSvc.SettingsService, db *bun.DB) *SettingsResource {
	return &SettingsResource{
		settingsService: svc,
		db:              db,
	}
}

type setSchoolSettingRequest struct {
	Value any `json:"value"`
}

// GetSchoolSettingsSchema returns the full settings schema with resolved values for a school.
// Operators bypass per-setting permission checks (nil permissions = all visible and writable).
func (rs *SettingsResource) GetSchoolSettingsSchema(w http.ResponseWriter, r *http.Request) {
	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}

	var schema *configSvc.SettingsSchema
	err := tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(ctx context.Context, _ bun.Tx) error {
		var schemaErr error
		schema, schemaErr = rs.settingsService.GetSchema(ctx, operatorWildcardPermissions)
		return schemaErr
	})
	if err != nil {
		render.Render(w, r, ErrInternal("Failed to retrieve settings schema")) //nolint:errcheck
		return
	}

	common.Respond(w, r, http.StatusOK, schema, "Schema retrieved successfully")
}

// SetSchoolSettingValue sets a setting value for a specific school.
func (rs *SettingsResource) SetSchoolSettingValue(w http.ResponseWriter, r *http.Request) {
	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}
	key := chi.URLParam(r, "key")

	var req setSchoolSettingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err)) //nolint:errcheck
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	changedBy := int64(claims.ID)

	err := tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(ctx context.Context, _ bun.Tx) error {
		return rs.settingsService.SetValue(ctx, key, req.Value, &changedBy, nil)
	})
	if err != nil {
		renderOperatorSettingsError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Value updated successfully")
}

// ResetSchoolSettingValue resets a setting value for a specific school to its default.
func (rs *SettingsResource) ResetSchoolSettingValue(w http.ResponseWriter, r *http.Request) {
	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}
	key := chi.URLParam(r, "key")

	claims := jwt.ClaimsFromCtx(r.Context())
	changedBy := int64(claims.ID)

	err := tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(ctx context.Context, _ bun.Tx) error {
		return rs.settingsService.ResetValue(ctx, key, &changedBy, nil)
	})
	if err != nil {
		renderOperatorSettingsError(w, r, err)
		return
	}

	common.RespondNoContent(w, r)
}

// RevealSchoolSettingValue reveals the unmasked value of a password/PIN setting for a school.
func (rs *SettingsResource) RevealSchoolSettingValue(w http.ResponseWriter, r *http.Request) {
	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}
	key := chi.URLParam(r, "key")

	def := configModel.GetDefinition(key)
	if def == nil {
		render.Render(w, r, ErrNotFound(fmt.Sprintf("setting %q not found", key))) //nolint:errcheck
		return
	}

	var value any
	err := tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(ctx context.Context, _ bun.Tx) error {
		var resolveErr error
		value, resolveErr = rs.settingsService.Resolve(ctx, key)
		return resolveErr
	})
	if err != nil {
		renderOperatorSettingsError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]any{"value": value}, "")
}

// renderOperatorSettingsError maps settings service errors to operator HTTP responses.
func renderOperatorSettingsError(w http.ResponseWriter, r *http.Request, err error) {
	var settingsErr *configSvc.SettingsError
	if !errors.As(err, &settingsErr) {
		render.Render(w, r, ErrInternal(err.Error())) //nolint:errcheck
		return
	}

	inner := settingsErr.Unwrap()

	var defNotFound *configSvc.DefinitionNotFoundError
	var invalidValue *configSvc.InvalidValueError
	var permDenied *configSvc.PermissionDeniedError

	switch {
	case errors.As(inner, &defNotFound):
		render.Render(w, r, ErrNotFound(err.Error())) //nolint:errcheck
	case errors.As(inner, &invalidValue):
		render.Render(w, r, ErrInvalidRequest(err)) //nolint:errcheck
	case errors.As(inner, &permDenied):
		render.Render(w, r, ErrForbidden(err.Error())) //nolint:errcheck
	default:
		render.Render(w, r, ErrInternal(err.Error())) //nolint:errcheck
	}
}
