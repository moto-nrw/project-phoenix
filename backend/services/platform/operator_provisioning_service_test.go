package platform_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/auth/authtest"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type mockOrganizationRepo struct {
	findByIDFn   func(context.Context, int64) (*platformModels.Organization, error)
	findBySlugFn func(context.Context, string) (*platformModels.Organization, error)
	createFn     func(context.Context, *platformModels.Organization) error
	updateFn     func(context.Context, *platformModels.Organization) error
	listFn       func(context.Context) ([]*platformModels.Organization, error)
	softDeleteFn func(context.Context, int64) error
	restoreFn    func(context.Context, int64) error
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
func (m *mockOrganizationRepo) FindByIDForShare(ctx context.Context, id int64) (*platformModels.Organization, error) {
	return m.FindByID(ctx, id)
}
func (m *mockOrganizationRepo) FindByIDForUpdate(ctx context.Context, id int64) (*platformModels.Organization, error) {
	return m.FindByID(ctx, id)
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
func (m *mockOrganizationRepo) CountByIDs(ctx context.Context, ids []int64) (int, error) {
	return len(ids), nil
}
func (m *mockOrganizationRepo) SoftDelete(ctx context.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}
func (m *mockOrganizationRepo) Restore(ctx context.Context, id int64) error {
	if m.restoreFn != nil {
		return m.restoreFn(ctx, id)
	}
	return nil
}

type mockDeviceRepo struct {
	createFn            func(context.Context, *iotModels.Device) error
	findByIDForUpdateFn func(context.Context, int64) (*iotModels.Device, error)
	updateFn            func(context.Context, *iotModels.Device) error
}

type mockActiveDeviceSessionRepo struct {
	findFn func(context.Context, int64) (*activeModels.Group, error)
}

func (m *mockActiveDeviceSessionRepo) FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*activeModels.Group, error) {
	if m.findFn != nil {
		return m.findFn(ctx, deviceID)
	}
	return nil, nil
}

func (m *mockDeviceRepo) Create(ctx context.Context, device *iotModels.Device) error {
	if m.createFn != nil {
		return m.createFn(ctx, device)
	}
	return nil
}
func (m *mockDeviceRepo) FindByID(context.Context, interface{}) (*iotModels.Device, error) {
	return nil, nil
}
func (m *mockDeviceRepo) FindByIDForUpdate(ctx context.Context, id int64) (*iotModels.Device, error) {
	if m.findByIDForUpdateFn != nil {
		return m.findByIDForUpdateFn(ctx, id)
	}
	return m.FindByID(ctx, id)
}
func (m *mockDeviceRepo) Update(ctx context.Context, device *iotModels.Device) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, device)
	}
	return nil
}
func (m *mockDeviceRepo) Delete(context.Context, interface{}) error { return nil }
func (m *mockDeviceRepo) List(context.Context, map[string]interface{}) ([]*iotModels.Device, error) {
	return nil, nil
}
func (m *mockDeviceRepo) FindByDeviceID(context.Context, string) (*iotModels.Device, error) {
	return nil, nil
}
func (m *mockDeviceRepo) FindByAPIKey(context.Context, string) (*iotModels.Device, error) {
	return nil, nil
}
func (m *mockDeviceRepo) FindByType(context.Context, string) ([]*iotModels.Device, error) {
	return nil, nil
}
func (m *mockDeviceRepo) FindByStatus(context.Context, iotModels.DeviceStatus) ([]*iotModels.Device, error) {
	return nil, nil
}
func (m *mockDeviceRepo) FindByRegisteredBy(context.Context, int64) ([]*iotModels.Device, error) {
	return nil, nil
}
func (m *mockDeviceRepo) UpdateLastSeen(context.Context, int64, time.Time) error { return nil }
func (m *mockDeviceRepo) UpdateRoomID(context.Context, int64, int64) error       { return nil }
func (m *mockDeviceRepo) UpdateStatus(context.Context, string, iotModels.DeviceStatus) error {
	return nil
}
func (m *mockDeviceRepo) FindOfflineDevices(context.Context, time.Duration) ([]*iotModels.Device, error) {
	return nil, nil
}
func (m *mockDeviceRepo) CountDevicesByType(context.Context) (map[string]int, error) {
	return nil, nil
}

