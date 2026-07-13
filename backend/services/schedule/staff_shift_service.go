package schedule

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

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
	// ErrShiftTypeInactive signals an attempt to assign a deactivated shift type
	// to a shift (maps to 400). Existing shifts keep their already-attached type.
	ErrShiftTypeInactive = errors.New("shift type is inactive")
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
	// ApplyCancellation atomically cancels (or reactivates) a shift and replaces
	// its full set of replacement covers in one operation (#1841). Cancelling
	// with N replacements, reactivating (which removes every replacement), and
	// re-planning the covers all run as a single unit so a partial failure never
	// leaves a half-changed schedule.
	ApplyCancellation(ctx context.Context, input CancelShiftInput) (*CancelShiftResult, error)
	// SetSeriesExceptionRepo injects the series exception repository (#1889);
	// see the implementation comment. Same optional-injection pattern as
	// WorkSessionService.SetStaffShiftRepo.
	SetSeriesExceptionRepo(repo scheduleModels.StaffShiftSeriesExceptionRepository)
}

type StaffShiftUpdateOptions struct {
	PreserveExistingNotes bool
	// PreserveExistingShiftType keeps the stored shift type when the update
	// request omitted shift_type_id entirely (stale client / third-party
	// consumer), so an unrelated edit does not silently clear the label. An
	// explicit null in the payload still clears it.
	PreserveExistingShiftType bool
	// PreserveExistingChangeReason keeps the stored change reason when the
	// update request omitted change_reason entirely, so an unrelated edit does
	// not silently drop the recorded "why" (#1841). An explicit null clears it.
	PreserveExistingChangeReason bool
	// PreserveExistingCancelled keeps the stored cancelled flag when the update
	// request omitted the cancelled key entirely, so a stale client or an
	// unrelated field edit does not silently reactivate a cancelled shift
	// (#1841). Cancellation/reactivation flows an explicit value through instead.
	PreserveExistingCancelled bool
}

type staffShiftService struct {
	repo          scheduleModels.StaffShiftRepository
	staffRepo     usersModels.StaffRepository
	shiftTypes    ShiftTypeService
	exceptionRepo scheduleModels.StaffShiftSeriesExceptionRepository
	db            *bun.DB
	logger        *slog.Logger
}

// NewStaffShiftService creates a new staff shift service. db is used for the
// per-staff advisory lock that makes the overlap check safe under concurrency.
// shiftTypes resolves the active flag when a shift is assigned a type; it may be
// nil in unit tests that never attach a shift type.
func NewStaffShiftService(repo scheduleModels.StaffShiftRepository, staffRepo usersModels.StaffRepository, shiftTypes ShiftTypeService, db *bun.DB, logger *slog.Logger) StaffShiftService {
	return &staffShiftService{repo: repo, staffRepo: staffRepo, shiftTypes: shiftTypes, db: db, logger: logger}
}

