package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type seedTraegerRequest struct {
	authorization string
	body          map[string]any
}

func TestSeedTraegerBueroStepAssignsTheSchoolAdminThroughTheOperatorRoute(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	requests := map[string]seedTraegerRequest{}
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		assert.Equal(t, http.MethodPost, r.Method)
		raw, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		var body map[string]any
		assert.NoError(t, json.Unmarshal(raw, &body))
		mu.Lock()
		requests[r.URL.Path] = seedTraegerRequest{authorization: r.Header.Get("Authorization"), body: body}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/traeger/auth/login":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"authenticated","access_token":"traeger-token","refresh_token":"traeger-refresh"}`))
		default:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"success","data":{"account_id":7,"roles":["administration"]}}`))
		}
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	tenantAuth := AuthRef{Kind: AuthBearer, Token: "tenant-token"}
	client.BindAuth(tenantAuth)
	runtime := &Runtime{
		Client:       client,
		OperatorAuth: AuthRef{Kind: AuthBearer, Token: "operator-token"},
		Bootstrap:    &bootstrapSeedState{OrganizationID: 12, AdminEmail: "admin@example.test", AdminPassword: "Geheim1234%"},
	}

	require.NoError(t, (seedTraegerBueroStep{}).Run(context.Background(), runtime))
	assign := requests["/operator/organizations/12/traeger-members"]
	assert.Equal(t, "Bearer operator-token", assign.authorization)
	assert.Equal(t, map[string]any{"email": "admin@example.test", "role": "administration"}, assign.body)
	login := requests["/traeger/auth/login"]
	assert.Empty(t, login.authorization, "the Träger sign-in carries no session")
	assert.Equal(t, map[string]any{"email": "admin@example.test", "password": "Geheim1234%", "organization_id": float64(12)}, login.body)
	invite := requests["/traeger/accounts/invitations"]
	assert.Equal(t, "Bearer traeger-token", invite.authorization, "the invitation uses the Träger session")
	assert.Equal(t, map[string]any{"email": seedTraegerInvitee, "roles": []any{"billing"}}, invite.body)
	assert.Equal(t, tenantAuth, client.auth, "the step restores the client's own session")
}
