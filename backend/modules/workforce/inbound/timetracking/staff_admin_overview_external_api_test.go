package timetracking

import (
	"net/http"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTimeTrackingOverviewAPI_LeavesExternalCaregiversOut pins #3823: an
// external caregiver records no working time, so the school-wide overview
// has no row for the guest profile while regular staff keep theirs.
func TestTimeTrackingOverviewAPI_LeavesExternalCaregiversOut(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	guest := testpkg.CreateTestGuest(t, ctx.tc.db, "Trommeln")
	testpkg.CreateTestStaff(t, ctx.tc.db, "Regulaere", "Kraft")

	rec := ctx.get("/staff/time-tracking/overview", "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Regulaere")
	assert.NotContains(t, rec.Body.String(), "Instructor", "guest %d must not appear", guest.StaffID)
}
