package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
)

// Correct rewrites a Stammdaten decision staff already took (#2267, stories
// 21-23).
//
// rejected → approved re-runs the ordinary apply, so every guard a fresh
// approval passes runs again.
//
// approved → rejected has to put the child's record back. It restores the old
// value ONLY while the live value still equals what the approval wrote: if the
// office (or an import, or another request) changed the field since, the value
// on record is newer than this request and overwriting it would silently
// discard an edit nobody can recover. The correction then refuses with
// parentrequests.ErrCorrectionUnsupported and names the value that is there
// now, so the reviewer can decide what should actually stand.
func (s *MasterDataDecisions) Correct(ctx context.Context, requestID int64, approve bool, expectedVersion, reason string, reviewedBy int64) error {
	req, err := s.loadCorrectableRequest(ctx, requestID, expectedVersion)
	if err != nil {
		return err
	}
	// Re-authorize against the CURRENT scope: a group leader who has since
	// lost the group must not be able to correct its decisions.
	if err := s.authorizeDecision(ctx, req.StudentID); err != nil {
		return err
	}
	trimmed := strings.TrimSpace(reason)
	if err := s.rewriteDecision(ctx, req, approve, trimmed, reviewedBy); err != nil {
		return err
	}
	// The ledger keeps BOTH decisions: the correction is a new entry naming
	// what it replaced, never an edit of the old one. Written inside the
	// correction's own transaction, so a rollback leaves no trace of it.
	newStatus := masterdatarequests.StatusRejected
	if approve {
		newStatus = masterdatarequests.StatusApproved
	}
	if err := s.record(ctx, ports.RequestLedgerEntry{
		StudentID: req.StudentID, RequestType: masterdatarequests.ParentRequestType, RequestID: req.ID,
		EventType: careplan.ParentRequestEventCorrected, ActorAccountID: reviewedBy, UpdatedAt: req.UpdatedAt,
		Payload: correctionEventPayload(approve, trimmed, req.Status, newStatus, req.ReviewedBy, req.ReviewReason),
	}); err != nil {
		return fmt.Errorf("review: record correction event: %w", err)
	}
	s.logger.Info("staff corrected master data decision",
		slog.Int64("request_id", req.ID),
		slog.Int64("student_id", req.StudentID),
		slog.Bool("approved", approve),
		slog.Int64("reviewed_by", reviewedBy),
	)
	if err := s.deferDecisionPill(ctx, req, reviewedBy, trimmed, approve); err != nil {
		return err
	}
	s.deferStudentUpdated(ctx, req.StudentID)
	return nil
}

func (s *MasterDataDecisions) loadCorrectableRequest(ctx context.Context, requestID int64, expectedVersion string) (*careplan.StudentDataChangeRequest, error) {
	if requestID <= 0 {
		return nil, masterdatarequests.ErrReviewNotFound
	}
	req, err := s.requests.FindStudentDataRequest(ctx, requestID, true)
	if errors.Is(err, careplan.ErrStudentDataRequestNotFound) {
		return nil, masterdatarequests.ErrReviewNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("review: load request for correction: %w", err)
	}
	if !req.CanCorrect() {
		return nil, parentrequests.ErrNotDecided
	}
	if expectedVersion != "" && careplan.ParentRequestVersion(req.UpdatedAt) != expectedVersion {
		return nil, parentrequests.ErrStale
	}
	return &req, nil
}

// rewriteDecision brings the child's record in line with the new verdict and
// re-decides the row.
func (s *MasterDataDecisions) rewriteDecision(ctx context.Context, req *careplan.StudentDataChangeRequest, approve bool, reason string, reviewedBy int64) error {
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	decision := careplan.StudentDataRequestDecision{
		ID: req.ID, Status: masterdatarequests.StatusRejected, Reason: reasonPtr, ReviewedBy: reviewedBy,
	}
	if approve {
		if _, err := s.applyChange(ctx, req, reviewedBy); err != nil {
			return err
		}
		decision.Status, decision.Applied = masterdatarequests.StatusApproved, true
	} else if err := s.revertApproval(ctx, req, reviewedBy); err != nil {
		return err
	}
	return s.requests.RedecideStudentDataRequest(ctx, decision)
}

// revertApproval writes the old value back, but only while the live value is
// provably still the one this approval produced.
func (s *MasterDataDecisions) revertApproval(ctx context.Context, req *careplan.StudentDataChangeRequest, reviewedBy int64) error {
	if req.Status != masterdatarequests.StatusApproved {
		return nil
	}
	live, readable, err := s.liveValue(ctx, req)
	if err != nil {
		return err
	}
	if !readable {
		return fmt.Errorf("%w: der aktuelle Wert dieses Feldes kann nicht gelesen werden",
			parentrequests.ErrCorrectionUnsupported)
	}
	if !domain.SameJSON(live, req.NewValue) {
		return fmt.Errorf("%w: der Wert wurde nach der Entscheidung auf %s geändert",
			parentrequests.ErrCorrectionUnsupported, domain.DisplayJSON(live))
	}
	// Swapping the two values turns the apply into an undo: the same path that
	// wrote the approval writes the baseline back, so validation, the change
	// history and the companion bookkeeping happen exactly as going forward.
	undo := *req
	undo.NewValue, undo.OldValue = req.OldValue, req.NewValue
	_, err = s.applyChange(ctx, &undo, reviewedBy)
	return err
}

// liveValue reads the field's current value. The second return says whether
// it could be read at all — an unreadable field is not "unchanged".
func (s *MasterDataDecisions) liveValue(ctx context.Context, req *careplan.StudentDataChangeRequest) (json.RawMessage, bool, error) {
	student, err := s.records.FindStudent(ctx, req.StudentID)
	if err != nil {
		return nil, false, fmt.Errorf("review: load students: %w", err)
	}
	switch req.Target {
	case masterdatarequests.TargetPerson:
		person, err := s.records.FindPerson(ctx, student.PersonID)
		if err != nil {
			return nil, false, fmt.Errorf("review: load persons: %w", err)
		}
		raw, fieldErr := masterDataPersonValue(&person, req.FieldKey)
		return raw, fieldErr == nil, nil
	case masterdatarequests.TargetStudent:
		if req.FieldKey != masterDataFieldSchoolClass {
			return nil, false, nil
		}
		return domain.JSONString(student.SchoolClass), true, nil
	case masterdatarequests.TargetDeparture:
		raw, err := json.Marshal(student.DepartureModes.Normalize())
		return raw, err == nil, nil
	default:
		return nil, false, nil
	}
}
