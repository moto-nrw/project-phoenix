package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestSeedBillingLettersConfirmTheReleasedRun(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var calls []string
	var body map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/traeger/auth/login" {
			_, _ = w.Write([]byte(`{"status":"authenticated","access_token":"traeger-token","refresh_token":"r"}`))
			return
		}
		assert.Equal(t, "Bearer traeger-token", r.Header.Get("Authorization"))
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	})
	defer srv.Close()

	runtime := &Runtime{
		Client:    newTestClient(srv.URL, false),
		Bootstrap: &bootstrapSeedState{AdminEmail: "admin@example.test", AdminPassword: testpkg.NewTestPassword()},
		Values:    map[string]any{seedBillingRunKey: int64(9)},
	}
	require.NoError(t, (seedTraegerBillingLettersStep{}).Run(context.Background(), runtime))
	assert.Equal(t, []string{"POST /traeger/auth/login", "POST /traeger/billing-runs/9/letters"}, calls)
	assert.NotEmpty(t, body["payment_due_on"])
	assert.Contains(t, body["notes"], "Hausschuhe")
	assert.Len(t, body["emergency_contacts"], 1)

	runtime.Values = map[string]any{}
	assert.ErrorContains(t, (seedTraegerBillingLettersStep{}).Run(context.Background(), runtime), "released holiday run")
}
