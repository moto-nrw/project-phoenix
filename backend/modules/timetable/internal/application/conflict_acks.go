package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) ListConflictAcks(ctx context.Context, accountID int64) (result []string, err error) {
	if accountID <= 0 {
		return nil, s.reject("list_conflict_acks", timetable.ErrInvalidConflictAck)
	}

	err = s.run(ctx, "list_conflict_acks", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListConflictAckFingerprints(ctx, accountID)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

// AcknowledgeConflict idempotently records the fingerprint for the account.
// Every actual insert prunes the account's oldest rows beyond
// domain.MaxConflictAcksPerAccount inside the same unit of work; a repeated
// acknowledgement cannot grow the set and skips the prune.
func (s *Service) AcknowledgeConflict(ctx context.Context, accountID int64, fingerprint string) error {
	if accountID <= 0 || !timetable.ValidConflictAckFingerprint(fingerprint) {
		return s.reject("acknowledge_conflict", timetable.ErrInvalidConflictAck)
	}

	return s.runWrite(ctx, "acknowledge_conflict", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		inserted, insertStats, err := s.store.InsertConflictAck(txCtx, accountID, fingerprint)
		stats.Add(insertStats)
		if err != nil || !inserted {
			return err
		}
		_, pruneStats, pruneErr := s.store.PruneConflictAcks(txCtx, accountID, domain.MaxConflictAcksPerAccount)
		stats.Add(pruneStats)
		return pruneErr
	})
}

func (s *Service) UnacknowledgeConflict(ctx context.Context, accountID int64, fingerprint string) error {
	if accountID <= 0 || !timetable.ValidConflictAckFingerprint(fingerprint) {
		return s.reject("unacknowledge_conflict", timetable.ErrInvalidConflictAck)
	}

	return s.runWrite(ctx, "unacknowledge_conflict", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteConflictAck(txCtx, accountID, fingerprint)
		stats.Add(queryStats)
		return err
	})
}
