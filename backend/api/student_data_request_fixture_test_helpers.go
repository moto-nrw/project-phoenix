//go:build testhooks

package api

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// The retained data-request adapter is used only by legacy repository fixtures.
// Production composes the native workflow DTOs and owner ports instead.
type studentDataRequestRepository struct{ capability careplan.Capability }

func NewStudentDataChangeRequestRepository(capability careplan.Capability) peoplePorts.StudentDataChangeRequestRepository {
	return studentDataRequestRepository{capability: capability}
}

func (r studentDataRequestRepository) Create(ctx context.Context, row *userModels.StudentDataChangeRequest) error {
	if row == nil {
		return errors.New("StudentDataChangeRequest cannot be nil or zero value")
	}
	created, err := r.capability.CreateStudentDataRequest(ctx, studentDataRequestToPublic(row))
	if err != nil {
		return err
	}
	*row = *studentDataRequestFromPublic(created)
	return nil
}

func (r studentDataRequestRepository) FindByID(ctx context.Context, raw any) (*userModels.StudentDataChangeRequest, error) {
	id, err := legacyScheduleID(raw)
	if err != nil {
		return nil, err
	}
	value, err := r.capability.FindStudentDataRequest(ctx, id, false)
	if errors.Is(err, careplan.ErrStudentDataRequestNotFound) {
		return nil, retainedNotFound("find by id")
	}
	if err != nil {
		return nil, err
	}
	return studentDataRequestFromPublic(value), nil
}

func (r studentDataRequestRepository) ListByStudent(ctx context.Context, studentID int64, statuses []string, limit int) ([]*userModels.StudentDataChangeRequest, error) {
	return r.list(ctx, careplan.StudentDataRequestFilter{StudentID: studentID, Statuses: statuses, Limit: limit})
}

func (r studentDataRequestRepository) ListParentVisibleByStudent(ctx context.Context, studentID int64, limit int) ([]*userModels.StudentDataChangeRequest, error) {
	return r.list(ctx, careplan.StudentDataRequestFilter{StudentID: studentID, ParentVisible: true, Limit: limit})
}

func (r studentDataRequestRepository) ListPendingForTenant(ctx context.Context, filters peoplePorts.RequestQueueFilters) ([]*userModels.StudentDataChangeRequest, error) {
	return r.list(ctx, careplan.StudentDataRequestFilter{Statuses: []string{userModels.DataChangeStatusPending}, Queue: publicQueueFilter(filters)})
}

func (r studentDataRequestRepository) ListDecidedForTenant(ctx context.Context, filters peoplePorts.RequestQueueFilters) ([]*userModels.StudentDataChangeRequest, error) {
	return r.list(ctx, careplan.StudentDataRequestFilter{Statuses: []string{userModels.DataChangeStatusAutoApplied, userModels.DataChangeStatusApproved, userModels.DataChangeStatusRejected}, Queue: publicQueueFilter(filters)})
}

func (r studentDataRequestRepository) list(ctx context.Context, filter careplan.StudentDataRequestFilter) ([]*userModels.StudentDataChangeRequest, error) {
	values, err := r.capability.ListStudentDataRequests(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]*userModels.StudentDataChangeRequest, 0, len(values))
	for _, value := range values {
		result = append(result, studentDataRequestFromPublic(value))
	}
	return result, nil
}

func (r studentDataRequestRepository) HasPendingForField(ctx context.Context, studentID int64, target, field string) (bool, error) {
	return r.capability.HasPendingStudentDataRequest(ctx, studentID, target, field)
}

func (r studentDataRequestRepository) FindPendingByIDForUpdate(ctx context.Context, id int64) (*userModels.StudentDataChangeRequest, error) {
	value, err := r.capability.FindPendingStudentDataRequest(ctx, id)
	if err != nil {
		return nil, mapStudentDataRequestError(err)
	}
	return studentDataRequestFromPublic(value), nil
}

func (r studentDataRequestRepository) FindByIDForUpdate(ctx context.Context, id int64) (*userModels.StudentDataChangeRequest, error) {
	value, err := r.capability.FindStudentDataRequest(ctx, id, true)
	if err != nil {
		return nil, mapStudentDataRequestError(err)
	}
	return studentDataRequestFromPublic(value), nil
}

func (r studentDataRequestRepository) UpdatePending(ctx context.Context, id int64, value json.RawMessage) error {
	return mapStudentDataRequestError(r.capability.UpdatePendingStudentDataRequest(ctx, id, value))
}

func (r studentDataRequestRepository) Decide(ctx context.Context, id int64, status string, reason *string, reviewedBy int64, applied bool) error {
	return mapStudentDataRequestError(r.capability.DecideStudentDataRequest(ctx, careplan.StudentDataRequestDecision{ID: id, Status: status, Reason: reason, ReviewedBy: reviewedBy, Applied: applied}))
}

func (r studentDataRequestRepository) Redecide(ctx context.Context, id int64, status string, reason *string, reviewedBy int64, applied bool) error {
	return mapStudentDataRequestError(r.capability.RedecideStudentDataRequest(ctx, careplan.StudentDataRequestDecision{ID: id, Status: status, Reason: reason, ReviewedBy: reviewedBy, Applied: applied}))
}

func studentDataRequestFromPublic(value careplan.StudentDataChangeRequest) *userModels.StudentDataChangeRequest {
	row := &userModels.StudentDataChangeRequest{StudentID: value.StudentID, SubmittedBy: value.SubmittedBy, Target: value.Target, TargetRefID: value.TargetRefID, FieldKey: value.FieldKey, OldValue: value.OldValue, NewValue: value.NewValue, Status: value.Status, ReviewReason: value.ReviewReason, ReviewedBy: value.ReviewedBy, ReviewedAt: value.ReviewedAt, AppliedAt: value.AppliedAt}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func studentDataRequestToPublic(row *userModels.StudentDataChangeRequest) careplan.StudentDataChangeRequest {
	return careplan.StudentDataChangeRequest{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy, Target: row.Target, TargetRefID: row.TargetRefID, FieldKey: row.FieldKey, OldValue: row.OldValue, NewValue: row.NewValue, Status: row.Status, ReviewReason: row.ReviewReason, ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt}
}
