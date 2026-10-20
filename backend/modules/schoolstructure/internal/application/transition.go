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
		current, findErr := s.findDraftTransition(ctx, tenantID, update.ID, stats)
		if findErr != nil {
			return findErr
		}
		updated, mappings, validateErr := applyTransitionUpdate(current, update)
		if validateErr != nil {
			return validateErr
		}
		written, writeErr := s.writeTransitionUpdate(ctx, tenantID, update.ID, updated, mappings, stats)
		if writeErr != nil {
			return writeErr
		}
		result = written
		return nil
	})
	return result, err
}

// findDraftTransition reads the transition an update edits; only a draft may
// be edited.
func (s *Service) findDraftTransition(ctx context.Context, tenantID, id int64, stats *domain.OperationStats) (domain.Transition, error) {
	current, found, queryStats, err := s.store.FindTransition(ctx, tenantID, id, "")
	stats.Add(queryStats)
	if err != nil {
		return domain.Transition{}, err
	}
	if !found {
		return domain.Transition{}, domain.ErrTransitionNotFound
	}
	if current.Status != domain.TransitionStatusDraft {
		return domain.Transition{}, domain.ErrTransitionStateConflict
	}
	return current, nil
}

// applyTransitionUpdate validates the update and applies its year and notes
// onto the current draft. The returned mappings replace the stored ones; nil
// keeps them, an empty list clears them.
func applyTransitionUpdate(current domain.Transition, update domain.TransitionUpdate) (domain.Transition, []domain.TransitionMappingInput, error) {
	if update.AcademicYear != nil {
		validated, err := domain.ValidateAcademicYear(*update.AcademicYear)
		if err != nil {
			return domain.Transition{}, nil, err
		}
		current.AcademicYear = validated
	}
	if update.Notes != nil {
		current.Notes = update.Notes
	}
	if update.Mappings == nil {
		return current, nil, nil
	}
	mappings, err := domain.NormalizeMappings(update.Mappings)
	if err != nil {
		return domain.Transition{}, nil, err
	}
	if mappings == nil {
		mappings = []domain.TransitionMappingInput{}
	}
	return current, mappings, nil
}

// writeTransitionUpdate stores the updated fields and, when mappings is not
// nil, replaces the stored mappings.
func (s *Service) writeTransitionUpdate(ctx context.Context, tenantID, id int64, updated domain.Transition, mappings []domain.TransitionMappingInput, stats *domain.OperationStats) (domain.Transition, error) {
	rows, writeStats, err := s.store.UpdateTransitionFields(ctx, tenantID, id, updated.AcademicYear, updated.Notes)
	stats.Add(writeStats)
	if err != nil {
		return domain.Transition{}, err
	}
	if rows == 0 {
		// The row was read as a draft moments ago; a zero-row update means
		// it left the draft status in between.
		return domain.Transition{}, domain.ErrTransitionStateConflict
	}
	if mappings == nil {
		return updated, nil
	}
	replaced, replaceStats, err := s.store.ReplaceMappings(ctx, tenantID, id, mappings)
	stats.Add(replaceStats)
	if err != nil {
		return domain.Transition{}, err
	}
	updated.Mappings = replaced
	return updated, nil
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
		if rows > 0 {
			return nil
		}
		// Only a draft may be deleted: the delete cascades the history and
		// ledgers a revert needs. Tell a missing row apart from a protected one.
		_, found, queryStats, findErr := s.store.FindTransition(ctx, tenantID, id, "")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if found {
			return domain.ErrTransitionStateConflict
		}
		return domain.ErrTransitionNotFound
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
