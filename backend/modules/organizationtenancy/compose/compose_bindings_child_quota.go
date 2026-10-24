package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/postgres"
)

type retainedChildQuotaLimits struct{}

func (retainedChildQuotaLimits) ChildQuotaLimit(ctx context.Context) (int, bool, error) {
	return postgres.ReadComposedChildQuota(ctx)
}
