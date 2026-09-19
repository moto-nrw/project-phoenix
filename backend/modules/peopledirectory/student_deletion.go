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
	// CountStudentGuardianLinks counts the current and legacy guardian links
	// the deletion removes, for the preview and its locked recheck.
	CountStudentGuardianLinks(ctx context.Context, studentID, personID int64) (int, error)
	// DeleteLegacyGuardianLinks removes the person-based guardian links. They
	// do not cascade from the student row because the anonymized person
	// tombstone remains.
	DeleteLegacyGuardianLinks(ctx context.Context, personID int64) (int64, error)
	// DeleteStudent hard-deletes the student row and returns 1, or 0 when the
	// row was already gone.
	DeleteStudent(ctx context.Context, studentID int64) (int64, error)
	// AnonymizeDeletedStudentPerson replaces the person's identifiers with
	// placeholders and tombstones the row while its updated_at still matches
	// the previewed value. False means a concurrent edit moved the row and the
	// confirmed name no longer describes it.
	AnonymizeDeletedStudentPerson(ctx context.Context, personID int64, updatedAt time.Time) (bool, error)
}

type studentDeletionEngine interface {
	CountStudentGuardianLinks(ctx context.Context, studentID, personID int64) (int, error)
	DeleteLegacyGuardianLinks(ctx context.Context, personID int64) (int64, error)
	DeleteStudent(ctx context.Context, id int64) (int64, error)
	AnonymizeDeletedStudentPerson(ctx context.Context, personID int64, updatedAt time.Time) (bool, error)
}

func (m *Module) CountStudentGuardianLinks(ctx context.Context, studentID, personID int64) (int, error) {
	if studentID <= 0 || personID <= 0 {
		return 0, invalidStudent("student and person IDs are required")
	}
	return m.engine.CountStudentGuardianLinks(ctx, studentID, personID)
}

func (m *Module) DeleteLegacyGuardianLinks(ctx context.Context, personID int64) (int64, error) {
	if personID <= 0 {
		return 0, invalid("person ID is required")
	}
	return m.engine.DeleteLegacyGuardianLinks(ctx, personID)
}

func (m *Module) DeleteStudent(ctx context.Context, studentID int64) (int64, error) {
	if studentID <= 0 {
		return 0, invalidStudent("student ID is required")
	}
	return m.engine.DeleteStudent(ctx, studentID)
}

func (m *Module) AnonymizeDeletedStudentPerson(ctx context.Context, personID int64, updatedAt time.Time) (bool, error) {
	if personID <= 0 {
		return false, invalid("person ID is required")
	}
	if updatedAt.IsZero() {
		return false, invalid("previewed person revision is required")
	}
	return m.engine.AnonymizeDeletedStudentPerson(ctx, personID, updatedAt)
}
