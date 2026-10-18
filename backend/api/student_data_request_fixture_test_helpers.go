//go:build testhooks

package api

import (
	"context"

	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// The retained data-request adapter is used only by legacy repository fixtures.
// Production composes the native workflow DTOs and owner ports instead.
type studentDataRequestRepository struct{ capability careplan.Capability }

func NewStudentDataChangeRequestRepository(capability careplan.Capability) studentDataRequestRepository {
	return studentDataRequestRepository{capability: capability}
}

func (r studentDataRequestRepository) Create(ctx context.Context, row *careplan.StudentDataChangeRequest) error {
	if row == nil {
		return errors.New("StudentDataChangeRequest cannot be nil or zero value")
	}
	created, err := r.capability.CreateStudentDataRequest(ctx, *row)
	if err != nil {
		return err
	}
	*row = created
	return nil
}

func (r studentDataRequestRepository) FindByID(ctx context.Context, raw any) (*careplan.StudentDataChangeRequest, error) {
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
	return &value, nil
}

func (r studentDataRequestRepository) ListByStudent(ctx context.Context, studentID int64, statuses []string, limit int) ([]*careplan.StudentDataChangeRequest, error) {
	return r.list(ctx, careplan.StudentDataRequestFilter{StudentID: studentID, Statuses: statuses, Limit: limit})
}

func (r studentDataRequestRepository) ListPendingForTenant(ctx context.Context, filters careplan.RequestQueueFilter) ([]*careplan.StudentDataChangeRequest, error) {
	return r.list(ctx, careplan.StudentDataRequestFilter{Statuses: []string{contact.DataChangeStatusPending}, Queue: &filters})
}

func (r studentDataRequestRepository) list(ctx context.Context, filter careplan.StudentDataRequestFilter) ([]*careplan.StudentDataChangeRequest, error) {
	values, err := r.capability.ListStudentDataRequests(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]*careplan.StudentDataChangeRequest, 0, len(values))
	for _, value := range values {
		result = append(result, &value)
	}
	return result, nil
}

func (r studentDataRequestRepository) HasPendingForField(ctx context.Context, studentID int64, target, field string) (bool, error) {
	return r.capability.HasPendingStudentDataRequest(ctx, studentID, target, field)
}

func (r studentDataRequestRepository) FindPendingByIDForUpdate(ctx context.Context, id int64) (*careplan.StudentDataChangeRequest, error) {
	value, err := r.capability.FindPendingStudentDataRequest(ctx, id)
	if err != nil {
		return nil, (err)
	}
	return &value, nil
}

func (r studentDataRequestRepository) Decide(ctx context.Context, id int64, status string, reason *string, reviewedBy int64, applied bool) error {
	return (r.capability.DecideStudentDataRequest(ctx, careplan.StudentDataRequestDecision{ID: id, Status: status, Reason: reason, ReviewedBy: reviewedBy, Applied: applied}))
}
