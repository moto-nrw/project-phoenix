package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// School Membership keeps the first-care bound of a child (#6041, ADR 0165):
// the earliest day it can have been cared for at this school. A child
// inserted pending with a start and no earlier membership gets that start;
// every write keeps the bound on the application's day. Care Plan holds the
// child back on every day before it, also after the activation.
func TestStudentMembershipKeepsTheFirstCareBound(t *testing.T) {
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
	bound := func(studentID int64) string {
		t.Helper()
		var value *calendar.Date
		require.NoError(t, db.NewRaw(`SELECT first_care_from FROM users.student_school_memberships
			WHERE tenant_id = ? AND student_profile_id = ? AND deleted_at IS NULL`, testpkg.Tenant(t), studentID).Scan(ctx, &value))
		if value == nil {
			return ""
		}
		return value.String()
	}
	const december, start = calendar.Date("2099-12-07"), "2100-02-03"
	enroll := func(name, status, from string) int64 {
		t.Helper()
		student := testpkg.CreateTestStudent(t, db, "Erste", name, "1a")
		_, err := db.ExecContext(ctx, "DELETE FROM users.student_school_memberships WHERE tenant_id = ? AND student_profile_id = ?", testpkg.Tenant(t), student.ID)
		require.NoError(t, err)
		_, err = on(december).Enroll(ctx, schoolmembership.StudentEnrollment{StudentID: student.ID, SchoolClass: "1a", Status: status, EnrolledFrom: from, EnrolledUntil: "2101-07-31"})
		require.NoError(t, err)
		return student.ID
	}
	renew := func(day calendar.Date, id int64, status, from string) {
		t.Helper()
		_, err := on(day).RenewEnrollment(ctx, schoolmembership.StudentEnrollment{StudentID: id, SchoolClass: "1a", Status: status, EnrolledFrom: from, EnrolledUntil: "2101-07-31"})
		require.NoError(t, err)
	}
	activate := func(day calendar.Date, id int64) {
		t.Helper()
		moved, err := on(day).TransitionStatus(ctx, id, "pending", "active")
		require.NoError(t, err)
		require.True(t, moved)
	}

	scheduled := enroll("Geplant", "pending", start)
	assert.Equal(t, start, bound(scheduled), "a scheduled approval inserts a new child: no care before its start")
	assert.Empty(t, bound(enroll("Sofort", "active", start)), "an immediately activated child may take part before its start")
	assert.Empty(t, bound(enroll("Ohne", "pending", "")), "a pending child without a start takes part")

	renew(december, scheduled, "pending", "2100-03-01")
	assert.Equal(t, "2100-03-01", bound(scheduled), "a start that lies ahead moves with its bound")
	renew(december, scheduled, "pending", start)
	assert.Equal(t, start, bound(scheduled), "and back")
	renew(december, scheduled, "pending", start)
	assert.Equal(t, start, bound(scheduled), "saving the child unchanged keeps the bound")

	activate("2100-02-03", scheduled)
	assert.Equal(t, start, bound(scheduled), "the activation on the start day keeps the bound")
	_, err := on("2100-06-30").EndCare(ctx, []int64{scheduled}, "2100-06-30")
	require.NoError(t, err)
	resumed, err := on("2100-07-01").ResumeCare(ctx, scheduled, "2101-02-01", "pending", "2100-07-01")
	require.NoError(t, err)
	require.True(t, resumed)
	assert.Equal(t, start, bound(scheduled), "a resumption keeps the earlier care: the bound only moves earlier")

	late := enroll("Spaet", "pending", start)
	activate("2100-02-05", late)
	assert.Equal(t, start, bound(late), "a late activation run keeps the bound")

	early := enroll("Frueh", "pending", start)
	activate("2100-01-12", early)
	assert.Equal(t, "2100-01-12", bound(early), "an activation before the start starts care that day")

	reached := enroll("Erreicht", "pending", "2099-09-01")
	renew(december, reached, "pending", "2100-09-01")
	assert.Equal(t, "2099-09-01", bound(reached), "a reached bound may have seen care: a later start keeps it")
	renew(december, reached, "pending", "2099-08-15")
	assert.Equal(t, "2099-08-15", bound(reached), "an earlier start moves it earlier")

	returning := enroll("Zurueck", "active", "2098-08-01")
	renew(december, returning, "pending", "2100-08-01")
	assert.Empty(t, bound(returning), "a renewal of a child cared for stays unknown")

	again := testpkg.CreateTestStudent(t, db, "Erste", "Wieder", "1a")
	_, err = db.ExecContext(ctx, "UPDATE users.student_school_memberships SET deleted_at = NOW() WHERE tenant_id = ? AND student_profile_id = ?", testpkg.Tenant(t), again.ID)
	require.NoError(t, err)
	_, err = on(december).Enroll(ctx, schoolmembership.StudentEnrollment{StudentID: again.ID, SchoolClass: "1a", Status: "pending", EnrolledFrom: start})
	require.NoError(t, err)
	assert.Empty(t, bound(again.ID), "an earlier membership may have had care")
}
