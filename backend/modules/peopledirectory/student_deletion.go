package peopledirectory

import (
	"context"
	"errors"
	"time"
)

// ErrStudentLockBusy is the retriable refusal of a NOWAIT student row lock
// (ReadEnrollmentStudent with lock "nowait"): another transaction holds the
// row and waiting for it head-on could deadlock against the ascending-id
// order every companion writer follows.
var ErrStudentLockBusy = errors.New("student row is locked by another transaction")

// StudentDeletionCommand is the People Directory half of a permanent child
// deletion (#2710). The commands join the caller's transaction; the
// coordinating workflow holds the student and person locks (see
// ReadEnrollmentStudent and FindPersonForMutation) and decides the order.
type StudentDeletionCommand interface {
	// CountStudentGuardianLinks counts the guardian relationships
	// the deletion removes, for the preview and its locked recheck.
	CountStudentGuardianLinks(ctx context.Context, studentID, personID int64) (int, error)
	// DeleteStudent hard-deletes the student row and returns 1, or 0 when the
	// row was already gone.
	DeleteStudent(ctx context.Context, studentID int64) (int64, error)
	// AnonymizeDeletedStudentPerson replaces the person's identifiers with
	// placeholders and tombstones the row while its updated_at still matches
	// the previewed value. False means a concurrent edit moved the row and the
	// confirmed name no longer describes it.
	AnonymizeDeletedStudentPerson(ctx context.Context, personID int64, updatedAt time.Time) (bool, error)
}