type mockRoleRepo struct {
	role       *authModels.Role
	roles      []*authModels.Role
	findByIDFn func(context.Context, interface{}) (*authModels.Role, error)
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
func (m *mockCategoryRepo) FindByIDForShare(context.Context, int64) (*activityModels.Category, error) {
	return nil, nil
}
func (m *mockCategoryRepo) Update(context.Context, *activityModels.Category) error { return nil }
func (m *mockCategoryRepo) UpdateIfActive(context.Context, *activityModels.Category) (bool, error) {
	return true, nil
}
func (m *mockCategoryRepo) Delete(context.Context, interface{}) error { return nil }
func (m *mockCategoryRepo) List(context.Context, *base.QueryOptions) ([]*activityModels.Category, error) {
	return nil, nil
}
func (m *mockCategoryRepo) FindByName(context.Context, string) (*activityModels.Category, error) {
	return nil, nil
}
func (m *mockCategoryRepo) FindByNameIncludingArchivedForShare(context.Context, string) (*activityModels.Category, error) {
	return nil, nil
}
func (m *mockCategoryRepo) ListAll(context.Context) ([]*activityModels.Category, error) {
	return nil, nil
}

func (m *mockCategoryRepo) SetShiftTypeForCategories(context.Context, int64, []int64) error {
	return nil
}

func (m *mockCategoryRepo) UpdateColumns(context.Context, *activityModels.Category, ...string) (int64, error) {
	return 1, nil
}

func (m *mockRoleRepo) Create(context.Context, *authModels.Role) error { return nil }
func (m *mockRoleRepo) FindByID(ctx context.Context, id interface{}) (*authModels.Role, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
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
func (m *mockRoleRepo) FindRoleNamesByAccountIDs(context.Context, []int64) (map[int64]string, error) {
	return nil, nil
}
func (m *mockRoleRepo) AssignRoleToAccount(context.Context, int64, int64) error { return nil }
func (m *mockRoleRepo) RemoveRoleFromAccount(context.Context, int64, int64) error {
	return nil
}
func (m *mockRoleRepo) GetRoleWithPermissions(context.Context, int64) (*authModels.Role, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// mockAuthService
// ---------------------------------------------------------------------------

type mockAuthService struct {
	registerFn func(ctx context.Context, email, username, password string, roleID *int64, tenantID int64) (*authModels.Account, error)
}

func (m *mockAuthService) VerifyAccountTenantMembership(_ context.Context, _, _ int64) (bool, error) {
	return true, nil
}

func (m *mockAuthService) Login(context.Context, string, string) (string, string, error) {
	return "", "", nil
}
func (m *mockAuthService) LoginWithAudit(context.Context, string, string, string, string, string) (string, string, error) {
	return "", "", nil
}

// No-op stubs for the MFA-related additions (issue #1308 phases 5 + 7a).
// The provisioning tests don't exercise these paths; the methods exist
// solely so *mockAuthService still satisfies the AuthService interface.
func (m *mockAuthService) IssueTokensForAuthenticatedAccount(context.Context, int64, int64, string, string) (string, string, error) {
	return "", "", nil
}
func (m *mockAuthService) LoginWithMFAGate(context.Context, string, string, string, string, string, string) (*authSvc.LoginResult, error) {
	return nil, nil
}
func (m *mockAuthService) SetMFAService(authSvc.MFAService) {}
func (m *mockAuthService) LoginParent(context.Context, string, string) (string, string, error) {
	return "", "", nil
}
func (m *mockAuthService) LoginParentWithAudit(context.Context, string, string, string, string) (string, string, error) {
	return "", "", nil
}
func (m *mockAuthService) Register(ctx context.Context, email, username, password string, roleID *int64, tenantID int64) (*authModels.Account, error) {
	if m.registerFn != nil {
		return m.registerFn(ctx, email, username, password, roleID, tenantID)
	}
	return nil, nil
}
func (m *mockAuthService) ValidateToken(context.Context, string) (*authModels.Account, *jwt.AppClaims, error) {
	return nil, nil, nil
}
func (m *mockAuthService) RefreshToken(context.Context, string) (string, string, error) {
	return "", "", nil
}
func (m *mockAuthService) RefreshTokenWithAudit(context.Context, string, string, string) (string, string, error) {
	return "", "", nil
}
func (m *mockAuthService) Logout(context.Context, string) error                          { return nil }
func (m *mockAuthService) LogoutWithAudit(context.Context, string, string, string) error { return nil }
func (m *mockAuthService) ChangePassword(context.Context, int, string, string) error     { return nil }
func (m *mockAuthService) GetAccountByID(context.Context, int) (*authModels.Account, error) {
	return nil, nil
}
func (m *mockAuthService) GetAccountByEmail(context.Context, string) (*authModels.Account, error) {
	return nil, nil
}
func (m *mockAuthService) CreateRole(context.Context, string, string, *string) (*authModels.Role, error) {
	return nil, nil
}
func (m *mockAuthService) GetRoleByID(context.Context, int) (*authModels.Role, error) {
	return nil, nil
}
func (m *mockAuthService) GetRoleByName(context.Context, string) (*authModels.Role, error) {
	return nil, nil
}
func (m *mockAuthService) ResolveAssignableSchoolRole(context.Context, int64, int64) (*authModels.Role, error) {
	return nil, nil
}
func (m *mockAuthService) UpdateRole(context.Context, *authModels.Role) error { return nil }
func (m *mockAuthService) DeleteRole(context.Context, int) error              { return nil }
func (m *mockAuthService) ListRoles(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
	return nil, nil
}
func (m *mockAuthService) AssignRoleToAccount(context.Context, int, int) error   { return nil }
func (m *mockAuthService) RemoveRoleFromAccount(context.Context, int, int) error { return nil }
func (m *mockAuthService) GetAccountRoles(context.Context, int) ([]*authModels.Role, error) {
	return nil, nil
}
func (m *mockAuthService) GetAccountRoleNames(context.Context, []int64) (map[int64]string, error) {
	return nil, nil
}
func (m *mockAuthService) GetAccountEmailsByIDs(context.Context, []int64) (map[int64]string, error) {
	return nil, nil
}
func (m *mockAuthService) GetAccountAvatarsByIDs(context.Context, []int64) (map[int64]string, error) {
	return nil, nil
}
func (m *mockAuthService) CreatePermission(context.Context, string, string, string, string) (*authModels.Permission, error) {
	return nil, nil
}
func (m *mockAuthService) GetPermissionByID(context.Context, int) (*authModels.Permission, error) {
	return nil, nil
}
func (m *mockAuthService) GetPermissionByName(context.Context, string) (*authModels.Permission, error) {
	return nil, nil
}
func (m *mockAuthService) UpdatePermission(context.Context, *authModels.Permission) error { return nil }
func (m *mockAuthService) DeletePermission(context.Context, int) error                    { return nil }
func (m *mockAuthService) ListPermissions(context.Context, map[string]interface{}) ([]*authModels.Permission, error) {
	return nil, nil
}
func (m *mockAuthService) GrantPermissionToAccount(context.Context, int, int) error    { return nil }
func (m *mockAuthService) DenyPermissionToAccount(context.Context, int, int) error     { return nil }
func (m *mockAuthService) RemovePermissionFromAccount(context.Context, int, int) error { return nil }
func (m *mockAuthService) GetAccountPermissions(context.Context, int) ([]*authModels.Permission, error) {
	return nil, nil
}
func (m *mockAuthService) GetAccountDirectPermissions(context.Context, int) ([]*authModels.Permission, error) {
	return nil, nil
}
func (m *mockAuthService) AssignPermissionToRole(context.Context, int, int) error   { return nil }
func (m *mockAuthService) RemovePermissionFromRole(context.Context, int, int) error { return nil }
func (m *mockAuthService) GetRolePermissions(context.Context, int) ([]*authModels.Permission, error) {
	return nil, nil
}
func (m *mockAuthService) ActivateAccount(context.Context, int) error               { return nil }
func (m *mockAuthService) DeactivateAccount(context.Context, int) error             { return nil }
func (m *mockAuthService) UpdateAccount(context.Context, *authModels.Account) error { return nil }
func (m *mockAuthService) IsPINLocked(*authModels.Account, time.Time) bool          { return false }
func (m *mockAuthService) RecordFailedPINAttempt(context.Context, int64) error      { return nil }
func (m *mockAuthService) ResetPINLockout(context.Context, int64) error             { return nil }
func (m *mockAuthService) ListAccounts(context.Context, map[string]interface{}) ([]*authModels.Account, error) {
	return nil, nil
}
func (m *mockAuthService) GetAccountsByRole(context.Context, string) ([]*authModels.Account, error) {
	return nil, nil
}
func (m *mockAuthService) GetAccountsWithRolesAndPermissions(context.Context, map[string]interface{}) ([]*authModels.Account, error) {
	return nil, nil
}
func (m *mockAuthService) InitiatePasswordReset(context.Context, string) (*authModels.PasswordResetToken, error) {
	return nil, nil
}
func (m *mockAuthService) InitiateParentPasswordReset(context.Context, string) (*authModels.PasswordResetToken, error) {
	return nil, nil
}
func (m *mockAuthService) ResetPassword(context.Context, string, string) error { return nil }
func (m *mockAuthService) CleanupExpiredRateLimits(context.Context) (int, error) {
	return 0, nil
}
func (m *mockAuthService) CleanupExpiredTokens(context.Context) (int, error) { return 0, nil }
func (m *mockAuthService) CleanupExpiredPasswordResetTokens(context.Context) (int, error) {
	return 0, nil
}
func (m *mockAuthService) RevokeAllTokens(context.Context, int) error                 { return nil }
func (m *mockAuthService) RevokeTokensByTenantID(context.Context, int64) (int, error) { return 0, nil }
func (m *mockAuthService) GetActiveTokens(context.Context, int) ([]*authModels.Token, error) {
	return nil, nil
}
func (m *mockAuthService) SwitchTenant(context.Context, int64, string) (string, string, error) {
	return "", "", nil
}
func (m *mockAuthService) LinkAccountToTenant(context.Context, string, *int64, int64) (*authModels.Account, error) {
	return nil, nil
}
func (m *mockAuthService) CreateParentAccount(context.Context, string, string, string) (*authModels.AccountParent, error) {
	return nil, nil
}
func (m *mockAuthService) GetParentAccountByID(context.Context, int) (*authModels.AccountParent, error) {
	return nil, nil
}
func (m *mockAuthService) GetParentAccountByEmail(context.Context, string) (*authModels.AccountParent, error) {
	return nil, nil
}
func (m *mockAuthService) UpdateParentAccount(context.Context, *authModels.AccountParent) error {
	return nil
}
func (m *mockAuthService) ActivateParentAccount(context.Context, int) error   { return nil }
func (m *mockAuthService) DeactivateParentAccount(context.Context, int) error { return nil }
func (m *mockAuthService) ListParentAccounts(context.Context, map[string]interface{}) ([]*authModels.AccountParent, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// mockPersonRepo
// ---------------------------------------------------------------------------

type mockPersonRepo struct {
	createFn        func(context.Context, *userModels.Person) error
	linkToAccountFn func(context.Context, int64, int64) error
}

func (m *mockPersonRepo) Create(ctx context.Context, person *userModels.Person) error {
	if m.createFn != nil {
		return m.createFn(ctx, person)
	}
	return nil
}
func (m *mockPersonRepo) FindByID(context.Context, interface{}) (*userModels.Person, error) {
	return nil, nil
}
func (m *mockPersonRepo) FindByIDForUpdate(context.Context, int64) (*userModels.Person, error) {
	return nil, nil
}
func (m *mockPersonRepo) FindByIDs(context.Context, []int64) (map[int64]*userModels.Person, error) {
	return nil, nil
}
func (m *mockPersonRepo) FindByTagID(context.Context, string) (*userModels.Person, error) {
	return nil, nil
}
func (m *mockPersonRepo) FindByAccountID(context.Context, int64) (*userModels.Person, error) {
	return nil, nil
}
func (m *mockPersonRepo) Update(context.Context, *userModels.Person) error { return nil }
func (m *mockPersonRepo) Delete(context.Context, interface{}) error        { return nil }
func (m *mockPersonRepo) List(context.Context, map[string]interface{}) ([]*userModels.Person, error) {
	return nil, nil
}
func (m *mockPersonRepo) ListWithOptions(context.Context, *base.QueryOptions) ([]*userModels.Person, error) {
	return nil, nil
}
func (m *mockPersonRepo) LinkToAccount(ctx context.Context, personID int64, accountID int64) error {
	if m.linkToAccountFn != nil {
		return m.linkToAccountFn(ctx, personID, accountID)
	}
	return nil
}
func (m *mockPersonRepo) UnlinkFromAccount(context.Context, int64) error      { return nil }
func (m *mockPersonRepo) LinkToRFIDCard(context.Context, int64, string) error { return nil }
func (m *mockPersonRepo) UnlinkFromRFIDCard(context.Context, int64) error     { return nil }
func (m *mockPersonRepo) FindWithAccount(context.Context, int64) (*userModels.Person, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// mockTeacherRepo
// ---------------------------------------------------------------------------

type mockTeacherRepo struct {
	createFn func(context.Context, *userModels.Teacher) error
}

func (m *mockTeacherRepo) Create(ctx context.Context, teacher *userModels.Teacher) error {
	if m.createFn != nil {
		return m.createFn(ctx, teacher)
	}
	return nil
}
func (m *mockTeacherRepo) FindByID(context.Context, interface{}) (*userModels.Teacher, error) {
	return nil, nil
}
func (m *mockTeacherRepo) FindByStaffID(context.Context, int64) (*userModels.Teacher, error) {
	return nil, nil
}
func (m *mockTeacherRepo) FindByStaffIDs(context.Context, []int64) (map[int64]*userModels.Teacher, error) {
	return nil, nil
}
func (m *mockTeacherRepo) FindBySpecialization(context.Context, string) ([]*userModels.Teacher, error) {
	return nil, nil
}
func (m *mockTeacherRepo) Update(context.Context, *userModels.Teacher) error { return nil }
func (m *mockTeacherRepo) Delete(context.Context, interface{}) error         { return nil }
func (m *mockTeacherRepo) List(context.Context, map[string]interface{}) ([]*userModels.Teacher, error) {
	return nil, nil
}
func (m *mockTeacherRepo) ListWithOptions(context.Context, *base.QueryOptions) ([]*userModels.Teacher, error) {
	return nil, nil
}
func (m *mockTeacherRepo) FindByGroupID(context.Context, int64) ([]*userModels.Teacher, error) {
	return nil, nil
}
func (m *mockTeacherRepo) UpdateQualifications(context.Context, int64, string) error { return nil }
func (m *mockTeacherRepo) FindWithStaffAndPerson(context.Context, int64) (*userModels.Teacher, error) {
	return nil, nil
}
func (m *mockTeacherRepo) ListAllWithStaffAndPerson(context.Context) ([]*userModels.Teacher, error) {
	return nil, nil
}
func (m *mockTeacherRepo) FindWithStaffAndPersonByIDs(context.Context, []int64) ([]*userModels.Teacher, error) {
	return nil, nil
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
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: base.Model{ID: 2}, Name: "Org B", Slug: "org-b", Active: true}, nil
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByOrganizationAndSlugFn: func(context.Context, int64, string) (*platformModels.School, error) {
				return nil, nil
			},
			FindBySubdomainFn: func(context.Context, string) (*platformModels.School, error) {
				return nil, nil
			},
			CreateFn: func(_ context.Context, school *platformModels.School) error {
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
		SummariesRepo: &mockSummariesRepo{},
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
		SummariesRepo: &mockSummariesRepo{},
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
		SummariesRepo: &mockSummariesRepo{},
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
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			ListFn: func(context.Context) ([]*platformModels.School, error) {
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

	var capturedReq authSvc.InvitationRequest
	invitations := &authtest.InvitationServiceMock{
		CreateInvitationFn: func(_ context.Context, req authSvc.InvitationRequest) (*authModels.InvitationToken, error) {
			capturedReq = req
			return &authModels.InvitationToken{
				Model:            base.Model{ID: 10},
				Email:            req.Email,
				RoleID:           req.RoleID,
				CreatedBy:        nil,
				ExpiresAt:        time.Now().Add(24 * time.Hour),
				FirstName:        req.FirstName,
				LastName:         req.LastName,
				Position:         req.Position,
				CaregiverEnabled: req.CaregiverEnabled,
			}, nil
		},
	}
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
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
		Email:            "principal@example.com",
		CaregiverEnabled: true,
	})
	require.NoError(t, err)
	require.NotNil(t, invitation)
	require.Equal(t, int64(0), capturedReq.CreatedBy)
	require.True(t, capturedReq.CaregiverEnabled)
}

func TestOperatorProvisioningService_CreateSchool_OrganizationNotFound(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo:    &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{},
		SchoolRepo:       &testpkg.SchoolRepoMock{},
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

func TestOperatorProvisioningService_CreateSchool_OrganizationDeleted(t *testing.T) {
	now := time.Now()
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{
					Model: base.Model{ID: id}, Name: "Org", Slug: "org", Active: true,
					DeletedAt: &now,
				}, nil
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{},
	})

	school, err := service.CreateSchool(context.Background(), &platformModels.School{
		OrganizationID: 7,
		Name:           "New School",
		Slug:           "new-school",
		Subdomain:      "new-school",
		Active:         true,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	require.Error(t, err)
	var orgDeleted *platformSvc.OrganizationDeletedError
	require.ErrorAs(t, err, &orgDeleted)
}

func TestOperatorProvisioningService_CreateSchool_SlugConflict(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: base.Model{ID: 2}, Name: "Org", Slug: "org", Active: true}, nil
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByOrganizationAndSlugFn: func(context.Context, int64, string) (*platformModels.School, error) {
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
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: base.Model{ID: 2}, Name: "Org", Slug: "org", Active: true}, nil
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByOrganizationAndSlugFn: func(context.Context, int64, string) (*platformModels.School, error) {
				return nil, nil
			},
			FindBySubdomainFn: func(context.Context, string) (*platformModels.School, error) {
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

// --- ListSchoolAccounts tests ---

func TestOperatorProvisioningService_ListSchoolAccounts_Success(t *testing.T) {
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

	schoolID := int64(9) // nolint: mock school ID for assertion
	expected := []authModels.TenantAccountInfo{
		{AccountID: 1, Email: "admin@example.com", Active: true, FirstName: "Admin", LastName: "User", RoleName: "admin", Status: "active"},
	}
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{Model: base.Model{ID: schoolID}, OrganizationID: 1, Name: "School", Slug: "school", Subdomain: "school", Active: true}, nil
			},
		},
		AccountTenantRepo: &mockAccountTenantRepo{
			listAccountsByTenantIDFn: func(_ context.Context, tenantID int64) ([]authModels.TenantAccountInfo, error) {
				assert.Equal(t, schoolID, tenantID)
				return expected, nil
			},
		},
		DB: bunDB,
	})

	accounts, err := service.ListSchoolAccounts(context.Background(), schoolID)
	require.NoError(t, err)
	require.Equal(t, expected, accounts)
}

func TestOperatorProvisioningService_ListSchoolAccounts_SchoolNotFound_NilReturn(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo:    &testpkg.SchoolRepoMock{},
	})

	accounts, err := service.ListSchoolAccounts(context.Background(), 999)
	require.Nil(t, accounts)
	require.Error(t, err)
	var notFoundErr *platformSvc.SchoolNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestOperatorProvisioningService_ListSchoolAccounts_SchoolNotFound_SqlErrNoRows(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return nil, sql.ErrNoRows
			},
		},
	})

	accounts, err := service.ListSchoolAccounts(context.Background(), 999)
	require.Nil(t, accounts)
	require.Error(t, err)
	var notFoundErr *platformSvc.SchoolNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

// --- ListOrganizationAccounts tests ---

