package application

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type partialAbsenceService struct {
	pickups   ports.PartialAbsenceStore
	conflicts ports.PartialAbsenceConflicts
	slots     ports.PartialAbsenceBlocks
	syncer    ports.PartialAbsenceSyncer
	tx        ports.EffectiveTimeTransaction
}

func NewPartialAbsences(pickups ports.PartialAbsenceStore, conflicts ports.PartialAbsenceConflicts,
	slots ports.PartialAbsenceBlocks, syncer ports.PartialAbsenceSyncer, tx ports.EffectiveTimeTransaction,
) careplan.PartialAbsenceService {
	return &partialAbsenceService{pickups: pickups, conflicts: conflicts, slots: slots, syncer: syncer, tx: tx}
}

func (s *partialAbsenceService) ListPartialAbsences(
	ctx context.Context, studentID int64, from, to calendar.Date,
) ([]*careplan.PickupException, error) {
	rows, err := s.pickups.FindByStudentIDAndDateRange(ctx, studentID, from, to)
	if err != nil {
		return nil, err
	}
	partial := make([]*careplan.PickupException, 0, len(rows))
	for _, row := range rows {
		if row != nil && row.ExcusedFrom != nil {
			partial = append(partial, row)
		}
	}
	return partial, nil
}

func (s *partialAbsenceService) CreatePartialAbsence(ctx context.Context, input careplan.PartialAbsenceInput) (*careplan.PickupException, error) {
	if err := validatePartialAbsenceInput(input); err != nil {
		return nil, err
	}
	var result *careplan.PickupException
	err := s.tx.WithinTenant(ctx, func(txCtx context.Context) error {
		if err := s.lockWritableDay(txCtx, input); err != nil {
			return err
		}
		var err error
		result, err = s.createOrClaim(txCtx, input)
		if err != nil {
			return err
		}
		_, err = s.slots.ApplyPartialAbsence(txCtx, result.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *partialAbsenceService) lockWritableDay(ctx context.Context, input careplan.PartialAbsenceInput) error {
	// The student row precedes the care-day lock, as in full-day status writes.
	if err := s.tx.LockStudentAndExceptionDay(ctx, input.StudentID, input.Date.String()); err != nil {
		return err
	}
	if err := s.ensureNoFullDayStatus(ctx, input.StudentID, input.Date); err != nil {
		return err
	}
	return s.ensureNoPendingExcusedRequest(ctx, input.StudentID, input.Date)
}

func (s *partialAbsenceService) createOrClaim(ctx context.Context, input careplan.PartialAbsenceInput) (*careplan.PickupException, error) {
	existing, err := s.pickups.FindByStudentIDAndDate(ctx, input.StudentID, input.Date)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return s.claimExisting(ctx, existing, input)
	}
	clock := calendar.NormalizeWallClock(input.FromTime)
	staffID := input.StaffID
	row := &careplan.PickupException{
		TenantID: s.tx.TenantID(ctx), StudentID: input.StudentID, ExceptionDate: careplan.Date(input.Date),
		PickupTime: &clock, ExcusedFrom: &clock, ExcusedReason: trimmedReason(input.Reason),
		ExcusedCreatedBy: &staffID, ExcusedOwnsPickupTime: true, Source: careplan.ExceptionSourceStaff, CreatedBy: staffID,
	}
	if err := s.pickups.Create(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}

func (s *partialAbsenceService) claimExisting(ctx context.Context, row *careplan.PickupException, input careplan.PartialAbsenceInput) (*careplan.PickupException, error) {
	if row.HasManualPartialAbsence() {
		return nil, careplan.ErrPartialAbsenceAlreadyExists
	}
	if row.PickupTime == nil {
		return nil, careplan.ErrPartialAbsencePickupConflict
	}
	// An auto-derived excusal yields to the explicit staff decision.
	if row.ExcusedAuto {
		if _, err := s.slots.ReleasePartialAbsence(ctx, row.ID); err != nil {
			return nil, err
		}
	}
	row.ExcusedOwnsPickupTime = false
	setManualExcusal(row, input)
	if err := s.pickups.Update(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}

func setManualExcusal(row *careplan.PickupException, input careplan.PartialAbsenceInput) {
	clock := calendar.NormalizeWallClock(input.FromTime)
	staffID := input.StaffID
	if row.ExcusedOwnsPickupTime {
		row.PickupTime = &clock
	}
	row.ExcusedFrom = &clock
	row.ExcusedReason = trimmedReason(input.Reason)
	row.ExcusedCreatedBy = &staffID
	row.ExcusedAuto = false
	row.NormalizeWallClockTimes()
}

func (s *partialAbsenceService) UpdatePartialAbsence(ctx context.Context, exceptionID int64, input careplan.PartialAbsenceInput) (*careplan.PickupException, error) {
	if err := validatePartialAbsenceInput(input); err != nil {
		return nil, err
	}
	var result *careplan.PickupException
	err := s.tx.WithinTenant(ctx, func(txCtx context.Context) error {
		if err := s.lockWritableDay(txCtx, input); err != nil {
			return err
		}
		row, err := s.findOwned(txCtx, exceptionID, input.StudentID)
		if err != nil {
			return err
		}
		if err := s.updatePartial(txCtx, row, input); err != nil {
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

func (s *partialAbsenceService) findOwned(ctx context.Context, exceptionID, studentID int64) (*careplan.PickupException, error) {
	row, err := s.pickups.FindByID(ctx, exceptionID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, careplan.ErrPartialAbsenceNotFound
	}
	if row.StudentID != studentID {
		return nil, careplan.ErrPartialAbsenceWrongStudent
	}
	return row, nil
}

func (s *partialAbsenceService) DeletePartialAbsence(ctx context.Context, exceptionID, studentID int64) error {
	return s.tx.WithinTenant(ctx, func(txCtx context.Context) error {
		// Resolve the date unlocked, then re-read under student and care-day locks.
		row, err := s.findOwned(txCtx, exceptionID, studentID)
		if err != nil {
			return err
		}
		if err := s.tx.LockStudentAndExceptionDay(txCtx, studentID, row.ExceptionDate.String()); err != nil {
			return err
		}
		row, err = s.findOwned(txCtx, exceptionID, studentID)
		if err != nil {
			return err
		}
		return s.removeManual(txCtx, row)
	})
}

func (s *partialAbsenceService) removeManual(ctx context.Context, row *careplan.PickupException) error {
	if row.ExcusedFrom == nil {
		return careplan.ErrPartialAbsenceNotFound
	}
	if row.ExcusedAuto {
		return careplan.ErrPartialAbsenceAutoManaged
	}
	if _, err := s.slots.ReleasePartialAbsence(ctx, row.ID); err != nil {
		return err
	}
	if row.ExcusedOwnsPickupTime {
		return s.pickups.Delete(ctx, row.ID)
	}
	row.ExcusedFrom = nil
	row.ExcusedReason = nil
	row.ExcusedCreatedBy = nil
	row.ExcusedOwnsPickupTime = false
	row.NormalizeWallClockTimes()
	if err := s.pickups.Update(ctx, row); err != nil {
		return err
	}
	// The pickup survives. Re-derive any automatic excusal after removing the override.
	if s.syncer != nil {
		_, err := s.syncer.Sync(ctx, row.ID)
		return err
	}
	return nil
}

func (s *partialAbsenceService) ensureNoFullDayStatus(ctx context.Context, studentID int64, date calendar.Date) error {
	exists, err := s.conflicts.HasFullDayStatus(ctx, studentID, date)
	if err != nil {
		return err
	}
	if exists {
		return careplan.ErrPartialAbsenceFullDayConflict
	}
	return nil
}

// ensureNoPendingExcusedRequest keeps partial-day writes mutually exclusive
// with open full-day parent requests for the same child and date.
func (s *partialAbsenceService) ensureNoPendingExcusedRequest(ctx context.Context, studentID int64, date calendar.Date) error {
	dates, err := s.conflicts.PendingExcusedDates(ctx, studentID)
	if err != nil {
		return err
	}
	for _, requested := range dates {
		if requested == date {
			return careplan.ErrPartialAbsencePendingRequestConflict
		}
	}
	return nil
}

func validatePartialAbsenceInput(input careplan.PartialAbsenceInput) error {
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

func (s *partialAbsenceService) updatePartial(ctx context.Context, row *careplan.PickupException, input careplan.PartialAbsenceInput) error {
	if row.ExceptionDate != careplan.Date(input.Date) {
		return careplan.ErrPartialAbsenceWrongStudent
	}
	if row.ExcusedFrom == nil {
		return careplan.ErrPartialAbsenceNotFound
	}
	if _, err := s.slots.ReleasePartialAbsence(ctx, row.ID); err != nil {
		return err
	}
	// Editing converts an auto-derived excusal into a manual one.
	setManualExcusal(row, input)
	if err := s.pickups.Update(ctx, row); err != nil {
		return err
	}
	if _, err := s.slots.ApplyPartialAbsence(ctx, row.ID); err != nil {
		return err
	}
	return nil
}
