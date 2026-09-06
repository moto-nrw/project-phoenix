// Package operator_test covers operator settings paths that the
// existing settings_integration_test.go does not exercise.
//
// Two paths in particular are <100% in coverage and matter for the
// student-photo-avatar feature:
//
//  1. resolveSchoolSlug — runs when schoolRepo is non-nil, attaching
//     `school_slug` to the mutation response so the operator proxy
//     can bust the slug-keyed `tenant-${slug}` Next.js cache after
//     flipping a tenant-resolve-affecting setting (e.g.
//     operations.student_photos_enabled). With a nil schoolRepo
//     (the existing test setup) this branch is skipped and the
//     response carries an empty slug.
//
//  2. The OnValueSet hook on a flag-flip path — proves that operator
//     writes to operations.student_photos_enabled invoke the hook the
//     same way tenant writes do, which is the critical wiring that
//     makes the photo purge fire on disable and the cache bust fire
//     on enable.
package operator_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-chi/chi/v5"

	operatorAPI "github.com/moto-nrw/project-phoenix/api/operator"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

// setupOperatorSettingsWithSchoolRepoRoute mirrors
// setupOperatorSettingsRoute but injects a real SchoolRepository so the
// slug-resolution branch in SetSchoolSettingValue / ResetSchoolSettingValue
// is exercised end to end. Without a real repo the response carries
// `school_slug: ""` (empty omitted) and resolveSchoolSlug returns
// before its real query path runs.
func setupOperatorSettingsWithSchoolRepoRoute(t *testing.T) *operatorSettingsTestContext {
	t.Helper()

	db, svc := testutil.SetupOperatorSettingsModule(t)
	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := operatorAPI.NewSettingsResource(
		svc.Settings, db, nil, platformSvc.NewSchoolService(schoolRepo), svc.Active, svc.CareLifecycle,
	)

	router := chi.NewRouter()
	router.Get("/schools/{id}/settings/schema", resource.GetSchoolSettingsSchema)
	router.Get("/schools/{id}/settings/values/{key}/reveal", resource.RevealSchoolSettingValue)
	router.Put("/schools/{id}/settings/values/{key}", resource.SetSchoolSettingValue)
	router.Delete("/schools/{id}/settings/values/{key}", resource.ResetSchoolSettingValue)

	return &operatorSettingsTestContext{
		db:       db,
		resource: resource,
		router:   router,
	}
}

