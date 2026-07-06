package schedule

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// maxShiftRangeDays caps date-range queries so a malformed request cannot
// pull years of shift rows in one call (the UI fetches one week at a time).
const maxShiftRangeDays = 62

var (
	// ErrShiftOverlap signals that a shift would overlap an existing shift
	// of the same staff member on the same day.
	ErrShiftOverlap = errors.New("shift overlaps an existing shift on this day")
	// ErrShiftRangeTooLarge signals a date range beyond maxShiftRangeDays.
	ErrShiftRangeTooLarge = errors.New("date range too large")
	// ErrShiftNotFound signals that the requested shift does not exist.
	ErrShiftNotFound = errors.New("shift not found")
	// ErrShiftInvalid wraps model/input validation failures (maps to 400).
	ErrShiftInvalid = errors.New("invalid shift")
)

// StaffShiftService manages planned per-date staff shifts (Dienstplan).
type StaffShiftService interface {
	// ListShifts returns all staff shifts in the range (admin week view).
	ListShifts(ctx context.Context, start, end timezone.Date) ([]*scheduleModels.StaffShift, error)
	// ListShiftsForStaff returns one staff member's shifts in the range.
	ListShiftsForStaff(ctx context.Context, staffID int64, start, end timezone.Date) ([]*scheduleModels.StaffShift, error)
	// CreateShift validates and persists a new shift.
	CreateShift(ctx context.Context, shift *scheduleModels.StaffShift) (*scheduleModels.StaffShift, error)
	// UpdateShift validates and persists changes to an existing shift.
	UpdateShift(ctx context.Context, shift *scheduleModels.StaffShift) (*scheduleModels.StaffShift, error)
	// UpdateShiftWithOptions validates and persists changes with update-specific merge options.
	UpdateShiftWithOptions(ctx context.Context, shift *scheduleModels.StaffShift, opts StaffShiftUpdateOptions) (*scheduleModels.StaffShift, error)
	// DeleteShift removes a shift.
	DeleteShift(ctx context.Context, id int64) error
}

type StaffShiftUpdateOptions struct {
	PreserveExistingNotes bool
}

type staffShiftService struct {
	repo      scheduleModels.StaffShiftRepository
	staffRepo usersModels.StaffRepository
	db        *bun.DB
	logger    *slog.Logger
}

// NewStaffShiftService creates a new staff shift service. db is used for the
// per-staff advisory lock that makes the overlap check safe under concurrency.
func NewStaffShiftService(repo scheduleModels.StaffShiftRepository, staffRepo usersModels.StaffRepository, db *bun.DB, logger *slog.Logger) StaffShiftService {
	return &staffShiftService{repo: repo, staffRepo: staffRepo, db: db, logger: logger}
}

// lockShiftWrites takes the per-staff advisory lock before the overlap
// read-check so concurrent writers serialize on the same staff member.
// Unit tests construct the service without a DB; they exercise the overlap
// logic, not concurrency, so a nil DB skips the lock.
func (s *staffShiftService) lockShiftWrites(ctx context.Context, staffID int64) error {
	if s.db == nil {
		return nil
	}
	return LockStaffShiftWrites(ctx, s.db, staffID)
}

