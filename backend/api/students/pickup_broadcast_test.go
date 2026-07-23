package students_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hasPickupScheduleBroadcast reports whether the recorder saw a
// pickup_schedule_changed event addressed to the given tenant.
//
// Deliberately reads CallsByMethod("tenant"), not "all": only the writing
// school can read the affected child, so a global fan-out would make every
// other school refetch student and care-plan data for an invisible write.
func hasPickupScheduleBroadcast(b *testpkg.RecordingBroadcaster, tenantID int64) bool {
	for _, c := range b.CallsByMethod("tenant") {
		if c.Event.Type == realtime.EventPickupScheduleChanged && c.TenantID == tenantID {
			return true
		}
	}
	return false
}

// hasGlobalPickupScheduleBroadcast reports whether any pickup_schedule_changed
// event was broadcast to EVERY connected staff client, in any tenant.
func hasGlobalPickupScheduleBroadcast(b *testpkg.RecordingBroadcaster) bool {
	for _, c := range b.CallsByMethod("all") {
		if c.Event.Type == realtime.EventPickupScheduleChanged {
			return true
		}
	}
	return false
}

// TestStaffPickupWrite_BroadcastsPickupScheduleChanged pins the staff-side half
// of a Gehzeit write. Before this event existed the pickup handlers woke only
// the child's guardians (#1725), so the parents app refreshed live while every
// staff view that renders the pickup time — the per-child Betreuungsplan, the
// detail header, the student lists — kept the previous value indefinitely:
// those caches disable focus revalidation, leaving SSE as their only live path.
//
// Every write path is asserted, because each one carries its own after-commit
// hook and a missed hook is invisible until a user reports a stale time. Each
// assertion also pins the SCOPE: the event must reach the writing school's
// clients and must not be a global fan-out, which would make every other school
// refetch student and care-plan data for a child it cannot read.
func TestStaffPickupWrite_BroadcastsPickupScheduleChanged(t *testing.T) {
	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "PickupCast", "Teacher")
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID)
	claims := testutil.AdminTestClaims(int(account.ID))

	t.Run("weekly_schedule_update", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "PickupCast", "Weekly", "PC1")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
		defer deletePickupSchedules(t, tc, student.ID)

		tc.broadcaster.Reset()
		body := map[string]any{
			"schedules": []map[string]any{{"weekday": 1, "pickup_time": "15:30"}},
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d/pickup-schedules", student.ID), body)
		rr := authExec(t, tc, req, claims, []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())

		assert.True(t, hasPickupScheduleBroadcast(tc.broadcaster, student.GetTenantID()),
			"a weekly Gehzeit update must broadcast pickup_schedule_changed to staff tabs")
		assert.False(t, hasGlobalPickupScheduleBroadcast(tc.broadcaster),
			"the event must stay scoped to the writing tenant, never fan out to every school")
	})

	t.Run("exception_create_update_delete", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "PickupCast", "Exception", "PC2")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
		defer deletePickupExceptions(t, tc, student.ID)

		// Create
		tc.broadcaster.Reset()
		body := map[string]any{
			"exception_date": "2026-05-12",
			"pickup_time":    "13:00",
			"reason":         "Arzttermin",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-exceptions", student.ID), body)
		rr := authExec(t, tc, req, claims, []string{"admin:*"})
		require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
		assert.True(t, hasPickupScheduleBroadcast(tc.broadcaster, student.GetTenantID()),
			"creating a pickup exception must broadcast pickup_schedule_changed")

		exceptionID := latestPickupExceptionID(t, tc, student.ID)

		// Update
		tc.broadcaster.Reset()
		body["pickup_time"] = "14:15"
		req = testutil.NewAuthenticatedRequest(t, "PUT",
			fmt.Sprintf("/%d/pickup-exceptions/%d", student.ID, exceptionID), body)
		rr = authExec(t, tc, req, claims, []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.True(t, hasPickupScheduleBroadcast(tc.broadcaster, student.GetTenantID()),
			"editing a pickup exception must broadcast pickup_schedule_changed")

		// Delete
		tc.broadcaster.Reset()
		req = testutil.NewRequest("DELETE",
			fmt.Sprintf("/%d/pickup-exceptions/%d", student.ID, exceptionID), nil)
		rr = authExec(t, tc, req, claims, []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.True(t, hasPickupScheduleBroadcast(tc.broadcaster, student.GetTenantID()),
			"deleting a pickup exception must broadcast pickup_schedule_changed")
		assert.False(t, hasGlobalPickupScheduleBroadcast(tc.broadcaster),
			"the event must stay scoped to the writing tenant, never fan out to every school")
	})

	// A day note travels in the same pickup payload as the times (the detail
	// header renders it next to the Gehzeit), so it goes stale the same way.
	t.Run("day_note_create", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "PickupCast", "Note", "PC3")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
		defer deletePickupNotes(t, tc, student.ID)

		tc.broadcaster.Reset()
		body := map[string]any{"note_date": "2026-05-12", "content": "Oma holt ab"}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/pickup-notes", student.ID), body)
		rr := authExec(t, tc, req, claims, []string{"admin:*"})
		require.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

		assert.True(t, hasPickupScheduleBroadcast(tc.broadcaster, student.GetTenantID()),
			"creating a pickup day note must broadcast pickup_schedule_changed")
	})
}

func deletePickupSchedules(t *testing.T, tc *testContext, studentID int64) {
	t.Helper()
	_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupSchedule)(nil)).
		ModelTableExpr("schedule.student_pickup_schedules").
		Where("student_id = ?", studentID).
		Exec(context.Background())
}

func deletePickupExceptions(t *testing.T, tc *testContext, studentID int64) {
	t.Helper()
	_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupException)(nil)).
		ModelTableExpr("schedule.student_pickup_exceptions").
		Where("student_id = ?", studentID).
		Exec(context.Background())
}

func deletePickupNotes(t *testing.T, tc *testContext, studentID int64) {
	t.Helper()
	_, _ = tc.db.NewDelete().Model((*scheduleModel.StudentPickupNote)(nil)).
		ModelTableExpr("schedule.student_pickup_notes").
		Where("student_id = ?", studentID).
		Exec(context.Background())
}

// latestPickupExceptionID returns the newest pickup exception of the student —
// the row the create step above wrote (fixtures never share a student).
func latestPickupExceptionID(t *testing.T, tc *testContext, studentID int64) int64 {
	t.Helper()
	exception := new(scheduleModel.StudentPickupException)
	err := tc.db.NewSelect().Model(exception).
		// Quoted alias: bun maps columns through the struct-derived alias, and
		// an unquoted one fails at runtime (root CLAUDE.md pattern 1).
		ModelTableExpr(`schedule.student_pickup_exceptions AS "student_pickup_exception"`).
		Where("student_id = ?", studentID).
		Order("id DESC").
		Limit(1).
		Scan(context.Background())
	require.NoError(t, err, "the created pickup exception must be readable")
	return exception.ID
}
