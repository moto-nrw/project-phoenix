package config_test

import (
	"context"
	"errors"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	configAPI "github.com/moto-nrw/project-phoenix/api/config"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// settingsTestContext holds dependencies for settings API integration tests.
type settingsTestContext struct {
	db       *testpkg.DB
	resource *configAPI.SettingsResource
}

func setupSettingsModule(t *testing.T) *settingsTestContext {
	t.Helper()

	db, svc := testutil.SetupSettingsModule(t)
	runtime := configAPI.NewRuntime(configAPI.RuntimeDependencies{
		Protected:   testutil.ProtectedTestTenantGroupFunc(db),
		Permission:  func(configAPI.Access) configAPI.Middleware { return testutil.IdentityMiddleware },
		TenantGuard: testutil.IdentityMiddleware,
		RequestActor: func(context.Context) (int64, int64, []string) {
			return testpkg.Tenant(t), 1, []string{"config:read", "config:update", "config:manage"}
		},
		Editable:  func(context.Context) bool { return true },
		Success:   testutil.RespondSuccess,
		NoContent: testutil.RespondNoContent,
		Failure:   testutil.RespondError,
	})
	homeLayouts, ok := svc.Settings.(configAPI.HomeLayoutOperations)
	require.True(t, ok)
	resource := configAPI.NewSettingsResource(svc.TenantSettings, homeLayouts, runtime)

	return &settingsTestContext{
		db:       db,
		resource: resource,
	}
}

// =============================================================================
// GET /schema
// =============================================================================

func TestSettingsGetSchema_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/schema", nil,
		testutil.WithTestTenant(t),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, 200)

	// Verify response contains tabs
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	if ok {
		_, hasTabs := data["tabs"]
		assert.True(t, hasTabs, "schema should contain tabs")
	}
}

func TestSettingsSetValue_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	router := ctx.resource.SettingsRouter()

	body := map[string]interface{}{
		"value": "18:30",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.student_daily_checkout_time", body,
		testutil.WithTestTenant(t),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, 200)
}

func TestSettingsSetValue_InvalidKey(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	router := ctx.resource.SettingsRouter()

	body := map[string]interface{}{
		"value": "test",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/nonexistent.key", body,
		testutil.WithTestTenant(t),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, 404)
}

func TestSettingsSetValue_InvalidValue(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	router := ctx.resource.SettingsRouter()

	body := map[string]interface{}{
		"value": "not-a-boolean",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.raumwechsel_enabled", body,
		testutil.WithTestTenant(t),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, 400)
}

func TestSettingsSetValue_WithConfigManage(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	router := ctx.resource.SettingsRouter()

	body := map[string]interface{}{
		"value": "17:00",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/gdpr.data_cleanup_time", body,
		testutil.WithTestTenant(t),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, 200)
}

// =============================================================================
// DELETE /values/{key}
// =============================================================================

func TestSettingsResetValue_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/values/operations.student_daily_checkout_time", nil,
		testutil.WithTestTenant(t),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, 204)
}

func TestSettingsResetValue_InvalidKey(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/values/nonexistent.key", nil,
		testutil.WithTestTenant(t),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, 404)
}

// =============================================================================
// GET /login-image
// =============================================================================

func TestSettingsGetLoginImage_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/login-image", nil,
		testutil.WithTestTenant(t),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, 200)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	assert.True(t, ok, "response should contain data")
	assert.Nil(t, data["login_image_url"], "default school should have no login image")
	assert.True(t, data["can_edit"].(bool), "admin should have edit permission")
}

func TestSettingsSetValue_OnValueSetCallbackInvoked(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	var callbackKey string
	var callbackValue any
	var callbackTenantID int64
	ctx.resource.OnValueSet(func(_ context.Context, tenantID int64, key string, value any) (func(), error) {
		callbackTenantID = tenantID
		callbackKey = key
		callbackValue = value
		return nil, nil
	})

	router := ctx.resource.SettingsRouter()

	body := map[string]interface{}{
		"value": true,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.schulhof_enabled", body,
		testutil.WithTestTenant(t),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, 200)

	assert.Equal(t, "checkout.schulhof_enabled", callbackKey)
	assert.Equal(t, true, callbackValue)
	assert.Greater(t, callbackTenantID, int64(0), "callback should receive a valid tenant_id")
}

