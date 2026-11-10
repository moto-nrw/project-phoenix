//go:build testhooks

// Package tenanttest exposes tenant transaction internals to tests in other
// packages. It is a leaf so the hooks do not change how the tenant package,
// and the large part of the backend importing it, compiles under testhooks
// (#5985). Production code must obtain transactions from WithinTenant or
// WithinAdmin.
package tenanttest

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant/internal/testseam"
)

// WithTransaction installs an adapter transaction for tests that need to
// exercise transaction-aware repository behavior without opening another
// UnitOfWork.
func WithTransaction(ctx context.Context, tx any) context.Context {
	requireLinked(testseam.WithTransaction != nil)
	return testseam.WithTransaction(ctx, tx)
}

// WithAfterCommitHooks lets other packages unit-test handlers that call
// tenant.RegisterAfterCommit without a real database. The returned drain runs
// the queued hooks the same way the outermost transaction does on COMMIT.
func WithAfterCommitHooks(ctx context.Context) (context.Context, func()) {
	requireLinked(testseam.WithAfterCommitHooks != nil)
	return testseam.WithAfterCommitHooks(ctx)
}

// requireLinked fails loudly when the tenant package, whose init fills
// testseam, is not linked into the test binary. Every caller hands the context
// to tenant code, so an empty seam is a wiring error, never a valid outcome.
func requireLinked(linked bool) {
	if !linked {
		panic("tenanttest: tenant package is not linked; import it from the test")
	}
}
