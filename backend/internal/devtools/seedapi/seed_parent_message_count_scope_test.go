package api

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedParentMessageCountScopeStepUsesStaffSession(t *testing.T) {
	t.Parallel()

	var loginEmail, scopeAuth string
	var scopeBody map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/login":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			loginEmail, _ = body["email"].(string)
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"access_token":"staff-token"}}`)
		case r.URL.Path == "/api/messages/count-scope" && r.Method == seedHTTPMethodPut:
			scopeAuth = r.Header.Get("Authorization")
			require.NoError(t, json.NewDecoder(r.Body).Decode(&scopeBody))
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"scope":"own_groups"}}`)
		default:
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	fs := NewFixedSeeder(client, false, "")
	for i, name := range []string{"Anna Admin", "Berta Betreuung"} {
		fs.staffCredentials = append(fs.staffCredentials, StaffCredentials{
			Email: fmt.Sprintf("staff%d@example.test", i), Password: "Staff1234%", Name: name,
		})
		fs.staffIDs[name] = int64(10 + i)
	}
	rt := &Runtime{Client: client, Adapter: client.adapter, FixedSeeder: fs}

	require.NoError(t, (seedParentMessageCountScopeStep{}).Run(t.Context(), rt))
	assert.Equal(t, "staff1@example.test", loginEmail, "the second account chooses, the developer login keeps the default")
	assert.Equal(t, "Bearer staff-token", scopeAuth)
	assert.Equal(t, map[string]any{"scope": "own_groups"}, scopeBody)
}

func TestSeedParentMessageCountScopeStepRequiresTwoStaffAccounts(t *testing.T) {
	t.Parallel()

	err := (seedParentMessageCountScopeStep{}).Run(t.Context(), &Runtime{})
	require.ErrorContains(t, err, "prerequisites")

	client := newTestClient("http://127.0.0.1:1", false)
	rt := &Runtime{Client: client, Adapter: client.adapter, FixedSeeder: NewFixedSeeder(client, false, "")}
	require.ErrorContains(t, (seedParentMessageCountScopeStep{}).Run(t.Context(), rt), "at least two staff accounts")
}
