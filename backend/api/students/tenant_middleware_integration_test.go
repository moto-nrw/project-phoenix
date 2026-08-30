package students

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantTxMiddlewareMarkRollbackDiscardsWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	runtime := testpkg.TenantRuntime(t, db)

	email := fmt.Sprintf("rollback-mw-%d@test.local", time.Now().UnixNano())
	profile := &users.GuardianProfile{
		FirstName:              "Rollback",
		LastName:               "Marker",
		Email:                  &email,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	repo := repositories.NewFactory(db).GuardianProfile
	handler := common.TenantRuntimeMiddleware(runtime)(common.TenantTxMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, repo.Create(r.Context(), profile))
		require.Positive(t, profile.ID)
		tenant.MarkRollback(r.Context())
		w.WriteHeader(http.StatusOK)
	})))

	request := httptest.NewRequest(http.MethodPost, "/api/guardians", nil)
	request = request.WithContext(tenant.WithTenantID(request.Context(), testpkg.Tenant(t)))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	_, err := repo.FindByID(testpkg.Ctx(t), profile.ID)
	assert.ErrorIs(t, err, users.ErrGuardianProfileNotFound)
}

func TestTenantTxMiddlewareServerErrorDiscardsWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	runtime := testpkg.TenantRuntime(t, db)

	email := fmt.Sprintf("rollback-5xx-%d@test.local", time.Now().UnixNano())
	profile := &users.GuardianProfile{
		FirstName:              "Rollback",
		LastName:               "ServerError",
		Email:                  &email,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	repo := repositories.NewFactory(db).GuardianProfile
	handler := common.TenantRuntimeMiddleware(runtime)(common.TenantTxMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, repo.Create(r.Context(), profile))
		http.Error(w, "failed", http.StatusInternalServerError)
	})))

	request := httptest.NewRequest(http.MethodPost, "/api/guardians", nil)
	request = request.WithContext(tenant.WithTenantID(request.Context(), testpkg.Tenant(t)))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	_, err := repo.FindByID(testpkg.Ctx(t), profile.ID)
	assert.ErrorIs(t, err, users.ErrGuardianProfileNotFound)
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
