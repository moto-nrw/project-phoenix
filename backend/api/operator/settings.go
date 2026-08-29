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
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// errAdminOnlyForOperator explains why an operator may not touch an
// AccessAdminOnly setting (e.g. the OGS device PIN). Surfaced as HTTP 403.
const errAdminOnlyForOperator = "this setting is admin-only and cannot be modified by operators"

const errLegalAGBDocumentManagedByUpload = "AGB document URL is managed by the file upload endpoint"

// guardOperatorWrite blocks the operator from set/reset/reveal on AccessAdminOnly settings.
// The access-policy decision lives in the settings service (CheckOperatorWritable);
// this maps the returned error to the operator HTTP response. Returns true when
// the handler should abort (response has already been written).
func (rs *SettingsResource) guardOperatorWrite(w http.ResponseWriter, r *http.Request, key string) bool {
	err := rs.settingsService.CheckOperatorWritable(key)
	if err == nil {
		return false
	}
	var notFound *configSvc.DefinitionNotFoundError
	if errors.As(err, &notFound) {
		render.Render(w, r, ErrNotFound(fmt.Sprintf("setting %q not found", key))) //nolint:errcheck
		return true
	}
	render.Render(w, r, ErrForbidden(errAdminOnlyForOperator)) //nolint:errcheck
	return true
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
	// operatorSettings owns the set/reset orchestration (presence-mode guard,
	// tenant transaction, side-effect hook, SSE broadcast).
	operatorSettings configSvc.OperatorSettingsService
	// schoolService lets the resource emit `school_slug` in set/reset
	// responses so the frontend operator proxy can bust the slug-keyed
	// `tenant-${slug}` Next.js cache after tenant-resolve-affecting
	// toggles (currently only operations.student_photos_enabled).
	schoolService platformSvc.SchoolService
	// onValueSet runs inside the write transaction and returns an optional
	// post-commit closure. Mirrors the tenant SettingsResource.OnValueSet
	// contract so side effects apply uniformly regardless of who flipped the
	// value; passed to the operatorSettings service on each write.
	onValueSet    configSvc.OperatorValueSetHook
	careLifecycle usersSvc.CareLifecycleService
}

type operatorSettingsRuntime struct{ db *bun.DB }

func (r operatorSettingsRuntime) WithinTenant(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
	return tenant.WithTenantTx(ctx, r.db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

func (operatorSettingsRuntime) AfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}

func (operatorSettingsRuntime) Today() configModel.CalendarDate { return timezone.TodayDate() }

// NewSettingsResource creates a new operator settings resource. broadcaster
// emits the cross-origin tenant_settings_changed SSE event so open tenant
// tabs invalidate their settings caches when an operator flips a value.
// schoolService enriches the response with the school's slug so the frontend
// operator proxy can additionally bust the `tenant-${slug}` Next.js cache
// for tenant-resolve-affecting settings (e.g. student_photos_enabled).
// broadcaster and activeService are optional — nil disables the corresponding
// mechanism (broadcast fan-out / presence-mode guard).
func NewSettingsResource(
	svc configSvc.SettingsService,
	db *bun.DB,
	broadcaster realtime.Broadcaster,
	schoolService platformSvc.SchoolService,
	activeService activeSvc.Service,
	lifecycle usersSvc.CareLifecycleService,
) *SettingsResource {
	return &SettingsResource{
		settingsService: svc,
		db:              db,
		operatorSettings: configSvc.NewOperatorSettingsService(
			svc,
			operatorSettingsRuntime{db: db},
			settingsChangedNotifier(broadcaster),
			activeService,
			slog.Default(),
		),
		schoolService: schoolService,
		careLifecycle: lifecycle,
	}
}

func settingsChangedNotifier(broadcaster realtime.Broadcaster) configSvc.SettingsChangedNotifier {
	if broadcaster == nil {
		return nil
	}
	return func(_ context.Context, tenantID int64, key string) {
		event := realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{Source: &key})
		_ = broadcaster.BroadcastToTenant(tenantID, event)
	}
}

// OnValueSet registers a callback that runs after a setting value change is
// validated and persisted. The callback runs inside the tenant transaction;
// the optional postCommit closure it returns runs only after a successful
// commit. Mirrors the tenant SettingsResource.OnValueSet contract so side
// effects apply uniformly regardless of who flipped the value.
func (rs *SettingsResource) OnValueSet(fn configSvc.OperatorValueSetHook) {
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

// GetBookingAuthorityImpact shows the facts an operator must review before
// enabling booking-led care. The write path repeats this evaluation under the
// booking-write lock, so a stale preview cannot bypass the guard.
func (rs *SettingsResource) GetBookingAuthorityImpact(w http.ResponseWriter, r *http.Request) {
	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}
	if rs.careLifecycle == nil {
		render.Render(w, r, ErrInternal("Booking authority impact service is not configured")) //nolint:errcheck
		return
	}
	var impact *usersSvc.BookingAuthorityImpact
	err := tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(ctx context.Context, _ bun.Tx) error {
		var impactErr error
		impact, impactErr = rs.careLifecycle.PreviewBookingAuthorityImpact(ctx, timezone.TodayDate())
		return impactErr
	})
	if err != nil {
		render.Render(w, r, ErrInternal("Failed to review booking authority impact")) //nolint:errcheck
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
		render.Render(w, r, ErrInvalidRequest(err)) //nolint:errcheck
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	changedBy := int64(claims.ID)
	// ?force=true lets operators bypass the presence-mode switch guard when
	// doing maintenance recovery (e.g. stuck attendance rows blocking the
	// switch). Everything else ignores the flag.
	force := r.URL.Query().Get("force") == "true"

	err := rs.operatorSettings.SetValue(r.Context(), schoolID, key, req.Value, changedBy, force, rs.onValueSet)
	if err != nil {
		// Dedicated 409 path for the mode-switch block so the frontend can
		// surface the "daily end required" copy without heuristics. errors.Is
		// keeps the branch resilient to wrapping, unlike string equality.
		if errors.Is(err, configSvc.ErrPresenceModeSwitchBlocked) {
			render.Render(w, r, ErrConflict(configSvc.ErrPresenceModeSwitchBlocked.Error())) //nolint:errcheck
			return
		}
		if errors.Is(err, usersSvc.ErrBookingAuthorityBlocked) {
			render.Render(w, r, ErrConflict(usersSvc.ErrBookingAuthorityBlocked.Error())) //nolint:errcheck
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

	if err := rs.operatorSettings.ResetValue(r.Context(), schoolID, key, changedBy, rs.onValueSet); err != nil {
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
