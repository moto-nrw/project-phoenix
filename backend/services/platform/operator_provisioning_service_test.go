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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type mockOrganizationRepo struct {
	findByIDFn   func(context.Context, int64) (*platformModels.Organization, error)
	findBySlugFn func(context.Context, string) (*platformModels.Organization, error)
	createFn     func(context.Context, *platformModels.Organization) error
	updateFn     func(context.Context, *platformModels.Organization) error
	listFn       func(context.Context) ([]*platformModels.Organization, error)
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
	if m.listFn != nil {
		return m.listFn(context.Background())
	}
	return nil, nil
}
func (m *mockOrganizationRepo) Update(ctx context.Context, org *platformModels.Organization) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, org)
	}
	return nil
}

type mockSchoolRepo struct {
	findByIDFn         func(context.Context, int64) (*platformModels.School, error)
	findByOrgAndSlugFn func(context.Context, int64, string) (*platformModels.School, error)
	findBySubdomainFn  func(context.Context, string) (*platformModels.School, error)
	createFn           func(context.Context, *platformModels.School) error
	updateFn           func(context.Context, *platformModels.School) error
	listFn             func(context.Context) ([]*platformModels.School, error)
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
	if m.listFn != nil {
		return m.listFn(context.Background())
	}
	return nil, nil
}
func (m *mockSchoolRepo) ListActive(context.Context) ([]platformModels.School, error) {
	return nil, nil
}
func (m *mockSchoolRepo) FindActiveByAccountID(context.Context, int64) ([]platformModels.School, error) {
	return nil, nil
}
func (m *mockSchoolRepo) Update(ctx context.Context, school *platformModels.School) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, school)
	}
	return nil
}

type mockRoleRepo struct {
	role  *authModels.Role
	roles []*authModels.Role
}

type mockCategoryRepo struct {
	created  []*activityModels.Category
	createFn func(context.Context, *activityModels.Category) error
}

func (m *mockCategoryRepo) Create(_ context.Context, category *activityModels.Category) error {
	if m.createFn != nil {
		return m.createFn(context.Background(), category)
	}
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

func TestOperatorProvisioningService_CreateOrganization_Success(t *testing.T) {
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
			findBySlugFn: func(context.Context, string) (*platformModels.Organization, error) {
				return nil, nil
			},
			createFn: func(_ context.Context, organization *platformModels.Organization) error {
				organization.ID = 77
				return nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	org, err := service.CreateOrganization(context.Background(), &platformModels.Organization{
		Name:   "Stadt Koeln",
		Slug:   "stadt-koeln",
		Active: true,
	}, 7, net.IPv4(127, 0, 0, 1))
	require.NoError(t, err)
	require.NotNil(t, org)
	require.Equal(t, int64(77), org.ID)
}

func TestOperatorProvisioningService_CreateOrganization_Conflict(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{
			findBySlugFn: func(context.Context, string) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: base.Model{ID: 5}, Name: "Existing", Slug: "stadt-koeln", Active: true}, nil
			},
		},
	})

	org, err := service.CreateOrganization(context.Background(), &platformModels.Organization{
		Name:   "Stadt Koeln",
		Slug:   "stadt-koeln",
		Active: true,
	}, 7, net.IPv4(127, 0, 0, 1))
	require.Nil(t, org)
	require.Error(t, err)
	var conflictErr *platformSvc.ConflictError
	require.ErrorAs(t, err, &conflictErr)
}

func TestOperatorProvisioningService_ListOrganizations(t *testing.T) {
	expected := []*platformModels.Organization{
		{Model: base.Model{ID: 1}, Name: "Org A", Slug: "org-a", Active: true},
	}
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{
			listFn: func(context.Context) ([]*platformModels.Organization, error) {
				return expected, nil
			},
		},
	})

	organizations, err := service.ListOrganizations(context.Background())
	require.NoError(t, err)
	require.Equal(t, expected, organizations)
}

