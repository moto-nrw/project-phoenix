package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type PickupAutoExcusalSyncer struct {
	pickups    ports.PickupExcusalStore
	weekly     careplan.PickupBaselineReader
	slots      ports.PartialAbsenceBlocks
	preview    ports.PartialAbsencePreview
	extensions ports.PickupExtensions
	locker     ports.PickupExcusalLocker
}

func NewPickupAutoExcusal(pickups ports.PickupExcusalStore, weekly careplan.PickupBaselineReader,
	slots ports.PartialAbsenceBlocks, preview ports.PartialAbsencePreview, extensions ports.PickupExtensions, locker ports.PickupExcusalLocker,
) careplan.PickupAutoExcusal {
	return &PickupAutoExcusalSyncer{pickups: pickups, weekly: weekly, slots: slots, preview: preview, extensions: extensions, locker: locker}
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
	// A staff-set partial absence is an explicit decision. Without later-pickup
	// tasks, no baseline is needed because the sync must leave it untouched.
	// With tasks, the baseline still decides whether the changed pickup is later.
	manualPartialAbsence := row.HasManualPartialAbsence()
	if manualPartialAbsence && s.extensions == nil {
		return false, nil
	}
	baseline, err := s.baselineClock(ctx, row)
	if err != nil {
		return false, err
	}
	if err := s.syncDayExtension(ctx, row, baseline); err != nil {
		return false, err
	}
	if manualPartialAbsence {
		return false, nil
	}
	return s.syncDerivedAbsence(ctx, row, baseline)
}

