package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStudentMembershipEnrollmentUsesProfileIdentityAndTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Enrollment", "Identity", "1a")
	module := buildModule(t, db)
	_, err := db.ExecContext(ctx, "UPDATE users.student_school_memberships SET deleted_at = NOW() WHERE tenant_id = ? AND student_profile_id = ?", testpkg.Tenant(t), student.ID)
	require.NoError(t, err)
	input := schoolmembership.StudentEnrollment{StudentID: student.ID, SchoolClass: "2a", Status: "active", EnrolledFrom: "2030-08-01"}
	membershipID, err := module.Enroll(ctx, input)
	require.NoError(t, err)
	require.Positive(t, membershipID)
	foreignCtx, _ := otherTenantContext(t, db)
	changed, err := module.ChangeClass(foreignCtx, []int64{student.ID}, "2a", "3a")
	require.NoError(t, err)
	require.Zero(t, changed)
	renewedID, err := module.RenewEnrollment(foreignCtx, input)
	require.NoError(t, err)
	require.Zero(t, renewedID)
	input.SchoolClass = "3a"
	renewedID, err = module.RenewEnrollment(ctx, input)
	require.NoError(t, err)
	require.Equal(t, membershipID, renewedID)
	_, err = module.Graduate(ctx, []int64{student.ID})
	require.NoError(t, err)
	renewedID, err = module.RenewEnrollment(ctx, input)
	require.NoError(t, err)
	require.Zero(t, renewedID, "renewal cannot implicitly reactivate an alumnus")
	assigned, err := module.AssignGroup(ctx, student.ID, nil)
	require.NoError(t, err)
	require.True(t, assigned, "an unchanged group is an idempotent no-op, not reactivation")
}

func TestStudentMembershipResumeUsesFrozenDayAndRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Care", "FrozenDay", "1a")
	module := buildModule(t, db)
	changed, err := module.EndCare(ctx, []int64{student.ID}, "2035-07-31")
	require.NoError(t, err)
	require.EqualValues(t, 1, changed)
	resumed, err := module.ResumeCare(ctx, student.ID, "2035-08-01", "active", "2035-07-31")
	require.NoError(t, err)
	require.False(t, resumed, "care includes its last day")
	failure := errors.New("care ledger failed")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		resumed, resumeErr := module.ResumeCare(txCtx, student.ID, "2035-08-01", "active", "2035-08-01")
		require.NoError(t, resumeErr)
		require.True(t, resumed)
		return failure
	})
	require.ErrorIs(t, err, failure)
	resumed, err = module.ResumeCare(ctx, student.ID, "2035-08-01", "active", "2035-08-01")
	require.NoError(t, err)
	require.True(t, resumed, "rollback must retain the ended interval")
	resumed, err = module.ResumeCare(ctx, student.ID, "2035-08-01", "active", "2035-08-01")
	require.NoError(t, err)
	require.False(t, resumed, "an open interval cannot be resumed again")
}

func TestStudentMembershipClassChangeRollsBackWithCaller(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Membership", "Rollback", "1a")
	membership, err := New(Dependencies{DB: db, Observe: func(Observation) {}, Employment: testEmployment})
	require.NoError(t, err)
	failure := errors.New("later owner failed")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		changed, changeErr := membership.ChangeClass(txCtx, []int64{student.ID}, "1a", "2a")
		require.NoError(t, changeErr)
		require.EqualValues(t, 1, changed)
		return failure
	})
	require.ErrorIs(t, err, failure)
	changed, err := membership.ChangeClass(ctx, []int64{student.ID}, "1a", "2a")
	require.NoError(t, err)
	require.EqualValues(t, 1, changed, "retry must still find the pre-transaction class")
	changed, err = membership.ChangeClass(ctx, []int64{student.ID}, "1a", "2a")
	require.NoError(t, err)
	require.Zero(t, changed, "a repeated transition must not change another row")
}

func TestStudentMembershipAlumniNeedExplicitReactivation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Alumni", "Guard", "4a")
	membership, err := New(Dependencies{DB: db, Observe: func(Observation) {}, Employment: testEmployment})
	require.NoError(t, err)
	changed, err := membership.Graduate(ctx, []int64{student.ID})
	require.NoError(t, err)
	require.EqualValues(t, 1, changed)
	changed, err = membership.ChangeClass(ctx, []int64{student.ID}, "4a", "4b")
	require.NoError(t, err)
	require.Zero(t, changed, "a class edit must not mutate an alumnus")
	changed, err = membership.EndCare(ctx, []int64{student.ID}, "2026-07-31")
	require.NoError(t, err)
	require.Zero(t, changed, "care exit must not mutate an alumnus")
	resumed, err := membership.ResumeCare(ctx, student.ID, "2026-09-01", "active", "2026-09-19")
	require.NoError(t, err)
	require.False(t, resumed, "care resume must not reactivate an alumnus")
	ids, err := membership.Reactivate(ctx, []int64{student.ID}, "active")
	require.NoError(t, err)
	require.Equal(t, []int64{student.ID}, ids)
	changed, err = membership.ChangeClass(ctx, []int64{student.ID}, "4a", "4b")
	require.NoError(t, err)
	require.EqualValues(t, 1, changed)
}
