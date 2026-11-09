package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Preserve the established "enrl" namespace, distinct from email locks.
const existingStudentMatchLockClass int32 = 0x656e726c

// childIdentityLockClass ("enid") namespaces the child identity locks.
const childIdentityLockClass int32 = 0x656e6964

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

// AcquireChildIdentityLock serializes, per school, the approvals of one
// child identity: first name, last name and birthday, normalized like the
// enrolled-child match (FindEnrolledMatchCandidates). An approval that waits
// on it reads the candidates the holder committed (#6170). A hash collision
// only serializes two unrelated children.
func (r *Store) AcquireChildIdentityLock(ctx context.Context, firstName, lastName string, birthday calendar.Date) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("AcquireChildIdentityLock: tenant id is required")
	}
	if birthday.IsZero() {
		return errors.New("AcquireChildIdentityLock: birthday is required")
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(`SELECT pg_advisory_xact_lock(?, hashtext(
		?::text || chr(31) || LOWER(NORMALIZE(TRIM(?::text), NFC) COLLATE public.german_name) || chr(31) ||
		LOWER(NORMALIZE(TRIM(?::text), NFC) COLLATE public.german_name) || chr(31) || ?::text))`,
		childIdentityLockClass, tenantID, strings.TrimSpace(firstName), strings.TrimSpace(lastName), birthday.String()).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire child identity lock: %w", err)
	}
	return nil
}
