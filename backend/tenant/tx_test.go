package tenant_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

func TestWithTenantTx_RejectsZeroTenantID(t *testing.T) {
	t.Parallel()

	// WithTenantTx should reject tenantID == 0 before even touching the DB.
	// We pass nil because the function should error before using it (mock-like unit test).
	err := tenant.WithTenantTx(context.Background(), nil, 0, func(_ context.Context, _ bun.Tx) error {
		t.Fatal("callback should not be called with zero tenant ID")
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-zero tenant_id")
}

// NOTE: Integration tests for WithTenantTx and WithAdminTx that verify
// SET LOCAL ROLE and set_config require the phoenix_tenant / phoenix_admin
// PostgreSQL roles to exist. These roles are created in Phase 2 (WP 2.1).
// Those tests will be added once the roles are available.
