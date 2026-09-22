package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func setupDatabaseStatsRoute(t *testing.T) chi.Router {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	resource, err := newDatabaseStatsTestRouter(db)
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/api/database", resource)
	return router
}

func TestDatabaseStatsRejectsMissingAuthentication(t *testing.T) {
	t.Parallel()

	router := setupDatabaseStatsRoute(t)
	request := httptest.NewRequest(http.MethodGet, "/api/database/stats", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

// The Datenverwaltung hub of a role a school defines itself (#3469) shows the
// counts its read permissions allow; a role without any of them gets nothing.
func TestDatabaseStatsFollowsTheReadPermissions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	lead, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Lead", "Stats")
	router := setupDatabaseStatsRoute(t)
	claims := testutil.TeacherTestClaims(int(lead.ID))
	claims.Roles = []string{"ogs-leitung"}

	claims.Permissions = []string{"users:read", "rooms:read"}
	request := httptest.NewRequest(http.MethodGet, "/api/database/stats", nil)
	request.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assert.Equal(t, http.StatusOK, response.Code, response.Body.String())

	claims.Permissions = []string{"calendar:own"}
	request = httptest.NewRequest(http.MethodGet, "/api/database/stats", nil)
	request.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assert.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
}

func TestDatabaseStatsReturnsDataForAdmin(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	admin, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Admin", "Stats")
	router := setupDatabaseStatsRoute(t)
	token := testutil.MintTestJWT(t, testutil.AdminTestClaims(int(admin.ID)))
	request := httptest.NewRequest(http.MethodGet, "/api/database/stats", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
}
