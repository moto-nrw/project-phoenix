package schedule

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

var (
	ErrPartialAbsenceAlreadyExists   = errors.New("partial absence already exists for this date")
	ErrPartialAbsenceFullDayConflict = errors.New("partial absence conflicts with a full-day status")
	ErrPartialAbsencePickupConflict  = errors.New("partial absence conflicts with a full-day pickup cancellation")
	ErrPartialAbsenceNotFound        = errors.New("partial absence not found")
	ErrPartialAbsenceWrongStudent    = errors.New("partial absence belongs to another student")
)

// PartialAbsenceInput is the one-day command accepted by the partial-absence
// module. FromTime is a wall-clock value; Date is a calendar date.
type PartialAbsenceInput struct {
	StudentID int64
	Date      timezone.Date
	FromTime  time.Time
	Reason    string
	StaffID   int64
}

// PartialAbsenceService coordinates the existing pickup-exception and
// per-block attendance sources. It deliberately stores no third schedule.
type PartialAbsenceService interface {
	List(ctx context.Context, studentID int64, from, to timezone.Date) ([]*scheduleModel.StudentPickupException, error)
	Create(ctx context.Context, input PartialAbsenceInput) (*scheduleModel.StudentPickupException, error)
	Update(ctx context.Context, exceptionID int64, input PartialAbsenceInput) (*scheduleModel.StudentPickupException, error)
	Delete(ctx context.Context, exceptionID, studentID int64) error
}

type partialAbsenceService struct {
	pickups    scheduleModel.StudentPickupExceptionRepository
	statusDays activeModel.StudentStatusDayRepository
	slots      scheduleModel.InstanceStudentRepository
	db         *bun.DB
}

func NewPartialAbsenceService(
	pickups scheduleModel.StudentPickupExceptionRepository,
	statusDays activeModel.StudentStatusDayRepository,
	slots scheduleModel.InstanceStudentRepository,
	db *bun.DB,
) PartialAbsenceService {
	return &partialAbsenceService{
		pickups:    pickups,
		statusDays: statusDays,
		slots:      slots,
		db:         db,
	}
}

func (s *partialAbsenceService) List(
	ctx context.Context, studentID int64, from, to timezone.Date,
) ([]*scheduleModel.StudentPickupException, error) {
	rows, err := s.pickups.FindByStudentIDAndDateRange(ctx, studentID, from, to)
	if err != nil {
		return nil, err
	}
	partial := make([]*scheduleModel.StudentPickupException, 0, len(rows))
	for _, row := range rows {
		if row != nil && row.ExcusedFrom != nil {
			partial = append(partial, row)
		}
	}
	return partial, nil
}

