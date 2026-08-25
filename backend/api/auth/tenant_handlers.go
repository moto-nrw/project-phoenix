package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
)

const tenantResolveSettingFailureMessage = "tenant resolve setting failed"

func resolveTenantGradeLevelMax(
	ctx context.Context,
	settings configSvc.SettingsService,
	tenantID int64,
) (int, error) {
	if settings == nil {
		return 0, errors.New("settings service not configured for grade_level_max")
	}
	value, err := settings.ResolveIntForTenant(ctx, tenantID, configModel.KeyEnrollmentGradeLevelMax)
	if err != nil {
		return 0, fmt.Errorf("resolve grade_level_max: %w", err)
	}
	if value < schoolclass.MinGradeLevel || value > schoolclass.MaxGradeLevel {
		return 0, fmt.Errorf(
			"grade_level_max %d outside %d..%d",
			value,
			schoolclass.MinGradeLevel,
			schoolclass.MaxGradeLevel,
		)
	}
	return value, nil
}

// resolveTenant handles GET /auth/tenant/resolve?slug={slug}
func (rs *Resource) resolveTenant(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(r.URL.Query().Get("slug"))
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("slug query parameter is required")))
		return
	}

	school, err := rs.SchoolService.GetSchoolBySubdomain(r.Context(), slug)
	if err != nil || school == nil || school.IsDeleted() || !school.Active {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("tenant not found")))
		return
	}

	// Parse settings JSON; fall back to empty object on invalid data
	settings := json.RawMessage(school.Settings)
	if !json.Valid(settings) {
		settings = json.RawMessage(`{}`)
	}

	var orgName string
	if school.Organization != nil {
		orgName = school.Organization.Name
	}

	// Shell settings resolve first: their batch includes grade_level_max, so
	// the request cache satisfies the hard-fail resolve below without a second
	// tenant transaction (issue #2065).
	resolved := rs.resolveTenantShellSettings(r.Context(), school.ID)

	// GradeLevelMax is a validation constraint, not an optional display hint.
	// The registry returns the legitimate default for tenants without an
	// override; a missing settings service, read error, or corrupt value must
	// fail the whole resolve contract instead of silently substituting grade 4.
	gradeLevelMax, err := resolveTenantGradeLevelMax(r.Context(), rs.SettingsService, school.ID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			http.StatusText(http.StatusInternalServerError),
			fmt.Errorf("tenant resolve: %w", err),
		))
		return
	}

	resp := &TenantResolveResponse{
		TenantID:                 school.ID,
		Slug:                     school.Slug,
		Name:                     school.Name,
		Subdomain:                school.Subdomain,
		OrganizationID:           school.OrganizationID,
		OrganizationName:         orgName,
		Hidden:                   school.Hidden,
		Settings:                 settings,
		PresenceMode:             resolved.presenceMode,
		StudentPhotosEnabled:     resolved.studentPhotosEnabled,
		NFCEnabled:               resolved.nfcEnabled,
		ParentMessagingEnabled:   resolved.parentMessagingEnabled,
		DisplayEnabled:           resolved.displayEnabled,
		GradeLevelMax:            gradeLevelMax,
		CareOfferingsEnabled:     resolved.careOfferingsEnabled,
		AttendanceWebEnabled:     resolved.attendanceWebEnabled,
		AttendanceLogEnabled:     resolved.attendanceLogEnabled,
		GroupMode:                resolved.groupMode,
		OperationalOverviewScope: resolved.overviewScope,
		ShowTimetableCounts:      resolved.showTimetableCounts,
		WaitlistEnabled:          resolved.waitlistEnabled,
	}

	common.Respond(w, r, http.StatusOK, resp, "Tenant resolved successfully")
}

