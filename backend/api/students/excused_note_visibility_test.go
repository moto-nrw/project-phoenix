package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestPendingExcusedNote_HiddenFromReadOnlySupervisor locks the #1845 review
// blocker. The pending excused-absence badge carries the parent's note and
// belongs to the users:update-gated review queue (Änderungsanfragen). The
// enrichment also runs inside the users:read student list/detail/export handlers,
// and PendingByStudentForDate scopes only to children the caller may WRITE
// (WritableStudentFilter) — it never checks the users:update permission. So a
// read-only staff member would otherwise receive pending_excused_note for a
// child while being unable to open or decide the queue. The fix gates the whole enrichment on
// users:update; this test pins that only that permission flips the note on.
func TestPendingExcusedNote_HiddenFromReadOnlySupervisor(t *testing.T) {
	tc := setupTestContext(t)

	// The caller is verified staff, so they have full read access to the child
	// (#2329) — HasFullAccess is true for BOTH calls below, isolating the
	// users:update permission as the only variable.
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "ExcusedNote", "Supervisor")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "ExcusedNoteGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Excused", "Note", "EN1")
	// A second account supplies a valid submitted_by (FK) for the seeded request.
	submitter, submitterAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Note", "Submitter")
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID, group.ID, student.ID, submitter.ID)
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	// Seed a pending excused request covering today for this child.
	const note = "Zahnarzttermin am Vormittag"
	err := tenant.WithTenantTx(context.Background(), tc.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, e := tc.services.ExcusedRequests.CreateRequest(
			txCtx, student.ID, submitterAccount.ID,
			[]timezone.Date{timezone.TodayDate()}, note,
		)
		return e
	})
	require.NoError(t, err)

	get := func(perms []string) *string {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := authExec(t, tc, req, claims, perms)
		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		var resp struct {
			Data struct {
				HasFullAccess      bool    `json:"has_full_access"`
				PendingExcusedNote *string `json:"pending_excused_note"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		require.True(t, resp.Data.HasFullAccess, "a staff member must have full read access to the child")
		return resp.Data.PendingExcusedNote
	}

	// Read-only supervisor → the parent's note must NOT leak, even though the
	// child is theirs to view.
	assert.Nil(t, get([]string{"users:read"}),
		"a users:read-only caller must not receive the pending excused note")

	// users:update (the review-queue permission) → the note is shown.
	shown := get([]string{"users:read", "users:update"})
	require.NotNil(t, shown, "a users:update caller should receive the pending excused note")
	assert.Equal(t, note, *shown)
}

// TestPendingExcusedNote_ShownToAbsenceReviewer is the counterpart for #2232:
// users:absence decides exactly these requests, so the badge announcing one
// must reach its reviewer. A note withheld from the person who decides it
// hides work they own — the same drift as the leak above, mirrored.
func TestPendingExcusedNote_ShownToAbsenceReviewer(t *testing.T) {
	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "AbsenceNote", "Supervisor")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "AbsenceNoteGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "AbsenceNote", "Kind", "AN1")
	submitter, submitterAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "AbsenceNote", "Submitter")
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID, group.ID, student.ID, submitter.ID)
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	const note = "Familienfeier, kommt später"
	require.NoError(t, tenant.WithTenantTx(context.Background(), tc.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, e := tc.services.ExcusedRequests.CreateRequest(
			txCtx, student.ID, submitterAccount.ID,
			[]timezone.Date{timezone.TodayDate()}, note,
		)
		return e
	}))

	rr := authExec(t, tc,
		testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil),
		testutil.TeacherTestClaims(int(account.ID)),
		[]string{"users:read", "users:absence"},
	)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var resp struct {
		Data struct {
			PendingExcusedNote *string `json:"pending_excused_note"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.NotNil(t, resp.Data.PendingExcusedNote, "an absence reviewer must receive the pending excused note")
	assert.Equal(t, note, *resp.Data.PendingExcusedNote)
}
