package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The login fakes hold no Träger role (ADR 0084); the Träger sign-in is
// covered by the HTTP tests against the real store.

func (s *fakeStore) ListTraegerRoles(context.Context, int64, int64) ([]string, domain.OperationStats, error) {
	return nil, domain.OperationStats{}, nil
}

func (s *fakeStore) LockLiveTraegerAccount(context.Context, int64) (live, active bool, stats domain.OperationStats, err error) {
	return true, true, domain.OperationStats{}, nil
}

func (s *fakeStore) InsertTraegerRole(context.Context, int64, int64, string) (domain.OperationStats, error) {
	return domain.OperationStats{}, nil
}

func (s *fakeStore) ListTraegerAccounts(context.Context, int64, []int64) ([]domain.TraegerAccount, domain.OperationStats, error) {
	return nil, domain.OperationStats{}, nil
}

func (s *fakeStore) DeleteTraegerSessions(context.Context, int64) (int64, domain.OperationStats, error) {
	return 0, domain.OperationStats{}, nil
}

func (s *fakeStore) FindTraegerSchoolRole(context.Context, string, int64) (domain.RoleAssignment, []string, bool, domain.OperationStats, error) {
	return domain.RoleAssignment{}, nil, false, domain.OperationStats{}, nil
}

// The login fakes hold no parents-portal access at a Träger (ADR 0216); the
// Träger parent session is covered against the real store.

func (s *fakeStore) ListTraegerParentAccess(context.Context, int64) ([]int64, domain.OperationStats, error) {
	return nil, domain.OperationStats{}, nil
}

func (s *fakeStore) HasTraegerParentAccess(context.Context, int64, int64, bool) (bool, domain.OperationStats, error) {
	return false, domain.OperationStats{}, nil
}

func (s *fakeStore) GrantTraegerParentAccess(context.Context, int64, int64) (domain.OperationStats, error) {
	return domain.OperationStats{}, nil
}