func TestOperatorProvisioningService_ListOrganizationAccounts_Success(t *testing.T) {
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

	orgID := int64(5) // nolint: mock org ID for assertion
	expected := []authModels.OrgAccountInfo{
		{
			TenantAccountInfo: authModels.TenantAccountInfo{AccountID: 1, Email: "admin@example.com", Active: true, RoleName: "admin", Status: "active"},
			SchoolID:          9,
			SchoolName:        "School A",
		},
	}
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: base.Model{ID: orgID}, Name: "Org", Slug: "org", Active: true}, nil
			},
		},
		AccountTenantRepo: &mockAccountTenantRepo{
			listAccountsByOrganizationFn: func(_ context.Context, receivedOrgID int64) ([]authModels.OrgAccountInfo, error) {
				assert.Equal(t, orgID, receivedOrgID)
				return expected, nil
			},
		},
		DB: bunDB,
	})

	accounts, err := service.ListOrganizationAccounts(context.Background(), orgID)
	require.NoError(t, err)
	require.Equal(t, expected, accounts)
}

func TestOperatorProvisioningService_ListOrganizationAccounts_OrgNotFound(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo:    &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{},
	})

	accounts, err := service.ListOrganizationAccounts(context.Background(), 999)
	require.Nil(t, accounts)
	require.Error(t, err)
	var notFoundErr *platformSvc.OrganizationNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

// --- ListAllAccounts tests ---

func TestOperatorProvisioningService_ListAllAccounts_Success(t *testing.T) {
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

	expected := []authModels.OrgAccountInfo{
		{
			TenantAccountInfo: authModels.TenantAccountInfo{AccountID: 1, Email: "admin@example.com", Active: true},
			SchoolID:          9,
			SchoolName:        "School A",
		},
		{
			TenantAccountInfo: authModels.TenantAccountInfo{AccountID: 2, Email: "teacher@example.com", Active: true},
			SchoolID:          10,
			SchoolName:        "School B",
		},
	}
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		AccountTenantRepo: &mockAccountTenantRepo{
			listAllAccountsFn: func(_ context.Context) ([]authModels.OrgAccountInfo, error) {
				return expected, nil
			},
		},
		DB: bunDB,
	})

	accounts, err := service.ListAllAccounts(context.Background())
	require.NoError(t, err)
	require.Equal(t, expected, accounts)
}

// --- ListSchoolDevices error tests ---

func TestOperatorProvisioningService_ListSchoolDevices_SchoolNotFound_NilReturn(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo:    &testpkg.SchoolRepoMock{},
	})

	devices, err := service.ListSchoolDevices(context.Background(), 999)
	require.Nil(t, devices)
	require.Error(t, err)
	var notFoundErr *platformSvc.SchoolNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestOperatorProvisioningService_ListSchoolDevices_SchoolNotFound_SqlErrNoRows(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return nil, sql.ErrNoRows
			},
		},
	})

	devices, err := service.ListSchoolDevices(context.Background(), 999)
	require.Nil(t, devices)
	require.Error(t, err)
	var notFoundErr *platformSvc.SchoolNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

// --- ListOrganizationDevices error tests ---

func TestOperatorProvisioningService_ListOrganizationDevices_OrgNotFound(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo:    &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{},
	})

	devices, err := service.ListOrganizationDevices(context.Background(), 999)
	require.Nil(t, devices)
	require.Error(t, err)
	var notFoundErr *platformSvc.OrganizationNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

// --- CreateOrganization unique violation on create (race condition) ---

func TestOperatorProvisioningService_CreateOrganization_UniqueViolationOnCreate(t *testing.T) {
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
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findBySlugFn: func(context.Context, string) (*platformModels.Organization, error) {
				return nil, nil // slug appears available
			},
			createFn: func(_ context.Context, org *platformModels.Organization) error {
				// Race condition: another request created it between FindBySlug and Create
				return assert.AnError // non-unique violation error for this test path
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	org, err := service.CreateOrganization(context.Background(), &platformModels.Organization{
		Name:   "Race Org",
		Slug:   "race-org",
		Active: true,
	}, 7, net.IPv4(127, 0, 0, 1))
	require.Nil(t, org)
	require.Error(t, err)
}

// --- CreateSchool nil input and validation error ---

func TestOperatorProvisioningService_CreateSchool_NilInput(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
	})

	school, err := service.CreateSchool(context.Background(), nil, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	require.Error(t, err)
	var invalidErr *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalidErr)
}

func TestOperatorProvisioningService_CreateSchool_ValidationError(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
	})

	school, err := service.CreateSchool(context.Background(), &platformModels.School{
		OrganizationID: 1,
		Name:           "", // empty name fails validation
		Slug:           "test",
		Subdomain:      "test",
		Active:         true,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	require.Error(t, err)
	var invalidErr *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalidErr)
}

func TestOperatorProvisioningService_CreateSchool_DeviceCreateError(t *testing.T) {
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
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: base.Model{ID: 2}, Name: "Org", Slug: "org", Active: true}, nil
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByOrganizationAndSlugFn: func(context.Context, int64, string) (*platformModels.School, error) {
				return nil, nil
			},
			FindBySubdomainFn: func(context.Context, string) (*platformModels.School, error) {
				return nil, nil
			},
			CreateFn: func(_ context.Context, school *platformModels.School) error {
				school.ID = 55
				return nil
			},
		},
		CategoryRepo: &mockCategoryRepo{},
		DeviceRepo: &mockDeviceRepo{
			createFn: func(context.Context, *iotModels.Device) error {
				return assert.AnError
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	school, err := service.CreateSchool(context.Background(), &platformModels.School{
		OrganizationID: 2,
		Name:           "Device Error School",
		Slug:           "device-error-school",
		Subdomain:      "device-error-school",
		Active:         true,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create web manual device")
}

func TestOperatorProvisioningService_CreateSchool_CategorySeedError(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: base.Model{ID: 2}, Name: "Org", Slug: "org", Active: true}, nil
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByOrganizationAndSlugFn: func(context.Context, int64, string) (*platformModels.School, error) {
				return nil, nil
			},
			FindBySubdomainFn: func(context.Context, string) (*platformModels.School, error) {
				return nil, nil
			},
			CreateFn: func(_ context.Context, school *platformModels.School) error {
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
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo:    &testpkg.SchoolRepoMock{},
		RoleRepo:      &mockRoleRepo{role: &authModels.Role{Model: base.Model{ID: 4}, Name: "admin", IsSystem: true}},
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
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
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
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
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
		SummariesRepo: &mockSummariesRepo{},
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
		SummariesRepo: &mockSummariesRepo{},
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

func TestOperatorProvisioningService_UpdateOrganization_AlreadyDeleted(t *testing.T) {
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

	deletedAt := time.Now()
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{
					Model:     base.Model{ID: 10},
					Name:      "Tombstoned",
					Slug:      "tombstoned",
					Active:    true,
					DeletedAt: &deletedAt,
				}, nil
			},
			updateFn: func(context.Context, *platformModels.Organization) error {
				t.Fatal("Update should not be called on a deleted organization")
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
	require.Nil(t, org)
	require.Error(t, err)
	var alreadyDeleted *platformSvc.OrganizationAlreadyDeletedError
	require.ErrorAs(t, err, &alreadyDeleted)
	assert.Equal(t, int64(10), alreadyDeleted.OrganizationID)
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
		SummariesRepo: &mockSummariesRepo{},
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
		SummariesRepo: &mockSummariesRepo{},
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
		SummariesRepo:    &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: 50},
					OrganizationID: 2,
					Name:           "Old School",
					Slug:           "old-school",
					Subdomain:      "old-school",
					Active:         true,
				}, nil
			},
			FindByOrganizationAndSlugFn: func(_ context.Context, orgID int64, slug string) (*platformModels.School, error) {
				return nil, nil // no conflict
			},
			FindBySubdomainFn: func(_ context.Context, subdomain string) (*platformModels.School, error) {
				return nil, nil // no conflict
			},
			UpdateFn: func(_ context.Context, school *platformModels.School) error {
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
		SummariesRepo:    &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
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
		SummariesRepo:    &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: 50},
					OrganizationID: 2,
					Name:           "My School",
					Slug:           "my-school",
					Subdomain:      "my-school",
					Active:         true,
				}, nil
			},
			FindByOrganizationAndSlugFn: func(_ context.Context, orgID int64, slug string) (*platformModels.School, error) {
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
		SummariesRepo:    &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: 50},
					OrganizationID: 2,
					Name:           "My School",
					Slug:           "my-school",
					Subdomain:      "my-school",
					Active:         true,
				}, nil
			},
			FindByOrganizationAndSlugFn: func(_ context.Context, orgID int64, slug string) (*platformModels.School, error) {
				return nil, nil // slug is fine
			},
			FindBySubdomainFn: func(_ context.Context, subdomain string) (*platformModels.School, error) {
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
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return nil, nil // org not found
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
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

func TestOperatorProvisioningService_UpdateSchool_ChangeOrganization_Deleted(t *testing.T) {
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

	now := time.Now()
	newOrgID := int64(5)
	updateCalled := false
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				if id == newOrgID {
					return &platformModels.Organization{
						Model: base.Model{ID: newOrgID}, Name: "New Org", Slug: "new-org", Active: true,
						DeletedAt: &now,
					}, nil
				}
				return nil, nil
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: 50},
					OrganizationID: 2,
					Name:           "My School",
					Slug:           "my-school",
					Subdomain:      "my-school",
					Active:         true,
				}, nil
			},
			UpdateFn: func(_ context.Context, _ *platformModels.School) error {
				updateCalled = true
				return nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	school, err := service.UpdateSchool(context.Background(), 50, platformSvc.UpdateSchoolRequest{
		OrganizationID: newOrgID,
		Name:           "My School",
		Slug:           "my-school",
		Subdomain:      "my-school",
		Active:         true,
	}, 7, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	require.Error(t, err)
	var orgDeleted *platformSvc.OrganizationDeletedError
	require.ErrorAs(t, err, &orgDeleted)
	assert.False(t, updateCalled, "must not update school into deleted org")
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
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				if id == newOrgID {
					return &platformModels.Organization{Model: base.Model{ID: newOrgID}, Name: "New Org", Slug: "new-org", Active: true}, nil
				}
				return nil, nil
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: 50},
					OrganizationID: 2,
					Name:           "My School",
					Slug:           "my-school",
					Subdomain:      "my-school",
					Active:         true,
				}, nil
			},
			FindByOrganizationAndSlugFn: func(_ context.Context, orgID int64, slug string) (*platformModels.School, error) {
				return nil, nil // no slug conflict in new org
			},
			FindBySubdomainFn: func(_ context.Context, subdomain string) (*platformModels.School, error) {
				return nil, nil
			},
			UpdateFn: func(_ context.Context, school *platformModels.School) error {
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

func TestOperatorProvisioningService_UpdateOrganization_UpdateError(t *testing.T) {
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
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: base.Model{ID: 10}, Name: "Old Name", Slug: "old-slug", Active: true}, nil
			},
			findBySlugFn: func(_ context.Context, slug string) (*platformModels.Organization, error) {
				return nil, nil // no slug conflict
			},
			updateFn: func(_ context.Context, org *platformModels.Organization) error {
				return assert.AnError // generic (non-unique-violation) error
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	org, err := service.UpdateOrganization(context.Background(), 10, platformSvc.UpdateOrganizationRequest{
		Name:   "New Name",
		Slug:   "new-slug",
		Active: true,
	}, 7, net.IPv4(127, 0, 0, 1))
	require.Nil(t, org)
	require.ErrorIs(t, err, assert.AnError)
}

