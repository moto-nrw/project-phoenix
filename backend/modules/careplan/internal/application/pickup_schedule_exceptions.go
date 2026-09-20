package application

import (
	"context"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *pickupScheduleService) GetStudentPickupExceptionByID(
	ctx context.Context,
	exceptionID int64,
) (*careplan.PickupException, error) {
	return s.exceptionByID(ctx, exceptionID)
}

func (s *pickupScheduleService) GetStudentPickupExceptionForDate(
	ctx context.Context,
	studentID int64,
	date calendar.Date,
) (*careplan.PickupException, error) {
	return s.exceptionForDate(ctx, studentID, date)
}

func (s *pickupScheduleService) GetStudentPickupExceptions(
	ctx context.Context,
	studentID int64,
) ([]*careplan.PickupException, error) {
	return s.loadExceptions(ctx, studentID)
}

func (s *pickupScheduleService) GetUpcomingStudentPickupExceptions(
	ctx context.Context,
	studentID int64,
) ([]*careplan.PickupException, error) {
	return s.upcomingExceptions(ctx, studentID)
}

func (s *pickupScheduleService) CreateStudentPickupException(
	ctx context.Context,
	row *careplan.PickupException,
) error {
	return s.createException(ctx, row)
}

func (s *pickupScheduleService) UpdateStudentPickupException(
	ctx context.Context,
	row *careplan.PickupException,
) error {
	return s.updateExceptionRow(ctx, row)
}