func (s *partialAbsenceService) Create(
	ctx context.Context, input PartialAbsenceInput,
) (*scheduleModel.StudentPickupException, error) {
	if err := validatePartialAbsenceInput(input); err != nil {
		return nil, err
	}

	var result *scheduleModel.StudentPickupException
	err := tenant.WithTenantTx(ctx, s.db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
		// Student row then care-day — same order as full-day status and
		// pickup/arrival exception writers (LockCareExceptionDay).
		if err := LockCareExceptionDay(txCtx, s.db, input.StudentID, input.Date); err != nil {
			return err
		}
		if err := s.ensureNoFullDayStatus(txCtx, input.StudentID, input.Date); err != nil {
			return err
		}

		existing, err := s.pickups.FindByStudentIDAndDate(txCtx, input.StudentID, input.Date)
		if err != nil {
			return err
		}
		clock := timezone.WallClock(input.FromTime)
		staffID := input.StaffID
		reason := trimmedReason(input.Reason)

		if existing != nil {
			if existing.ExcusedFrom != nil {
				return ErrPartialAbsenceAlreadyExists
			}
			if existing.PickupTime == nil {
				return ErrPartialAbsencePickupConflict
			}
			existing.ExcusedFrom = &clock
			existing.ExcusedReason = reason
			existing.ExcusedCreatedBy = &staffID
			existing.ExcusedOwnsPickupTime = false
			normalizePickupExceptionTimes(existing)
			if err := s.pickups.Update(txCtx, existing); err != nil {
				return err
			}
			result = existing
		} else {
			result = &scheduleModel.StudentPickupException{
				StudentID:             input.StudentID,
				ExceptionDate:         input.Date,
				PickupTime:            &clock,
				ExcusedFrom:           &clock,
				ExcusedReason:         reason,
				ExcusedCreatedBy:      &staffID,
				ExcusedOwnsPickupTime: true,
				Source:                scheduleModel.ExceptionSourceStaff,
				CreatedBy:             input.StaffID,
			}
			result.SetTenantID(tenant.FromContext(txCtx))
			if err := s.pickups.Create(txCtx, result); err != nil {
				return err
			}
		}

		_, err = s.slots.ApplyPartialAbsence(txCtx, result.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *partialAbsenceService) Update(
	ctx context.Context, exceptionID int64, input PartialAbsenceInput,
) (*scheduleModel.StudentPickupException, error) {
	if err := validatePartialAbsenceInput(input); err != nil {
		return nil, err
	}

	var result *scheduleModel.StudentPickupException
	err := tenant.WithTenantTx(ctx, s.db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
		if err := LockCareExceptionDay(txCtx, s.db, input.StudentID, input.Date); err != nil {
			return err
		}
		if err := s.ensureNoFullDayStatus(txCtx, input.StudentID, input.Date); err != nil {
			return err
		}

		row, err := s.pickups.FindByID(txCtx, exceptionID)
		if err != nil || row == nil {
			if err != nil {
				return err
			}
			return ErrPartialAbsenceNotFound
		}
		if row.StudentID != input.StudentID || row.ExceptionDate != input.Date {
			return ErrPartialAbsenceWrongStudent
		}
		if row.ExcusedFrom == nil {
			return ErrPartialAbsenceNotFound
		}
		if _, err := s.slots.ReleasePartialAbsence(txCtx, row.ID); err != nil {
			return err
		}

		clock := timezone.WallClock(input.FromTime)
		if row.ExcusedOwnsPickupTime {
			row.PickupTime = &clock
		}
		staffID := input.StaffID
		row.ExcusedFrom = &clock
		row.ExcusedReason = trimmedReason(input.Reason)
		row.ExcusedCreatedBy = &staffID
		normalizePickupExceptionTimes(row)
		if err := s.pickups.Update(txCtx, row); err != nil {
			return err
		}
		if _, err := s.slots.ApplyPartialAbsence(txCtx, row.ID); err != nil {
			return err
		}
		result = row
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *partialAbsenceService) Delete(ctx context.Context, exceptionID, studentID int64) error {
	return tenant.WithTenantTx(ctx, s.db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
		// Resolve the date first (unlocked) so the student→care-day lock order
		// matches Create/Update. Re-read under the locks before mutating.
		row, err := s.pickups.FindByID(txCtx, exceptionID)
		if err != nil || row == nil {
			if err != nil {
				return err
			}
			return ErrPartialAbsenceNotFound
		}
		if row.StudentID != studentID {
			return ErrPartialAbsenceWrongStudent
		}
		if err := LockCareExceptionDay(txCtx, s.db, studentID, row.ExceptionDate); err != nil {
			return err
		}
		row, err = s.pickups.FindByID(txCtx, exceptionID)
		if err != nil || row == nil {
			if err != nil {
				return err
			}
			return ErrPartialAbsenceNotFound
		}
		if row.StudentID != studentID {
			return ErrPartialAbsenceWrongStudent
		}
		if row.ExcusedFrom == nil {
			return ErrPartialAbsenceNotFound
		}
		if _, err := s.slots.ReleasePartialAbsence(txCtx, row.ID); err != nil {
			return err
		}
		if row.ExcusedOwnsPickupTime {
			return s.pickups.Delete(txCtx, row.ID)
		}
		row.ExcusedFrom = nil
		row.ExcusedReason = nil
		row.ExcusedCreatedBy = nil
		row.ExcusedOwnsPickupTime = false
		normalizePickupExceptionTimes(row)
		return s.pickups.Update(txCtx, row)
	})
}

func normalizePickupExceptionTimes(row *scheduleModel.StudentPickupException) {
	if row.PickupTime != nil {
		clock := timezone.WallClock(*row.PickupTime)
		row.PickupTime = &clock
	}
	if row.ExcusedFrom != nil {
		clock := timezone.WallClock(*row.ExcusedFrom)
		row.ExcusedFrom = &clock
	}
}

func (s *partialAbsenceService) ensureNoFullDayStatus(
	ctx context.Context, studentID int64, date timezone.Date,
) error {
	rows, err := s.statusDays.FindActiveByStudentAndDateRange(ctx, studentID, date, date)
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		return ErrPartialAbsenceFullDayConflict
	}
	return nil
}

func validatePartialAbsenceInput(input PartialAbsenceInput) error {
	if input.StudentID <= 0 {
		return errors.New("student_id is required")
	}
	if input.Date.IsZero() {
		return errors.New("date is required")
	}
	if input.FromTime.IsZero() {
		return errors.New("from_time is required")
	}
	if input.StaffID <= 0 {
		return errors.New("staff_id is required")
	}
	if len(strings.TrimSpace(input.Reason)) > 255 {
		return errors.New("reason cannot exceed 255 characters")
	}
	return nil
}

func trimmedReason(reason string) *string {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
