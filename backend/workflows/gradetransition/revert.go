package gradetransition

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
)

// Revert undoes the most recently applied transition. It takes the same
// gates as Apply, so a revert cannot interleave with an apply of a newer
// draft or with a materialization pass, and it enforces the reverse-order
// unwind on the server: the latest applied transition is locked FOR UPDATE
// and must be the one being reverted.
func (w *Workflow) Revert(ctx context.Context, id int64) (result Result, err error) {
	err = w.run(ctx, "revert", OperationApply, id, func(txCtx context.Context, actor Actor) error {
		if err := w.lockGates(txCtx); err != nil {
			return err
		}
		transition, err := w.deps.Structure.FindTransition(txCtx, id)
		if err != nil {
			return translateStructureError(err)
		}
		if err := validateCanRevert(transition); err != nil {
			return err
		}
		latest, found, err := w.deps.Structure.LockLatestAppliedTransition(txCtx)
		if err != nil {
			return fmt.Errorf("failed to resolve latest applied transition: %w", translateStructureError(err))
		}
		if !found || latest.ID != id {
			return ErrNotLatestApplied
		}
		history, err := w.deps.Structure.ListTransitionHistory(txCtx, id)
		if err != nil {
			return fmt.Errorf("failed to get transition history: %w", translateStructureError(err))
		}
		reverted, err := w.executeRevert(txCtx, actor, transition, history)
		if err != nil {
			return err
		}
		result = reverted
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

// validateCanRevert wraps both refusals in ErrTransitionNotApplied: a draft
// was never applied, and an already-reverted transition has nothing left to
// undo.
func validateCanRevert(transition schoolstructure.Transition) error {
	if transition.IsApplied() {
		return nil
	}
	if transition.IsDraft() {
		return fmt.Errorf("%w: transition has not been applied yet", ErrTransitionNotApplied)
	}
	return fmt.Errorf("%w: transition has already been reverted", ErrTransitionNotApplied)
}

func (w *Workflow) executeRevert(ctx context.Context, actor Actor, transition schoolstructure.Transition, history []schoolstructure.TransitionHistoryEntry) (Result, error) {
	result := Result{TransitionID: transition.ID, Warnings: make([]string, 0)}

	graduated, err := w.revertPromotedStudents(ctx, history, &result)
	if err != nil {
		return result, err
	}
	restoredActive, err := w.revertGraduatedStudents(ctx, graduated, &result)
	if err != nil {
		return result, err
	}

	// Re-add the restored children to the rosters they were dropped from
	// (replayed from the archive) or never added to (materialized while
	// alumni). Only the children this revert restored AS ACTIVE: one
	// restored to pending or inactive belongs off actionable rosters like
	// any other non-active child. MUST run before the offering-source
	// resync, so the replay puts the archived sourced rows back first and
	// the resync retains them instead of recreating plain rows.
	if len(restoredActive) > 0 {
		if err := w.deps.Rosters.RestoreStudentsToFutureRosters(ctx, transition.ID, restoredActive, transition.RosterBaselineInstanceID); err != nil {
			return result, fmt.Errorf("failed to reconcile restored rosters: %w", err)
		}
	}

	// Replay the recorded ledgers backwards: created rows are deleted,
	// removed rows are restored, and rows the apply never touched stay.
	if err := w.revertClassTeacherAssignments(ctx, transition.ID, transition.Mappings); err != nil {
		return result, err
	}
	if err := w.revertClassListEntries(ctx, actor, transition.ID, transition.Mappings); err != nil {
		return result, err
	}

	if err := w.deps.ResyncOfferingRosters(ctx, w.deps.Today()); err != nil {
		return result, fmt.Errorf("failed to resync offering-sourced templates: %w", err)
	}

	if err := w.deps.Structure.MarkTransitionReverted(ctx, transition.ID, actor.AccountID, w.deps.Now()); err != nil {
		if errors.Is(err, schoolstructure.ErrTransitionStateConflict) {
			return result, fmt.Errorf("%w: transition has already been reverted", ErrTransitionNotApplied)
		}
		return result, fmt.Errorf("failed to update transition status: %w", translateStructureError(err))
	}
	result.Status = schoolstructure.TransitionStatusReverted
	result.CanRevert = false
	return result, nil
}

// revertPromotedStudents moves every promoted child back while their class
// still equals the one this transition assigned; a child moved elsewhere
// since (a manual correction or a later transition) is left untouched and
// counted in a warning. It returns the graduated ledger rows.
func (w *Workflow) revertPromotedStudents(ctx context.Context, history []schoolstructure.TransitionHistoryEntry, result *Result) ([]schoolstructure.TransitionHistoryEntry, error) {
	graduated := make([]schoolstructure.TransitionHistoryEntry, 0)
	missing := 0
	for _, entry := range history {
		switch {
		case entry.WasPromoted():
			if entry.ToClass == nil {
				missing++
				continue
			}
			rows, err := w.deps.Directory.RevertStudentClass(ctx, entry.StudentID, entry.FromClass, *entry.ToClass)
			if err != nil {
				return nil, fmt.Errorf("failed to revert student %d: %w", entry.StudentID, err)
			}
			if rows > 0 {
				result.StudentsPromoted++
			} else {
				missing++
			}
		case entry.WasGraduated():
			graduated = append(graduated, entry)
		}
	}
	if missing > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%d promoted students could not be reverted (deleted or class changed since promotion)", missing))
	}
	return graduated, nil
}

// revertGraduatedStudents restores the alumni to the lifecycle status they
// held before the transition (rows written before from_status existed fall
// back to active) and hands their bracelets back. It returns the ids
// restored AS ACTIVE, and only those, for the roster reconciliation.
func (w *Workflow) revertGraduatedStudents(ctx context.Context, graduated []schoolstructure.TransitionHistoryEntry, result *Result) ([]int64, error) {
	if len(graduated) == 0 {
		return nil, nil
	}
	byStatus := make(map[string][]int64)
	for _, entry := range graduated {
		status := peopledirectory.StudentStatusActive
		if entry.FromStatus != nil && *entry.FromStatus != "" {
			status = *entry.FromStatus
		}
		byStatus[status] = append(byStatus[status], entry.StudentID)
	}
	statuses := make([]string, 0, len(byStatus))
	for status := range byStatus {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	restored := make([]int64, 0, len(graduated))
	restoredActive := make([]int64, 0, len(graduated))
	for _, status := range statuses {
		reactivated, err := w.deps.Directory.ReactivateStudents(ctx, byStatus[status], status)
		if err != nil {
			return nil, fmt.Errorf("failed to reactivate graduated students: %w", err)
		}
		restored = append(restored, reactivated...)
		if status == peopledirectory.StudentStatusActive {
			restoredActive = append(restoredActive, reactivated...)
		}
	}
	result.StudentsGraduated = len(restored)
	if notRestored := len(graduated) - len(restored); notRestored > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%d graduated students could not be restored (deleted or status changed since graduation)", notRestored))
	}
	if err := w.restoreGraduateTags(ctx, graduated, restored, result); err != nil {
		return nil, err
	}
	return restoredActive, nil
}

// restoreGraduateTags re-links the bracelets the apply released, for the
// children this revert actually reactivated. A tag since issued to another
// child, or a child without a person row, is left as is and reported.
func (w *Workflow) restoreGraduateTags(ctx context.Context, graduated []schoolstructure.TransitionHistoryEntry, restored []int64, result *Result) error {
	restoredSet := make(map[int64]bool, len(restored))
	for _, id := range restored {
		restoredSet[id] = true
	}
	candidates := make([]int64, 0, len(graduated))
	for _, entry := range graduated {
		if entry.RFIDTag != nil && *entry.RFIDTag != "" && restoredSet[entry.StudentID] {
			candidates = append(candidates, entry.StudentID)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	students, err := w.deps.Directory.ListStudentsByID(ctx, candidates)
	if err != nil {
		return fmt.Errorf("failed to resolve graduate persons: %w", err)
	}
	personByStudent := make(map[int64]int64, len(students))
	for _, student := range students {
		personByStudent[student.ID] = student.PersonID
	}
	reissued := 0
	for _, entry := range graduated {
		if entry.RFIDTag == nil || *entry.RFIDTag == "" || !restoredSet[entry.StudentID] {
			continue
		}
		personID, found := personByStudent[entry.StudentID]
		if !found {
			reissued++
			continue
		}
		relinked, err := w.deps.Directory.RestoreTag(ctx, personID, *entry.RFIDTag)
		if err != nil {
			return fmt.Errorf("failed to restore RFID tag for student %d: %w", entry.StudentID, err)
		}
		if !relinked {
			reissued++
		}
	}
	if reissued > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%d RFID tags could not be re-linked (reassigned to another child since graduation)", reissued))
	}
	return nil
}
