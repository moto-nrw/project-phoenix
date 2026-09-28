package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSeedChildQuotaStepLeavesTheDemoSchoolAlmostFull pins the seed of #3567:
// the operator sets a Kinderkontingent a few places above the children in
// care, resending the school's editable fields the full update needs.
func TestSeedChildQuotaStepLeavesTheDemoSchoolAlmostFull(t *testing.T) {
	t.Parallel()

	var update map[string]any
	var updateAuth, countAuth string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/students":
			countAuth = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"status":"success","data":[],"pagination":{"total_records":47}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/operator/schools":
			_, _ = w.Write([]byte(`{"status":"success","data":[{"id":9,"organization_id":3,"name":"Andere","slug":"andere","subdomain":"andere","active":true},{"id":1,"organization_id":3,"name":"Vollbetrieb","slug":"vollbetrieb","subdomain":"vollbetrieb","city":"Köln","active":true}]}`))
		case r.Method == http.MethodPut && r.URL.Path == "/operator/schools/1":
			updateAuth = r.Header.Get("Authorization")
			require.NoError(t, json.NewDecoder(r.Body).Decode(&update))
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":1}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	runtime := &Runtime{
		Client:       newTestClient(srv.URL, false),
		OperatorAuth: AuthRef{Kind: AuthBearer, Token: "operator-token"},
		TenantAuth:   AuthRef{Kind: AuthBearer, Token: "tenant-token"},
		Bootstrap:    &bootstrapSeedState{SchoolID: 1},
	}

	require.NoError(t, (seedChildQuotaStep{}).Run(context.Background(), runtime))
	assert.Equal(t, "Bearer tenant-token", countAuth)
	assert.Equal(t, "Bearer operator-token", updateAuth)
	assert.Equal(t, map[string]any{"bundles": float64(1), "bundle_size": float64(50)}, update["child_quota"],
		"47 children plus three free places")
	assert.Equal(t, "vollbetrieb", update["subdomain"])
	assert.Equal(t, "Köln", update["city"], "the full update keeps the other fields")
}
