package studentdeletion_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
	"github.com/stretchr/testify/require"
)

type lateDeletionCompanion struct {
	studentdeletion.CarePlan
	afterFirstRead func()
	reads          int
}

func (c *lateDeletionCompanion) ListCompanionLinks(ctx context.Context, ids []int64) (map[int64][]careplan.CompanionLink, error) {
	links, err := c.CarePlan.ListCompanionLinks(ctx, ids)
	c.reads++
	if err == nil && c.reads == 1 {
		c.afterFirstRead()
	}
	return links, err
}

// The first graph snapshot does not contain an edge committed before the
// subject lock. Its newly discovered far end must still use the old ordered
// lock protocol, even when the subject snapshot is passed to later steps.
func TestStudentDeletionWorkflow_LateCompanionLockWindows(t *testing.T) {
	t.Parallel()
	for _, lower := range []bool{true, false} {
		name := "higher_waits"
		if lower {
			name = "lower_nowait"
		}
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			ctx := testpkg.Ctx(t)
			f := newDeletionFixture(t, db)
			subject := testpkg.CreateTestStudent(t, db, "Late", "Subject", "1a")
			companion := testpkg.CreateTestStudent(t, db, "Late", "Companion", "1a")
			if lower {
				subject, companion = companion, subject
			}
			preview, err := f.workflow(t).Preview(ctx, subject.ID)
			require.NoError(t, err)
			links := &lateDeletionCompanion{CarePlan: f.deps.CarePlan, afterFirstRead: func() {
				linkCompanions(t, ctx, db, subject.ID, companion.ID)
				testpkg.HoldStudentRowLock(t, db, companion.ID)
			}}
			f.deps.CarePlan = links
			lockCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
			defer cancel()
			_, err = f.workflow(t).Execute(lockCtx, subject.ID, confirm(preview, studentdeletion.ReasonTestData))
			require.Error(t, err)
			if lower {
				require.ErrorIs(t, err, studentdeletion.ErrCompanionLockBusy)
			} else {
				require.NotErrorIs(t, err, studentdeletion.ErrCompanionLockBusy)
			}
			require.Equal(t, 2, links.reads, "lock failure precedes the stranding and count reads")
			require.Equal(t, 1, rowCount(t, db, "users.student_profiles", subject.ID))
			require.Equal(t, 1, rowCount(t, db, "users.student_profiles", companion.ID))
		})
	}
}

// Nonpositive IDs are dropped by the graph lock pass. The later subject read
// must retain the directory validation error, not validate a zero person ID.
func TestStudentDeletionWorkflow_InvalidSubjectErrorPrecedence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newDeletionFixture(t, db)
	for _, id := range []int64{0, -1} {
		_, expected := f.deps.Directory.ReadEnrollmentStudent(ctx, id, "update")
		require.Error(t, expected)
		_, err := f.workflow(t).Execute(ctx, id, studentdeletion.Confirmation{
			ExpectedFingerprint: "invalid-subject", ConfirmationName: "Invalid Subject",
			Reason: studentdeletion.ReasonTestData, Acknowledged: true,
		})
		require.EqualError(t, err, expected.Error())
		require.IsType(t, expected, err)
	}
}