func TestOperatorProvisioningService_UpdateSchool_UpdateError(t *testing.T) {
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
		SummariesRepo:    &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: 50},
					OrganizationID: 2,
					Name:           "My School",
					Slug:           "my-school",
					Subdomain:      "my-school",
					Active:         true,
				}, nil
			},
			FindByOrganizationAndSlugFn: func(_ context.Context, orgID int64, slug string) (*platformModels.School, error) {
				return nil, nil // no slug conflict
			},
			FindBySubdomainFn: func(_ context.Context, subdomain string) (*platformModels.School, error) {
				return nil, nil // no subdomain conflict
			},
			UpdateFn: func(_ context.Context, school *platformModels.School) error {
				return assert.AnError // generic (non-unique-violation) error
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	school, err := service.UpdateSchool(context.Background(), 50, platformSvc.UpdateSchoolRequest{
		OrganizationID: 2,
		Name:           "My School",
		Slug:           "my-school",
		Subdomain:      "my-school",
		Active:         true,
	}, 7, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	require.ErrorIs(t, err, assert.AnError)
}

// ---------------------------------------------------------------------------
// CreateSchoolAccount tests
// ---------------------------------------------------------------------------

func TestCreateSchoolAccount_Success(t *testing.T) {
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

	roleID := int64(5)
	var createdStaff *userModels.Staff
	var createdTeacher *userModels.Teacher

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: 9},
					OrganizationID: 3,
					Name:           "School",
					Slug:           "school",
					Subdomain:      "school",
					Active:         true,
				}, nil
			},
		},
		RoleRepo: &mockRoleRepo{
			findByIDFn: func(_ context.Context, id interface{}) (*authModels.Role, error) {
				return &authModels.Role{
					Model:    base.Model{ID: roleID},
					Name:     "user",
					IsSystem: true,
				}, nil
			},
		},
		AuthService: &mockAuthService{
			registerFn: func(_ context.Context, email, username, password string, rID *int64, tenantID int64) (*authModels.Account, error) {
				return &authModels.Account{
					Model: base.Model{ID: 100},
					Email: email,
				}, nil
			},
		},
		PersonRepo: &mockPersonRepo{
			createFn: func(_ context.Context, person *userModels.Person) error {
				person.ID = 200
				return nil
			},
		},
		StaffRepo: &testpkg.StaffRepoMock{
			CreateFn: func(_ context.Context, staff *userModels.Staff) error {
				staff.ID = 300
				createdStaff = staff
				return nil
			},
		},
		TeacherRepo: &mockTeacherRepo{
			createFn: func(_ context.Context, teacher *userModels.Teacher) error {
				teacher.ID = 400
				createdTeacher = teacher
				return nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	account, err := service.CreateSchoolAccount(context.Background(), 9, 7, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:     "teacher@example.com",
		Password:  "SecureP@ss1",
		FirstName: "Jane",
		LastName:  "Doe",
		RoleID:    &roleID,
	})
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, int64(100), account.ID)
	require.NotNil(t, createdStaff)
	assert.Equal(t, int64(200), createdStaff.PersonID)
	require.NotNil(t, createdTeacher)
	assert.Equal(t, int64(300), createdTeacher.StaffID)
}

// The operator API must refuse the same combination the invitation flow
// refuses (#1772): lehrkraft plus caregiver_enabled would grant the full
// user role and a caregiver profile, defeating the role's class-scoped
// read-only design.
func TestCreateSchoolAccount_RejectsLehrkraftCaregiverCombo(t *testing.T) {
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

	roleID := int64(5)

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: 9},
					OrganizationID: 3,
					Name:           "School",
					Slug:           "school",
					Subdomain:      "school",
					Active:         true,
				}, nil
			},
		},
		RoleRepo: &mockRoleRepo{
			findByIDFn: func(_ context.Context, id interface{}) (*authModels.Role, error) {
				return &authModels.Role{
					Model:    base.Model{ID: roleID},
					Name:     "lehrkraft",
					IsSystem: true,
				}, nil
			},
		},
		AuthService: &mockAuthService{
			registerFn: func(_ context.Context, _, _, _ string, _ *int64, _ int64) (*authModels.Account, error) {
				t.Fatal("no account must be created for a rejected lehrkraft+caregiver request")
				return nil, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	_, err = service.CreateSchoolAccount(context.Background(), 9, 7, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:            "lehrkraft@example.com",
		Password:         "SecureP@ss1",
		FirstName:        "Jane",
		LastName:         "Doe",
		RoleID:           &roleID,
		CaregiverEnabled: true,
	})
	require.Error(t, err)
	var invalidErr *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalidErr)
	assert.Contains(t, invalidErr.Error(), "lehrkraft")
}

func TestCreateSchoolAccount_DefaultsToAdminRole(t *testing.T) {
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

	var registeredRoleID *int64

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: 9},
					OrganizationID: 3,
					Name:           "School",
					Slug:           "school",
					Subdomain:      "school",
					Active:         true,
				}, nil
			},
		},
		RoleRepo: &mockRoleRepo{
			roles: []*authModels.Role{
				{Model: base.Model{ID: 4}, Name: "admin", IsSystem: true},
			},
			findByIDFn: func(_ context.Context, id interface{}) (*authModels.Role, error) {
				return &authModels.Role{
					Model:    base.Model{ID: 4},
					Name:     "admin",
					IsSystem: true,
				}, nil
			},
		},
		AuthService: &mockAuthService{
			registerFn: func(_ context.Context, email, username, password string, rID *int64, tenantID int64) (*authModels.Account, error) {
				registeredRoleID = rID
				return &authModels.Account{
					Model: base.Model{ID: 100},
					Email: email,
				}, nil
			},
		},
		PersonRepo: &mockPersonRepo{
			createFn: func(_ context.Context, person *userModels.Person) error {
				person.ID = 200
				return nil
			},
		},
		StaffRepo:    &testpkg.StaffRepoMock{},
		TeacherRepo:  &mockTeacherRepo{},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	account, err := service.CreateSchoolAccount(context.Background(), 9, 7, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:     "admin@example.com",
		Password:  "SecureP@ss1",
		FirstName: "Admin",
		LastName:  "User",
		RoleID:    nil, // should default to admin
	})
	require.NoError(t, err)
	require.NotNil(t, account)
	require.NotNil(t, registeredRoleID)
	assert.Equal(t, int64(4), *registeredRoleID)
}

func TestCreateSchoolAccount_SchoolNotFound(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo:    &testpkg.SchoolRepoMock{},
	})

	account, err := service.CreateSchoolAccount(context.Background(), 999, 7, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:     "test@example.com",
		Password:  "SecureP@ss1",
		FirstName: "Test",
		LastName:  "User",
	})
	require.Nil(t, account)
	require.Error(t, err)
	var notFoundErr *platformSvc.SchoolNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestCreateSchoolAccount_InactiveSchool(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: 9},
					OrganizationID: 3,
					Name:           "School",
					Slug:           "school",
					Subdomain:      "school",
					Active:         false,
				}, nil
			},
		},
	})

	account, err := service.CreateSchoolAccount(context.Background(), 9, 7, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:     "test@example.com",
		Password:  "SecureP@ss1",
		FirstName: "Test",
		LastName:  "User",
	})
	require.Nil(t, account)
	require.Error(t, err)
	var invalidErr *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalidErr)
	assert.Contains(t, err.Error(), "school is inactive")
}

func TestCreateSchoolAccount_RegisterFails(t *testing.T) {
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

	registerErr := errors.New("email already exists")
	roleID := int64(5)

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: 9}, OrganizationID: 3, Name: "School", Slug: "school", Subdomain: "school", Active: true,
				}, nil
			},
		},
		RoleRepo: &mockRoleRepo{
			findByIDFn: func(_ context.Context, id interface{}) (*authModels.Role, error) {
				return &authModels.Role{Model: base.Model{ID: roleID}, Name: "admin", IsSystem: true}, nil
			},
		},
		AuthService: &mockAuthService{
			registerFn: func(context.Context, string, string, string, *int64, int64) (*authModels.Account, error) {
				return nil, registerErr
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	account, err := service.CreateSchoolAccount(context.Background(), 9, 7, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:     "test@example.com",
		Password:  "SecureP@ss1",
		FirstName: "Test",
		LastName:  "User",
		RoleID:    &roleID,
	})
	require.Nil(t, account)
	require.ErrorIs(t, err, registerErr)
}

func TestCreateSchoolAccount_PersonCreateFails(t *testing.T) {
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

	roleID := int64(5)

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: 9}, OrganizationID: 3, Name: "School", Slug: "school", Subdomain: "school", Active: true,
				}, nil
			},
		},
		RoleRepo: &mockRoleRepo{
			findByIDFn: func(_ context.Context, id interface{}) (*authModels.Role, error) {
				return &authModels.Role{Model: base.Model{ID: roleID}, Name: "admin", IsSystem: true}, nil
			},
		},
		AuthService: &mockAuthService{
			registerFn: func(_ context.Context, email, username, password string, rID *int64, tenantID int64) (*authModels.Account, error) {
				return &authModels.Account{Model: base.Model{ID: 100}, Email: email}, nil
			},
		},
		PersonRepo: &mockPersonRepo{
			createFn: func(context.Context, *userModels.Person) error {
				return assert.AnError
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	account, err := service.CreateSchoolAccount(context.Background(), 9, 7, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:     "test@example.com",
		Password:  "SecureP@ss1",
		FirstName: "Test",
		LastName:  "User",
		RoleID:    &roleID,
	})
	require.Nil(t, account)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create person")
}

func TestCreateSchoolAccount_LinkPersonFails(t *testing.T) {
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

	roleID := int64(5)

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: 9}, OrganizationID: 3, Name: "School", Slug: "school", Subdomain: "school", Active: true,
				}, nil
			},
		},
		RoleRepo: &mockRoleRepo{
			findByIDFn: func(_ context.Context, id interface{}) (*authModels.Role, error) {
				return &authModels.Role{Model: base.Model{ID: roleID}, Name: "admin", IsSystem: true}, nil
			},
		},
		AuthService: &mockAuthService{
			registerFn: func(_ context.Context, email, username, password string, rID *int64, tenantID int64) (*authModels.Account, error) {
				return &authModels.Account{Model: base.Model{ID: 100}, Email: email}, nil
			},
		},
		PersonRepo: &mockPersonRepo{
			createFn: func(_ context.Context, person *userModels.Person) error {
				person.ID = 200
				return nil
			},
			linkToAccountFn: func(context.Context, int64, int64) error {
				return assert.AnError
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	account, err := service.CreateSchoolAccount(context.Background(), 9, 7, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:     "test@example.com",
		Password:  "SecureP@ss1",
		FirstName: "Test",
		LastName:  "User",
		RoleID:    &roleID,
	})
	require.Nil(t, account)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "link person to account")
}

func TestCreateSchoolAccount_StaffCreateFails(t *testing.T) {
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

	roleID := int64(5)

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: 9}, OrganizationID: 3, Name: "School", Slug: "school", Subdomain: "school", Active: true,
				}, nil
			},
		},
		RoleRepo: &mockRoleRepo{
			findByIDFn: func(_ context.Context, id interface{}) (*authModels.Role, error) {
				return &authModels.Role{
					Model:    base.Model{ID: roleID},
					Name:     "user",
					IsSystem: true,
				}, nil
			},
		},
		AuthService: &mockAuthService{
			registerFn: func(_ context.Context, email, username, password string, rID *int64, tenantID int64) (*authModels.Account, error) {
				return &authModels.Account{Model: base.Model{ID: 100}, Email: email}, nil
			},
		},
		PersonRepo: &mockPersonRepo{
			createFn: func(_ context.Context, person *userModels.Person) error {
				person.ID = 200
				return nil
			},
		},
		StaffRepo: &testpkg.StaffRepoMock{
			CreateFn: func(context.Context, *userModels.Staff) error {
				return assert.AnError
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	account, err := service.CreateSchoolAccount(context.Background(), 9, 7, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:     "test@example.com",
		Password:  "SecureP@ss1",
		FirstName: "Test",
		LastName:  "User",
		RoleID:    &roleID,
	})
	require.Nil(t, account)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create staff")
}

