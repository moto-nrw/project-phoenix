// Package testseam hands tenant internals to tenant/tenanttest. Only the
// tenant subtree can import it, and nothing in production calls these values.
// It keeps the tenant package identical with and without the testhooks build
// tag, so tagged test builds reuse its compiled form and that of every
// package depending on it (#5985).
package testseam

import "context"

var (
	// WithTransaction installs an adapter transaction in ctx.
	WithTransaction func(ctx context.Context, tx any) context.Context
	// WithAfterCommitHooks attaches a hook holder and returns its commit drain.
	WithAfterCommitHooks func(ctx context.Context) (context.Context, func())
)
