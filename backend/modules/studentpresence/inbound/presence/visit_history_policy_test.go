package presence_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVisitReadsRespectDisabledHistory(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	student := testpkg.CreateTestStudent(t, tc.db, "Synthetisch", "Leseschutz", "3a")
	group := testpkg.CreateTestActiveGroupForTenant(t, tc.db, testpkg.Tenant(t))
	// Fixed historical data stays outside today's window without midnight races.
	exit := calendar.NewDate(2026, 1, 12).BerlinMidnight().Add(12 * time.Hour)
	visit := testpkg.CreateTestVisit(t, tc.db, student.ID, group.ID, exit.Add(-time.Hour), &exit)
	paths := []string{"/active/visits?active=false", "/active/visits", fmt.Sprintf("/active/visits/group/%d", group.ID)}
	for _, enabled := range []bool{false, true} {
		require.NoError(t, tc.settings.SetBool(testpkg.Ctx(t), "gdpr.attendance_log_enabled", enabled, nil, nil))
		for _, path := range paths {
			req := testutil.NewJSONRequest(t, "GET", path, nil)
			rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(1), []string{permissions.GroupsRead})
			require.Equal(t, 200, rr.Code, path)
			data := testutil.ParseJSONResponse(t, rr.Body.Bytes())["data"].([]interface{})
			if enabled {
				assert.Len(t, data, 1, path)
			} else {
				assert.Empty(t, data, path)
			}
		}
		req := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/active/visits/%d", visit.ID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(1), []string{permissions.GroupsRead})
		if enabled {
			assert.Equal(t, 200, rr.Code)
		} else {
			assert.Equal(t, 404, rr.Code)
		}
	}
}

func TestVisitHistoryReadFailsClosedOnSettingError(t *testing.T) {
	t.Parallel()
	tc := setupActiveRoute(t, &settingsDouble{ResolveBoolFn: func(context.Context, string) (bool, error) {
		return true, errors.New("private settings failure")
	}})
	router := tc.resource.Router()
	req := testutil.NewJSONRequest(t, "GET", "/visits?active=false", nil)
	rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(1), []string{permissions.GroupsRead})
	assert.Equal(t, 500, rr.Code)
	assert.NotContains(t, rr.Body.String(), "private settings failure")
	assert.NotContains(t, rr.Body.String(), "Visits retrieved successfully")
}
