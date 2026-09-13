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

type replacePermissionsRoleRepo struct {
	noopRoleRepository
}

func (replacePermissionsRoleRepo) FindByIDForUpdate(context.Context, int64) (*authModel.Role, error) {
	return &authModel.Role{}, nil
}

type failingPermissionLookupRepo struct {
	authModel.PermissionRepository
	err error
}

func (r failingPermissionLookupRepo) FindByID(context.Context, interface{}) (*authModel.Permission, error) {
	return nil, r.err
}

func TestReplaceRolePermissions_PropagatesPermissionLookupFailure(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("permission database unavailable")
	service := &Service{
		repos: &repositories.Factory{
			Role:       replacePermissionsRoleRepo{},
			Permission: failingPermissionLookupRepo{err: expectedErr},
		},
	}

	err := service.ReplaceRolePermissions(context.Background(), 1, []int64{2})

	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	assert.NotErrorIs(t, err, ErrPermissionNotFound)
}
