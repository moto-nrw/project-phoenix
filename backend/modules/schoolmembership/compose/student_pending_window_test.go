package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A phase edit moves the window of a still pending child through
// MovePendingWindow (#6505). A bound moves only while it still equals the
// phase's old bound; a child that is no longer pending stays as it is, and a
// window that would end before it starts is refused before any write. The
// first-care bound follows a start that lies ahead.
func TestMovePendingWindowMovesOnlyThePhaseBounds(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}, Employment: testEmployment, ChildQuota: newChildQuotas(),
		Today: func() calendar.Date { return "2099-12-07" }})
	require.NoError(t, err)
	enroll := func(name, status, from, until string) int64 {
		t.Helper()
		student := testpkg.CreateTestStudent(t, db, "Fenster", name, "1a")
		_, err := db.ExecContext(ctx, "DELETE FROM users.student_school_memberships WHERE tenant_id = ? AND student_profile_id = ?", testpkg.Tenant(t), student.ID)
		require.NoError(t, err)
		_, err = module.Enroll(ctx, schoolmembership.StudentEnrollment{StudentID: student.ID, SchoolClass: "1a", Status: status, EnrolledFrom: from, EnrolledUntil: until})
		require.NoError(t, err)
		return student.ID
	}
	window := func(id int64) (string, string, string, string) {
		t.Helper()
		var row struct {
			Status    string         `bun:"status"`
			From      *calendar.Date `bun:"enrolled_from"`
			Until     *calendar.Date `bun:"enrolled_until"`
			FirstCare *calendar.Date `bun:"first_care_from"`
		}
		require.NoError(t, db.NewRaw(`SELECT status, enrolled_from, enrolled_until, first_care_from FROM users.student_school_memberships
			WHERE tenant_id = ? AND student_profile_id = ? AND deleted_at IS NULL`, testpkg.Tenant(t), id).Scan(ctx, &row))
		text := func(day *calendar.Date) string {
			if day == nil {
				return ""
			}
			return day.String()
		}
		return row.Status, text(row.From), text(row.Until), text(row.FirstCare)
	}
	move := func(id int64, from, until string) schoolmembership.PendingWindowMove {
		return schoolmembership.PendingWindowMove{StudentID: id,
			FromBefore: "2100-08-01", FromAfter: from, UntilBefore: "2101-07-31", UntilAfter: until}
	}

	both := enroll("Beide", "pending", "2100-08-01", "2101-07-31")
	moved, err := module.MovePendingWindow(ctx, move(both, "2100-08-08", "2101-07-24"))
	require.NoError(t, err)
	assert.True(t, moved)
	status, from, until, firstCare := window(both)
	assert.Equal(t, []string{"pending", "2100-08-08", "2101-07-24", "2100-08-08"}, []string{status, from, until, firstCare},
		"both phase bounds and the first-care bound move")

	manual := enroll("Hand", "pending", "2100-09-15", "2101-07-31")
	moved, err = module.MovePendingWindow(ctx, move(manual, "2100-08-08", "2101-07-24"))
	require.NoError(t, err)
	assert.True(t, moved, "the child is still pending")
	_, from, until, _ = window(manual)
	assert.Equal(t, []string{"2100-09-15", "2101-07-24"}, []string{from, until}, "a start set by hand stays, the phase end moves")

	active := enroll("Aktiv", "active", "2100-08-01", "2101-07-31")
	moved, err = module.MovePendingWindow(ctx, move(active, "2100-08-08", "2101-07-24"))
	require.NoError(t, err)
	assert.False(t, moved, "an active child is not moved")
	status, from, until, _ = window(active)
	assert.Equal(t, []string{"active", "2100-08-01", "2101-07-31"}, []string{status, from, until})

	short := enroll("Kurz", "pending", "2100-08-01", "2100-08-05")
	_, err = module.MovePendingWindow(ctx, move(short, "2100-08-08", "2101-07-24"))
	var invalid *schoolmembership.InvalidMembershipError
	require.ErrorAs(t, err, &invalid, "a window ending before its start is refused")
	_, from, until, _ = window(short)
	assert.Equal(t, []string{"2100-08-01", "2100-08-05"}, []string{from, until}, "nothing was written")
}