func TestCreateSchoolAccount_TeacherCreateFails(t *testing.T) {
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

	roleID := int64(5)

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: 9}, OrganizationID: 3, Name: "School", Slug: "school", Subdomain: "school", Active: true,
				}, nil
			},
		},
		RoleRepo: &mockRoleRepo{
			findByIDFn: func(_ context.Context, id interface{}) (*authModels.Role, error) {
				return &authModels.Role{
					Model:    base.Model{ID: roleID},
					Name:     "user",
					IsSystem: true,
				}, nil
			},
		},
		AuthService: &mockAuthService{
			registerFn: func(_ context.Context, email, username, password string, rID *int64, tenantID int64) (*authModels.Account, error) {
				return &authModels.Account{Model: base.Model{ID: 100}, Email: email}, nil
			},
		},
		PersonRepo: &mockPersonRepo{
			createFn: func(_ context.Context, person *userModels.Person) error {
				person.ID = 200
				return nil
			},
		},
		StaffRepo: &testpkg.StaffRepoMock{
			CreateFn: func(_ context.Context, staff *userModels.Staff) error {
				staff.ID = 300
				return nil
			},
		},
		TeacherRepo: &mockTeacherRepo{
			createFn: func(context.Context, *userModels.Teacher) error {
				return assert.AnError
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	account, err := service.CreateSchoolAccount(context.Background(), 9, 7, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:     "test@example.com",
		Password:  "SecureP@ss1",
		FirstName: "Test",
		LastName:  "User",
		RoleID:    &roleID,
	})
	require.Nil(t, account)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create teacher")
}

func TestCreateSchoolAccount_NonSystemRole_Rejected(t *testing.T) {
	roleID := int64(5)

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: 9}, OrganizationID: 3, Name: "School", Slug: "school", Subdomain: "school", Active: true,
				}, nil
			},
		},
		RoleRepo: &mockRoleRepo{
			findByIDFn: func(_ context.Context, id interface{}) (*authModels.Role, error) {
				return &authModels.Role{
					Model:    base.Model{ID: roleID},
					Name:     "custom-role",
					IsSystem: false,
				}, nil
			},
		},
	})

	account, err := service.CreateSchoolAccount(context.Background(), 9, 7, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:     "test@example.com",
		Password:  "SecureP@ss1",
		FirstName: "Test",
		LastName:  "User",
		RoleID:    &roleID,
	})
	require.Nil(t, account)
	require.Error(t, err)
	var invalidErr *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalidErr)
	assert.Contains(t, err.Error(), "only system roles are allowed")
}

func TestCreateSchoolAccount_GuardianRole_Rejected(t *testing.T) {
	roleID := int64(6)

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: 9}, OrganizationID: 3, Name: "School", Slug: "school", Subdomain: "school", Active: true,
				}, nil
			},
		},
		RoleRepo: &mockRoleRepo{
			findByIDFn: func(_ context.Context, id interface{}) (*authModels.Role, error) {
				return &authModels.Role{
					Model:    base.Model{ID: roleID},
					Name:     "guardian",
					IsSystem: true,
				}, nil
			},
		},
	})

	account, err := service.CreateSchoolAccount(context.Background(), 9, 7, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:     "parent@example.com",
		Password:  "SecureP@ss1",
		FirstName: "Parent",
		LastName:  "User",
		RoleID:    &roleID,
	})
	require.Nil(t, account)
	require.Error(t, err)
	var invalidErr *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalidErr)
	assert.Contains(t, err.Error(), "guardian")
}

func TestCreateSchoolAccount_TeacherRole_Rejected(t *testing.T) {
	roleID := int64(7)

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: 9}, OrganizationID: 3, Name: "School", Slug: "school", Subdomain: "school", Active: true,
				}, nil
			},
		},
		RoleRepo: &mockRoleRepo{
			findByIDFn: func(_ context.Context, id interface{}) (*authModels.Role, error) {
				return &authModels.Role{
					Model:    base.Model{ID: roleID},
					Name:     "teacher",
					IsSystem: true,
				}, nil
			},
		},
	})

	account, err := service.CreateSchoolAccount(context.Background(), 9, 7, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:     "teacher@example.com",
		Password:  "SecureP@ss1",
		FirstName: "Legacy",
		LastName:  "Teacher",
		RoleID:    &roleID,
	})
	require.Nil(t, account)
	require.Error(t, err)
	var invalidErr *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalidErr)
	assert.Contains(t, err.Error(), "legacy teacher role is no longer assignable")
}

func TestCreateSchoolAccount_RoleNotFound_Rejected(t *testing.T) {
	roleID := int64(999)

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: 9}, OrganizationID: 3, Name: "School", Slug: "school", Subdomain: "school", Active: true,
				}, nil
			},
		},
		RoleRepo: &mockRoleRepo{
			findByIDFn: func(_ context.Context, id interface{}) (*authModels.Role, error) {
				return nil, nil
			},
		},
	})

	account, err := service.CreateSchoolAccount(context.Background(), 9, 7, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:     "test@example.com",
		Password:  "SecureP@ss1",
		FirstName: "Test",
		LastName:  "User",
		RoleID:    &roleID,
	})
	require.Nil(t, account)
	require.Error(t, err)
	var invalidErr *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalidErr)
	assert.Contains(t, err.Error(), "not found")
}

func TestCreateSchoolAccount_RoleLookupError_Propagated(t *testing.T) {
	roleID := int64(5)

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: 9}, OrganizationID: 3, Name: "School", Slug: "school", Subdomain: "school", Active: true,
				}, nil
			},
		},
		RoleRepo: &mockRoleRepo{
			findByIDFn: func(_ context.Context, id interface{}) (*authModels.Role, error) {
				return nil, assert.AnError
			},
		},
	})

	account, err := service.CreateSchoolAccount(context.Background(), 9, 7, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:     "test@example.com",
		Password:  "SecureP@ss1",
		FirstName: "Test",
		LastName:  "User",
		RoleID:    &roleID,
	})
	require.Nil(t, account)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lookup role")
}

func TestCreateSchoolAccount_AdminRole_NoTeacher(t *testing.T) {
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

	roleID := int64(4)
	teacherCreated := false

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: 9}, OrganizationID: 3, Name: "School", Slug: "school", Subdomain: "school", Active: true,
				}, nil
			},
		},
		RoleRepo: &mockRoleRepo{
			findByIDFn: func(_ context.Context, id interface{}) (*authModels.Role, error) {
				return &authModels.Role{
					Model:    base.Model{ID: roleID},
					Name:     "admin",
					IsSystem: true,
				}, nil
			},
		},
		AuthService: &mockAuthService{
			registerFn: func(_ context.Context, email, username, password string, rID *int64, tenantID int64) (*authModels.Account, error) {
				return &authModels.Account{Model: base.Model{ID: 100}, Email: email}, nil
			},
		},
		PersonRepo: &mockPersonRepo{
			createFn: func(_ context.Context, person *userModels.Person) error {
				person.ID = 200
				return nil
			},
		},
		StaffRepo: &testpkg.StaffRepoMock{
			CreateFn: func(_ context.Context, staff *userModels.Staff) error {
				staff.ID = 300
				return nil
			},
		},
		TeacherRepo: &mockTeacherRepo{
			createFn: func(context.Context, *userModels.Teacher) error {
				teacherCreated = true
				return nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	account, err := service.CreateSchoolAccount(context.Background(), 9, 7, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:     "admin@example.com",
		Password:  "SecureP@ss1",
		FirstName: "Admin",
		LastName:  "User",
		RoleID:    &roleID,
	})
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.False(t, teacherCreated, "teacher should not be created for admin role")
}

func TestCreateSchoolAccount_WithPosition(t *testing.T) {
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

	roleID := int64(5)
	var createdTeacher *userModels.Teacher

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: 9}, OrganizationID: 3, Name: "School", Slug: "school", Subdomain: "school", Active: true,
				}, nil
			},
		},
		RoleRepo: &mockRoleRepo{
			findByIDFn: func(_ context.Context, id interface{}) (*authModels.Role, error) {
				return &authModels.Role{
					Model:    base.Model{ID: roleID},
					Name:     "user",
					IsSystem: true,
				}, nil
			},
		},
		AuthService: &mockAuthService{
			registerFn: func(_ context.Context, email, username, password string, rID *int64, tenantID int64) (*authModels.Account, error) {
				return &authModels.Account{Model: base.Model{ID: 100}, Email: email}, nil
			},
		},
		PersonRepo: &mockPersonRepo{
			createFn: func(_ context.Context, person *userModels.Person) error {
				person.ID = 200
				return nil
			},
		},
		StaffRepo: &testpkg.StaffRepoMock{
			CreateFn: func(_ context.Context, staff *userModels.Staff) error {
				staff.ID = 300
				return nil
			},
		},
		TeacherRepo: &mockTeacherRepo{
			createFn: func(_ context.Context, teacher *userModels.Teacher) error {
				createdTeacher = teacher
				return nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
	})

	account, err := service.CreateSchoolAccount(context.Background(), 9, 7, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:     "teacher@example.com",
		Password:  "SecureP@ss1",
		FirstName: "Jane",
		LastName:  "Doe",
		RoleID:    &roleID,
		Position:  "Klassenlehrerin",
	})
	require.NoError(t, err)
	require.NotNil(t, account)
	require.NotNil(t, createdTeacher)
	assert.Equal(t, "Klassenlehrerin", createdTeacher.Role)
}

// ---------------------------------------------------------------------------
// ListSystemRoles tests
// ---------------------------------------------------------------------------

func TestListSystemRoles_Success(t *testing.T) {
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

	expected := []*authModels.Role{
		{Model: base.Model{ID: 1}, Name: "admin", IsSystem: true},
		{Model: base.Model{ID: 2}, Name: "user", IsSystem: true},
	}

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		RoleRepo:      &mockRoleRepo{roles: expected},
		DB:            bunDB,
	})

	roles, err := service.ListSystemRoles(context.Background())
	require.NoError(t, err)
	require.Equal(t, expected, roles)
}

func TestListSystemRoles_Error(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		RoleRepo:      &mockRoleRepo{},
	})

	// With no txHandler (nil DB), withAdminTx runs callback directly.
	// Default mockRoleRepo returns nil, nil when both role and roles are nil.
	roles, err := service.ListSystemRoles(context.Background())
	require.NoError(t, err)
	require.Nil(t, roles)
}

// --- SoftDeletePerson tests ---

func TestOperatorProvisioningService_SoftDeletePerson_InvalidID(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
	})

	err := service.SoftDeletePerson(context.Background(), 0, 1, net.IPv4(127, 0, 0, 1))
	require.Error(t, err)
	var invalidErr *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalidErr)
}

func TestOperatorProvisioningService_SoftDeletePerson_NegativeID(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
	})

	err := service.SoftDeletePerson(context.Background(), -5, 1, net.IPv4(127, 0, 0, 1))
	require.Error(t, err)
	var invalidErr *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalidErr)
}

// --- ListSchoolPersons tests ---

func TestOperatorProvisioningService_ListSchoolPersons_SchoolNotFound_NilReturn(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo:    &testpkg.SchoolRepoMock{},
	})

	persons, err := service.ListSchoolPersons(context.Background(), 999)
	require.Nil(t, persons)
	require.Error(t, err)
	var notFoundErr *platformSvc.SchoolNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestOperatorProvisioningService_ListSchoolPersons_SchoolNotFound_SqlErrNoRows(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return nil, sql.ErrNoRows
			},
		},
	})

	persons, err := service.ListSchoolPersons(context.Background(), 999)
	require.Nil(t, persons)
	require.Error(t, err)
	var notFoundErr *platformSvc.SchoolNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestOperatorProvisioningService_SoftDeleteSchool_Success(t *testing.T) {
	softDeleteCalled := false
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: id}, Name: "Test School",
					Slug: "test", Subdomain: "test", Active: true,
				}, nil
			},
			SoftDeleteFn: func(_ context.Context, _ int64) error {
				softDeleteCalled = true
				return nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	err := service.SoftDeleteSchool(context.Background(), 900, 10, nil)
	require.NoError(t, err)
	assert.True(t, softDeleteCalled, "should call SoftDelete on repo")
}

func TestOperatorProvisioningService_SoftDeleteSchool_NotFound(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo:    &testpkg.SchoolRepoMock{},
		AuditLogRepo:  &mockAuditLogRepoShared{},
	})

	err := service.SoftDeleteSchool(context.Background(), 999999, 10, nil)
	require.Error(t, err)
	var notFound *platformSvc.SchoolNotFoundError
	assert.ErrorAs(t, err, &notFound)
}

