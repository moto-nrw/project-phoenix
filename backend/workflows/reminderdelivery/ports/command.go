package ports

import (
	"context"
	"time"
)

// GuardianPreparation evaluates due occurrences and writes email rows and push
// claims in the caller's transaction. Its continuation may run only after commit.
type GuardianPreparation interface {
	PrepareAppointmentReminders(context.Context, time.Time, time.Time) (int, func(context.Context) error, error)
}

// CommandRuntime binds the workflow's independent tenant UnitOfWork.
type CommandRuntime struct {
	TenantID     func(context.Context) int64
	Detached     func(context.Context) context.Context
	WithinTenant func(context.Context, int64, func(context.Context) error) error
	AfterCommit  func(context.Context, func())
}
