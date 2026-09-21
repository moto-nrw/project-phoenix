package presence

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CreateForDates records the reported status for one student across the given
// dates inside a tenant transaction: locked-row re-authorization, rejecting
// active conflicts without partial writes, upserting the reported rows,
// mutating today's live sick/excused flags, and registering the after-commit
// broadcast.
func (s *StudentStatusDayService) CreateForDates(ctx context.Context, wc StatusDayWriteContext, studentID int64, status, reason string, dates []timezone.Date) error {
	if len(dates) == 0 {
		return errors.New("student status day dates are required")
	}
	now := s.now()
	notePtr := statusDayNote(reason)
	return tenant.WithTenantTx(ctx, wc.DB, wc.TenantID, func(ctx context.Context, _ any) error {
		return s.createForDatesLocked(ctx, wc, studentID, status, dates, notePtr, now)
	})
}

// statusDayNote is the trimmed reason, or nil when it is blank.
func statusDayNote(reason string) *string {
	if note := strings.TrimSpace(reason); note != "" {
		return &note
	}
	return nil
}

func (s *StudentStatusDayService) createForDatesLocked(ctx context.Context, wc StatusDayWriteContext, studentID int64, status string, dates []timezone.Date, notePtr *string, now time.Time) error {
	fresh, err := wc.StudentService.LockForStatusWrite(ctx, studentID, status)
	if err != nil {
		return err
	}
	if err := s.lockAndCheckStatusDates(ctx, studentID, dates); err != nil {
		return err
	}

	conflicts, err := s.findRequestedActiveConflicts(ctx, studentID, dates)
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		return &StudentStatusDayConflictError{Conflicts: conflicts}
	}

	notifyAbsence, err := s.writeStatusForStudent(ctx, wc, fresh, status, dates, notePtr, now, timezone.DateFromTime(now))
	if err != nil {
		return err
	}
	tenant.RegisterAfterCommit(ctx, func() { wc.AfterCommit(studentID) })
	if notifyAbsence && wc.AfterCreate != nil {
		return wc.AfterCreate(ctx, []int64{studentID})
	}
	return nil
}

// BulkCreateForDates applies CreateForDates' orchestration to several students
// within a single tenant transaction. Authorization and conflict preflight run
// for the full set before any writes so a mixed-scope or conflicting selection
// cannot partially commit or overwrite existing status rows.
func (s *StudentStatusDayService) BulkCreateForDates(ctx context.Context, wc StatusDayWriteContext, studentIDs []int64, status, reason string, dates []timezone.Date) error {
	if len(dates) == 0 {
		return errors.New("student status day dates are required")
	}
	studentIDs = dedupeStudentIDs(studentIDs)
	now := s.now()
	notePtr := statusDayNote(reason)
	return tenant.WithTenantTx(ctx, wc.DB, wc.TenantID, func(ctx context.Context, _ any) error {
		// Phase 1: lock and authorize every student before writing any row.
		// Order by ID for stable lock acquisition across concurrent bulk ops.
		sortedIDs := append([]int64(nil), studentIDs...)
		slices.Sort(sortedIDs)
		lockedStudents, err := s.lockBulkStatusStudents(ctx, wc, sortedIDs, status, dates)
		if err != nil {
			return err
		}

		// Phase 2: preflight every student/date pair so no existing sick,
		// excused, or class-trip row is cleared or overwritten.
		if err := s.rejectBulkActiveConflicts(ctx, sortedIDs, dates); err != nil {
			return err
		}

		// Phase 3: write only after the full selection is in scope and clear.
		return s.writeBulkStatuses(ctx, wc, studentIDs, lockedStudents, status, dates, notePtr, now)
	})
}

// lockBulkStatusStudents locks and re-authorizes every student, then locks
// their requested dates and rejects dates a manual partial absence holds.
func (s *StudentStatusDayService) lockBulkStatusStudents(ctx context.Context, wc StatusDayWriteContext, sortedIDs []int64, status string, dates []timezone.Date) (map[int64]*StudentRecord, error) {
	lockedStudents := make(map[int64]*StudentRecord, len(sortedIDs))
	for _, studentID := range sortedIDs {
		fresh, err := wc.StudentService.LockForStatusWrite(ctx, studentID, status)
		if err != nil {
			return nil, err
		}
		lockedStudents[studentID] = fresh
	}
	for _, studentID := range sortedIDs {
		if err := s.lockAndCheckStatusDates(ctx, studentID, dates); err != nil {
			return nil, err
		}
	}
	return lockedStudents, nil
}

