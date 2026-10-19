package parentportal

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type ExcusedAbsenceRequests interface{ care.ExcusedAbsenceRequests }
type ExcusedAbsenceRequestsCallbacks struct {
	EditRequest    func(ctx context.Context, input ExcusedRequestEditInputSnapshot) (*ExcusedAbsenceRequestSnapshot, error)
	ListForStudent func(ctx context.Context, id int64, since time.Time) ([]*ExcusedAbsenceRequestSnapshot, error)
	Submit         func(ctx context.Context, input ExcusedRequestCreateInputSnapshot) (*ExcusedAbsenceRequestSnapshot, error)
}

func NewExcusedAbsenceRequests(callbacks ExcusedAbsenceRequestsCallbacks) ExcusedAbsenceRequests {
	return excusedAbsenceRequests{callbacks}
}

type excusedAbsenceRequests struct {
	callbacks ExcusedAbsenceRequestsCallbacks
}

func (a excusedAbsenceRequests) EditRequest(ctx context.Context, input care.ExcusedRequestEditInput) (*care.ExcusedAbsenceRequest, error) {
	value, err := a.callbacks.EditRequest(ctx, carePlanFromAppExcusedRequestEditInput(input))
	return carePlanValuePointer(value, carePlanToAppExcusedAbsenceRequest), err
}
func (a excusedAbsenceRequests) ListForStudent(ctx context.Context, id int64, since time.Time) ([]*care.ExcusedAbsenceRequest, error) {
	value, err := a.callbacks.ListForStudent(ctx, id, since)
	return carePlanValueSlice(value, func(v *ExcusedAbsenceRequestSnapshot) *care.ExcusedAbsenceRequest {
		return carePlanValuePointer(v, carePlanToAppExcusedAbsenceRequest)
	}), err
}
func (a excusedAbsenceRequests) Submit(ctx context.Context, input care.ExcusedRequestCreateInput) (*care.ExcusedAbsenceRequest, error) {
	value, err := a.callbacks.Submit(ctx, carePlanFromAppExcusedRequestCreateInput(input))
	return carePlanValuePointer(value, carePlanToAppExcusedAbsenceRequest), err
}
