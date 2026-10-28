package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

// The owner fakes hold no Träger membership (ADR 0084); the Träger flows are
// covered by the HTTP tests against the real store.

func (fakeOwnerStore) ListActiveAccountTraegerIDs(context.Context, int64) ([]int64, domain.OperationStats, error) {
	return nil, domain.OperationStats{}, nil
}

func (fakeOwnerStore) ListTraegerSchoolIDs(context.Context, int64) ([]int64, domain.OperationStats, error) {
	return nil, domain.OperationStats{}, nil
}

func (fakeOwnerStore) ListTraegerMemberships(context.Context, int64) ([]organizationtenancy.TraegerMembership, domain.OperationStats, error) {
	return nil, domain.OperationStats{}, nil
}

func (fakeOwnerStore) UpsertActiveTraegerMembership(context.Context, int64, int64) (organizationtenancy.TraegerMembership, domain.OperationStats, error) {
	return organizationtenancy.TraegerMembership{}, domain.OperationStats{}, nil
}

func (fakeOwnerStore) DeactivateTraegerMembership(context.Context, int64, int64) (bool, domain.OperationStats, error) {
	return false, domain.OperationStats{}, nil
}

func (*fakeIdentity) FindAccountIDByEmail(context.Context, string) (int64, bool, error) {
	return 0, false, nil
}

func (*fakeIdentity) AssignTraegerRole(context.Context, int64, int64, string) error { return nil }

func (*fakeIdentity) InviteTraegerAccount(context.Context, int64, string, []string) (organizationtenancy.TraegerInvitation, error) {
	return organizationtenancy.TraegerInvitation{}, nil
}

func (*fakeIdentity) ListTraegerAccounts(context.Context, int64, []int64) ([]organizationtenancy.TraegerAccount, error) {
	return nil, nil
}

func (*fakeIdentity) ResetTraegerMFA(context.Context, int64) error { return nil }

func (*fakeIdentity) EndTraegerSessions(context.Context, int64) (int, error) { return 0, nil }

func (fakeOwnerStore) FindOpenTraegerErasureOrder(context.Context, int64, string) (organizationtenancy.TraegerErasureOrder, bool, domain.OperationStats, error) {
	return organizationtenancy.TraegerErasureOrder{}, false, domain.OperationStats{}, nil
}

func (fakeOwnerStore) InsertTraegerErasureOrder(context.Context, organizationtenancy.NewTraegerErasureOrder) (organizationtenancy.TraegerErasureOrder, domain.OperationStats, error) {
	return organizationtenancy.TraegerErasureOrder{}, domain.OperationStats{}, nil
}

func (fakeOwnerStore) WithdrawTraegerErasureOrder(context.Context, int64, int64) (bool, domain.OperationStats, error) {
	return false, domain.OperationStats{}, nil
}
