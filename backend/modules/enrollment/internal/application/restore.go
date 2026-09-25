package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// RestoreWithdrawn undoes a parent-initiated withdraw (#2157): every
// withdrawn child of the request goes back to submitted with cleared review
// metadata, and requests.withdrawn_at is nulled — exactly the fields the
// withdraw path stamped. Children that were terminal before the withdraw
// (approved/rejected) are untouched because the withdraw path never changed
// them in the first place.
//
// Guards mirror the submit flow: the phase must still be active, the
// submit-time duplicate checks re-run under the same advisory locks so a
// restore cannot produce a second active request for the same child in the
// phase (e.g. when the parent already re-submitted after withdrawing), and
// the capacity gate re-runs under the same offering row locks a submission
// takes. If another family claimed the freed slots after the withdraw, the
// affected children come back as waitlisted instead of submitted — exactly
// what a submission would have produced — or, in a reject-mode phase, the
// restore fails with ErrCareOfferingFull.
//
// The append-only audit row (who restored what, when) is written in the
// same tenant transaction; if it fails the restore rolls back with it.
// Joins the handler transaction, or starts one for a standalone caller; an
// invalid request id or a missing audit trail is refused before either.
func (d *Decisions) RestoreWithdrawn(ctx context.Context, requestID, restoredBy int64) (*enrollment.RestoreOutcome, error) {
	if requestID <= 0 {
		return nil, careplan.ErrBookingRequestNotFound
	}
	if d.deps.Restorations == nil {
		return nil, fmt.Errorf("restore: audit repository not configured")
	}
	var outcome *enrollment.RestoreOutcome
	err := d.deps.Runtime.Transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		outcome, err = d.restoreWithdrawn(txCtx, requestID, restoredBy)
		return err
	})
	if err != nil {
		return nil, err
	}
	return outcome, nil
}

