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

func TestSeedTraegerBueroStepAssignsTheSchoolAdminThroughTheOperatorRoute(t *testing.T) {
	t.Parallel()

	var authorization, path string
	var body map[string]string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		assert.Equal(t, http.MethodPost, r.Method)
		path = r.URL.Path
		authorization = r.Header.Get("Authorization")
		raw, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.NoError(t, json.Unmarshal(raw, &body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"success","data":{"account_id":7,"roles":["administration"]}}`))
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

	require.NoError(t, (seedTraegerBueroStep{}).Run(context.Background(), runtime))
	assert.Equal(t, "/operator/organizations/12/traeger-members", path)
	assert.Equal(t, "Bearer operator-token", authorization)
	assert.Equal(t, map[string]string{"email": "admin@example.test", "role": "administration"}, body)
	assert.Equal(t, tenantAuth, client.auth, "the step restores the client's own session")
}
