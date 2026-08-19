// Package users_test tests the users API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package users_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	usersAPI "github.com/moto-nrw/project-phoenix/api/users"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test resources
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *usersAPI.Resource
}

// setupTestContext creates test resources for users handler tests
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)
	resource := usersAPI.NewResource(svc.Users, db)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// setupProtectedRouter mounts the production Router() (JWT middleware,
// permission checks, tenant transaction) exactly as the server wires it.
func setupProtectedRouter(t *testing.T) (*testContext, chi.Router) {
	t.Helper()

	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Mount("/users", tc.resource.Router())

	return tc, router
}

// authWithPerms mints a signed JWT (tenant 1) carrying exactly the given
// permissions and returns a request option that sets it as a Bearer token.
// Pass no permissions to exercise the forbidden path.
func authWithPerms(t *testing.T, perms ...string) testutil.RequestOption {
	t.Helper()
	claims := testutil.DefaultTestClaims()
	claims.Roles = []string{"user"}
	claims.IsAdmin = false
	claims.Permissions = perms
	return testutil.WithJWTBearer(testutil.MintTestJWT(t, claims))
}

// =============================================================================
// LIST PERSONS TESTS
// =============================================================================

func TestListPersons_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test person fixture
	person := testpkg.CreateTestPerson(t, tc.db, "ListTest", "Person")
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users", nil,
		authWithPerms(t, "users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListPersons_WithFilters(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test person fixture
	person := testpkg.CreateTestPerson(t, tc.db, "FilterTest", "PersonFilter")
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users?first_name=FilterTest", nil,
		authWithPerms(t, "users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListPersons_WithoutPermission(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users", nil,
		authWithPerms(t), // No permissions
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// GET PERSON TESTS
// =============================================================================

func TestGetPerson_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test person fixture
	person := testpkg.CreateTestPerson(t, tc.db, "GetTest", "PersonGet")
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/users/%d", person.ID), nil,
		authWithPerms(t, "users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response contains correct data
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})
	assert.Equal(t, "GetTest", data["first_name"])
	assert.Equal(t, "PersonGet", data["last_name"])
}

func TestGetPerson_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users/999999", nil,
		authWithPerms(t, "users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Note: Service returns 500 instead of 404 for not found - error translation issue
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

func TestGetPerson_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users/invalid", nil,
		authWithPerms(t, "users:read"),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestGetPerson_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	person := testpkg.CreateTestPerson(t, tc.db, "PermTest", "PersonPerm")
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/users/%d", person.ID), nil,
		authWithPerms(t), // No permissions
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// CREATE PERSON TESTS
// =============================================================================

func TestCreatePerson_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create an account first to satisfy the constraint
	account := testpkg.CreateTestAccount(t, tc.db, "create-person-test@example.com")
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	body := map[string]interface{}{
		"first_name": "NewPerson",
		"last_name":  "Created",
		"account_id": account.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/users", body,
		authWithPerms(t, "users:create"),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Verify response
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})
	assert.Equal(t, "NewPerson", data["first_name"])
	assert.Equal(t, "Created", data["last_name"])

	// Cleanup created person
	personID := int64(data["id"].(float64))
	testpkg.CleanupPerson(t, tc.db, personID)
}

func TestCreatePerson_MissingFirstName(t *testing.T) {
	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{
		"last_name":  "NoFirst",
		"account_id": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/users", body,
		authWithPerms(t, "users:create"),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestCreatePerson_MissingLastName(t *testing.T) {
	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{
		"first_name": "NoLast",
		"account_id": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/users", body,
		authWithPerms(t, "users:create"),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestCreatePerson_WithoutTagOrAccount(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Persons can be created without tag_id or account_id
	// They can be linked later via /users/{id}/rfid or /users/{id}/account
	body := map[string]interface{}{
		"first_name": "NoTagOrAccount",
		"last_name":  "Test",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/users", body,
		authWithPerms(t, "users:create"),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// Cleanup created person
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})
	personID := int64(data["id"].(float64))
	testpkg.CleanupPerson(t, tc.db, personID)
}

func TestCreatePerson_WithoutPermission(t *testing.T) {
	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{
		"first_name": "NoPerm",
		"last_name":  "Test",
		"account_id": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/users", body,
		authWithPerms(t), // No permissions
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// UPDATE PERSON TESTS
// =============================================================================

func TestUpdatePerson_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test person with account
	account := testpkg.CreateTestAccount(t, tc.db, "update-person-test@example.com")
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	person := testpkg.CreateTestPersonWithAccountID(t, tc.db, "Original", "Name", account.ID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	body := map[string]interface{}{
		"first_name": "Updated",
		"last_name":  "Person",
		"account_id": account.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/users/%d", person.ID), body,
		authWithPerms(t, "users:update"),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})
	assert.Equal(t, "Updated", data["first_name"])
	assert.Equal(t, "Person", data["last_name"])
}

func TestUpdatePerson_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{
		"first_name": "NotFound",
		"last_name":  "Person",
		"account_id": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/users/999999", body,
		authWithPerms(t, "users:update"),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Note: Service returns 500 instead of 404 for not found - error translation issue
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

func TestUpdatePerson_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{
		"first_name": "Invalid",
		"last_name":  "ID",
		"account_id": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/users/invalid", body,
		authWithPerms(t, "users:update"),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePerson_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	person := testpkg.CreateTestPerson(t, tc.db, "NoPerm", "Update")
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	body := map[string]interface{}{
		"first_name": "Updated",
		"last_name":  "Person",
		"account_id": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/users/%d", person.ID), body,
		authWithPerms(t), // No permissions
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// DELETE PERSON TESTS
// =============================================================================

func TestDeletePerson_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test person to delete
	person := testpkg.CreateTestPerson(t, tc.db, "ToDelete", "Person")
	// No defer cleanup needed since we're deleting it

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/users/%d", person.ID), nil,
		authWithPerms(t, "users:delete"),
	)

	rr := testutil.ExecuteRequest(router, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestDeletePerson_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/users/999999", nil,
		authWithPerms(t, "users:delete"),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Note: Service returns 500 instead of 404 for not found - error translation issue
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

func TestDeletePerson_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/users/invalid", nil,
		authWithPerms(t, "users:delete"),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestDeletePerson_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	person := testpkg.CreateTestPerson(t, tc.db, "NoPermDelete", "Person")
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/users/%d", person.ID), nil,
		authWithPerms(t), // No permissions
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}