func (d *Decisions) restoreWithdrawn(ctx context.Context, requestID, restoredBy int64) (*enrollment.RestoreOutcome, error) {
	request, withdrawn, err := d.lockWithdrawnChildren(ctx, requestID)
	if err != nil {
		return nil, err
	}
	phase, err := d.deps.Phases.Phase(ctx, request.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("restore: load phase: %w", err)
	}
	if !phase.IsActive {
		return nil, enrollment.ErrRestorePhaseInactive
	}
	if err := d.checkRestoreDuplicates(ctx, phase, request, withdrawn); err != nil {
		return nil, err
	}
	// Capacity gate, under the same offering row locks a submission takes.
	// Runs BEFORE the status flip so the restored children's own (still
	// withdrawn) claims cannot inflate the count.
	waitlistedIDs, err := d.restoreCapacityWaitlist(ctx, phase, withdrawn)
	if err != nil {
		return nil, err
	}
	restoredIDs, err := d.restoreChildren(ctx, request, withdrawn, waitlistedIDs)
	if err != nil {
		return nil, err
	}
	var actor *int64
	if restoredBy > 0 {
		actor = &restoredBy
	}
	if err := d.deps.Restorations.RecordRestoration(ctx, Restoration{
		RequestID:      requestID,
		ChildIDs:       restoredIDs,
		ActorAccountID: actor,
		RestoredAt:     time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("restore: write audit event: %w", err)
	}
	d.logger().Info("enrollment request restored",
		slog.Int64("request_id", requestID),
		slog.Int("restored_children", len(restoredIDs)),
		slog.Int("waitlisted_children", len(waitlistedIDs)),
		slog.Int64("restored_by", restoredBy),
	)
	return &enrollment.RestoreOutcome{RestoredChildIDs: restoredIDs, WaitlistedChildIDs: waitlistedIDs}, nil
}

// lockWithdrawnChildren locks the parent before its children — same order as
// Decide, cleanup, and the withdraw path, so restore serializes cleanly
// against all of them without lock inversion — and returns the withdrawn
// children.
func (d *Decisions) lockWithdrawnChildren(ctx context.Context, requestID int64) (*enrollmentModels.Request, []*RequestChild, error) {
	request, err := decodedRequestByID(ctx, d.deps.Requests, requestID, true)
	if err != nil {
		// Only a genuinely missing row becomes the 404 sentinel; connection
		// and query failures keep flowing so the handler's transient-503 /
		// default-500 mapping still sees them.
		if d.deps.Runtime.NotFound(err) {
			return nil, nil, careplan.ErrBookingRequestNotFound
		}
		return nil, nil, fmt.Errorf("restore: load request: %w", err)
	}
	children, err := decodedChildrenOfRequest(ctx, d.deps.Children, requestID, true)
	if err != nil {
		return nil, nil, fmt.Errorf("restore: load children: %w", err)
	}
	withdrawn := make([]*RequestChild, 0, len(children))
	for _, child := range children {
		if child.Status == enrollmentModels.ChildStatusWithdrawn {
			withdrawn = append(withdrawn, child)
		}
	}
	if len(withdrawn) == 0 {
		return nil, nil, enrollment.ErrRestoreNothingWithdrawn
	}
	return request, withdrawn, nil
}

// checkRestoreDuplicates serializes against concurrent submissions for the
// same (phase, guardian email) before re-running the submit-time duplicate
// check, exactly like a submission does. The lock auto-releases at tx end.
// The error stays name-free on purpose: it flows into handler logs and the
// API response, and student names must not reach Info-level logs.
func (d *Decisions) checkRestoreDuplicates(ctx context.Context, phase *enrollment.Phase, request *enrollmentModels.Request, withdrawn []*RequestChild) error {
	emailLC := strings.ToLower(strings.TrimSpace(request.GuardianEmail))
	if err := d.deps.Requests.AcquireSubmissionDedupLock(ctx, phase.ID, enrollment.SubmissionDedupLockKey(emailLC)); err != nil {
		return fmt.Errorf("restore: acquire dedup lock: %w", err)
	}
	dupKeys := make([]enrollment.DuplicateChildKey, 0, len(withdrawn))
	for _, child := range withdrawn {
		dupKeys = append(dupKeys, enrollment.DuplicateChildKey{FirstName: child.FirstName, LastName: child.LastName})
	}
	dupes, err := d.deps.Requests.ActiveDuplicateChildren(ctx, phase.ID, request.GuardianEmail, dupKeys, request.ID)
	if err != nil {
		return fmt.Errorf("restore: duplicate check: %w", err)
	}
	if len(dupes) > 0 {
		return enrollment.ErrRestoreDuplicateActive
	}
	return d.checkRestoreMatchedStudents(ctx, phase, withdrawn)
}

// checkRestoreMatchedStudents keeps an existing-students child pinned to a
// live student from coming back next to a second active request targeting
// that student (different-email submissions bypass the email-scoped check).
func (d *Decisions) checkRestoreMatchedStudents(ctx context.Context, phase *enrollment.Phase, withdrawn []*RequestChild) error {
	for _, child := range withdrawn {
		if child.MatchedStudentID == nil {
			continue
		}
		if err := d.deps.Requests.AcquireExistingStudentMatchLock(ctx, phase.ID); err != nil {
			return fmt.Errorf("restore: acquire existing-student match lock: %w", err)
		}
		has, err := d.deps.Requests.HasActiveRequestForMatchedStudent(ctx, phase.ID, *child.MatchedStudentID, child.ID)
		if err != nil {
			return fmt.Errorf("restore: matched-student duplicate check: %w", err)
		}
		if has {
			return enrollment.ErrRestoreDuplicateActive
		}
	}
	return nil
}

// restoreChildren flips the withdrawn children back and clears the request's
// withdrawal.
func (d *Decisions) restoreChildren(ctx context.Context, request *enrollmentModels.Request, withdrawn []*RequestChild, waitlistedIDs []int64) ([]int64, error) {
	restoredIDs, err := d.deps.Children.RestoreWithdrawnChildren(ctx, request.ID, waitlistedIDs)
	if err != nil {
		return nil, fmt.Errorf("restore: update children: %w", err)
	}
	if len(restoredIDs) != len(withdrawn) {
		return nil, fmt.Errorf("restore: expected %d restored children, got %d", len(withdrawn), len(restoredIDs))
	}
	if request.WithdrawnAt != nil {
		if err := d.deps.Requests.SetRequestWithdrawal(ctx, request.ID, nil); err != nil {
			return nil, fmt.Errorf("restore: clear withdrawn_at: %w", err)
		}
	}
	return restoredIDs, nil
}

// restoreCapacityWaitlist re-runs the submit-time capacity gate for the
// children about to be restored and returns the ids that must come back as
// waitlisted because an offering is meanwhile full. The claims are the
// children's surviving bookings, reduced to those that still cover a day of
// the phase's remaining capacity window (a dated switch whose interval
// already ended holds no future slot); each claim keeps its ValidFrom /
// ValidUntil so it is only checked against occupancy inside its own
// interval, never against capacity pressure it doesn't overlap — while
// restored siblings whose intervals do overlap queue against each other, so
// a partially overlapping pair cannot jointly overbook a slot both fit into
// alone. Reject-mode phases surface ErrCareOfferingFull instead; a
// meanwhile-deactivated offering fails closed with ErrCareOfferingClosed.
func (d *Decisions) restoreCapacityWaitlist(ctx context.Context, phase *enrollment.Phase, withdrawn []*RequestChild) ([]int64, error) {
	if d.deps.Children == nil || d.deps.Offerings == nil {
		return nil, nil
	}
	offeringsEnabled, err := d.careOfferingsEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("restore: resolve care offerings setting: %w", err)
	}
	if !offeringsEnabled {
		return nil, nil
	}
	childIDs := make([]int64, 0, len(withdrawn))
	for _, child := range withdrawn {
		childIDs = append(childIDs, child.ID)
	}
	rows, err := enrollment.OfferingHistoryRecordsForChildren(ctx, d.deps.Children, childIDs)
	if err != nil {
		return nil, fmt.Errorf("restore: load offering selections: %w", err)
	}
	claims, anyClaim := restoreClaims(phase, rows, withdrawn)
	if !anyClaim {
		return nil, nil
	}
	overrides, err := d.capacity.ApplyCapacityOverflow(ctx, enrollment.CapacityCheck{
		Phase: phase, Claims: claims, ReplacedRequestChildIDs: childIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("restore: capacity gate: %w", err)
	}
	waitlisted := make([]int64, 0, len(overrides))
	for idx, status := range overrides {
		if status == enrollmentModels.ChildStatusWaitlisted {
			waitlisted = append(waitlisted, withdrawn[idx].ID)
		}
	}
	return waitlisted, nil
}

// restoreClaims collects the capacity claims per withdrawn child. ValidUntil
// is exclusive: an interval ending on or before the window start holds no
// remaining slot.
func restoreClaims(phase *enrollment.Phase, rows []*enrollment.RequestChildOfferingRecord, withdrawn []*RequestChild) ([][]enrollment.OfferingClaim, bool) {
	capacityFrom := calendar.TodayDate()
	if calendar.Date(phase.ServiceStartDate).After(capacityFrom) {
		capacityFrom = calendar.Date(phase.ServiceStartDate)
	}
	claimsByChild := make(map[int64][]enrollment.OfferingClaim)
	anyClaim := false
	for _, row := range rows {
		if row.ValidUntil != nil && !row.ValidUntil.After(capacityFrom) {
			continue
		}
		claimsByChild[row.RequestChildID] = append(claimsByChild[row.RequestChildID], enrollment.OfferingClaim{
			OfferingID: row.CareOfferingID,
			ValidFrom:  row.ValidFrom,
			ValidUntil: row.ValidUntil,
		})
		anyClaim = true
	}
	claims := make([][]enrollment.OfferingClaim, len(withdrawn))
	for i, child := range withdrawn {
		claims[i] = claimsByChild[child.ID]
	}
	return claims, anyClaim
}
