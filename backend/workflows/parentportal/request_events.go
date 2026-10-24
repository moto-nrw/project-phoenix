package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type RequestEventSnapshot care.RequestEvent
type RequestEventCallbacks struct {
	Create func(context.Context, *RequestEventSnapshot) error
	List   func(context.Context, string, int64) ([]*RequestEventSnapshot, error)
}
type RequestEvents interface {
	care.RequestEventWriter
	ListForRequest(context.Context, string, int64) ([]*care.RequestEvent, error)
}

func NewRequestEvents(callbacks RequestEventCallbacks) RequestEvents { return requestEvents{callbacks} }

type requestEvents struct{ callbacks RequestEventCallbacks }

func (r requestEvents) Create(ctx context.Context, value *care.RequestEvent) error {
	return r.callbacks.Create(ctx, (*RequestEventSnapshot)(value))
}
func (r requestEvents) ListForRequest(ctx context.Context, kind string, id int64) ([]*care.RequestEvent, error) {
	rows, err := r.callbacks.List(ctx, kind, id)
	if rows == nil {
		return nil, err
	}
	result := make([]*care.RequestEvent, len(rows))
	for i, row := range rows {
		result[i] = (*care.RequestEvent)(row)
	}
	return result, err
}
