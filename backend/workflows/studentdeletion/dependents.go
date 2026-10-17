package studentdeletion

import (
	"context"
	"time"
)

// releaseDependents runs the owner commands that must see the child before
// the cascade removes it.
//
// The document rows go with the child, the queued cleanup intents do not,
// and they are what gets the bytes off disk. The withdrawal tasks lose their
// child references. The accepted enrollments leave with the child (#3962).
// The preview counts every enrollment reference under the lock so a child
// without any skips the lookup; the audit records only the accepted children
// the owner actually removes.
func (w *Workflow) releaseDependents(ctx context.Context, actor Actor, preview Preview, now time.Time, result *Result) error {
	var err error
	if result.DocumentCleanups, err = w.deps.CarePlan.QueueCareDocumentCleanupForDeletedStudent(ctx, preview.StudentID, now); err != nil {
		return err
	}
	if result.WithdrawalsRedacted, err = w.deps.CarePlan.RedactWithdrawalsForDeletedStudent(ctx, preview.StudentID, actor.AccountID, now); err != nil {
		return err
	}
	if preview.Counts.EnrollmentReferences == 0 {
		return nil
	}
	result.Counts.EnrollmentReferences, err = w.deps.Enrollment.DeleteApprovedEnrollmentsOfStudent(ctx, preview.StudentID)
	return err
}
