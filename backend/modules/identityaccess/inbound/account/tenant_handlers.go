package account

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
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/settings"
)

const tenantResolveSettingFailureMessage = "tenant resolve setting failed"

// Bounds of enrollment.grade_level_max, the same range the settings registry
// validates on write. The read side re-checks them so a corrupt override
// fails the tenant shell instead of reaching the public form.
const (
	minGradeLevel = 1
	maxGradeLevel = 13
)

func resolveTenantGradeLevelMax(
	ctx context.Context,
	reader settings.TenantReader,
	tenantID int64,
) (int, error) {
	if reader == nil {
		return 0, errors.New("settings service not configured for grade_level_max")
	}
	value, err := reader.ResolveIntForTenant(ctx, tenantID, settings.KeyEnrollmentGradeLevelMax)
	if err != nil {
		return 0, fmt.Errorf("resolve grade_level_max: %w", err)
	}
	if value < minGradeLevel || value > maxGradeLevel {
		return 0, fmt.Errorf(
			"grade_level_max %d outside %d..%d",
			value,
			minGradeLevel,
			maxGradeLevel,
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
	if err != nil || school == nil || school.Deleted || !school.Active {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("tenant not found")))
		return
	}

	// Shell settings resolve first: their batch includes grade_level_max, so
	// the request cache satisfies the hard-fail resolve below without a second
	// tenant transaction (issue #2065).
	resolved, err := rs.resolveTenantShellSettings(r.Context(), school.ID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			http.StatusText(http.StatusInternalServerError),
			fmt.Errorf("tenant resolve: %w", err),
		))
		return
	}

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

	common.Respond(w, r, http.StatusOK, newTenantResolveResponse(school, resolved, gradeLevelMax), "Tenant resolved successfully")
}

func newTenantResolveResponse(school *TenantSchool, resolved tenantShellSettings, gradeLevelMax int) *TenantResolveResponse {
	// Parse settings JSON; fall back to empty object on invalid data
	schoolSettings := json.RawMessage(school.Settings)
	if !json.Valid(schoolSettings) {
		schoolSettings = json.RawMessage(`{}`)
	}

	return &TenantResolveResponse{
		TenantID:                   school.ID,
		Slug:                       school.Slug,
		Name:                       school.Name,
		Subdomain:                  school.Subdomain,
		OrganizationID:             school.OrganizationID,
		OrganizationName:           school.OrganizationName,
		Hidden:                     school.Hidden,
		Settings:                   schoolSettings,
		PresenceMode:               resolved.presenceMode,
		StudentPhotosEnabled:       resolved.studentPhotosEnabled,
		NFCEnabled:                 resolved.nfcEnabled,
		ParentMessagingEnabled:     resolved.parentMessagingEnabled,
		StaffMessagingEnabled:      resolved.staffMessagingEnabled,
		DisplayEnabled:             resolved.displayEnabled,
		CalDAVEnabled:              resolved.calDAVEnabled,
		GradeLevelMax:              gradeLevelMax,
		CareOfferingsEnabled:       resolved.careOfferingsEnabled,
		AttendanceWebEnabled:       resolved.attendanceWebEnabled,
		AttendanceLogEnabled:       resolved.attendanceLogEnabled,
		GroupMode:                  resolved.groupMode,
		OperationalOverviewScope:   resolved.overviewScope,
		AttendanceEditScope:        resolved.attendanceEditScope,
		ParentRequestReasonPolicy:  resolved.reasonPolicy,
		ShowTimetableCounts:        resolved.showTimetableCounts,
		TimetableEnabled:           resolved.timetableEnabled,
		WaitlistEnabled:            resolved.waitlistEnabled,
		EmergencyHealthInfoEnabled: resolved.emergencyHealthInfo,
		AnalyticsFreigabe:          resolved.analyticsFreigabe,
		// Without the Freigabe the sample has no effect; the client never sees
		// a recording share for a school that did not agree.
		AnalyticsRecordingSamplePercent: analyticsSamplePercent(resolved),
	}
}

func analyticsSamplePercent(resolved tenantShellSettings) int {
	if !resolved.analyticsFreigabe {
		return 0
	}
	return resolved.analyticsSamplePercent
}

