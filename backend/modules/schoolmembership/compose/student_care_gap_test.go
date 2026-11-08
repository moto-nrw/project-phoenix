package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// School Membership keeps the care gap of a returning child (#6073, ADR 0177):
// a resumption or a renewal that restarts an ended care with a later start
// records the last care day before it and the day care resumes. The end of
// the gap moves like the first-care bound; saving the child again keeps it.
func TestStudentMembershipKeepsTheCareGapOfAReturningChild(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	on := func(day calendar.Date) schoolmembership.Owner {
		t.Helper()
		module, err := New(Dependencies{DB: db, Observe: func(Observation) {}, Employment: testEmployment, ChildQuota: newChildQuotas(),
			Today: func() calendar.Date { return day }})
		require.NoError(t, err)
		return module
	}
	gap := func(studentID int64) [2]string {
		t.Helper()
		var after, resumed *calendar.Date
		require.NoError(t, db.NewRaw(`SELECT care_paused_after, care_resumed_from FROM users.student_school_memberships
			WHERE tenant_id = ? AND student_profile_id = ? AND deleted_at IS NULL`, testpkg.Tenant(t), studentID).Scan(ctx, &after, &resumed))
		var result [2]string
		if after != nil {
			result[0] = after.String()
		}
		if resumed != nil {
			result[1] = resumed.String()
		}
		return result
	}
	const ended, restartDay, february = "2099-10-31", calendar.Date("2099-11-07"), "2100-02-01"
	endedChild := func(name string) int64 {
		t.Helper()
		student := testpkg.CreateTestStudent(t, db, "Luecke", name, "1a")
		_, err := on("2099-10-20").EndCare(ctx, []int64{student.ID}, ended)
		require.NoError(t, err)
		require.Equal(t, [2]string{}, gap(student.ID), "an end alone is no gap")
		return student.ID
	}
	renew := func(day calendar.Date, id int64, status, from, until string) {
		t.Helper()
		_, err := on(day).RenewEnrollment(ctx, schoolmembership.StudentEnrollment{StudentID: id, SchoolClass: "1a", Status: status, EnrolledFrom: from, EnrolledUntil: until})
		require.NoError(t, err)
	}

	resumed := endedChild("Wiederaufnahme")
	moved, err := on(restartDay).ResumeCare(ctx, resumed, february, "pending", restartDay.String())
	require.NoError(t, err)
	require.True(t, moved)
	assert.Equal(t, [2]string{ended, february}, gap(resumed), "the resumption keeps the earlier end and the new start")
	renew(restartDay, resumed, "pending", "2100-03-01", "")
	assert.Equal(t, [2]string{ended, "2100-03-01"}, gap(resumed), "a start that lies ahead moves the end of the gap")
	renew(restartDay, resumed, "pending", february, "")
	moved, err = on("2100-02-01").TransitionStatus(ctx, resumed, "pending", "active")
	require.NoError(t, err)
	require.True(t, moved)
	assert.Equal(t, [2]string{ended, february}, gap(resumed), "the activation on the start day keeps the gap")
	renew("2100-03-10", resumed, "active", february, "")
	assert.Equal(t, [2]string{ended, february}, gap(resumed), "saving the child again keeps the gap")

	early := endedChild("Frueh")
	renew(restartDay, early, "pending", february, "")
	assert.Equal(t, [2]string{ended, february}, gap(early), "a renewal of an ended child restarts care with a gap")
	moved, err = on("2099-12-15").TransitionStatus(ctx, early, "pending", "active")
	require.NoError(t, err)
	require.True(t, moved)
	assert.Equal(t, [2]string{ended, "2099-12-15"}, gap(early), "an activation before the start ends the gap that day")

	activeNow := endedChild("Sofort")
	renew(restartDay, activeNow, "active", february, "")
	assert.Equal(t, [2]string{ended, restartDay.String()}, gap(activeNow), "an immediate activation resumes care today")

	continuing := endedChild("Anschluss")
	renew(restartDay, continuing, "active", "2099-11-01", "")
	assert.Equal(t, [2]string{}, gap(continuing), "a start right after the end leaves no gap")

	running := testpkg.CreateTestStudent(t, db, "Luecke", "Laufend", "1a")
	renew(restartDay, running.ID, "pending", february, "")
	assert.Equal(t, [2]string{}, gap(running.ID), "a child without an end has no gap")
}
