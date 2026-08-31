package auth

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/require"
)

type membershipRevokedAccountRepoStub struct {
	noopAccountRepository
}

func (membershipRevokedAccountRepoStub) FindManageableByID(_ context.Context, id int64) (*authModel.Account, error) {
	return &authModel.Account{
		Model:  modelBase.Model{ID: id},
		Email:  "original@example.test",
		Active: true,
	}, nil
}

func (membershipRevokedAccountRepoStub) UpdateManageable(context.Context, *authModel.Account) error {
	return &modelBase.DatabaseError{Op: "update account", Err: modelBase.ErrNotFound}
}

func TestAccountManagement_MembershipRevokedDuringWriteReturnsNotFound(t *testing.T) {
	t.Parallel()

	service := &Service{
		repos: &repositories.Factory{Account: membershipRevokedAccountRepoStub{}},
	}
	tests := []struct {
		name   string
		action func() error
	}{
		{name: "update", action: func() error {
			return service.UpdateAccount(context.Background(), &authModel.Account{
				Model: modelBase.Model{ID: 42},
				Email: "updated@example.test",
			})
		}},
		{name: "activate", action: func() error {
			return service.ActivateAccount(context.Background(), 42)
		}},
		{name: "deactivate", action: func() error {
			return service.DeactivateAccount(context.Background(), 42)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, tt.action(), ErrAccountNotFound)
		})
	}
}
