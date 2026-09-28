package planning

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/realtime"
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
	ListShifts(ctx context.Context, start, end timezone.Date) ([]*StaffShift, error)
	// ListShiftsForStaff returns one staff member's shifts in the range.
	ListShiftsForStaff(ctx context.Context, staffID int64, start, end timezone.Date) ([]*StaffShift, error)
	// CreateShift validates and persists a new shift.
	CreateShift(ctx context.Context, shift *StaffShift) (*StaffShift, error)
	// UpdateShift validates and persists changes to an existing shift.
	UpdateShift(ctx context.Context, shift *StaffShift) (*StaffShift, error)
	// UpdateShiftWithOptions validates and persists changes with update-specific merge options.
	UpdateShiftWithOptions(ctx context.Context, shift *StaffShift, opts StaffShiftUpdateOptions) (*StaffShift, error)
	// MoveShift atomically and retry-safely moves one concrete shift to another
	// person and/or slot while preserving its identity and metadata.
	MoveShift(ctx context.Context, input MoveShiftInput) (*StaffShift, error)
	// DeleteShift removes a shift.
	DeleteShift(ctx context.Context, id int64) error
	// ApplyCancellation atomically cancels (or reactivates) a shift and replaces
	// its full set of replacement covers in one operation (#1841). Cancelling
	// with N replacements, reactivating (which removes every replacement), and
	// re-planning the covers all run as a single unit so a partial failure never
	// leaves a half-changed schedule.
	ApplyCancellation(ctx context.Context, input CancelShiftInput) (*CancelShiftResult, error)
}

// StaffShiftOption binds an optional collaborator at construction time. The
// options replaced the post-construction setters when the service moved out
// of services/schedule (#3219): the composition surface guard records
// mutable wiring per package, and a relocated setter counts as growth.
type StaffShiftOption func(*staffShiftService)

// WithStaffShiftSeriesExceptions wires the series exception repository
// (#1889). When set, deleting a series-backed shift records the date as a
// series exception so re-plans never regenerate the removed occurrence.
// Optional so unit tests without series stay unchanged.
func WithStaffShiftSeriesExceptions(repo staffShiftSeriesExceptionRows) StaffShiftOption {
	return func(s *staffShiftService) { s.exceptionRepo = repo }
}

// WithStaffShiftDeviationEvents wires the Änderungsprotokoll repository
// (#1884); when set, MoveShift appends a shift_moved audit event inside the
// same tenant tx.
func WithStaffShiftDeviationEvents(repo auditModels.DeviationEventRepository) StaffShiftOption {
	return func(s *staffShiftService) { s.deviationEventRepo = repo }
}

// WithStaffShiftBroadcaster injects the tenant-wide SSE broadcaster used to
// invalidate time-tracking views after committed shift writes.
func WithStaffShiftBroadcaster(broadcaster realtime.Broadcaster) StaffShiftOption {
	return func(s *staffShiftService) { s.broadcaster = broadcaster }
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
	repo          staffShiftRows
	staffRepo     staffDirectory
	shiftTypes    ShiftTypeService
	exceptionRepo staffShiftSeriesExceptionRows
	// deviationEventRepo appends shift_moved Änderungsprotokoll rows (#1884).
	// Optional so unit tests without audit stay unchanged; production wiring
	// (services/factory.go) always sets it, making the event write fail-closed
	// there (an audit failure rolls the move back).
	deviationEventRepo auditModels.DeviationEventRepository
	// lockStaffShifts serializes the writes of one staff member. Nil skips the
	// lock, which is what unit tests without a database exercise.
	lockStaffShifts func(ctx context.Context, staffID int64) error
	broadcaster     realtime.Broadcaster
	logger          *slog.Logger
	// lockObserver, when set, is invoked with the sorted, de-duplicated staff-id
	// set of every lockStaffWritesOrdered acquisition. It is a test-only seam: the
	// lock is a no-op without one bound, so a unit test cannot otherwise assert
	// which staff members an operation serializes on (e.g. that a dropped cover's
	// staff is folded into the cancellation lock set, #1841). Nil in production.
	lockObserver func(staffIDs []int64)
}

