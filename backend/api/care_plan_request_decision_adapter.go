package api

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
)

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
	row := &peopleValues.StudentState{PersonID: student.PersonID, SchoolClass: student.SchoolClass, GroupID: student.GroupID}
	row.ID = student.ID
	return a.s.canReviewStudent(ctx, row)
}

func (a requestDecisionAdapter) Snapshot(ctx context.Context, request *carerequests.Request) *carerequests.DecisionSnapshot {
	return a.s.requests.diffs.Snapshot(ctx, request)
}
func (a requestDecisionAdapter) ApplyWeekly(ctx context.Context, request *carerequests.Request, actorID int64) (bool, error) {
	return a.s.applyCareScheduleRequest(ctx, request, actorID)
}
func (a requestDecisionAdapter) ApplyPickup(ctx context.Context, request *carerequests.Request, token *string, required bool) (carerequests.PickupApplied, error) {
	return a.s.applyPickupChangeRequest(ctx, request, token, required)
}
