package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
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
type RestoreOutcome struct {
	RestoredChildIDs []int64
}

// RestoreWithdrawn undoes a parent-initiated withdraw (#2157): every
// withdrawn child of the request goes back to submitted with cleared review
// metadata, and requests.withdrawn_at is nulled — exactly the fields the
// withdraw path stamped. Children that were terminal before the withdraw
// (approved/rejected) are untouched because the withdraw path never changed
// them in the first place.
//
// Guards mirror the submit flow: the phase must still be active, and the
// submit-time duplicate checks re-run under the same advisory locks so a
// restore cannot produce a second active request for the same child in the
// phase (e.g. when the parent already re-submitted after withdrawing).
// Restored children count against offering capacity again automatically —
// every capacity query filters on non-terminal statuses.
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

	restoredIDs, err := s.RequestChildRepo.RestoreWithdrawnByRequestID(ctx, requestID)
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
		slog.Int64("restored_by", restoredBy),
	)
	return &RestoreOutcome{RestoredChildIDs: restoredIDs}, nil
}
