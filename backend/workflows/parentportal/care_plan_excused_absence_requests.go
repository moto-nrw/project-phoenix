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
	return a.callbacks.EditRequest(ctx, input)
}
func (a excusedAbsenceRequests) ListForStudent(ctx context.Context, id int64, since time.Time) ([]*care.ExcusedAbsenceRequest, error) {
	return a.callbacks.ListForStudent(ctx, id, since)
}
func (a excusedAbsenceRequests) Submit(ctx context.Context, input care.ExcusedRequestCreateInput) (*care.ExcusedAbsenceRequest, error) {
	return a.callbacks.Submit(ctx, input)
}
