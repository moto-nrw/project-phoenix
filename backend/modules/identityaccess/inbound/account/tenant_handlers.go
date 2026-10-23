package account

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/settings"
)

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
		TenantID:                  school.ID,
		Slug:                      school.Slug,
		Name:                      school.Name,
		Subdomain:                 school.Subdomain,
		OrganizationID:            school.OrganizationID,
		OrganizationName:          school.OrganizationName,
		Hidden:                    school.Hidden,
		Settings:                  schoolSettings,
		PresenceMode:              resolved.presenceMode,
		StudentPhotosEnabled:      resolved.studentPhotosEnabled,
		NFCEnabled:                resolved.nfcEnabled,
		ParentMessagingEnabled:    resolved.parentMessagingEnabled,
		StaffMessagingEnabled:     resolved.staffMessagingEnabled,
		DisplayEnabled:            resolved.displayEnabled,
		CalDAVEnabled:             resolved.calDAVEnabled,
		GradeLevelMax:             gradeLevelMax,
		CareOfferingsEnabled:      resolved.careOfferingsEnabled,
		AttendanceWebEnabled:      resolved.attendanceWebEnabled,
		AttendanceLogEnabled:      resolved.attendanceLogEnabled,
		GroupMode:                 resolved.groupMode,
		OperationalOverviewScope:  resolved.overviewScope,
		AttendanceEditScope:       resolved.attendanceEditScope,
		ParentRequestReasonPolicy: resolved.reasonPolicy,
		ShowTimetableCounts:       resolved.showTimetableCounts,
		TimetableEnabled:          resolved.timetableEnabled,
		MealPlanEnabled:           resolved.mealPlanEnabled,
		MealRegistrationEnabled:   resolved.mealRegistrationEnabled,
		WaitlistEnabled:           resolved.waitlistEnabled,
		AnalyticsEnabled:          resolved.analyticsEnabled,
		AnalyticsFreigabe:         resolved.analyticsEnabled && resolved.analyticsFreigabe,
		// Without the Freigabe the sample has no effect; the client never sees
		// a recording share for a school that did not agree.
		AnalyticsRecordingSamplePercent:   analyticsSamplePercent(resolved),
		EarlyCheckoutNoteEnabled:          resolved.earlyCheckoutNote,
		EarlyCheckoutNoteToleranceMinutes: resolved.earlyCheckoutTolerance,
		WeekendFollowsFriday:              resolved.weekendFollowsFriday,
		ChildSearchOnlyNavigation:         resolved.childSearchOnlyNavigation,
	}
}

func analyticsSamplePercent(resolved tenantShellSettings) int {
	if !resolved.analyticsEnabled || !resolved.analyticsFreigabe {
		return 0
	}
	return resolved.analyticsSamplePercent
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
		settings.KeyMealPlanEnabled,
		settings.KeyMealRegistrationEnabled,
		settings.KeyEnrollmentWaitlistEnabled,
		settings.KeyGroupMode,
		settings.KeyOperationalOverviewScope,
		settings.KeyAttendanceEditScope,
		settings.KeyParentRequestReasonPolicy,
		settings.KeyParentNotesEnabled,
		settings.KeyStaffMessagingEnabled,
		settings.KeyAnalyticsEnabled,
		settings.KeyAnalyticsFreigabe,
		settings.KeyAnalyticsRecordingSamplePercent,
		settings.KeyEarlyCheckoutNoteEnabled,
		settings.KeyEarlyCheckoutNoteToleranceMinutes,
		settings.KeyWeekendFollowsFriday,
		settings.KeyChildSearchOnlyNavigation,
		// Not read from this snapshot — prefetched so the hard-fail
		// resolveTenantGradeLevelMax call hits the request cache instead of
		// opening a second tenant transaction (issue #2065).
		settings.KeyEnrollmentGradeLevelMax,
	}
}

func (rs *Resource) resolveTenantShellSettings(ctx context.Context, tenantID int64) (tenantShellSettings, error) {
	if batch, ok := rs.SettingsService.(interface {
		ResolveManyForTenant(context.Context, int64, []string) (*settings.Snapshot, error)
	}); ok {
		snapshot, err := batch.ResolveManyForTenant(ctx, tenantID, tenantShellSettingKeys())
		if err != nil {
			return tenantShellSettings{}, fmt.Errorf("resolve tenant shell settings: %w", err)
		}
		if snapshot == nil {
			return tenantShellSettings{}, errors.New("tenant shell settings snapshot is missing")
		}
		return resolveTenantShellValues(snapshot.Bool, snapshot.String, snapshot.Int)
	}
	return resolveTenantShellValues(
		func(key string) (bool, error) { return rs.SettingsService.ResolveBoolForTenant(ctx, tenantID, key) },
		func(key string) (string, error) { return rs.SettingsService.ResolveStringForTenant(ctx, tenantID, key) },
		func(key string) (int, error) { return rs.SettingsService.ResolveIntForTenant(ctx, tenantID, key) },
	)
}

