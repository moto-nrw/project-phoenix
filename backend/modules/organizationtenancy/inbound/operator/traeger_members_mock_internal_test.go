package operator

import (
	"context"
	"net"

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

func (m *mockProvisioningService) FindOpenTraegerErasureOrder(context.Context, int64) (organizationtenancy.TraegerErasureOrder, bool, error) {
	return organizationtenancy.TraegerErasureOrder{}, false, nil
}

func (m *mockProvisioningService) RecordTraegerErasureOrder(context.Context, organizationtenancy.RecordTraegerErasureOrder) (organizationtenancy.TraegerErasureOrder, error) {
	return organizationtenancy.TraegerErasureOrder{}, nil
}

func (m *mockProvisioningService) WithdrawTraegerErasureOrder(context.Context, int64, int64, net.IP) error {
	return nil
}
