package messaging

import "context"

// Runtime supplies the ambient school and commit boundary; the root binds it.
type Runtime interface {
	TenantID(context.Context) int64
	RegisterAfterCommit(context.Context, func())
}

// EventRuntime carries the same unit of work into detached event writes.
type EventRuntime interface {
	RegisterAfterCommit(context.Context, func())
	Configured() bool
	BackgroundContext() context.Context
	InTransaction(context.Context) bool
	WithinTenant(context.Context, int64, func(context.Context) error) error
}
