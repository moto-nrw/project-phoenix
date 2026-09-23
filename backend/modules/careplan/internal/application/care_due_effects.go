package application

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// careDueEffects is what one effect-day pass changed.
type careDueEffects struct {
	applied, closedPresence, closedRequests, releasedTags int
}

func (e *careDueEffects) add(page careDueEffects) {
	e.applied += page.applied
	e.closedPresence += page.closedPresence
	e.closedRequests += page.closedRequests
	e.releasedTags += page.releasedTags
}

// ApplyDueEffects is the effect-day command of the care lifecycle. The
// scheduler runs it for every tenant through this public method; it is
// idempotent: a second pass finds nothing open, no bracelet to release and no
// request to close.
func (s *CareLifecycle) ApplyDueEffects(ctx context.Context, asOf calendar.Date) (int, error) {
	if err := s.reconcileExpiredCareBookings(ctx, asOf); err != nil {
		return 0, err
	}
	// The candidates are exactly the children the activation tick is about to
	// move to 'inactive': still active, interval already run out. That makes
	// the pass self-limiting, and it therefore runs BEFORE the status
	// transition, which the scheduler guarantees. The interval's upper bound
	// is inclusive, so the boundary is the day before asOf: a child is still
	// in care on the last care day.
	ids, err := s.owners.Students.ListDueForDeactivation(ctx, asOf.AddDays(-1))
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	ids = domain.DedupeSortedIDs(ids)
	// A school's due set is not bounded by any request, so it is worked off in
	// pages, each in its own transaction. Every child's effects are
	// independent and the pass is idempotent: a page that failed is simply
	// found again by the next tick.
	var effects careDueEffects
	for page := range slices.Chunk(ids, careplan.MaxCareExitBatchSize) {
		if err := s.unit(ctx, func(txCtx context.Context) error {
			pageEffects, applyErr := s.applyDueEffects(txCtx, page, asOf)
			if applyErr != nil {
				return applyErr
			}
			effects.add(pageEffects)
			return nil
		}); err != nil {
			return effects.applied, err
		}
	}
	if effects.closedPresence > 0 || effects.closedRequests > 0 || effects.releasedTags > 0 {
		s.logger.Info("care exit effects applied",
			slog.Int("students", effects.applied),
			slog.Int("presence_records_closed", effects.closedPresence),
			slog.Int("parent_requests_closed", effects.closedRequests),
			slog.Int("tags_released", effects.releasedTags),
		)
	}
	return effects.applied, nil
}

func (s *CareLifecycle) applyDueEffects(ctx context.Context, ids []int64, asOf calendar.Date) (careDueEffects, error) {
	var effects careDueEffects
	if err := s.lockCareBookingWrites(ctx); err != nil {
		return effects, fmt.Errorf("care lifecycle: lock care booking writes for due effects: %w", err)
	}
	// The candidate lookup ran outside this transaction. An operator can
	// change or resume an exit in between, so every row is locked and
	// revalidated before any irreversible effect runs.
	locked, err := s.owners.Students.FindCareStudents(ctx, ids, true)
	if err != nil {
		return effects, err
	}
	current := make([]int64, 0, len(ids))
	for _, id := range ids {
		if student, found := locked[id]; found && student.Status == careplan.StudentStatusActive && student.CareEndedOn(asOf) {
			current = append(current, id)
		}
	}
	if len(current) == 0 {
		return effects, nil
	}
	now := time.Now()
	if effects.closedPresence, err = s.closeOpenPresence(ctx, current, now); err != nil {
		return effects, err
	}
	if effects.closedRequests, err = s.closeOpenRequests(ctx, current, nil, now); err != nil {
		return effects, err
	}
	if effects.releasedTags, err = s.owners.Tags.ReleaseStudentTags(ctx, current); err != nil {
		return effects, err
	}
	if err := s.removePlansWrittenAfterExitConfirmation(ctx, current, locked); err != nil {
		return effects, err
	}
	// The exit is final now. What it removed from the plan stays removed, so
	// the ledger that would have put it back is dropped (#2487).
	if err := s.discardRemovals(ctx, current); err != nil {
		return effects, err
	}
	effects.applied = len(current)
	return effects, nil
}

// reconcileExpiredCareBookings plans the withdrawal tasks of booking-led
// care whose last care booking ran out. In weekly-plan mode it does nothing.
func (s *CareLifecycle) reconcileExpiredCareBookings(ctx context.Context, asOf calendar.Date) error {
	return s.unit(ctx, func(txCtx context.Context) error {
		if err := s.lockCareBookingWrites(txCtx); err != nil {
			return fmt.Errorf("care lifecycle: lock care booking writes for booking expiry: %w", err)
		}
		authoritative, err := s.bookingsAuthoritative(txCtx)
		if err != nil || !authoritative {
			return err
		}
		evaluations, err := s.evaluateCareBookings(txCtx, asOf)
		if err != nil {
			return err
		}
		return s.reconcileBookingEvaluations(txCtx, evaluations, asOf)
	})
}

// removePlansWrittenAfterExitConfirmation removes what was planned for the
// children after their exit was confirmed: the plan rows between the
// confirmation and the effect day would otherwise outlive the care.
func (s *CareLifecycle) removePlansWrittenAfterExitConfirmation(ctx context.Context, studentIDs []int64, students map[int64]domain.CareStudent) error {
	for _, studentID := range studentIDs {
		student, found := students[studentID]
		if !found || student.EnrolledUntil == nil {
			continue
		}
		ids := []int64{studentID}
		if _, err := s.removePlannedRosterAfter(ctx, ids, *student.EnrolledUntil); err != nil {
			return err
		}
		validUntil := student.EnrolledUntil.AddDays(1)
		if _, err := s.capBookings(ctx, ids, validUntil); err != nil {
			return err
		}
		if _, err := s.endSourceBookings(ctx, ids, validUntil, true); err != nil {
			return err
		}
	}
	return nil
}
