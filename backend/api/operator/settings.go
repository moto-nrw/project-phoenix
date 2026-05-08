package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// errAdminOnlyForOperator explains why an operator may not touch an
// AccessAdminOnly setting (e.g. the OGS device PIN). Surfaced as HTTP 403.
const errAdminOnlyForOperator = "this setting is admin-only and cannot be modified by operators"

const errPresenceModeSwitchBlockedMsg = "Moduswechsel während aktiver Anwesenheit nicht möglich. Bitte zunächst Tagesabschluss durchführen."

//nolint:staticcheck // ST1005: German user-facing message
var ErrPresenceModeSwitchBlocked = errors.New(errPresenceModeSwitchBlockedMsg)

func enforcePresenceModeSwitchGuard(ctx context.Context, tx bun.Tx, key string, force bool) error {
	if key != configModel.KeyPresenceMode {
		return nil
	}
	if force {
		return nil
	}

	today := timezone.TodayUTC()
	var exists bool
	err := tx.NewRaw(`
		SELECT EXISTS(
			SELECT 1 FROM active.attendance
			WHERE date = ?
			  AND check_out_time IS NULL
		)
	`, today).Scan(ctx, &exists)
	if err != nil {
		return fmt.Errorf("failed to check active attendance before mode switch: %w", err)
	}
	if exists {
		return ErrPresenceModeSwitchBlocked
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
	schoolRepo      platformModels.SchoolRepository
	// onValueSet shares its signature with config.ValueSetCallback — see
	// that type for the in-tx vs post-commit contract. Duplicated here
	// (rather than imported) so api/operator stays free of api/config
	// dependency, mirroring the rest of the operator package's isolation.
	onValueSet func(ctx context.Context, tenantID int64, key string, value any) (postCommit func(), err error)
}

// NewSettingsResource creates a new operator settings resource. schoolRepo is
// used to enrich set/reset responses with `school_slug` so the frontend
// proxy can bust the slug-keyed `tenant-${slug}` Next.js cache after the
// operator flips a tenant-resolve-affecting setting (e.g.
// operations.student_photos_enabled). Without that bust, [tenant]/layout.tsx
// keeps serving the stale resolve payload for up to 5 minutes.
func NewSettingsResource(svc configSvc.SettingsService, db *bun.DB, schoolRepo platformModels.SchoolRepository) *SettingsResource {
	return &SettingsResource{
		settingsService: svc,
		db:              db,
		schoolRepo:      schoolRepo,
	}
}

// OnValueSet registers a callback that runs after a setting value change is
// validated and persisted. The callback runs inside the tenant transaction;
// the optional postCommit closure it returns runs only after a successful
// commit. Mirrors the tenant SettingsResource.OnValueSet contract so side
// effects apply uniformly regardless of who flipped the value.
//
// The hook always fires on PUT. DELETE replays it only for keys that need
// reset-specific side effects, currently operations.student_photos_enabled,
// so unrelated settings keep their development behavior.
func (rs *SettingsResource) OnValueSet(fn func(ctx context.Context, tenantID int64, key string, value any) (postCommit func(), err error)) {
	rs.onValueSet = fn
}

type setSchoolSettingRequest struct {
	Value any `json:"value"`
}

func requiresPhotoMutationResponse(key string) bool {
	return key == configModel.KeyStudentPhotosEnabled
}

// schoolSettingMutationResponse carries the school slug back to the frontend
// so the operator proxy can bust the `tenant-${slug}` Next.js cache. The slug
// is omitted when the lookup fails (the mutation already committed by then —
// failing the response would lie about the underlying state).
type schoolSettingMutationResponse struct {
	SchoolSlug string `json:"school_slug,omitempty"`
}

// resolveSchoolSlug fetches the school's slug for inclusion in the
// mutation response. Failures are logged but never propagated — the slug
// is only used for cache invalidation, and a missed bust is recoverable
// (cache TTL is 5 min) whereas a 500 here would mislead the operator about
// whether the setting actually persisted.
func (rs *SettingsResource) resolveSchoolSlug(ctx context.Context, schoolID int64) string {
	if rs.schoolRepo == nil {
		return ""
	}
	school, err := rs.schoolRepo.FindByID(ctx, schoolID)
	if err != nil || school == nil {
		slog.Warn("operator settings: school slug lookup failed",
			slog.Int64("school_id", schoolID),
			slog.Any("error", err),
		)
		return ""
	}
	return school.Slug
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
	force := r.URL.Query().Get("force") == "true"

	if force && key == configModel.KeyPresenceMode {
		slog.Warn("operator_setting_force_bypass",
			slog.Int64("operator_id", changedBy),
			slog.Int64("school_id", schoolID),
			slog.String("setting_key", key),
		)
	}

	err := tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(ctx context.Context, tx bun.Tx) error {
		if err := enforcePresenceModeSwitchGuard(ctx, tx, key, force); err != nil {
			return err
		}
		if err := rs.settingsService.SetValue(ctx, key, req.Value, &changedBy, nil); err != nil {
			return err
		}
		if rs.onValueSet != nil {
			cb, err := rs.onValueSet(ctx, schoolID, key, req.Value)
			if err != nil {
				return err
			}
			// Use the after-commit hook even though operator routes don't
			// currently sit behind TenantTxMiddleware: this WithTenantTx is
			// the outermost call here, so the hook will fire after THIS
			// commit. Routing-topology assumptions are fragile — if a
			// future change wraps operator routes in a middleware tx, the
			// hook automatically promotes to running after the outer
			// commit instead of leaking destructive side effects out.
			tenant.RegisterAfterCommit(ctx, cb)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrPresenceModeSwitchBlocked) {
			render.Render(w, r, ErrConflict(errPresenceModeSwitchBlockedMsg)) //nolint:errcheck
			return
		}
		renderOperatorSettingsError(w, r, err)
		return
	}

	if !requiresPhotoMutationResponse(key) {
		common.Respond(w, r, http.StatusOK, nil, "Value updated successfully")
		return
	}

	resp := schoolSettingMutationResponse{SchoolSlug: rs.resolveSchoolSlug(r.Context(), schoolID)}
	common.Respond(w, r, http.StatusOK, resp, "Value updated successfully")
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
		if err := rs.settingsService.ResetValue(ctx, key, &changedBy, nil); err != nil {
			return err
		}
		// Photo disable/reset needs the same downstream cleanup regardless of
		// whether the operator chose PUT false or DELETE reset. Other settings
		// keep their development-era reset semantics.
		if rs.onValueSet != nil && requiresPhotoMutationResponse(key) {
			def := configModel.GetDefinition(key)
			if def != nil {
				cb, err := rs.onValueSet(ctx, schoolID, key, def.Default)
				if err != nil {
					return err
				}
				// Same after-commit reasoning as SetSchoolSettingValue.
				tenant.RegisterAfterCommit(ctx, cb)
			}
		}
		return nil
	})
	if err != nil {
		renderOperatorSettingsError(w, r, err)
		return
	}

	if !requiresPhotoMutationResponse(key) {
		common.RespondNoContent(w, r)
		return
	}

	// Photo reset returns a body so the proxy can read the slug and bust the
	// tenant-resolve cache immediately.
	resp := schoolSettingMutationResponse{SchoolSlug: rs.resolveSchoolSlug(r.Context(), schoolID)}
	common.Respond(w, r, http.StatusOK, resp, "Value reset successfully")
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
