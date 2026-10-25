package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttendanceLogSettingPurgesPreviousDaysOnDisableAndReset(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			route := setupSettingsCallbackRoute(t)
			request := func(method string, value bool) int {
				t.Helper()
				req := testutil.NewAuthenticatedRequest(t, method, "/values/gdpr.attendance_log_enabled", map[string]any{"value": value}, testutil.WithJWTBearer(testutil.MintTestJWT(t, adminClaimsForCallback())))
				return testutil.ExecuteRequest(route.router, req).Code
			}
			require.Equal(t, http.StatusOK, request(http.MethodPut, true))
			student := testpkg.CreateTestStudent(t, db, "Synthetisch", "Schalter", "3a")
			group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
			exit := calendar.NewDate(2026, 1, 12).BerlinMidnight().Add(12 * time.Hour)
			visit := testpkg.CreateTestVisit(t, db, student.ID, group.ID, exit.Add(-time.Hour), &exit)
			expected := http.StatusOK
			if method == http.MethodDelete {
				expected = http.StatusNoContent
			}
			require.Equal(t, expected, request(method, false))
			exists, err := db.NewSelect().Table("active.visits").Where("id = ?", visit.ID).Exists(testpkg.Ctx(t))
			require.NoError(t, err)
			assert.False(t, exists)
			count, err := db.NewSelect().Table("audit.data_deletions").Where("tenant_id = ?", testpkg.Tenant(t)).Where("student_id = ?", student.ID).Where("deletion_type = 'visit_retention'").Count(testpkg.Ctx(t))
			require.NoError(t, err)
			assert.EqualValues(t, 1, count)
		})
	}
}
