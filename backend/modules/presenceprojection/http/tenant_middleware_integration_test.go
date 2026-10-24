package presenceprojectionhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTestTenantTxMiddlewareDecidesLikeProduction pins the test root helper
// (testpkg.TenantTxMiddleware) to the production decision so a test cannot
// pass on a transaction shape production would reject.
func TestTestTenantTxMiddlewareDecidesLikeProduction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID, err := tenant.NewTenantID(testpkg.Tenant(t))
	require.NoError(t, err)
	middleware := testpkg.TenantTxMiddleware(db)

	serve := func(ctx context.Context) (int, bool, bool) {
		reached, admin := false, false
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reached = true
			admin = tenant.IsAdminTx(r.Context())
			w.WriteHeader(http.StatusNoContent)
		}))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
		return recorder.Code, reached, admin
	}

	code, reached, admin := serve(tenant.WithTenant(context.Background(), tenantID))
	assert.Equal(t, http.StatusNoContent, code, "tenant runs in a tenant transaction")
	assert.True(t, reached)
	assert.False(t, admin)

	code, reached, admin = serve(tenant.WithScope(context.Background(), tenant.ScopePlatform))
	assert.Equal(t, http.StatusNoContent, code, "platform scope without tenant runs administratively")
	assert.True(t, reached)
	assert.True(t, admin)

	code, reached, _ = serve(tenant.WithScope(context.Background(), tenant.ScopeParent))
	assert.Equal(t, http.StatusInternalServerError, code, "authenticated request without tenant is rejected")
	assert.False(t, reached)

	code, reached, _ = serve(context.Background())
	assert.Equal(t, http.StatusNoContent, code, "unauthenticated request passes through like the production root")
	assert.True(t, reached)
}