// rejectBulkActiveConflicts fails the bulk write when any requested date
// already has an active status. It keeps only a sample of rows for the 409
// body; Total carries the full count.
func (s *StudentStatusDayService) rejectBulkActiveConflicts(ctx context.Context, sortedIDs []int64, dates []timezone.Date) error {
	conflictSamples := make([]*absencerecords.StudentStatusDay, 0, MaxStudentStatusDayConflictDetails)
	conflictTotal := 0
	for _, studentID := range sortedIDs {
		studentConflicts, err := s.findRequestedActiveConflicts(ctx, studentID, dates)
		if err != nil {
			return err
		}
		conflictTotal += len(studentConflicts)
		remaining := MaxStudentStatusDayConflictDetails - len(conflictSamples)
		if remaining <= 0 {
			continue
		}
		conflictSamples = append(conflictSamples, studentConflicts[:min(len(studentConflicts), remaining)]...)
	}
	if conflictTotal > 0 {
		return &StudentStatusDayConflictError{
			Conflicts: conflictSamples,
			Total:     conflictTotal,
		}
	}
	return nil
}

// writeBulkStatuses writes each student's status in request order, queues
// their after-commit broadcasts and notifies the absence recipients once.
func (s *StudentStatusDayService) writeBulkStatuses(
	ctx context.Context,
	wc StatusDayWriteContext,
	studentIDs []int64,
	lockedStudents map[int64]*StudentRecord,
	status string,
	dates []timezone.Date,
	notePtr *string,
	now time.Time,
) error {
	today := timezone.DateFromTime(now)
	absenceStudentIDs := make([]int64, 0, len(studentIDs))
	for _, studentID := range studentIDs {
		notifyAbsence, err := s.writeStatusForStudent(ctx, wc, lockedStudents[studentID], status, dates, notePtr, now, today)
		if err != nil {
			return err
		}
		if notifyAbsence {
			absenceStudentIDs = append(absenceStudentIDs, studentID)
		}
		tenant.RegisterAfterCommit(ctx, func() { wc.AfterCommit(studentID) })
	}
	if len(absenceStudentIDs) > 0 && wc.AfterCreate != nil {
		return wc.AfterCreate(ctx, absenceStudentIDs)
	}
	return nil
}

// lockAndCheckStatusDates locks the student's requested dates and rejects a
// date a manual partial absence already holds.
func (s *StudentStatusDayService) lockAndCheckStatusDates(ctx context.Context, studentID int64, dates []timezone.Date) error {
	if err := s.lockStudentStatusDates(ctx, studentID, dates); err != nil {
		return err
	}
	return s.ensureNoPartialAbsenceConflicts(ctx, studentID, dates)
}

func (s *StudentStatusDayService) lockStudentStatusDates(ctx context.Context, studentID int64, dates []timezone.Date) error {
	sortedDates := append([]timezone.Date(nil), dates...)
	slices.SortFunc(sortedDates, timezone.Date.Compare)
	for _, date := range sortedDates {
		if err := s.lockStatusDate(ctx, studentID, date.String()); err != nil {
			return err
		}
	}
	return nil
}

func (s *StudentStatusDayService) ensureNoPartialAbsenceConflicts(
	ctx context.Context, studentID int64, dates []timezone.Date,
) error {
	if s.pickupExceptions == nil || len(dates) == 0 {
		return nil
	}
	requested := make(map[timezone.Date]struct{}, len(dates))
	for _, date := range dates {
		requested[date] = struct{}{}
	}
	rows, err := s.pickupExceptions.ManualPartialAbsenceDates(
		ctx,
		studentID,
		slices.MinFunc(dates, timezone.Date.Compare),
		slices.MaxFunc(dates, timezone.Date.Compare),
	)
	if err != nil {
		return err
	}
	for _, row := range rows {
		// Auto-derived excusals (pulled-forward pickup time, #2360) do not
		// block a broad day status: the two coexist via disjoint slot
		// ownership, and refusing would make every pickup change block a sick
		// report. Only a staff-set manual partial absence conflicts.
		if _, matches := requested[row]; matches {
			return ErrStudentStatusDayPartialAbsenceConflict
		}
	}
	return nil
}

