package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestDatabaseStatsRejectsMissingAuthentication(t *testing.T) {
	t.Parallel()

	apiInstance := newGoldenAPI(t)
	request := httptest.NewRequest(http.MethodGet, "/api/database/stats", nil)
	response := httptest.NewRecorder()

	apiInstance.Router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestDatabaseStatsReturnsDataForAdmin(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	admin, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Admin", "Stats")
	apiInstance := newGoldenAPI(t)
	token := testutil.MintTestJWT(t, testutil.AdminTestClaims(int(admin.ID)))
	request := httptest.NewRequest(http.MethodGet, "/api/database/stats", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	apiInstance.Router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
}