func (s *pickupScheduleService) CreateOrReclaimException(
	ctx context.Context,
	studentID int64,
	date calendar.Date,
	pickupTime *time.Time,
	reason *string,
	staffID int64,
	resolveStaffID func() (int64, error),
) (*careplan.PickupException, error) {
	if s.autoExcusal == nil {
		return s.createOrReclaimException(
			ctx,
			studentID,
			date,
			pickupTime,
			reason,
			staffID,
			resolveStaffID,
		)
	}
	var result *careplan.PickupException
	err := s.tx.WithinTenant(ctx, func(txCtx context.Context) error {
		if err := s.tx.LockStudentAndExceptionDay(txCtx, studentID, date.String()); err != nil {
			return err
		}
		// An existing auto excusal is detached first so the overwrite cannot
		// strand block absences whose provenance the write replaces; the sync
		// below re-derives the excusal from the new pickup time.
		if err := s.autoExcusal.DetachForDate(txCtx, studentID, date); err != nil {
			return err
		}
		row, err := s.createOrReclaimException(txCtx, studentID, date, pickupTime, reason, staffID, resolveStaffID)
		if err != nil {
			return err
		}
		result = row
		return s.resyncAutoExcusal(txCtx, row.ID, &result)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// resyncAutoExcusal runs the auto-excusal sync for the written exception and
// refreshes *result so callers respond with the synced row state.
func (s *pickupScheduleService) resyncAutoExcusal(
	ctx context.Context,
	exceptionID int64,
	result **careplan.PickupException,
) error {
	changed, err := s.autoExcusal.Sync(ctx, exceptionID)
	if err != nil {
		return err
	}
	// Only a sync that actually rewrote the row warrants replacing the
	// caller-visible result with a re-read — the common no-op path keeps the
	// core's in-memory row (and its nil-reason semantics) untouched.
	if !changed {
		return nil
	}
	fresh, err := s.exceptionByID(ctx, exceptionID)
	if err != nil {
		return err
	}
	if fresh != nil {
		*result = fresh
	}
	return nil
}

func (s *pickupScheduleService) UpdateException(
	ctx context.Context,
	exceptionID int64,
	studentID int64,
	date calendar.Date,
	reason *string,
	pickupTime *time.Time,
	clearPickupTime bool,
	resolveStaffID func() (int64, error),
) (*careplan.PickupException, error) {
	if s.autoExcusal == nil {
		return s.updateException(
			ctx,
			exceptionID,
			studentID,
			date,
			reason,
			pickupTime,
			clearPickupTime,
			resolveStaffID,
		)
	}
	var result *careplan.PickupException
	err := s.tx.WithinTenant(ctx, func(txCtx context.Context) error {
		if err := s.lockPickupUpdateDays(txCtx, exceptionID, studentID, date); err != nil {
			return err
		}
		fresh, err := s.exceptionByID(txCtx, exceptionID)
		if err != nil {
			return err
		}
		if err := s.autoExcusal.DetachRow(txCtx, fresh); err != nil {
			return err
		}
		row, err := s.updateException(txCtx, exceptionID, studentID, date, reason, pickupTime, clearPickupTime, resolveStaffID)
		if err != nil {
			return err
		}
		result = row
		return s.resyncAutoExcusal(txCtx, row.ID, &result)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *pickupScheduleService) DeleteStudentPickupException(
	ctx context.Context,
	exceptionID, studentID int64,
) error {
	if s.autoExcusal == nil {
		return s.deleteException(ctx, exceptionID, studentID)
	}
	return s.tx.WithinTenant(ctx, func(txCtx context.Context) error {
		initial, err := s.exceptionByID(txCtx, exceptionID)
		if err != nil && !s.tx.IsNotFound(err) {
			return err
		}
		if initial != nil {
			if err := s.detachExceptionForStudent(txCtx, initial, studentID); err != nil {
				return err
			}
		}
		return s.deleteException(txCtx, exceptionID, studentID)
	})
}

func (s *pickupScheduleService) detachExceptionForStudent(ctx context.Context, initial *careplan.PickupException, studentID int64) error {
	if initial.StudentID != studentID {
		return careplan.ErrCareExceptionWrongStudent
	}
	if err := s.tx.LockStudentAndExceptionDay(ctx, studentID, initial.ExceptionDate.String()); err != nil {
		return err
	}
	// Re-read under the lock before restoring the auto-excused blocks.
	// Detachment must precede deletion so the FK cannot erase provenance.
	fresh, err := s.exceptionByID(ctx, initial.ID)
	if err != nil {
		return err
	}
	if fresh != nil && fresh.StudentID != studentID {
		return careplan.ErrCareExceptionWrongStudent
	}
	return s.autoExcusal.DetachRow(ctx, fresh)
}

func (s *pickupScheduleService) DeleteAllStudentPickupExceptions(
	ctx context.Context,
	studentID int64,
) error {
	if s.autoExcusal == nil {
		return s.deleteAllExceptions(ctx, studentID)
	}
	return s.tx.WithinTenant(ctx, func(txCtx context.Context) error {
		// Student lock FIRST: every care-day writer takes it before its day
		// lock, so once held no concurrent exception write can commit between
		// the snapshot below and the delete — an auto excusal created in that
		// window would otherwise be deleted without release, stranding its
		// blocks as absent once the FK clears their provenance (#2360 review).
		// A missing student cannot race those writers (they fail on the same
		// lock), so the plain delete-all suffices then.
		if err := s.tx.LockStudent(txCtx, studentID); err != nil {
			if s.tx.IsNotFound(err) {
				return s.deleteAllExceptions(txCtx, studentID)
			}
			return err
		}
		if err := s.detachStudentAutoExceptions(txCtx, studentID); err != nil {
			return err
		}
		return s.deleteAllExceptions(txCtx, studentID)
	})
}

func (s *pickupScheduleService) lockPickupUpdateDays(ctx context.Context, exceptionID, studentID int64, date calendar.Date) error {
	// The row's stored date may differ from the submitted one; both days'
	// block absences can be affected, so lock them in ascending order (the
	// same convention DeleteAllExceptions uses) before detaching.
	existing, err := s.exceptionByID(ctx, exceptionID)
	if err != nil && !s.tx.IsNotFound(err) {
		return err
	}
	lockDates := []calendar.Date{date}
	if existing != nil && calendar.Date(existing.ExceptionDate) != date {
		lockDates = append(lockDates, calendar.Date(existing.ExceptionDate))
		sort.Slice(lockDates, func(i, j int) bool { return lockDates[i].Before(lockDates[j]) })
	}
	for _, lockDate := range lockDates {
		if err := s.tx.LockStudentAndExceptionDay(ctx, studentID, lockDate.String()); err != nil {
			return err
		}
	}
	return nil
}

func (s *pickupScheduleService) detachStudentAutoExceptions(ctx context.Context, studentID int64) error {
	rows, err := s.loadExceptions(ctx, studentID)
	if err != nil {
		return err
	}
	autoRows := make([]*careplan.PickupException, 0, len(rows))
	for _, row := range rows {
		if row != nil && row.ExcusedAuto {
			autoRows = append(autoRows, row)
		}
	}
	sort.Slice(autoRows, func(i, j int) bool {
		if autoRows[i].ExceptionDate == autoRows[j].ExceptionDate {
			return autoRows[i].ID < autoRows[j].ID
		}
		return autoRows[i].ExceptionDate.Before(autoRows[j].ExceptionDate)
	})
	for _, row := range autoRows {
		if err := s.tx.LockStudentAndExceptionDay(ctx, studentID, row.ExceptionDate.String()); err != nil {
			return err
		}
		fresh, err := s.exceptionByID(ctx, row.ID)
		if err != nil {
			return err
		}
		if err := s.autoExcusal.DetachRow(ctx, fresh); err != nil {
			return err
		}
	}
	return nil
}
