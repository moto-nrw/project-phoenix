package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

func (s *careScheduleRequestService) Correct(ctx context.Context, id int64, approve bool, version, reason string, actorID int64) error {
	a := requestCorrectionAdapter{requestDecisionAdapter{requestEditAdapter{requestSubmissionAdapter{s}}}}
	service, err := compose.NewRequestCorrections(compose.RequestCorrectionDependencies{Records: s.requestRecords, People: a, Plans: a, Effects: a})
	if err != nil {
		return err
	}
	err = service.Correct(ctx, id, approve, version, reason, actorID)
	if errors.Is(err, careplan.ErrParentRequestNotDecided) {
		return usersService.ErrParentRequestNotDecided
	}
	if errors.Is(err, careplan.ErrParentRequestCorrectionUnsupported) {
		return fmt.Errorf("%w%s", usersService.ErrParentRequestCorrectionUnsupported, strings.TrimPrefix(err.Error(), careplan.ErrParentRequestCorrectionUnsupported.Error()))
	}
	return legacyEditError(err)
}

type requestCorrectionAdapter struct{ requestDecisionAdapter }

func (a requestCorrectionAdapter) CorrectionHistory(ctx context.Context, request *carerequests.Request) ([]compose.RequestCorrectionEvent, error) {
	if a.s.events == nil {
		return nil, nil
	}
	row := request
	events, err := a.s.events.ListForRequest(ctx, careRequestLedgerType(row), row.ID)
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
func (a requestCorrectionAdapter) RecordCorrection(ctx context.Context, request *carerequests.Request, actorID int64, payload map[string]any) error {
	row := request
	return a.s.recordCareRequestEvent(ctx, row, usersModels.ParentRequestEventCorrected, actorID, payload)
}
func (a requestCorrectionAdapter) NotifyCorrection(ctx context.Context, request *carerequests.Request, actorID int64, approve bool, reason string) error {
	row := request
	status := usersModels.ParentMessageRequestStatusRejected
	if approve {
		status = usersModels.ParentMessageRequestStatusDone
	}
	return a.s.emitRequestPillAfterCommit(ctx, row, parentmessaging.ChildEvent{
		EventType: usersModels.ParentMessageEventRequestStatus, ActorKind: usersModels.ParentMessageSenderStaff,
		ActorAccountID: actorID, Body: "Entscheidung geändert. " + correctedCarePillBody(row, approve),
		RequestType: careRequestPillType(row.RequestKind), RequestStatus: status, DecisionReason: reason, Payload: pickupPillPayload(row),
	})
}
