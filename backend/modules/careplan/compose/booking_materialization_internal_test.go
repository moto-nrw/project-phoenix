package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
)

// A changed offering pickup projection is announced only once the tenant
// transaction committed (#3560): the composition binds Care Plan's
// announcement to these hooks, so a rolled-back reset or adjustment must not
// wake staff or guardian clients.
func TestTenantHooksDeferTheAnnouncementUntilCommit(t *testing.T) {
	t.Parallel()

	ctx, commit := tenant.WithAfterCommitHooksForTest(tenant.WithTenantID(context.Background(), 7))
	hooks := TenantHooks{}
	var announced []int64
	tenantID := hooks.TenantID(ctx)
	hooks.AfterCommit(ctx, func() { announced = append(announced, tenantID) })
	assert.Empty(t, announced, "nobody may wake before commit")

	commit()
	assert.Equal(t, []int64{7}, announced)
}