func (s *PickupAutoExcusalSyncer) syncDerivedAbsence(ctx context.Context, row *careplan.PickupException, baseline *time.Time) (bool, error) {
	desired, cutoff := desiredCutoff(row, baseline)

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
	row.NormalizeWallClockTimes()
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
) ([]carerequests.Block, error) {
	row := &careplan.PickupException{
		StudentID:     studentID,
		ExceptionDate: careplan.Date(date),
		PickupTime:    &pickupTime,
	}
	baseline, err := s.baselineClock(ctx, row)
	if err != nil {
		return nil, err
	}
	desired, cutoff := desiredCutoff(row, baseline)
	if !desired {
		return []carerequests.Block{}, nil
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

// DetachRow is DetachForDate for an already loaded row. It first removes an
// open later-pickup decision for the row's stored date, because a following
// update may move that same exception to another date. Nil rows are no-ops.
func (s *PickupAutoExcusalSyncer) DetachRow(ctx context.Context, row *careplan.PickupException) error {
	if row == nil {
		return nil
	}
	if s.extensions != nil {
		date := timezone.Date(row.ExceptionDate).String()
		if err := s.extensions.ClearPickupDayExtension(ctx, row.StudentID, date); err != nil {
			return fmt.Errorf("pickup extension: clear day %s: %w", date, err)
		}
	}
	if !row.ExcusedAuto {
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
	row.NormalizeWallClockTimes()
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
	if err := s.locker.LockStudent(ctx, studentID); err != nil {
		if s.locker.IsNotFound(err) {
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
		if err := s.locker.LockStudentAndExceptionDay(ctx, row.StudentID, row.ExceptionDate.String()); err != nil {
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
func (s *PickupAutoExcusalSyncer) ReleaseBeforeDelete(ctx context.Context, row *careplan.PickupException) error {
	if row == nil || !row.ExcusedAuto {
		return nil
	}
	if _, err := s.slots.ReleasePartialAbsence(ctx, row.ID); err != nil {
		return fmt.Errorf("auto excusal: release blocks for exception %d: %w", row.ID, err)
	}
	return nil
}

// baselineClock returns the weekly pickup time that applies to the
// exception's date. Timeless exceptions, weekend days and days without a
// weekly time have no baseline.
func (s *PickupAutoExcusalSyncer) baselineClock(
	ctx context.Context, row *careplan.PickupException,
) (*time.Time, error) {
	if row.PickupTime == nil {
		return nil, nil
	}
	date := timezone.Date(row.ExceptionDate)
	if domain.ISOWeekday(date) > 5 {
		return nil, nil
	}
	projection, err := s.weekly.Project(ctx, []int64{row.StudentID}, date, date)
	if err != nil {
		return nil, fmt.Errorf("auto excusal: load weekly pickup baseline: %w", err)
	}
	baseline := projection.ForDate(row.StudentID, date)
	if baseline == nil {
		return nil, nil
	}
	clock := timezone.NormalizeWallClock(baseline.PickupTime)
	return &clock, nil
}

// desiredCutoff decides whether the exception should carry an auto excusal
// and from which wall-clock time. Only a pull-forward against the weekly
// baseline couples; without a baseline there is no "Vorverlegung".
func desiredCutoff(row *careplan.PickupException, baseline *time.Time) (bool, time.Time) {
	if row.PickupTime == nil || baseline == nil {
		return false, time.Time{}
	}
	exceptionClock := timezone.NormalizeWallClock(*row.PickupTime)
	if !exceptionClock.Before(*baseline) {
		return false, time.Time{}
	}
	return true, exceptionClock
}

// syncDayExtension keeps the open block decision of a day in step with the
// exception: a pickup later than the weekly time opens or updates it, any
// other state removes it.
func (s *PickupAutoExcusalSyncer) syncDayExtension(
	ctx context.Context, row *careplan.PickupException, baseline *time.Time,
) error {
	if s.extensions == nil {
		return nil
	}
	date := timezone.Date(row.ExceptionDate).String()
	if row.PickupTime == nil || baseline == nil || !timezone.NormalizeWallClock(*row.PickupTime).After(*baseline) {
		if err := s.extensions.ClearPickupDayExtension(ctx, row.StudentID, date); err != nil {
			return fmt.Errorf("pickup extension: clear day %s: %w", date, err)
		}
		return nil
	}
	err := s.extensions.RecordPickupDayExtension(ctx, ports.PickupDayExtension{
		StudentID:         row.StudentID,
		PickupExceptionID: row.ID,
		Date:              date,
		PreviousPickup:    baseline.Format("15:04"),
		Pickup:            timezone.NormalizeWallClock(*row.PickupTime).Format("15:04"),
	})
	if err != nil {
		return fmt.Errorf("pickup extension: record day %s: %w", date, err)
	}
	return nil
}

// SnapshotWeeklyPickups captures the regular pickup times on date before a
// weekly write. It returns nil when later pickups are not recorded.
func (s *PickupAutoExcusalSyncer) SnapshotWeeklyPickups(
	ctx context.Context, studentID int64, date timezone.Date,
) (careplan.WeeklyPickupSnapshot, error) {
	if s.extensions == nil {
		return nil, nil
	}
	projection, err := s.weekly.Project(ctx, []int64{studentID}, date, date)
	if err != nil {
		return nil, fmt.Errorf("pickup extension: load weekly pickup times: %w", err)
	}
	snapshot := careplan.WeeklyPickupSnapshot{}
	for weekday, row := range projection.WeeklyForDate(studentID, date) {
		if row != nil && weekday >= 1 && weekday <= 5 {
			snapshot[weekday] = timezone.NormalizeWallClock(row.PickupTime).Format("15:04")
		}
	}
	return snapshot, nil
}

// RecordWeeklyPickupChanges compares the weekly pickup times on date with the
// snapshot taken before the write (#3261). A changed weekday is handed to the
// Timetable owner, which opens, keeps or closes its task; a removed weekday
// closes it. A weekday that had no time before opens nothing: there is no
// "longer than before" to plan for. A nil snapshot is a no-op.
func (s *PickupAutoExcusalSyncer) RecordWeeklyPickupChanges(
	ctx context.Context, studentID int64, date timezone.Date, before careplan.WeeklyPickupSnapshot,
) error {
	if s.extensions == nil || before == nil {
		return nil
	}
	after, err := s.SnapshotWeeklyPickups(ctx, studentID, date)
	if err != nil {
		return err
	}
	for weekday := 1; weekday <= 5; weekday++ {
		previous, hadTime := before[weekday]
		current, hasTime := after[weekday]
		switch {
		case !hasTime:
			err = s.extensions.ClearPickupWeekdayExtension(ctx, studentID, weekday)
		case !hadTime || previous == current:
			continue
		default:
			err = s.extensions.RecordPickupWeekdayExtension(ctx, ports.PickupWeekdayExtension{
				StudentID:      studentID,
				Weekday:        weekday,
				EffectiveFrom:  date.String(),
				PreviousPickup: previous,
				Pickup:         current,
			})
		}
		if err != nil {
			return fmt.Errorf("pickup extension: weekday %d: %w", weekday, err)
		}
	}
	return nil
}
