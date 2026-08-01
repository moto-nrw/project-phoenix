package schedule

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
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
	// ErrShiftConflict signals that a move was based on a stale source owner.
	// It maps to 409 so the client can reload instead of overwriting a newer move.
	ErrShiftConflict = errors.New("shift changed concurrently")
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
	// MoveShift atomically and retry-safely moves one concrete shift to another
	// person and/or slot while preserving its identity and metadata.
	MoveShift(ctx context.Context, input MoveShiftInput) (*scheduleModels.StaffShift, error)
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
	// SetDeviationEventRepo injects the Änderungsprotokoll repository (#1884);
	// when set, MoveShift appends a shift_moved audit event inside the same
	// tenant tx. Same optional-injection pattern as SetSeriesExceptionRepo.
	SetDeviationEventRepo(repo auditModels.DeviationEventRepository)
}

type StaffShiftUpdateOptions struct {
	// SuppressTimeTrackingBroadcast lets a larger operation send one shared
	// invalidation after all of its shift writes have completed.
	SuppressTimeTrackingBroadcast bool
	PreserveExistingNotes         bool
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
	// deviationEventRepo appends shift_moved Änderungsprotokoll rows (#1884).
	// Optional so unit tests without audit stay unchanged; production wiring
	// (services/factory.go) always sets it, making the event write fail-closed
	// there (an audit failure rolls the move back).
	deviationEventRepo auditModels.DeviationEventRepository
	db                 *bun.DB
	broadcaster        realtime.Broadcaster
	logger             *slog.Logger
	// lockObserver, when set, is invoked with the sorted, de-duplicated staff-id
	// set of every lockStaffWritesOrdered acquisition. It is a test-only seam: the
	// advisory lock is a no-op without a DB, so a unit test cannot otherwise assert
	// which staff members an operation serializes on (e.g. that a dropped cover's
	// staff is folded into the cancellation lock set, #1841). Nil in production.
	lockObserver func(staffIDs []int64)
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

// SetDeviationEventRepo wires the Änderungsprotokoll repository (#1884).
func (s *staffShiftService) SetDeviationEventRepo(repo auditModels.DeviationEventRepository) {
	s.deviationEventRepo = repo
}

// SetBroadcaster injects the tenant-wide SSE broadcaster used to invalidate
// time-tracking views after committed shift writes.
func (s *staffShiftService) SetBroadcaster(broadcaster realtime.Broadcaster) {
	s.broadcaster = broadcaster
}

func (s *staffShiftService) broadcastTimeTrackingChanged(ctx context.Context) {
	realtime.QueueStaffTimeTrackingChanged(ctx, s.broadcaster, s.getLogger())
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
	return s.ensureShiftTypeAssignable(ctx, id, nil)
}

// ensureShiftTypeAssignable is ensureShiftTypeActive with an allowlist of type
// ids that may remain even when deactivated. Rebuilding an existing cover set
// re-sends each cover's own (possibly since-deactivated) type; that type is
// grandfathered in here, mirroring the ordinary-edit rule that lets a shift keep
// its already-attached inactive type (#1841). A type still has to exist — an
// unknown id maps to ErrShiftTypeNotFound regardless of the allowlist.
func (s *staffShiftService) ensureShiftTypeAssignable(ctx context.Context, id *int64, allowedInactive map[int64]bool) error {
	if id == nil || s.shiftTypes == nil {
		return nil
	}
	shiftType, err := s.shiftTypes.GetShiftType(ctx, *id)
	if err != nil {
		return err
	}
	if !shiftType.IsActive && !allowedInactive[*id] {
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
	shifts, err := s.repo.FindByDateRange(ctx, start, end)
	if err != nil {
		return nil, err
	}
	if err := s.attachShiftTypes(ctx, shifts); err != nil {
		return nil, err
	}
	return shifts, nil
}

func (s *staffShiftService) ListShiftsForStaff(ctx context.Context, staffID int64, start, end timezone.Date) ([]*scheduleModels.StaffShift, error) {
	if staffID <= 0 {
		return nil, fmt.Errorf("%w: staff ID is required", ErrShiftInvalid)
	}
	if err := validateShiftRange(start, end); err != nil {
		return nil, err
	}
	shifts, err := s.repo.FindByStaffAndDateRange(ctx, staffID, start, end)
	if err != nil {
		return nil, err
	}
	if err := s.attachShiftTypes(ctx, shifts); err != nil {
		return nil, err
	}
	return shifts, nil
}

// attachShiftTypes resolves each shift's ShiftTypeID to its Schichtart (name +
// color) and sets StaffShift.ShiftType, so a reader who cannot call the
// admin-only /api/shift-types endpoint still gets the label (#1844). One tenant
// has a handful of shift types, so a single ListShiftTypes call is cheaper than
// per-shift lookups. A nil shiftTypes dependency (unit tests) is a no-op; a
// lookup error propagates — these reads run inside TenantTxMiddleware, where a
// failed statement can abort the transaction, so swallowing it would return a
// 200 whose commit then fails after the response was written.
func (s *staffShiftService) attachShiftTypes(ctx context.Context, shifts []*scheduleModels.StaffShift) error {
	if s.shiftTypes == nil || len(shifts) == 0 {
		return nil
	}
	needsType := false
	for _, sh := range shifts {
		if sh.ShiftTypeID != nil {
			needsType = true
			break
		}
	}
	if !needsType {
		return nil
	}
	types, err := s.shiftTypes.ListShiftTypes(ctx)
	if err != nil {
		return fmt.Errorf("failed to resolve shift types: %w", err)
	}
	byID := make(map[int64]*scheduleModels.ShiftType, len(types))
	for _, t := range types {
		byID[t.ID] = t
	}
	for _, sh := range shifts {
		if sh.ShiftTypeID != nil {
			sh.ShiftType = byID[*sh.ShiftTypeID]
		}
	}
	return nil
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

// lockStaffWritesOrdered takes the per-staff write lock for every given staff
// member in a stable (sorted, de-duplicated) order. Any operation that touches
// more than one staff member's shift rows in the same transaction — a
// replacement covering another person, a cancellation with cross-staff covers —
// MUST acquire its locks through here: taking two staff locks in an
// operation-dependent order lets two cross-covering requests grab the same pair
// in opposite orders, and PostgreSQL then aborts one on a deadlock (#1841). A
// stable global order removes the cycle. Advisory xact locks are re-grantable
// within a transaction, so re-locking an already-held staff member is a no-op.
func (s *staffShiftService) lockStaffWritesOrdered(ctx context.Context, staffIDs []int64) error {
	unique := make([]int64, 0, len(staffIDs))
	seen := make(map[int64]bool, len(staffIDs))
	for _, id := range staffIDs {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}
	slices.Sort(unique)
	if s.lockObserver != nil {
		s.lockObserver(unique)
	}
	for _, id := range unique {
		if err := s.lockShiftWrites(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// validateOriginLink re-reads a replacement's origin shift under the caller's
// already-held locks and enforces the cover invariants. A replacement only
// covers a real gap, and a gap exists only when the origin was left cancelled —
// an active origin still contributes its own planned minutes, so pointing a
// replacement at it would double-count coverage (#1841). The caller MUST have
// locked both the replacement's own staff and the origin's staff (via
// lockStaffWritesOrdered) before calling this: re-reading the origin under the
// lock means a reactivation that committed before we acquired the lock is now
// visible, so a stale "cancelled" read cannot slip through. A nil origin is a
// normal shift and needs no check.
func (s *staffShiftService) validateOriginLink(ctx context.Context, shift *scheduleModels.StaffShift) error {
	if shift.OriginShiftID == nil {
		return nil
	}
	origin, err := s.loadOriginShift(ctx, *shift.OriginShiftID)
	if err != nil {
		return err
	}
	// A person cannot cover their own cancelled shift: that would mark them absent
	// (the cancelled origin) and simultaneously present (an active cover counting
	// planned minutes and driving auto-checkout). Overlap checks ignore the
	// cancelled origin, so this must be rejected explicitly (#1841).
	if origin.StaffID == shift.StaffID {
		return fmt.Errorf("%w: a shift cannot be its own replacement", ErrShiftInvalid)
	}
	if origin.Date != shift.Date {
		return fmt.Errorf("%w: replacement must be on the same date as the shift it covers", ErrShiftInvalid)
	}
	if !origin.Cancelled {
		return fmt.Errorf("%w: replacement origin must be a cancelled shift", ErrShiftInvalid)
	}
	// A replacement covers part of the origin's gap, so its window must fall
	// entirely within the origin's window. Without this a 06:00-18:00 cover could
	// attach to an 08:00-10:00 origin and inflate the covering employee's planned
	// minutes and auto-checkout beyond the actual gap. The origin is re-read under
	// lock above, so a same-save resize of the origin (ApplyCancellation applies
	// the edited window before rebuilding covers) is validated against the new
	// window, not the stale one (#1841).
	if !origin.Contains(shift) {
		return fmt.Errorf("%w: replacement must fall within the shift it covers", ErrShiftInvalid)
	}
	return nil
}

func (s *staffShiftService) CreateShift(ctx context.Context, shift *scheduleModels.StaffShift) (*scheduleModels.StaffShift, error) {
	// A create always assigns the type fresh, so no deactivated type is
	// grandfathered in (nil allowlist).
	created, err := s.createShift(ctx, shift, nil)
	if err != nil {
		return nil, err
	}
	s.broadcastTimeTrackingChanged(ctx)
	return created, nil
}

// createShift is the shared create path. allowedInactiveTypes grandfathers a set
// of deactivated shift-type ids (a rebuilt cover set re-sends each cover's own,
// possibly since-deactivated, type — see ApplyCancellation); pass nil for an
// ordinary create, where every assigned type must still be active (#1841).
func (s *staffShiftService) createShift(ctx context.Context, shift *scheduleModels.StaffShift, allowedInactiveTypes map[int64]bool) (*scheduleModels.StaffShift, error) {
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
	if err := s.ensureShiftTypeAssignable(ctx, shift.ShiftTypeID, allowedInactiveTypes); err != nil {
		return nil, err
	}
	// A replacement covers a gap; it must never itself be a gap. Reject a create
	// that is both cancelled and a replacement — it would be inserted as a
	// cancelled cover that contributes no planned time and that ApplyCancellation
	// (which rejects replacement rows) could never repair (#1841).
	if shift.Cancelled && shift.OriginShiftID != nil {
		return nil, fmt.Errorf("%w: a replacement shift cannot be cancelled", ErrShiftInvalid)
	}
	// Lock every staff member this create touches — the shift's own staff plus,
	// for a replacement, the origin's staff — in a stable sorted order so two
	// cross-covering creates never lock the same pair in opposite orders (#1841
	// deadlock). The origin is read first only to discover whose lock to take.
	lockIDs := []int64{shift.StaffID}
	if shift.OriginShiftID != nil {
		origin, err := s.loadOriginShift(ctx, *shift.OriginShiftID)
		if err != nil {
			return nil, err
		}
		lockIDs = append(lockIDs, origin.StaffID)
	}
	if err := s.lockStaffWritesOrdered(ctx, lockIDs); err != nil {
		return nil, err
	}
	// A replacement shift must point at a real gap on the same day; re-read under
	// the locks so a concurrent reactivation cannot flip the origin active before
	// the replacement is inserted (#1841).
	if err := s.validateOriginLink(ctx, shift); err != nil {
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
	updated, err := s.updateShiftWithOptions(ctx, shift, opts)
	if err != nil {
		return nil, err
	}
	if !opts.SuppressTimeTrackingBroadcast {
		s.broadcastTimeTrackingChanged(ctx)
	}
	return updated, nil
}

func (s *staffShiftService) updateShiftWithOptions(ctx context.Context, shift *scheduleModels.StaffShift, opts StaffShiftUpdateOptions) (*scheduleModels.StaffShift, error) {
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
	// Sick provenance is written only by the #1843 cascade via UpdateColumns
	// after ApplyCancellation. Any ordinary edit takes ownership of the shift,
	// including keeping an already-cancelled shift cancelled with a new reason;
	// deleting the sick report must never undo that administrator decision.
	shift.SickAbsenceID = nil
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
	shift.SeriesOccurrenceDate = existing.SeriesOccurrenceDate
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
	// A plain edit that moves a replacement to a different day, or resizes its
	// wall-clock window, would silently break the create-time invariants (a cover
	// shares its origin's date AND stays within the cancelled origin's gap), so
	// re-validate the preserved link whenever the date or the window changes.
	// validateOriginLink re-reads the origin and re-checks origin.Contains(shift),
	// so a same-day extension past the gap is now rejected too. A pure
	// notes/type/break edit that leaves date and window untouched still skips the
	// extra origin read (#1841).
	revalidateOrigin := shift.OriginShiftID != nil &&
		(shift.Date != existing.Date ||
			!timezone.SameClockTime(shift.StartTime, existing.StartTime) ||
			!timezone.SameClockTime(shift.EndTime, existing.EndTime))
	// Lock every staff member this edit touches — the shift's own staff plus, for
	// a replacement date/window edit, the origin's staff — in a stable sorted order
	// so concurrent cross-covering edits never lock the same pair in opposite
	// orders (#1841 deadlock). The origin is read first only to discover its lock.
	lockIDs := []int64{shift.StaffID}
	if revalidateOrigin {
		origin, err := s.loadOriginShift(ctx, *shift.OriginShiftID)
		if err != nil {
			return nil, err
		}
		lockIDs = append(lockIDs, origin.StaffID)
	}
	if err := s.lockStaffWritesOrdered(ctx, lockIDs); err != nil {
		return nil, err
	}
	// A normal edit may have loaded the row before a concurrent MoveShift acquired
	// this same lock. Re-read after locking and reject every move-relevant change,
	// including a same-person date/window move: otherwise the stale edit payload
	// would silently move the row back. UpdatedAt additionally catches other
	// concurrent whole-row updates while this request waited.
	lockedExisting, err := s.repo.FindByID(ctx, shift.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrShiftNotFound
		}
		return nil, fmt.Errorf("reload staff shift after lock: %w", err)
	}
	if lockedExisting == nil {
		return nil, ErrShiftNotFound
	}
	if staffShiftMoveChanged(existing, lockedExisting) ||
		!lockedExisting.UpdatedAt.Equal(existing.UpdatedAt) {
		return nil, fmt.Errorf("%w: shift changed while the edit was waiting for its write lock", ErrShiftConflict)
	}
	if revalidateOrigin {
		if err := s.validateOriginLink(ctx, shift); err != nil {
			return nil, err
		}
	}
	// Editing a covered ORIGIN (not a replacement) through the ordinary path must
	// not strand its replacements: moving it to another date leaves them
	// referencing a date the origin no longer occupies, and shrinking its window
	// leaves a replacement outside the gap it fills. Rebuilding covers around a new
	// window is the atomic cancellation flow's job (it deletes the covers before
	// touching the origin, so this check never fires there), so here reject any
	// date or window change while covers still hang off this shift. The cover load
	// is skipped entirely when neither the date nor the window moved (#1841).
	originWindowChanged := !timezone.SameClockTime(shift.StartTime, existing.StartTime) ||
		!timezone.SameClockTime(shift.EndTime, existing.EndTime)
	if shift.OriginShiftID == nil && (shift.Date != existing.Date || originWindowChanged) {
		covers, err := s.repo.FindByOriginShiftID(ctx, shift.ID)
		if err != nil {
			return nil, fmt.Errorf("check covers before origin edit: %w", err)
		}
		for _, cover := range covers {
			if shift.Date != cover.Date {
				return nil, fmt.Errorf("%w: cannot move a shift that has replacements to another date", ErrShiftInvalid)
			}
			if !shift.Contains(cover) {
				return nil, fmt.Errorf("%w: cannot resize a shift so a replacement no longer fits within it", ErrShiftInvalid)
			}
		}
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
	// MoveShift may have changed the owner while this delete waited for the old
	// owner's lock. Refuse the stale delete instead of mutating the new owner's
	// schedule without holding that owner's lock.
	lockedExisting, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrShiftNotFound
		}
		return fmt.Errorf("reload staff shift before delete: %w", err)
	}
	if lockedExisting == nil {
		return ErrShiftNotFound
	}
	if lockedExisting.StaffID != existing.StaffID {
		return fmt.Errorf("%w: staff assignment changed", ErrShiftConflict)
	}
	existing = lockedExisting
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	// Deleting a series-backed row records the date as a series exception so
	// re-plans and splits never regenerate the removed occurrence (#1889).
	if existing.SeriesID != nil && s.exceptionRepo != nil {
		if err := s.recordSeriesException(ctx, existing, seriesOccurrenceDate(existing)); err != nil {
			return fmt.Errorf("record series exception for deleted shift: %w", err)
		}
	}
	s.getLogger().Info("staff shift deleted",
		"shift_id", id,
		"staff_id", existing.StaffID,
	)
	s.broadcastTimeTrackingChanged(ctx)
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
	lockedStaffID := existing.StaffID
	// A replacement is not itself a cancellable gap: it covers one. Cancelling a
	// cover would orphan the notion of "who covers the origin".
	if existing.OriginShiftID != nil {
		return nil, fmt.Errorf("%w: a replacement shift cannot be cancelled", ErrShiftInvalid)
	}

	// Discover who already covers this origin BEFORE taking the lock set. Each
	// existing cover is a shift owned by its own staff member and is editable or
	// deletable through the ordinary path — which locks only that cover's staff. A
	// cover this request drops (its staff absent from input.Replacements) would
	// otherwise never enter the lock set at all, so a parallel edit or delete of it
	// could interleave with the delete-and-rebuild below and be silently overwritten,
	// deleted twice, or resurrected from this operation's snapshot. Fold every
	// current cover's staff into the lock set so those rows serialize against us too
	// (#1841). MoveShift can change a cover's owner, so the authoritative reload
	// below verifies that every current owner was included in this snapshot.
	discoveredCovers, err := s.repo.FindByOriginShiftID(ctx, existing.ID)
	if err != nil {
		return nil, fmt.Errorf("discover existing replacements: %w", err)
	}

	// Lock every staff member this operation touches — the origin's own staff, each
	// requested replacement's staff, and each current cover's staff — up front in a
	// single stable sorted order, so two concurrent cross-covering cancellations
	// never lock the same pair in opposite orders (#1841 deadlock). The
	// per-replacement createShift calls below re-lock a subset of this same set;
	// advisory xact locks are re-grantable, so those are no-ops.
	lockIDs := []int64{existing.StaffID}
	for _, r := range input.Replacements {
		lockIDs = append(lockIDs, r.StaffID)
	}
	for _, cover := range discoveredCovers {
		lockIDs = append(lockIDs, cover.StaffID)
	}
	if err := s.lockStaffWritesOrdered(ctx, lockIDs); err != nil {
		return nil, err
	}
	lockedStaffIDs := make(map[int64]bool, len(lockIDs))
	for _, id := range lockIDs {
		if id > 0 {
			lockedStaffIDs[id] = true
		}
	}

	// existing was read before the advisory lock. A concurrent normal edit that
	// committed while this request waited for the lock leaves that snapshot stale,
	// so a cancellation that preserves the stored window (ApplyOriginEdits == false)
	// would otherwise revert the newer date/window/type/break and create the covers
	// on the stale date. Re-read the locked origin and build from that state (#1841).
	// StaffID is immutable, so the lock set computed above is still correct.
	existing, err = s.repo.FindByID(ctx, input.ShiftID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrShiftNotFound
		}
		return nil, fmt.Errorf("find staff shift: %w", err)
	}
	if existing == nil {
		return nil, ErrShiftNotFound
	}
	if existing.StaffID != lockedStaffID {
		// MoveShift won while this cancellation waited for the source owner's
		// lock. The new owner was not part of this operation's lock contract.
		return nil, fmt.Errorf("%w: staff assignment changed", ErrShiftConflict)
	}

	// Re-read the cover set under the lock — the pre-lock discovery read above only
	// seeded the lock set and may be stale. This authoritative post-lock snapshot
	// drives the delete-and-rebuild below (including the per-cover reason match), and
	// removing the current covers first lets the origin's flag flip without its old
	// replacements interfering with the reactivation overlap check.
	covers, err := s.repo.FindByOriginShiftID(ctx, existing.ID)
	if err != nil {
		return nil, fmt.Errorf("reload existing replacements: %w", err)
	}
	for _, cover := range covers {
		if !lockedStaffIDs[cover.StaffID] {
			// A replacement moved after discovery but before this transaction
			// acquired the origin lock. Taking the newly discovered lock now could
			// violate the global lock order and deadlock; reject the stale snapshot
			// so a retry discovers and locks the current owner from the outset.
			return nil, fmt.Errorf("%w: replacement staff assignment changed", ErrShiftConflict)
		}
	}
	// The rebuilt cover set re-sends each existing cover's own shift type; a type
	// deactivated after the cover was created must still be allowed to remain. Scope
	// that allowance to the staff member whose current cover actually carries the
	// type — otherwise a brand-new cover, or one transferring the inactive type to a
	// different person, would pass validation merely because an unrelated old cover
	// used the same id (#1841).
	grandfatheredTypesByStaff := make(map[int64]map[int64]bool)
	for _, cover := range covers {
		if cover.ShiftTypeID == nil {
			continue
		}
		types := grandfatheredTypesByStaff[cover.StaffID]
		if types == nil {
			types = make(map[int64]bool)
			grandfatheredTypesByStaff[cover.StaffID] = types
		}
		types[*cover.ShiftTypeID] = true
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
	updated, err := s.updateShiftWithOptions(ctx, originUpdate, StaffShiftUpdateOptions{
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
		s.broadcastTimeTrackingChanged(ctx)
		return result, nil
	}

	// Recreate the cover set against the now-cancelled origin. Each create
	// re-validates the origin (must be cancelled, same date, not self-covered) and
	// overlap for its own staff member, and grandfathers a since-deactivated type
	// only for the staff member whose preserved cover already carried it.
	//
	// A cover's change reason is set on the cover itself when it is edited directly
	// (the ordinary PUT), and ReplacementRequest carries no per-cover reason, so
	// blindly stamping the origin's reason onto every rebuilt cover would silently
	// overwrite an unchanged cover's own reason. Match each rebuilt cover back to
	// the existing cover it corresponds to (same staff + wall-clock window) and
	// carry that reason across; a genuinely new cover takes the origin's reason.
	// coverConsumed prevents two identical rows from both claiming one cover (#1841).
	coverConsumed := make([]bool, len(covers))
	for _, r := range input.Replacements {
		reason := input.ChangeReason
		for i, cover := range covers {
			if coverConsumed[i] || cover.StaffID != r.StaffID {
				continue
			}
			if timezone.SameClockTime(cover.StartTime, r.StartTime) &&
				timezone.SameClockTime(cover.EndTime, r.EndTime) {
				reason = cover.ChangeReason
				coverConsumed[i] = true
				break
			}
		}
		replacement := &scheduleModels.StaffShift{
			StaffID:       r.StaffID,
			Date:          existing.Date,
			StartTime:     r.StartTime,
			EndTime:       r.EndTime,
			BreakMinutes:  r.BreakMinutes,
			ShiftTypeID:   r.ShiftTypeID,
			OriginShiftID: &existing.ID,
			ChangeReason:  reason,
			CreatedBy:     input.ActorStaffID,
		}
		created, err := s.createShift(ctx, replacement, grandfatheredTypesByStaff[r.StaffID])
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
	s.broadcastTimeTrackingChanged(ctx)
	return result, nil
}
