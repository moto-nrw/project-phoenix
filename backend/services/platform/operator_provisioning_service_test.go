package platform_test

import (
	"context"
	"net"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type mockOrganizationRepo struct {
	findByIDFn   func(context.Context, int64) (*platformModels.Organization, error)
	findBySlugFn func(context.Context, string) (*platformModels.Organization, error)
	createFn     func(context.Context, *platformModels.Organization) error
}

func (m *mockOrganizationRepo) Create(ctx context.Context, organization *platformModels.Organization) error {
	if m.createFn != nil {
		return m.createFn(ctx, organization)
	}
	return nil
}
func (m *mockOrganizationRepo) FindByID(ctx context.Context, id int64) (*platformModels.Organization, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockOrganizationRepo) FindBySlug(ctx context.Context, slug string) (*platformModels.Organization, error) {
	if m.findBySlugFn != nil {
		return m.findBySlugFn(ctx, slug)
	}
	return nil, nil
}
func (m *mockOrganizationRepo) List(context.Context) ([]*platformModels.Organization, error) {
	return nil, nil
}

type mockSchoolRepo struct {
	findByIDFn         func(context.Context, int64) (*platformModels.School, error)
	findByOrgAndSlugFn func(context.Context, int64, string) (*platformModels.School, error)
	findBySubdomainFn  func(context.Context, string) (*platformModels.School, error)
	createFn           func(context.Context, *platformModels.School) error
}

func (m *mockSchoolRepo) Create(ctx context.Context, school *platformModels.School) error {
	if m.createFn != nil {
		return m.createFn(ctx, school)
	}
	return nil
}
func (m *mockSchoolRepo) FindByID(ctx context.Context, id int64) (*platformModels.School, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockSchoolRepo) FindBySlug(context.Context, string) (*platformModels.School, error) {
	return nil, nil
}
func (m *mockSchoolRepo) FindByOrganizationAndSlug(ctx context.Context, organizationID int64, slug string) (*platformModels.School, error) {
	if m.findByOrgAndSlugFn != nil {
		return m.findByOrgAndSlugFn(ctx, organizationID, slug)
	}
	return nil, nil
}
func (m *mockSchoolRepo) FindBySubdomain(ctx context.Context, subdomain string) (*platformModels.School, error) {
	if m.findBySubdomainFn != nil {
		return m.findBySubdomainFn(ctx, subdomain)
	}
	return nil, nil
}
func (m *mockSchoolRepo) List(context.Context) ([]*platformModels.School, error) {
	return nil, nil
}
func (m *mockSchoolRepo) ListActive(context.Context) ([]platformModels.School, error) {
	return nil, nil
}
func (m *mockSchoolRepo) FindActiveByAccountID(context.Context, int64) ([]platformModels.School, error) {
	return nil, nil
}

type mockRoleRepo struct {
	role  *authModels.Role
	roles []*authModels.Role
}

type mockCategoryRepo struct {
	created []*activityModels.Category
}

func (m *mockCategoryRepo) Create(_ context.Context, category *activityModels.Category) error {
	if category != nil {
		m.created = append(m.created, category)
	}
	return nil
}
func (m *mockCategoryRepo) FindByID(context.Context, interface{}) (*activityModels.Category, error) {
	return nil, nil
}
func (m *mockCategoryRepo) Update(context.Context, *activityModels.Category) error { return nil }
func (m *mockCategoryRepo) Delete(context.Context, interface{}) error              { return nil }
func (m *mockCategoryRepo) List(context.Context, *base.QueryOptions) ([]*activityModels.Category, error) {
	return nil, nil
}
func (m *mockCategoryRepo) FindByName(context.Context, string) (*activityModels.Category, error) {
	return nil, nil
}
func (m *mockCategoryRepo) ListAll(context.Context) ([]*activityModels.Category, error) {
	return nil, nil
}

func (m *mockRoleRepo) Create(context.Context, *authModels.Role) error { return nil }
func (m *mockRoleRepo) FindByID(context.Context, interface{}) (*authModels.Role, error) {
	return nil, nil
}
func (m *mockRoleRepo) Update(context.Context, *authModels.Role) error { return nil }
func (m *mockRoleRepo) Delete(context.Context, interface{}) error      { return nil }
func (m *mockRoleRepo) List(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
	if m.roles != nil {
		return m.roles, nil
	}
	if m.role != nil {
		return []*authModels.Role{m.role}, nil
	}
	return nil, nil
}
func (m *mockRoleRepo) FindByName(context.Context, string) (*authModels.Role, error) {
	return m.role, nil
}
func (m *mockRoleRepo) FindByAccountID(context.Context, int64) ([]*authModels.Role, error) {
	return nil, nil
}
func (m *mockRoleRepo) AssignRoleToAccount(context.Context, int64, int64) error { return nil }
func (m *mockRoleRepo) RemoveRoleFromAccount(context.Context, int64, int64) error {
	return nil
}
func (m *mockRoleRepo) GetRoleWithPermissions(context.Context, int64) (*authModels.Role, error) {
	return nil, nil
}

type mockInvitationService struct {
	req authSvc.InvitationRequest
}

func (m *mockInvitationService) WithTx(tx bun.Tx) interface{} { return m }
func (m *mockInvitationService) CreateInvitation(_ context.Context, req authSvc.InvitationRequest) (*authModels.InvitationToken, error) {
	m.req = req
	return &authModels.InvitationToken{
		Model:     base.Model{ID: 10},
		Email:     req.Email,
		RoleID:    req.RoleID,
		CreatedBy: nil,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}, nil
}
func (m *mockInvitationService) ValidateInvitation(context.Context, string) (*authSvc.InvitationValidationResult, error) {
	return nil, nil
}
func (m *mockInvitationService) AcceptInvitation(context.Context, string, authSvc.UserRegistrationData) (*authModels.Account, error) {
	return nil, nil
}
func (m *mockInvitationService) ResendInvitation(context.Context, int64, int64) error { return nil }
func (m *mockInvitationService) ListPendingInvitations(context.Context) ([]*authModels.InvitationToken, error) {
	return nil, nil
}
func (m *mockInvitationService) RevokeInvitation(context.Context, int64, int64) error { return nil }
func (m *mockInvitationService) CleanupExpiredInvitations(context.Context) (int, error) {
	return 0, nil
}

func TestOperatorProvisioningService_CreateSchool_AllowsDuplicateSlugAcrossOrganizations(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: base.Model{ID: 2}, Name: "Org B", Slug: "org-b", Active: true}, nil
			},
		},
		SchoolRepo: &mockSchoolRepo{
			findByOrgAndSlugFn: func(context.Context, int64, string) (*platformModels.School, error) {
				return nil, nil
			},
			findBySubdomainFn: func(context.Context, string) (*platformModels.School, error) {
				return nil, nil
			},
			createFn: func(_ context.Context, school *platformModels.School) error {
				school.ID = 55
				return nil
			},
		},
		CategoryRepo: &mockCategoryRepo{},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	school, err := service.CreateSchool(context.Background(), &platformModels.School{
		OrganizationID: 2,
		Name:           "GGS Europaschule",
		Slug:           "ggs-europa",
		Subdomain:      "ggs-europa-org-b",
		Active:         true,
	}, 7, net.IPv4(127, 0, 0, 1))
	require.NoError(t, err)
	require.NotNil(t, school)
	require.Equal(t, int64(55), school.ID)
}

func TestOperatorProvisioningService_InviteSchoolAdmin_DoesNotRequireAuthCreatorAccount(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	invitations := &mockInvitationService{}
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SchoolRepo: &mockSchoolRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{Model: base.Model{ID: 9}, OrganizationID: 3, Name: "School", Slug: "school", Subdomain: "school", Active: true}, nil
			},
		},
		RoleRepo: &mockRoleRepo{roles: []*authModels.Role{
			{Model: base.Model{ID: 4}, Name: "admin", IsSystem: true},
			{Model: base.Model{ID: 5}, Name: "admin", IsSystem: false, TenantID: base.Int64Ptr(9)},
		}},
		InvitationService: invitations,
		AuditLogRepo:      &mockAuditLogRepoShared{},
		DB:                bunDB,
	})

	invitation, err := service.InviteSchoolAdmin(context.Background(), 9, 11, net.IPv4(127, 0, 0, 1), authSvc.InvitationRequest{
		Email: "principal@example.com",
	})
	require.NoError(t, err)
	require.NotNil(t, invitation)
	require.Equal(t, int64(0), invitations.req.CreatedBy)
}
