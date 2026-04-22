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

// errAdminOnlyForOperator explains why an operator may not touch an
// AccessAdminOnly setting (e.g. the OGS device PIN). Surfaced as HTTP 403.
const errAdminOnlyForOperator = "this setting is admin-only and cannot be modified by operators"

// errPresenceModeSwitchBlocked is shown to the operator when they try to flip
// operations.presence_mode while students are still checked in for the day.
// German copy intentionally omits trailing punctuation to satisfy staticcheck
// ST1005 — the settings UI joins this into a longer sentence anyway.
const errPresenceModeSwitchBlocked = "Moduswechsel während aktiver Anwesenheit nicht möglich — bitte zunächst Tagesabschluss durchführen"

// enforcePresenceModeSwitchGuard rejects an in-progress write to
// operations.presence_mode when any student is currently checked in for the
// tenant. Runs inside the same tenant transaction as the write itself so RLS
// scopes the attendance query automatically. Callers may bypass the guard
// with `?force=true` for operational recovery.
//
// Why guard only the switch, not every write: the cascading impact of
// flipping presence mode mid-day (stale open visits, SSE events mis-keyed,
// device UX inconsistent with the tenant it's authenticated for) is far
// larger than any other operator setting. Other keys don't need this.
func enforcePresenceModeSwitchGuard(ctx context.Context, tx bun.Tx, key string, force bool) error {
	if key != configModel.KeyPresenceMode {
		return nil
	}
	if force {
		return nil
	}
	var exists bool
	err := tx.NewRaw(`
		SELECT EXISTS(
			SELECT 1 FROM active.attendance
			WHERE date = CURRENT_DATE
			  AND check_out_time IS NULL
		)
	`).Scan(ctx, &exists)
	if err != nil {
		return fmt.Errorf("failed to check active attendance before mode switch: %w", err)
	}
	if exists {
		return errors.New(errPresenceModeSwitchBlocked)
	}
	return nil
}

// guardOperatorWrite blocks the operator from set/reset/reveal on AccessAdminOnly settings.
// Returns true when the handler should abort (response has already been written).
func guardOperatorWrite(w http.ResponseWriter, r *http.Request, key string) bool {
	def := configModel.GetDefinition(key)
	if def == nil {
		render.Render(w, r, ErrNotFound(fmt.Sprintf("setting %q not found", key))) //nolint:errcheck
		return true
	}
	if def.AccessPolicy == configModel.AccessAdminOnly {
		render.Render(w, r, ErrForbidden(errAdminOnlyForOperator)) //nolint:errcheck
		return true
	}
	return false
}

// SettingsResource handles operator-level settings management for schools.
type SettingsResource struct {
	settingsService configSvc.SettingsService
	db              *bun.DB
	onValueSet      func(ctx context.Context, tenantID int64, key string, value any) error
}

// NewSettingsResource creates a new operator settings resource.
func NewSettingsResource(svc configSvc.SettingsService, db *bun.DB) *SettingsResource {
	return &SettingsResource{
		settingsService: svc,
		db:              db,
	}
}

// OnValueSet registers a callback that runs after a setting value is validated
// and saved. The callback executes inside the same tenant transaction as the
// setting write, so returning an error aborts the request and rolls back the
// update. Mirrors the tenant SettingsResource.OnValueSet contract so side
// effects (e.g. auto-creating the Schulhof/WC rooms when the corresponding
// checkout toggle flips on) apply uniformly regardless of who flipped it.
func (rs *SettingsResource) OnValueSet(fn func(ctx context.Context, tenantID int64, key string, value any) error) {
	rs.onValueSet = fn
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
		schema, schemaErr = rs.settingsService.GetSchemaForOperator(ctx, nil)
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
	if guardOperatorWrite(w, r, key) {
		return
	}

	var req setSchoolSettingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err)) //nolint:errcheck
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	changedBy := int64(claims.ID)
	// ?force=true lets operators bypass the presence-mode switch guard when
	// doing maintenance recovery (e.g. stuck attendance rows blocking the
	// switch). Everything else ignores the flag.
	force := r.URL.Query().Get("force") == "true"

	err := tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(ctx context.Context, tx bun.Tx) error {
		if err := enforcePresenceModeSwitchGuard(ctx, tx, key, force); err != nil {
			return err
		}
		if err := rs.settingsService.SetValue(ctx, key, req.Value, &changedBy, nil); err != nil {
			return err
		}
		if rs.onValueSet != nil {
			return rs.onValueSet(ctx, schoolID, key, req.Value)
		}
		return nil
	})
	if err != nil {
		// Dedicated 409 path for the mode-switch block so the frontend can
		// surface the "daily end required" copy without heuristics.
		if err.Error() == errPresenceModeSwitchBlocked {
			render.Render(w, r, ErrConflict(errPresenceModeSwitchBlocked)) //nolint:errcheck
			return
		}
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
	if guardOperatorWrite(w, r, key) {
		return
	}

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
	if guardOperatorWrite(w, r, key) {
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
