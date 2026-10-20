// Package studentdeletion coordinates the permanent deletion and
// anonymization of a child across owner capabilities (#2710). It owns no
// tables and performs no irreversible work itself: every row is removed or
// anonymized by its owner inside one tenant transaction, and file cleanup is
// recorded as durable intents that the owners execute after commit.
package studentdeletion

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func (w *Workflow) countOtherStudentRecords(ctx context.Context, studentID int64) (int, error) {
	rosterRemovals, err := w.deps.Timetable.CountStudentRosterRemovals(ctx, studentID)
	if err != nil {
		return 0, err
	}
	history, err := w.deps.Structure.CountStudentTransitionHistory(ctx, studentID)
	if err != nil {
		return 0, err
	}
	appointments, err := w.deps.CountAppointments(ctx, studentID)
	if err != nil {
		return 0, err
	}
	auditReferences, err := w.deps.CountAuditReferences(ctx, studentID)
	if err != nil {
		return 0, err
	}
	feedback, err := w.deps.CountFeedback(ctx, studentID)
	if err != nil {
		return 0, err
	}
	return rosterRemovals + history + appointments + auditReferences + feedback, nil
}

func (w *Workflow) lockInitialCompanions(ctx context.Context, subjectID int64, ids []int64) (map[int64]bool, int64, error) {
	held := make(map[int64]bool, len(ids))
	var maxHeld int64
	for _, id := range ascending(ids) {
		found, err := w.lockStudent(ctx, id, "update")
		if err != nil {
			return nil, 0, err
		}
		if !found && id == subjectID {
			return nil, 0, ErrStudentNotFound
		}
		held[id] = true
		if id > maxHeld {
			maxHeld = id
		}
	}
	return held, maxHeld, nil
}

func (w *Workflow) requireCompanionUnstranded(ctx context.Context, id int64, days []string, covered map[string]bool) error {
	// The rows are already held by lockCompanionGraph; this read only
	// fetches the plan under the lock this transaction owns.
	companion, err := w.deps.Directory.ReadEnrollmentStudent(ctx, id, "")
	if errors.Is(err, peopledirectory.ErrStudentNotFound) {
		return nil // deleted or another tenant: nothing left to strand
	}
	if err != nil {
		return err
	}
	if companion.DepartureCompanionNote != nil && strings.TrimSpace(*companion.DepartureCompanionNote) != "" {
		return nil // the free-text note carries the detail for every day
	}
	accompanied := accompaniedWeekdays(companion.AllowedDepartureModes, companion.DepartureDays)
	for _, day := range days {
		if accompanied[day] && !covered[day] {
			return ErrCompanionWouldLoseDeparture
		}
	}
	return nil
}
