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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// resolveResp is the shape we care about — TenantResolveResponse has more
// fields, but the test only asserts on the photo flag.
type resolveResp struct {
	Status string `json:"status"`
	Data   struct {
		StudentPhotosEnabled bool   `json:"student_photos_enabled"`
		Slug                 string `json:"slug"`
	} `json:"data"`
}

// TestResolveTenant_StudentPhotosEnabled_DefaultFalse verifies that a
// tenant that has never set `operations.student_photos_enabled` returns
// false. The registry default is false, and the resolver must not auto-
// enable the feature for a school that has not opted in.
func TestResolveTenant_StudentPhotosEnabled_DefaultFalse(t *testing.T) {
	db, svc := testutil.SetupAPITest(t)
	defer func() { _ = db.Close() }()

	// SetupTestDB ensures tenant 1 exists with subdomain "t1". Reset any
	// prior override so the test starts from the registry default.
	_, err := db.ExecContext(t.Context(),
		`DELETE FROM config.setting_values WHERE tenant_id = 1 AND setting_key = ?`,
		configModel.KeyStudentPhotosEnabled)
	require.NoError(t, err)

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(svc.Auth, svc.Invitation, schoolRepo, db)
	resource.SettingsService = svc.Settings

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug=default", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp resolveResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.False(t, resp.Data.StudentPhotosEnabled,
		"registry default for student_photos_enabled is false; resolver must surface that")
}

// TestResolveTenant_StudentPhotosEnabled_OverrideTrue verifies that a
// tenant override of true round-trips through the resolver. This is the
// critical wiring that keeps the avatar UI in sync after an admin flips
// the toggle in /settings.
func TestResolveTenant_StudentPhotosEnabled_OverrideTrue(t *testing.T) {
	db, svc := testutil.SetupAPITest(t)
	defer func() { _ = db.Close() }()

	// Set the override for tenant 1 and clean up after.
	ctx := testpkg.TenantContext(1)
	require.NoError(t,
		svc.Settings.SetValue(ctx, configModel.KeyStudentPhotosEnabled, true, nil, nil),
		"enable student_photos_enabled for tenant 1")
	t.Cleanup(func() {
		_ = svc.Settings.ResetValue(ctx, configModel.KeyStudentPhotosEnabled, nil, nil)
	})

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(svc.Auth, svc.Invitation, schoolRepo, db)
	resource.SettingsService = svc.Settings

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug=default", nil)
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

// TestResolveTenant_StudentPhotosEnabled_NilSettingsServiceFallsBackFalse
// verifies the defensive default path: when SettingsService is nil
// (e.g. local dev without the registry wired) the resolver returns
// false — never silently true. A settings outage MUST NOT auto-enable
// the photo UI for opt-out schools.
func TestResolveTenant_StudentPhotosEnabled_NilSettingsServiceFallsBackFalse(t *testing.T) {
	db, svc := testutil.SetupAPITest(t)
	defer func() { _ = db.Close() }()

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(svc.Auth, svc.Invitation, schoolRepo, db)
	// Deliberately do NOT set SettingsService — exercises the nil branch.

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug=default", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp resolveResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.False(t, resp.Data.StudentPhotosEnabled,
		"nil SettingsService must produce false, not silently true")
}

// TestResolveTenant_MissingSlug_400 covers the early-return path the
// existing test suite doesn't exercise. The resolver requires the slug
// query parameter and returns 400 without it.
func TestResolveTenant_MissingSlug_400(t *testing.T) {
	db, svc := testutil.SetupAPITest(t)
	defer func() { _ = db.Close() }()

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(svc.Auth, svc.Invitation, schoolRepo, db)

	router := chi.NewRouter()
	router.Mount("/auth", resource.Router())

	req := httptest.NewRequest("GET", "/auth/tenant/resolve", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
}
