package studentdeletion_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/moto-ogs/project-phoenix/workflows/studentdeletion"
	"github.com/stretchr/testify/require"
)

type deletionCountWindows struct {
	studentdeletion.CarePlan
	calls  int
	failAt int
	fail   error
}

func (c *deletionCountWindows) CountCarePlanDeletionRecords(ctx context.Context, id int64) (careplan.CarePlanDeletionCounts, error) {
	c.calls++
	if c.calls == c.failAt {
		return careplan.CarePlanDeletionCounts{}, c.fail
	}
	return c.CarePlan.CountCarePlanDeletionRecords(ctx, id)
}

func TestStudentDeletionWorkflow_CarePlanCountWindowsAndFirstError(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	for _, failAt := range []int{0, 1, 2} {
		f := newDeletionFixture(t, db)
		student := testpkg.CreateTestStudent(t, db, "Deletion", "Count windows", "1a")
		preview, err := f.workflow(t).Preview(ctx, student.ID)
		require.NoError(t, err)
		failure := errors.New("care plan counter unavailable")
		windows := &deletionCountWindows{CarePlan: f.deps.CarePlan, failAt: failAt, fail: failure}
		f.deps.CarePlan = windows
		guardianCounts := 0
		// Guardian invitations follow Care Plan and guardian/companion links.
		// A Care Plan error must return before this later owner's counter.
		original := f.deps.CountGuardianInvitations
		f.deps.CountGuardianInvitations = func(ctx context.Context, id int64) (int, error) {
			guardianCounts++
			return original(ctx, id)
		}
		result, err := f.workflow(t).Execute(ctx, student.ID, confirm(preview, studentdeletion.ReasonTestData))
		if failAt == 0 {
			require.NoError(t, err)
			require.Equal(t, preview.Counts, result.Counts)
			require.Equal(t, 2, windows.calls, "fingerprint and pre-cascade counts remain separate owner reads")
			require.Equal(t, 2, guardianCounts)
			continue
		}
		require.ErrorIs(t, err, failure)
		require.Equal(t, failAt, windows.calls)
		require.Equal(t, failAt-1, guardianCounts, "no later owner runs after the failed count window")
		require.Equal(t, 1, rowCount(t, db, "users.student_profiles", student.ID), "counter failure prevents the cascade")
	}
}
