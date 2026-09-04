package api

import (
	"context"
	"encoding/json"
	"fmt"
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
