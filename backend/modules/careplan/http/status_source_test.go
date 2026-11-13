package careplanhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An edit of other fields of a child the guardians reported sick leaves the
// day's report as it is: the status day keeps the parents as its source and
// its report time (#6291). Before, every update while the flag was set
// rewrote today's day as the OGS's own manual entry.
func TestUpdateStudent_KeepsTheSourceOfAStandingStatus(t *testing.T) {
	t.Parallel()

	date := calendar.NewDate(2026, 11, 10)
	now := date.BerlinMidnight().Add(11 * time.Hour)
	reported := date.BerlinMidnight().Add(7 * time.Hour)
	tc := setupStudentsRoute(t, func() time.Time { return now })
	student := testpkg.CreateTestStudent(t, tc.db, "Paula", "Elternmeldung", "PE1")
	_, err := tc.db.ExecContext(testpkg.Ctx(t), `INSERT INTO active.student_status_days (tenant_id, student_id, date, status, reported_at, source)
		VALUES (?, ?, ?, 'sick', ?, 'parent')`, testpkg.Tenant(t), student.ID, date, reported)
	require.NoError(t, err)
	_, err = tc.db.ExecContext(testpkg.Ctx(t), `UPDATE users.student_care_profiles SET sick = true, sick_since = ?
		WHERE membership_id IN (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ?)`, reported, student.ID)
	require.NoError(t, err)

	rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
		map[string]any{"last_name": "Umbenannt"}), testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var day struct {
		Source     string    `bun:"source"`
		ReportedAt time.Time `bun:"reported_at"`
		Cleared    bool      `bun:"cleared"`
	}
	require.NoError(t, tc.db.NewRaw(`SELECT source, reported_at, cleared_at IS NOT NULL AS cleared FROM active.student_status_days
		WHERE student_id = ? AND date = ? AND status = 'sick'`, student.ID, date).Scan(testpkg.Ctx(t), &day))
	assert.Equal(t, "parent", day.Source)
	assert.True(t, reported.Equal(day.ReportedAt), "report time %s", day.ReportedAt)
	assert.False(t, day.Cleared)
}

// The profile displays end_of_day rows after their live flag was cleared.
// Explicit false must withdraw that same row; unrelated edits must keep it.
func TestUpdateStudent_WithdrawsAnArchivedStatus(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"sick", "excused"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			date := calendar.NewDate(2026, 11, 10)
			now := date.BerlinMidnight().Add(19 * time.Hour)
			tc := setupStudentsRoute(t, func() time.Time { return now })
			student := testpkg.CreateTestStudent(t, tc.db, "Paula", "Archiviert", "AR1")
			var statusDayID int64
			err := tc.db.NewRaw(`INSERT INTO active.student_status_days
				(tenant_id, student_id, date, status, reported_at, cleared_at, source)
				VALUES (?, ?, ?, ?, ?, ?, 'end_of_day') RETURNING id`,
				testpkg.Tenant(t), student.ID, date, status,
				date.BerlinMidnight().Add(7*time.Hour), date.BerlinMidnight().Add(18*time.Hour)).Scan(testpkg.Ctx(t), &statusDayID)
			require.NoError(t, err)
			// Older archives also left the day's absence on timetable slots.
			room := testpkg.CreateTestRoom(t, tc.db, "Archiv")
			instance := testpkg.CreateTestActivityInstance(t, tc.db, date, room.ID, testpkg.ActivityInstanceOpts{})
			slot := testpkg.CreateTestInstanceStudent(t, tc.db, instance.ID, student.ID,
				testpkg.ScheduleAttendanceStatusAbsent, testpkg.InstanceStudentOpts{StudentStatusDayID: &statusDayID})
			path := fmt.Sprintf("/%d", student.ID)
			claims := testutil.AdminTestClaims(1)
			perms := []string{"admin:*"}
			assertDisplayed := func(want bool) {
				t.Helper()
				rr := authExec(t, tc, testutil.NewRequest("GET", path, nil), claims, perms)
				require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
				var response struct {
					Data map[string]json.RawMessage `json:"data"`
				}
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
				var displayed bool
				require.NoError(t, json.Unmarshal(response.Data[status], &displayed))
				assert.Equal(t, want, displayed, "displayed %s", status)
			}
			assertDisplayed(true)
			rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", path,
				map[string]any{"last_name": "Umbenannt"}), claims, perms)
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			assertDisplayed(true)

			rr = authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", path,
				map[string]any{status: false}), claims, perms)
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			assertDisplayed(false)
			released := testpkg.InstanceStudentByIDContext(t, testpkg.Ctx(t), tc.db, slot.ID)
			assert.Equal(t, testpkg.ScheduleAttendanceStatusExpected, released.Status)
			assert.Nil(t, released.StudentStatusDayID)
			clearedAt := now
			now = now.Add(time.Minute)
			rr = authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", path,
				map[string]any{status: false}), claims, perms)
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			var row struct {
				Source        string    `bun:"source"`
				ClearedSource string    `bun:"cleared_source"`
				ClearedAt     time.Time `bun:"cleared_at"`
			}
			require.NoError(t, tc.db.NewRaw(`SELECT source, cleared_source, cleared_at FROM active.student_status_days
				WHERE student_id = ? AND date = ? AND status = ?`, student.ID, date, status).Scan(testpkg.Ctx(t), &row))
			assert.Equal(t, "end_of_day", row.Source, "the withdrawal keeps the archive's source (#6842)")
			assert.Equal(t, "manual", row.ClearedSource)
			assert.True(t, clearedAt.Equal(row.ClearedAt), "a repeated withdrawal keeps the first clear time")
		})
	}
}
