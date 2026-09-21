package application

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type RequestCorrections struct {
	Records ports.RequestCorrectionRecords
	People  ports.RequestCorrectionPeople
	Plans   ports.RequestCorrectionPlans
	Effects ports.RequestCorrectionEffects
}

func (s *RequestCorrections) Correct(ctx context.Context, id int64, approve bool, version, reason string, actorID int64) error {
	if id <= 0 {
		return careplan.ErrCareScheduleRequestNotFound
	}
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > maxDecisionReasonRunes {
		return carerequests.ErrRejectReasonTooLong
	}
	request, err := s.loadCorrection(ctx, id, version)
	if err != nil {
		return err
	}
	var exceptionID int64
	if approve {
		exceptionID, err = s.Plans.ApplyPickup(ctx, request, nil, false)
	} else {
		err = s.revertPickup(ctx, request)
	}
	if err != nil {
		return err
	}
	status := "rejected"
	if approve {
		status = "approved"
	}
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	if err := s.Records.Redecide(ctx, id, status, reasonPtr, actorID, approve); err != nil {
		return err
	}
	payload := map[string]any{"approve": approve, "reason": reason, "from": request.Status, "to": status}
	if request.ReviewedBy != nil {
		payload["prior_reviewer"] = *request.ReviewedBy
	}
	if request.DecisionReason != nil {
		payload["prior_reason"] = *request.DecisionReason
	}
	if exceptionID > 0 {
		payload["pickup_exception_id"] = exceptionID
	}
	if err := s.Effects.RecordCorrection(ctx, request, actorID, payload); err != nil {
		return err
	}
	return s.Effects.NotifyCorrection(ctx, request, actorID, approve, reason)
}

func (s *RequestCorrections) loadCorrection(ctx context.Context, id int64, version string) (*carerequests.Request, error) {
	request, err := s.Records.Find(ctx, id, true)
	if err != nil {
		return nil, err
	}
	if request.Status != "approved" && request.Status != "rejected" {
		return nil, careplan.ErrParentRequestNotDecided
	}
	if version != "" && careplan.ParentRequestVersion(request.UpdatedAt) != version {
		return nil, careplan.ErrParentRequestStale
	}
	if request.RequestKind != "pickup_change" {
		return nil, fmt.Errorf("%w: Betreuungszeiten speichern keinen Stand von vor der Entscheidung. Bitte ändern Sie den Wochenplan des Kindes direkt", careplan.ErrParentRequestCorrectionUnsupported)
	}
	student, err := s.People.LockStudent(ctx, request.StudentID)
	if err != nil {
		return nil, fmt.Errorf("schedule: load student for care request correction: %w", err)
	}
	allowed, err := s.People.CanReview(ctx, student)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, carerequests.ErrCareRequestForbidden
	}
	return &request, nil
}

func (s *RequestCorrections) revertPickup(ctx context.Context, request *carerequests.Request) error {
	if request.Status != "approved" {
		return nil
	}
	events, err := s.Effects.CorrectionHistory(ctx, request)
	if err != nil {
		return fmt.Errorf("schedule: read care request ledger: %w", err)
	}
	var exceptionID int64
	var found bool
	for _, event := range events {
		if event.Type != "decided" {
			continue
		}
		if id, ok := correctionExceptionID(event.Payload["pickup_exception_id"]); ok {
			exceptionID, found = id, true
		}
	}
	if !found {
		return fmt.Errorf("%w: zu dieser Entscheidung ist kein Eintrag gespeichert, der zurückgenommen werden kann. Bitte ändern Sie die Abholzeit direkt", careplan.ErrParentRequestCorrectionUnsupported)
	}
	row, err := s.Records.FindException(ctx, exceptionID)
	if err != nil {
		return err
	}
	if row == nil {
		return nil
	}
	if request.ReviewedAt != nil && row.UpdatedAt.After(*request.ReviewedAt) {
		return fmt.Errorf("%w: die Abholzeit am %s wurde nach der Entscheidung geändert", careplan.ErrParentRequestCorrectionUnsupported, row.ExceptionDate.Format("02.01.2006"))
	}
	return s.Records.DeleteException(ctx, row.ID)
}

func correctionExceptionID(value any) (int64, bool) {
	switch id := value.(type) {
	case float64:
		return int64(id), id > 0
	case int64:
		return id, id > 0
	default:
		return 0, false
	}
}
