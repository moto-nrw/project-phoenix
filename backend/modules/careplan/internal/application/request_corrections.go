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
	var applied carerequests.PickupApplied
	if approve {
		applied, err = s.Plans.ApplyPickup(ctx, request, nil, false)
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
	appliedLedgerPayload(payload, applied)
	if err := s.Effects.RecordCorrection(ctx, request, actorID, payload); err != nil {
		return err
	}
	return s.Effects.NotifyCorrection(ctx, request, actorID, approve, reason)
}

func (s *RequestCorrections) loadCorrection(ctx context.Context, id int64, version string) (*carerequests.Request, error) {
	request, student, err := lockCareRequestStudent(ctx, s.Records, s.People, id, func(row carerequests.Request) error {
		return validateCareCorrection(row, version)
	})
	if err != nil {
		return nil, err
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
	// The newest decision names the rows it wrote: a time, a day Abholart
	// (#3945), or both.
	var applied carerequests.PickupApplied
	var found bool
	for _, event := range events {
		if event.Type != "decided" {
			continue
		}
		pickupID, pickupOK := correctionExceptionID(event.Payload["pickup_exception_id"])
		departureID, departureOK := correctionExceptionID(event.Payload["departure_exception_id"])
		noteID, noteOK := correctionExceptionID(event.Payload["pickup_note_id"])
		if pickupOK || departureOK || noteOK {
			applied = carerequests.PickupApplied{PickupExceptionID: pickupID, DepartureExceptionID: departureID, PickupNoteID: noteID}
			found = true
		}
	}
	if !found {
		return fmt.Errorf("%w: zu dieser Entscheidung ist kein Eintrag gespeichert, der zurückgenommen werden kann. Bitte ändern Sie die Abholung direkt", careplan.ErrParentRequestCorrectionUnsupported)
	}
	if applied.PickupExceptionID > 0 {
		if err := s.revertPickupException(ctx, request, applied.PickupExceptionID); err != nil {
			return err
		}
	}
	if applied.DepartureExceptionID > 0 {
		if err := s.revertDepartureException(ctx, request, applied.DepartureExceptionID); err != nil {
			return err
		}
	}
	if applied.PickupNoteID > 0 {
		// The day's collector note the approval wrote (#4780).
		return s.Records.DeletePickupNote(ctx, applied.PickupNoteID)
	}
	return nil
}

func (s *RequestCorrections) revertPickupException(ctx context.Context, request *carerequests.Request, id int64) error {
	row, err := s.Records.FindException(ctx, id)
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

func (s *RequestCorrections) revertDepartureException(ctx context.Context, request *carerequests.Request, id int64) error {
	row, err := s.Records.FindDepartureException(ctx, id)
	if err != nil {
		return err
	}
	if row == nil {
		return nil
	}
	if request.ReviewedAt != nil && row.UpdatedAt.After(*request.ReviewedAt) {
		return fmt.Errorf("%w: die Abholart am %s wurde nach der Entscheidung geändert", careplan.ErrParentRequestCorrectionUnsupported, row.ExceptionDate.Format("02.01.2006"))
	}
	return s.Records.DeleteDepartureException(ctx, row.ID)
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

func validateCareCorrection(request carerequests.Request, version string) error {
	if request.Status != "approved" && request.Status != "rejected" {
		return careplan.ErrParentRequestNotDecided
	}
	if version != "" && careplan.ParentRequestVersion(request.UpdatedAt) != version {
		return careplan.ErrParentRequestStale
	}
	if request.RequestKind != "pickup_change" {
		return fmt.Errorf("%w: Betreuungszeiten speichern keinen Stand von vor der Entscheidung. Bitte ändern Sie den Wochenplan des Kindes direkt", careplan.ErrParentRequestCorrectionUnsupported)
	}
	return nil
}
