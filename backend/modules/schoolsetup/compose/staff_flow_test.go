package compose_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/schoolsetup"
	"github.com/moto-nrw/project-phoenix/modules/schoolsetup/compose"
	schoolsetuphttp "github.com/moto-nrw/project-phoenix/modules/schoolsetup/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staffHarness struct {
	router chi.Router
}

func newStaffHarness(t *testing.T) (*staffHarness, *harness) {
	t.Helper()
	h := newHarness(t)
	service, err := compose.NewStaffOnboarding()
	require.NoError(t, err)
	return &staffHarness{router: schoolsetuphttp.NewStaffResource(service, harnessRuntime(h.db)).Router()}, h
}

// do sends one request as the account and decodes a 200 body into the status.
func (h *staffHarness) do(t *testing.T, accountID int64, method, path string, body any) (*httptest.ResponseRecorder, schoolsetup.StaffStatus) {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
	}
	ctx := context.WithValue(testpkg.Ctx(t), accountKey{}, accountID)
	request := httptest.NewRequest(method, path, &payload).WithContext(ctx)
	recorder := httptest.NewRecorder()
	h.router.ServeHTTP(recorder, request)
	var status schoolsetup.StaffStatus
	if recorder.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &status))
	}
	return recorder, status
}

// TestStaffFirstStepsRunAgainstPostgres walks one care worker through the
// first steps (#3748): the checklist waits for a group or a child, keeps the
// person's finished and skipped tours, and hides for good once dismissed.
func TestStaffFirstStepsRunAgainstPostgres(t *testing.T) {
	t.Parallel()
	staff, h := newStaffHarness(t)
	worker := testpkg.CreateTestAccount(t, h.db, "first-steps-worker")

	response, status := staff.do(t, worker.ID, http.MethodGet, "/", nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.False(t, status.SchoolReady, "a school without groups or children has nothing to show")
	assert.False(t, status.Dismissed)
	assert.Empty(t, status.DoneSteps)

	testpkg.CreateTestEducationGroup(t, h.db, "Erste Schritte Gruppe")
	_, status = staff.do(t, worker.ID, http.MethodGet, "/", nil)
	assert.True(t, status.SchoolReady)

	response, status = staff.do(t, worker.ID, http.MethodPut, "/steps/students", map[string]any{"state": "done"})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, []string{"students"}, status.DoneSteps)

	response, status = staff.do(t, worker.ID, http.MethodPut, "/steps/work_time", map[string]any{"state": "skipped"})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, []string{"work_time"}, status.SkippedSteps)
	assert.Equal(t, []string{"students"}, status.DoneSteps, "the earlier tour stays done")

	response, status = staff.do(t, worker.ID, http.MethodPut, "/dismissal", map[string]any{"dismissed": true})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.True(t, status.Dismissed)
	assert.Equal(t, []string{"students"}, status.DoneSteps, "hiding keeps the progress")
}

func TestStaffFirstStepsArePersonal(t *testing.T) {
	t.Parallel()
	staff, h := newStaffHarness(t)
	worker := testpkg.CreateTestAccount(t, h.db, "first-steps-own")
	colleague := testpkg.CreateTestAccount(t, h.db, "first-steps-colleague")

	response, _ := staff.do(t, worker.ID, http.MethodPut, "/steps/calendar", map[string]any{"state": "done"})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	response, _ = staff.do(t, worker.ID, http.MethodPut, "/dismissal", map[string]any{"dismissed": true})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	_, status := staff.do(t, colleague.ID, http.MethodGet, "/", nil)
	assert.False(t, status.Dismissed)
	assert.Empty(t, status.DoneSteps)
}

func TestStaffFirstStepsRejectBadRequests(t *testing.T) {
	t.Parallel()
	staff, h := newStaffHarness(t)
	worker := testpkg.CreateTestAccount(t, h.db, "first-steps-bad")

	response, _ := staff.do(t, worker.ID, http.MethodPut, "/steps/team", map[string]any{"state": "done"})
	assert.Equal(t, http.StatusNotFound, response.Code, "the school wizard's steps are not staff steps")

	response, _ = staff.do(t, worker.ID, http.MethodPut, "/steps/students", map[string]any{"state": "finished"})
	assert.Equal(t, http.StatusBadRequest, response.Code)

	response, _ = staff.do(t, worker.ID, http.MethodPut, "/steps/students", nil)
	assert.Equal(t, http.StatusBadRequest, response.Code, "a missing body is no state")

	response, _ = staff.do(t, 0, http.MethodGet, "/", nil)
	assert.Equal(t, http.StatusForbidden, response.Code)
}