// SetSeriesExceptionRepo wires the series exception repository (#1889).
// When set, deleting a series-backed shift records the date as a series
// exception so re-plans never regenerate the removed occurrence. Optional so
// unit tests without series stay unchanged (same pattern as the auto-checkout
// shift repo injection).
func (s *staffShiftService) SetSeriesExceptionRepo(repo scheduleModels.StaffShiftSeriesExceptionRepository) {
	s.exceptionRepo = repo
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

// ensureShiftTypeActive rejects assigning a shift type that does not exist in
// the tenant (ErrShiftTypeNotFound -> 400) or has been deactivated
// (ErrShiftTypeInactive -> 400). A nil id is an untyped shift and always valid.
// Deactivation must block *new* assignments even for a stale client or a direct
// API caller that still sends the id; callers skip this check when an update
// keeps the shift's already-attached (possibly inactive) type. A nil
// shiftTypes dependency (unit tests without a shift type) skips the lookup.
func (s *staffShiftService) ensureShiftTypeActive(ctx context.Context, id *int64) error {
	if id == nil || s.shiftTypes == nil {
		return nil
	}
	shiftType, err := s.shiftTypes.GetShiftType(ctx, *id)
	if err != nil {
		return err
	}
	if !shiftType.IsActive {
		return ErrShiftTypeInactive
	}
	return nil
}

// sameShiftTypeID reports whether two nullable shift-type ids refer to the same
// type (both nil counts as equal).
func sameShiftTypeID(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
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
// same staff member on the same date (excluding itself on update). A cancelled
// shift does not take place, so it neither blocks other shifts nor is blocked
// by them (#1841): a replacement or a shortened return can reuse the freed
// window.
func (s *staffShiftService) checkOverlap(ctx context.Context, shift *scheduleModels.StaffShift) error {
	if shift.Cancelled {
		return nil
	}
	existing, err := s.repo.FindByStaffAndDateRange(ctx, shift.StaffID, shift.Date, shift.Date)
	if err != nil {
		return err
	}
	for _, other := range existing {
		if other.ID == shift.ID || other.Cancelled {
			continue
		}
		if shift.Overlaps(other) {
			return ErrShiftOverlap
		}
	}
	return nil
}

// loadOriginShift reads a replacement's origin shift, mapping a missing row
// (FindByID is tenant-scoped, so a cross-tenant origin also reads as not found)
// to ErrShiftInvalid.
func (s *staffShiftService) loadOriginShift(ctx context.Context, originID int64) (*scheduleModels.StaffShift, error) {
	origin, err := s.repo.FindByID(ctx, originID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: origin shift not found", ErrShiftInvalid)
		}
		return nil, fmt.Errorf("find origin shift: %w", err)
	}
	if origin == nil {
		return nil, fmt.Errorf("%w: origin shift not found", ErrShiftInvalid)
	}
	return origin, nil
}

// lockAndValidateOrigin validates a replacement's origin shift under the origin
// staff member's write lock. A replacement only covers a real gap, and a gap
// exists only when the origin was left cancelled — an active origin still
// contributes its own planned minutes, so pointing a replacement at it would
// double-count coverage (#1841). Reading the origin, locking its staff member,
// and re-reading it must happen together: without the lock a concurrent
// reactivation can flip the origin active between the check and the replacement
// insert, leaving an active origin alongside its replacement. The lock order
// (origin staff first, then the replacement's own staff in the caller) mirrors
// ApplyCancellation so the two paths never lock in opposite orders. A nil origin
// is a normal shift and needs neither lock nor check.
func (s *staffShiftService) lockAndValidateOrigin(ctx context.Context, shift *scheduleModels.StaffShift) error {
	if shift.OriginShiftID == nil {
		return nil
	}
	// First read discovers the origin's staff member so we can lock it.
	origin, err := s.loadOriginShift(ctx, *shift.OriginShiftID)
	if err != nil {
		return err
	}
	if err := s.lockShiftWrites(ctx, origin.StaffID); err != nil {
		return err
	}
	// Re-read under the lock: a reactivation that committed before we acquired the
	// lock is now visible, so a stale "cancelled" read cannot slip through.
	origin, err = s.loadOriginShift(ctx, *shift.OriginShiftID)
	if err != nil {
		return err
	}
	if origin.Date != shift.Date {
		return fmt.Errorf("%w: replacement must be on the same date as the shift it covers", ErrShiftInvalid)
	}
	if !origin.Cancelled {
		return fmt.Errorf("%w: replacement origin must be a cancelled shift", ErrShiftInvalid)
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
	// A create always assigns the type fresh, so it must be active.
	if err := s.ensureShiftTypeActive(ctx, shift.ShiftTypeID); err != nil {
		return nil, err
	}
	// A replacement covers a gap; it must never itself be a gap. Reject a create
	// that is both cancelled and a replacement — it would be inserted as a
	// cancelled cover that contributes no planned time and that ApplyCancellation
	// (which rejects replacement rows) could never repair (#1841).
	if shift.Cancelled && shift.OriginShiftID != nil {
		return nil, fmt.Errorf("%w: a replacement shift cannot be cancelled", ErrShiftInvalid)
	}
	// A replacement shift must point at a real gap on the same day; the origin's
	// staff member is locked here so a concurrent reactivation cannot flip it
	// active before the replacement is inserted (#1841).
	if err := s.lockAndValidateOrigin(ctx, shift); err != nil {
		return nil, err
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
	// A replacement stays a replacement of the same origin — the cover link is
	// set at creation and never re-pointed by a plain edit (#1841).
	shift.OriginShiftID = existing.OriginShiftID
	// If a plain edit moves a replacement to a different day, the create-time
	// invariant (a cover shares its origin's date and the origin is cancelled)
	// would silently break — re-validate the preserved link against the new
	// date (#1841). Same-date edits keep the guarantee they were created with,
	// and skipping the origin read for them avoids an extra query on every edit.
	if shift.OriginShiftID != nil && shift.Date != existing.Date {
		if err := s.lockAndValidateOrigin(ctx, shift); err != nil {
			return nil, err
		}
	}
	if opts.PreserveExistingNotes {
		shift.Notes = existing.Notes
	}
	if opts.PreserveExistingShiftType {
		shift.ShiftTypeID = existing.ShiftTypeID
	}
	if opts.PreserveExistingChangeReason {
		shift.ChangeReason = existing.ChangeReason
	}
	if opts.PreserveExistingCancelled {
		shift.Cancelled = existing.Cancelled
	}
	// The request model has a zero CreatedAt; the whole-model update would
	// otherwise write created_at = DEFAULT and reset it to now().
	shift.CreatedAt = existing.CreatedAt
	// Editing a series-backed row IS the "Nur diese Woche" deviation: the row
	// keeps its series link but detaches, so re-plans and splits leave it
	// alone (#1889). Standalone rows stay untouched by this.
	shift.SeriesID = existing.SeriesID
	shift.Detached = existing.Detached
	if existing.SeriesID != nil {
		shift.Detached = true
	}
	if err := shift.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrShiftInvalid, err.Error())
	}
	// A newly assigned type must be active; keeping the shift's already-attached
	// (possibly deactivated) type stays allowed.
	if !sameShiftTypeID(shift.ShiftTypeID, existing.ShiftTypeID) {
		if err := s.ensureShiftTypeActive(ctx, shift.ShiftTypeID); err != nil {
			return nil, err
		}
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
	// Deletes write series exceptions below, so they must serialize against
	// concurrent series splits on the same staff member: without the lock a
	// racing split could re-point the row while the exception lands on the
	// old, capped series.
	if err := s.lockShiftWrites(ctx, existing.StaffID); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	// Deleting a series-backed row records the date as a series exception so
	// re-plans and splits never regenerate the removed occurrence (#1889).
	if existing.SeriesID != nil && s.exceptionRepo != nil {
		exception := &scheduleModels.StaffShiftSeriesException{
			SeriesID:  *existing.SeriesID,
			Date:      existing.Date,
			CreatedBy: existing.CreatedBy,
		}
		exception.TenantID = existing.TenantID
		if err := s.exceptionRepo.Create(ctx, exception); err != nil {
			return fmt.Errorf("record series exception for deleted shift: %w", err)
		}
	}
	s.getLogger().Info("staff shift deleted",
		"shift_id", id,
		"staff_id", existing.StaffID,
	)
	return nil
}

// ShiftReplacementInput is one person covering part of a cancelled shift's gap.
type ShiftReplacementInput struct {
	StaffID      int64
	StartTime    time.Time
	EndTime      time.Time
	BreakMinutes int
	ShiftTypeID  *int64
}

// CancelShiftInput drives ApplyCancellation: set Cancelled on the origin shift,
// record ChangeReason, and — when cancelling — recreate exactly the given set of
// replacements. Reactivating (Cancelled == false) removes every replacement and
// rejects any supplied cover.
type CancelShiftInput struct {
	ShiftID      int64
	Cancelled    bool
	ChangeReason *string
	// ApplyOriginEdits, when true, applies the StartTime/EndTime/BreakMinutes/
	// ShiftTypeID below to the origin shift instead of preserving its stored
	// values (#1841). The admin modal edits the shift's own window in the same
	// save as the cancellation/reactivation, so those edits must not be silently
	// dropped — and a reactivation must re-check overlap against the edited
	// window, not the obsolete stored one. Callers that only flip the flag leave
	// this false and the stored window/type is kept.
	ApplyOriginEdits bool
	StartTime        time.Time
	EndTime          time.Time
	BreakMinutes     int
	ShiftTypeID      *int64
	Replacements     []ShiftReplacementInput
	ActorStaffID     int64
}

// CancelShiftResult reports the updated origin and the freshly created covers.
type CancelShiftResult struct {
	Shift        *scheduleModels.StaffShift
	Replacements []*scheduleModels.StaffShift
}

// ApplyCancellation cancels/reactivates a shift and rebuilds its replacement set
// as one atomic operation (#1841). It runs inside the request's tenant
// transaction, so any error rolls back every write together — the handler must
// mark the transaction for rollback on the non-5xx errors this returns (overlap,
// invalid input) so a partially applied change never commits.
//
// Order matters: existing replacements are deleted first, then the origin flag
// flips (so a reactivation's overlap check no longer sees the covers), then —
// only when cancelling — the new covers are created (which re-validates that the
// now-cancelled origin is a legal replacement target).
func (s *staffShiftService) ApplyCancellation(ctx context.Context, input CancelShiftInput) (*CancelShiftResult, error) {
	if input.ShiftID <= 0 {
		return nil, ErrShiftNotFound
	}
	if !input.Cancelled && len(input.Replacements) > 0 {
		return nil, fmt.Errorf("%w: replacements are only valid when cancelling a shift", ErrShiftInvalid)
	}
	existing, err := s.repo.FindByID(ctx, input.ShiftID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrShiftNotFound
		}
		return nil, fmt.Errorf("find staff shift: %w", err)
	}
	if existing == nil {
		return nil, ErrShiftNotFound
	}
	// A replacement is not itself a cancellable gap: it covers one. Cancelling a
	// cover would orphan the notion of "who covers the origin".
	if existing.OriginShiftID != nil {
		return nil, fmt.Errorf("%w: a replacement shift cannot be cancelled", ErrShiftInvalid)
	}

	// Serialize on the origin's staff member for the whole operation; the
	// per-replacement CreateShift calls below lock their own staff members too.
	if err := s.lockShiftWrites(ctx, existing.StaffID); err != nil {
		return nil, err
	}

	// Remove the current cover set first so the origin's flag can flip without
	// its old replacements interfering with the reactivation overlap check.
	covers, err := s.repo.FindByOriginShiftID(ctx, existing.ID)
	if err != nil {
		return nil, fmt.Errorf("load existing replacements: %w", err)
	}
	for _, cover := range covers {
		if err := s.repo.Delete(ctx, cover.ID); err != nil {
			return nil, fmt.Errorf("remove existing replacement: %w", err)
		}
	}

	// Flip the origin's cancellation via the normal update path so the
	// series-detach and (on reactivation) overlap rules apply. Notes are always
	// preserved (the modal has no notes field); the window/type default to the
	// stored values but are overwritten with the caller's edits when supplied,
	// so an edit made in the same save is honoured and a reactivation's overlap
	// check runs against the edited window (#1841).
	originUpdate := &scheduleModels.StaffShift{
		Date:         existing.Date,
		StartTime:    existing.StartTime,
		EndTime:      existing.EndTime,
		BreakMinutes: existing.BreakMinutes,
		ShiftTypeID:  existing.ShiftTypeID,
		Cancelled:    input.Cancelled,
		ChangeReason: input.ChangeReason,
	}
	preserveType := true
	if input.ApplyOriginEdits {
		originUpdate.StartTime = input.StartTime
		originUpdate.EndTime = input.EndTime
		originUpdate.BreakMinutes = input.BreakMinutes
		originUpdate.ShiftTypeID = input.ShiftTypeID
		preserveType = false
	}
	originUpdate.ID = existing.ID
	if input.ActorStaffID > 0 {
		originUpdate.UpdatedBy = &input.ActorStaffID
	}
	updated, err := s.UpdateShiftWithOptions(ctx, originUpdate, StaffShiftUpdateOptions{
		PreserveExistingNotes:     true,
		PreserveExistingShiftType: preserveType,
	})
	if err != nil {
		return nil, err
	}

	result := &CancelShiftResult{Shift: updated}
	if !input.Cancelled {
		s.getLogger().Info("staff shift reactivated",
			"shift_id", updated.ID,
			"staff_id", updated.StaffID,
			"removed_replacements", len(covers),
		)
		return result, nil
	}

	// Recreate the cover set against the now-cancelled origin. Each CreateShift
	// re-validates the origin (must be cancelled, same date) and overlap for its
	// own staff member.
	for _, r := range input.Replacements {
		replacement := &scheduleModels.StaffShift{
			StaffID:       r.StaffID,
			Date:          existing.Date,
			StartTime:     r.StartTime,
			EndTime:       r.EndTime,
			BreakMinutes:  r.BreakMinutes,
			ShiftTypeID:   r.ShiftTypeID,
			OriginShiftID: &existing.ID,
			ChangeReason:  input.ChangeReason,
			CreatedBy:     input.ActorStaffID,
		}
		created, err := s.CreateShift(ctx, replacement)
		if err != nil {
			return nil, err
		}
		result.Replacements = append(result.Replacements, created)
	}
	s.getLogger().Info("staff shift cancelled",
		"shift_id", updated.ID,
		"staff_id", updated.StaffID,
		"replacements", len(result.Replacements),
	)
	return result, nil
}
