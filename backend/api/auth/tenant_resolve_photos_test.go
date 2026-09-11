// Package auth_test exercises the photo-feature enrichment of the public
// GET /auth/tenant/resolve endpoint. The endpoint is unauthenticated and
// every page in the frontend tenant shell hits it once on load. The
// `student_photos_enabled` flag must round-trip the per-tenant
// `operations.student_photos_enabled` setting so non-admin Betreuer
// (who don't carry config:read and therefore can't fetch /api/settings/schema)
// still get the boolean they need to decide whether to render avatars.
//
// Why this is in its own file:
//   - the existing TestResolveTenant_DeletedSchool_ReturnsNotFound covers
//     the soft-delete branch but not the photo-feature wiring
//   - this file is intentionally narrow so a future change to the photo
//     enrichment path doesn't have to touch the broader auth_test.go
package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// resolveResp is the shape we care about — TenantResolveResponse has more
// fields, but the test only asserts on the photo flag.
type resolveResp struct {
	Status string `json:"status"`
	Data   struct {
		StudentPhotosEnabled bool   `json:"student_photos_enabled"`
		Slug                 string `json:"slug"`
		GradeLevelMax        int    `json:"grade_level_max"`
	} `json:"data"`
}

func newTenantResolveScope(t *testing.T, db *bun.DB) (testpkg.TenantScope, string) {
	t.Helper()
	scope := testpkg.NewTenantScope(t, db)
	t.Cleanup(func() {
		ctx := context.Background()
		_, err := db.ExecContext(ctx, `DELETE FROM config.setting_audit WHERE tenant_id = ?`, scope.TenantID)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `DELETE FROM config.setting_values WHERE tenant_id = ?`, scope.TenantID)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, scope.TenantID)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, scope.TenantID)
		require.NoError(t, err)
	})
	return scope, "t" + strconv.FormatInt(scope.TenantID, 10)
}

// TestResolveTenant_StudentPhotosEnabled_DefaultFalse verifies that a
// tenant that has never set `operations.student_photos_enabled` returns
// false. The registry default is false, and the resolver must not auto-
// enable the feature for a school that has not opted in.
func TestResolveTenant_StudentPhotosEnabled_DefaultFalse(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	_, slug := newTenantResolveScope(t, db)

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(authRoute.AuthService, authRoute.InvitationService, platformSvc.NewSchoolService(schoolRepo), db)
	resource.SettingsService = authRoute.SettingsService

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp resolveResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.False(t, resp.Data.StudentPhotosEnabled,
		"registry default for student_photos_enabled is false; resolver must surface that")
	assert.Equal(t, 4, resp.Data.GradeLevelMax,
		"tenant resolve must surface the enrollment grade cap without config:read")
}

// TestResolveTenant_StudentPhotosEnabled_OverrideTrue verifies that a
// tenant override of true round-trips through the resolver. This is the
// critical wiring that keeps the avatar UI in sync after an admin flips
// the toggle in /settings.
func TestResolveTenant_StudentPhotosEnabled_OverrideTrue(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	scope, slug := newTenantResolveScope(t, db)

	ctx := scope.Context()
	require.NoError(t,
		authRoute.SettingsService.SetValue(ctx, configModel.KeyStudentPhotosEnabled, true, nil, nil),
		"enable student_photos_enabled for the isolated tenant")
	t.Cleanup(func() {
		require.NoError(t, authRoute.SettingsService.ResetValue(ctx, configModel.KeyStudentPhotosEnabled, nil, nil))
	})

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(authRoute.AuthService, authRoute.InvitationService, platformSvc.NewSchoolService(schoolRepo), db)
	resource.SettingsService = authRoute.SettingsService

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp resolveResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.True(t, resp.Data.StudentPhotosEnabled,
		"override of true must round-trip through resolveTenant")
	assert.NotEmpty(t, resp.Data.Slug,
		"resolver must return the canonical slug")
}

