// Package testseam hands three tenant internals to tenant/tenanttest. Only the
// tenant subtree can import it, and nothing in production calls these values.
// It keeps the tenant package identical with and without the testhooks build
// tag, so tagged test builds reuse its compiled form and that of every
// package depending on it (#5985).
package testseam

import "context"

var (
	// WithTransaction installs an adapter transaction in ctx.
	WithTransaction func(ctx context.Context, tx any) context.Context
	// WithTransactionHooks attaches a hook holder and returns its commit and
	// rollback drains.
	WithTransactionHooks func(ctx context.Context) (context.Context, func(), func())
	// RegisterAfterRollback queues fn for the rollback of the surrounding
	// transaction.
	RegisterAfterRollback func(ctx context.Context, fn func())
)