// NewStaffShiftService creates a new staff shift service. lockStaffShifts is
// the per-staff write lock that makes the overlap check safe under
// concurrency; the composition binds the advisory lock over its database
// handle, and a nil lock (unit tests) skips it. shiftTypes resolves the active
// flag when a shift is assigned a type; it may be nil in unit tests that never
// attach a shift type.
func NewStaffShiftService(repo staffShiftRows, staffRepo staffDirectory, shiftTypes ShiftTypeService, lockStaffShifts func(ctx context.Context, staffID int64) error, logger *slog.Logger, opts ...StaffShiftOption) StaffShiftService {
	service := &staffShiftService{repo: repo, staffRepo: staffRepo, shiftTypes: shiftTypes, lockStaffShifts: lockStaffShifts, logger: logger}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func (s *staffShiftService) broadcastTimeTrackingChanged(ctx context.Context) {
	realtimeevents.QueueStaffTimeTrackingChanged(ctx, s.broadcaster, s.getLogger())
}

// lockShiftWrites takes the per-staff write lock before the overlap
// read-check so concurrent writers serialize on the same staff member.
// Unit tests construct the service without a lock; they exercise the overlap
// logic, not concurrency, so a nil lock is a no-op.
func (s *staffShiftService) lockShiftWrites(ctx context.Context, staffID int64) error {
	if s.lockStaffShifts == nil {
		return nil
	}
	return s.lockStaffShifts(ctx, staffID)
}

func (s *staffShiftService) getLogger() *slog.Logger {
	return loggerOrDefault(s.logger)
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

func (s *staffShiftService) ListShifts(ctx context.Context, start, end timezone.Date) ([]*StaffShift, error) {
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

func (s *staffShiftService) ListShiftsForStaff(ctx context.Context, staffID int64, start, end timezone.Date) ([]*StaffShift, error) {
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
func (s *staffShiftService) attachShiftTypes(ctx context.Context, shifts []*StaffShift) error {
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
	byID := make(map[int64]*ShiftType, len(types))
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
func (s *staffShiftService) checkOverlap(ctx context.Context, shift *StaffShift) error {
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
func (s *staffShiftService) loadOriginShift(ctx context.Context, originID int64) (*StaffShift, error) {
	origin, err := s.repo.FindByID(ctx, originID)
	if err != nil {
		if modelBase.IsNoRows(err) {
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
func (s *staffShiftService) validateOriginLink(ctx context.Context, shift *StaffShift) error {
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

func (s *staffShiftService) CreateShift(ctx context.Context, shift *StaffShift) (*StaffShift, error) {
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
func (s *staffShiftService) createShift(ctx context.Context, shift *StaffShift, allowedInactiveTypes map[int64]bool) (*StaffShift, error) {
	if err := shift.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrShiftInvalid, err.Error())
	}
	staff, err := s.staffRepo.FindByID(ctx, shift.StaffID)
	if err != nil {
		if modelBase.IsNoRows(err) {
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

func (s *staffShiftService) UpdateShift(ctx context.Context, shift *StaffShift) (*StaffShift, error) {
	return s.UpdateShiftWithOptions(ctx, shift, StaffShiftUpdateOptions{})
}

func (s *staffShiftService) UpdateShiftWithOptions(ctx context.Context, shift *StaffShift, opts StaffShiftUpdateOptions) (*StaffShift, error) {
	updated, err := s.updateShiftWithOptions(ctx, shift, opts)
	if err != nil {
		return nil, err
	}
	if !opts.SuppressTimeTrackingBroadcast {
		s.broadcastTimeTrackingChanged(ctx)
	}
	return updated, nil
}

func (s *staffShiftService) updateShiftWithOptions(ctx context.Context, shift *StaffShift, opts StaffShiftUpdateOptions) (*StaffShift, error) {
	if shift.ID <= 0 {
		return nil, ErrShiftNotFound
	}
	existing, err := s.repo.FindByID(ctx, shift.ID)
	if err != nil {
		if modelBase.IsNoRows(err) {
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
		if modelBase.IsNoRows(err) {
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
		if modelBase.IsNoRows(err) {
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
		if modelBase.IsNoRows(err) {
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
