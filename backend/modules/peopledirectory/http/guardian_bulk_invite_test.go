package users_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	usersHTTP "github.com/moto-nrw/project-phoenix/modules/peopledirectory/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bulkInviteDirectory() *fakeGuardianDirectory {
	return &fakeGuardianDirectory{students: map[int64]peopledirectory.Student{
		3: {ID: 3, Status: "active"},
		4: {ID: 4, Status: peopledirectory.StudentStatusAlumnus},
	}}
}

func TestBulkInviteIsGatedLikeTheSingleInvite(t *testing.T) {
	t.Parallel()
	h := newGuardianHarness(t, bulkInviteDirectory())
	body := map[string]any{"student_ids": []int64{3}}

	assert.Equal(t, http.StatusForbidden, h.do(t, http.MethodPost, "/guardians/bulk-invite", body).Code, "users:create is required")

	h.permitted["users:create"] = true
	h.staff = false
	assert.Equal(t, http.StatusForbidden, h.do(t, http.MethodPost, "/guardians/bulk-invite", body).Code, "neither admin nor verified staff")

	h.actorID = 0
	assert.Equal(t, http.StatusUnauthorized, h.do(t, http.MethodPost, "/guardians/bulk-invite", body).Code)
	assert.Empty(t, h.bulkInvites, "a refused request never reaches the invitation flow")
}

func TestBulkInviteOnlyPassesActiveChildrenOfTheSchool(t *testing.T) {
	t.Parallel()
	h := newGuardianHarness(t, bulkInviteDirectory())
	h.permitted["users:create"] = true
	h.bulkResult = usersHTTP.GuardianBulkInviteResult{
		Invited: 2, SkippedOpen: 1,
		Problems: []usersHTTP.GuardianBulkInviteProblem{{GuardianProfileID: 9, GuardianName: "Katharina Brenner", Reason: "missing_email"}},
	}

	recorder := h.do(t, http.MethodPost, "/guardians/bulk-invite", map[string]any{
		"student_ids": []int64{3, 4, 99}, "resend_open": true, "dry_run": true,
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, h.bulkInvites, 1)
	assert.Equal(t, usersHTTP.GuardianBulkInvite{StudentIDs: []int64{3}, ActorAccountID: 42, ResendOpen: true, DryRun: true}, h.bulkInvites[0],
		"the graduated child and the unknown ID are dropped")
	assert.JSONEq(t, `{
		"dry_run": true, "invited": 2, "linked_existing_account": 0, "resent": 0,
		"skipped_active": 0, "skipped_open": 1, "skipped_restricted": 0,
		"problems": [{"guardian_profile_id": "9", "guardian_name": "Katharina Brenner", "student_names": [], "reason": "missing_email"}]
	}`, string(decode(t, recorder).Data))
}

func TestBulkInviteRejectsSelectionsWithoutAnActiveChild(t *testing.T) {
	t.Parallel()
	h := newGuardianHarness(t, bulkInviteDirectory())
	h.permitted["users:create"] = true

	assert.Equal(t, http.StatusBadRequest, h.do(t, http.MethodPost, "/guardians/bulk-invite", map[string]any{"student_ids": []int64{}}).Code)
	assert.Equal(t, http.StatusNotFound, h.do(t, http.MethodPost, "/guardians/bulk-invite", map[string]any{"student_ids": []int64{4, 99}}).Code)
	assert.Empty(t, h.bulkInvites)
}

func TestBulkInviteRejectsOversizedSelectionsBeforeReadingStudents(t *testing.T) {
	t.Parallel()
	directory := bulkInviteDirectory()
	h := newGuardianHarness(t, directory)
	h.permitted["users:create"] = true

	studentIDs := make([]int64, 2001)
	assert.Equal(t, http.StatusBadRequest, h.do(t, http.MethodPost, "/guardians/bulk-invite", map[string]any{"student_ids": studentIDs}).Code)
	assert.Zero(t, directory.studentLookups)
	assert.Empty(t, h.bulkInvites)
}

func TestBulkInviteFailureUsesTheInviteClassification(t *testing.T) {
	t.Parallel()
	h := newGuardianHarness(t, bulkInviteDirectory())
	h.permitted["users:create"] = true
	h.bulkErr = errors.New("boom")

	assert.Equal(t, http.StatusForbidden, h.do(t, http.MethodPost, "/guardians/bulk-invite", map[string]any{"student_ids": []int64{3}}).Code)
}
