package services

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
)

type requestSubmissionAdapter struct{ s *careScheduleRequestService }

func (a requestSubmissionAdapter) PickupTime(ctx context.Context, id int64, date timezone.Date) (*time.Time, error) {
	if a.s.pickup == nil {
		return nil, nil
	}
	effective, err := a.s.pickup.GetEffectivePickupTimeForDate(ctx, id, date)
	if err != nil || effective == nil {
		return nil, err
	}
	return effective.PickupTime, nil
}

func (a requestSubmissionAdapter) RecordSubmission(ctx context.Context, request *carerequests.Request) error {
	row := request
	return a.s.recordCareRequestEvent(ctx, row, usersModels.ParentRequestEventSubmitted, row.SubmittedBy, nil)
}

func (a requestSubmissionAdapter) NotifySubmission(ctx context.Context, request *carerequests.Request) error {
	row := request
	event := parentmessaging.ChildEvent{EventType: "request_created", ActorKind: usersModels.ParentMessageSenderGuardian,
		ActorAccountID: row.SubmittedBy, Body: careRequestCreatedBody, RequestType: usersModels.ParentMessageRequestCareSchedule, RequestStatus: usersModels.ParentMessageRequestStatusOpen}
	if row.RequestKind == "pickup_change" {
		event.Body = withPickupDetail(pickupRequestCreatedBody, row)
		event.RequestType = usersModels.ParentMessageRequestPickupChange
		event.Payload = pickupPillPayload(row)
	}
	return a.s.emitRequestPillAfterCommit(ctx, row, event)
}

func (a requestSubmissionAdapter) WakeGuardians(ctx context.Context, request *carerequests.Request) {
	row := request
	a.s.wakeGuardiansAfterCommit(ctx, row)
}
