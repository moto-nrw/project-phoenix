package planning

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

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
	Shift        *StaffShift
	Replacements []*StaffShift
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
	existing, err := s.findShiftOrNotFound(ctx, input.ShiftID, "find staff shift")
	if err != nil {
		return nil, err
	}
	// A replacement is not itself a cancellable gap: it covers one. Cancelling a
	// cover would orphan the notion of "who covers the origin".
	if existing.OriginShiftID != nil {
		return nil, fmt.Errorf("%w: a replacement shift cannot be cancelled", ErrShiftInvalid)
	}
	lockedStaffIDs, err := s.lockCancellationStaff(ctx, existing, input.Replacements)
	if err != nil {
		return nil, err
	}
	existing, err = s.reloadLockedCancellationOrigin(ctx, input.ShiftID, existing.StaffID)
	if err != nil {
		return nil, err
	}
	covers, err := s.reloadLockedCovers(ctx, existing.ID, lockedStaffIDs)
	if err != nil {
		return nil, err
	}
	grandfatheredTypesByStaff := grandfatheredCoverTypes(covers)
	if err := s.deleteCovers(ctx, covers); err != nil {
		return nil, err
	}
	originUpdate, updateOptions := cancellationOriginUpdate(existing, input)
	updated, err := s.updateShiftWithOptions(ctx, originUpdate, updateOptions)
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

	result.Replacements, err = s.rebuildReplacements(ctx, existing, input, covers, grandfatheredTypesByStaff)
	if err != nil {
		return nil, err
	}
	s.getLogger().Info("staff shift cancelled",
		"shift_id", updated.ID,
		"staff_id", updated.StaffID,
		"replacements", len(result.Replacements),
	)
	s.broadcastTimeTrackingChanged(ctx)
	return result, nil
}

