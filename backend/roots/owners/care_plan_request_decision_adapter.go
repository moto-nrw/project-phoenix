package owners

import (
	"context"
	"errors"
	"fmt"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
)

type requestDecisionAdapter struct{ requestEditAdapter }

func (a requestDecisionAdapter) LockStudent(ctx context.Context, id int64) (compose.ReviewStudent, error) {
	student, err := a.s.People.FindStudentRecordForMutation(ctx, id)
	if errors.Is(err, peopleValues.ErrStudentNotFound) {
		return compose.ReviewStudent{}, careplan.ErrCareScheduleRequestNotFound
	}
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
	return a.s.Requests.diffs.Snapshot(ctx, request)
}

func (a requestDecisionAdapter) ApplyWeekly(ctx context.Context, request *carerequests.Request, actorID int64) (bool, error) {
	return a.s.applyCareScheduleRequest(ctx, request, actorID)
}

func (a requestDecisionAdapter) ApplyPickup(ctx context.Context, request *carerequests.Request, token *string, required bool) (carerequests.PickupApplied, error) {
	return a.s.applyPickupChangeRequest(ctx, request, token, required)
}

// ApplyPickupPermission grants the pickup permission an approved request asks
// for (#4780): the contact may collect the child on the asked weekdays. The
// request was built by the parent portal flow from the child's verified link,
// so its relationship belongs to this child; a link removed since then
// refuses the approval.
func (a requestDecisionAdapter) ApplyPickupPermission(ctx context.Context, request *carerequests.Request) error {
	if a.s.PickupPermissionApprovals == nil {
		return errors.New("schedule: pickup permission approvals not configured")
	}
	permission, err := carerequests.ParsePickupPermission(request.Payload)
	if err != nil {
		return err
	}
	return a.s.ApplyPickupPermission(ctx, carerequests.PickupPermissionApproval{
		TenantID: request.TenantID, StudentID: request.StudentID,
		GuardianProfileID: permission.GuardianProfileID, RelationshipID: permission.RelationshipID,
		Weekdays: permission.Weekdays,
	})
}

// RevertPickupPermission restores the precise permission state frozen when
// the request was filed. A correction must not infer that state from the
// current contact, which may have a different weekday restriction.
func (a requestDecisionAdapter) RevertPickupPermission(ctx context.Context, request *carerequests.Request) error {
	if a.s.PickupPermissionCommands == nil {
		return errors.New("schedule: pickup permission commands not configured")
	}
	permission, err := carerequests.ParsePickupPermission(request.Payload)
	if err != nil {
		return err
	}
	if permission.PreviousPermission == nil {
		return fmt.Errorf("%w: Abholberechtigungen ohne gespeicherten vorherigen Stand können nicht sicher korrigiert werden", careplan.ErrParentRequestCorrectionUnsupported)
	}
	found, err := a.s.PickupPermissionCommands.ChangeGuardianPickupPermission(ctx, careplan.GuardianPickupPermissionChange{
		TenantID: request.TenantID, RelationshipID: permission.RelationshipID,
		CanPickup: &permission.PreviousPermission.CanPickup, SetWeekdays: true, Weekdays: permission.PreviousPermission.Weekdays,
		Expected: &careplan.GuardianPickupPermissionState{CanPickup: true, Weekdays: permission.Weekdays},
	})
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%w: die Abholberechtigung wurde nach der Entscheidung geändert", careplan.ErrParentRequestCorrectionUnsupported)
	}
	return nil
}
