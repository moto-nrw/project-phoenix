package application

import (
	"context"
	"errors"
	"time"

	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

type command struct {
	runtime ports.CommandRuntime
	source  ports.GuardianPreparation
}

func NewCommand(runtime ports.CommandRuntime, source ports.GuardianPreparation) reminder.Command {
	if source == nil || runtime.TenantID == nil || runtime.Detached == nil || runtime.WithinTenant == nil || runtime.AfterCommit == nil {
		panic("reminder command: all dependencies are required")
	}
	return &command{runtime: runtime, source: source}
}

func (c *command) EnqueueDueAppointmentReminders(ctx context.Context, from, to time.Time) (int, error) {
	tenantID := c.runtime.TenantID(ctx)
	if tenantID <= 0 {
		return 0, errors.New("calendar: tenant id is required for appointment reminders")
	}
	var queued int
	var dispatchErr error
	// An outer scheduler transaction must not own these claims: its later
	// rollback could otherwise erase the email after its push was delivered.
	err := c.runtime.WithinTenant(c.runtime.Detached(ctx), tenantID, func(txCtx context.Context) error {
		var dispatch func(context.Context) error
		var prepareErr error
		queued, dispatch, prepareErr = c.source.PrepareAppointmentReminders(txCtx, from, to)
		if prepareErr == nil && dispatch != nil {
			postCommitCtx := c.runtime.Detached(txCtx)
			c.runtime.AfterCommit(txCtx, func() { dispatchErr = dispatch(postCommitCtx) })
		}
		return prepareErr
	})
	if err != nil {
		return queued, err
	}
	return queued, dispatchErr
}
