package studentdeletion

import "context"

// countSnapshot retains the internal enrollment-reference fact the workflow
// needs to recheck links that the preview does not show as deletions.
type countSnapshot struct {
	Counts
	hasEnrollmentRefs bool
}

func (w *Workflow) loadEnrollmentRefs(ctx context.Context, studentID int64, counts *countSnapshot) error {
	approved, total, err := w.deps.Enrollment.CountStudentReferencesForDeletion(ctx, studentID)
	if err != nil {
		return err
	}
	counts.EnrollmentReferences = approved
	counts.hasEnrollmentRefs = total > 0
	return nil
}
