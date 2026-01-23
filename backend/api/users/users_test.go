// Package users_test tests the users API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package users_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	usersAPI "github.com/moto-nrw/project-phoenix/api/users"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// generateHexID generates a unique hexadecimal ID for testing RFID cards
func generateHexID(prefix string) string {
	// Use nanoseconds converted to hex for unique, valid RFID IDs
	nano := time.Now().UnixNano()
	return fmt.Sprintf("%s%X", prefix, nano)
}

// createHexRFIDCard creates an RFID card with a valid hexadecimal ID for service-layer testing
func createHexRFIDCard(t *testing.T, db *bun.DB, prefix string) *users.RFIDCard {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hexID := generateHexID(prefix)
	card := &users.RFIDCard{
		Active: true,
	}
	card.ID = hexID

	err := db.NewInsert().
		Model(card).
		ModelTableExpr(`users.rfid_cards`).
		Scan(ctx)
	require.NoError(t, err, "Failed to create test RFID card with hex ID")

	return card
}

// testContext holds shared test resources
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *usersAPI.Resource
	ogsID    string
}

// setupTestContext creates test resources for users handler tests
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)
	ogsID := testpkg.SetupTestOGS(t, db)
	resource := usersAPI.NewResource(svc.Users)

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	})

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
		ogsID:    ogsID,
	}
}

// setupProtectedRouter creates a router for testing protected endpoints
func setupProtectedRouter(t *testing.T) (*testContext, chi.Router) {
	t.Helper()

	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Use(testutil.TenantRLSMiddleware(tc.db))

	// Mount routes without JWT middleware for testing
	router.Route("/users", func(r chi.Router) {
		// Read operations
		r.With(authorize.RequiresPermission("users:read")).Get("/", tc.resource.ListPersonsHandler())
		r.With(authorize.RequiresPermission("users:read")).Get("/{id}", tc.resource.GetPersonHandler())
		r.With(authorize.RequiresPermission("users:read")).Get("/by-tag/{tagId}", tc.resource.GetPersonByTagHandler())
		r.With(authorize.RequiresPermission("users:read")).Get("/search", tc.resource.SearchPersonsHandler())
		r.With(authorize.RequiresPermission("users:read")).Get("/by-account/{accountId}", tc.resource.GetPersonByAccountHandler())
		r.With(authorize.RequiresPermission("users:read")).Get("/rfid-cards/available", tc.resource.ListAvailableRFIDCardsHandler())

		// Write operations
		r.With(authorize.RequiresPermission("users:create")).Post("/", tc.resource.CreatePersonHandler())
		r.With(authorize.RequiresPermission("users:update")).Put("/{id}", tc.resource.UpdatePersonHandler())
		r.With(authorize.RequiresPermission("users:delete")).Delete("/{id}", tc.resource.DeletePersonHandler())

		// Special operations
		r.With(authorize.RequiresPermission("users:update")).Put("/{id}/rfid", tc.resource.LinkRFIDHandler())
		r.With(authorize.RequiresPermission("users:update")).Delete("/{id}/rfid", tc.resource.UnlinkRFIDHandler())
		r.With(authorize.RequiresPermission("users:update")).Put("/{id}/account", tc.resource.LinkAccountHandler())
		r.With(authorize.RequiresPermission("users:update")).Delete("/{id}/account", tc.resource.UnlinkAccountHandler())
		r.With(authorize.RequiresPermission("users:read")).Get("/{id}/profile", tc.resource.GetFullProfileHandler())
	})

	return tc, router
}

// =============================================================================
// LIST PERSONS TESTS
// =============================================================================

