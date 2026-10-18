package students

import (
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
