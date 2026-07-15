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
	"github.com/moto-nrw/project-phoenix/realtime"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// errAdminOnlyForOperator explains why an operator may not touch an
// AccessAdminOnly setting (e.g. the OGS device PIN). Surfaced as HTTP 403.
const errAdminOnlyForOperator = "this setting is admin-only and cannot be modified by operators"

const errLegalAGBDocumentManagedByUpload = "AGB document URL is managed by the file upload endpoint"

// errPresenceModeSwitchBlockedMsg is the German user-facing copy returned
// when an operator tries to flip operations.presence_mode while students are
// still checked in for the day. staticcheck ST1005 (capitalization / trailing
// punctuation) is waived at the callsite with a nolint directive, matching
// the convention in api/groups for German user-facing errors.
const errPresenceModeSwitchBlockedMsg = "Moduswechsel während aktiver Anwesenheit nicht möglich. Bitte zunächst Tagesabschluss durchführen."

// ErrPresenceModeSwitchBlocked is the sentinel returned by
// enforcePresenceModeSwitchGuard so callers can branch on it via errors.Is
// without relying on string equality (which breaks the moment any wrapper
// modifies the message). Mirrors the SettingsService error pattern
// (DefinitionNotFoundError / InvalidValueError).
//
//nolint:staticcheck // ST1005: German user-facing message
var ErrPresenceModeSwitchBlocked = errors.New(errPresenceModeSwitchBlockedMsg)

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
func enforcePresenceModeSwitchGuard(ctx context.Context, activeSvc activeSvc.Service, key string, force bool) error {
	if key != configModel.KeyPresenceMode {
		return nil
	}
	if force {
		return nil
	}
	// Bind the Berlin calendar date as a DATE literal (matches how
	// performCheckIn writes active.attendance.date). Using CURRENT_DATE on
	// the postgres side would be wrong: the PG session is UTC, so between
	// 22:00–24:00 UTC (i.e. ~00:00–02:00 Berlin) CURRENT_DATE returns
	// "yesterday" while open rows for the new Berlin day already exist
	// under "today" — and the guard would silently let the switch through
	// in exactly the window it's supposed to block.
	today := timezone.TodayDate()
	exists, err := activeSvc.HasOpenAttendanceOn(ctx, today)
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

func guardOperatorDirectManagedSettingWrite(w http.ResponseWriter, r *http.Request, key string) bool {
	if key != configModel.KeyEnrollmentLegalAGBDocumentURL {
		return false
	}
	render.Render(w, r, ErrForbidden(errLegalAGBDocumentManagedByUpload)) //nolint:errcheck
	return true
}

// SettingsResource handles operator-level settings management for schools.
type SettingsResource struct {
	settingsService configSvc.SettingsService
	db              *bun.DB
	broadcaster     realtime.Broadcaster
	// schoolRepo lets the resource emit `school_slug` in set/reset
	// responses so the frontend operator proxy can bust the slug-keyed
	// `tenant-${slug}` Next.js cache after tenant-resolve-affecting
	// toggles (currently only operations.student_photos_enabled).
	schoolService platformSvc.SchoolService
	activeService activeSvc.Service
	// onValueSet shares its signature with config.ValueSetCallback — see
	// that type for the in-tx vs post-commit contract. Duplicated here
	// (rather than imported) so api/operator stays free of api/config
	// dependency, mirroring the rest of the operator package's isolation.
	onValueSet func(ctx context.Context, tenantID int64, key string, value any) (postCommit func(), err error)
}

// NewSettingsResource creates a new operator settings resource. broadcaster
// emits the cross-origin tenant_settings_changed SSE event so open tenant
// tabs invalidate their settings caches when an operator flips a value.
// schoolRepo enriches the response with the school's slug so the frontend
// operator proxy can additionally bust the `tenant-${slug}` Next.js cache
// for tenant-resolve-affecting settings (e.g. student_photos_enabled).
// Both are optional — nil disables the corresponding mechanism.
func NewSettingsResource(svc configSvc.SettingsService, db *bun.DB, broadcaster realtime.Broadcaster, schoolService platformSvc.SchoolService, activeService activeSvc.Service) *SettingsResource {
	return &SettingsResource{
		settingsService: svc,
		db:              db,
		broadcaster:     broadcaster,
		schoolService:   schoolService,
		activeService:   activeService,
	}
}

// scheduleSettingsBroadcast delegates to the shared cross-portal helper; see
// common.ScheduleTenantSettingsBroadcast for the SSE contract.
func (rs *SettingsResource) scheduleSettingsBroadcast(ctx context.Context, tenantID int64, key string) {
	common.ScheduleTenantSettingsBroadcast(ctx, rs.broadcaster, tenantID, key)
}

// OnValueSet registers a callback that runs after a setting value change is
// validated and persisted. The callback runs inside the tenant transaction;
// the optional postCommit closure it returns runs only after a successful
// commit. Mirrors the tenant SettingsResource.OnValueSet contract so side
// effects apply uniformly regardless of who flipped the value.
func (rs *SettingsResource) OnValueSet(fn func(ctx context.Context, tenantID int64, key string, value any) (postCommit func(), err error)) {
	rs.onValueSet = fn
}

type setSchoolSettingRequest struct {
	Value any `json:"value"`
}

// requiresPhotoMutationResponse gates which keys carry a body in set/reset
// responses (vs. an empty 204 / generic 200). Today only the photo-feature
// flag needs the slug for cache busting; other settings stay on the
// development-era empty-body response.
func requiresPhotoMutationResponse(key string) bool {
	return key == configModel.KeyStudentPhotosEnabled
}

// schoolSettingMutationResponse carries the school slug back to the frontend
// so the operator proxy can bust the `tenant-${slug}` Next.js cache. The
// slug is omitted when the lookup fails (the mutation already committed by
// then — failing the response would lie about the underlying state).
type schoolSettingMutationResponse struct {
	SchoolSlug string `json:"school_slug,omitempty"`
}

// resolveSchoolSlug fetches the school's slug for inclusion in the mutation
// response. Failures are logged but never propagated — the slug is only
// used for cache invalidation, and a missed bust is recoverable (cache TTL
// is 5 min) whereas a 500 here would mislead the operator about whether the
// setting actually persisted.
func (rs *SettingsResource) resolveSchoolSlug(ctx context.Context, schoolID int64) string {
	if rs.schoolService == nil {
		return ""
	}
	school, err := rs.schoolService.GetSchoolByID(ctx, schoolID)
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
	if guardOperatorDirectManagedSettingWrite(w, r, key) {
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

	// Audit-log force-bypass writes on guarded keys so we have a trail when
	// an operator overrides the safety check. This is intentionally a Warn
	// (not Info) so it surfaces in standard log review.
	if force && key == configModel.KeyPresenceMode {
		slog.Warn("operator_setting_force_bypass",
			slog.Int64("operator_id", changedBy),
			slog.Int64("school_id", schoolID),
			slog.String("setting_key", key),
		)
	}

	err := tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(ctx context.Context, tx bun.Tx) error {
		if err := enforcePresenceModeSwitchGuard(ctx, rs.activeService, key, force); err != nil {
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
			tenant.RegisterAfterCommit(ctx, cb)
		}
		rs.scheduleSettingsBroadcast(ctx, schoolID, key)
		return nil
	})
	if err != nil {
		// Dedicated 409 path for the mode-switch block so the frontend can
		// surface the "daily end required" copy without heuristics. errors.Is
		// keeps the branch resilient to wrapping, unlike string equality.
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
	if guardOperatorDirectManagedSettingWrite(w, r, key) {
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	changedBy := int64(claims.ID)

	err := tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(ctx context.Context, _ bun.Tx) error {
		if err := rs.settingsService.ResetValue(ctx, key, &changedBy, nil); err != nil {
			return err
		}
		// Photo disable/reset needs the same downstream cleanup regardless
		// of whether the operator chose PUT false or DELETE reset. Other
		// settings keep their development-era reset semantics.
		if rs.onValueSet != nil && requiresPhotoMutationResponse(key) {
			def := configModel.GetDefinition(key)
			if def != nil {
				cb, err := rs.onValueSet(ctx, schoolID, key, def.Default)
				if err != nil {
					return err
				}
				tenant.RegisterAfterCommit(ctx, cb)
			}
		}
		rs.scheduleSettingsBroadcast(ctx, schoolID, key)
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

	// Photo reset returns a body so the proxy can read the slug and bust
	// the tenant-resolve cache immediately.
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
