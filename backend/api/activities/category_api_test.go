// Category Stammdaten endpoint tests (#2131).
//
// Driven through Resource.Router() so the production middleware chain and the
// activities:manage_categories permission gate are exercised, not bypassed.
package activities_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// categoryManagerClaims are the claims of an OGS-Leitung: the dedicated
// category permission and nothing else that could mask a missing gate.
func categoryManagerClaims() jwt.AppClaims {
	claims := testutil.DefaultTestClaims()
	claims.IsAdmin = false
	claims.Roles = []string{"user"}
	claims.Permissions = []string{permissions.ActivitiesManageCategories}
	return claims
}

// caregiverClaims mirror a Betreuer: every activities:* permission the `user`
// role actually holds, but not the category one.
func caregiverClaims() jwt.AppClaims {
	claims := testutil.DefaultTestClaims()
	claims.IsAdmin = false
	claims.Roles = []string{"user"}
	claims.Permissions = []string{
		permissions.ActivitiesCreate,
		permissions.ActivitiesRead,
		permissions.ActivitiesUpdate,
		permissions.ActivitiesDelete,
		permissions.ActivitiesList,
		"activities:manage",
	}
	return claims
}

func categoryDataFromResponse(t *testing.T, body []byte) map[string]any {
	t.Helper()
	response := testutil.ParseJSONResponse(t, body)
	data, ok := response["data"].(map[string]any)
	require.True(t, ok, "expected object payload, got: %s", string(body))
	return data
}

func TestCreateCategory_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	name := fmt.Sprintf("Essen-%d", time.Now().UnixNano())
	body := map[string]string{"name": name, "description": "Mittagessen", "color": "#FF9500"}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities/categories", body)
	rr := testutil.ExecuteWithAuth(t, ctx.router, req, categoryManagerClaims())

	require.Equal(t, http.StatusCreated, rr.Code, "body: %s", rr.Body.String())
	data := categoryDataFromResponse(t, rr.Body.Bytes())
	assert.Equal(t, name, data["name"])
	assert.Equal(t, false, data["is_system"])

	categoryID := int64(data["id"].(float64))
	defer cleanupCategory(t, ctx.db, categoryID)
}

func TestCreateCategory_RequiresManageCategoriesPermission(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	body := map[string]string{"name": fmt.Sprintf("Verboten-%d", time.Now().UnixNano())}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities/categories", body)
	rr := testutil.ExecuteWithAuth(t, ctx.router, req, caregiverClaims())

	assert.Equal(t, http.StatusForbidden, rr.Code, "a Betreuer must not manage category Stammdaten; body: %s", rr.Body.String())
}

func TestCreateCategory_RejectsEmptyName(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities/categories", map[string]string{"name": "   "})
	rr := testutil.ExecuteWithAuth(t, ctx.router, req, categoryManagerClaims())

	assert.Equal(t, http.StatusBadRequest, rr.Code, "body: %s", rr.Body.String())
}

func TestCreateCategory_ConflictOnDuplicateName(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	existing := testpkg.CreateTestActivityCategory(t, ctx.db, "ApiDuplicate")
	defer cleanupCategory(t, ctx.db, existing.ID)

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities/categories", map[string]string{"name": existing.Name})
	rr := testutil.ExecuteWithAuth(t, ctx.router, req, categoryManagerClaims())

	assert.Equal(t, http.StatusConflict, rr.Code, "body: %s", rr.Body.String())
}

func TestUpdateCategory_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	category := testpkg.CreateTestActivityCategory(t, ctx.db, "ApiRename")
	defer cleanupCategory(t, ctx.db, category.ID)

	newName := fmt.Sprintf("Umbenannt-%d", time.Now().UnixNano())
	body := map[string]string{"name": newName, "color": "#5080D8"}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/activities/categories/%d", category.ID), body)
	rr := testutil.ExecuteWithAuth(t, ctx.router, req, categoryManagerClaims())

	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
	data := categoryDataFromResponse(t, rr.Body.Bytes())
	assert.Equal(t, newName, data["name"])
	assert.Equal(t, "#5080D8", data["color"])
}

func TestArchiveAndRestoreCategory(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	category := testpkg.CreateTestActivityCategory(t, ctx.db, "ApiArchive")
	defer cleanupCategory(t, ctx.db, category.ID)

	archiveReq := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/activities/categories/%d", category.ID), nil)
	archiveRR := testutil.ExecuteWithAuth(t, ctx.router, archiveReq, categoryManagerClaims())
	require.Equal(t, http.StatusOK, archiveRR.Code, "body: %s", archiveRR.Body.String())
	assert.NotEmpty(t, categoryDataFromResponse(t, archiveRR.Body.Bytes())["archived_at"])

	// Default list = what a Termin/Aktivität picker offers: archived gone.
	listReq := testutil.NewAuthenticatedRequest(t, "GET", "/activities/categories", nil)
	listRR := testutil.ExecuteWithAuth(t, ctx.router, listReq, categoryManagerClaims())
	require.Equal(t, http.StatusOK, listRR.Code)
	assert.NotContains(t, listRR.Body.String(), category.Name, "archived category must not be offered")

	// The management screen opts in and can therefore restore it.
	inclReq := testutil.NewAuthenticatedRequest(t, "GET", "/activities/categories?include_archived=true", nil)
	inclRR := testutil.ExecuteWithAuth(t, ctx.router, inclReq, categoryManagerClaims())
	require.Equal(t, http.StatusOK, inclRR.Code)
	assert.Contains(t, inclRR.Body.String(), category.Name)

	restoreReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/activities/categories/%d/restore", category.ID), nil)
	restoreRR := testutil.ExecuteWithAuth(t, ctx.router, restoreReq, categoryManagerClaims())
	require.Equal(t, http.StatusOK, restoreRR.Code, "body: %s", restoreRR.Body.String())
	assert.Nil(t, categoryDataFromResponse(t, restoreRR.Body.Bytes())["archived_at"])
}

func TestArchiveCategory_RequiresManageCategoriesPermission(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	category := testpkg.CreateTestActivityCategory(t, ctx.db, "ApiArchiveDenied")
	defer cleanupCategory(t, ctx.db, category.ID)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/activities/categories/%d", category.ID), nil)
	rr := testutil.ExecuteWithAuth(t, ctx.router, req, caregiverClaims())

	assert.Equal(t, http.StatusForbidden, rr.Code, "body: %s", rr.Body.String())
}

func TestListCategories_ReportsUsageCount(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("UsageApi-%d", time.Now().UnixNano()))
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities/categories", nil)
	rr := testutil.ExecuteWithAuth(t, ctx.router, req, categoryManagerClaims())
	require.Equal(t, http.StatusOK, rr.Code)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	list, ok := response["data"].([]any)
	require.True(t, ok, "expected array payload")

	var found bool
	for _, raw := range list {
		entry, entryOK := raw.(map[string]any)
		require.True(t, entryOK)
		if int64(entry["id"].(float64)) != activity.CategoryID {
			continue
		}
		found = true
		assert.Equal(t, float64(1), entry["usage_count"])
	}
	assert.True(t, found, "the activity's category must appear in the list")
}
