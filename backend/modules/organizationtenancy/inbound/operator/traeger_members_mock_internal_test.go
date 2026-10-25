package operator

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// The route mock answers the Träger member capability empty (ADR 0084); the
// Träger member routes are covered against the composed provisioning.

func (m *mockProvisioningService) ListTraegerMembers(context.Context, int64) ([]organizationtenancy.TraegerMember, error) {
	return nil, nil
}

func (m *mockProvisioningService) AssignTraegerMember(context.Context, organizationtenancy.AssignTraegerMember) (organizationtenancy.TraegerMember, error) {
	return organizationtenancy.TraegerMember{}, nil
}

func (m *mockProvisioningService) ResetTraegerMemberMFA(context.Context, organizationtenancy.ResetTraegerMemberMFA) error {
	return nil
}