// lockCancellationStaff takes the advisory lock set of a cancellation and
// returns the locked staff IDs.
//
// Discover who already covers this origin BEFORE taking the lock set. Each
// existing cover is a shift owned by its own staff member and is editable or
// deletable through the ordinary path — which locks only that cover's staff. A
// cover this request drops (its staff absent from the requested replacements)
// would otherwise never enter the lock set at all, so a parallel edit or delete
// of it could interleave with the delete-and-rebuild and be silently
// overwritten, deleted twice, or resurrected from this operation's snapshot.
// Fold every current cover's staff into the lock set so those rows serialize
// against us too (#1841). MoveShift can change a cover's owner, so the
// authoritative reload verifies that every current owner was included in this
// snapshot.
//
// Lock every staff member this operation touches — the origin's own staff, each
// requested replacement's staff, and each current cover's staff — up front in a
// single stable sorted order, so two concurrent cross-covering cancellations
// never lock the same pair in opposite orders (#1841 deadlock). The
// per-replacement createShift calls re-lock a subset of this same set;
// advisory xact locks are re-grantable, so those are no-ops.
func (s *staffShiftService) lockCancellationStaff(ctx context.Context, existing *StaffShift, replacements []ShiftReplacementInput) (map[int64]bool, error) {
	discoveredCovers, err := s.repo.FindByOriginShiftID(ctx, existing.ID)
	if err != nil {
		return nil, fmt.Errorf("discover existing replacements: %w", err)
	}
	lockIDs := []int64{existing.StaffID}
	for _, r := range replacements {
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
	return lockedStaffIDs, nil
}

// reloadLockedCancellationOrigin re-reads the origin after the lock is held.
//
// The first read happened before the advisory lock. A concurrent normal edit
// that committed while this request waited for the lock leaves that snapshot
// stale, so a cancellation that preserves the stored window (ApplyOriginEdits
// == false) would otherwise revert the newer date/window/type/break and create
// the covers on the stale date. Re-read the locked origin and build from that
// state (#1841). StaffID is immutable, so the lock set computed before is still
// correct.
func (s *staffShiftService) reloadLockedCancellationOrigin(ctx context.Context, shiftID, lockedStaffID int64) (*StaffShift, error) {
	existing, err := s.findShiftOrNotFound(ctx, shiftID, "find staff shift")
	if err != nil {
		return nil, err
	}
	if existing.StaffID != lockedStaffID {
		// MoveShift won while this cancellation waited for the source owner's
		// lock. The new owner was not part of this operation's lock contract.
		return nil, fmt.Errorf("%w: staff assignment changed", ErrShiftConflict)
	}
	return existing, nil
}

// reloadLockedCovers re-reads the cover set under the lock — the pre-lock
// discovery read only seeded the lock set and may be stale. This authoritative
// post-lock snapshot drives the delete-and-rebuild (including the per-cover
// reason match), and removing the current covers first lets the origin's flag
// flip without its old replacements interfering with the reactivation overlap
// check.
func (s *staffShiftService) reloadLockedCovers(ctx context.Context, originID int64, lockedStaffIDs map[int64]bool) ([]*StaffShift, error) {
	covers, err := s.repo.FindByOriginShiftID(ctx, originID)
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
	return covers, nil
}

// deleteCovers removes the origin's current replacements before the origin's
// flag flips.
func (s *staffShiftService) deleteCovers(ctx context.Context, covers []*StaffShift) error {
	for _, cover := range covers {
		if err := s.repo.Delete(ctx, cover.ID); err != nil {
			return fmt.Errorf("remove existing replacement: %w", err)
		}
	}
	return nil
}

// grandfatheredCoverTypes maps each cover's staff member to the shift types
// their current covers carry.
//
// The rebuilt cover set re-sends each existing cover's own shift type; a type
// deactivated after the cover was created must still be allowed to remain.
// Scope that allowance to the staff member whose current cover actually carries
// the type — otherwise a brand-new cover, or one transferring the inactive type
// to a different person, would pass validation merely because an unrelated old
// cover used the same id (#1841).
func grandfatheredCoverTypes(covers []*StaffShift) map[int64]map[int64]bool {
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
	return grandfatheredTypesByStaff
}

// cancellationOriginUpdate builds the origin's update for the normal update
// path, so the series-detach and (on reactivation) overlap rules apply. Notes
// are always preserved (the modal has no notes field); the window/type default
// to the stored values but are overwritten with the caller's edits when
// supplied, so an edit made in the same save is honoured and a reactivation's
// overlap check runs against the edited window (#1841).
func cancellationOriginUpdate(existing *StaffShift, input CancelShiftInput) (*StaffShift, StaffShiftUpdateOptions) {
	originUpdate := &StaffShift{
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
	return originUpdate, StaffShiftUpdateOptions{
		PreserveExistingNotes:     true,
		PreserveExistingShiftType: preserveType,
	}
}

// rebuildReplacements recreates the cover set against the now-cancelled
// origin. Each create re-validates the origin (must be cancelled, same date,
// not self-covered) and overlap for its own staff member, and grandfathers a
// since-deactivated type only for the staff member whose preserved cover
// already carried it.
func (s *staffShiftService) rebuildReplacements(
	ctx context.Context,
	existing *StaffShift,
	input CancelShiftInput,
	covers []*StaffShift,
	grandfatheredTypesByStaff map[int64]map[int64]bool,
) ([]*StaffShift, error) {
	var created []*StaffShift
	coverConsumed := make([]bool, len(covers))
	for _, r := range input.Replacements {
		replacement := &StaffShift{
			StaffID:       r.StaffID,
			Date:          existing.Date,
			StartTime:     r.StartTime,
			EndTime:       r.EndTime,
			BreakMinutes:  r.BreakMinutes,
			ShiftTypeID:   r.ShiftTypeID,
			OriginShiftID: &existing.ID,
			ChangeReason:  matchedCoverReason(covers, coverConsumed, r, input.ChangeReason),
			CreatedBy:     input.ActorStaffID,
		}
		shift, err := s.createShift(ctx, replacement, grandfatheredTypesByStaff[r.StaffID])
		if err != nil {
			return nil, err
		}
		created = append(created, shift)
	}
	return created, nil
}

// matchedCoverReason picks the change reason of a rebuilt cover.
//
// A cover's change reason is set on the cover itself when it is edited directly
// (the ordinary PUT), and ReplacementRequest carries no per-cover reason, so
// blindly stamping the origin's reason onto every rebuilt cover would silently
// overwrite an unchanged cover's own reason. Match each rebuilt cover back to
// the existing cover it corresponds to (same staff + wall-clock window) and
// carry that reason across; a genuinely new cover takes the origin's reason.
// coverConsumed prevents two identical rows from both claiming one cover (#1841).
func matchedCoverReason(covers []*StaffShift, coverConsumed []bool, r ShiftReplacementInput, originReason *string) *string {
	for i, cover := range covers {
		if coverConsumed[i] || cover.StaffID != r.StaffID {
			continue
		}
		if calendar.SameClockTime(cover.StartTime, r.StartTime) &&
			calendar.SameClockTime(cover.EndTime, r.EndTime) {
			coverConsumed[i] = true
			return cover.ChangeReason
		}
	}
	return originReason
}
