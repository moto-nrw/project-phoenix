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
	// ErrPartialAbsencePendingRequestConflict refuses a partial-day excusal when
	// a parent full-day excused request is still pending for that date. Approval
	// would later hit ensureNoPartialAbsence and leave the request unapprovable.
	ErrPartialAbsencePendingRequestConflict = errors.New("partial absence conflicts with a pending full-day excused request")
	ErrPartialAbsencePickupConflict         = errors.New("partial absence conflicts with a full-day pickup cancellation")
	ErrPartialAbsenceNotFound               = errors.New("partial absence not found")
	ErrPartialAbsenceWrongStudent           = errors.New("partial absence belongs to another student")
	// ErrPartialAbsenceAutoManaged refuses deleting an auto-derived excusal
	// through the manual endpoints: it would be re-derived on the next pickup
	// write anyway — undoing it means changing or removing the day's pickup
	// time (#2360).
	ErrPartialAbsenceAutoManaged = errors.New("partial absence is derived from the day's pickup time")
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
	requests   activeModel.ExcusedAbsenceRequestRepository
	slots      scheduleModel.InstanceStudentRepository
	syncer     *PickupAutoExcusalSyncer
	db         *bun.DB
}

// NewPartialAbsenceService wires partial-day excusal writes. requests may be
// nil in tests that do not exercise the pending full-day request guard;
// syncer may be nil in tests that do not exercise auto re-derivation after
// removing a manual override.
func NewPartialAbsenceService(
	pickups scheduleModel.StudentPickupExceptionRepository,
	statusDays activeModel.StudentStatusDayRepository,
	requests activeModel.ExcusedAbsenceRequestRepository,
	slots scheduleModel.InstanceStudentRepository,
	syncer *PickupAutoExcusalSyncer,
	db *bun.DB,
) PartialAbsenceService {
	return &partialAbsenceService{
		pickups:    pickups,
		statusDays: statusDays,
		requests:   requests,
		slots:      slots,
		syncer:     syncer,
		db:         db,
	}
}

func (s *partialAbsenceService) List(
	ctx context.Context, studentID int64, from, to timezone.Date,
) ([]*scheduleModel.StudentPickupException, error) {
	rows, err := s.pickups.FindByStudentIDAndDateRange(ctx, studentID, scheduleModel.Date(from), scheduleModel.Date(to))
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
		if err := s.ensureNoPendingExcusedRequest(txCtx, input.StudentID, input.Date); err != nil {
			return err
		}

		existing, err := s.pickups.FindByStudentIDAndDate(txCtx, input.StudentID, scheduleModel.Date(input.Date))
		if err != nil {
			return err
		}
		clock := timezone.NormalizeWallClock(input.FromTime)
		staffID := input.StaffID
		reason := trimmedReason(input.Reason)

		if existing != nil {
			if existing.HasManualPartialAbsence() {
				return ErrPartialAbsenceAlreadyExists
			}
			if existing.PickupTime == nil {
				return ErrPartialAbsencePickupConflict
			}
			// An auto-derived excusal (#2360) yields to the explicit staff
			// decision: release its block absences, then take over as manual.
			if existing.ExcusedAuto {
				if _, err := s.slots.ReleasePartialAbsence(txCtx, existing.ID); err != nil {
					return err
				}
				existing.ExcusedAuto = false
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
				ExceptionDate:         scheduleModel.Date(input.Date),
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
		if err := s.ensureNoPendingExcusedRequest(txCtx, input.StudentID, input.Date); err != nil {
			return err
		}

		row, err := s.pickups.FindByID(txCtx, exceptionID)
		if err != nil || row == nil {
			if err != nil {
				return err
			}
			return ErrPartialAbsenceNotFound
		}
		if row.StudentID != input.StudentID || row.ExceptionDate != scheduleModel.Date(input.Date) {
			return ErrPartialAbsenceWrongStudent
		}
		if row.ExcusedFrom == nil {
			return ErrPartialAbsenceNotFound
		}
		if _, err := s.slots.ReleasePartialAbsence(txCtx, row.ID); err != nil {
			return err
		}

		clock := timezone.NormalizeWallClock(input.FromTime)
		if row.ExcusedOwnsPickupTime {
			row.PickupTime = &clock
		}
		staffID := input.StaffID
		row.ExcusedFrom = &clock
		row.ExcusedReason = trimmedReason(input.Reason)
		row.ExcusedCreatedBy = &staffID
		// Editing an auto-derived excusal converts it into a manual one — the
		// staff time now wins over the pickup-time derivation (#2360).
		row.ExcusedAuto = false
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
		if err := LockCareExceptionDay(txCtx, s.db, studentID, timezone.Date(row.ExceptionDate)); err != nil {
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
		if row.ExcusedAuto {
			return ErrPartialAbsenceAutoManaged
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
		if err := s.pickups.Update(txCtx, row); err != nil {
			return err
		}
		// The day's pickup time survives this delete (the exception owned it
		// before the manual override existed, or the override converted an
		// auto excusal). With the manual decision gone, the pull-forward rule
		// applies again — re-derive instead of leaving later blocks expected
		// while the child is still picked up early (#2360).
		if s.syncer != nil {
			if _, err := s.syncer.Sync(txCtx, row.ID); err != nil {
				return err
			}
		}
		return nil
	})
}

func normalizePickupExceptionTimes(row *scheduleModel.StudentPickupException) {
	row.NormalizeWallClockTimes()
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

// ensureNoPendingExcusedRequest keeps partial-day writes mutually exclusive
// with open full-day parent requests for the same child and date.
func (s *partialAbsenceService) ensureNoPendingExcusedRequest(
	ctx context.Context, studentID int64, date timezone.Date,
) error {
	if s.requests == nil {
		return nil
	}
	pending, err := s.requests.ListPendingForStudent(ctx, studentID)
	if err != nil {
		return err
	}
	for _, req := range pending {
		if req == nil {
			continue
		}
		for _, requested := range req.Dates {
			if requested == date {
				return ErrPartialAbsencePendingRequestConflict
			}
		}
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
