package compose

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// A changed offering pickup projection is announced only once the tenant
// transaction committed (#3560): a rolled-back reset or adjustment must not
// wake staff or guardian clients.
func TestAfterCommitAnnouncesOnlyOnCommit(t *testing.T) {
	t.Parallel()

	var tenants []int64
	var students [][]int64
	announce := afterCommit(func(tenantID int64, studentIDs []int64) {
		tenants = append(tenants, tenantID)
		students = append(students, studentIDs)
	})
	ctx, commit := tenant.WithAfterCommitHooksForTest(tenant.WithTenantID(context.Background(), 7))

	announce(ctx, []int64{42, 43})
	assert.Empty(t, tenants, "nobody may wake before commit")

	commit()
	assert.Equal(t, []int64{7}, tenants)
	assert.Equal(t, [][]int64{{42, 43}}, students)
}

func TestAfterCommitKeepsAMissingAnnouncerMissing(t *testing.T) {
	t.Parallel()

	assert.Nil(t, afterCommit(nil))
}