func TestOperatorProvisioningService_SoftDeleteSchool_AlreadyDeleted(t *testing.T) {
	now := time.Now()
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: id}, DeletedAt: &now,
					Name: "Test", Slug: "test", Subdomain: "test",
				}, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	err := service.SoftDeleteSchool(context.Background(), 900, 10, nil)
	require.Error(t, err)
	var alreadyDeleted *platformSvc.SchoolAlreadyDeletedError
	assert.ErrorAs(t, err, &alreadyDeleted)
}

func TestOperatorProvisioningService_RestoreSchool_Success(t *testing.T) {
	restoreCalled := false
	now := time.Now()
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: id}, DeletedAt: &now,
					Name: "Test", Slug: "test", Subdomain: "test",
					OrganizationID: 1,
				}, nil
			},
			RestoreFn: func(_ context.Context, _ int64) error {
				restoreCalled = true
				return nil
			},
		},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{
					Model: base.Model{ID: id}, Name: "Org", Slug: "org", Active: true,
				}, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	err := service.RestoreSchool(context.Background(), 900, 10, nil)
	require.NoError(t, err)
	assert.True(t, restoreCalled, "should call Restore on repo")
}

func TestOperatorProvisioningService_RestoreSchool_ParentOrgDeleted(t *testing.T) {
	now := time.Now()
	restoreCalled := false
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: id}, DeletedAt: &now,
					Name: "Test", Slug: "test", Subdomain: "test",
					OrganizationID: 1,
				}, nil
			},
			RestoreFn: func(_ context.Context, _ int64) error {
				restoreCalled = true
				return nil
			},
		},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{
					Model: base.Model{ID: id}, Name: "Org", Slug: "org", Active: true,
					DeletedAt: &now,
				}, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	err := service.RestoreSchool(context.Background(), 900, 10, nil)
	require.Error(t, err)
	var orgDeleted *platformSvc.OrganizationDeletedError
	assert.ErrorAs(t, err, &orgDeleted)
	assert.False(t, restoreCalled, "must not restore school under deleted org")
}

func TestOperatorProvisioningService_RestoreSchool_NotDeleted(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: id}, Name: "Test",
					Slug: "test", Subdomain: "test", Active: true,
				}, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	err := service.RestoreSchool(context.Background(), 900, 10, nil)
	require.Error(t, err)
	var notDeleted *platformSvc.SchoolNotDeletedError
	assert.ErrorAs(t, err, &notDeleted)
}

// ---------------------------------------------------------------------------
// SoftDeleteOrganization / RestoreOrganization
// ---------------------------------------------------------------------------

func TestOperatorProvisioningService_SoftDeleteOrganization_Success(t *testing.T) {
	softDeleteCalled := false
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{
					Model: base.Model{ID: id}, Name: "Test Org", Slug: "test-org", Active: true,
				}, nil
			},
			softDeleteFn: func(_ context.Context, _ int64) error {
				softDeleteCalled = true
				return nil
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{
			CountNonDeletedByOrganizationIDFn: func(_ context.Context, _ int64) (int, error) {
				return 0, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	err := service.SoftDeleteOrganization(context.Background(), 100, 10, nil)
	require.NoError(t, err)
	assert.True(t, softDeleteCalled, "should call SoftDelete on org repo")
}

func TestOperatorProvisioningService_SoftDeleteOrganization_NotFound(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo:    &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{},
		SchoolRepo:       &testpkg.SchoolRepoMock{},
		AuditLogRepo:     &mockAuditLogRepoShared{},
	})

	err := service.SoftDeleteOrganization(context.Background(), 999999, 10, nil)
	require.Error(t, err)
	var notFound *platformSvc.OrganizationNotFoundError
	assert.ErrorAs(t, err, &notFound)
}

func TestOperatorProvisioningService_SoftDeleteOrganization_AlreadyDeleted(t *testing.T) {
	now := time.Now()
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{
					Model: base.Model{ID: id}, Name: "Test", Slug: "test", DeletedAt: &now,
				}, nil
			},
		},
		SchoolRepo:   &testpkg.SchoolRepoMock{},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	err := service.SoftDeleteOrganization(context.Background(), 100, 10, nil)
	require.Error(t, err)
	var alreadyDeleted *platformSvc.OrganizationAlreadyDeletedError
	assert.ErrorAs(t, err, &alreadyDeleted)
}

func TestOperatorProvisioningService_SoftDeleteOrganization_HasActiveSchools(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{
					Model: base.Model{ID: id}, Name: "Test Org", Slug: "test-org", Active: true,
				}, nil
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{
			CountNonDeletedByOrganizationIDFn: func(_ context.Context, _ int64) (int, error) {
				return 3, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	err := service.SoftDeleteOrganization(context.Background(), 100, 10, nil)
	require.Error(t, err)
	var hasSchools *platformSvc.OrganizationHasSchoolsError
	require.ErrorAs(t, err, &hasSchools)
	assert.Equal(t, 3, hasSchools.SchoolCount)
}

func TestOperatorProvisioningService_RestoreOrganization_Success(t *testing.T) {
	restoreCalled := false
	now := time.Now()
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{
					Model: base.Model{ID: id}, Name: "Test", Slug: "test", DeletedAt: &now,
				}, nil
			},
			restoreFn: func(_ context.Context, _ int64) error {
				restoreCalled = true
				return nil
			},
		},
		SchoolRepo:   &testpkg.SchoolRepoMock{},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	err := service.RestoreOrganization(context.Background(), 100, 10, nil)
	require.NoError(t, err)
	assert.True(t, restoreCalled, "should call Restore on org repo")
}

func TestOperatorProvisioningService_RestoreOrganization_NotFound(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo:    &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{},
		SchoolRepo:       &testpkg.SchoolRepoMock{},
		AuditLogRepo:     &mockAuditLogRepoShared{},
	})

	err := service.RestoreOrganization(context.Background(), 999999, 10, nil)
	require.Error(t, err)
	var notFound *platformSvc.OrganizationNotFoundError
	assert.ErrorAs(t, err, &notFound)
}

func TestOperatorProvisioningService_RestoreOrganization_NotDeleted(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{
					Model: base.Model{ID: id}, Name: "Test", Slug: "test", Active: true,
				}, nil
			},
		},
		SchoolRepo:   &testpkg.SchoolRepoMock{},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	err := service.RestoreOrganization(context.Background(), 100, 10, nil)
	require.Error(t, err)
	var notDeleted *platformSvc.OrganizationNotDeletedError
	assert.ErrorAs(t, err, &notDeleted)
}

func TestOperatorProvisioningService_RestoreSchool_ParentOrgLookupFails(t *testing.T) {
	// Covers the FindByIDForShare error branch in RestoreSchool: the repo
	// error must bubble up unchanged, the school must not be restored.
	now := time.Now()
	lookupErr := errors.New("db connection lost")
	restoreCalled := false
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: id}, DeletedAt: &now,
					Name: "Test", Slug: "test", Subdomain: "test",
					OrganizationID: 1,
				}, nil
			},
			RestoreFn: func(_ context.Context, _ int64) error {
				restoreCalled = true
				return nil
			},
		},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return nil, lookupErr
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	err := service.RestoreSchool(context.Background(), 900, 10, nil)
	require.ErrorIs(t, err, lookupErr)
	assert.False(t, restoreCalled, "must not restore when parent org lookup fails")
}

func TestOperatorProvisioningService_RestoreSchool_ParentOrgMissing(t *testing.T) {
	// Covers the parentOrg == nil branch in RestoreSchool: should map to
	// OrganizationNotFoundError and not attempt to restore.
	now := time.Now()
	restoreCalled := false
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model: base.Model{ID: id}, DeletedAt: &now,
					Name: "Test", Slug: "test", Subdomain: "test",
					OrganizationID: 42,
				}, nil
			},
			RestoreFn: func(_ context.Context, _ int64) error {
				restoreCalled = true
				return nil
			},
		},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return nil, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	err := service.RestoreSchool(context.Background(), 900, 10, nil)
	require.Error(t, err)
	var notFound *platformSvc.OrganizationNotFoundError
	assert.ErrorAs(t, err, &notFound)
	assert.Equal(t, int64(42), notFound.OrganizationID)
	assert.False(t, restoreCalled)
}

func TestOperatorProvisioningService_SoftDeleteOrganization_CountSchoolsFails(t *testing.T) {
	// Covers the CountNonDeletedByOrganizationID error branch in
	// SoftDeleteOrganization: the repo error must be wrapped and returned,
	// and the org must not be soft-deleted.
	countErr := errors.New("count query failed")
	softDeleteCalled := false
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{
					Model: base.Model{ID: id}, Name: "Org", Slug: "org", Active: true,
				}, nil
			},
			softDeleteFn: func(context.Context, int64) error {
				softDeleteCalled = true
				return nil
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{
			CountNonDeletedByOrganizationIDFn: func(context.Context, int64) (int, error) {
				return 0, countErr
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	err := service.SoftDeleteOrganization(context.Background(), 100, 10, nil)
	require.ErrorIs(t, err, countErr)
	assert.False(t, softDeleteCalled, "must not soft-delete when count query fails")
}

func TestOperatorProvisioningService_SoftDeleteOrganization_RepoErrorFallsThrough(t *testing.T) {
	// Covers the non-rows-mismatch branch of SoftDeleteOrganization: a raw
	// repo error (not a rows-affected mismatch) must propagate unchanged.
	softDeleteErr := errors.New("unexpected db error")
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{
					Model: base.Model{ID: id}, Name: "Org", Slug: "org", Active: true,
				}, nil
			},
			softDeleteFn: func(context.Context, int64) error {
				return softDeleteErr
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{
			CountNonDeletedByOrganizationIDFn: func(context.Context, int64) (int, error) {
				return 0, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	err := service.SoftDeleteOrganization(context.Background(), 100, 10, nil)
	require.ErrorIs(t, err, softDeleteErr)
}

func TestOperatorProvisioningService_RestoreOrganization_RepoErrorFallsThrough(t *testing.T) {
	// Covers the non-rows-mismatch branch of RestoreOrganization: a raw
	// repo error (not a rows-affected mismatch) must propagate unchanged.
	now := time.Now()
	restoreErr := errors.New("unexpected db error")
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{
					Model: base.Model{ID: id}, Name: "Org", Slug: "org", Active: true,
					DeletedAt: &now,
				}, nil
			},
			restoreFn: func(context.Context, int64) error {
				return restoreErr
			},
		},
		SchoolRepo:   &testpkg.SchoolRepoMock{},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	err := service.RestoreOrganization(context.Background(), 100, 10, nil)
	require.ErrorIs(t, err, restoreErr)
}

// ---------------------------------------------------------------------------
// mockDeviceRepoWithFind extends mockDeviceRepo with a configurable FindByID.
// ---------------------------------------------------------------------------

type mockDeviceRepoWithFind struct {
	mockDeviceRepo
	findByIDFn func(context.Context, interface{}) (*iotModels.Device, error)
}

func (m *mockDeviceRepoWithFind) FindByID(ctx context.Context, id interface{}) (*iotModels.Device, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// Soft-delete guard tests: verify that methods reject soft-deleted schools
// ---------------------------------------------------------------------------

func TestOperatorProvisioningService_UpdateSchool_RejectsDeletedSchool(t *testing.T) {
	now := time.Now()
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: id},
					OrganizationID: 100,
					Name:           "Deleted School",
					Slug:           "deleted-school",
					Subdomain:      "deleted-school",
					Active:         true,
					DeletedAt:      &now,
				}, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	updated, err := service.UpdateSchool(context.Background(), 500, platformSvc.UpdateSchoolRequest{
		OrganizationID: 100,
		Name:           "Updated Name",
		Slug:           "deleted-school",
		Subdomain:      "deleted-school",
		Active:         true,
	}, 10, net.IPv4(127, 0, 0, 1))
	require.Nil(t, updated)
	require.Error(t, err)
	var deletedErr *platformSvc.SchoolAlreadyDeletedError
	require.ErrorAs(t, err, &deletedErr)
}

func TestOperatorProvisioningService_ListSchoolAccounts_RejectsDeletedSchool(t *testing.T) {
	now := time.Now()
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: id},
					OrganizationID: 100,
					Name:           "Deleted School",
					Slug:           "deleted-school",
					Subdomain:      "deleted-school",
					Active:         true,
					DeletedAt:      &now,
				}, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	accounts, err := service.ListSchoolAccounts(context.Background(), 500)
	require.Nil(t, accounts)
	require.Error(t, err)
	var deletedErr *platformSvc.SchoolAlreadyDeletedError
	require.ErrorAs(t, err, &deletedErr)
}

