package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// Correcting a Stammdaten decision (#2267, stories 21-23).
//
// rejected → approved re-runs the ordinary apply, so every guard a fresh
// approval passes runs again.
//
// approved → rejected has to put the child's record back. It restores
// old_value ONLY while the live value still equals what the approval wrote:
// if the office (or an import, or another request) changed the field since,
// the value on record is newer than this request and overwriting it would
// silently discard an edit nobody can recover. In that case the correction
// refuses with ErrParentRequestCorrectionUnsupported and names the value that
// is there now, so the reviewer can decide what should actually stand.
func (s *masterDataReviewService) Correct(
	ctx context.Context,
	requestID int64,
	approve bool,
	expectedVersion, reason string,
	reviewedBy int64,
) error {
	if requestID <= 0 {
		return ErrReviewNotFound
	}
	req, err := s.changeRequestRepo.FindByIDForUpdate(ctx, requestID)
	if err != nil {
		if errors.Is(err, userModels.ErrChangeRequestNotFound) {
			return ErrReviewNotFound
		}
		return fmt.Errorf("review: load request for correction: %w", err)
	}
	if !isCorrectableMasterDataStatus(req.Status) {
		return ErrParentRequestNotDecided
	}
	if expectedVersion != "" && ParentRequestVersion(req.UpdatedAt) != expectedVersion {
		return ErrParentRequestStale
	}
	// Re-authorize against the CURRENT policy: a group leader who has since
	// lost the group must not be able to correct its decisions.
	if err := s.authorizeMasterDataDecision(ctx, req.StudentID); err != nil {
		return err
	}

	trimmed := strings.TrimSpace(reason)
	var reasonPtr *string
	if trimmed != "" {
		reasonPtr = &trimmed
	}

	if approve {
		applyCtx, _ := userModels.ContextWithCompanionChangeRecorder(ctx)
		if err := s.applyApprovedChange(applyCtx, req, reviewedBy); err != nil {
			return err
		}
		if err := s.changeRequestRepo.Redecide(
			ctx, req.ID, userModels.DataChangeStatusApproved, reasonPtr, reviewedBy, true,
		); err != nil {
			return err
		}
	} else {
		if err := s.revertApprovedMasterData(ctx, req, reviewedBy); err != nil {
			return err
		}
		if err := s.changeRequestRepo.Redecide(
			ctx, req.ID, userModels.DataChangeStatusRejected, reasonPtr, reviewedBy, false,
		); err != nil {
			return err
		}
	}
	// The ledger keeps BOTH decisions: the correction is a new entry naming
	// what it replaced, never an edit of the old one. Written inside the
	// correction's own transaction, so a rollback leaves no trace of it.
	newStatus := userModels.DataChangeStatusRejected
	if approve {
		newStatus = userModels.DataChangeStatusApproved
	}
	if err := RecordParentRequestEvent(ctx, s.events, ParentRequestEventInput{
		StudentID:      req.StudentID,
		RequestType:    userModels.ParentRequestTypeMasterData,
		RequestID:      req.ID,
		EventType:      userModels.ParentRequestEventCorrected,
		ActorAccountID: reviewedBy,
		UpdatedAt:      req.UpdatedAt,
		Payload: CorrectionEventPayload(
			approve, trimmed, req.Status, newStatus, req.ReviewedBy, req.ReviewReason,
		),
	}); err != nil {
		return fmt.Errorf("review: record correction event: %w", err)
	}
	s.logger.Info("staff corrected master data decision",
		slog.Int64("request_id", req.ID),
		slog.Int64("student_id", req.StudentID),
		slog.Bool("approved", approve),
		slog.Int64("reviewed_by", reviewedBy),
	)
	s.deferDecisionPill(ctx, req, MasterDataReviewDecideInput{
		RequestID: req.ID, Approve: approve, Reason: trimmed, ReviewedBy: reviewedBy,
	}, approve)
	s.deferStudentUpdated(ctx, req.StudentID)
	return nil
}

func isCorrectableMasterDataStatus(status string) bool {
	return status == userModels.DataChangeStatusApproved ||
		status == userModels.DataChangeStatusRejected
}

// revertApprovedMasterData writes old_value back, but only while the live
// value is provably still the one this approval produced.
func (s *masterDataReviewService) revertApprovedMasterData(
	ctx context.Context,
	req *userModels.StudentDataChangeRequest,
	reviewedBy int64,
) error {
	if req.Status != userModels.DataChangeStatusApproved {
		return nil
	}
	scope, err := s.loadStudentScope(ctx, []int64{req.StudentID})
	if err != nil {
		return err
	}
	live, readable := masterDataLiveValue(req, scope)
	if !readable {
		return fmt.Errorf("%w: der aktuelle Wert dieses Feldes kann nicht gelesen werden",
			ErrParentRequestCorrectionUnsupported)
	}
	if !jsonRawEqual(live, req.NewValue) {
		return fmt.Errorf("%w: der Wert wurde nach der Entscheidung auf %s geändert",
			ErrParentRequestCorrectionUnsupported, jsonRawDisplay(live))
	}
	// Swapping the two values turns the apply into an undo: the same code path
	// that wrote the approval writes the baseline back, so validation, audit
	// and companion bookkeeping all happen exactly as they did going forward.
	undo := *req
	undo.NewValue = req.OldValue
	undo.OldValue = req.NewValue
	applyCtx, _ := userModels.ContextWithCompanionChangeRecorder(ctx)
	return s.applyApprovedChange(applyCtx, &undo, reviewedBy)
}

// masterDataLiveValue reads the field's current value. The second return says
// whether it could be read at all — an unreadable field is not "unchanged".
func masterDataLiveValue(
	req *userModels.StudentDataChangeRequest,
	scope *reviewStudentScope,
) (json.RawMessage, bool) {
	student := scope.students[req.StudentID]
	if student == nil {
		return nil, false
	}
	switch req.Target {
	case userModels.DataChangeTargetPerson:
		person := scope.persons[student.PersonID]
		if person == nil {
			return nil, false
		}
		raw, err := personFieldRaw(person, req.FieldKey)
		if err != nil {
			return nil, false
		}
		return raw, true
	case userModels.DataChangeTargetStudent:
		if req.FieldKey != "school_class" {
			return nil, false
		}
		return jsonString(student.SchoolClass), true
	case userModels.DataChangeTargetDeparture:
		raw, err := json.Marshal(student.AllowedDepartureModes.Normalize())
		if err != nil {
			return nil, false
		}
		return raw, true
	default:
		return nil, false
	}
}

// jsonRawDisplay renders a stored value for a German error sentence: a JSON
// string loses its quotes, anything else is shown as it is stored.
func jsonRawDisplay(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	return string(raw)
}
