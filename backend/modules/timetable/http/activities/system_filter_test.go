// Tests for the is_system filtering on the activities list endpoints
// (issue #923): auto-provisioned system activities and categories
// (Schulhof Freispiel, WC) are hidden from staff-facing lists by default
// and only returned with ?include_system=true.
package httpadapter_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// markGroupAsSystem flags an activity group as system infrastructure,
// mirroring what the IoT auto-provisioning code does at creation time.
func markGroupAsSystem(t *testing.T, db *bun.DB, groupID int64) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("activities.groups").
		Set("is_system = TRUE").
		Where("id = ?", groupID).
		Exec(context.Background())
	require.NoError(t, err, "failed to flag group as system")
}

// markCategoryAsSystem flags a category as system infrastructure.
func markCategoryAsSystem(t *testing.T, db *bun.DB, categoryID int64) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("activities.categories").
		Set("is_system = TRUE").
		Where("id = ?", categoryID).
		Exec(context.Background())
	require.NoError(t, err, "failed to flag category as system")
}

// responseIDs extracts the "id" fields from a list response's data array.
func responseIDs(t *testing.T, body []byte) map[int64]bool {
	t.Helper()
	response := testutil.ParseJSONResponse(t, body)
	data, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")

	ids := make(map[int64]bool, len(data))
	for _, entry := range data {
		item, ok := entry.(map[string]interface{})
		require.True(t, ok, "Expected list entry to be an object")
		id, ok := item["id"].(float64)
		require.True(t, ok, "Expected entry id to be a number")
		ids[int64(id)] = true
	}
	return ids
}

func TestListActivities_ExcludesSystemActivitiesByDefault(t *testing.T) {
	t.Parallel()
	ctx := setupActivitiesRoute(t)

	normal := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("NormalAct-%d", time.Now().UnixNano()))
	system := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("SystemAct-%d", time.Now().UnixNano()))

	markGroupAsSystem(t, ctx.db, system.ID)

	t.Run("default_excludes_system", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/activities", nil)
		rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		ids := responseIDs(t, rr.Body.Bytes())
		assert.True(t, ids[normal.ID], "normal activity should be listed")
		assert.False(t, ids[system.ID], "system activity must be hidden by default")
	})

	t.Run("include_system_returns_system", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/activities?include_system=true", nil)
		rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		ids := responseIDs(t, rr.Body.Bytes())
		assert.True(t, ids[normal.ID], "normal activity should be listed")
		assert.True(t, ids[system.ID], "system activity must appear with include_system=true")
	})
}

func TestListCategories_ExcludesSystemCategoriesByDefault(t *testing.T) {
	t.Parallel()
	ctx := setupActivitiesRoute(t)

	normal := testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("NormalCat-%d", time.Now().UnixNano()))
	system := testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("SystemCat-%d", time.Now().UnixNano()))

	markCategoryAsSystem(t, ctx.db, system.ID)

	t.Run("default_excludes_system", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/activities/categories", nil)
		rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		ids := responseIDs(t, rr.Body.Bytes())
		assert.True(t, ids[normal.ID], "normal category should be listed")
		assert.False(t, ids[system.ID], "system category must be hidden by default")
	})

	t.Run("include_system_returns_system", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/activities/categories?include_system=true", nil)
		rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		ids := responseIDs(t, rr.Body.Bytes())
		assert.True(t, ids[normal.ID], "normal category should be listed")
		assert.True(t, ids[system.ID], "system category must appear with include_system=true")
	})
}

func TestGetAvailableActivities_ExcludesSystemActivities(t *testing.T) {
	t.Parallel()
	ctx := setupActivitiesRoute(t)

	student := testpkg.CreateTestStudent(t, ctx.db, "Avail", fmt.Sprintf("Student-%d", time.Now().UnixNano()), "1a")

	normal := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("AvailNormal-%d", time.Now().UnixNano()))
	system := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("AvailSystem-%d", time.Now().UnixNano()))

	// Both groups open for enrollment; one flagged as system infrastructure
	// (mirrors Schulhof Freispiel/WC, which are created with is_open = true).
	for _, id := range []int64{normal.ID, system.ID} {
		_, err := ctx.db.NewUpdate().
			TableExpr("activities.groups").
			Set("is_open = TRUE").
			Where("id = ?", id).
			Exec(context.Background())
		require.NoError(t, err)
	}
	markGroupAsSystem(t, ctx.db, system.ID)

	req := testutil.NewRequest("GET", fmt.Sprintf("/activities/students/%d/available", student.ID), nil)
	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	ids := responseIDs(t, rr.Body.Bytes())
	assert.True(t, ids[normal.ID], "normal open activity should be available for enrollment")
	assert.False(t, ids[system.ID], "system activity must never be offered for enrollment")
}