// findRequestedActiveConflicts returns active status-day rows that already
// cover any of the requested dates for one student. Callers must reject the
// whole write when this list is non-empty.
func (s *StudentStatusDayService) findRequestedActiveConflicts(ctx context.Context, studentID int64, dates []timezone.Date) ([]*absencerecords.StudentStatusDay, error) {
	if len(dates) == 0 {
		return nil, nil
	}
	requestedDates := make(map[timezone.Date]struct{}, len(dates))
	for _, date := range dates {
		requestedDates[date] = struct{}{}
	}
	activeRows, err := s.repo.FindActiveByStudentAndDateRange(
		ctx,
		studentID,
		slices.MinFunc(dates, timezone.Date.Compare),
		slices.MaxFunc(dates, timezone.Date.Compare),
	)
	if err != nil {
		return nil, err
	}
	conflicts := make([]*absencerecords.StudentStatusDay, 0, len(activeRows))
	for _, row := range activeRows {
		if _, requested := requestedDates[row.Date]; requested {
			conflicts = append(conflicts, row)
		}
	}
	return conflicts, nil
}

func dedupeStudentIDs(studentIDs []int64) []int64 {
	seen := make(map[int64]struct{}, len(studentIDs))
	unique := make([]int64, 0, len(studentIDs))
	for _, studentID := range studentIDs {
		if _, exists := seen[studentID]; exists {
			continue
		}
		seen[studentID] = struct{}{}
		unique = append(unique, studentID)
	}
	return unique
}

// DeleteByID clears a single status-day row after ownership and locked-row
// re-authorization checks, resetting today's live flags when the row is today.
func (s *StudentStatusDayService) DeleteByID(ctx context.Context, wc StatusDayWriteContext, statusDayID, studentID int64) error {
	now := s.now()
	today := timezone.DateFromTime(now)
	return tenant.WithTenantTx(ctx, wc.DB, wc.TenantID, func(ctx context.Context, _ any) error {
		row, err := s.repo.FindActiveByID(ctx, statusDayID)
		if err != nil {
			return err
		}
		if row.StudentID != studentID {
			return base.ErrNotFound
		}

		fresh, err := wc.StudentService.LockForStatusWrite(ctx, studentID, row.Status)
		if err != nil {
			return err
		}

		if err := s.repo.MarkClearedByID(ctx, row.ID, now, absencerecords.StudentStatusSourceManual); err != nil {
			return err
		}
		if row.Date == today {
			studentpresence.ClearLiveStatusForToday(fresh, row.Status)
			if err := wc.StudentService.UpdateLiveStatus(ctx, fresh); err != nil {
				return err
			}
		}

		tenant.RegisterAfterCommit(ctx, func() { wc.AfterCommit(studentID) })
		return nil
	})
}

func (s *StudentStatusDayService) writeStatusForStudent(ctx context.Context, wc StatusDayWriteContext, fresh *StudentRecord, status string, dates []timezone.Date, notePtr *string, now time.Time, today timezone.Date) (bool, error) {
	notifyAbsence := isNewReportableAbsence(fresh, status, dates, today)
	if err := s.clearOtherStatusDaysForDates(ctx, fresh.ID, status, dates, now); err != nil {
		return false, err
	}
	for _, date := range dates {
		if err := s.repo.UpsertReported(ctx, &absencerecords.StudentStatusDay{
			StudentID:  fresh.ID,
			Date:       date,
			Status:     status,
			ReportedAt: now,
			Source:     absencerecords.StudentStatusSourcePlanned,
			Note:       notePtr,
		}); err != nil {
			return false, err
		}
	}
	if slices.Contains(dates, today) {
		studentpresence.ApplyLiveStatusForToday(fresh, status, now)
		if err := wc.StudentService.UpdateLiveStatus(ctx, fresh); err != nil {
			return false, err
		}
	}
	return notifyAbsence, nil
}

func isNewReportableAbsence(student *StudentRecord, status string, dates []timezone.Date, today timezone.Date) bool {
	if !slices.Contains(dates, today) {
		return false
	}
	switch status {
	case absencerecords.StudentStatusDaySick:
		return student.Sick == nil || !*student.Sick
	case absencerecords.StudentStatusDayExcused:
		return student.Excused == nil || !*student.Excused
	default:
		return false
	}
}

func (s *StudentStatusDayService) clearOtherStatusDaysForDates(ctx context.Context, studentID int64, status string, dates []timezone.Date, now time.Time) error {
	for _, otherStatus := range absencerecords.StudentStatusDayStatusesExcept(status) {
		if err := s.repo.MarkClearedForDates(ctx, studentID, otherStatus, dates, now, absencerecords.StudentStatusSourceManual); err != nil {
			return err
		}
	}
	return nil
}
