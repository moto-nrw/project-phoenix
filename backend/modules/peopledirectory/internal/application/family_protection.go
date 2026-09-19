package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *StudentService) CurrentFamilyProtection(ctx context.Context, ids []int64) (result map[int64]bool, err error) {
	err = s.run(ctx, "current_family_protection", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CurrentFamilyProtection(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

// SetFamilyProtection appends one ledger entry for the child and returns the
// state that is now current. It locks the student row first, so a graduation
// committing in the meantime is refused instead of protecting an alumnus, and
// two concurrent flips cannot both read "unchanged" and skip their write.
//
// A request that matches the current state returns that state together with
// domain.ErrFamilyProtectionUnchanged: repeating a switch is not an error the
// caller has to fix, and the ledger stays append-only.
func (s *StudentService) SetFamilyProtection(ctx context.Context, change domain.FamilyProtectionChange) (enabled bool, err error) {
	change.Reason = strings.TrimSpace(change.Reason)
	if change.StudentID <= 0 || change.ActorAccountID <= 0 || change.Reason == "" ||
		len([]rune(change.Reason)) > domain.MaxFamilyProtectionReasonRunes {
		return false, domain.ErrFamilyProtectionInvalid
	}
	err = s.run(ctx, "set_family_protection", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		status, found, lockStats, err := s.store.LockLifecycle(txCtx, change.StudentID)
		stats.Add(lockStats)
		if err != nil {
			return err
		}
		if !found || status == domain.StudentStatusAlumnus {
			return domain.ErrStudentNotFound
		}
		current, queryStats, err := s.store.CurrentFamilyProtection(txCtx, []int64{change.StudentID})
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if existing, recorded := current[change.StudentID]; recorded && existing == change.Enabled {
			enabled = existing
			return domain.ErrFamilyProtectionUnchanged
		}
		appendStats, err := s.store.AppendFamilyProtection(txCtx, change)
		stats.Add(appendStats)
		if err != nil {
			return err
		}
		enabled = change.Enabled
		return nil
	})
	return enabled, err
}
