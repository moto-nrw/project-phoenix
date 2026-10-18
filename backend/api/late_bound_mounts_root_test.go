package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// checkFormerLateBoundMounts drives one live request through each route the
// old service graph mounted over a value it bound only after construction
// (ADR 0059): the meal plan and feedback settings, the room deletion guard,
// the guardian directory provider and the calendar period administration.
// Each answers over the dependency the serving root now passes at
// construction, never through an unbound fallback.
func checkFormerLateBoundMounts(t *testing.T, graph *goldenRuntime) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("late-bound-%d@test.local", time.Now().UnixNano()))
	token := testutil.MintTestJWT(t, testutil.AdminTestClaimsForTenant(int(account.ID), testpkg.Tenant(t)))
	request := func(method, path, body string) (int, string) {
		response := checkpointRequest(graph, checkpointScenario{Method: method, Path: path, Authenticated: true, Body: body}, token)
		return response.Code, response.Body.String()
	}

	t.Run("meal plan settings", func(t *testing.T) {
		code, body := request(http.MethodGet, "/api/meal-plan/?week_start=2030-01-07", "")
		assert.Contains(t, []int{http.StatusOK, http.StatusForbidden}, code, body)
		if code == http.StatusForbidden {
			assert.Contains(t, body, "meals_plan_disabled", "a refusal comes from the tenant setting")
		}
	})
	t.Run("feedback settings", func(t *testing.T) {
		code, body := request(http.MethodGet, "/api/feedback/", "")
		assert.Contains(t, []int{http.StatusOK, http.StatusForbidden}, code, body)
		if code == http.StatusForbidden {
			assert.Contains(t, body, "feedback_disabled", "a refusal comes from the tenant setting")
		}
	})
	t.Run("room deletion guard", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("late-bound-room-%d", time.Now().UnixNano()))
		code, body := request(http.MethodDelete, fmt.Sprintf("/api/rooms/%d", room.ID), "")
		assert.Contains(t, []int{http.StatusOK, http.StatusNoContent}, code, body)
	})
	t.Run("guardian directory provider", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Late", "Bound", "1a")
		code, body := request(http.MethodGet, fmt.Sprintf("/api/guardians/students/%d/guardians", student.ID), "")
		assert.Equal(t, http.StatusOK, code, body)
	})
	t.Run("calendar period administration", func(t *testing.T) {
		code, body := request(http.MethodPost, "/api/timetable/periods/",
			fmt.Sprintf(`{"name":"Late-Bound-%d","period_type":"school_year","start_date":"2040-08-01","end_date":"2041-07-31","is_active":false}`, time.Now().UnixNano()))
		require.Equal(t, http.StatusCreated, code, body)
	})
}
