package care

import (
	"context"
	"encoding/json"
	"errors"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// createDataRequest records a Stammdaten change request through Care Plan and
// hands back the stored row.
func (s *Service) createDataRequest(ctx context.Context, row *usersModels.StudentDataChangeRequest) error {
	if s.DataRequests == nil {
		return errors.New("parent: student data request command is not configured")
	}
	created, err := s.DataRequests.CreateStudentDataRequest(ctx, dataRequestToCarePlan(row))
	if err != nil {
		return err
	}
	*row = *dataRequestFromCarePlan(created)
	return nil
}

// updatePendingDataRequest rewrites a pending request's proposed value
// through Care Plan and keeps the sentinels the handlers match.
func (s *Service) updatePendingDataRequest(ctx context.Context, id int64, value json.RawMessage) error {
	if s.DataRequests == nil {
		return errors.New("parent: student data request command is not configured")
	}
	err := s.DataRequests.UpdatePendingStudentDataRequest(ctx, id, value)
	switch {
	case errors.Is(err, careplan.ErrStudentDataRequestNotFound):
		return usersModels.ErrChangeRequestNotFound
	case errors.Is(err, careplan.ErrStudentDataRequestNotPending):
		return usersModels.ErrChangeRequestNotPending
	default:
		return err
	}
}

func dataRequestToCarePlan(row *usersModels.StudentDataChangeRequest) careplan.StudentDataChangeRequest {
	return careplan.StudentDataChangeRequest{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, Target: row.Target, TargetRefID: row.TargetRefID,
		FieldKey: row.FieldKey, OldValue: row.OldValue, NewValue: row.NewValue, Status: row.Status,
		ReviewReason: row.ReviewReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt,
	}
}

func dataRequestFromCarePlan(value careplan.StudentDataChangeRequest) *usersModels.StudentDataChangeRequest {
	row := &usersModels.StudentDataChangeRequest{
		StudentID: value.StudentID, SubmittedBy: value.SubmittedBy, Target: value.Target, TargetRefID: value.TargetRefID,
		FieldKey: value.FieldKey, OldValue: value.OldValue, NewValue: value.NewValue, Status: value.Status,
		ReviewReason: value.ReviewReason, ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt,
	}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}
