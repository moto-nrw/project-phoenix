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

func (s *fakeStore) LockLiveTraegerAccount(context.Context, int64) (bool, domain.OperationStats, error) {
	return true, domain.OperationStats{}, nil
}

func (s *fakeStore) InsertTraegerRole(context.Context, int64, int64, string) (domain.OperationStats, error) {
	return domain.OperationStats{}, nil
}

func (s *fakeStore) ListTraegerAccounts(context.Context, int64, []int64) ([]domain.TraegerAccount, domain.OperationStats, error) {
	return nil, domain.OperationStats{}, nil
}
