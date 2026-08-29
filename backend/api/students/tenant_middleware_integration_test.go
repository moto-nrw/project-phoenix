package students

import (
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