func TestOperatorProvisioningService_ListSchoolDevices_RejectsDeletedSchool(t *testing.T) {
	now := time.Now()
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: id},
					OrganizationID: 100,
					Name:           "Deleted School",
					Slug:           "deleted-school",
					Subdomain:      "deleted-school",
					Active:         true,
					DeletedAt:      &now,
				}, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	devices, err := service.ListSchoolDevices(context.Background(), 500)
	require.Nil(t, devices)
	require.Error(t, err)
	var deletedErr *platformSvc.SchoolAlreadyDeletedError
	require.ErrorAs(t, err, &deletedErr)
}

func TestOperatorProvisioningService_CreateDevice_RejectsDeletedSchool(t *testing.T) {
	now := time.Now()
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: id},
					OrganizationID: 100,
					Name:           "Deleted School",
					Slug:           "deleted-school",
					Subdomain:      "deleted-school",
					Active:         true,
					DeletedAt:      &now,
				}, nil
			},
		},
		DeviceRepo:   &mockDeviceRepo{},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	device, err := service.CreateDevice(context.Background(), 500, "device-100", "rfid", nil, nil, 10, net.IPv4(127, 0, 0, 1))
	require.Nil(t, device)
	require.Error(t, err)
	var deletedErr *platformSvc.SchoolAlreadyDeletedError
	require.ErrorAs(t, err, &deletedErr)
}

func TestOperatorProvisioningService_SetDeviceAPIKey_RejectsDeletedSchool(t *testing.T) {
	now := time.Now()
	schoolID := int64(500)
	apiKey := "dev_testkey1234567890abcdef1234567890abcdef1234567890abcdef12345678"

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		DeviceRepo: &mockDeviceRepoWithFind{
			findByIDFn: func(_ context.Context, id interface{}) (*iotModels.Device, error) {
				d := &iotModels.Device{
					Model:      base.Model{ID: 200},
					DeviceID:   "device-200",
					DeviceType: "rfid",
					Status:     iotModels.DeviceStatusActive,
					APIKey:     &apiKey,
				}
				d.SetTenantID(schoolID)
				return d, nil
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: id},
					OrganizationID: 100,
					Name:           "Deleted School",
					Slug:           "deleted-school",
					Subdomain:      "deleted-school",
					Active:         true,
					DeletedAt:      &now,
				}, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	result, err := service.SetDeviceAPIKey(context.Background(), 200, nil, 10, net.IPv4(127, 0, 0, 1))
	require.Nil(t, result)
	require.Error(t, err)
	var deletedErr *platformSvc.SchoolAlreadyDeletedError
	require.ErrorAs(t, err, &deletedErr)
}

func TestOperatorProvisioningService_SetDeviceAPIKey_RejectsNilSchool(t *testing.T) {
	schoolID := int64(500)
	apiKey := "dev_testkey1234567890abcdef1234567890abcdef1234567890abcdef12345678"

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		DeviceRepo: &mockDeviceRepoWithFind{
			findByIDFn: func(_ context.Context, _ interface{}) (*iotModels.Device, error) {
				d := &iotModels.Device{
					Model:      base.Model{ID: 200},
					DeviceID:   "device-200",
					DeviceType: "rfid",
					Status:     iotModels.DeviceStatusActive,
					APIKey:     &apiKey,
				}
				d.SetTenantID(schoolID)
				return d, nil
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, _ int64) (*platformModels.School, error) {
				return nil, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	result, err := service.SetDeviceAPIKey(context.Background(), 200, nil, 10, net.IPv4(127, 0, 0, 1))
	require.Nil(t, result)
	require.Error(t, err)
	var notFoundErr *platformSvc.SchoolNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestOperatorProvisioningService_SetDeviceAPIKey_RejectsInactiveSchool(t *testing.T) {
	schoolID := int64(500)
	apiKey := "dev_testkey1234567890abcdef1234567890abcdef1234567890abcdef12345678"

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		DeviceRepo: &mockDeviceRepoWithFind{
			findByIDFn: func(_ context.Context, _ interface{}) (*iotModels.Device, error) {
				d := &iotModels.Device{
					Model:      base.Model{ID: 200},
					DeviceID:   "device-200",
					DeviceType: "rfid",
					Status:     iotModels.DeviceStatusActive,
					APIKey:     &apiKey,
				}
				d.SetTenantID(schoolID)
				return d, nil
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: id},
					OrganizationID: 100,
					Name:           "Inactive School",
					Slug:           "inactive-school",
					Subdomain:      "inactive-school",
					Active:         false,
				}, nil
			},
		},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	result, err := service.SetDeviceAPIKey(context.Background(), 200, nil, 10, net.IPv4(127, 0, 0, 1))
	require.Nil(t, result)
	require.Error(t, err)
	var inactiveErr *platformSvc.SchoolInactiveError
	require.ErrorAs(t, err, &inactiveErr)
}

func TestOperatorProvisioningService_GetDeviceTransferStatus_ReportsBlockers(t *testing.T) {
	now := time.Now()
	device := &iotModels.Device{
		Model:      base.Model{ID: 200},
		DeviceID:   "BURBACH-2",
		DeviceType: "terminal",
		Status:     iotModels.DeviceStatusActive,
		LastSeen:   &now,
	}
	device.SetTenantID(10)

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		DeviceRepo: &mockDeviceRepoWithFind{findByIDFn: func(context.Context, interface{}) (*iotModels.Device, error) {
			return device, nil
		}},
		ActiveGroupRepo: &mockActiveDeviceSessionRepo{findFn: func(context.Context, int64) (*activeModels.Group, error) {
			return &activeModels.Group{Model: base.Model{ID: 300}, StartTime: now.Add(-time.Hour)}, nil
		}},
	})

	status, err := service.GetDeviceTransferStatus(context.Background(), 200)
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.False(t, status.CanTransfer)
	assert.True(t, status.IsOnline)
	require.NotNil(t, status.ActiveSession)
	assert.Equal(t, int64(300), status.ActiveSession.ID)
}

func TestOperatorProvisioningService_GetDeviceTransferStatus_RejectsInvalidID(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
	})

	status, err := service.GetDeviceTransferStatus(context.Background(), 0)

	require.Nil(t, status)
	var invalidData *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalidData)
}

func TestOperatorProvisioningService_GetDeviceTransferStatus_ReportsProtectedDevice(t *testing.T) {
	device := &iotModels.Device{
		Model:    base.Model{ID: 200},
		DeviceID: iotModels.WebManualDeviceID,
	}
	device.SetTenantID(10)
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		DeviceRepo: &mockDeviceRepoWithFind{findByIDFn: func(context.Context, interface{}) (*iotModels.Device, error) {
			return device, nil
		}},
	})

	status, err := service.GetDeviceTransferStatus(context.Background(), device.ID)

	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.IsProtected)
	assert.False(t, status.CanTransfer)
}

func TestOperatorProvisioningService_GetDeviceTransferStatus_IncludesSessionNames(t *testing.T) {
	startedAt := time.Now().Add(-time.Hour)
	device := &iotModels.Device{Model: base.Model{ID: 200}, DeviceID: "BURBACH-2"}
	device.SetTenantID(10)
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		DeviceRepo: &mockDeviceRepoWithFind{findByIDFn: func(context.Context, interface{}) (*iotModels.Device, error) {
			return device, nil
		}},
		ActiveGroupRepo: &mockActiveDeviceSessionRepo{findFn: func(context.Context, int64) (*activeModels.Group, error) {
			return &activeModels.Group{
				Model:       base.Model{ID: 300},
				StartTime:   startedAt,
				ActualGroup: &activityModels.Group{Name: "Mensa"},
				Room:        &facilitiesModels.Room{Name: "Speisesaal"},
			}, nil
		}},
	})

	status, err := service.GetDeviceTransferStatus(context.Background(), device.ID)

	require.NoError(t, err)
	require.NotNil(t, status)
	require.NotNil(t, status.ActiveSession)
	require.NotNil(t, status.ActiveSession.ActivityName)
	assert.Equal(t, "Mensa", *status.ActiveSession.ActivityName)
	require.NotNil(t, status.ActiveSession.RoomName)
	assert.Equal(t, "Speisesaal", *status.ActiveSession.RoomName)
}

func TestOperatorProvisioningService_GetDeviceTransferStatus_NotFound(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		DeviceRepo: &mockDeviceRepoWithFind{findByIDFn: func(context.Context, interface{}) (*iotModels.Device, error) {
			return nil, sql.ErrNoRows
		}},
	})

	status, err := service.GetDeviceTransferStatus(context.Background(), 200)

	require.Nil(t, status)
	var notFound *platformSvc.OperatorDeviceNotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestOperatorProvisioningService_GetDeviceTransferStatus_NilDeviceIsNotFound(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		DeviceRepo: &mockDeviceRepoWithFind{findByIDFn: func(context.Context, interface{}) (*iotModels.Device, error) {
			return nil, nil
		}},
	})

	status, err := service.GetDeviceTransferStatus(context.Background(), 200)

	require.Nil(t, status)
	var notFound *platformSvc.OperatorDeviceNotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestOperatorProvisioningService_GetDeviceTransferStatus_SessionLookupFailure(t *testing.T) {
	device := &iotModels.Device{Model: base.Model{ID: 200}, DeviceID: "BURBACH-2"}
	device.SetTenantID(10)
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		DeviceRepo: &mockDeviceRepoWithFind{findByIDFn: func(context.Context, interface{}) (*iotModels.Device, error) {
			return device, nil
		}},
		ActiveGroupRepo: &mockActiveDeviceSessionRepo{findFn: func(context.Context, int64) (*activeModels.Group, error) {
			return nil, assert.AnError
		}},
	})

	status, err := service.GetDeviceTransferStatus(context.Background(), device.ID)

	require.Nil(t, status)
	require.ErrorIs(t, err, assert.AnError)
}

func TestOperatorProvisioningService_GetDeviceTransferStatus_UsesTenantOnlineWindowSetting(t *testing.T) {
	lastSeen := time.Now().Add(-10 * time.Minute)
	device := &iotModels.Device{Model: base.Model{ID: 200}, DeviceID: "BURBACH-2", LastSeen: &lastSeen}
	device.SetTenantID(10)

	var resolvedTenantID int64
	var resolvedKey string
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		DeviceRepo: &mockDeviceRepoWithFind{findByIDFn: func(context.Context, interface{}) (*iotModels.Device, error) {
			return device, nil
		}},
		ActiveGroupRepo: &mockActiveDeviceSessionRepo{},
		Settings: &configtest.Mock{ResolveIntForTenantFn: func(_ context.Context, tenantID int64, key string) (int, error) {
			resolvedTenantID = tenantID
			resolvedKey = key
			return 15, nil
		}},
	})

	status, err := service.GetDeviceTransferStatus(context.Background(), device.ID)

	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, int64(10), resolvedTenantID)
	assert.Equal(t, configModel.KeyDeviceOnlineWindowMinutes, resolvedKey)
	// Seen 10 minutes ago is inside the tenant's 15-minute window.
	assert.True(t, status.IsOnline)
	assert.False(t, status.CanTransfer)
}