// TestOperatorSetSchoolSettingValue_StudentPhotosEnabled_ReturnsSlug
// verifies the slug-resolution path runs when an operator flips
// operations.student_photos_enabled. The frontend proxy depends on
// `school_slug` being present in the response to bust the
// `tenant-${slug}` Next.js cache; without that bust the layout
// keeps serving the stale resolve payload for up to 5 minutes and
// the avatar UI flickers between "feature on" and "feature off".
func TestOperatorSetSchoolSettingValue_StudentPhotosEnabled_ReturnsSlug(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsWithSchoolRepoRoute(t)

	// Reset prior override so the test starts clean.
	t.Cleanup(func() {
		_, _ = ctx.db.ExecContext(context.Background(),
			`DELETE FROM config.setting_values WHERE tenant_id = ? AND setting_key = ?`,
			testpkg.Tenant(t), configModel.KeyStudentPhotosEnabled)
	})

	body := map[string]interface{}{"value": true}
	req := newOperatorRequest(t, http.MethodPut,
		fmt.Sprintf("/schools/%d/settings/values/", testpkg.Tenant(t))+configModel.KeyStudentPhotosEnabled, body)
	rr := testutil.ExecuteRequest(ctx.router, req)

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	// Parse and assert school_slug is present and non-empty — that's the
	// signal the proxy uses to bust the cache.
	var resp struct {
		Status string `json:"status"`
		Data   struct {
			SchoolSlug string `json:"school_slug"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Data.SchoolSlug,
		"PUT response must include school_slug for cache busting; got empty slug")
}

// TestOperatorResetSchoolSettingValue_StudentPhotosEnabled_ReturnsSlug
// mirrors the PUT test for the DELETE/reset path. Reset is a real
// state change for callers that previously overrode the value, so the
// same cache bust contract applies.
func TestOperatorResetSchoolSettingValue_StudentPhotosEnabled_ReturnsSlug(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsWithSchoolRepoRoute(t)

	// Set an override first so reset has something to delete. Without
	// this, resetting an unset key still succeeds but the OnValueSet
	// hook fires with the registry default — the slug branch we want
	// to exercise runs either way, so the prior write is just a
	// realism touch.
	setReq := newOperatorRequest(t, http.MethodPut,
		fmt.Sprintf("/schools/%d/settings/values/", testpkg.Tenant(t))+configModel.KeyStudentPhotosEnabled,
		map[string]interface{}{"value": true})
	require.Equal(t, http.StatusOK, testutil.ExecuteRequest(ctx.router, setReq).Code)

	t.Cleanup(func() {
		_, _ = ctx.db.ExecContext(context.Background(),
			`DELETE FROM config.setting_values WHERE tenant_id = ? AND setting_key = ?`,
			testpkg.Tenant(t), configModel.KeyStudentPhotosEnabled)
	})

	resetReq := newOperatorRequest(t, http.MethodDelete,
		fmt.Sprintf("/schools/%d/settings/values/", testpkg.Tenant(t))+configModel.KeyStudentPhotosEnabled, nil)
	rr := testutil.ExecuteRequest(ctx.router, resetReq)

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Status string `json:"status"`
		Data   struct {
			SchoolSlug string `json:"school_slug"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Data.SchoolSlug,
		"DELETE response must include school_slug for cache busting; got empty slug")
}

// TestOperatorSetSchoolSettingValue_StudentPhotosEnabled_HookFires
// proves that flipping operations.student_photos_enabled at the
// operator level fires the OnValueSet hook with the expected
// (tenantID, key, value) triple. This is the wiring api/base.go
// relies on to schedule the photo-purge / cache-bust closures —
// without this assertion a future refactor that silently drops
// the hook on photo-flag flips could ship green.
func TestOperatorSetSchoolSettingValue_StudentPhotosEnabled_HookFires(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsWithSchoolRepoRoute(t)

	t.Cleanup(func() {
		_, _ = ctx.db.ExecContext(context.Background(),
			`DELETE FROM config.setting_values WHERE tenant_id = ? AND setting_key = ?`,
			testpkg.Tenant(t), configModel.KeyStudentPhotosEnabled)
	})

	var hookCalled bool
	var capturedKey string
	var capturedValue any
	var capturedTenantID int64
	var postCommitFired bool
	ctx.resource.OnValueSet(func(_ context.Context, tenantID int64, key string, value any) (func(), error) {
		hookCalled = true
		capturedTenantID = tenantID
		capturedKey = key
		capturedValue = value
		return func() { postCommitFired = true }, nil
	})

	body := map[string]interface{}{"value": true}
	req := newOperatorRequest(t, http.MethodPut,
		fmt.Sprintf("/schools/%d/settings/values/", testpkg.Tenant(t))+configModel.KeyStudentPhotosEnabled, body)
	rr := testutil.ExecuteRequest(ctx.router, req)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	require.True(t, hookCalled, "OnValueSet must fire on photo-flag flip")
	assert.Equal(t, testpkg.Tenant(t), capturedTenantID)
	assert.Equal(t, configModel.KeyStudentPhotosEnabled, capturedKey)
	assert.Equal(t, true, capturedValue)
	assert.True(t, postCommitFired,
		"post-commit closure must fire after the operator's tenant-tx commits")
}

// TestOperatorResetSchoolSettingValue_StudentPhotosEnabled_HookFiresWithDefault
// covers the reset variant: even though the operator is clearing an
// override, the OnValueSet hook must fire with the registry default
// so downstream side effects (photo purge on disable) run when the
// effective value transitions to false.
func TestOperatorResetSchoolSettingValue_StudentPhotosEnabled_HookFiresWithDefault(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsWithSchoolRepoRoute(t)

	// Override to true first so the reset is a real state change.
	setReq := newOperatorRequest(t, http.MethodPut,
		fmt.Sprintf("/schools/%d/settings/values/", testpkg.Tenant(t))+configModel.KeyStudentPhotosEnabled,
		map[string]interface{}{"value": true})
	require.Equal(t, http.StatusOK, testutil.ExecuteRequest(ctx.router, setReq).Code)

	t.Cleanup(func() {
		_, _ = ctx.db.ExecContext(context.Background(),
			`DELETE FROM config.setting_values WHERE tenant_id = ? AND setting_key = ?`,
			testpkg.Tenant(t), configModel.KeyStudentPhotosEnabled)
	})

	var hookCalled bool
	var capturedValue any
	ctx.resource.OnValueSet(func(_ context.Context, _ int64, _ string, value any) (func(), error) {
		hookCalled = true
		capturedValue = value
		return nil, nil
	})

	resetReq := newOperatorRequest(t, http.MethodDelete,
		fmt.Sprintf("/schools/%d/settings/values/", testpkg.Tenant(t))+configModel.KeyStudentPhotosEnabled, nil)
	rr := testutil.ExecuteRequest(ctx.router, resetReq)
	require.Equal(t, http.StatusOK, rr.Code)

	require.True(t, hookCalled,
		"reset must fire OnValueSet with the registry default so downstream side effects run")
	// Default for student_photos_enabled is false; reset transitions
	// from true to default, so the hook receives false.
	assert.Equal(t, false, capturedValue,
		"reset must fire the hook with the registry default value")
}
