package studentdeletion_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRetention composes the automatic deletion (#3962) over the fixture's
// production workflow, on a fixed day and window, with each child in a
// savepoint like the root composes it.
func (f *deletionFixture) newRetention(t *testing.T, today string, days int) *studentdeletion.Retention {
	t.Helper()
	deps := f.deps
	deps.Today = func() string { return today }
	workflow, err := studentdeletion.New(deps)
	require.NoError(t, err)
	retention, err := studentdeletion.NewRetention(studentdeletion.RetentionDependencies{
		Workflow: workflow,
		Window:   func(context.Context) (int, error) { return days, nil },
		Tenant:   tenant.FromContext,
		Isolate: func(ctx context.Context, fn func(context.Context) error) error {
			err := tenant.WithSavepoint(ctx, fn)
			if errors.Is(err, tenant.ErrSavepointControl) {
				return errors.Join(studentdeletion.ErrIsolationFailed, err)
			}
			return err
		},
	})
	require.NoError(t, err)
	return retention
}

func (f *deletionFixture) runRetention(t *testing.T, retention *studentdeletion.Retention) studentdeletion.RetentionResult {
	t.Helper()
	var result studentdeletion.RetentionResult
	require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), f.db, testpkg.Tenant(t), func(ctx context.Context) error {
		var err error
		result, err = retention.DeleteExpiredStudents(ctx)
		return err
	}))
	return result
}

// A child the deletion refuses rolls back alone: the other expired children
// of the school are still deleted, and the refused one is tried again on the
// next run.
func TestStudentRetention_SkipsARefusedChildAndDeletesTheRest(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newDeletionFixture(t, db)
	lastCareDay := calendar.NewDate(2026, 5, 29)

	refused := testpkg.CreateTestStudent(t, db, "Retention", "Linked", "3a")
	deleted := testpkg.CreateTestStudent(t, db, "Retention", "Alone", "3a")
	companion := testpkg.CreateTestStudent(t, db, "Retention", "Companion", "3a")
	for _, id := range []int64{refused.ID, deleted.ID} {
		testpkg.SetStudentLifecycle(t, db, id, "inactive", nil, &lastCareDay)
	}
	giveAccompaniedPlan(t, db, ctx, refused.ID, "mon")
	giveAccompaniedPlan(t, db, ctx, companion.ID, "mon")
	linkCompanions(t, ctx, db, refused.ID, companion.ID)
	clearCompanionNote(t, db, companion.ID)

	// 2027-05-31 lies 367 days after the last care day.
	retention := f.newRetention(t, "2027-05-31", 365)
	result := f.runRetention(t, retention)

	assert.Equal(t, studentdeletion.RetentionResult{Deleted: 1, Refused: 1, RetentionDays: 365}, result)
	assert.Zero(t, rowCount(t, db, "users.student_profiles", deleted.ID))
	assert.Equal(t, 1, rowCount(t, db, "users.student_profiles", refused.ID), "the linked child would lose its way home")
	links, err := companionLinks(db).ListLinksForStudents(ctx, []int64{companion.ID})
	require.NoError(t, err)
	assert.Len(t, links[companion.ID], 1, "the refused deletion left the companion link untouched")

	giveAccompaniedPlan(t, db, ctx, companion.ID, "mon")
	result = f.runRetention(t, retention)

	assert.Equal(t, studentdeletion.RetentionResult{Deleted: 1, RetentionDays: 365}, result, "the next run deletes the child once nothing is stranded")
	assert.Zero(t, rowCount(t, db, "users.student_profiles", refused.ID))
}

// An owner failure is not a refusal: the savepoint rolls its child back and
// the failed cleanup run reaches the scheduler, which can retry it.
func TestStudentRetention_AbortsOnAnUnexpectedDeletionFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	f := newDeletionFixture(t, db)
	student := testpkg.CreateTestStudent(t, db, "Retention", "Audit", "3a")
	lastCareDay := calendar.NewDate(2026, 5, 29)
	testpkg.SetStudentLifecycle(t, db, student.ID, "inactive", nil, &lastCareDay)

	failure := errors.New("injected audit failure")
	f.deps.AppendAudit = func(context.Context, studentdeletion.Actor, studentdeletion.Result) error { return failure }
	retention := f.newRetention(t, "2027-05-31", 365)
	err := testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context) error {
		_, err := retention.DeleteExpiredStudents(ctx)
		return err
	})

	require.ErrorIs(t, err, failure)
	assert.Equal(t, 1, rowCount(t, db, "users.student_profiles", student.ID), "the savepoint rolls back the failed deletion")
}

// The retention refuses a window outside the frame of ADR 0048 before it
// reads a child.
func TestStudentRetention_RefusesAWindowOutsideTheFrame(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	f := newDeletionFixture(t, db)
	for _, days := range []int{studentdeletion.MinRetentionDays - 1, studentdeletion.MaxRetentionDays + 1} {
		retention := f.newRetention(t, "2027-05-31", days)
		err := testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context) error {
			_, err := retention.DeleteExpiredStudents(ctx)
			return err
		})
		require.ErrorContains(t, err, "outside 30 to 1095")
	}
}
