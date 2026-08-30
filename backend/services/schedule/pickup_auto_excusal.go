package schedule

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// PickupAutoExcusalSyncer derives partial absences from pulled-forward day
// pickup times (#2360). It reuses the #2216 partial-absence mechanics
// (excused_from on schedule.student_pickup_exceptions +
// Apply/ReleasePartialAbsence on schedule.instance_students) — there is
// deliberately no third schedule.
//
// The trigger rule (decided on the issue): a day exception couples an
// automatic excusal exactly when the child has a weekly pickup time for that
// weekday AND the exception's pickup time is EARLIER than it. Later or equal
// times, timeless exceptions ("Kommt heute nicht"), and days without a weekly
// baseline never couple. Manual partial absences (ExcusedAuto == false with
// ExcusedFrom set) are never touched.
//
// Every method must run inside a tenant transaction with
// LockCareExceptionDay already held for the student and date — the same
// serialization contract the manual partial-absence writers follow.
type PickupAutoExcusalSyncer struct {
	pickups scheduleModel.StudentPickupExceptionRepository
	weekly  PickupBaselineReader
	slots   scheduleModel.InstanceStudentRepository
	preview partialAbsenceBlockFinder
	db      *bun.DB
}

type partialAbsenceBlockFinder interface {
	FindPartialAbsenceBlocks(context.Context, int64, timezone.Date, time.Time) ([]scheduleModel.PartialAbsenceBlock, error)
}

// NewPickupAutoExcusalSyncer wires the auto-excusal sync used by the staff
// pickup-exception writers, the parent care-exception writers, and the
// weekly-baseline writers (ResyncFutureExceptions needs db for the per-day
// care locks).
func NewPickupAutoExcusalSyncer(
	pickups scheduleModel.StudentPickupExceptionRepository,
	weekly PickupBaselineReader,
	slots scheduleModel.InstanceStudentRepository,
	db *bun.DB,
) *PickupAutoExcusalSyncer {
	preview, _ := slots.(partialAbsenceBlockFinder)
	return &PickupAutoExcusalSyncer{
		pickups: pickups,
		weekly:  weekly,
		slots:   slots,
		preview: preview,
		db:      db,
	}
}

// Sync reconciles the auto excusal for one freshly written pickup exception.
// It re-reads the row, decides whether an auto excusal is desired, and
// applies, updates, or releases the per-block absences accordingly. The
// returned bool reports whether the exception row was rewritten — callers use
// it to decide whether their in-memory copy went stale.
func (s *PickupAutoExcusalSyncer) Sync(ctx context.Context, exceptionID int64) (bool, error) {
	row, err := s.pickups.FindByID(ctx, exceptionID)
	if err != nil {
		return false, fmt.Errorf("auto excusal: load pickup exception %d: %w", exceptionID, err)
	}
	if row == nil {
		return false, nil
	}
	// A staff-set partial absence is an explicit decision; the sync never
	// overrides it, even when the pickup time changes underneath.
	if row.HasManualPartialAbsence() {
		return false, nil
	}

	desired, cutoff, err := s.desiredCutoff(ctx, row)
	if err != nil {
		return false, err
	}

	current := row.ExcusedAuto && row.ExcusedFrom != nil
	if desired && current && timezone.SameClockTime(*row.ExcusedFrom, cutoff) {
		return false, nil
	}
	if !desired && !current {
		return false, nil
	}

	if current {
		if _, err := s.slots.ReleasePartialAbsence(ctx, row.ID); err != nil {
			return false, fmt.Errorf("auto excusal: release blocks for exception %d: %w", row.ID, err)
		}
	}

	if desired {
		row.ExcusedFrom = &cutoff
		row.ExcusedAuto = true
	} else {
		row.ExcusedFrom = nil
		row.ExcusedAuto = false
	}
	row.ExcusedReason = nil
	row.ExcusedCreatedBy = nil
	row.ExcusedOwnsPickupTime = false
	// Re-anchor scanned TIME values before the full-row update — the driver
	// scans them onto year 0, which bun would bind back out of range.
	normalizePickupExceptionTimes(row)
	if err := s.pickups.Update(ctx, row); err != nil {
		return false, fmt.Errorf("auto excusal: update pickup exception %d: %w", row.ID, err)
	}

	if desired {
		if _, err := s.slots.ApplyPartialAbsence(ctx, row.ID); err != nil {
			return false, fmt.Errorf("auto excusal: apply blocks for exception %d: %w", row.ID, err)
		}
	}
	return true, nil
}

// Preview lists the timetable blocks that would be excused by a proposed
// pickup time. A later or unchanged pickup has no removal impact.
func (s *PickupAutoExcusalSyncer) Preview(
	ctx context.Context, studentID int64, date timezone.Date, pickupTime time.Time,
) ([]scheduleModel.PartialAbsenceBlock, error) {
	row := &scheduleModel.StudentPickupException{
		StudentID:     studentID,
		ExceptionDate: date,
		PickupTime:    &pickupTime,
	}
	desired, cutoff, err := s.desiredCutoff(ctx, row)
	if err != nil {
		return nil, err
	}
	if !desired {
		return []scheduleModel.PartialAbsenceBlock{}, nil
	}
	if s.preview == nil {
		return nil, errors.New("auto excusal: block preview repository not configured")
	}
	blocks, err := s.preview.FindPartialAbsenceBlocks(ctx, studentID, date, cutoff)
	if err != nil {
		return nil, fmt.Errorf("auto excusal: preview affected blocks: %w", err)
	}
	return blocks, nil
}

