package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedBillingKeyDateCountsStepUsesTheLocalOperatorRoute(t *testing.T) {
	t.Parallel()

	var authorization, seedHeader string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/operator/billing/key-date-counts/seed", r.URL.Path)
		authorization = r.Header.Get("Authorization")
		seedHeader = r.Header.Get(seedTokenHeader)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"written":1}}`))
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	tenantAuth := AuthRef{Kind: AuthBearer, Token: "tenant-token"}
	client.BindAuth(tenantAuth)
	runtime := &Runtime{
		Client:       client,
		OperatorAuth: AuthRef{Kind: AuthBearer, Token: "operator-token"},
	}

	require.NoError(t, (seedBillingKeyDateCountsStep{}).Run(context.Background(), runtime))
	assert.Equal(t, "Bearer operator-token", authorization)
	assert.Equal(t, "true", seedHeader)
	assert.Equal(t, tenantAuth, client.auth)
}