func TestSettingsSetValue_OnValueSetCallbackErrorRollsBack(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	ctx.resource.OnValueSet(func(_ context.Context, _ int64, _ string, _ any) (func(), error) {
		return nil, errors.New("hook failed")
	})

	router := ctx.resource.SettingsRouter()

	body := map[string]interface{}{
		"value": "17:45",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.student_daily_checkout_time", body,
		testutil.WithTestTenant(t),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, 500)

	count, err := ctx.db.NewSelect().
		TableExpr("config.setting_values").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("setting_key = ?", "operations.student_daily_checkout_time").
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count, "failed callback should roll back the setting update")
}

func TestSettingsSetValue_OnValueSetNotCalledOnError(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	callbackInvoked := false
	ctx.resource.OnValueSet(func(_ context.Context, _ int64, _ string, _ any) (func(), error) {
		callbackInvoked = true
		return nil, nil
	})

	router := ctx.resource.SettingsRouter()

	body := map[string]interface{}{
		"value": "not-a-boolean",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.schulhof_enabled", body,
		testutil.WithTestTenant(t),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, 400)

	assert.False(t, callbackInvoked, "callback should not be invoked on validation error")
}

// TestSettingsSetValue_PostCommitRunsOnSuccess locks in the contract that
// the post-commit closure returned by the OnValueSet callback runs after a
// successful tx. Photo-purge file unlinks rely on this — running them in
// the tx would mean a commit failure leaves files deleted but the photo_path
// rows untouched.
func TestSettingsSetValue_PostCommitRunsOnSuccess(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	postCommitRan := false
	ctx.resource.OnValueSet(func(_ context.Context, _ int64, _ string, _ any) (func(), error) {
		return func() {
			postCommitRan = true
		}, nil
	})

	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.schulhof_enabled",
		map[string]interface{}{"value": true},
		testutil.WithTestTenant(t),
	)
	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, 200)

	assert.True(t, postCommitRan, "post-commit closure must run after a successful PUT")
}

// TestSettingsSetValue_PostCommitSkippedOnHookError ensures the post-commit
// closure does NOT run when the in-tx hook returns an error. If it did, a
// photo-purge file unlink would still happen for a tx that rolled back —
// the exact bug the two-phase contract was introduced to prevent.
func TestSettingsSetValue_PostCommitSkippedOnHookError(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	postCommitRan := false
	ctx.resource.OnValueSet(func(_ context.Context, _ int64, _ string, _ any) (func(), error) {
		// Return BOTH a closure and an error — the handler must not run
		// the closure because the tx rolled back. Returning the closure
		// alongside the error proves the handler honours the contract by
		// gating on err, not on cb-presence.
		return func() {
			postCommitRan = true
		}, errors.New("hook failed")
	})

	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.schulhof_enabled",
		map[string]interface{}{"value": true},
		testutil.WithTestTenant(t),
	)
	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, 500)

	assert.False(t, postCommitRan, "post-commit closure must not run when the tx rolled back")
}

func TestSettingsSetValue_NilCallbackDoesNotPanic(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	// No OnValueSet registered — should not panic
	router := ctx.resource.SettingsRouter()

	body := map[string]interface{}{
		"value": true,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.wc_enabled", body,
		testutil.WithTestTenant(t),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, 200)
}

// TestSettingsResetValue_OnValueSetCallbackInvoked guarantees the side-effect
// hook fires on reset (DELETE /values/{key}) for student_photos_enabled.
// Reset puts the setting back on its registry default, so the callback
// receives that default — otherwise resetting student_photos_enabled would
// leave already-stored photos on disk because the purge callback never runs.
func TestSettingsResetValue_OnValueSetCallbackInvoked(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	router := ctx.resource.SettingsRouter()

	seed := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.student_photos_enabled", map[string]interface{}{
		"value": true,
	}, testutil.WithTestTenant(t))
	rr := testutil.ExecuteRequest(router, seed)
	testutil.AssertSuccessResponse(t, rr, 200)

	// Now register the callback and reset.
	var callbackKey string
	var callbackValue any
	var callbackTenantID int64
	ctx.resource.OnValueSet(func(_ context.Context, tenantID int64, key string, value any) (func(), error) {
		callbackTenantID = tenantID
		callbackKey = key
		callbackValue = value
		return nil, nil
	})

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/values/operations.student_photos_enabled", nil,
		testutil.WithTestTenant(t),
	)
	rr = testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, 204)

	assert.Equal(t, "operations.student_photos_enabled", callbackKey)
	assert.Equal(t, false, callbackValue)
	assert.Greater(t, callbackTenantID, int64(0), "callback should receive a valid tenant_id")
}