func TestOperatorProvisioningService_ListSchools(t *testing.T) {
	expected := []*platformModels.School{
		{Model: base.Model{ID: 2}, OrganizationID: 1, Name: "School A", Slug: "school-a", Subdomain: "school-a", Active: true},
	}
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SchoolRepo: &mockSchoolRepo{
			listFn: func(context.Context) ([]*platformModels.School, error) {
				return expected, nil
			},
		},
	})

	schools, err := service.ListSchools(context.Background())
	require.NoError(t, err)
	require.Equal(t, expected, schools)
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

func TestOperatorProvisioningService_CreateSchool_OrganizationNotFound(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{},
		SchoolRepo:       &mockSchoolRepo{},
	})

	school, err := service.CreateSchool(context.Background(), &platformModels.School{
		OrganizationID: 99,
		Name:           "Missing Org School",
		Slug:           "missing-org-school",
		Subdomain:      "missing-org-school",
		Active:         true,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	require.Error(t, err)
	var notFoundErr *platformSvc.OrganizationNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestOperatorProvisioningService_CreateSchool_SlugConflict(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: base.Model{ID: 2}, Name: "Org", Slug: "org", Active: true}, nil
			},
		},
		SchoolRepo: &mockSchoolRepo{
			findByOrgAndSlugFn: func(context.Context, int64, string) (*platformModels.School, error) {
				return &platformModels.School{Model: base.Model{ID: 8}, OrganizationID: 2, Name: "Existing", Slug: "shared", Subdomain: "existing", Active: true}, nil
			},
		},
	})

	school, err := service.CreateSchool(context.Background(), &platformModels.School{
		OrganizationID: 2,
		Name:           "New School",
		Slug:           "shared",
		Subdomain:      "new-school",
		Active:         true,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	require.Error(t, err)
	var conflictErr *platformSvc.ConflictError
	require.ErrorAs(t, err, &conflictErr)
}

func TestOperatorProvisioningService_CreateSchool_SubdomainConflict(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: base.Model{ID: 2}, Name: "Org", Slug: "org", Active: true}, nil
			},
		},
		SchoolRepo: &mockSchoolRepo{
			findByOrgAndSlugFn: func(context.Context, int64, string) (*platformModels.School, error) {
				return nil, nil
			},
			findBySubdomainFn: func(context.Context, string) (*platformModels.School, error) {
				return &platformModels.School{Model: base.Model{ID: 8}, OrganizationID: 3, Name: "Existing", Slug: "existing", Subdomain: "shared-subdomain", Active: true}, nil
			},
		},
	})

	school, err := service.CreateSchool(context.Background(), &platformModels.School{
		OrganizationID: 2,
		Name:           "New School",
		Slug:           "new-school",
		Subdomain:      "shared-subdomain",
		Active:         true,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	require.Error(t, err)
	var conflictErr *platformSvc.ConflictError
	require.ErrorAs(t, err, &conflictErr)
}

func TestOperatorProvisioningService_CreateSchool_CategorySeedError(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: base.Model{ID: 2}, Name: "Org", Slug: "org", Active: true}, nil
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
		CategoryRepo: &mockCategoryRepo{
			createFn: func(context.Context, *activityModels.Category) error {
				return assert.AnError
			},
		},
	})

	school, err := service.CreateSchool(context.Background(), &platformModels.School{
		OrganizationID: 2,
		Name:           "Seed Error School",
		Slug:           "seed-error-school",
		Subdomain:      "seed-error-school",
		Active:         true,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	require.ErrorIs(t, err, assert.AnError)
}

func TestOperatorProvisioningService_InviteSchoolAdmin_SchoolNotFound(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SchoolRepo: &mockSchoolRepo{},
		RoleRepo:   &mockRoleRepo{role: &authModels.Role{Model: base.Model{ID: 4}, Name: "admin", IsSystem: true}},
	})

	invitation, err := service.InviteSchoolAdmin(context.Background(), 404, 11, net.IPv4(127, 0, 0, 1), authSvc.InvitationRequest{
		Email: "principal@example.com",
	})
	require.Nil(t, invitation)
	require.Error(t, err)
	var schoolErr *platformSvc.SchoolNotFoundError
	require.ErrorAs(t, err, &schoolErr)
}