func TestResolveTenant_GradeLevelMax_Override(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	scope, slug := newTenantResolveScope(t, db)

	ctx := scope.Context()
	require.NoError(t,
		authRoute.SettingsService.SetValue(ctx, configModel.KeyEnrollmentGradeLevelMax, 13, nil, nil))
	t.Cleanup(func() {
		require.NoError(t, authRoute.SettingsService.ResetValue(ctx, configModel.KeyEnrollmentGradeLevelMax, nil, nil))
	})

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(authRoute.AuthService, authRoute.InvitationService, platformSvc.NewSchoolService(schoolRepo), db)
	resource.SettingsService = authRoute.SettingsService
	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp resolveResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 13, resp.Data.GradeLevelMax)
}

// Grade-level metadata is a validation constraint used by both enrollment
// forms and timetable targets. Without the settings service, tenant resolve
// must fail rather than inventing grade 4 while server validation may enforce
// a different cap. The unrelated optional feature flags retain their own
// fail-open/fail-closed behavior whenever settings resolution is available.
func TestResolveTenant_NilSettingsServiceFailsGradeMetadata(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	_, slug := newTenantResolveScope(t, db)

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(authRoute.AuthService, authRoute.InvitationService, platformSvc.NewSchoolService(schoolRepo), db)
	// Deliberately do NOT set SettingsService — exercises the nil branch.

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), http.StatusText(http.StatusInternalServerError))
	assert.NotContains(t, rr.Body.String(), "settings service not configured")
}

func TestResolveTenant_GradeLevelSettingsFailureIsGeneric500(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	_, slug := newTenantResolveScope(t, db)

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(authRoute.AuthService, authRoute.InvitationService, platformSvc.NewSchoolService(schoolRepo), db)
	resource.SettingsService = &configtest.Mock{
		ResolveIntForTenantFn: func(context.Context, int64, string) (int, error) {
			return 0, errors.New("private database failure detail")
		},
	}
	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), http.StatusText(http.StatusInternalServerError))
	assert.NotContains(t, rr.Body.String(), "private database failure detail")
}

func TestResolveTenant_StaffMessagingSettingsFailureIsGeneric500(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)
	_, slug := newTenantResolveScope(t, db)

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(authRoute.AuthService, authRoute.InvitationService, platformSvc.NewSchoolService(schoolRepo), db)
	resource.SettingsService = &configtest.Mock{
		ResolveBoolForTenantFn: func(_ context.Context, _ int64, key string) (bool, error) {
			if key == configModel.KeyStaffMessagingEnabled {
				return false, errors.New("private staff messaging settings failure")
			}
			return false, nil
		},
		ResolveIntForTenantFn: func(context.Context, int64, string) (int, error) {
			return 4, nil
		},
	}
	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), http.StatusText(http.StatusInternalServerError))
	assert.NotContains(t, rr.Body.String(), "private staff messaging settings failure")
}

func TestResolveTenant_OutOfRangeGradeLevelIsGeneric500(t *testing.T) {
	t.Parallel()

	for _, value := range []int{0, 14} {
		t.Run("value_"+strconv.Itoa(value), func(t *testing.T) {
			db, authRoute := setupAuthDependenciesRoute(t)
			_, slug := newTenantResolveScope(t, db)

			schoolRepo := platformRepo.NewSchoolRepository(db)
			resource := authAPI.NewResource(authRoute.AuthService, authRoute.InvitationService, platformSvc.NewSchoolService(schoolRepo), db)
			resource.SettingsService = &configtest.Mock{
				ResolveIntForTenantFn: func(context.Context, int64, string) (int, error) {
					return value, nil
				},
			}
			router := chi.NewRouter()
			router.Mount("/auth", resource.Router())

			req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			require.Equal(t, http.StatusInternalServerError, rr.Code, "Body: %s", rr.Body.String())
			assert.Contains(t, rr.Body.String(), http.StatusText(http.StatusInternalServerError))
			assert.NotContains(t, rr.Body.String(), "outside")
		})
	}
}

// TestResolveTenant_MissingSlug_400 covers the early-return path the
// existing test suite doesn't exercise. The resolver requires the slug
// query parameter and returns 400 without it.
func TestResolveTenant_MissingSlug_400(t *testing.T) {
	t.Parallel()

	db, authRoute := setupAuthDependenciesRoute(t)

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(authRoute.AuthService, authRoute.InvitationService, platformSvc.NewSchoolService(schoolRepo), db)

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
}
