package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// School Membership records whether a membership still waits for its first
// care (#6041 review 2): only a membership inserted pending with a start, that
// stayed pending and whose start was not moved past a reached day. Care Plan
// holds such a child back on every day before its start; every other child
// keeps its past days, because its earlier care is not recorded elsewhere.
func TestStudentMembershipRecordsWhetherCareNeverStarted(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := buildModule(t, db)
	awaits := func(studentID int64) bool {
		t.Helper()
		var value bool
		require.NoError(t, db.NewRaw(`SELECT awaits_first_care FROM users.student_school_memberships
			WHERE tenant_id = ? AND student_profile_id = ? AND deleted_at IS NULL`, testpkg.Tenant(t), studentID).Scan(ctx, &value))
		return value
	}
	enroll := func(name, status, from string) int64 {
		t.Helper()
		student := testpkg.CreateTestStudent(t, db, "Erste", name, "1a")
		_, err := db.ExecContext(ctx, "DELETE FROM users.student_school_memberships WHERE tenant_id = ? AND student_profile_id = ?", testpkg.Tenant(t), student.ID)
		require.NoError(t, err)
		_, err = module.Enroll(ctx, schoolmembership.StudentEnrollment{StudentID: student.ID, SchoolClass: "1a", Status: status, EnrolledFrom: from, EnrolledUntil: "2100-07-31"})
		require.NoError(t, err)
		return student.ID
	}

	scheduled := enroll("Geplant", "pending", "2099-02-03")
	assert.True(t, awaits(scheduled), "a scheduled approval inserts a child that never had care")
	assert.False(t, awaits(enroll("Sofort", "active", "2099-02-03")), "an immediately activated child may take part before its start")
	assert.False(t, awaits(enroll("Ohne", "pending", "")), "a pending child without a start takes part")

	// The school corrects the start before it is reached: still no care.
	_, err := module.RenewEnrollment(ctx, schoolmembership.StudentEnrollment{StudentID: scheduled, SchoolClass: "1b", Status: "pending", EnrolledFrom: "2099-03-01", EnrolledUntil: "2100-07-31"})
	require.NoError(t, err)
	assert.True(t, awaits(scheduled), "moving a start that lies ahead keeps the fact")

	// Activation on the start day, then end and resumption: the child had care.
	_, err = module.TransitionStatus(ctx, scheduled, "pending", "active")
	require.NoError(t, err)
	assert.False(t, awaits(scheduled), "once active the child may have had care")
	_, err = module.EndCare(ctx, []int64{scheduled}, "2099-06-30")
	require.NoError(t, err)
	resumed, err := module.ResumeCare(ctx, scheduled, "2100-02-01", "pending", "2099-07-01")
	require.NoError(t, err)
	require.True(t, resumed)
	assert.False(t, awaits(scheduled), "a resumption is a returning child, never a new one")

	// A renewal of a child the school cared for, to a later start.
	returning := enroll("Zurueck", "active", "2098-08-01")
	_, err = module.RenewEnrollment(ctx, schoolmembership.StudentEnrollment{StudentID: returning, SchoolClass: "2a", Status: "pending", EnrolledFrom: "2099-08-01", EnrolledUntil: "2100-07-31"})
	require.NoError(t, err)
	assert.False(t, awaits(returning), "a renewal keeps the earlier care of a returning child")

	// A start already reached that moves later may have had care in between.
	reached := enroll("Erreicht", "pending", "2020-09-01")
	require.True(t, awaits(reached))
	_, err = module.RenewEnrollment(ctx, schoolmembership.StudentEnrollment{StudentID: reached, SchoolClass: "1a", Status: "pending", EnrolledFrom: "2099-09-01", EnrolledUntil: "2100-07-31"})
	require.NoError(t, err)
	assert.False(t, awaits(reached))

	// A second membership of the same child is no new child.
	again := testpkg.CreateTestStudent(t, db, "Erste", "Wieder", "1a")
	_, err = db.ExecContext(ctx, "UPDATE users.student_school_memberships SET deleted_at = NOW() WHERE tenant_id = ? AND student_profile_id = ?", testpkg.Tenant(t), again.ID)
	require.NoError(t, err)
	_, err = module.Enroll(ctx, schoolmembership.StudentEnrollment{StudentID: again.ID, SchoolClass: "1a", Status: "pending", EnrolledFrom: "2099-02-03"})
	require.NoError(t, err)
	assert.False(t, awaits(again.ID), "an earlier membership may have had care")
}
