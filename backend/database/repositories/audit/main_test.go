package audit

import (
	"context"
	"testing"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func auditTestTenantID(ctx context.Context) int64 { return auditModels.TenantIDFromContext(ctx) }

// TestMain gives every test in this binary its own tenant (#2419), so
// parallel tests in the shared package clone cannot see or overwrite
// each other's rows.
func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
