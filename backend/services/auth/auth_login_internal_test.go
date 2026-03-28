package auth

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type stubAuthLoginSchoolRepo struct {
	findByIDFn        func(context.Context, int64) (*platformModels.School, error)
	findBySubdomainFn func(context.Context, string) (*platformModels.School, error)
}

func (s stubAuthLoginSchoolRepo) Create(context.Context, *platformModels.School) error {
	panic("unexpected Create")
}
func (s stubAuthLoginSchoolRepo) FindByID(ctx context.Context, id int64) (*platformModels.School, error) {
	return s.findByIDFn(ctx, id)
}
func (s stubAuthLoginSchoolRepo) FindBySlug(context.Context, string) (*platformModels.School, error) {
	panic("unexpected FindBySlug")
}
func (s stubAuthLoginSchoolRepo) FindByOrganizationAndSlug(context.Context, int64, string) (*platformModels.School, error) {
	panic("unexpected FindByOrganizationAndSlug")
}
func (s stubAuthLoginSchoolRepo) FindBySubdomain(ctx context.Context, subdomain string) (*platformModels.School, error) {
	return s.findBySubdomainFn(ctx, subdomain)
}
func (s stubAuthLoginSchoolRepo) List(context.Context) ([]*platformModels.School, error) {
	panic("unexpected List")
}
func (s stubAuthLoginSchoolRepo) ListActive(context.Context) ([]platformModels.School, error) {
	panic("unexpected ListActive")
}
func (s stubAuthLoginSchoolRepo) ListPublic(context.Context) ([]platformModels.School, error) {
	return nil, nil
}
func (s stubAuthLoginSchoolRepo) FindActiveByAccountID(context.Context, int64) ([]platformModels.School, error) {
	panic("unexpected FindActiveByAccountID")
}
func (s stubAuthLoginSchoolRepo) Update(context.Context, *platformModels.School) error {
	panic("unexpected Update")
}
func (s stubAuthLoginSchoolRepo) CountByIDs(context.Context, []int64) (int, error) {
	panic("unexpected CountByIDs")
}

type stubAuthLoginAccountTenantRepo struct {
	findActiveFn func(context.Context, int64) ([]authModels.AccountTenant, error)
	existsFn     func(context.Context, int64, int64) (bool, error)
}

func (s stubAuthLoginAccountTenantRepo) Create(context.Context, *authModels.AccountTenant) error {
	panic("unexpected Create")
}
func (s stubAuthLoginAccountTenantRepo) FindActiveByAccountID(ctx context.Context, accountID int64) ([]authModels.AccountTenant, error) {
	return s.findActiveFn(ctx, accountID)
}
func (s stubAuthLoginAccountTenantRepo) ExistsByAccountAndTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	return s.existsFn(ctx, accountID, tenantID)
}
func (s stubAuthLoginAccountTenantRepo) ListAccountsByTenantID(context.Context, int64) ([]authModels.TenantAccountInfo, error) {
	panic("unexpected ListAccountsByTenantID")
}
func (s stubAuthLoginAccountTenantRepo) ListAccountsByOrganizationID(context.Context, int64) ([]authModels.OrgAccountInfo, error) {
	panic("unexpected ListAccountsByOrganizationID")
}
func (s stubAuthLoginAccountTenantRepo) ListAllAccounts(context.Context) ([]authModels.OrgAccountInfo, error) {
	panic("unexpected ListAllAccounts")
}

func setupInternalAuthService(t *testing.T, db *bun.DB) *Service {
	t.Helper()
	repoFactory := repositories.NewFactory(db)
	cfg, err := NewServiceConfig(nil, email.Email{}, "http://localhost:3000", time.Hour)
	require.NoError(t, err)
	service, err := NewService(repoFactory, cfg, db, slog.Default())
	require.NoError(t, err)
	return service
}

func TestResolveAccountTenantBySlug_ReturnsTenantNotFoundWhenSchoolLookupReturnsNil(t *testing.T) {
	service := &Service{
		repos: &repositories.Factory{
			School: stubAuthLoginSchoolRepo{
				findBySubdomainFn: func(context.Context, string) (*platformModels.School, error) { return nil, nil },
			},
			AccountTenant: stubAuthLoginAccountTenantRepo{
				existsFn: func(context.Context, int64, int64) (bool, error) { return false, nil },
			},
		},
		logger: slog.Default(),
	}

	tenantID, orgID, err := service.resolveAccountTenantBySlug(context.Background(), 5, "missing-school")
	require.Error(t, err)
	assert.Zero(t, tenantID)
	assert.Zero(t, orgID)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, ErrTenantNotFound)
}

func TestResolveAccountTenantDefault_ReturnsTenantWithoutOrgWhenSchoolLookupReturnsNil(t *testing.T) {
	service := &Service{
		repos: &repositories.Factory{
			AccountTenant: stubAuthLoginAccountTenantRepo{
				findActiveFn: func(context.Context, int64) ([]authModels.AccountTenant, error) {
					return []authModels.AccountTenant{{TenantID: 99, Status: authModels.AccountTenantStatusActive}}, nil
				},
			},
			School: stubAuthLoginSchoolRepo{
				findByIDFn: func(context.Context, int64) (*platformModels.School, error) { return nil, nil },
			},
		},
		logger: slog.Default(),
	}

	tenantID, orgID, err := service.resolveAccountTenantDefault(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, int64(99), tenantID)
	assert.Zero(t, orgID)
}

func TestPersistAccountWithRole_CreatesTenantMappingWhenTenantIDProvided(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupInternalAuthService(t, db)
	ctx := testpkg.TenantContext(1)
	email := "persist-role-" + time.Now().Format("150405.000000000") + "@test.local"
	passwordHash := "$argon2id$v=19$m=65536,t=3,p=4$persist"
	account := &authModels.Account{
		Model:        modelBase.Model{},
		Email:        email,
		PasswordHash: &passwordHash,
		Active:       true,
	}
	role := testpkg.CreateTestRole(t, db, "PersistRole")
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_roles WHERE account_id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.roles WHERE id = ?`, role.ID)
	})

	err := service.persistAccountWithRole(ctx, account, &role.ID, 1)
	require.NoError(t, err)
	assert.NotZero(t, account.ID)

	exists, err := service.repos.AccountTenant.ExistsByAccountAndTenant(ctx, account.ID, 1)
	require.NoError(t, err)
	assert.True(t, exists)
}