func (rs *Resource) resolveTenantShellSettings(ctx context.Context, tenantID int64) tenantShellSettings {
	resolved := tenantShellSettings{
		presenceMode:           configModel.PresenceModeDetailed,
		parentMessagingEnabled: true,
		careOfferingsEnabled:   true,
		groupMode:              configModel.GroupModeFixedGroups,
		overviewScope:          configModel.OverviewScopeOwn,
		showTimetableCounts:    true,
		waitlistEnabled:        true,
	}
	if rs.SettingsService == nil {
		return resolved
	}

	keys := []string{
		configModel.KeyPresenceMode,
		configModel.KeyStudentPhotosEnabled,
		configModel.KeyAttendanceNFCEnabled,
		configModel.KeyDisplayEnabled,
		configModel.KeyEnrollmentCareOfferingsEnabled,
		configModel.KeyAttendanceWebEnabled,
		configModel.KeyAttendanceLogEnabled,
		configModel.KeyTimetableShowExpectedChildrenCount,
		configModel.KeyEnrollmentWaitlistEnabled,
		configModel.KeyGroupMode,
		configModel.KeyOperationalOverviewScope,
		configModel.KeyParentNotesEnabled,
		// Not read from this snapshot — prefetched so the hard-fail
		// resolveTenantGradeLevelMax call hits the request cache instead of
		// opening a second tenant transaction (issue #2065).
		configModel.KeyEnrollmentGradeLevelMax,
	}
	if batch, ok := rs.SettingsService.(interface {
		ResolveManyForTenant(context.Context, int64, []string) (*configSvc.SettingsSnapshot, error)
	}); ok {
		snapshot, err := batch.ResolveManyForTenant(ctx, tenantID, keys)
		if err != nil {
			logTenantResolveSettingFailure(ctx, tenantID, "tenant_shell", err, slog.LevelError)
			return resolved
		}
		if snapshot != nil {
			return resolveTenantShellSnapshot(ctx, tenantID, snapshot, resolved)
		}
	}

	resolved.presenceMode = configSvc.ResolvePresenceModeForTenant(ctx, rs.SettingsService, tenantID, nil)
	if value, err := rs.SettingsService.ResolveStringForTenant(ctx, tenantID, configModel.KeyStudentPhotosEnabled); err == nil {
		resolved.studentPhotosEnabled = value == "true"
	}
	if value, err := rs.SettingsService.ResolveBoolForTenant(ctx, tenantID, configModel.KeyAttendanceNFCEnabled); err == nil {
		resolved.nfcEnabled = value
	}
	if value, err := rs.SettingsService.ResolveBoolForTenant(ctx, tenantID, configModel.KeyDisplayEnabled); err == nil {
		resolved.displayEnabled = value
	}
	resolved.careOfferingsEnabled = rs.resolveTenantShellBool(ctx, tenantID, configModel.KeyEnrollmentCareOfferingsEnabled, false, slog.LevelError)
	resolved.attendanceWebEnabled = rs.resolveTenantShellBool(ctx, tenantID, configModel.KeyAttendanceWebEnabled, false, slog.LevelError)
	resolved.attendanceLogEnabled = rs.resolveTenantShellBool(ctx, tenantID, configModel.KeyAttendanceLogEnabled, false, slog.LevelError)
	resolved.showTimetableCounts = rs.resolveTenantShellBool(ctx, tenantID, configModel.KeyTimetableShowExpectedChildrenCount, true, slog.LevelWarn)
	resolved.waitlistEnabled = rs.resolveTenantShellBool(ctx, tenantID, configModel.KeyEnrollmentWaitlistEnabled, true, slog.LevelError)
	resolved.groupMode = rs.resolveTenantGroupMode(ctx, tenantID)
	resolved.overviewScope = rs.resolveTenantOverviewScope(ctx, tenantID)

	// Messaging compose visibility intentionally fails open so it stays in
	// lockstep with the unread badge, inbox row pills, and reply path.
	resolved.parentMessagingEnabled = parentmessaging.MessagingEnabledForTenant(ctx, rs.SettingsService, tenantID, nil)
	return resolved
}

func resolveTenantShellSnapshot(
	ctx context.Context,
	tenantID int64,
	snapshot *configSvc.SettingsSnapshot,
	resolved tenantShellSettings,
) tenantShellSettings {
	resolveBool := func(key string, fallback bool, level slog.Level) bool {
		value, err := snapshot.Bool(key)
		if err != nil {
			logTenantResolveSettingFailure(ctx, tenantID, key, err, level)
			return fallback
		}
		return value
	}
	resolveString := func(key, fallback string, level slog.Level) string {
		value, err := snapshot.String(key)
		if err != nil {
			logTenantResolveSettingFailure(ctx, tenantID, key, err, level)
			return fallback
		}
		return value
	}

	resolved.studentPhotosEnabled = resolveString(configModel.KeyStudentPhotosEnabled, "false", slog.LevelError) == "true"
	resolved.nfcEnabled = resolveBool(configModel.KeyAttendanceNFCEnabled, false, slog.LevelError)
	resolved.displayEnabled = resolveBool(configModel.KeyDisplayEnabled, false, slog.LevelError)
	resolved.careOfferingsEnabled = resolveBool(configModel.KeyEnrollmentCareOfferingsEnabled, false, slog.LevelError)
	resolved.attendanceWebEnabled = resolveBool(configModel.KeyAttendanceWebEnabled, false, slog.LevelError)
	resolved.attendanceLogEnabled = resolveBool(configModel.KeyAttendanceLogEnabled, false, slog.LevelError)
	resolved.showTimetableCounts = resolveBool(configModel.KeyTimetableShowExpectedChildrenCount, true, slog.LevelWarn)
	resolved.waitlistEnabled = resolveBool(configModel.KeyEnrollmentWaitlistEnabled, true, slog.LevelError)
	resolved.parentMessagingEnabled = resolveBool(configModel.KeyParentNotesEnabled, true, slog.LevelWarn)

	mode := resolveString(configModel.KeyPresenceMode, configModel.PresenceModeDetailed, slog.LevelWarn)
	if mode != "" {
		resolved.presenceMode = mode
	}
	groupMode := resolveString(configModel.KeyGroupMode, configModel.GroupModeFixedGroups, slog.LevelError)
	if groupMode == configModel.GroupModeOpenCare {
		resolved.groupMode = groupMode
	}
	resolved.overviewScope = normalizeOverviewScope(
		resolveString(configModel.KeyOperationalOverviewScope, configModel.OverviewScopeOwn, slog.LevelError),
	)
	return resolved
}

