// Late-arrival guard for the apply cohort (#405 review).
//
// The row locks the apply takes bound what a concurrent writer can CHANGE, not
// what it can ADD. A child who lands in a mapped class after the cohort snapshot
// is never locked and never written, so without a guard the transition would be
// marked applied while that child stays behind in a class it just emptied — a
// half-done bulk change reported as a finished one, with no history row for the
// revert to undo.
package gradetransition

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lateArrivalDirectory answers the cohort snapshot first and, on the re-read
// under the locks, an extra child that committed in between — the deterministic
// stand-in for the concurrent writer the exclusive class-writes gate cannot
// exclude retroactively.
type lateArrivalDirectory struct {
	Directory
	snapshot []peopledirectory.Student
	late     peopledirectory.Student
	reads    int
	locked   []int64
}

func (d *lateArrivalDirectory) LockEnrollmentClassWritesExclusive(context.Context) error { return nil }

func (d *lateArrivalDirectory) ListStudentsByClasses(
	context.Context, []string,
) ([]peopledirectory.Student, error) {
	d.reads++
	if d.reads == 1 {
		return d.snapshot, nil
	}
	return append(append([]peopledirectory.Student{}, d.snapshot...), d.late), nil
}

func (d *lateArrivalDirectory) ReadEnrollmentStudent(
	_ context.Context, id int64, lock string,
) (peopledirectory.EnrollmentRecord, error) {
	if lock != "update" {
		return peopledirectory.EnrollmentRecord{}, assertLockError(lock)
	}
	d.locked = append(d.locked, id)
	for _, student := range d.snapshot {
		if student.ID == id {
			return peopledirectory.EnrollmentRecord{
				ID: student.ID, PersonID: student.PersonID,
				SchoolClass: student.SchoolClass, Status: student.Status,
			}, nil
		}
	}
	return peopledirectory.EnrollmentRecord{}, peopledirectory.ErrStudentNotFound
}

func assertLockError(lock string) error {
	return &peopledirectory.InvalidStudentError{Reason: "cohort rows must be locked FOR UPDATE, got " + lock}
}

// transitionStub serves one applicable draft and accepts the transition gate.
// Every other School Structure command is left nil on purpose: an apply that
// gets past the guard would panic instead of silently writing.
type transitionStub struct {
	Structure
	transition schoolstructure.Transition
}

func (s transitionStub) LockTransitions(context.Context) error { return nil }

func (s transitionStub) FindTransition(context.Context, int64) (schoolstructure.Transition, error) {
	return s.transition, nil
}

func TestApplyRefusesChildAddedAfterCohortSnapshot(t *testing.T) {
	t.Parallel()

	const transitionID = int64(8241)
	gradClass := "4late"
	present := peopledirectory.Student{ID: 5501, PersonID: 6601, SchoolClass: gradClass, Status: peopledirectory.StudentStatusActive}
	late := peopledirectory.Student{ID: 5502, PersonID: 6602, SchoolClass: gradClass, Status: peopledirectory.StudentStatusActive}

	directory := &lateArrivalDirectory{snapshot: []peopledirectory.Student{present}, late: late}
	workflow := &Workflow{deps: Dependencies{
		UnitOfWork: func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
		Authorize: func(context.Context, string) (Actor, error) {
			return Actor{TenantID: 4711, AccountID: 4712}, nil
		},
		Observe:              func(Observation) {},
		LockRecurrenceWrites: func(context.Context) error { return nil },
		Directory:            directory,
		Structure: transitionStub{transition: schoolstructure.Transition{
			ID: transitionID, Status: schoolstructure.TransitionStatusDraft,
			Mappings: []schoolstructure.TransitionMapping{{TransitionID: transitionID, FromClass: gradClass}},
		}},
	}}

	_, err := workflow.Apply(context.Background(), transitionID, "")
	require.ErrorIs(t, err, ErrPreviewStale,
		"a child added to a mapped class after the snapshot must abort the apply, not be skipped")

	// The guard fired after the snapshot child was locked, and nothing was
	// written: the graduation, history and roster commands are nil ports that
	// would have panicked.
	assert.Equal(t, []int64{present.ID}, directory.locked)
	assert.Equal(t, 2, directory.reads, "the cohort is read once for the snapshot and once under the locks")
}
