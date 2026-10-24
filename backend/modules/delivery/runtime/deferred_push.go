package delivery

import (
	"context"
	"errors"
)

// PushDispatcher claims exactly one committed intent and returns its transport
// result. A transport failure is terminal, so a manual retry cannot send twice.
type PushDispatcher struct {
	dispatch func(context.Context, int64, int64) error
}

func NewPushDispatcher(dispatch func(context.Context, int64, int64) error) *PushDispatcher {
	return &PushDispatcher{dispatch: dispatch}
}

func (d *PushDispatcher) Deliver(ctx context.Context, tenantID, id int64) error {
	if tenantID <= 0 || id <= 0 {
		return errors.New("delivery: tenant and push intent are required")
	}
	return d.dispatch(ctx, tenantID, id)
}
