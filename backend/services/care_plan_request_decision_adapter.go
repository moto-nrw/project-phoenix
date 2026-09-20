package services

import (
	"context"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
)

func (a requestDecisionAdapter) RecordMarkedDone(ctx context.Context, request *carerequests.Request, actorID int64, reason string) error {
	row := request
	return a.s.recordCareRequestEvent(ctx, row, usersModels.ParentRequestEventMarkedDone, actorID, map[string]any{"reason": reason})
}

func (a requestDecisionAdapter) NotifyMarkedDone(ctx context.Context, request *carerequests.Request, actorID int64, reason string) error {
	row := request
	return a.s.emitRequestPillAfterCommit(ctx, row, parentmessaging.ChildEvent{
		EventType: "request_status", ActorKind: usersModels.ParentMessageSenderStaff,
		ActorAccountID: actorID, Body: withPickupDetail(parentRequestDoneBody, row),
		RequestType: careRequestPillType(row.RequestKind), RequestStatus: usersModels.ParentMessageRequestStatusDone,
		DecisionReason: reason, Payload: pickupPillPayload(row),
	})
}

type requestDecisionAdapter struct{ requestEditAdapter }

func (a requestDecisionAdapter) LockStudent(ctx context.Context, id int64) (compose.ReviewStudent, error) {
	student, err := a.s.people.FindStudentRecordForMutation(ctx, id)
	if err != nil {
		return compose.ReviewStudent{}, requestStudentReadError(err)
	}
	result := compose.ReviewStudent{ID: student.ID, PersonID: student.PersonID, SchoolClass: student.SchoolClass, GroupID: student.GroupID, Alumnus: student.IsAlumnus()}
	if student.EnrolledUntil != "" {
		result.EnrolledUntil = careplan.Date(student.EnrolledUntil)
	}
	return result, nil
}
func (a requestDecisionAdapter) CanReview(ctx context.Context, student compose.ReviewStudent) (bool, error) {
	row := &usersModels.Student{PersonID: student.PersonID, SchoolClass: student.SchoolClass, GroupID: student.GroupID}
	row.ID = student.ID
	return a.s.canReviewStudent(ctx, row)
}
func (a requestDecisionAdapter) GuardianHasAccess(ctx context.Context, studentID, guardianID int64) (bool, error) {
	if a.s.emitter == nil {
		return true, nil
	}
	return a.s.emitter.GuardianHasChildAccess(ctx, studentID, guardianID)
}

func (a requestDecisionAdapter) Snapshot(ctx context.Context, request *carerequests.Request) *carerequests.DecisionSnapshot {
	return a.s.requests.diffs.Snapshot(ctx, request)
}
func (a requestDecisionAdapter) ApplyWeekly(ctx context.Context, request *carerequests.Request, actorID int64) (bool, error) {
	row := request
	return a.s.applyCareScheduleRequest(ctx, row, actorID)
}
func (a requestDecisionAdapter) ApplyPickup(ctx context.Context, request *carerequests.Request, token *string, required bool) (int64, error) {
	row := request
	return a.s.applyPickupChangeRequest(ctx, row, token, required)
}

func (a requestDecisionAdapter) RegisterDecision(ctx context.Context, request *carerequests.Request, input carerequests.DecideInput, reason string, changed bool) error {
	row := request
	return a.s.registerCareDecisionEffects(ctx, row, input, newCareDecisionState(row, input.Approve, reason), changed)
}
func (a requestDecisionAdapter) ReloadDecision(ctx context.Context, id int64) (*carerequests.ReviewItem, error) {
	return a.s.reloadCareDecisionItem(ctx, id)
}
func (a requestDecisionAdapter) RecordDecision(ctx context.Context, request *carerequests.Request, actorID int64, payload map[string]any) error {
	row := request
	return a.s.recordCareRequestEvent(ctx, row, usersModels.ParentRequestEventDecided, actorID, payload)
}
