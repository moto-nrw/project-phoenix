package services

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

// DepartureModes is the Abholart of the day before a request changes it: the
// OGS's day value (#3945), else the weekly plan's set for the plan weekday.
// A weekday without a configured set is "alone", the weekly diff's reading.
func (a requestSubmissionAdapter) DepartureModes(ctx context.Context, id int64, date timezone.Date) ([]string, error) {
	days, err := a.s.requestRecords.ListDepartureExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Date: careplan.Date(date)})
	if err != nil {
		return nil, err
	}
	if len(days) > 0 {
		return []string{days[0].DepartureMode}, nil
	}
	weekday, err := calendar.PlanWeekday(ctx, calendar.Date(date))
	if err != nil || weekday < 1 || weekday > 5 {
		return nil, err
	}
	student, err := a.s.people.FindStudentRecord(ctx, id)
	if err != nil {
		return nil, requestStudentReadError(err)
	}
	modes := make([]string, 0, 2)
	for _, mode := range student.AllowedDepartureModes[weekdayDepartureKeys[weekday]] {
		modes = append(modes, string(mode))
	}
	if len(modes) == 0 {
		modes = append(modes, careplan.DepartureModeAlone)
	}
	return modes, nil
}

var weekdayDepartureKeys = [...]string{"", "mon", "tue", "wed", "thu", "fri"}

func (a requestSubmissionAdapter) RecordSubmission(ctx context.Context, request *carerequests.Request) error {
	row := request
	return a.s.recordCareRequestEvent(ctx, row, usersModels.ParentRequestEventSubmitted, row.SubmittedBy, nil)
}

func (a requestSubmissionAdapter) NotifySubmission(ctx context.Context, request *carerequests.Request) error {
	row := request
	event := parentmessaging.ChildEvent{EventType: "request_created", ActorKind: usersModels.ParentMessageSenderGuardian,
		ActorAccountID: row.SubmittedBy, Body: careRequestCreatedBody, RequestType: usersModels.ParentMessageRequestCareSchedule, RequestStatus: usersModels.ParentMessageRequestStatusOpen}
	if row.RequestKind == "pickup_change" {
		event.Body = withPickupDetail(pickupPillBody(pickupRequestCreatedVerb, row), row)
		event.RequestType = usersModels.ParentMessageRequestPickupChange
		event.Payload = pickupPillPayload(row)
	}
	return a.s.emitRequestPillAfterCommit(ctx, row, event)
}

func (a requestSubmissionAdapter) WakeGuardians(ctx context.Context, request *carerequests.Request) {
	row := request
	a.s.wakeGuardiansAfterCommit(ctx, row)
}