func TestListPersons_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test person fixture
	person := testpkg.CreateTestPerson(t, tc.db, "ListTest", "Person", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users", nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListPersons_WithFilters(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test person fixture
	person := testpkg.CreateTestPerson(t, tc.db, "FilterTest", "PersonFilter", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users?first_name=FilterTest", nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListPersons_WithoutPermission(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users", nil,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
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
	person := testpkg.CreateTestPerson(t, tc.db, "GetTest", "PersonGet", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/users/%d", person.ID), nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
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
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Note: Service returns 500 instead of 404 for not found - error translation issue
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

func TestGetPerson_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users/invalid", nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestGetPerson_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	person := testpkg.CreateTestPerson(t, tc.db, "PermTest", "PersonPerm", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/users/%d", person.ID), nil,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// SEARCH PERSONS TESTS
// =============================================================================

func TestSearchPersons_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create test person fixture
	person := testpkg.CreateTestPerson(t, tc.db, "SearchTest", "PersonSearch", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users/search?first_name=SearchTest", nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestSearchPersons_ByLastName(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	person := testpkg.CreateTestPerson(t, tc.db, "First", "UniqueSearchLast", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users/search?last_name=UniqueSearchLast", nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestSearchPersons_NoParams(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users/search", nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
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
		testutil.WithPermissions("users:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(tc.ogsID)),
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
		testutil.WithPermissions("users:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
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
		testutil.WithPermissions("users:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
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
		testutil.WithPermissions("users:create"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(tc.ogsID)),
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
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
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

	person := testpkg.CreateTestPersonWithAccountID(t, tc.db, "Original", "Name", account.ID, tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	body := map[string]interface{}{
		"first_name": "Updated",
		"last_name":  "Person",
		"account_id": account.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/users/%d", person.ID), body,
		testutil.WithPermissions("users:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(tc.ogsID)),
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
		testutil.WithPermissions("users:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
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
		testutil.WithPermissions("users:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestUpdatePerson_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	person := testpkg.CreateTestPerson(t, tc.db, "NoPerm", "Update", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	body := map[string]interface{}{
		"first_name": "Updated",
		"last_name":  "Person",
		"account_id": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/users/%d", person.ID), body,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
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
	person := testpkg.CreateTestPerson(t, tc.db, "ToDelete", "Person", tc.ogsID)
	// No defer cleanup needed since we're deleting it

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/users/%d", person.ID), nil,
		testutil.WithPermissions("users:delete"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(tc.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestDeletePerson_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/users/999999", nil,
		testutil.WithPermissions("users:delete"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Note: Service returns 500 instead of 404 for not found - error translation issue
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

func TestDeletePerson_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/users/invalid", nil,
		testutil.WithPermissions("users:delete"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestDeletePerson_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	person := testpkg.CreateTestPerson(t, tc.db, "NoPermDelete", "Person", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/users/%d", person.ID), nil,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// GET FULL PROFILE TESTS
// =============================================================================

func TestGetFullProfile_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	person := testpkg.CreateTestPerson(t, tc.db, "Profile", "Test", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/users/%d/profile", person.ID), nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response contains profile data
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})
	assert.Equal(t, "Profile", data["first_name"])
	assert.Equal(t, "Test", data["last_name"])
}

func TestGetFullProfile_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users/999999/profile", nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Note: Service returns 500 instead of 404 for not found - error translation issue
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

// =============================================================================
// LIST AVAILABLE RFID CARDS TESTS
// =============================================================================

func TestListAvailableRFIDCards_Success(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users/rfid-cards/available", nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestListAvailableRFIDCards_WithoutPermission(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users/rfid-cards/available", nil,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// LINK/UNLINK RFID TESTS
// =============================================================================

func TestLinkRFID_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{
		"tag_id": "TEST123",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/users/invalid/rfid", body,
		testutil.WithPermissions("users:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestLinkRFID_MissingTagID(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	person := testpkg.CreateTestPerson(t, tc.db, "RFID", "LinkTest", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	body := map[string]interface{}{} // Missing tag_id

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/users/%d/rfid", person.ID), body,
		testutil.WithPermissions("users:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestUnlinkRFID_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/users/invalid/rfid", nil,
		testutil.WithPermissions("users:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestUnlinkRFID_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/users/999999/rfid", nil,
		testutil.WithPermissions("users:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Note: Service returns 500 instead of 404 for not found - error translation issue
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

// =============================================================================
// LINK/UNLINK ACCOUNT TESTS
// =============================================================================

func TestLinkAccount_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	body := map[string]interface{}{
		"account_id": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/users/invalid/account", body,
		testutil.WithPermissions("users:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestLinkAccount_MissingAccountID(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	person := testpkg.CreateTestPerson(t, tc.db, "Account", "LinkTest", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	body := map[string]interface{}{} // Missing account_id

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/users/%d/account", person.ID), body,
		testutil.WithPermissions("users:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestUnlinkAccount_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/users/invalid/account", nil,
		testutil.WithPermissions("users:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestUnlinkAccount_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/users/999999/account", nil,
		testutil.WithPermissions("users:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Note: Service returns 500 instead of 404 for not found - error translation issue
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

// =============================================================================
// GET PERSON BY TAG TESTS
// =============================================================================

func TestGetPersonByTag_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users/by-tag/NONEXISTENT123", nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// GET PERSON BY ACCOUNT TESTS
// =============================================================================

func TestGetPersonByAccount_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create account and person linked to it
	account := testpkg.CreateTestAccount(t, tc.db, "person-by-account-test@example.com")
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	person := testpkg.CreateTestPersonWithAccountID(t, tc.db, "ByAccount", "Test", account.ID, tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/users/by-account/%d", account.ID), nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]interface{})
	assert.Equal(t, "ByAccount", data["first_name"])
}

func TestGetPersonByAccount_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users/by-account/invalid", nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestGetPersonByAccount_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users/by-account/999999", nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(jwt.AppClaims{ID: 1}),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertNotFound(t, rr)
}

func TestGetPersonByAccount_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	account := testpkg.CreateTestAccount(t, tc.db, "byaccount-noperm@example.com")
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	person := testpkg.CreateTestPersonWithAccountID(t, tc.db, "ByAccountNoPerm", "Test", account.ID, tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/users/by-account/%d", account.ID), nil,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// GET PERSON BY TAG TESTS (Additional)
// =============================================================================

func TestGetPersonByTag_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create RFID card first (must be valid hex, 8+ chars)
	card := createHexRFIDCard(t, tc.db, "A1B2C3D4")
	defer testpkg.CleanupRFIDCards(t, tc.db, card.ID)

	// Create person with this tag
	person := testpkg.CreateTestPerson(t, tc.db, "TagTest", "Person", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	// Link RFID to person via direct DB update
	testpkg.LinkRFIDToStudent(t, tc.db, person.ID, card.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/users/by-tag/%s", card.ID), nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	assert.Equal(t, "TagTest", data["first_name"])
}

func TestGetPersonByTag_WithoutPermission(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users/by-tag/SOMETAG123", nil,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// SEARCH PERSONS TESTS (Additional)
// =============================================================================

func TestSearchPersons_WithoutPermission(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users/search?first_name=Test", nil,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// GET FULL PROFILE TESTS (Additional)
// =============================================================================

func TestGetFullProfile_InvalidID(t *testing.T) {
	_, router := setupProtectedRouter(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/users/invalid/profile", nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

func TestGetFullProfile_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	person := testpkg.CreateTestPerson(t, tc.db, "ProfileNoPerm", "Test", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/users/%d/profile", person.ID), nil,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

func TestGetFullProfile_WithRFIDAndAccount(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create account
	account := testpkg.CreateTestAccount(t, tc.db, "fullprofile-test@example.com")
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	// Create RFID card (must be valid hex, 8+ chars)
	card := createHexRFIDCard(t, tc.db, "F1E2D3C4")
	defer testpkg.CleanupRFIDCards(t, tc.db, card.ID)

	// Create person with account
	person := testpkg.CreateTestPersonWithAccountID(t, tc.db, "FullProfile", "Test", account.ID, tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	// Link RFID to person via direct DB update
	testpkg.LinkRFIDToStudent(t, tc.db, person.ID, card.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/users/%d/profile", person.ID), nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response contains profile data with RFID and account
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	assert.Equal(t, "FullProfile", data["first_name"])
	assert.NotNil(t, data["account"])
}

// =============================================================================
// LIST PERSONS TESTS (Additional)
// =============================================================================

func TestListPersons_WithTagFilter(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create RFID card (must be valid hex, 8+ chars)
	card := createHexRFIDCard(t, tc.db, "B1C2D3E4")
	defer testpkg.CleanupRFIDCards(t, tc.db, card.ID)

	// Create person with this tag
	person := testpkg.CreateTestPerson(t, tc.db, "ListTagTest", "Person", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	// Link RFID to person via direct DB update
	testpkg.LinkRFIDToStudent(t, tc.db, person.ID, card.ID)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/users?tag_id=%s", card.ID), nil,
		testutil.WithPermissions("users:read"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// LINK RFID TESTS (Additional)
// =============================================================================

func TestLinkRFID_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create person without RFID
	person := testpkg.CreateTestPerson(t, tc.db, "RFIDLink", "Success", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	// Create RFID card (must be valid hex, 8+ chars)
	card := createHexRFIDCard(t, tc.db, "C1D2E3F4")
	defer testpkg.CleanupRFIDCards(t, tc.db, card.ID)

	body := map[string]any{
		"tag_id": card.ID, // Use the actual card ID created
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/users/%d/rfid", person.ID), body,
		testutil.WithPermissions("users:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(tc.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response contains the tag_id (uppercase because service normalizes it)
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	assert.NotEmpty(t, data["tag_id"])
}

func TestLinkRFID_NotFound(t *testing.T) {
	_, router := setupProtectedRouter(t)

	body := map[string]any{
		"tag_id": "ABCD1234EFAB5678",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/users/999999/rfid", body,
		testutil.WithPermissions("users:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Service returns error when person not found
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

func TestLinkRFID_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	person := testpkg.CreateTestPerson(t, tc.db, "RFIDLinkNoPerm", "Test", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	body := map[string]any{
		"tag_id": "ABCD1234EFAB5678",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/users/%d/rfid", person.ID), body,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// UNLINK RFID TESTS (Additional)
// =============================================================================

func TestUnlinkRFID_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create RFID card (must be valid hex, 8+ chars)
	card := createHexRFIDCard(t, tc.db, "D1E2F3A4")
	defer testpkg.CleanupRFIDCards(t, tc.db, card.ID)

	// Create person with this tag
	person := testpkg.CreateTestPerson(t, tc.db, "RFIDUnlink", "Success", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	// Link RFID to person via direct DB update
	testpkg.LinkRFIDToStudent(t, tc.db, person.ID, card.ID)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/users/%d/rfid", person.ID), nil,
		testutil.WithPermissions("users:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(tc.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response has no tag_id
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	assert.Empty(t, data["tag_id"])
}

func TestUnlinkRFID_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	person := testpkg.CreateTestPerson(t, tc.db, "RFIDUnlinkNoPerm", "Test", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/users/%d/rfid", person.ID), nil,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// LINK ACCOUNT TESTS (Additional)
// =============================================================================

func TestLinkAccount_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create person without account
	person := testpkg.CreateTestPerson(t, tc.db, "AccountLink", "Success", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	// Create account to link
	account := testpkg.CreateTestAccount(t, tc.db, "link-account-success@example.com")
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	body := map[string]any{
		"account_id": account.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/users/%d/account", person.ID), body,
		testutil.WithPermissions("users:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(tc.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	assert.Equal(t, float64(account.ID), data["account_id"])
}

func TestLinkAccount_NotFound(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create a valid account to link
	account := testpkg.CreateTestAccount(t, tc.db, "link-account-notfound@example.com")
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	body := map[string]any{
		"account_id": account.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/users/999999/account", body,
		testutil.WithPermissions("users:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	// Service returns error when person not found
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

func TestLinkAccount_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	person := testpkg.CreateTestPerson(t, tc.db, "AccountLinkNoPerm", "Test", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	body := map[string]any{
		"account_id": 1,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/users/%d/account", person.ID), body,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// =============================================================================
// UNLINK ACCOUNT TESTS (Additional)
// =============================================================================

func TestUnlinkAccount_Success(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	// Create account
	account := testpkg.CreateTestAccount(t, tc.db, "unlink-account-success@example.com")
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	// Create person with this account
	person := testpkg.CreateTestPersonWithAccountID(t, tc.db, "AccountUnlink", "Success", account.ID, tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/users/%d/account", person.ID), nil,
		testutil.WithPermissions("users:update"),
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(tc.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response has no account_id (omitempty means it's nil/missing when 0)
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data := response["data"].(map[string]any)
	// account_id is omitempty in JSON, so when unlinked it's either 0 or nil
	accountID := data["account_id"]
	if accountID != nil {
		assert.Equal(t, float64(0), accountID)
	}
}

func TestUnlinkAccount_WithoutPermission(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	person := testpkg.CreateTestPerson(t, tc.db, "AccountUnlinkNoPerm", "Test", tc.ogsID)
	defer testpkg.CleanupPerson(t, tc.db, person.ID)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/users/%d/account", person.ID), nil,
		testutil.WithPermissions(), // No permissions
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}
