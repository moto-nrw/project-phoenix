package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// Sentinel errors for the admin restore flow (#2157). Mapped to HTTP
// statuses by the admin handler.
var (
	ErrRestoreNothingWithdrawn = errors.New("request has no withdrawn children to restore")
	ErrRestorePhaseInactive    = errors.New("enrollment phase is not active")
	ErrRestoreDuplicateActive  = errors.New("an active enrollment already exists for a child of this request")
)

// RestoreOutcome is what the admin handler gets back from RestoreWithdrawn.
// WaitlistedChildIDs is the subset of RestoredChildIDs that came back as
// waitlisted instead of submitted because an offering is meanwhile full.
type RestoreOutcome struct {
	RestoredChildIDs   []int64
	WaitlistedChildIDs []int64
}

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
// the capacity gate re-runs under the same offering row locks Submit takes
// (applyCapacityOverflowCore). If another family claimed the freed slots
// after the withdraw, the affected children come back as waitlisted instead
// of submitted — exactly what Submit would have produced — or, in a
// reject-mode phase, the restore fails with ErrCareOfferingFull.
//
// The append-only audit row (who restored what, when) is written in the
// same tenant transaction; if it fails the restore rolls back with it.
// Must run inside the handler-provided tenant transaction, like Decide.
func (s *decisionService) RestoreWithdrawn(ctx context.Context, requestID, restoredBy int64) (*RestoreOutcome, error) {
	if requestID <= 0 {
		return nil, ErrDecisionRequestNotFound
	}
	if s.RestorationAuditRepo == nil {
		return nil, fmt.Errorf("restore: audit repository not configured")
	}

	// Lock the parent before its children — same order as Decide, cleanup,
	// and the withdraw path, so restore serializes cleanly against all of
	// them without lock inversion.
	request, err := s.RequestRepo.FindByIDForUpdate(ctx, requestID)
	if err != nil {
		return nil, ErrDecisionRequestNotFound
	}
	children, err := s.RequestChildRepo.ListByRequestIDForUpdate(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("restore: load children: %w", err)
	}

	withdrawn := make([]*enrollmentModels.RequestChild, 0, len(children))
	for _, child := range children {
		if child.Status == enrollmentModels.ChildStatusWithdrawn {
			withdrawn = append(withdrawn, child)
		}
	}
	if len(withdrawn) == 0 {
		return nil, ErrRestoreNothingWithdrawn
	}

	phase, err := s.PhaseRepo.FindByID(ctx, request.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("restore: load phase: %w", err)
	}
	if !phase.IsActive {
		return nil, ErrRestorePhaseInactive
	}

	// Serialize against concurrent submissions for the same (phase,
	// guardian email) before re-running the submit-time duplicate check,
	// exactly like Submit does. The lock auto-releases at tx end.
	emailLC := strings.ToLower(strings.TrimSpace(request.GuardianEmail))
	if err := s.RequestRepo.AcquireSubmissionDedupLock(ctx, phase.ID, fnvHash64(emailLC)); err != nil {
		return nil, fmt.Errorf("restore: acquire dedup lock: %w", err)
	}
	dupKeys := make([]enrollmentModels.DuplicateChildKey, 0, len(withdrawn))
	for _, child := range withdrawn {
		dupKeys = append(dupKeys, enrollmentModels.DuplicateChildKey{
			FirstName: child.FirstName,
			LastName:  child.LastName,
		})
	}
	// The error stays name-free on purpose: it flows into handler logs and
	// the API response, and student names must not reach Info-level logs.
	dupes, err := s.RequestRepo.FindActiveDuplicateExcludingRequest(ctx, phase.ID, request.GuardianEmail, dupKeys, request.ID)
	if err != nil {
		return nil, fmt.Errorf("restore: duplicate check: %w", err)
	}
	if len(dupes) > 0 {
		return nil, ErrRestoreDuplicateActive
	}

	// Existing-students children are pinned to a live student; a restore
	// must not produce a second active request targeting that student
	// (different-email submissions bypass the email-scoped check above).
	for _, child := range withdrawn {
		if child.MatchedStudentID == nil {
			continue
		}
		if err := s.RequestRepo.AcquireExistingStudentMatchLock(ctx, phase.ID); err != nil {
			return nil, fmt.Errorf("restore: acquire existing-student match lock: %w", err)
		}
		has, err := s.RequestRepo.HasActiveRequestForMatchedStudent(ctx, phase.ID, *child.MatchedStudentID, child.ID)
		if err != nil {
			return nil, fmt.Errorf("restore: matched-student duplicate check: %w", err)
		}
		if has {
			return nil, ErrRestoreDuplicateActive
		}
	}

	// Capacity gate, under the same offering row locks Submit takes. Runs
	// BEFORE the status flip so the restored children's own (still
	// withdrawn) claims cannot inflate the count.
	waitlistedIDs, err := s.restoreCapacityWaitlist(ctx, phase, withdrawn)
	if err != nil {
		return nil, err
	}

	restoredIDs, err := s.RequestChildRepo.RestoreWithdrawnByRequestID(ctx, requestID, waitlistedIDs)
	if err != nil {
		return nil, fmt.Errorf("restore: update children: %w", err)
	}
	if len(restoredIDs) != len(withdrawn) {
		return nil, fmt.Errorf("restore: expected %d restored children, got %d", len(withdrawn), len(restoredIDs))
	}

	if request.WithdrawnAt != nil {
		if err := s.RequestRepo.ClearWithdrawn(ctx, requestID); err != nil {
			return nil, fmt.Errorf("restore: clear withdrawn_at: %w", err)
		}
	}

	var actor *int64
	if restoredBy > 0 {
		actor = &restoredBy
	}
	if err := s.RestorationAuditRepo.Create(ctx, &auditModels.EnrollmentRestoration{
		RequestID:      requestID,
		ChildIDs:       restoredIDs,
		ActorAccountID: actor,
		RestoredAt:     time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("restore: write audit event: %w", err)
	}

	s.Logger.Info("enrollment request restored",
		slog.Int64("request_id", requestID),
		slog.Int("restored_children", len(restoredIDs)),
		slog.Int("waitlisted_children", len(waitlistedIDs)),
		slog.Int64("restored_by", restoredBy),
	)
	return &RestoreOutcome{RestoredChildIDs: restoredIDs, WaitlistedChildIDs: waitlistedIDs}, nil
}

// restoreCapacityWaitlist re-runs the submit-time capacity gate for the
// children about to be restored and returns the ids that must come back as
// waitlisted because an offering is meanwhile full. The claims are the
// children's surviving request_child_offerings rows, reduced to those that
// still cover a day of the phase's remaining capacity window (a dated
// switch whose interval already ended holds no future slot); each claim
// keeps its ValidFrom/ValidUntil so it is only checked against occupancy
// inside its own interval, never against capacity pressure it doesn't
// overlap. Reject-mode phases surface ErrCareOfferingFull instead; a
// meanwhile-deactivated offering fails closed with ErrCareOfferingClosed.
func (s *decisionService) restoreCapacityWaitlist(
	ctx context.Context,
	phase *enrollmentModels.Phase,
	withdrawn []*enrollmentModels.RequestChild,
) ([]int64, error) {
	if s.RequestChildOfferingRepo == nil || s.CareOfferingRepo == nil {
		return nil, nil
	}
	offeringsEnabled, err := s.resolveDecisionBool(ctx, configModel.KeyEnrollmentCareOfferingsEnabled, true)
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
	rows, err := s.RequestChildOfferingRepo.ListByRequestChildIDs(ctx, childIDs)
	if err != nil {
		return nil, fmt.Errorf("restore: load offering selections: %w", err)
	}
	capacityFrom := timezone.TodayDate()
	if phase.ServiceStartDate.After(capacityFrom) {
		capacityFrom = phase.ServiceStartDate
	}
	claimsByChild := make(map[int64][]OfferingClaim)
	anyClaim := false
	for _, row := range rows {
		// ValidUntil is exclusive: an interval ending on or before the
		// window start holds no remaining slot.
		if row.ValidUntil != nil && !row.ValidUntil.After(capacityFrom) {
			continue
		}
		claimsByChild[row.RequestChildID] = append(claimsByChild[row.RequestChildID], OfferingClaim{
			OfferingID: row.CareOfferingID,
			ValidFrom:  row.ValidFrom,
			ValidUntil: row.ValidUntil,
		})
		anyClaim = true
	}
	if !anyClaim {
		return nil, nil
	}

	claims := make([][]OfferingClaim, len(withdrawn))
	for i, child := range withdrawn {
		claims[i] = claimsByChild[child.ID]
	}
	overrides, err := applyCapacityOverflowCore(ctx, s.CareOfferingRepo, s.RequestChildOfferingRepo,
		func(ctx context.Context) (bool, error) {
			return s.resolveDecisionBool(ctx, configModel.KeyEnrollmentWaitlistEnabled, true)
		},
		phase, claims, nil, nil, childIDs)
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
