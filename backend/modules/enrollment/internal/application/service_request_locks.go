package application

import (
	"context"
)

// AcquireSubmissionDedupLock requires the caller's transaction so the lock
// remains held through duplicate detection and submission persistence.
func (m *Service) AcquireSubmissionDedupLock(ctx context.Context, phaseID int64, emailHash uint64) error {
	return m.store.AcquireSubmissionDedupLock(ctx, phaseID, emailHash)
}

// AcquireExistingStudentMatchLock serializes matching across all guardians
// submitting to a phase, until the caller's transaction ends.
func (m *Service) AcquireExistingStudentMatchLock(ctx context.Context, phaseID int64) error {
	return m.store.AcquireExistingStudentMatchLock(ctx, phaseID)
}
