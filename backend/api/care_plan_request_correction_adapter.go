package api

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
)

func (s *careScheduleRequestService) Correct(ctx context.Context, id int64, approve bool, version, reason string, actorID int64) error {
	err := s.requests.corrections.Correct(ctx, id, approve, version, reason, actorID)
	if errors.Is(err, careplan.ErrParentRequestNotDecided) {
		return parentrequests.ErrNotDecided
	}
	if errors.Is(err, careplan.ErrParentRequestCorrectionUnsupported) {
		return fmt.Errorf("%w%s", parentrequests.ErrCorrectionUnsupported, strings.TrimPrefix(err.Error(), careplan.ErrParentRequestCorrectionUnsupported.Error()))
	}
	return legacyEditError(err)
}

// requestCorrectionEffects reads a request's decision history from the
// People Directory ledger; Care Plan's effects record and announce the
// correction.
type requestCorrectionEffects struct {
	compose.CareRequestEffects
	ledger *ParentRequestLedger
}

func (e requestCorrectionEffects) CorrectionHistory(ctx context.Context, request *carerequests.Request) ([]compose.RequestCorrectionEvent, error) {
	if e.ledger == nil {
		return nil, nil
	}
	events, err := e.ledger.ListForRequest(ctx, carerequests.LedgerRequestType(request.RequestKind), request.ID)
	if err != nil {
		return nil, err
	}
	result := make([]compose.RequestCorrectionEvent, 0, len(events))
	for _, event := range events {
		if event != nil {
			result = append(result, compose.RequestCorrectionEvent{Type: event.EventType, Payload: event.Payload})
		}
	}
	return result, nil
}
