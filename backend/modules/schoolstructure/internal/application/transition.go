package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The grade-transition commands run inside the workflow's tenant
// transaction; they take the tenant from context like the ledger commands
// and never open a transaction of their own. The service validates the
// owner's own invariants (academic year form, mapping shape, ledger row
// completeness) and reports missing rows with the public errors; lifecycle
// rules such as "only a draft can be applied" belong to the workflow that
// coordinates the transition.

func invalidTransition(reason string) error {
	return &schoolstructure.InvalidTransitionError{Reason: reason}
}

func (s *Service) FindTransition(ctx context.Context, id int64) (schoolstructure.Transition, error) {
	return s.findTransition(ctx, id, "")
}

// LockTransitionForMutation reads the transition with its mappings and holds
// the row FOR UPDATE for the caller's transaction.
func (s *Service) LockTransitionForMutation(ctx context.Context, id int64) (schoolstructure.Transition, error) {
	return s.findTransition(ctx, id, "update")
}

func (s *Service) findTransition(ctx context.Context, id int64, lock string) (schoolstructure.Transition, error) {
	if id <= 0 {
		return schoolstructure.Transition{}, invalidTransition("transition ID is required")
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return schoolstructure.Transition{}, err
	}
	var result schoolstructure.Transition
	err = s.run("find_transition", func(stats *domain.OperationStats) error {
		if tenantID.Int64() <= 0 {
			return errTenantRequired
		}
		transition, found, queryStats, findErr := s.store.FindTransition(ctx, tenantID.Int64(), id, lock)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return schoolstructure.ErrTransitionNotFound
		}
		result = transition
		return nil
	})
	return result, err
}

