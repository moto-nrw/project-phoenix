package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedTestTraegerStepCreatesTheTestTraegerAndItsSampleChildren(t *testing.T) {
	t.Parallel()

	var paths, authorizations []string
	var order map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		assert.Equal(t, http.MethodPost, r.Method)
		paths = append(paths, r.URL.Path)
		authorizations = append(authorizations, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/operator/organizations/test-traeger" {
			raw, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			assert.NoError(t, json.Unmarshal(raw, &order))
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"success","data":{"organization":{"id":31,"test_system":true},"schools":[]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"created_count":72,"schools":[]}}`))
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	tenantAuth := AuthRef{Kind: AuthBearer, Token: "tenant-token"}
	client.BindAuth(tenantAuth)
	runtime := &Runtime{
		Client:       client,
		OperatorAuth: AuthRef{Kind: AuthBearer, Token: "operator-token"},
		Bootstrap:    &bootstrapSeedState{OrganizationID: 12, AdminEmail: "admin@example.test"},
	}

	require.NoError(t, (seedTestTraegerStep{}).Run(context.Background(), runtime))
	assert.Equal(t, []string{"/operator/organizations/test-traeger", "/operator/organizations/31/sample-children"}, paths)
	assert.Equal(t, []string{"Bearer operator-token", "Bearer operator-token"}, authorizations)
	assert.Equal(t, map[string]any{"name": "Test-Träger", "slug": "test-traeger", "school_count": float64(3), "contact_email": "admin@example.test"}, order)
	assert.Equal(t, tenantAuth, client.auth, "the step restores the client's own session")
}

func TestSeedTestTraegerStepLeavesAnExistingTestTraegerAlone(t *testing.T) {
	t.Parallel()

	calls := 0
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, _ *seedHTTPRequest) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"status":409,"code":"provisioning.organization_slug_taken","detail":"organization slug already exists"}`))
	})
	defer srv.Close()

	runtime := &Runtime{
		Client:       newTestClient(srv.URL, false),
		OperatorAuth: AuthRef{Kind: AuthBearer, Token: "operator-token"},
		Bootstrap:    &bootstrapSeedState{OrganizationID: 12, AdminEmail: "admin@example.test"},
	}
	require.NoError(t, (seedTestTraegerStep{}).Run(context.Background(), runtime))
	assert.Equal(t, 1, calls, "a second seed run creates no second test Träger and no sample children")
}
