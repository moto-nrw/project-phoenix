package students_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The Änderungsanfragen badge must only count queues its holder may open
// (#2232): the Stammdaten, Betreuungszeiten and Angebots queues are
// users:update-gated, the excused-absence queue also accepts users:absence.
// Their services scope per child but never re-check the permission, so an
// absence-only caller would otherwise get a badge leading to three 403s.
func TestPendingChangeRequestCount_AbsenceOnlyExcludesStudentDataQueues(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Count", "Teacher")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "CountBadgeGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Count", "Kind", "CB1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID, group.ID, student.ID)
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	claims := testutil.TeacherTestClaims(int(account.ID))
	absencePerms := []string{"users:read", "users:absence"}
	updatePerms := []string{"users:read", "users:update"}

	pendingCount := func(t *testing.T, perms []string) int {
		t.Helper()
		rr := authExec(t, tc, testutil.NewRequest("GET", "/change-requests/pending-count", nil), claims, perms)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

		var env struct {
			Data struct {
				PendingCount int `json:"pending_count"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
		return env.Data.PendingCount
	}

	// Baselines first: the shared test database carries rows from other tests,
	// so the assertion is on the delta this request causes, not on absolutes.
	absenceBefore := pendingCount(t, absencePerms)
	updateBefore := pendingCount(t, updatePerms)

	request := &userModels.StudentDataChangeRequest{
		StudentID:   student.ID,
		SubmittedBy: account.ID,
		Target:      userModels.DataChangeTargetPerson,
		FieldKey:    "first_name",
		NewValue:    json.RawMessage(`"Neuer Vorname"`),
		Status:      userModels.DataChangeStatusPending,
	}
	request.TenantID = student.TenantID
	_, err := tc.db.NewInsert().Model(request).Exec(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, cleanupErr := tc.db.NewDelete().Model((*userModels.StudentDataChangeRequest)(nil)).
			Where("id = ?", request.ID).Exec(t.Context())
		if cleanupErr != nil {
			t.Logf("cleanup of change request %d failed: %v", request.ID, cleanupErr)
		}
	})

	assert.Equal(t, updateBefore+1, pendingCount(t, updatePerms),
		"a users:update reviewer must see the new Stammdaten request in the badge")
	assert.Equal(t, absenceBefore, pendingCount(t, absencePerms),
		fmt.Sprintf("the absence-only badge must ignore the users:update-gated queues (request %d)", request.ID))
}

// The badge counts only what its holder may open, and the queue behind it must
// stay open to them: an absence reviewer sees the excused queue itself.
func TestExcusedQueueReachable_WithAbsencePermission(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Queue", "Reviewer")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "QueueReviewGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Queue", "Kind", "QR1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID, group.ID, student.ID)
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	rr := authExec(t, tc,
		testutil.NewRequest("GET", "/excused-absence-requests", nil),
		testutil.TeacherTestClaims(int(account.ID)),
		[]string{"users:read", "users:absence"},
	)
	assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}
