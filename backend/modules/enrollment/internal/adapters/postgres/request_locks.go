package postgres

import (
	"context"
	"fmt"
	"math"
)

// Preserve the established "enrl" namespace, distinct from email locks.
const existingStudentMatchLockClass int32 = 0x656e726c

func (r *Store) AcquireSubmissionDedupLock(ctx context.Context, phaseID int64, emailHash uint64) error {
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(`SELECT pg_advisory_xact_lock(?, ?)`, int32(phaseID&0x7fffffff), int32(emailHash&0x7fffffff)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire enrollment submission dedup lock: %w", err)
	}
	return nil
}

func (r *Store) AcquireExistingStudentMatchLock(ctx context.Context, phaseID int64) error {
	if phaseID <= 0 || phaseID > math.MaxInt32 {
		return fmt.Errorf("AcquireExistingStudentMatchLock: phase_id %d out of advisory-lock range", phaseID)
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(`SELECT pg_advisory_xact_lock(?, ?)`, existingStudentMatchLockClass, int32(phaseID)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire existing-student match lock: %w", err)
	}
	return nil
}
