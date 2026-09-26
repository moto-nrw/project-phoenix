package api

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigureProfileStep_UsesDeclaredSettingManager(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	requests := make(map[string]string)
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		mu.Lock()
		requests[r.URL.Path] = r.Header.Get("Authorization")
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":"success"}`)
	})
	defer srv.Close()

	definition := demoProfileDefinition{Key: "test", Settings: map[string]SeedSetting{
		"operator.key": {Value: json.RawMessage(`true`), ManagedBy: SettingManagedByOperator},
		"tenant.key":   {Value: json.RawMessage(`"fixed"`), ManagedBy: SettingManagedByTenant},
	}}
	rt := &Runtime{
		Client: newTestClient(srv.URL, false), OperatorAuth: AuthRef{Kind: AuthBearer, Token: "operator-token"},
		TenantAuth: AuthRef{Kind: AuthBearer, Token: "tenant-token"}, Bootstrap: &bootstrapSeedState{SchoolID: 42},
	}

	err := (configureProfileStep{definition: definition}).Run(context.Background(), rt)
	require.NoError(t, err)
	assert.Equal(t, "Bearer operator-token", requests["/operator/schools/42/settings/values/operator.key"])
	assert.Equal(t, "Bearer tenant-token", requests["/api/settings/values/tenant.key"])
}

func TestConfigureDevicePINStep_WritesStaffPINAsSchoolAdmin(t *testing.T) {
	t.Parallel()

	var method, path, authorization string
	var body map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		method, path, authorization = r.Method, r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":"success"}`)
	})
	defer srv.Close()

	rt := &Runtime{
		Client: newTestClient(srv.URL, false), StaffPIN: "4711",
		OperatorAuth: AuthRef{Kind: AuthBearer, Token: "operator-token"},
		TenantAuth:   AuthRef{Kind: AuthBearer, Token: "tenant-token"},
	}
	require.NoError(t, (configureDevicePINStep{}).Run(context.Background(), rt))
	assert.Equal(t, seedHTTPMethodPut, method)
	// security.ogs_device_pin is admin-only: the operator path answers 403.
	assert.Equal(t, "/api/settings/values/security.ogs_device_pin", path)
	assert.Equal(t, "Bearer tenant-token", authorization)
	assert.Equal(t, map[string]any{"value": "4711"}, body)
}

func TestFullDemoWorkflowSetsDevicePINBeforeDeviceAuth(t *testing.T) {
	t.Parallel()

	steps := fullDemoWorkflow(&Seeder{options: SeedOptions{OnlyProfile: DefaultProfileKey}}).Steps
	pin := slices.IndexFunc(steps, func(step Step) bool { _, ok := step.(configureDevicePINStep); return ok })
	bootstrap := slices.IndexFunc(steps, func(step Step) bool { _, ok := step.(bootstrapTenantStep); return ok })
	firstDeviceStep := slices.IndexFunc(steps, func(step Step) bool { _, ok := step.(seedStatisticsDemoStep); return ok })
	require.NotEqual(t, -1, pin, "the workflow must write the device PIN")
	assert.Greater(t, pin, bootstrap, "the school admin writes the PIN")
	assert.Less(t, pin, firstDeviceStep, "the PIN must be set before the first device request")
}

func TestVerifyProfileSettings_RejectsReadBackMismatch(t *testing.T) {
	t.Parallel()

	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, _ *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":{"tabs":[{"categories":[{"items":[{"key":"test.key","value":false}]}]}]}}`)
	})
	defer srv.Close()

	rt := &Runtime{
		Client: newTestClient(srv.URL, false), OperatorAuth: AuthRef{Kind: AuthBearer, Token: "operator"},
		TenantAuth: AuthRef{Kind: AuthBearer, Token: "tenant"}, Bootstrap: &bootstrapSeedState{SchoolID: 42},
	}
	err := verifyProfileSettings(rt, demoProfileDefinition{Settings: map[string]SeedSetting{
		"test.key": {Value: json.RawMessage(`true`), ManagedBy: SettingManagedByTenant},
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected true, got false")
}

func TestConfigureDemoProfilesOrdersAttendanceScopeChanges(t *testing.T) {
	t.Parallel()
	for _, definition := range []demoProfileDefinition{fullOperationProfileDefinition(), manualProfileDefinition()} {
		t.Run(definition.Key, func(t *testing.T) {
			var keys []string
			srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
				keys = append(keys, r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:])
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{"status":"success"}`)
			})
			defer srv.Close()
			rt := &Runtime{
				Client: newTestClient(srv.URL, false), Bootstrap: &bootstrapSeedState{SchoolID: 42},
				TenantAuth: AuthRef{Kind: AuthBearer, Token: "tenant"}, OperatorAuth: AuthRef{Kind: AuthBearer, Token: "operator"},
			}
			require.NoError(t, (configureProfileStep{definition: definition}).Run(context.Background(), rt))
			visibility := slices.Index(keys, profileSettingOverviewScope)
			attendance := slices.Index(keys, profileSettingAttendanceScope)
			starting := slices.Index(keys, profileSettingBlockStartScope)
			ending := slices.Index(keys, profileSettingBlockCompleteScope)
			require.NotEqual(t, -1, visibility)
			require.NotEqual(t, -1, attendance)
			require.NotEqual(t, -1, starting)
			require.NotEqual(t, -1, ending)
			if definition.Key == DefaultProfileKey {
				assert.Less(t, visibility, attendance)
				assert.Less(t, visibility, starting)
				assert.Less(t, visibility, ending)
				atSchool, ok := definition.Settings[profileSettingSessionEndTime]
				require.True(t, ok)
				assert.Equal(t, json.RawMessage(`"23:59"`), atSchool.Value)
				assert.Equal(t, SettingManagedByOperator, atSchool.ManagedBy)
			} else {
				assert.Less(t, attendance, visibility)
				assert.Less(t, starting, visibility)
				assert.Less(t, ending, visibility)
			}
			require.Contains(t, keys, profileSettingParentSickMode)
			require.Contains(t, keys, profileSettingParentExcusedMode)
			require.NotEqual(t, definition.Settings[profileSettingParentSickMode].Value, definition.Settings[profileSettingParentExcusedMode].Value)
		})
	}
}
