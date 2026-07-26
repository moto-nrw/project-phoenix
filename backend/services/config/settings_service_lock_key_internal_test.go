package config

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
)

// TestClassCollectionLockKey_IsPerTenant pins the advisory-lock key shared by
// the settings-side collection guard and the enrollment-side phase eligibility
// guard (#1663). Both reach it through the same LockClassCollectionPair method,
// so the two conflict by construction — but only as long as the key stays
// tenant-scoped. A key that ignored the tenant would serialize every school
// against every other; a key that varied per call site would stop the two
// writers from conflicting at all, which is the race the lock exists to close.
func TestClassCollectionLockKey_IsPerTenant(t *testing.T) {
	keyFor := func(tenantID int64) string {
		return classCollectionLockKey(tenant.WithTenantID(context.Background(), tenantID))
	}

	assert.Equal(t, "enrollment-class-collection:1", keyFor(1))
	assert.Equal(t, keyFor(1), keyFor(1), "the same tenant must always land on the same lock")
	assert.NotEqual(t, keyFor(1), keyFor(2), "different tenants must not block each other")
}