func TestSettingsResetValue_NonPhotoKeyDoesNotInvokeOnValueSet(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	router := ctx.resource.SettingsRouter()

	seed := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.schulhof_enabled", map[string]interface{}{
		"value": true,
	}, testutil.WithTestTenant(t))
	rr := testutil.ExecuteRequest(router, seed)
	testutil.AssertSuccessResponse(t, rr, 200)

	var called bool
	ctx.resource.OnValueSet(func(_ context.Context, _ int64, _ string, _ any) (func(), error) {
		called = true
		return nil, nil
	})

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/values/checkout.schulhof_enabled", nil,
		testutil.WithTestTenant(t),
	)
	rr = testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, 204)

	assert.False(t, called, "non-photo reset must not fire OnValueSet")
}

// TestSettingsResetValue_OnValueSetCallbackErrorRollsBack ensures the reset
// path participates in the same tx contract as PUT for the photo flag — a
// hook error must roll back the override deletion.
func TestSettingsResetValue_OnValueSetCallbackErrorRollsBack(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	router := ctx.resource.SettingsRouter()

	seed := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.student_photos_enabled", map[string]interface{}{
		"value": true,
	}, testutil.WithTestTenant(t))
	rr := testutil.ExecuteRequest(router, seed)
	testutil.AssertSuccessResponse(t, rr, 200)

	ctx.resource.OnValueSet(func(_ context.Context, _ int64, _ string, _ any) (func(), error) {
		return nil, errors.New("hook failed")
	})

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/values/operations.student_photos_enabled", nil,
		testutil.WithTestTenant(t),
	)
	rr = testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, 500)

	// Override should still exist — the failed hook rolled back the delete.
	count, err := ctx.db.NewSelect().
		TableExpr("config.setting_values").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("setting_key = ?", "operations.student_photos_enabled").
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count, "failed hook on reset must roll back the override deletion")
}

func TestSettingsSetValue_OperatorOnlyForbidden(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	router := ctx.resource.SettingsRouter()

	// operations.session_end_time is AccessOperatorOnly — tenant admins must not
	// write it even if they hold config:update. The UI hides these keys; this
	// test guards the direct-API path.
	body := map[string]interface{}{"value": "19:00"}
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.session_end_time", body,
		testutil.WithTestTenant(t),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, 403)
}

func TestSettingsResetValue_OperatorOnlyForbidden(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/values/operations.session_end_time", nil,
		testutil.WithTestTenant(t),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertErrorResponse(t, rr, 403)
}

func TestSettingsGetSchema_HidesOperatorOnly(t *testing.T) {
	t.Parallel()

	ctx := setupSettingsModule(t)

	router := ctx.resource.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/schema", nil,
		testutil.WithTestTenant(t),
	)
	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, 200)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, _ := response["data"].(map[string]interface{})
	tabs, _ := data["tabs"].([]interface{})

	// Walk every item and confirm no AccessOperatorOnly keys appear.
	operatorOnlyKeys := map[string]bool{
		"operations.session_end_enabled":                 true,
		"operations.session_end_time":                    true,
		"operations.session_end_timeout_minutes":         true,
		"operations.session_cleanup_enabled":             true,
		"operations.session_cleanup_interval_minutes":    true,
		"operations.session_abandoned_threshold_minutes": true,
	}
	for _, tabRaw := range tabs {
		tab := tabRaw.(map[string]interface{})
		categories, _ := tab["categories"].([]interface{})
		for _, catRaw := range categories {
			cat := catRaw.(map[string]interface{})
			items, _ := cat["items"].([]interface{})
			for _, itemRaw := range items {
				item := itemRaw.(map[string]interface{})
				key := item["key"].(string)
				assert.False(t, operatorOnlyKeys[key],
					"admin schema must not include AccessOperatorOnly key %s", key)
			}
		}
	}
}