func defaultTenantShellSettings() tenantShellSettings {
	return tenantShellSettings{
		presenceMode:           settings.PresenceModeDetailed,
		parentMessagingEnabled: true,
		careOfferingsEnabled:   true,
		groupMode:              settings.GroupModeFixedGroups,
		overviewScope:          settings.OverviewScopeOwn,
		attendanceEditScope:    settings.AttendanceEditScopeOwn,
		reasonPolicy:           settings.ReasonPolicyBoth,
		showTimetableCounts:    true,
		timetableEnabled:       true,
		waitlistEnabled:        true,
		emergencyHealthInfo:    false,
	}
}

// tenantShellSettingKeys lists every setting the tenant shell reads, for the
// one batched resolve.
func tenantShellSettingKeys() []string {
	return []string{
		settings.KeyPresenceMode,
		settings.KeyStudentPhotosEnabled,
		settings.KeyAttendanceNFCEnabled,
		settings.KeyDisplayEnabled,
		settings.KeyCalendarCalDAVEnabled,
		settings.KeyEnrollmentCareOfferingsEnabled,
		settings.KeyAttendanceWebEnabled,
		settings.KeyAttendanceLogEnabled,
		settings.KeyTimetableShowExpectedChildrenCount,
		settings.KeyTimetableEnabled,
		settings.KeyEnrollmentWaitlistEnabled,
		settings.KeyGroupMode,
		settings.KeyOperationalOverviewScope,
		settings.KeyAttendanceEditScope,
		settings.KeyParentRequestReasonPolicy,
		settings.KeyParentNotesEnabled,
		settings.KeyEmergencyListHealthInfo,
		settings.KeyStaffMessagingEnabled,
		settings.KeyAnalyticsFreigabe,
		settings.KeyAnalyticsRecordingSamplePercent,
		// Not read from this snapshot — prefetched so the hard-fail
		// resolveTenantGradeLevelMax call hits the request cache instead of
		// opening a second tenant transaction (issue #2065).
		settings.KeyEnrollmentGradeLevelMax,
	}
}

func (rs *Resource) resolveTenantShellSettings(ctx context.Context, tenantID int64) (tenantShellSettings, error) {
	resolved := defaultTenantShellSettings()
	if rs.SettingsService == nil {
		return resolved, nil
	}

	if batch, ok := rs.SettingsService.(interface {
		ResolveManyForTenant(context.Context, int64, []string) (*settings.Snapshot, error)
	}); ok {
		snapshot, err := batch.ResolveManyForTenant(ctx, tenantID, tenantShellSettingKeys())
		if err != nil {
			logTenantResolveSettingFailure(ctx, tenantID, "tenant_shell", err, slog.LevelError)
			return resolved, fmt.Errorf("resolve tenant shell settings: %w", err)
		}
		if snapshot != nil {
			return resolveTenantShellSnapshot(ctx, tenantID, snapshot, resolved)
		}
	}
	return rs.resolveTenantShellSettingsOneByOne(ctx, tenantID, resolved)
}

