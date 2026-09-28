package compose_test

import (
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolsetup"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWizardRunsFromFirstStepToCompletion walks one new school through the
// wizard against Postgres: the steps follow the registry defaults and the
// school's data, and a completed school refuses further wizard writes
// (ADR 0040).
func TestWizardRunsFromFirstStepToCompletion(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	admin := testpkg.CreateTestAccount(t, h.db, "wizard-admin")

	response, status := h.do(t, admin.ID, http.MethodGet, "/", nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.False(t, status.Completed)
	require.Len(t, status.Steps, len(schoolsetup.StepKeys))
	for _, candidate := range status.Steps {
		assert.True(t, candidate.Applies, "a new school on the registry defaults gets every step: %s", candidate.Key)
		assert.False(t, candidate.Done, candidate.Key)
	}

	testpkg.CreateTestStudent(t, h.db, "Wizard", "Kind", "1a")
	response, status = h.do(t, admin.ID, http.MethodGet, "/", nil)
	require.Equal(t, http.StatusOK, response.Code)
	assert.True(t, step(t, status, string(schoolsetup.StepStudents)).Done)

	response, _ = h.do(t, admin.ID, http.MethodPost, "/complete", nil)
	require.Equal(t, http.StatusConflict, response.Code)
	assert.Equal(t, "school_setup_incomplete", response.Header().Get("X-Conflict-Code"))

	for _, key := range []string{"team", "rooms", "groups", "guardians"} {
		response, _ = h.do(t, admin.ID, http.MethodPut, "/steps/"+key, map[string]any{"skipped": true})
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	}
	response, status = h.do(t, admin.ID, http.MethodPost, "/complete", nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.True(t, status.Completed)

	response, _ = h.do(t, admin.ID, http.MethodPut, "/steps/team", map[string]any{"skipped": false})
	require.Equal(t, http.StatusConflict, response.Code)
	assert.Equal(t, "school_setup_completed", response.Header().Get("X-Conflict-Code"))
}

func TestWizardDismissalIsPersonal(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	admin := testpkg.CreateTestAccount(t, h.db, "wizard-hide")
	colleague := testpkg.CreateTestAccount(t, h.db, "wizard-colleague")

	response, status := h.do(t, admin.ID, http.MethodPut, "/dismissal", map[string]any{"dismissed": true})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.True(t, status.Dismissed)

	_, status = h.do(t, colleague.ID, http.MethodGet, "/", nil)
	assert.False(t, status.Dismissed)

	_, status = h.do(t, admin.ID, http.MethodPut, "/dismissal", map[string]any{"dismissed": false})
	assert.False(t, status.Dismissed)
}

func TestWizardRejectsBadRequests(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	admin := testpkg.CreateTestAccount(t, h.db, "wizard-bad")

	response, _ := h.do(t, admin.ID, http.MethodPut, "/steps/unknown", map[string]any{"skipped": true})
	assert.Equal(t, http.StatusNotFound, response.Code)

	response, _ = h.do(t, admin.ID, http.MethodPut, "/steps/basics", map[string]any{"skipped": true})
	assert.Equal(t, http.StatusNotFound, response.Code, "the removed first step is no step any more")

	response, _ = h.do(t, admin.ID, http.MethodPut, "/basics", map[string]any{"presence_mode": "binary", "parent_app_used": true})
	assert.Equal(t, http.StatusNotFound, response.Code, "the wizard no longer writes the presence mode")

	response, _ = h.do(t, 0, http.MethodGet, "/", nil)
	assert.Equal(t, http.StatusForbidden, response.Code)
}
