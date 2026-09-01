package test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// GroupSupervisorCreator persists supervisors for cross-module service tests.
// Production composition injects services/active, whose own tests cover the
// additional validation and automatic work-session check-in.
type GroupSupervisorCreator struct {
	Repository active.GroupSupervisorRepository
}

func (creator GroupSupervisorCreator) CreateGroupSupervisor(ctx context.Context, supervisor *active.GroupSupervisor) error {
	supervisor.SetTenantID(tenant.FromContext(ctx))
	return creator.Repository.Create(ctx, supervisor)
}