// resolveTenantShellSettingsOneByOne is the fallback for a settings service
// without a batched resolve: each key is resolved on its own.
func (rs *Resource) resolveTenantShellSettingsOneByOne(ctx context.Context, tenantID int64, resolved tenantShellSettings) (tenantShellSettings, error) {
	resolved.presenceMode = settings.ResolvePresenceModeForTenant(ctx, rs.SettingsService, tenantID)
	if value, err := rs.SettingsService.ResolveStringForTenant(ctx, tenantID, settings.KeyStudentPhotosEnabled); err == nil {
		resolved.studentPhotosEnabled = value == "true"
	}
	if value, err := rs.SettingsService.ResolveBoolForTenant(ctx, tenantID, settings.KeyAttendanceNFCEnabled); err == nil {
		resolved.nfcEnabled = value
	}
	if value, err := rs.SettingsService.ResolveBoolForTenant(ctx, tenantID, settings.KeyDisplayEnabled); err == nil {
		resolved.displayEnabled = value
	}
	resolved.calDAVEnabled = rs.resolveTenantShellBool(ctx, tenantID, settings.KeyCalendarCalDAVEnabled, false, slog.LevelError)
	resolved.careOfferingsEnabled = rs.resolveTenantShellBool(ctx, tenantID, settings.KeyEnrollmentCareOfferingsEnabled, false, slog.LevelError)
	resolved.attendanceWebEnabled = rs.resolveTenantShellBool(ctx, tenantID, settings.KeyAttendanceWebEnabled, false, slog.LevelError)
	resolved.attendanceLogEnabled = rs.resolveTenantShellBool(ctx, tenantID, settings.KeyAttendanceLogEnabled, false, slog.LevelError)
	resolved.showTimetableCounts = rs.resolveTenantShellBool(ctx, tenantID, settings.KeyTimetableShowExpectedChildrenCount, true, slog.LevelWarn)
	resolved.timetableEnabled = rs.resolveTenantShellBool(ctx, tenantID, settings.KeyTimetableEnabled, true, slog.LevelWarn)
	resolved.waitlistEnabled = rs.resolveTenantShellBool(ctx, tenantID, settings.KeyEnrollmentWaitlistEnabled, true, slog.LevelError)
	resolved.emergencyHealthInfo = rs.resolveTenantShellBool(ctx, tenantID, settings.KeyEmergencyListHealthInfo, false, slog.LevelWarn)
	resolved.groupMode = rs.resolveTenantGroupMode(ctx, tenantID)
	resolved.overviewScope = rs.resolveTenantOverviewScope(ctx, tenantID)
	resolved.attendanceEditScope = rs.resolveTenantAttendanceEditScope(ctx, tenantID)
	resolved.reasonPolicy = rs.resolveTenantReasonPolicy(ctx, tenantID)
	resolved.analyticsFreigabe = rs.resolveTenantShellBool(ctx, tenantID, settings.KeyAnalyticsFreigabe, false, slog.LevelError)
	// The sample has no effect without the Freigabe; it is read only with it.
	if resolved.analyticsFreigabe {
		if value, err := rs.SettingsService.ResolveIntForTenant(ctx, tenantID, settings.KeyAnalyticsRecordingSamplePercent); err == nil {
			resolved.analyticsSamplePercent = value
		} else {
			logTenantResolveSettingFailure(ctx, tenantID, settings.KeyAnalyticsRecordingSamplePercent, err, slog.LevelError)
		}
	}

	// Messaging compose visibility intentionally fails open so it stays in
	// lockstep with the unread badge, inbox row pills, and reply path.
	resolved.parentMessagingEnabled = rs.resolveTenantShellBool(ctx, tenantID, settings.KeyParentNotesEnabled, true, slog.LevelWarn)
	// The internal Team-Chat (#2598) must resolve cleanly: false is a real
	// switch-off, while resolver errors must not masquerade as policy.
	staffMessagingEnabled, err := rs.resolveRequiredTenantShellBool(ctx, tenantID, settings.KeyStaffMessagingEnabled)
	if err != nil {
		return resolved, err
	}
	resolved.staffMessagingEnabled = staffMessagingEnabled
	return resolved, nil
}

