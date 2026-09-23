package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

var errChildQuotaUnbound = errors.New("school membership: child quota is not bound")

// countingWrite runs a membership write that can raise the Kontingentzahl
// against the school's Kinderkontingent (#3567). It is the one place every
// such write passes through.
//
// The shared class-writes gate comes first so the acquisition order stays
// gate-then-quota for every writer, then the exclusive per-school quota lock,
// then the limit read and count. Taking the lock before the read serializes a
// membership write with an operator changing the Kinderkontingent. The write
// and the second count run in a savepoint, so a refused write is undone even
// when the caller catches the error and commits. The whole write is judged at
// once: a batch is all or nothing.
func (s *Service) countingWrite(ctx context.Context, stats *domain.OperationStats, write func(context.Context) error) error {
	if s.quota == nil {
		return errChildQuotaUnbound
	}
	gateStats, err := s.store.LockStudentClassWrites(ctx, false)
	stats.Add(gateStats)
	if err != nil {
		return err
	}
	lockStats, err := s.store.LockChildQuota(ctx)
	stats.Add(lockStats)
	if err != nil {
		return err
	}
	limit, limited, err := s.quota.ChildQuotaLimit(ctx)
	if err != nil {
		return fmt.Errorf("school membership: read child quota: %w", err)
	}
	if !limited {
		return write(ctx)
	}
	day := s.today()
	before, countStats, err := s.store.CountChildQuota(ctx, day)
	stats.Add(countStats)
	if err != nil {
		return err
	}
	return s.tx.RunSavepoint(ctx, func(savepointCtx context.Context) error {
		if err := write(savepointCtx); err != nil {
			return err
		}
		after, countStats, err := s.store.CountChildQuota(savepointCtx, day)
		stats.Add(countStats)
		if err != nil {
			return err
		}
		return domain.CheckChildQuota(limit, before, after)
	})
}

// countingTransition runs the scheduler's conditional pending → active write.
// Its limit read comes after the compare-and-set, inside the savepoint: a
// no-op for a child from another tenant must stay a no-op even when that
// tenant has no school record. The quota lock and the before count still
// precede the write, so a successful transition is checked atomically.
func (s *Service) countingTransition(ctx context.Context, stats *domain.OperationStats, write func(context.Context) (bool, error)) error {
	if s.quota == nil {
		return errChildQuotaUnbound
	}
	lockStats, err := s.store.LockChildQuota(ctx)
	stats.Add(lockStats)
	if err != nil {
		return err
	}
	day := s.today()
	before, countStats, err := s.store.CountChildQuota(ctx, day)
	stats.Add(countStats)
	if err != nil {
		return err
	}
	return s.tx.RunSavepoint(ctx, func(savepointCtx context.Context) error {
		changed, err := write(savepointCtx)
		if err != nil || !changed {
			return err
		}
		limit, limited, err := s.quota.ChildQuotaLimit(savepointCtx)
		if err != nil {
			return fmt.Errorf("school membership: read child quota: %w", err)
		}
		if !limited {
			return nil
		}
		after, countStats, err := s.store.CountChildQuota(savepointCtx, day)
		stats.Add(countStats)
		if err != nil {
			return err
		}
		return domain.CheckChildQuota(limit, before, after)
	})
}
