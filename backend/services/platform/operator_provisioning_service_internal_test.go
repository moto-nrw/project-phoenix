package platform

import (
	"context"
	"database/sql"
	"testing"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type internalSchoolRepoStub struct {
	findByOrgAndSlugFn func(context.Context, int64, string) (*platformModels.School, error)
	findBySubdomainFn  func(context.Context, string) (*platformModels.School, error)
}

func (s *internalSchoolRepoStub) Create(context.Context, *platformModels.School) error { return nil }
func (s *internalSchoolRepoStub) FindByID(context.Context, int64) (*platformModels.School, error) {
	return nil, nil
}
func (s *internalSchoolRepoStub) FindBySlug(context.Context, string) (*platformModels.School, error) {
	return nil, nil
}
func (s *internalSchoolRepoStub) FindByOrganizationAndSlug(ctx context.Context, organizationID int64, slug string) (*platformModels.School, error) {
	if s.findByOrgAndSlugFn != nil {
		return s.findByOrgAndSlugFn(ctx, organizationID, slug)
	}
	return nil, nil
}
func (s *internalSchoolRepoStub) FindBySubdomain(ctx context.Context, subdomain string) (*platformModels.School, error) {
	if s.findBySubdomainFn != nil {
		return s.findBySubdomainFn(ctx, subdomain)
	}
	return nil, nil
}
func (s *internalSchoolRepoStub) List(context.Context) ([]*platformModels.School, error) {
	return nil, nil
}
func (s *internalSchoolRepoStub) ListActive(context.Context) ([]platformModels.School, error) {
	return nil, nil
}
func (s *internalSchoolRepoStub) FindActiveByAccountID(context.Context, int64) ([]platformModels.School, error) {
	return nil, nil
}

func TestNormalizeAdminInviteRequest(t *testing.T) {
	req := normalizeAdminInviteRequest(authSvc.InvitationRequest{Email: " Principal@Example.com "}, 4, 9)
	assert.Equal(t, "principal@example.com", req.Email)
	assert.Equal(t, int64(4), req.RoleID)
	assert.Equal(t, int64(9), req.TenantID)
}

func TestIsSchoolLookupNotFound(t *testing.T) {
	assert.True(t, isSchoolLookupNotFound(sql.ErrNoRows))
	assert.True(t, isSchoolLookupNotFound(&modelBase.DatabaseError{Op: "find", Err: sql.ErrNoRows}))
	assert.False(t, isSchoolLookupNotFound(assert.AnError))
	assert.False(t, isSchoolLookupNotFound(nil))
}

func TestMapSchoolCreateConflict_NilSchool(t *testing.T) {
	err := mapSchoolCreateConflict(context.Background(), &internalSchoolRepoStub{}, nil)
	require.Error(t, err)
	var conflictErr *ConflictError
	require.ErrorAs(t, err, &conflictErr)
}

func TestMapSchoolCreateConflict_SlugConflict(t *testing.T) {
	err := mapSchoolCreateConflict(context.Background(), &internalSchoolRepoStub{
		findByOrgAndSlugFn: func(context.Context, int64, string) (*platformModels.School, error) {
			return &platformModels.School{Model: modelBase.Model{ID: 1}, Name: "Existing"}, nil
		},
	}, &platformModels.School{
		OrganizationID: 2,
		Name:           "New School",
		Slug:           "new-school",
		Subdomain:      "new-school",
	})
	require.Error(t, err)
	var conflictErr *ConflictError
	require.ErrorAs(t, err, &conflictErr)
	assert.Contains(t, err.Error(), "school slug already exists")
}

func TestMapSchoolCreateConflict_SubdomainConflict(t *testing.T) {
	err := mapSchoolCreateConflict(context.Background(), &internalSchoolRepoStub{
		findBySubdomainFn: func(context.Context, string) (*platformModels.School, error) {
			return &platformModels.School{Model: modelBase.Model{ID: 2}, Name: "Existing"}, nil
		},
	}, &platformModels.School{
		OrganizationID: 2,
		Name:           "New School",
		Slug:           "new-school",
		Subdomain:      "new-school",
	})
	require.Error(t, err)
	var conflictErr *ConflictError
	require.ErrorAs(t, err, &conflictErr)
	assert.Contains(t, err.Error(), "school subdomain already exists")
}

func TestWithAdminTx_WithoutHandlerRunsCallback(t *testing.T) {
	service := &operatorProvisioningService{}
	called := false
	err := service.withAdminTx(context.Background(), func(context.Context) error {
		called = true
		return nil
	})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestGetLogger_Default(t *testing.T) {
	service := &operatorProvisioningService{}
	assert.NotNil(t, service.getLogger())
}
