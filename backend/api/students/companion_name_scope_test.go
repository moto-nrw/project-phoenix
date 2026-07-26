package students_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestGetStudentCompanions_RedactsUnreadableNames pins the read scope of the
// COMPANION names, not just of the subject.
//
// Linking across groups is the point of a Laufgemeinschaft, so a group
// supervisor routinely opens a child of theirs whose companion belongs to
// another group. Under the default gdpr.student_data_scope
// (group_supervisors_only) that other child's first and last name are personal
// data the caller may not read — the link itself stays visible (they may edit
// the list), the name does not.
func TestGetStudentCompanions_RedactsUnreadableNames(t *testing.T) {
	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "CompanionScope", "Supervisor")
	myGroup := testpkg.CreateTestEducationGroup(t, tc.db, "CompanionScopeMine")
	otherGroup := testpkg.CreateTestEducationGroup(t, tc.db, "CompanionScopeOther")

	subject := testpkg.CreateTestStudent(t, tc.db, "CompanionScope", "Subject", "CS1")
	sameGroup := testpkg.CreateTestStudent(t, tc.db, "CompanionScope", "Nachbar", "CS1")
	crossGroup := testpkg.CreateTestStudent(t, tc.db, "CompanionScope", "Fremd", "CS2")
	// Students before groups: the cleanup deletes education.groups in the same
	// pass, and a group still referenced by a student cannot go.
	defer testpkg.CleanupActivityFixtures(t, tc.db, subject.ID, sameGroup.ID, crossGroup.ID, myGroup.ID, otherGroup.ID, teacher.ID)

	testpkg.AssignStudentToGroup(t, tc.db, subject.ID, myGroup.ID)
	testpkg.AssignStudentToGroup(t, tc.db, sameGroup.ID, myGroup.ID)
	testpkg.AssignStudentToGroup(t, tc.db, crossGroup.ID, otherGroup.ID)
	testpkg.CreateTestGroupTeacher(t, tc.db, myGroup.ID, teacher.ID)

	// Every child needs an accompanied Monday before the links are legal; the
	// links themselves are written through the student PUT (there is no
	// companion PUT), so an admin sets them up.
	putStudent(t, tc, sameGroup.ID, accompaniedPlanBody(nil))
	putStudent(t, tc, crossGroup.ID, accompaniedPlanBody(nil))
	putStudent(t, tc, subject.ID, accompaniedPlanBody(map[string]any{
		"companions": []map[string]any{
			{"companion_student_id": sameGroup.ID, "weekdays": []string{"mon"}},
			{"companion_student_id": crossGroup.ID, "weekdays": []string{"mon"}},
		},
		"companions_fingerprint": mondayLinkFingerprint(),
	}))

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d/companions", subject.ID), nil)
	rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), []string{"users:read"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Data []struct {
			CompanionStudentID int64    `json:"companion_student_id"`
			FirstName          string   `json:"first_name"`
			LastName           string   `json:"last_name"`
			Weekdays           []string `json:"weekdays"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 2, "both links must stay visible")

	byID := map[int64]struct {
		first, last string
		weekdays    []string
	}{}
	for _, entry := range resp.Data {
		byID[entry.CompanionStudentID] = struct {
			first, last string
			weekdays    []string
		}{entry.FirstName, entry.LastName, entry.Weekdays}
	}

	mine, ok := byID[sameGroup.ID]
	require.True(t, ok, "the supervised companion must be listed")
	assert.Equal(t, "CompanionScope", mine.first, "a companion in the caller's own group keeps its name")
	assert.Equal(t, "Nachbar", mine.last)

	foreign, ok := byID[crossGroup.ID]
	require.True(t, ok, "the cross-group link itself must stay visible")
	assert.Empty(t, foreign.first, "a companion outside the caller's read scope must not disclose its name")
	assert.Empty(t, foreign.last, "a companion outside the caller's read scope must not disclose its name")
	assert.Equal(t, []string{"mon"}, foreign.weekdays, "the weekdays of a redacted link still travel")
}
