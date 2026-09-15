package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
)

// The grade-transition ledger commands run inside the caller's tenant
// transaction. The service validates the owner's own invariants (academic
// year form, mapping shape, ledger row completeness) and translates missing
// rows into domain errors; lifecycle rules such as "only a draft can be
// applied" belong to the workflow that coordinates the transition.

func (s *Service) FindTransition(ctx context.Context, tenantID, id int64, lock string) (result domain.Transition, err error) {
	err = s.run("find_transition", func(stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return errTenantRequired
		}
		transition, found, queryStats, findErr := s.store.FindTransition(ctx, tenantID, id, lock)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrTransitionNotFound
		}
		result = transition
		return nil
	})
	return result, err
}

func (s *Service) ListTransitions(ctx context.Context, tenantID int64, filter domain.TransitionFilter) (result []domain.Transition, total int, err error) {
	err = s.run("list_transitions", func(stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return errTenantRequired
		}
		transitions, count, queryStats, listErr := s.store.ListTransitions(ctx, tenantID, filter)
		stats.Add(queryStats)
		result, total = transitions, count
		return listErr
	})
	return result, total, err
}

func (s *Service) CreateTransition(ctx context.Context, tenantID int64, draft domain.TransitionDraft) (result domain.Transition, err error) {
	err = s.run("create_transition", func(stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return errTenantRequired
		}
		year, validateErr := domain.ValidateAcademicYear(draft.AcademicYear)
		if validateErr != nil {
			return validateErr
		}
		draft.AcademicYear = year
		if draft.CreatedBy <= 0 {
			return &domain.InvalidTransitionError{Reason: "created_by is required"}
		}
		mappings, validateErr := domain.NormalizeMappings(draft.Mappings)
		if validateErr != nil {
			return validateErr
		}
		draft.Mappings = mappings
		transition, writeStats, writeErr := s.store.InsertTransition(ctx, tenantID, draft)
		stats.Add(writeStats)
		result = transition
		return writeErr
	})
	return result, err
}

func (s *Service) UpdateTransition(ctx context.Context, tenantID int64, update domain.TransitionUpdate) (result domain.Transition, err error) {
	err = s.run("update_transition", func(stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return errTenantRequired
		}
		current, found, queryStats, findErr := s.store.FindTransition(ctx, tenantID, update.ID, "")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrTransitionNotFound
		}
		year := current.AcademicYear
		if update.AcademicYear != nil {
			validated, validateErr := domain.ValidateAcademicYear(*update.AcademicYear)
			if validateErr != nil {
				return validateErr
			}
			year = validated
		}
		notes := current.Notes
		if update.Notes != nil {
			notes = update.Notes
		}
		var mappings []domain.TransitionMappingInput
		if update.Mappings != nil {
			normalized, validateErr := domain.NormalizeMappings(update.Mappings)
			if validateErr != nil {
				return validateErr
			}
			mappings = normalized
			if mappings == nil {
				mappings = []domain.TransitionMappingInput{}
			}
		}
		rows, writeStats, writeErr := s.store.UpdateTransitionFields(ctx, tenantID, update.ID, year, notes)
		stats.Add(writeStats)
		if writeErr != nil {
			return writeErr
		}
		if rows == 0 {
			return domain.ErrTransitionNotFound
		}
		current.AcademicYear, current.Notes = year, notes
		if update.Mappings != nil {
			replaced, replaceStats, replaceErr := s.store.ReplaceMappings(ctx, tenantID, update.ID, mappings)
			stats.Add(replaceStats)
			if replaceErr != nil {
				return replaceErr
			}
			current.Mappings = replaced
		}
		result = current
		return nil
	})
	return result, err
}

func (s *Service) DeleteTransition(ctx context.Context, tenantID, id int64) error {
	return s.run("delete_transition", func(stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return errTenantRequired
		}
		rows, writeStats, writeErr := s.store.DeleteTransition(ctx, tenantID, id)
		stats.Add(writeStats)
		if writeErr != nil {
			return writeErr
		}
		if rows == 0 {
			return domain.ErrTransitionNotFound
		}
		return nil
	})
}

func (s *Service) LockTransitions(ctx context.Context, tenantID int64) error {
	return s.run("lock_transitions", func(stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return errTenantRequired
		}
		lockStats, err := s.store.LockTransitions(ctx, tenantID)
		stats.Add(lockStats)
		return err
	})
}