// Both read paths use the same contract: an unreadable setting fails the shell,
// never a successful response assembled from local policy defaults.
func resolveTenantShellValues(readBool func(string) (bool, error), readString func(string) (string, error), readInt func(string) (int, error)) (tenantShellSettings, error) {
	var resolved tenantShellSettings
	if err := resolved.resolveFeatureFlags(readBool); err != nil {
		return tenantShellSettings{}, err
	}
	stringFields := []struct {
		key    string
		target *string
	}{
		{settings.KeyPresenceMode, &resolved.presenceMode},
		{settings.KeyGroupMode, &resolved.groupMode},
		{settings.KeyOperationalOverviewScope, &resolved.overviewScope},
		{settings.KeyAttendanceEditScope, &resolved.attendanceEditScope},
		{settings.KeyParentRequestReasonPolicy, &resolved.reasonPolicy},
	}
	for _, field := range stringFields {
		value, err := readString(field.key)
		if err != nil {
			return tenantShellSettings{}, fmt.Errorf("resolve %s: %w", field.key, err)
		}
		*field.target = value
	}
	resolved.overviewScope = normalizeOverviewScope(resolved.overviewScope)
	resolved.attendanceEditScope = normalizeAttendanceEditScope(resolved.attendanceEditScope)
	resolved.reasonPolicy = normalizeReasonPolicy(resolved.reasonPolicy)
	if resolved.groupMode != settings.GroupModeOpenCare {
		resolved.groupMode = settings.GroupModeFixedGroups
	}
	if resolved.presenceMode == "" {
		resolved.presenceMode = settings.PresenceModeDetailed
	}
	if resolved.analyticsFreigabe {
		value, err := readInt(settings.KeyAnalyticsRecordingSamplePercent)
		if err != nil {
			return tenantShellSettings{}, fmt.Errorf("resolve %s: %w", settings.KeyAnalyticsRecordingSamplePercent, err)
		}
		resolved.analyticsSamplePercent = value
	}
	{
		value, err := readInt(settings.KeyEarlyCheckoutNoteToleranceMinutes)
		if err != nil {
			return tenantShellSettings{}, fmt.Errorf("resolve %s: %w", settings.KeyEarlyCheckoutNoteToleranceMinutes, err)
		}
		resolved.earlyCheckoutTolerance = value
	}
	return resolved, nil
}

// resolveFeatureFlags loads shell capabilities without substituting toggle values.
func (resolved *tenantShellSettings) resolveFeatureFlags(readBool func(string) (bool, error)) error {
	boolFields := []struct {
		key    string
		target *bool
	}{
		{settings.KeyStudentPhotosEnabled, &resolved.studentPhotosEnabled},
		{settings.KeyAttendanceNFCEnabled, &resolved.nfcEnabled},
		{settings.KeyDisplayEnabled, &resolved.displayEnabled},
		{settings.KeyCalendarCalDAVEnabled, &resolved.calDAVEnabled},
		{settings.KeyEnrollmentCareOfferingsEnabled, &resolved.careOfferingsEnabled},
		{settings.KeyAttendanceWebEnabled, &resolved.attendanceWebEnabled},
		{settings.KeyAttendanceLogEnabled, &resolved.attendanceLogEnabled},
		{settings.KeyTimetableShowExpectedChildrenCount, &resolved.showTimetableCounts},
		{settings.KeyTimetableEnabled, &resolved.timetableEnabled},
		{settings.KeyMealPlanEnabled, &resolved.mealPlanEnabled},
		{settings.KeyMealRegistrationEnabled, &resolved.mealRegistrationEnabled},
		{settings.KeyEnrollmentWaitlistEnabled, &resolved.waitlistEnabled},
		{settings.KeyWeekendFollowsFriday, &resolved.weekendFollowsFriday},
		{settings.KeyChildSearchOnlyNavigation, &resolved.childSearchOnlyNavigation},
		{settings.KeyParentNotesEnabled, &resolved.parentMessagingEnabled},
		{settings.KeyStaffMessagingEnabled, &resolved.staffMessagingEnabled},
		{settings.KeyAnalyticsEnabled, &resolved.analyticsEnabled},
		{settings.KeyAnalyticsFreigabe, &resolved.analyticsFreigabe},
		{settings.KeyEarlyCheckoutNoteEnabled, &resolved.earlyCheckoutNote},
	}
	for _, field := range boolFields {
		value, err := readBool(field.key)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", field.key, err)
		}
		*field.target = value
	}
	return nil
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

// normalizeAttendanceEditScope keeps an unknown wire value from widening the
// client: anything unrecognised is "own".
func normalizeAttendanceEditScope(value string) string {
	if value == settings.AttendanceEditScopeAllStaff {
		return value
	}
	return settings.AttendanceEditScopeOwn
}

// switchTenant handles POST /auth/switch-tenant
func (rs *Resource) switchTenant(w http.ResponseWriter, r *http.Request) {
	req := &SwitchTenantRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Get account ID from JWT claims
	claims := sessiontoken.ClaimsFromCtx(r.Context())

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
				// The message text stays for older clients; the switcher reads
				// the code (#2517).
				common.RenderError(w, r, common.ErrorUnauthorizedWithCode(identityaccess.ErrTenantAccessDenied, common.CodeIdentityTenantAccessDenied))
			case errors.Is(err, identityaccess.ErrMustUseSchoolPortal):
				common.RenderError(w, r, common.ErrorForbiddenWithCode(
					identityaccess.ErrMustUseSchoolPortal, common.CodeIdentityUseSchoolPortal))
			case errors.Is(err, identityaccess.ErrDemoSessionTenantLocked):
				common.RenderError(w, r, common.ErrorForbiddenWithCode(
					identityaccess.ErrDemoSessionTenantLocked, common.CodeIdentityDemoSession))
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
	claims := sessiontoken.ClaimsFromCtx(r.Context())

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
