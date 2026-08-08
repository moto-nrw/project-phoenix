// Router-level tests for /api/staff/{id}/school-classes (#1772): reading
// rides on users:read like the other staff detail reads, replacing the set is
// a users:update staff-admin write.
package staff_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchoolClassesAPI(t *testing.T) {
	ctx := setupOverviewAPI(t)
	target := testpkg.CreateTestStaff(t, ctx.tc.db, "SchoolClasses", fmt.Sprintf("API-%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		cleanupClassTeacherAssignments(t, ctx, target.ID)
		testpkg.CleanupStaffFixtures(t, ctx.tc.db, target.ID)
	})
	base := fmt.Sprintf("/staff/%d/school-classes", target.ID)

	// Permission split: users:read may read but not write; the write tier is
	// users:update.
	rec := ctx.put(base, `{"school_classes":["1a"]}`, "users:read")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = ctx.get(base, "visits:read")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	// Roundtrip: set, then read back. The response carries the deduped,
	// trimmed set in class order.
	rec = ctx.put(base, `{"school_classes":["2b","1a"," 1A "]}`, "users:update")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"school_classes":["1a","2b"]`)

	rec = ctx.get(base, "users:read")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"school_classes":["1a","2b"]`)

	// Replacing shrinks the set.
	rec = ctx.put(base, `{"school_classes":["3c"]}`, "users:update")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"school_classes":["3c"]`)

	// Blank class names are caller errors.
	rec = ctx.put(base, `{"school_classes":["  "]}`, "users:update")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	// Missing field is a caller error too.
	rec = ctx.put(base, `{}`, "users:update")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestSchoolClassesAPI_UnknownStaff(t *testing.T) {
	ctx := setupOverviewAPI(t)
	ghost := testpkg.CreateTestStaff(t, ctx.tc.db, "SchoolClasses", fmt.Sprintf("Ghost-%d", time.Now().UnixNano()))
	ghostID := ghost.ID
	testpkg.CleanupStaffFixtures(t, ctx.tc.db, ghost.ID)
	base := fmt.Sprintf("/staff/%d/school-classes", ghostID)

	rec := ctx.get(base, "users:read")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	rec = ctx.put(base, `{"school_classes":["1a"]}`, "users:update")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func cleanupClassTeacherAssignments(t *testing.T, ctx *overviewAPIContext, staffID int64) {
	t.Helper()
	tenantCtx := testpkg.TenantContext(1)
	_, _ = ctx.tc.db.NewDelete().TableExpr("education.class_teachers").Where("staff_id = ?", staffID).Exec(tenantCtx)
}
