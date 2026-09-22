package compose_test

import (
	"net/http"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/schoolsetup"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWizardRunsFromFirstAnswerToCompletion walks one new school through the
// wizard against Postgres: the presence mode an admin sets during setup is
// written, the steps follow the answers and the school's data, and a
// completed school refuses further wizard writes (ADR 0040).
func TestWizardRunsFromFirstAnswerToCompletion(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	admin := testpkg.CreateTestAccount(t, h.db, "wizard-admin")

	response, status := h.do(t, admin.ID, http.MethodGet, "/", nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.False(t, status.Completed)
	assert.False(t, step(t, status, string(schoolsetup.StepBasics)).Done)

	response, status = h.do(t, admin.ID, http.MethodPut, "/basics", map[string]any{
		"presence_mode":   schoolsetup.PresenceModeBinary,
		"parent_app_used": false,
	})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, configModel.PresenceModeBinary, h.presenceMode(t), "the admin's answer reaches the operator-only setting")
	assert.True(t, step(t, status, string(schoolsetup.StepBasics)).Done)
	assert.False(t, step(t, status, string(schoolsetup.StepRooms)).Applies)
	assert.False(t, step(t, status, string(schoolsetup.StepGuardians)).Applies)

	testpkg.CreateTestStudent(t, h.db, "Wizard", "Kind", "1a")
	response, status = h.do(t, admin.ID, http.MethodGet, "/", nil)
	require.Equal(t, http.StatusOK, response.Code)
	assert.True(t, step(t, status, string(schoolsetup.StepStudents)).Done)

	response, _ = h.do(t, admin.ID, http.MethodPost, "/complete", nil)
	require.Equal(t, http.StatusConflict, response.Code)
	assert.Equal(t, "school_setup_incomplete", response.Header().Get("X-Conflict-Code"))

	for _, key := range []string{"team", "groups"} {
		response, _ = h.do(t, admin.ID, http.MethodPut, "/steps/"+key, map[string]any{"skipped": true})
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	}
	response, status = h.do(t, admin.ID, http.MethodPost, "/complete", nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.True(t, status.Completed)

	response, _ = h.do(t, admin.ID, http.MethodPut, "/basics", map[string]any{
		"presence_mode":   schoolsetup.PresenceModeDetailed,
		"parent_app_used": true,
	})
	require.Equal(t, http.StatusConflict, response.Code)
	assert.Equal(t, "school_setup_completed", response.Header().Get("X-Conflict-Code"))
	assert.Equal(t, configModel.PresenceModeBinary, h.presenceMode(t), "after completion only moto changes the presence mode")
}

// TestWizardKeepsNothingWhenThePresenceGuardRefuses pins that the presence
// mode and the answers commit together: a refused switch stores no answer.
func TestWizardKeepsNothingWhenThePresenceGuardRefuses(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.openAttendance = true
	admin := testpkg.CreateTestAccount(t, h.db, "wizard-guard")

	response, _ := h.do(t, admin.ID, http.MethodPut, "/basics", map[string]any{
		"presence_mode":   schoolsetup.PresenceModeBinary,
		"parent_app_used": true,
	})

	require.Equal(t, http.StatusConflict, response.Code)
	assert.Equal(t, "presence_mode_switch_blocked", response.Header().Get("X-Conflict-Code"))
	_, status := h.do(t, admin.ID, http.MethodGet, "/", nil)
	assert.False(t, step(t, status, string(schoolsetup.StepBasics)).Done)
	assert.Nil(t, status.Basics.ParentAppUsed)
	assert.Equal(t, configModel.PresenceModeDetailed, h.presenceMode(t))
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
	assert.Equal(t, http.StatusBadRequest, response.Code)

	response, _ = h.do(t, admin.ID, http.MethodPut, "/basics", map[string]any{"presence_mode": schoolsetup.PresenceModeBinary})
	assert.Equal(t, http.StatusBadRequest, response.Code, "parent_app_used is required")

	response, _ = h.do(t, admin.ID, http.MethodPut, "/basics", map[string]any{"presence_mode": "everything", "parent_app_used": true})
	assert.Equal(t, http.StatusBadRequest, response.Code)

	response, _ = h.do(t, 0, http.MethodGet, "/", nil)
	assert.Equal(t, http.StatusForbidden, response.Code)
}