func (s *Service) LockLatestApplied(ctx context.Context, tenantID int64) (result domain.Transition, found bool, err error) {
	err = s.run("lock_latest_applied_transition", func(stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return errTenantRequired
		}
		transition, exists, queryStats, lockErr := s.store.LockLatestApplied(ctx, tenantID)
		stats.Add(queryStats)
		result, found = transition, exists
		return lockErr
	})
	return result, found, err
}

func (s *Service) MarkApplied(ctx context.Context, tenantID, id, accountID int64, at time.Time, rosterBaselineInstanceID *int64) error {
	return s.run("mark_transition_applied", func(stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return errTenantRequired
		}
		if accountID <= 0 {
			return &domain.InvalidTransitionError{Reason: "applied_by is required"}
		}
		rows, writeStats, writeErr := s.store.MarkApplied(ctx, tenantID, id, accountID, at, rosterBaselineInstanceID)
		stats.Add(writeStats)
		if writeErr != nil {
			return writeErr
		}
		if rows == 0 {
			return domain.ErrTransitionStateConflict
		}
		return nil
	})
}

func (s *Service) MarkReverted(ctx context.Context, tenantID, id, accountID int64, at time.Time) error {
	return s.run("mark_transition_reverted", func(stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return errTenantRequired
		}
		if accountID <= 0 {
			return &domain.InvalidTransitionError{Reason: "reverted_by is required"}
		}
		rows, writeStats, writeErr := s.store.MarkReverted(ctx, tenantID, id, accountID, at)
		stats.Add(writeStats)
		if writeErr != nil {
			return writeErr
		}
		if rows == 0 {
			return domain.ErrTransitionStateConflict
		}
		return nil
	})
}

func (s *Service) AppendHistory(ctx context.Context, tenantID int64, entries []domain.TransitionHistoryEntry) error {
	return s.run("append_transition_history", func(stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return errTenantRequired
		}
		if err := domain.ValidateHistory(entries); err != nil {
			return err
		}
		writeStats, err := s.store.InsertHistory(ctx, tenantID, entries)
		stats.Add(writeStats)
		return err
	})
}

func (s *Service) ListHistory(ctx context.Context, tenantID, transitionID int64) (result []domain.TransitionHistoryEntry, err error) {
	err = s.run("list_transition_history", func(stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return errTenantRequired
		}
		entries, queryStats, listErr := s.store.ListHistory(ctx, tenantID, transitionID)
		stats.Add(queryStats)
		result = entries
		return listErr
	})
	return result, err
}

func (s *Service) AppendClassTeacherLedger(ctx context.Context, tenantID int64, entries []domain.TransitionClassTeacherEntry) error {
	return s.run("append_transition_class_teacher_ledger", func(stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return errTenantRequired
		}
		if err := domain.ValidateClassTeacherLedger(entries); err != nil {
			return err
		}
		writeStats, err := s.store.InsertClassTeacherLedger(ctx, tenantID, entries)
		stats.Add(writeStats)
		return err
	})
}

func (s *Service) ListClassTeacherLedger(ctx context.Context, tenantID, transitionID int64) (result []domain.TransitionClassTeacherEntry, err error) {
	err = s.run("list_transition_class_teacher_ledger", func(stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return errTenantRequired
		}
		entries, queryStats, listErr := s.store.ListClassTeacherLedger(ctx, tenantID, transitionID)
		stats.Add(queryStats)
		result = entries
		return listErr
	})
	return result, err
}

func (s *Service) AppendClassListLedger(ctx context.Context, tenantID int64, entries []domain.TransitionClassListEntry) error {
	return s.run("append_transition_class_list_ledger", func(stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return errTenantRequired
		}
		if err := domain.ValidateClassListLedger(entries); err != nil {
			return err
		}
		writeStats, err := s.store.InsertClassListLedger(ctx, tenantID, entries)
		stats.Add(writeStats)
		return err
	})
}

func (s *Service) ListClassListLedger(ctx context.Context, tenantID, transitionID int64) (result []domain.TransitionClassListEntry, err error) {
	err = s.run("list_transition_class_list_ledger", func(stats *domain.OperationStats) error {
		if tenantID <= 0 {
			return errTenantRequired
		}
		entries, queryStats, listErr := s.store.ListClassListLedger(ctx, tenantID, transitionID)
		stats.Add(queryStats)
		result = entries
		return listErr
	})
	return result, err
}