func TestOperatorProvisioningService_InviteSchoolAdmin_InactiveSchool(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SchoolRepo: &mockSchoolRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{Model: base.Model{ID: 9}, OrganizationID: 3, Name: "School", Slug: "school", Subdomain: "school", Active: false}, nil
			},
		},
		RoleRepo: &mockRoleRepo{role: &authModels.Role{Model: base.Model{ID: 4}, Name: "admin", IsSystem: true}},
	})

	invitation, err := service.InviteSchoolAdmin(context.Background(), 9, 11, net.IPv4(127, 0, 0, 1), authSvc.InvitationRequest{
		Email: "principal@example.com",
	})
	require.Nil(t, invitation)
	require.Error(t, err)
	var invalidErr *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalidErr)
}

func TestOperatorProvisioningService_InviteSchoolAdmin_AdminRoleMissing(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SchoolRepo: &mockSchoolRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{Model: base.Model{ID: 9}, OrganizationID: 3, Name: "School", Slug: "school", Subdomain: "school", Active: true}, nil
			},
		},
		RoleRepo: &mockRoleRepo{roles: []*authModels.Role{
			{Model: base.Model{ID: 5}, Name: "admin", IsSystem: false, TenantID: base.Int64Ptr(9)},
		}},
	})

	invitation, err := service.InviteSchoolAdmin(context.Background(), 9, 11, net.IPv4(127, 0, 0, 1), authSvc.InvitationRequest{
		Email: "principal@example.com",
	})
	require.Nil(t, invitation)
	require.Error(t, err)
	var invalidErr *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalidErr)
}

// --- UpdateOrganization tests ---

func TestOperatorProvisioningService_UpdateOrganization_Success(t *testing.T) {
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

	var updatedOrg *platformModels.Organization
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: base.Model{ID: 10}, Name: "Old Name", Slug: "old-slug", Active: true}, nil
			},
			findBySlugFn: func(_ context.Context, slug string) (*platformModels.Organization, error) {
				return nil, nil // no conflict
			},
			updateFn: func(_ context.Context, org *platformModels.Organization) error {
				updatedOrg = org
				return nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	org, err := service.UpdateOrganization(context.Background(), 10, platformSvc.UpdateOrganizationRequest{
		Name:   "New Name",
		Slug:   "new-slug",
		Active: false,
	}, 7, net.IPv4(127, 0, 0, 1))
	require.NoError(t, err)
	require.NotNil(t, org)
	assert.Equal(t, int64(10), org.ID)
	assert.Equal(t, "New Name", org.Name)
	assert.Equal(t, "new-slug", org.Slug)
	assert.False(t, org.Active)
	assert.Equal(t, updatedOrg, org)
}

func TestOperatorProvisioningService_UpdateOrganization_NotFound(t *testing.T) {
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
	mock.ExpectRollback()

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return nil, nil // not found
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	org, err := service.UpdateOrganization(context.Background(), 999, platformSvc.UpdateOrganizationRequest{
		Name:   "Anything",
		Slug:   "anything",
		Active: true,
	}, 7, net.IPv4(127, 0, 0, 1))
	require.Nil(t, org)
	require.Error(t, err)
	var notFoundErr *platformSvc.OrganizationNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestOperatorProvisioningService_UpdateOrganization_SlugConflict(t *testing.T) {
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
	mock.ExpectRollback()

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: base.Model{ID: 10}, Name: "My Org", Slug: "my-org", Active: true}, nil
			},
			findBySlugFn: func(_ context.Context, slug string) (*platformModels.Organization, error) {
				// slug taken by a different org
				return &platformModels.Organization{Model: base.Model{ID: 20}, Name: "Other Org", Slug: "taken-slug", Active: true}, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	org, err := service.UpdateOrganization(context.Background(), 10, platformSvc.UpdateOrganizationRequest{
		Name:   "My Org",
		Slug:   "taken-slug",
		Active: true,
	}, 7, net.IPv4(127, 0, 0, 1))
	require.Nil(t, org)
	require.Error(t, err)
	var conflictErr *platformSvc.ConflictError
	require.ErrorAs(t, err, &conflictErr)
}

