package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockReplacePermissionsRoleRepo struct {
	noopRoleRepository
}

func (mockReplacePermissionsRoleRepo) FindByIDForUpdate(context.Context, int64) (*authModel.Role, error) {
	return &authModel.Role{}, nil
}

type mockPermissionLookupRepo struct {
	authModel.PermissionRepository
	err error
}

func (r mockPermissionLookupRepo) FindByID(context.Context, interface{}) (*authModel.Permission, error) {
	return nil, r.err
}

func TestReplaceRolePermissions_PropagatesPermissionLookupFailure(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("permission database unavailable")
	service := &Service{
		repos: &repositories.Factory{
			Role:       mockReplacePermissionsRoleRepo{},
			Permission: mockPermissionLookupRepo{err: expectedErr},
		},
	}

	err := service.ReplaceRolePermissions(context.Background(), 1, []int64{2})

	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	assert.NotErrorIs(t, err, ErrPermissionNotFound)
}