func resolveTenantShellSnapshot(
	ctx context.Context,
	tenantID int64,
	snapshot *settings.Snapshot,
	resolved tenantShellSettings,
) (tenantShellSettings, error) {
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

	resolved.studentPhotosEnabled = resolveString(settings.KeyStudentPhotosEnabled, "false", slog.LevelError) == "true"
	resolved.nfcEnabled = resolveBool(settings.KeyAttendanceNFCEnabled, false, slog.LevelError)
	resolved.displayEnabled = resolveBool(settings.KeyDisplayEnabled, false, slog.LevelError)
	resolved.calDAVEnabled = resolveBool(settings.KeyCalendarCalDAVEnabled, false, slog.LevelError)
	resolved.careOfferingsEnabled = resolveBool(settings.KeyEnrollmentCareOfferingsEnabled, false, slog.LevelError)
	resolved.attendanceWebEnabled = resolveBool(settings.KeyAttendanceWebEnabled, false, slog.LevelError)
	resolved.attendanceLogEnabled = resolveBool(settings.KeyAttendanceLogEnabled, false, slog.LevelError)
	resolved.showTimetableCounts = resolveBool(settings.KeyTimetableShowExpectedChildrenCount, true, slog.LevelWarn)
	resolved.timetableEnabled = resolveBool(settings.KeyTimetableEnabled, true, slog.LevelWarn)
	resolved.waitlistEnabled = resolveBool(settings.KeyEnrollmentWaitlistEnabled, true, slog.LevelError)
	resolved.parentMessagingEnabled = resolveBool(settings.KeyParentNotesEnabled, true, slog.LevelWarn)
	resolved.emergencyHealthInfo = resolveBool(settings.KeyEmergencyListHealthInfo, false, slog.LevelWarn)
	resolved.analyticsFreigabe, resolved.analyticsSamplePercent = resolveTenantAnalyticsSnapshot(ctx, tenantID, snapshot)
	staffMessagingEnabled, err := snapshot.Bool(settings.KeyStaffMessagingEnabled)
	if err != nil {
		logTenantResolveSettingFailure(ctx, tenantID, settings.KeyStaffMessagingEnabled, err, slog.LevelError)
		return resolved, fmt.Errorf("resolve %s: %w", settings.KeyStaffMessagingEnabled, err)
	}
	resolved.staffMessagingEnabled = staffMessagingEnabled

	mode := resolveString(settings.KeyPresenceMode, settings.PresenceModeDetailed, slog.LevelWarn)
	if mode != "" {
		resolved.presenceMode = mode
	}
	groupMode := resolveString(settings.KeyGroupMode, settings.GroupModeFixedGroups, slog.LevelError)
	if groupMode == settings.GroupModeOpenCare {
		resolved.groupMode = groupMode
	}
	resolved.overviewScope = normalizeOverviewScope(
		resolveString(settings.KeyOperationalOverviewScope, settings.OverviewScopeOwn, slog.LevelError),
	)
	resolved.attendanceEditScope = normalizeAttendanceEditScope(
		resolveString(settings.KeyAttendanceEditScope, settings.AttendanceEditScopeOwn, slog.LevelError),
	)
	resolved.reasonPolicy = normalizeReasonPolicy(
		resolveString(settings.KeyParentRequestReasonPolicy, settings.ReasonPolicyBoth, slog.LevelError),
	)
	return resolved, nil
}

// resolveTenantAnalyticsSnapshot reads the Analyse-Freigabe and its recording
// share (#3603). Both fail closed: a value nobody can read never turns
// recording or pseudonymous IDs on, and an unreadable share records nothing.
func resolveTenantAnalyticsSnapshot(ctx context.Context, tenantID int64, snapshot *settings.Snapshot) (bool, int) {
	freigabe, err := snapshot.Bool(settings.KeyAnalyticsFreigabe)
	if err != nil {
		logTenantResolveSettingFailure(ctx, tenantID, settings.KeyAnalyticsFreigabe, err, slog.LevelError)
		return false, 0
	}
	percent, err := snapshot.Int(settings.KeyAnalyticsRecordingSamplePercent)
	if err != nil {
		logTenantResolveSettingFailure(ctx, tenantID, settings.KeyAnalyticsRecordingSamplePercent, err, slog.LevelError)
		return freigabe, 0
	}
	return freigabe, percent
}

// normalizeOverviewScope keeps an unknown wire value from reaching the client
// as a scope it would have to guess about: anything unrecognised is "own".
func normalizeOverviewScope(value string) string {
	switch value {
	case settings.OverviewScopeAdmins, settings.OverviewScopeAllStaff:
		return value
	default:
		return settings.OverviewScopeOwn
	}
}