func TestOperatorProvisioningService_UpdateOrganization_SameSlugNoConflict(t *testing.T) {
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
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: base.Model{ID: 10}, Name: "Old Name", Slug: "same-slug", Active: true}, nil
			},
			// FindBySlug should not be called when slug is unchanged, but if it is,
			// returning the same org (same ID) must not trigger a conflict.
			findBySlugFn: func(_ context.Context, slug string) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: base.Model{ID: 10}, Name: "Old Name", Slug: "same-slug", Active: true}, nil
			},
			updateFn: func(_ context.Context, org *platformModels.Organization) error {
				return nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	org, err := service.UpdateOrganization(context.Background(), 10, platformSvc.UpdateOrganizationRequest{
		Name:   "Updated Name",
		Slug:   "same-slug",
		Active: true,
	}, 7, net.IPv4(127, 0, 0, 1))
	require.NoError(t, err)
	require.NotNil(t, org)
	assert.Equal(t, "Updated Name", org.Name)
	assert.Equal(t, "same-slug", org.Slug)
}

// --- UpdateSchool tests ---

func TestOperatorProvisioningService_UpdateSchool_Success(t *testing.T) {
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

	var updatedSchool *platformModels.School
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{},
		SchoolRepo: &mockSchoolRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: 50},
					OrganizationID: 2,
					Name:           "Old School",
					Slug:           "old-school",
					Subdomain:      "old-school",
					Active:         true,
				}, nil
			},
			findByOrgAndSlugFn: func(_ context.Context, orgID int64, slug string) (*platformModels.School, error) {
				return nil, nil // no conflict
			},
			findBySubdomainFn: func(_ context.Context, subdomain string) (*platformModels.School, error) {
				return nil, nil // no conflict
			},
			updateFn: func(_ context.Context, school *platformModels.School) error {
				updatedSchool = school
				return nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	school, err := service.UpdateSchool(context.Background(), 50, platformSvc.UpdateSchoolRequest{
		OrganizationID: 2,
		Name:           "New School Name",
		Slug:           "new-school",
		Subdomain:      "new-subdomain",
		Address:        "123 Main St",
		City:           "Cologne",
		Zip:            "50667",
		Phone:          "+49 221 1234567",
		Email:          "school@example.com",
		Active:         true,
	}, 7, net.IPv4(127, 0, 0, 1))
	require.NoError(t, err)
	require.NotNil(t, school)
	assert.Equal(t, int64(50), school.ID)
	assert.Equal(t, "New School Name", school.Name)
	assert.Equal(t, "new-school", school.Slug)
	assert.Equal(t, "new-subdomain", school.Subdomain)
	assert.Equal(t, "123 Main St", school.Address)
	assert.Equal(t, updatedSchool, school)
}

