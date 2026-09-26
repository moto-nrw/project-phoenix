package students

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantTxMiddlewareMarkRollbackDiscardsWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	runtime := testpkg.TenantRuntime(t, db)

	probe := testpkg.NewTransactionProbe(t, db)
	handler := common.TenantRuntimeMiddleware(runtime)(common.TenantTxMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, probe.Write(r.Context()))
		require.True(t, probe.Written())
		tenant.MarkRollback(r.Context())
		w.WriteHeader(http.StatusOK)
	})))

	request := httptest.NewRequest(http.MethodPost, "/api/guardians", nil)
	request = request.WithContext(tenant.WithTenantID(request.Context(), testpkg.Tenant(t)))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.False(t, probe.Committed(t), "the written guardian profile must be rolled back")
}

func TestTenantTxMiddlewareServerErrorDiscardsWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	runtime := testpkg.TenantRuntime(t, db)

	probe := testpkg.NewTransactionProbe(t, db)
	handler := common.TenantRuntimeMiddleware(runtime)(common.TenantTxMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, probe.Write(r.Context()))
		http.Error(w, "failed", http.StatusInternalServerError)
	})))

	request := httptest.NewRequest(http.MethodPost, "/api/guardians", nil)
	request = request.WithContext(tenant.WithTenantID(request.Context(), testpkg.Tenant(t)))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.False(t, probe.Committed(t), "the written guardian profile must be rolled back")
}

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
