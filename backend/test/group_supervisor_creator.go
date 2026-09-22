package test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// GroupSupervisorCreator persists supervisors for cross-module service tests.
// Production composition injects modules/studentpresence/internal/application/presence, whose own tests cover the
// additional validation and automatic work-session check-in.
type GroupSupervisorCreator struct {
	Repository studentpresence.SupervisionRecords
}

func (creator GroupSupervisorCreator) CreateGroupSupervisor(ctx context.Context, supervisor *studentpresence.GroupSupervision) error {
	supervisor.TenantID = tenant.FromContext(ctx)
	return creator.Repository.CreateSupervision(ctx, supervisor)
}