func (s *Service) ListTransitions(ctx context.Context, filter schoolstructure.TransitionFilter) ([]schoolstructure.Transition, int, error) {
	if filter.Limit < 0 || filter.Offset < 0 || (filter.AfterID != nil && *filter.AfterID < 0) {
		return nil, 0, invalidTransition("invalid transition listing window")
	}
	filter.Status = strings.TrimSpace(filter.Status)
	filter.AcademicYear = strings.TrimSpace(filter.AcademicYear)
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	var result []schoolstructure.Transition
	var total int
	err = s.run("list_transitions", func(stats *domain.OperationStats) error {
		if tenantID.Int64() <= 0 {
			return errTenantRequired
		}
		transitions, count, queryStats, listErr := s.store.ListTransitions(ctx, tenantID.Int64(), filter)
		stats.Add(queryStats)
		result, total = transitions, count
		return listErr
	})
	if err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

func (s *Service) CreateTransition(ctx context.Context, draft schoolstructure.TransitionDraft) (schoolstructure.Transition, error) {
	if draft.CreatedBy <= 0 {
		return schoolstructure.Transition{}, invalidTransition("created_by is required")
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return schoolstructure.Transition{}, err
	}
	var result schoolstructure.Transition
	err = s.run("create_transition", func(stats *domain.OperationStats) error {
		if tenantID.Int64() <= 0 {
			return errTenantRequired
		}
		year, validateErr := validateAcademicYear(draft.AcademicYear)
		if validateErr != nil {
			return validateErr
		}
		draft.AcademicYear = year
		mappings, validateErr := normalizeMappings(draft.Mappings)
		if validateErr != nil {
			return validateErr
		}
		draft.Mappings = mappings
		transition, writeStats, writeErr := s.store.InsertTransition(ctx, tenantID.Int64(), draft)
		stats.Add(writeStats)
		result = transition
		return writeErr
	})
	return result, err
}

func (s *Service) UpdateTransition(ctx context.Context, update schoolstructure.TransitionUpdate) (schoolstructure.Transition, error) {
	if update.ID <= 0 {
		return schoolstructure.Transition{}, invalidTransition("transition ID is required")
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return schoolstructure.Transition{}, err
	}
	var result schoolstructure.Transition
	err = s.run("update_transition", func(stats *domain.OperationStats) error {
		if tenantID.Int64() <= 0 {
			return errTenantRequired
		}
		current, findErr := s.findDraftTransition(ctx, tenantID.Int64(), update.ID, stats)
		if findErr != nil {
			return findErr
		}
		updated, mappings, validateErr := applyTransitionUpdate(current, update)
		if validateErr != nil {
			return validateErr
		}
		written, writeErr := s.writeTransitionUpdate(ctx, tenantID.Int64(), update.ID, updated, mappings, stats)
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
func (s *Service) findDraftTransition(ctx context.Context, tenantID, id int64, stats *domain.OperationStats) (schoolstructure.Transition, error) {
	current, found, queryStats, err := s.store.FindTransition(ctx, tenantID, id, "")
	stats.Add(queryStats)
	if err != nil {
		return schoolstructure.Transition{}, err
	}
	if !found {
		return schoolstructure.Transition{}, schoolstructure.ErrTransitionNotFound
	}
	if current.Status != schoolstructure.TransitionStatusDraft {
		return schoolstructure.Transition{}, schoolstructure.ErrTransitionStateConflict
	}
	return current, nil
}

// applyTransitionUpdate validates the update and applies its year and notes
// onto the current draft. The returned mappings replace the stored ones; nil
// keeps them, an empty list clears them.
func applyTransitionUpdate(current schoolstructure.Transition, update schoolstructure.TransitionUpdate) (schoolstructure.Transition, []schoolstructure.TransitionMappingInput, error) {
	if update.AcademicYear != nil {
		validated, err := validateAcademicYear(*update.AcademicYear)
		if err != nil {
			return schoolstructure.Transition{}, nil, err
		}
		current.AcademicYear = validated
	}
	if update.Notes != nil {
		current.Notes = update.Notes
	}
	if update.Mappings == nil {
		return current, nil, nil
	}
	mappings, err := normalizeMappings(update.Mappings)
	if err != nil {
		return schoolstructure.Transition{}, nil, err
	}
	if mappings == nil {
		mappings = []schoolstructure.TransitionMappingInput{}
	}
	return current, mappings, nil
}

// writeTransitionUpdate stores the updated fields and, when mappings is not
// nil, replaces the stored mappings.
func (s *Service) writeTransitionUpdate(ctx context.Context, tenantID, id int64, updated schoolstructure.Transition, mappings []schoolstructure.TransitionMappingInput, stats *domain.OperationStats) (schoolstructure.Transition, error) {
	rows, writeStats, err := s.store.UpdateTransitionFields(ctx, tenantID, id, updated.AcademicYear, updated.Notes)
	stats.Add(writeStats)
	if err != nil {
		return schoolstructure.Transition{}, err
	}
	if rows == 0 {
		// The row was read as a draft moments ago; a zero-row update means
		// it left the draft status in between.
		return schoolstructure.Transition{}, schoolstructure.ErrTransitionStateConflict
	}
	if mappings == nil {
		return updated, nil
	}
	replaced, replaceStats, err := s.store.ReplaceMappings(ctx, tenantID, id, mappings)
	stats.Add(replaceStats)
	if err != nil {
		return schoolstructure.Transition{}, err
	}
	updated.Mappings = replaced
	return updated, nil
}

func (s *Service) DeleteTransition(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidTransition("transition ID is required")
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	return s.run("delete_transition", func(stats *domain.OperationStats) error {
		if tenantID.Int64() <= 0 {
			return errTenantRequired
		}
		rows, writeStats, writeErr := s.store.DeleteTransition(ctx, tenantID.Int64(), id)
		stats.Add(writeStats)
		if writeErr != nil {
			return writeErr
		}
		if rows > 0 {
			return nil
		}
		// Only a draft may be deleted: the delete cascades the history and
		// ledgers a revert needs. Tell a missing row apart from a protected one.
		_, found, queryStats, findErr := s.store.FindTransition(ctx, tenantID.Int64(), id, "")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if found {
			return schoolstructure.ErrTransitionStateConflict
		}
		return schoolstructure.ErrTransitionNotFound
	})
}

// LockTransitions takes the tenant-wide transition gate; it requires a
// tenant transaction.
func (s *Service) LockTransitions(ctx context.Context) error {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return errors.New("school structure: the transition gate requires a tenant transaction")
	}
	return s.run("lock_transitions", func(stats *domain.OperationStats) error {
		if tenantID.Int64() <= 0 {
			return errTenantRequired
		}
		lockStats, err := s.store.LockTransitions(ctx, tenantID.Int64())
		stats.Add(lockStats)
		return err
	})
}

func (s *Service) LockLatestAppliedTransition(ctx context.Context) (schoolstructure.Transition, bool, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return schoolstructure.Transition{}, false, err
	}
	var result schoolstructure.Transition
	var found bool
	err = s.run("lock_latest_applied_transition", func(stats *domain.OperationStats) error {
		if tenantID.Int64() <= 0 {
			return errTenantRequired
		}
		transition, exists, queryStats, lockErr := s.store.LockLatestApplied(ctx, tenantID.Int64())
		stats.Add(queryStats)
		result, found = transition, exists
		return lockErr
	})
	return result, found, err
}

func (s *Service) MarkTransitionApplied(ctx context.Context, id, accountID int64, at time.Time, rosterBaselineInstanceID *int64) error {
	if id <= 0 || accountID <= 0 || at.IsZero() {
		return invalidTransition("transition ID, account and instant are required")
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	return s.run("mark_transition_applied", func(stats *domain.OperationStats) error {
		if tenantID.Int64() <= 0 {
			return errTenantRequired
		}
		rows, writeStats, writeErr := s.store.MarkApplied(ctx, tenantID.Int64(), id, accountID, at, rosterBaselineInstanceID)
		stats.Add(writeStats)
		if writeErr != nil {
			return writeErr
		}
		if rows == 0 {
			return schoolstructure.ErrTransitionStateConflict
		}
		return nil
	})
}

func (s *Service) MarkTransitionReverted(ctx context.Context, id, accountID int64, at time.Time) error {
	if id <= 0 || accountID <= 0 || at.IsZero() {
		return invalidTransition("transition ID, account and instant are required")
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	return s.run("mark_transition_reverted", func(stats *domain.OperationStats) error {
		if tenantID.Int64() <= 0 {
			return errTenantRequired
		}
		rows, writeStats, writeErr := s.store.MarkReverted(ctx, tenantID.Int64(), id, accountID, at)
		stats.Add(writeStats)
		if writeErr != nil {
			return writeErr
		}
		if rows == 0 {
			return schoolstructure.ErrTransitionStateConflict
		}
		return nil
	})
}

func (s *Service) AppendTransitionHistory(ctx context.Context, entries []schoolstructure.TransitionHistoryEntry) error {
	if len(entries) == 0 {
		return nil
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	return s.run("append_transition_history", func(stats *domain.OperationStats) error {
		if tenantID.Int64() <= 0 {
			return errTenantRequired
		}
		if err := validateHistory(entries); err != nil {
			return err
		}
		writeStats, err := s.store.InsertHistory(ctx, tenantID.Int64(), entries)
		stats.Add(writeStats)
		return err
	})
}

func (s *Service) ListTransitionHistory(ctx context.Context, transitionID int64) ([]schoolstructure.TransitionHistoryEntry, error) {
	if transitionID <= 0 {
		return nil, invalidTransition("transition ID is required")
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var result []schoolstructure.TransitionHistoryEntry
	err = s.run("list_transition_history", func(stats *domain.OperationStats) error {
		if tenantID.Int64() <= 0 {
			return errTenantRequired
		}
		entries, queryStats, listErr := s.store.ListHistory(ctx, tenantID.Int64(), transitionID)
		stats.Add(queryStats)
		result = entries
		return listErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) AppendTransitionClassTeacherLedger(ctx context.Context, entries []schoolstructure.TransitionClassTeacherEntry) error {
	if len(entries) == 0 {
		return nil
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	return s.run("append_transition_class_teacher_ledger", func(stats *domain.OperationStats) error {
		if tenantID.Int64() <= 0 {
			return errTenantRequired
		}
		if err := validateClassTeacherLedger(entries); err != nil {
			return err
		}
		writeStats, err := s.store.InsertClassTeacherLedger(ctx, tenantID.Int64(), entries)
		stats.Add(writeStats)
		return err
	})
}

func (s *Service) ListTransitionClassTeacherLedger(ctx context.Context, transitionID int64) ([]schoolstructure.TransitionClassTeacherEntry, error) {
	if transitionID <= 0 {
		return nil, invalidTransition("transition ID is required")
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var result []schoolstructure.TransitionClassTeacherEntry
	err = s.run("list_transition_class_teacher_ledger", func(stats *domain.OperationStats) error {
		if tenantID.Int64() <= 0 {
			return errTenantRequired
		}
		entries, queryStats, listErr := s.store.ListClassTeacherLedger(ctx, tenantID.Int64(), transitionID)
		stats.Add(queryStats)
		result = entries
		return listErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) AppendTransitionClassListLedger(ctx context.Context, entries []schoolstructure.TransitionClassListEntry) error {
	if len(entries) == 0 {
		return nil
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	return s.run("append_transition_class_list_ledger", func(stats *domain.OperationStats) error {
		if tenantID.Int64() <= 0 {
			return errTenantRequired
		}
		if err := validateClassListLedger(entries); err != nil {
			return err
		}
		writeStats, err := s.store.InsertClassListLedger(ctx, tenantID.Int64(), entries)
		stats.Add(writeStats)
		return err
	})
}

func (s *Service) ListTransitionClassListLedger(ctx context.Context, transitionID int64) ([]schoolstructure.TransitionClassListEntry, error) {
	if transitionID <= 0 {
		return nil, invalidTransition("transition ID is required")
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var result []schoolstructure.TransitionClassListEntry
	err = s.run("list_transition_class_list_ledger", func(stats *domain.OperationStats) error {
		if tenantID.Int64() <= 0 {
			return errTenantRequired
		}
		entries, queryStats, listErr := s.store.ListClassListLedger(ctx, tenantID.Int64(), transitionID)
		stats.Add(queryStats)
		result = entries
		return listErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