func TestOperatorProvisioningService_GetDeviceTransferStatus_OnlineWindowResolveErrorFallsBack(t *testing.T) {
	lastSeen := time.Now().Add(-10 * time.Minute)
	device := &iotModels.Device{Model: base.Model{ID: 200}, DeviceID: "BURBACH-2", LastSeen: &lastSeen}
	device.SetTenantID(10)

	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		DeviceRepo: &mockDeviceRepoWithFind{findByIDFn: func(context.Context, interface{}) (*iotModels.Device, error) {
			return device, nil
		}},
		ActiveGroupRepo: &mockActiveDeviceSessionRepo{},
		Settings: &configtest.Mock{ResolveIntForTenantFn: func(context.Context, int64, string) (int, error) {
			return 0, assert.AnError
		}},
	})

	status, err := service.GetDeviceTransferStatus(context.Background(), device.ID)

	require.NoError(t, err)
	require.NotNil(t, status)
	// Fallback window is 5 minutes; seen 10 minutes ago counts as offline.
	assert.False(t, status.IsOnline)
	assert.True(t, status.CanTransfer)
}

func TestOperatorProvisioningService_TransferDevice_ArchivesSourceAndPreservesIdentity(t *testing.T) {
	apiKey := "dev_existing-key"
	deviceName := "Burbach 2"
	source := &iotModels.Device{
		Model:      base.Model{ID: 200},
		DeviceID:   "BURBACH-2",
		DeviceType: "terminal",
		Name:       &deviceName,
		Status:     iotModels.DeviceStatusActive,
		APIKey:     &apiKey,
	}
	source.SetTenantID(10)

	var target *iotModels.Device
	var updateSnapshots []iotModels.Device
	var auditEntry *platformModels.OperatorAuditLog
	deviceRepo := &mockDeviceRepo{
		findByIDForUpdateFn: func(context.Context, int64) (*iotModels.Device, error) {
			return source, nil
		},
		updateFn: func(_ context.Context, device *iotModels.Device) error {
			updateSnapshots = append(updateSnapshots, *device)
			return nil
		},
		createFn: func(_ context.Context, device *iotModels.Device) error {
			device.ID = 201
			target = device
			return nil
		},
	}
	summaries := &mockSummariesRepo{devicesFn: func(_ context.Context, filter platformModels.OperatorDeviceFilter) ([]platformModels.OperatorDeviceRow, error) {
		require.NotNil(t, filter.DeviceRowID)
		assert.Equal(t, int64(201), *filter.DeviceRowID)
		return []platformModels.OperatorDeviceRow{{
			ID: 201, DeviceID: "BURBACH-2", DeviceType: "terminal", Name: &deviceName,
			Status: "active", APIKey: &apiKey, SchoolID: 20, SchoolName: "Walbach",
			OrganizationID: 1, OrganizationName: "Talent OGS", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}}, nil
	}}
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: summaries,
		DeviceRepo:    deviceRepo,
		SchoolRepo: &testpkg.SchoolRepoMock{FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
			return &platformModels.School{Model: base.Model{ID: id}, OrganizationID: 1, Name: map[int64]string{10: "Burbach", 20: "Walbach"}[id], Slug: fmt.Sprintf("school-%d", id), Subdomain: fmt.Sprintf("school-%d", id), Active: true}, nil
		}},
		ActiveGroupRepo: &mockActiveDeviceSessionRepo{},
		AuditLogRepo: &mockAuditLogRepoShared{createFn: func(_ context.Context, entry *platformModels.OperatorAuditLog) error {
			auditEntry = entry
			return nil
		}},
	})

	result, err := service.TransferDevice(context.Background(), 200, 20, 7, net.IPv4(127, 0, 0, 1))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(201), result.ID)
	require.NotNil(t, target)
	assert.Equal(t, int64(20), target.TenantID)
	assert.Equal(t, "BURBACH-2", target.DeviceID)
	require.NotNil(t, target.APIKey)
	assert.Equal(t, apiKey, *target.APIKey)
	assert.Nil(t, target.LastSeen)
	require.Len(t, updateSnapshots, 2)
	require.NotNil(t, updateSnapshots[0].ArchivedAt)
	assert.Nil(t, updateSnapshots[0].APIKey)
	assert.Equal(t, iotModels.DeviceStatusInactive, updateSnapshots[0].Status)
	require.NotNil(t, updateSnapshots[1].TransferredToDeviceID)
	assert.Equal(t, int64(201), *updateSnapshots[1].TransferredToDeviceID)
	require.NotNil(t, auditEntry)
	assert.Equal(t, platformModels.ActionTransfer, auditEntry.Action)
}

func TestOperatorProvisioningService_TransferDevice_RejectsDifferentOrganization(t *testing.T) {
	source := &iotModels.Device{Model: base.Model{ID: 200}, DeviceID: "BURBACH-2", DeviceType: "terminal", Status: iotModels.DeviceStatusActive}
	source.SetTenantID(10)
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		DeviceRepo: &mockDeviceRepo{findByIDForUpdateFn: func(context.Context, int64) (*iotModels.Device, error) {
			return source, nil
		}},
		SchoolRepo: &testpkg.SchoolRepoMock{FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
			return &platformModels.School{Model: base.Model{ID: id}, OrganizationID: id, Name: "School", Slug: fmt.Sprintf("school-%d", id), Subdomain: fmt.Sprintf("school-%d", id), Active: true}, nil
		}},
	})

	result, err := service.TransferDevice(context.Background(), 200, 20, 7, nil)
	require.Nil(t, result)
	var mismatch *platformSvc.DeviceTransferOrganizationMismatchError
	require.ErrorAs(t, err, &mismatch)
}

func TestOperatorProvisioningService_TransferDevice_OnlineDeviceDoesNotWrite(t *testing.T) {
	now := time.Now()
	source := &iotModels.Device{
		Model:      base.Model{ID: 200},
		DeviceID:   "BURBACH-2",
		DeviceType: "terminal",
		Status:     iotModels.DeviceStatusActive,
		LastSeen:   &now,
	}
	source.SetTenantID(10)
	writes := 0
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		DeviceRepo: &mockDeviceRepo{
			findByIDForUpdateFn: func(context.Context, int64) (*iotModels.Device, error) { return source, nil },
			updateFn: func(context.Context, *iotModels.Device) error {
				writes++
				return nil
			},
			createFn: func(context.Context, *iotModels.Device) error {
				writes++
				return nil
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
			return &platformModels.School{Model: base.Model{ID: id}, OrganizationID: 1, Name: "School", Slug: fmt.Sprintf("school-%d", id), Subdomain: fmt.Sprintf("school-%d", id), Active: true}, nil
		}},
		ActiveGroupRepo: &mockActiveDeviceSessionRepo{},
	})

	result, err := service.TransferDevice(context.Background(), 200, 20, 7, nil)

	require.Nil(t, result)
	var blocked *platformSvc.DeviceTransferBlockedError
	require.ErrorAs(t, err, &blocked)
	assert.Equal(t, platformSvc.DeviceTransferBlockedOnline, blocked.Reason)
	assert.Zero(t, writes)
}

func TestOperatorProvisioningService_TransferDevice_RejectsSameSchool(t *testing.T) {
	source := &iotModels.Device{Model: base.Model{ID: 200}, DeviceID: "BURBACH-2"}
	source.SetTenantID(10)
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		DeviceRepo: &mockDeviceRepo{findByIDForUpdateFn: func(context.Context, int64) (*iotModels.Device, error) {
			return source, nil
		}},
	})

	result, err := service.TransferDevice(context.Background(), source.ID, source.TenantID, 7, nil)

	require.Nil(t, result)
	var sameSchool *platformSvc.DeviceTransferSameSchoolError
	require.ErrorAs(t, err, &sameSchool)
}

func TestOperatorProvisioningService_TransferDevice_RejectsProtectedDevice(t *testing.T) {
	source := &iotModels.Device{Model: base.Model{ID: 200}, DeviceID: iotModels.WebManualDeviceID}
	source.SetTenantID(10)
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		DeviceRepo: &mockDeviceRepo{findByIDForUpdateFn: func(context.Context, int64) (*iotModels.Device, error) {
			return source, nil
		}},
	})

	result, err := service.TransferDevice(context.Background(), source.ID, 20, 7, nil)

	require.Nil(t, result)
	var protected *platformSvc.DeviceTransferProtectedError
	require.ErrorAs(t, err, &protected)
}

func TestOperatorProvisioningService_TransferDevice_RejectsInvalidIDs(t *testing.T) {
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
	})

	result, err := service.TransferDevice(context.Background(), 0, 20, 7, nil)

	require.Nil(t, result)
	var invalidData *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalidData)
}

func TestOperatorProvisioningService_TransferDevice_ActiveSessionDoesNotWrite(t *testing.T) {
	source := &iotModels.Device{Model: base.Model{ID: 200}, DeviceID: "BURBACH-2"}
	source.SetTenantID(10)
	writes := 0
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		DeviceRepo: &mockDeviceRepo{
			findByIDForUpdateFn: func(context.Context, int64) (*iotModels.Device, error) { return source, nil },
			updateFn: func(context.Context, *iotModels.Device) error {
				writes++
				return nil
			},
			createFn: func(context.Context, *iotModels.Device) error {
				writes++
				return nil
			},
		},
		SchoolRepo: &testpkg.SchoolRepoMock{FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
			return &platformModels.School{Model: base.Model{ID: id}, OrganizationID: 1, Name: "School", Slug: fmt.Sprintf("school-%d", id), Subdomain: fmt.Sprintf("school-%d", id), Active: true}, nil
		}},
		ActiveGroupRepo: &mockActiveDeviceSessionRepo{findFn: func(context.Context, int64) (*activeModels.Group, error) {
			return &activeModels.Group{Model: base.Model{ID: 300}, StartTime: time.Now().Add(-time.Hour)}, nil
		}},
	})

	result, err := service.TransferDevice(context.Background(), source.ID, 20, 7, nil)

	require.Nil(t, result)
	var blocked *platformSvc.DeviceTransferBlockedError
	require.ErrorAs(t, err, &blocked)
	assert.Equal(t, platformSvc.DeviceTransferBlockedActiveSession, blocked.Reason)
	assert.Zero(t, writes)
}

// TestOperatorProvisioningService_LoadActiveSchool_RejectsDeletedSchool tests
// the private loadActiveSchool method indirectly via CreateSchoolAccount, which
// calls loadActiveSchool as its first step.
func TestOperatorProvisioningService_LoadActiveSchool_RejectsDeletedSchool(t *testing.T) {
	now := time.Now()
	service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{},
		SchoolRepo: &testpkg.SchoolRepoMock{
			FindByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          base.Model{ID: id},
					OrganizationID: 100,
					Name:           "Deleted School",
					Slug:           "deleted-school",
					Subdomain:      "deleted-school",
					Active:         true,
					DeletedAt:      &now,
				}, nil
			},
		},
		RoleRepo:     &mockRoleRepo{},
		AuditLogRepo: &mockAuditLogRepoShared{},
	})

	account, err := service.CreateSchoolAccount(context.Background(), 500, 10, net.IPv4(127, 0, 0, 1), platformSvc.CreateSchoolAccountRequest{
		Email:     "test@example.com",
		Password:  "SecureP@ss123!",
		FirstName: "Test",
		LastName:  "User",
	})
	require.Nil(t, account)
	require.Error(t, err)
	var deletedErr *platformSvc.SchoolAlreadyDeletedError
	require.ErrorAs(t, err, &deletedErr)
}

// Stub for the issue #585 refactor interface addition — unused here.
func (m *mockPersonRepo) AnonymizeAndSoftDelete(context.Context, int64) error { return nil }

// Stub for the issue #585 refactor interface addition — unused here.
func (m *mockSummariesRepo) ListDeviceRows(ctx context.Context, filter platformModels.OperatorDeviceFilter) ([]platformModels.OperatorDeviceRow, error) {
	if m.devicesFn != nil {
		return m.devicesFn(ctx, filter)
	}
	return nil, nil
}

func (m *mockTeacherRepo) ListActiveCaregivers(context.Context) ([]*userModels.ActiveCaregiver, error) {
	return nil, nil
}

func (m *mockTeacherRepo) FindActiveCaregiverByAccountID(context.Context, int64) (*userModels.ActiveCaregiver, error) {
	return nil, nil
}
