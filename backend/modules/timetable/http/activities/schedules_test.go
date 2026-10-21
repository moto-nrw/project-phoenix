// The schedules API tests drive the timeframe list through the production
// middleware chain over the test database (no mocks).
package httpadapter_test

import (
	"net/http"
	"testing"

	rootAPI "github.com/moto-nrw/project-phoenix/api"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	schedulesAPI "github.com/moto-nrw/project-phoenix/modules/timetable/http/activities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// init seeds JWT viper defaults before any test mints claims. CI runs without
// a .env so AUTH_JWT_SECRET is unset; without a secret jwx refuses HMAC signing.
func init() {
	testutil.SeedTestJWTConfig()
}

// schedulesTestContext holds shared test dependencies.
type schedulesTestContext struct {
	timetable timetable.Capability
	router    chi.Router
}

// setupSchedulesRoute initializes test database, services, and resource. The
// router serves the resource through the production middleware chain (Verifier →
// Authenticator → TenantMiddleware → RequiresPermission → TenantTxMiddleware)
// exactly as the real server does.
func setupSchedulesRoute(t *testing.T) *schedulesTestContext {
	t.Helper()

	_, _, owner, _, _ := setupActivitiesModule(t)
	resource := schedulesAPI.NewSchedulesResource(owner)

	return &schedulesTestContext{
		timetable: owner,
		router:    resource.Router(rootAPI.ProtectedTenantRoutes),
	}
}

// TestSchedulesListTimeframes_Success: GET /timeframes is the one schedules
// route the frontend calls (activity forms). It lists the tenant's timeframes
// with the fields the frontend maps: id, start/end clock, active flag and
// description.
func TestSchedulesListTimeframes_Success(t *testing.T) {
	t.Parallel()

	ctx := setupSchedulesRoute(t)
	end := "16:00:00"
	timeframe, err := ctx.timetable.CreateTimeframe(testpkg.Ctx(t), timetable.TimeframeInput{
		StartTime: "08:00:00", EndTime: &end, IsActive: true, Description: "Nachmittag",
	})
	require.NoError(t, err)

	req := testutil.NewRequest("GET", "/timeframes", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.AdminTestClaims(1))

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")

	var listed map[string]interface{}
	for _, entry := range data {
		row, isObject := entry.(map[string]interface{})
		require.True(t, isObject, "Expected every timeframe to be an object")
		id, isNumber := row["id"].(float64)
		require.True(t, isNumber, "Expected every timeframe to carry a numeric id")
		if int64(id) == timeframe.ID {
			listed = row
		}
	}
	require.NotNil(t, listed, "the tenant's timeframe must be listed")
	assert.Equal(t, "Nachmittag", listed["description"])
	assert.Equal(t, true, listed["is_active"])
	assert.Equal(t, "0000-01-01T08:00:00Z", listed["start_time"])
	assert.Equal(t, "0000-01-01T16:00:00Z", listed["end_time"])
}