func (s *staffShiftService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

func validateShiftRange(start, end timezone.Date) error {
	if start.IsZero() || end.IsZero() {
		return fmt.Errorf("%w: start and end dates are required", ErrShiftInvalid)
	}
	if end.Before(start) {
		return fmt.Errorf("%w: end date must not be before start date", ErrShiftInvalid)
	}
	if start.DaysUntil(end) > maxShiftRangeDays {
		return ErrShiftRangeTooLarge
	}
	return nil
}

func (s *staffShiftService) ListShifts(ctx context.Context, start, end timezone.Date) ([]*scheduleModels.StaffShift, error) {
	if err := validateShiftRange(start, end); err != nil {
		return nil, err
	}
	return s.repo.FindByDateRange(ctx, start, end)
}

func (s *staffShiftService) ListShiftsForStaff(ctx context.Context, staffID int64, start, end timezone.Date) ([]*scheduleModels.StaffShift, error) {
	if staffID <= 0 {
		return nil, fmt.Errorf("%w: staff ID is required", ErrShiftInvalid)
	}
	if err := validateShiftRange(start, end); err != nil {
		return nil, err
	}
	return s.repo.FindByStaffAndDateRange(ctx, staffID, start, end)
}

// checkOverlap rejects the shift when it intersects another shift of the
// same staff member on the same date (excluding itself on update).
func (s *staffShiftService) checkOverlap(ctx context.Context, shift *scheduleModels.StaffShift) error {
	existing, err := s.repo.FindByStaffAndDateRange(ctx, shift.StaffID, shift.Date, shift.Date)
	if err != nil {
		return err
	}
	for _, other := range existing {
		if other.ID == shift.ID {
			continue
		}
		if shift.Overlaps(other) {
			return ErrShiftOverlap
		}
	}
	return nil
}

func (s *staffShiftService) CreateShift(ctx context.Context, shift *scheduleModels.StaffShift) (*scheduleModels.StaffShift, error) {
	if err := shift.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrShiftInvalid, err.Error())
	}
	staff, err := s.staffRepo.FindByID(ctx, shift.StaffID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: staff member not found", ErrShiftInvalid)
		}
		return nil, fmt.Errorf("find staff member for shift: %w", err)
	}
	if staff == nil {
		return nil, fmt.Errorf("%w: staff member not found", ErrShiftInvalid)
	}
	if err := s.lockShiftWrites(ctx, shift.StaffID); err != nil {
		return nil, err
	}
	if err := s.checkOverlap(ctx, shift); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, shift); err != nil {
		return nil, err
	}
	s.getLogger().Info("staff shift created",
		"shift_id", shift.ID,
		"staff_id", shift.StaffID,
		"date", shift.Date.String(),
	)
	return shift, nil
}

func (s *staffShiftService) UpdateShift(ctx context.Context, shift *scheduleModels.StaffShift) (*scheduleModels.StaffShift, error) {
	return s.UpdateShiftWithOptions(ctx, shift, StaffShiftUpdateOptions{})
}

func (s *staffShiftService) UpdateShiftWithOptions(ctx context.Context, shift *scheduleModels.StaffShift, opts StaffShiftUpdateOptions) (*scheduleModels.StaffShift, error) {
	if shift.ID <= 0 {
		return nil, ErrShiftNotFound
	}
	existing, err := s.repo.FindByID(ctx, shift.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrShiftNotFound
		}
		return nil, fmt.Errorf("find staff shift: %w", err)
	}
	if existing == nil {
		return nil, ErrShiftNotFound
	}
	// Staff assignment is immutable; the shift stays with its staff member.
	shift.StaffID = existing.StaffID
	shift.CreatedBy = existing.CreatedBy
	shift.TenantID = existing.TenantID
	if opts.PreserveExistingNotes {
		shift.Notes = existing.Notes
	}
	// The request model has a zero CreatedAt; the whole-model update would
	// otherwise write created_at = DEFAULT and reset it to now().
	shift.CreatedAt = existing.CreatedAt
	if err := shift.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrShiftInvalid, err.Error())
	}
	if err := s.lockShiftWrites(ctx, shift.StaffID); err != nil {
		return nil, err
	}
	if err := s.checkOverlap(ctx, shift); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, shift); err != nil {
		return nil, err
	}
	s.getLogger().Info("staff shift updated",
		"shift_id", shift.ID,
		"staff_id", shift.StaffID,
		"date", shift.Date.String(),
	)
	return shift, nil
}

func (s *staffShiftService) DeleteShift(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrShiftNotFound
	}
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrShiftNotFound
		}
		return fmt.Errorf("find staff shift: %w", err)
	}
	if existing == nil {
		return ErrShiftNotFound
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.getLogger().Info("staff shift deleted",
		"shift_id", id,
		"staff_id", existing.StaffID,
	)
	return nil
}