// DetachForDate releases an existing auto excusal on the student's exception
// for the date and clears its metadata, so a following overwrite or delete of
// the exception row starts from a clean state. Manual partial absences are
// left in place — their guards in the exception writers still apply.
func (s *PickupAutoExcusalSyncer) DetachForDate(ctx context.Context, studentID int64, date timezone.Date) error {
	row, err := s.pickups.FindByStudentIDAndDate(ctx, studentID, date)
	if err != nil {
		return fmt.Errorf("auto excusal: load pickup exception for detach: %w", err)
	}
	return s.DetachRow(ctx, row)
}

// DetachRow is DetachForDate for an already loaded row. Nil rows and rows
// without an auto excusal are no-ops.
func (s *PickupAutoExcusalSyncer) DetachRow(ctx context.Context, row *scheduleModel.StudentPickupException) error {
	if row == nil || !row.ExcusedAuto {
		return nil
	}
	if _, err := s.slots.ReleasePartialAbsence(ctx, row.ID); err != nil {
		return fmt.Errorf("auto excusal: release blocks for exception %d: %w", row.ID, err)
	}
	row.ExcusedFrom = nil
	row.ExcusedReason = nil
	row.ExcusedCreatedBy = nil
	row.ExcusedOwnsPickupTime = false
	row.ExcusedAuto = false
	normalizePickupExceptionTimes(row)
	if err := s.pickups.Update(ctx, row); err != nil {
		return fmt.Errorf("auto excusal: clear pickup exception %d: %w", row.ID, err)
	}
	return nil
}

// ResyncFutureExceptions re-runs the auto-excusal decision for every upcoming
// pickup exception (today onwards) of the student. Weekly-baseline writers
// call it in the same transaction as their change: moving a weekday time
// earlier, later, or removing it re-derives or releases the coupled block
// absences instead of leaving them stale until someone edits the day
// exception. Manual partial absences stay untouched, matching Sync.
//
// Must run inside a tenant transaction. It takes the student row lock first
// (the shared first lock of every care-day writer — this also stabilizes the
// upcoming-exception set) and then the per-day care locks in ascending date
// order. A missing student (offboarding in the same transaction) is a no-op.
func (s *PickupAutoExcusalSyncer) ResyncFutureExceptions(ctx context.Context, studentID int64) error {
	if err := LockCareStudent(ctx, s.db, studentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("auto excusal: lock student %d for weekly resync: %w", studentID, err)
	}
	rows, err := s.pickups.FindUpcomingByStudentID(ctx, studentID)
	if err != nil {
		return fmt.Errorf("auto excusal: load upcoming pickup exceptions for student %d: %w", studentID, err)
	}
	for _, row := range rows {
		if row == nil || row.HasManualPartialAbsence() {
			continue
		}
		// Rows that can neither gain nor lose an auto excusal need no day lock.
		if row.PickupTime == nil && !row.ExcusedAuto {
			continue
		}
		if err := LockCareExceptionDay(ctx, s.db, row.StudentID, row.ExceptionDate); err != nil {
			return fmt.Errorf("auto excusal: lock care day %s for weekly resync: %w", row.ExceptionDate, err)
		}
		if _, err := s.Sync(ctx, row.ID); err != nil {
			return err
		}
	}
	return nil
}

// ReleaseBeforeDelete releases the auto excusal's block absences without
// touching the row itself — for callers that delete the exception row in the
// same transaction. Without the release, the FK's ON DELETE SET NULL would
// strand the blocks as absent with no provenance to restore them from.
func (s *PickupAutoExcusalSyncer) ReleaseBeforeDelete(ctx context.Context, row *scheduleModel.StudentPickupException) error {
	if row == nil || !row.ExcusedAuto {
		return nil
	}
	if _, err := s.slots.ReleasePartialAbsence(ctx, row.ID); err != nil {
		return fmt.Errorf("auto excusal: release blocks for exception %d: %w", row.ID, err)
	}
	return nil
}

// desiredCutoff decides whether the exception should carry an auto excusal
// and from which wall-clock time. Only a pull-forward against the weekly
// baseline couples; without a baseline there is no "Vorverlegung".
func (s *PickupAutoExcusalSyncer) desiredCutoff(
	ctx context.Context, row *scheduleModel.StudentPickupException,
) (bool, time.Time, error) {
	var zero time.Time
	if row.PickupTime == nil {
		return false, zero, nil
	}
	weekday := effectiveISOWeekday(row.ExceptionDate)
	if weekday > scheduleModel.WeekdayFriday {
		return false, zero, nil
	}
	projection, err := s.weekly.Project(ctx, []int64{row.StudentID}, row.ExceptionDate, row.ExceptionDate)
	if err != nil {
		return false, zero, fmt.Errorf("auto excusal: load weekly pickup baseline: %w", err)
	}
	baseline := projection.ForDate(row.StudentID, row.ExceptionDate)
	if baseline == nil {
		return false, zero, nil
	}
	exceptionClock := timezone.NormalizeWallClock(*row.PickupTime)
	baselineClock := timezone.NormalizeWallClock(baseline.PickupTime)
	if !exceptionClock.Before(baselineClock) {
		return false, zero, nil
	}
	return true, exceptionClock, nil
}