func TestOperatorProvisioningService_UpdateSchool_NotFound(t *testing.T) {
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
	mock.ExpectRollback()

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{},
		SchoolRepo: &mockSchoolRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return nil, nil // not found
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	school, err := service.UpdateSchool(context.Background(), 404, platformSvc.UpdateSchoolRequest{
		OrganizationID: 1,
		Name:           "Anything",
		Slug:           "anything",
		Subdomain:      "anything",
		Active:         true,
	}, 7, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	require.Error(t, err)
	var notFoundErr *platformSvc.SchoolNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestOperatorProvisioningService_UpdateSchool_SlugConflict(t *testing.T) {
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
	mock.ExpectRollback()

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{},
		SchoolRepo: &mockSchoolRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: 50},
					OrganizationID: 2,
					Name:           "My School",
					Slug:           "my-school",
					Subdomain:      "my-school",
					Active:         true,
				}, nil
			},
			findByOrgAndSlugFn: func(_ context.Context, orgID int64, slug string) (*platformModels.School, error) {
				// slug taken by a different school in the same org
				return &platformModels.School{Model: base.Model{ID: 99}, OrganizationID: 2, Name: "Other", Slug: "taken-slug", Subdomain: "other", Active: true}, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	school, err := service.UpdateSchool(context.Background(), 50, platformSvc.UpdateSchoolRequest{
		OrganizationID: 2,
		Name:           "My School",
		Slug:           "taken-slug",
		Subdomain:      "my-school",
		Active:         true,
	}, 7, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	require.Error(t, err)
	var conflictErr *platformSvc.ConflictError
	require.ErrorAs(t, err, &conflictErr)
}

func TestOperatorProvisioningService_UpdateSchool_SubdomainConflict(t *testing.T) {
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
	mock.ExpectRollback()

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{},
		SchoolRepo: &mockSchoolRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: 50},
					OrganizationID: 2,
					Name:           "My School",
					Slug:           "my-school",
					Subdomain:      "my-school",
					Active:         true,
				}, nil
			},
			findByOrgAndSlugFn: func(_ context.Context, orgID int64, slug string) (*platformModels.School, error) {
				return nil, nil // slug is fine
			},
			findBySubdomainFn: func(_ context.Context, subdomain string) (*platformModels.School, error) {
				// subdomain taken by a different school
				return &platformModels.School{Model: base.Model{ID: 88}, OrganizationID: 3, Name: "Other", Slug: "other", Subdomain: "taken-sub", Active: true}, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	school, err := service.UpdateSchool(context.Background(), 50, platformSvc.UpdateSchoolRequest{
		OrganizationID: 2,
		Name:           "My School",
		Slug:           "my-school",
		Subdomain:      "taken-sub",
		Active:         true,
	}, 7, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	require.Error(t, err)
	var conflictErr *platformSvc.ConflictError
	require.ErrorAs(t, err, &conflictErr)
}

func TestOperatorProvisioningService_UpdateSchool_OrganizationNotFound(t *testing.T) {
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
	mock.ExpectRollback()

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return nil, nil // org not found
			},
		},
		SchoolRepo: &mockSchoolRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: 50},
					OrganizationID: 2,
					Name:           "My School",
					Slug:           "my-school",
					Subdomain:      "my-school",
					Active:         true,
				}, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	school, err := service.UpdateSchool(context.Background(), 50, platformSvc.UpdateSchoolRequest{
		OrganizationID: 999, // changing to nonexistent org
		Name:           "My School",
		Slug:           "my-school",
		Subdomain:      "my-school",
		Active:         true,
	}, 7, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	require.Error(t, err)
	var notFoundErr *platformSvc.OrganizationNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestOperatorProvisioningService_UpdateSchool_ChangeOrganization(t *testing.T) {
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

	newOrgID := int64(5) // nolint: target organization for the move
	var updatedSchool *platformModels.School
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				if id == newOrgID {
					return &platformModels.Organization{Model: base.Model{ID: newOrgID}, Name: "New Org", Slug: "new-org", Active: true}, nil
				}
				return nil, nil
			},
		},
		SchoolRepo: &mockSchoolRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: 50},
					OrganizationID: 2,
					Name:           "My School",
					Slug:           "my-school",
					Subdomain:      "my-school",
					Active:         true,
				}, nil
			},
			findByOrgAndSlugFn: func(_ context.Context, orgID int64, slug string) (*platformModels.School, error) {
				return nil, nil // no slug conflict in new org
			},
			findBySubdomainFn: func(_ context.Context, subdomain string) (*platformModels.School, error) {
				return nil, nil
			},
			updateFn: func(_ context.Context, school *platformModels.School) error {
				updatedSchool = school
				return nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	school, err := service.UpdateSchool(context.Background(), 50, platformSvc.UpdateSchoolRequest{
		OrganizationID: newOrgID, // changed from 2 to newOrgID
		Name:           "My School",
		Slug:           "my-school",
		Subdomain:      "my-school",
		Active:         true,
	}, 7, net.IPv4(127, 0, 0, 1))
	require.NoError(t, err)
	require.NotNil(t, school)
	assert.Equal(t, newOrgID, school.OrganizationID)
	assert.Equal(t, int64(50), school.ID)
	assert.Equal(t, updatedSchool, school)
}
