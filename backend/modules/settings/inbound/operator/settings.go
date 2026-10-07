// Package operator serves the operator school settings routes of Settings
// Platform (#3232): the school's settings schema, the booking authority
// review, and setting, resetting and revealing a school's values. The
// operator router in api/operator mounts these handlers behind its
// middleware chain, so the operator wire format and authorization stay
// unchanged. The handlers call the public OperatorSchoolSettings capability
// (#2736); the tenant transaction, the side effects and the broadcast live
// behind it.
package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/settings"
)

// errAdminOnlyForOperator explains why an operator may not touch an
// AccessAdminOnly setting (e.g. the OGS device PIN). Surfaced as HTTP 403.
const errAdminOnlyForOperator = "this setting is admin-only and cannot be modified by operators"

const errLegalAGBDocumentManagedByUpload = "AGB document URL is managed by the file upload endpoint"

// guardOperatorWrite blocks the operator from set/reset/reveal on AccessAdminOnly settings.
// The access-policy decision lives in the settings capability (CheckOperatorWritable);
// this maps the returned error to the operator HTTP response. Returns true when
// the handler should abort (response has already been written).
func (rs *SettingsResource) guardOperatorWrite(w http.ResponseWriter, r *http.Request, key string) bool {
	err := rs.settings.CheckOperatorWritable(key)
	if err == nil {
		return false
	}
	if _, ok := errors.AsType[*settings.DefinitionNotFoundError](err); ok {
		common.RenderError(w, r, common.OperatorRejection(http.StatusNotFound, common.CodeSettingsNotFound, fmt.Sprintf("setting %q not found", key)))
		return true
	}
	common.RenderError(w, r, common.OperatorRejection(http.StatusForbidden, common.CodeSettingsAdminOnly, errAdminOnlyForOperator))
	return true
}

func guardOperatorDirectManagedSettingWrite(w http.ResponseWriter, r *http.Request, key string) bool {
	if key != settings.KeyEnrollmentLegalAGBDocumentURL {
		return false
	}
	common.RenderError(w, r, common.OperatorRejection(http.StatusForbidden, common.CodeSettingsManagedByUpload, errLegalAGBDocumentManagedByUpload))
	return true
}

// SettingsResource handles operator-level settings management for schools.
type SettingsResource struct {
	// settings owns the reads and the set/reset orchestration (presence-mode
	// guard, tenant transaction, side-effect hook, SSE broadcast).
	settings settings.OperatorSchoolSettings
	// schoolService lets the resource emit `school_slug` in set/reset
	// responses so the frontend operator proxy can bust the slug-keyed
	// `tenant-${slug}` Next.js cache after tenant-resolve-affecting
	// toggles (currently only operations.student_photos_enabled).
	schoolService SchoolLookup
}

// SettingsConfig holds the operator settings routes' dependencies.
type SettingsConfig struct {
	Settings settings.OperatorSchoolSettings
	// Schools enriches the response with the school's slug so the frontend
	// operator proxy can additionally bust the `tenant-${slug}` Next.js cache
	// for tenant-resolve-affecting settings (e.g. student_photos_enabled).
	Schools SchoolLookup
}

// NewSettingsResource creates a new operator settings resource.
func NewSettingsResource(cfg SettingsConfig) *SettingsResource {
	return &SettingsResource{
		settings:      cfg.Settings,
		schoolService: cfg.Schools,
	}
}

type setSchoolSettingRequest struct {
	Value any `json:"value"`
}

// requiresPhotoMutationResponse gates which keys carry a body in set/reset
// responses (vs. an empty 204 / generic 200). Today only the photo-feature
// flag needs the slug for cache busting; other settings stay on the
// development-era empty-body response.
func requiresPhotoMutationResponse(key string) bool {
	return key == settings.KeyStudentPhotosEnabled
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
	school, err := rs.schoolService.FindSchool(ctx, schoolID)
	if err != nil {
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

	schema, err := rs.settings.Schema(r.Context(), schoolID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("Failed to retrieve settings schema", err))
		return
	}

	common.Respond(w, r, http.StatusOK, schema, "Schema retrieved successfully")
}