// normalizeReasonPolicy keeps an unknown wire value from reaching the client:
// anything unrecognised becomes "both", the strictest policy, so the UI asks
// for a reason the server might require rather than hiding a required field.
func normalizeReasonPolicy(value string) string {
	switch value {
	case settings.ReasonPolicyNobody, settings.ReasonPolicyGuardians, settings.ReasonPolicyStaff:
		return value
	default:
		return settings.ReasonPolicyBoth
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

func (rs *Resource) resolveRequiredTenantShellBool(ctx context.Context, tenantID int64, key string) (bool, error) {
	value, err := rs.SettingsService.ResolveBoolForTenant(ctx, tenantID, key)
	if err != nil {
		logTenantResolveSettingFailure(ctx, tenantID, key, err, slog.LevelError)
		return false, fmt.Errorf("resolve %s: %w", key, err)
	}
	return value, nil
}

func (rs *Resource) resolveTenantGroupMode(ctx context.Context, tenantID int64) string {
	value, err := rs.SettingsService.ResolveStringForTenant(ctx, tenantID, settings.KeyGroupMode)
	if err != nil {
		logTenantResolveSettingFailure(ctx, tenantID, settings.KeyGroupMode, err, slog.LevelError)
		return settings.GroupModeFixedGroups
	}
	if value == settings.GroupModeOpenCare {
		return value
	}
	return settings.GroupModeFixedGroups
}

func (rs *Resource) resolveTenantOverviewScope(ctx context.Context, tenantID int64) string {
	value, err := rs.SettingsService.ResolveStringForTenant(ctx, tenantID, settings.KeyOperationalOverviewScope)
	if err != nil {
		logTenantResolveSettingFailure(ctx, tenantID, settings.KeyOperationalOverviewScope, err, slog.LevelError)
		return settings.OverviewScopeOwn
	}
	return normalizeOverviewScope(value)
}

func (rs *Resource) resolveTenantAttendanceEditScope(ctx context.Context, tenantID int64) string {
	value, err := rs.SettingsService.ResolveStringForTenant(ctx, tenantID, settings.KeyAttendanceEditScope)
	if err != nil {
		logTenantResolveSettingFailure(ctx, tenantID, settings.KeyAttendanceEditScope, err, slog.LevelError)
		return settings.AttendanceEditScopeOwn
	}
	return normalizeAttendanceEditScope(value)
}

// normalizeAttendanceEditScope keeps an unknown wire value from widening the
// client: anything unrecognised is "own".
func normalizeAttendanceEditScope(value string) string {
	if value == settings.AttendanceEditScopeAllStaff {
		return value
	}
	return settings.AttendanceEditScopeOwn
}

func (rs *Resource) resolveTenantReasonPolicy(ctx context.Context, tenantID int64) string {
	value, err := rs.SettingsService.ResolveStringForTenant(ctx, tenantID, settings.KeyParentRequestReasonPolicy)
	if err != nil {
		logTenantResolveSettingFailure(ctx, tenantID, settings.KeyParentRequestReasonPolicy, err, slog.LevelError)
		return settings.ReasonPolicyBoth
	}
	return normalizeReasonPolicy(value)
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

	if rs.Sessions == nil {
		common.RenderError(w, r, common.ErrorServiceUnavailable(errors.New("tenant switch unavailable")))
		return
	}
	accessToken, refreshToken, err := rs.Sessions.SwitchTenant(r.Context(), int64(claims.ID), req.TenantSlug, claims.FamilyID)
	if err != nil {
		var authErr *identityaccess.AuthenticationError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(err, identityaccess.ErrAccountNotFound):
				common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrAccountNotFound))
			case errors.Is(err, identityaccess.ErrAccountInactive):
				common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrAccountInactive))
			case errors.Is(err, identityaccess.ErrTenantNotFound):
				common.RenderError(w, r, common.ErrorNotFound(identityaccess.ErrTenantNotFound))
			case errors.Is(err, identityaccess.ErrTenantAccessDenied):
				common.RenderError(w, r, common.ErrorUnauthorized(identityaccess.ErrTenantAccessDenied))
			case errors.Is(err, identityaccess.ErrMustUseSchoolPortal):
				common.RenderError(w, r, common.ErrorForbiddenWithCode(
					identityaccess.ErrMustUseSchoolPortal, "use_school_portal"))
			case errors.Is(err, identityaccess.ErrDemoSessionTenantLocked):
				common.RenderError(w, r, common.ErrorForbiddenWithCode(
					identityaccess.ErrDemoSessionTenantLocked, "demo_session"))
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
		responses = append(responses, AccountTenantResponse{
			TenantID:         school.ID,
			Slug:             school.Slug,
			Name:             school.Name,
			Subdomain:        school.Subdomain,
			OrganizationID:   school.OrganizationID,
			OrganizationName: school.OrganizationName,
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
		responses = append(responses, PublicTenantResponse{
			Slug:             school.Slug,
			Name:             school.Name,
			Subdomain:        school.Subdomain,
			OrganizationName: school.OrganizationName,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Tenants retrieved successfully")
}
