package tenant_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
)

func TestHasAfterCommitHooks(t *testing.T) {
	assert.False(t, tenant.HasAfterCommitHooks(context.Background()))
	ctx, drain := tenant.WithAfterCommitHooksForTest(context.Background())
	assert.True(t, tenant.HasAfterCommitHooks(ctx))
	drain()
}

// TestRegisterAfterCommit_RunsInlineWithoutTx locks down the contract that
// callers outside any WithTenantTx (e.g. tests, CLI commands, scheduler init)
// see fn fire synchronously instead of being silently dropped. The
// alternative — holding the callback in a context that no tx will ever
// commit — would silently never execute, which is a worse failure mode than
// running once now.
func TestRegisterAfterCommit_RunsInlineWithoutTx(t *testing.T) {
	called := false
	tenant.RegisterAfterCommit(context.Background(), func() {
		called = true
	})
	assert.True(t, called, "fn should run inline when no tenant tx is in context")
}

// TestRegisterAfterCommit_NilFnIsNoop covers the null-callback case so callers
// that conditionally produce a hook can pass nil without guarding.
func TestRegisterAfterCommit_NilFnIsNoop(t *testing.T) {
	// No assertions besides "does not panic" — nil fn must be silently ignored.
	tenant.RegisterAfterCommit(context.Background(), nil)
}

// NOTE: integration coverage for the queue-and-drain behaviour (hook fires
// after the OUTERMOST WithTenantTx commits, hook drops when fn returns an
// error or middleware rolls back on 5xx) lives in
// api/config/settings_integration_test.go::TestSettingsSetValue_PostCommit*.
// Those tests already exercise the full TenantTxMiddleware → handler →
// WithTenantTx nesting path against a real DB, which is the only place the
// drain semantics actually matter.