// GetBookingAuthorityImpact shows the facts an operator must review before
// enabling booking-led care. The write path repeats this evaluation under the
// booking-write lock, so a stale preview cannot bypass the guard.
func (rs *SettingsResource) GetBookingAuthorityImpact(w http.ResponseWriter, r *http.Request) {
	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}
	impact, err := rs.settings.BookingAuthorityImpact(r.Context(), schoolID)
	if errors.Is(err, settings.ErrBookingAuthorityImpactUnavailable) {
		common.RenderError(w, r, common.ErrorInternalServerWrap("Booking authority impact service is not configured", err))
		return
	}
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("Failed to review booking authority impact", err))
		return
	}
	common.Respond(w, r, http.StatusOK, impact, "Booking authority impact retrieved successfully")
}

// SetSchoolSettingValue sets a setting value for a specific school.
func (rs *SettingsResource) SetSchoolSettingValue(w http.ResponseWriter, r *http.Request) {
	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}
	key := chi.URLParam(r, "key")
	if rs.guardOperatorWrite(w, r, key) {
		return
	}
	if guardOperatorDirectManagedSettingWrite(w, r, key) {
		return
	}

	var req setSchoolSettingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	changedBy := int64(claims.ID)
	// ?force=true lets operators bypass the presence-mode switch guard when
	// doing maintenance recovery (e.g. stuck attendance rows blocking the
	// switch). Everything else ignores the flag.
	force := r.URL.Query().Get("force") == "true"

	err := rs.settings.SetValue(r.Context(), schoolID, key, req.Value, changedBy, force)
	if err != nil {
		// Dedicated 409 path for the mode-switch block so the frontend can
		// surface the "daily end required" copy without heuristics. errors.Is
		// keeps the branch resilient to wrapping, unlike string equality.
		if errors.Is(err, settings.ErrPresenceModeSwitchBlocked) {
			common.RenderError(w, r, common.OperatorRejection(http.StatusConflict, common.CodeSettingsPresenceModeSwitchBlocked, settings.ErrPresenceModeSwitchBlocked.Error()))
			return
		}
		if errors.Is(err, careplan.ErrBookingAuthorityBlocked) {
			common.RenderError(w, r, common.OperatorRejection(http.StatusConflict, common.CodeSettingsBookingAuthorityBlocked, careplan.ErrBookingAuthorityBlocked.Error()))
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
	if rs.guardOperatorWrite(w, r, key) {
		return
	}
	if guardOperatorDirectManagedSettingWrite(w, r, key) {
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	changedBy := int64(claims.ID)

	if err := rs.settings.ResetValue(r.Context(), schoolID, key, changedBy); err != nil {
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
	if rs.guardOperatorWrite(w, r, key) {
		return
	}

	value, err := rs.settings.Reveal(r.Context(), schoolID, key)
	if err != nil {
		renderOperatorSettingsError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]any{"value": value}, "")
}

// renderOperatorSettingsError maps settings errors to operator HTTP responses.
func renderOperatorSettingsError(w http.ResponseWriter, r *http.Request, err error) {
	settingsErr, ok := errors.AsType[*settings.SettingsError](err)
	if !ok {
		// The cause is logged and reported, never sent to the client (#2519).
		common.RenderError(w, r, common.ErrorInternalServerWrap("Failed to change setting", err))
		return
	}

	inner := settingsErr.Unwrap()

	var defNotFound *settings.DefinitionNotFoundError
	var invalidValue *settings.InvalidValueError
	var permDenied *settings.PermissionDeniedError

	switch {
	case errors.As(inner, &defNotFound):
		common.RenderError(w, r, common.OperatorRejection(http.StatusNotFound, common.CodeSettingsNotFound, err.Error()))
	case errors.As(inner, &invalidValue):
		common.RenderError(w, r, common.OperatorInvalidRequestWithCode(err, common.CodeSettingsInvalidValue))
	case errors.As(inner, &permDenied):
		common.RenderError(w, r, common.OperatorForbidden(err.Error()))
	default:
		common.RenderError(w, r, common.ErrorInternalServerWrap("Failed to change setting", err))
	}
}