// normalizeOverviewScope keeps an unknown wire value from reaching the client
// as a scope it would have to guess about: anything unrecognised is "own".
func normalizeOverviewScope(value string) string {
	switch value {
	case configModel.OverviewScopeAdmins, configModel.OverviewScopeAllStaff:
		return value
	default:
		return configModel.OverviewScopeOwn
	}
}

func (rs *Resource) resolveTenantShellBool(ctx context.Context, tenantID int64, key string, fallback bool, level slog.Level) bool {
	value, err := rs.SettingsService.ResolveBoolForTenant(ctx, tenantID, key)
	if err == nil {
		return value
	}
	logTenantResolveSettingFailure(ctx, tenantID, key, err, level)
	return fallback
}

func (rs *Resource) resolveTenantGroupMode(ctx context.Context, tenantID int64) string {
	value, err := rs.SettingsService.ResolveStringForTenant(ctx, tenantID, configModel.KeyGroupMode)
	if err != nil {
		logTenantResolveSettingFailure(ctx, tenantID, configModel.KeyGroupMode, err, slog.LevelError)
		return configModel.GroupModeFixedGroups
	}
	if value == configModel.GroupModeOpenCare {
		return value
	}
	return configModel.GroupModeFixedGroups
}

func (rs *Resource) resolveTenantOverviewScope(ctx context.Context, tenantID int64) string {
	value, err := rs.SettingsService.ResolveStringForTenant(ctx, tenantID, configModel.KeyOperationalOverviewScope)
	if err != nil {
		logTenantResolveSettingFailure(ctx, tenantID, configModel.KeyOperationalOverviewScope, err, slog.LevelError)
		return configModel.OverviewScopeOwn
	}
	return normalizeOverviewScope(value)
}

func logTenantResolveSettingFailure(ctx context.Context, tenantID int64, key string, err error, level slog.Level) {
	slog.Default().LogAttrs(ctx, level, tenantResolveSettingFailureMessage,
		slog.Int64("tenant_id", tenantID),
		slog.String("key", key),
		slog.String("error", err.Error()))
}

// switchTenant handles POST /auth/switch-tenant
func (rs *Resource) switchTenant(w http.ResponseWriter, r *http.Request) {
	req := &SwitchTenantRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Get account ID from JWT claims
	claims := jwt.ClaimsFromCtx(r.Context())

	accessToken, refreshToken, err := rs.AuthService.SwitchTenant(r.Context(), int64(claims.ID), req.TenantSlug)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountNotFound))
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrAccountInactive))
			case errors.Is(authErr.Err, authService.ErrTenantNotFound):
				common.RenderError(w, r, common.ErrorNotFound(authService.ErrTenantNotFound))
			case errors.Is(authErr.Err, authService.ErrTenantAccessDenied):
				common.RenderError(w, r, common.ErrorUnauthorized(authService.ErrTenantAccessDenied))
			case errors.Is(authErr.Err, authService.ErrMustUseSchoolPortal):
				common.RenderError(w, r, common.ErrorForbiddenWithCode(
					authService.ErrMustUseSchoolPortal, "use_school_portal"))
			default:
				common.RenderError(w, r, common.ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Return tokens in the same format as login
	render.JSON(w, r, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// listAccountTenants handles GET /auth/account/tenants
func (rs *Resource) listAccountTenants(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())

	schools, err := rs.SchoolService.ListActiveSchoolsByAccountID(r.Context(), int64(claims.ID))
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]AccountTenantResponse, 0, len(schools))
	for _, school := range schools {
		var orgName string
		if school.Organization != nil {
			orgName = school.Organization.Name
		}
		responses = append(responses, AccountTenantResponse{
			TenantID:         school.ID,
			Slug:             school.Slug,
			Name:             school.Name,
			Subdomain:        school.Subdomain,
			OrganizationID:   school.OrganizationID,
			OrganizationName: orgName,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Account tenants retrieved successfully")
}

// listTenants handles GET /auth/tenants (public, no auth required).
// Uses ListPublic to exclude hidden schools from the public landing page.
// Hidden schools remain accessible via direct subdomain link.
func (rs *Resource) listTenants(w http.ResponseWriter, r *http.Request) {
	schools, err := rs.SchoolService.ListPublicSchools(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]PublicTenantResponse, 0, len(schools))
	for _, school := range schools {
		var orgName string
		if school.Organization != nil {
			orgName = school.Organization.Name
		}
		responses = append(responses, PublicTenantResponse{
			Slug:             school.Slug,
			Name:             school.Name,
			Subdomain:        school.Subdomain,
			OrganizationName: orgName,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Tenants retrieved successfully")
}
