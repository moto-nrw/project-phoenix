package platform

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/auth/authtest"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

type internalOrgRepoStub struct {
	findByIDFn   func(context.Context, int64) (*platformModels.Organization, error)
	findBySlugFn func(context.Context, string) (*platformModels.Organization, error)
	createFn     func(context.Context, *platformModels.Organization) error
	listFn       func(context.Context) ([]*platformModels.Organization, error)
	softDeleteFn func(context.Context, int64) error
	restoreFn    func(context.Context, int64) error
}

func (s *internalOrgRepoStub) Create(ctx context.Context, org *platformModels.Organization) error {
	if s.createFn != nil {
		return s.createFn(ctx, org)
	}
	return nil
}
func (s *internalOrgRepoStub) FindByID(ctx context.Context, id int64) (*platformModels.Organization, error) {
	if s.findByIDFn != nil {
		return s.findByIDFn(ctx, id)
	}
	return nil, nil
}
func (s *internalOrgRepoStub) FindByIDForShare(ctx context.Context, id int64) (*platformModels.Organization, error) {
	return s.FindByID(ctx, id)
}
func (s *internalOrgRepoStub) FindByIDForUpdate(ctx context.Context, id int64) (*platformModels.Organization, error) {
	return s.FindByID(ctx, id)
}
func (s *internalOrgRepoStub) FindBySlug(ctx context.Context, slug string) (*platformModels.Organization, error) {
	if s.findBySlugFn != nil {
		return s.findBySlugFn(ctx, slug)
	}
	return nil, nil
}
func (s *internalOrgRepoStub) List(ctx context.Context) ([]*platformModels.Organization, error) {
	if s.listFn != nil {
		return s.listFn(ctx)
	}
	return nil, nil
}
func (s *internalOrgRepoStub) Update(context.Context, *platformModels.Organization) error {
	return nil
}
func (s *internalOrgRepoStub) CountByIDs(context.Context, []int64) (int, error) {
	return 0, nil
}
func (s *internalOrgRepoStub) SoftDelete(ctx context.Context, id int64) error {
	if s.softDeleteFn != nil {
		return s.softDeleteFn(ctx, id)
	}
	return nil
}
func (s *internalOrgRepoStub) Restore(ctx context.Context, id int64) error {
	if s.restoreFn != nil {
		return s.restoreFn(ctx, id)
	}
	return nil
}

type internalDeviceRepoStub struct {
	createFn   func(context.Context, *iotModels.Device) error
	findByIDFn func(context.Context, interface{}) (*iotModels.Device, error)
	updateFn   func(context.Context, *iotModels.Device) error
	deleteFn   func(context.Context, interface{}) error
}

func (s *internalDeviceRepoStub) Create(ctx context.Context, device *iotModels.Device) error {
	if s.createFn != nil {
		return s.createFn(ctx, device)
	}
	return nil
}
func (s *internalDeviceRepoStub) FindByID(ctx context.Context, id interface{}) (*iotModels.Device, error) {
	if s.findByIDFn != nil {
		return s.findByIDFn(ctx, id)
	}
	return nil, nil
}
func (s *internalDeviceRepoStub) FindByIDForUpdate(ctx context.Context, id int64) (*iotModels.Device, error) {
	return s.FindByID(ctx, id)
}
func (s *internalDeviceRepoStub) Update(ctx context.Context, device *iotModels.Device) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, device)
	}
	return nil
}
func (s *internalDeviceRepoStub) Delete(ctx context.Context, id interface{}) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}
func (s *internalDeviceRepoStub) List(context.Context, map[string]interface{}) ([]*iotModels.Device, error) {
	return nil, nil
}
func (s *internalDeviceRepoStub) FindByDeviceID(context.Context, string) (*iotModels.Device, error) {
	return nil, nil
}
func (s *internalDeviceRepoStub) FindByAPIKey(context.Context, string) (*iotModels.Device, error) {
	return nil, nil
}
func (s *internalDeviceRepoStub) FindByType(context.Context, string) ([]*iotModels.Device, error) {
	return nil, nil
}
func (s *internalDeviceRepoStub) FindByStatus(context.Context, iotModels.DeviceStatus) ([]*iotModels.Device, error) {
	return nil, nil
}
func (s *internalDeviceRepoStub) FindByRegisteredBy(context.Context, int64) ([]*iotModels.Device, error) {
	return nil, nil
}
func (s *internalDeviceRepoStub) UpdateLastSeen(context.Context, int64, time.Time) error {
	return nil
}
func (s *internalDeviceRepoStub) UpdateRoomID(context.Context, int64, int64) error {
	return nil
}
func (s *internalDeviceRepoStub) UpdateStatus(context.Context, string, iotModels.DeviceStatus) error {
	return nil
}
func (s *internalDeviceRepoStub) FindOfflineDevices(context.Context, time.Duration) ([]*iotModels.Device, error) {
	return nil, nil
}
func (s *internalDeviceRepoStub) CountDevicesByType(context.Context) (map[string]int, error) {
	return nil, nil
}

type internalCategoryRepoStub struct {
	createFn func(context.Context, *activityModels.Category) error
}

func (s *internalCategoryRepoStub) Create(ctx context.Context, cat *activityModels.Category) error {
	if s.createFn != nil {
		return s.createFn(ctx, cat)
	}
	return nil
}
func (s *internalCategoryRepoStub) FindByID(context.Context, interface{}) (*activityModels.Category, error) {
	return nil, nil
}
func (s *internalCategoryRepoStub) FindByIDForShare(context.Context, int64) (*activityModels.Category, error) {
	return nil, nil
}
func (s *internalCategoryRepoStub) Update(context.Context, *activityModels.Category) error {
	return nil
}
func (s *internalCategoryRepoStub) UpdateIfActive(context.Context, *activityModels.Category) (bool, error) {
	return true, nil
}
func (s *internalCategoryRepoStub) Delete(context.Context, interface{}) error { return nil }
func (s *internalCategoryRepoStub) List(context.Context, *modelBase.QueryOptions) ([]*activityModels.Category, error) {
	return nil, nil
}
func (s *internalCategoryRepoStub) FindByName(context.Context, string) (*activityModels.Category, error) {
	return nil, nil
}
func (s *internalCategoryRepoStub) FindByNameIncludingArchivedForShare(context.Context, string) (*activityModels.Category, error) {
	return nil, nil
}
func (s *internalCategoryRepoStub) ListAll(context.Context) ([]*activityModels.Category, error) {
	return nil, nil
}

func (s *internalCategoryRepoStub) SetShiftTypeForCategories(context.Context, int64, []int64) error {
	return nil
}

func (s *internalCategoryRepoStub) UpdateColumns(context.Context, *activityModels.Category, ...string) (int64, error) {
	return 1, nil
}

type internalAuditLogRepoStub struct {
	createFn func(context.Context, *platformModels.OperatorAuditLog) error
}

func (s *internalAuditLogRepoStub) Create(ctx context.Context, entry *platformModels.OperatorAuditLog) error {
	if s.createFn != nil {
		return s.createFn(ctx, entry)
	}
	return nil
}
func (s *internalAuditLogRepoStub) FindByOperatorID(context.Context, int64, int) ([]*platformModels.OperatorAuditLog, error) {
	return nil, nil
}
func (s *internalAuditLogRepoStub) FindByResourceType(context.Context, string, int) ([]*platformModels.OperatorAuditLog, error) {
	return nil, nil
}
func (s *internalAuditLogRepoStub) FindByDateRange(context.Context, time.Time, time.Time, int) ([]*platformModels.OperatorAuditLog, error) {
	return nil, nil
}

type internalRoleRepoStub struct {
	listFn func(context.Context, map[string]interface{}) ([]*authModels.Role, error)
}

func (s *internalRoleRepoStub) Create(context.Context, *authModels.Role) error { return nil }
func (s *internalRoleRepoStub) FindByID(context.Context, interface{}) (*authModels.Role, error) {
	return nil, nil
}
func (s *internalRoleRepoStub) Update(context.Context, *authModels.Role) error { return nil }
func (s *internalRoleRepoStub) Delete(context.Context, interface{}) error      { return nil }
func (s *internalRoleRepoStub) List(ctx context.Context, filters map[string]interface{}) ([]*authModels.Role, error) {
	if s.listFn != nil {
		return s.listFn(ctx, filters)
	}
	return nil, nil
}
func (s *internalRoleRepoStub) FindByName(context.Context, string) (*authModels.Role, error) {
	return nil, nil
}
func (s *internalRoleRepoStub) FindByAccountID(context.Context, int64) ([]*authModels.Role, error) {
	return nil, nil
}
func (s *internalRoleRepoStub) FindRoleNamesByAccountIDs(context.Context, []int64) (map[int64]string, error) {
	return nil, nil
}
func (s *internalRoleRepoStub) AssignRoleToAccount(context.Context, int64, int64) error   { return nil }
func (s *internalRoleRepoStub) RemoveRoleFromAccount(context.Context, int64, int64) error { return nil }
func (s *internalRoleRepoStub) GetRoleWithPermissions(context.Context, int64) (*authModels.Role, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newPgError constructs a pgdriver.Error with the given SQLSTATE code.
// Uses reflect+unsafe because pgdriver.Error has no exported constructor.
func newPgError(code string) error {
	pgErr := pgdriver.Error{}
	v := reflect.ValueOf(&pgErr).Elem()
	mField := v.FieldByName("m")
	ptr := unsafe.Pointer(mField.UnsafeAddr()) //nolint:gosec
	*(*map[byte]string)(ptr) = map[byte]string{'C': code}
	return pgErr
}

// newPgErrorWithConstraint constructs a pgdriver.Error with SQLSTATE and constraint name.
// Field 'C' = SQLSTATE code, field 'n' = constraint name (PostgreSQL protocol).
func newPgErrorWithConstraint(code, constraintName string) error {
	pgErr := pgdriver.Error{}
	v := reflect.ValueOf(&pgErr).Elem()
	mField := v.FieldByName("m")
	ptr := unsafe.Pointer(mField.UnsafeAddr()) //nolint:gosec
	*(*map[byte]string)(ptr) = map[byte]string{'C': code, 'n': constraintName}
	return pgErr
}

// ---------------------------------------------------------------------------
// Existing tests (preserved)
// ---------------------------------------------------------------------------

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
	err := mapSchoolCreateConflict(context.Background(), &testpkg.SchoolRepoMock{}, nil)
	require.Error(t, err)
	var conflictErr *ConflictError
	require.ErrorAs(t, err, &conflictErr)
}

func TestMapSchoolCreateConflict_SlugConflict(t *testing.T) {
	err := mapSchoolCreateConflict(context.Background(), &testpkg.SchoolRepoMock{
		FindByOrganizationAndSlugFn: func(context.Context, int64, string) (*platformModels.School, error) {
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
	err := mapSchoolCreateConflict(context.Background(), &testpkg.SchoolRepoMock{
		FindBySubdomainFn: func(context.Context, string) (*platformModels.School, error) {
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
	err := tenant.WithAdminTxOrDirect(context.Background(), service.adminDB(), func(context.Context) error {
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

// ---------------------------------------------------------------------------
// createWebManualDevice
// ---------------------------------------------------------------------------

func TestCreateWebManualDevice_NilDeviceRepo(t *testing.T) {
	svc := &operatorProvisioningService{}
	require.NoError(t, svc.createWebManualDevice(context.Background(), 1))
}

func TestCreateWebManualDevice_ZeroTenantID(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{DeviceRepo: &internalDeviceRepoStub{}}}
	require.NoError(t, svc.createWebManualDevice(context.Background(), 0))
}

func TestCreateWebManualDevice_NegativeTenantID(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{DeviceRepo: &internalDeviceRepoStub{}}}
	require.NoError(t, svc.createWebManualDevice(context.Background(), -1))
}

func TestCreateWebManualDevice_Success(t *testing.T) {
	var created *iotModels.Device
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{DeviceRepo: &internalDeviceRepoStub{
		createFn: func(_ context.Context, device *iotModels.Device) error {
			created = device
			return nil
		},
	}, Logger: slog.Default()},
	}

	err := svc.createWebManualDevice(context.Background(), 42)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "WEB-MANUAL-001", created.DeviceID)
	assert.Equal(t, "virtual", created.DeviceType)
	assert.Equal(t, iotModels.DeviceStatusActive, created.Status)
	require.NotNil(t, created.Name)
	assert.Equal(t, "Web-Portal (Manuell)", *created.Name)
}

func TestCreateWebManualDevice_UniqueViolation(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{DeviceRepo: &internalDeviceRepoStub{
		createFn: func(context.Context, *iotModels.Device) error {
			return newPgError("23505")
		},
	}},
	}
	require.NoError(t, svc.createWebManualDevice(context.Background(), 42))
}

func TestCreateWebManualDevice_CreateError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{DeviceRepo: &internalDeviceRepoStub{
		createFn: func(context.Context, *iotModels.Device) error {
			return assert.AnError
		},
	}},
	}
	err := svc.createWebManualDevice(context.Background(), 42)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create web manual device for tenant 42")
	assert.ErrorIs(t, err, assert.AnError)
}

// ---------------------------------------------------------------------------
// seedDefaultActivityCategories
// ---------------------------------------------------------------------------

func TestSeedDefaultActivityCategories_NilCategoryRepo(t *testing.T) {
	svc := &operatorProvisioningService{}
	require.NoError(t, svc.seedDefaultActivityCategories(context.Background(), 1))
}

func TestSeedDefaultActivityCategories_ZeroTenantID(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{CategoryRepo: &internalCategoryRepoStub{}}}
	require.NoError(t, svc.seedDefaultActivityCategories(context.Background(), 0))
}

func TestSeedDefaultActivityCategories_UniqueViolationSkipped(t *testing.T) {
	var count int
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{CategoryRepo: &internalCategoryRepoStub{
		createFn: func(context.Context, *activityModels.Category) error {
			count++
			if count <= 2 {
				return newPgError("23505")
			}
			return nil
		},
	}},
	}

	err := svc.seedDefaultActivityCategories(context.Background(), 1)
	require.NoError(t, err)
	// 10 since issue #2131 added "Mensa" to the default list; the assertion
	// pins "every default is attempted even after a unique violation", not the
	// list length itself — TestSeedDefaultActivityCategories_IncludesMensa
	// covers the content.
	assert.Equal(t, 10, count)
}

// TestSeedDefaultActivityCategories_IncludesMensa pins the Essenszeiten
// requirement from issue #2131: a newly provisioned school must come with a
// "Mensa" category, otherwise Termine for lunch have no fitting
// Pflichtkategorie to select.
func TestSeedDefaultActivityCategories_IncludesMensa(t *testing.T) {
	var seeded []string
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{CategoryRepo: &internalCategoryRepoStub{
		createFn: func(_ context.Context, category *activityModels.Category) error {
			seeded = append(seeded, category.Name)
			return nil
		},
	}}}

	require.NoError(t, svc.seedDefaultActivityCategories(context.Background(), 42))
	assert.Contains(t, seeded, "Mensa")
}

// ---------------------------------------------------------------------------
// isUniqueViolation
// ---------------------------------------------------------------------------

func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"generic error", assert.AnError, false},
		{"pg 23505 unique violation", newPgError("23505"), true},
		{"pg 23503 foreign key (not unique)", newPgError("23503"), false},
		{"pg 42000 syntax error", newPgError("42000"), false},
		{
			"DatabaseError wrapping pg 23505",
			&modelBase.DatabaseError{Op: "create", Err: newPgError("23505")},
			true,
		},
		{
			"DatabaseError wrapping generic",
			&modelBase.DatabaseError{Op: "create", Err: assert.AnError},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, modelBase.IsUniqueViolation(tt.err))
		})
	}
}

// ---------------------------------------------------------------------------
// resolveSystemRoleByName
// ---------------------------------------------------------------------------

func TestResolveSystemRoleByName_NilRoleSkipped(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{RoleRepo: &internalRoleRepoStub{
		listFn: func(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
			return []*authModels.Role{nil}, nil
		},
	}},
	}
	role, err := authSvc.ResolveSystemRoleByName(context.Background(), svc.RoleRepo, "admin")
	require.NoError(t, err)
	assert.Nil(t, role)
}

func TestResolveSystemRoleByName_NameMismatch(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{RoleRepo: &internalRoleRepoStub{
		listFn: func(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
			return []*authModels.Role{
				{Model: modelBase.Model{ID: 1}, Name: "teacher", IsSystem: true},
			}, nil
		},
	}},
	}
	role, err := authSvc.ResolveSystemRoleByName(context.Background(), svc.RoleRepo, "admin")
	require.NoError(t, err)
	assert.Nil(t, role)
}

func TestResolveSystemRoleByName_TenantSpecificSkipped(t *testing.T) {
	tenantID := int64(5)
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{RoleRepo: &internalRoleRepoStub{
		listFn: func(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
			return []*authModels.Role{
				{Model: modelBase.Model{ID: 1}, Name: "admin", IsSystem: true, TenantID: &tenantID},
			}, nil
		},
	}},
	}
	role, err := authSvc.ResolveSystemRoleByName(context.Background(), svc.RoleRepo, "admin")
	require.NoError(t, err)
	assert.Nil(t, role)
}

func TestResolveSystemRoleByName_NonSystemSkipped(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{RoleRepo: &internalRoleRepoStub{
		listFn: func(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
			return []*authModels.Role{
				{Model: modelBase.Model{ID: 1}, Name: "admin", IsSystem: false},
			}, nil
		},
	}},
	}
	role, err := authSvc.ResolveSystemRoleByName(context.Background(), svc.RoleRepo, "admin")
	require.NoError(t, err)
	assert.Nil(t, role)
}

func TestResolveSystemRoleByName_RepoError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{RoleRepo: &internalRoleRepoStub{
		listFn: func(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
			return nil, assert.AnError
		},
	}},
	}
	role, err := authSvc.ResolveSystemRoleByName(context.Background(), svc.RoleRepo, "admin")
	require.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, role)
}

func TestResolveSystemRoleByName_Success(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{RoleRepo: &internalRoleRepoStub{
		listFn: func(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
			return []*authModels.Role{
				{Model: modelBase.Model{ID: 7}, Name: "admin", IsSystem: true},
			}, nil
		},
	}},
	}
	role, err := authSvc.ResolveSystemRoleByName(context.Background(), svc.RoleRepo, "admin")
	require.NoError(t, err)
	require.NotNil(t, role)
	assert.Equal(t, int64(7), role.ID)
}

func TestListSystemRoles_Error(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{RoleRepo: &internalRoleRepoStub{
		listFn: func(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
			return nil, assert.AnError
		},
	}},
	}

	roles, err := svc.ListSystemRoles(context.Background())
	require.Nil(t, roles)
	require.ErrorIs(t, err, assert.AnError)
}

// ---------------------------------------------------------------------------
// logAction
// ---------------------------------------------------------------------------

func TestLogAction_AuditLogError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{AuditLogRepo: &internalAuditLogRepoStub{
		createFn: func(context.Context, *platformModels.OperatorAuditLog) error {
			return assert.AnError
		},
	}, Logger: slog.Default()},
	}
	resourceID := int64(1)
	// Must not panic; error is logged but not returned.
	svc.logAction(context.Background(), 1, "create", "school", &resourceID, net.IPv4(127, 0, 0, 1), map[string]any{
		"name": "test",
	})
}

func TestLogAction_EmptyChanges(t *testing.T) {
	var entry *platformModels.OperatorAuditLog
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{AuditLogRepo: &internalAuditLogRepoStub{
		createFn: func(_ context.Context, e *platformModels.OperatorAuditLog) error {
			entry = e
			return nil
		},
	}},
	}
	resourceID := int64(1)
	svc.logAction(context.Background(), 1, "create", "school", &resourceID, net.IPv4(127, 0, 0, 1), nil)
	require.NotNil(t, entry)
	assert.Nil(t, entry.Changes)
}

func TestLogAction_WithChanges(t *testing.T) {
	var entry *platformModels.OperatorAuditLog
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{AuditLogRepo: &internalAuditLogRepoStub{
		createFn: func(_ context.Context, e *platformModels.OperatorAuditLog) error {
			entry = e
			return nil
		},
	}},
	}
	resourceID := int64(1)
	svc.logAction(context.Background(), 1, "create", "school", &resourceID, net.IPv4(127, 0, 0, 1), map[string]any{
		"name": "test",
	})
	require.NotNil(t, entry)
	assert.NotNil(t, entry.Changes)
}

// ---------------------------------------------------------------------------
// mapSchoolCreateConflict — generic fallthrough
// ---------------------------------------------------------------------------

func TestMapSchoolCreateConflict_GenericFallthrough(t *testing.T) {
	err := mapSchoolCreateConflict(context.Background(), &testpkg.SchoolRepoMock{}, &platformModels.School{
		OrganizationID: 2,
		Name:           "School",
		Slug:           "school",
		Subdomain:      "school",
	})
	require.Error(t, err)
	var conflictErr *ConflictError
	require.ErrorAs(t, err, &conflictErr)
	assert.Contains(t, err.Error(), "school already exists")
}

// ---------------------------------------------------------------------------
// loadActiveSchool
// ---------------------------------------------------------------------------

func TestLoadActiveSchool_SqlErrNoRows(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
			return nil, sql.ErrNoRows
		},
	}},
	}
	school, err := svc.loadActiveSchool(context.Background(), 1)
	require.Nil(t, school)
	var notFoundErr *SchoolNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestLoadActiveSchool_DatabaseError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
			return nil, &modelBase.DatabaseError{Op: "find", Err: sql.ErrNoRows}
		},
	}},
	}
	school, err := svc.loadActiveSchool(context.Background(), 1)
	require.Nil(t, school)
	var notFoundErr *SchoolNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestLoadActiveSchool_GenericError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
			return nil, assert.AnError
		},
	}},
	}
	school, err := svc.loadActiveSchool(context.Background(), 1)
	require.Nil(t, school)
	require.ErrorIs(t, err, assert.AnError)
}

// ---------------------------------------------------------------------------
// CreateOrganization — error paths
// ---------------------------------------------------------------------------

func TestCreateOrganization_NilInput(t *testing.T) {
	svc := &operatorProvisioningService{}
	org, err := svc.CreateOrganization(context.Background(), nil, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, org)
	var invalidErr *InvalidDataError
	require.ErrorAs(t, err, &invalidErr)
}

func TestCreateOrganization_ValidationError(t *testing.T) {
	svc := &operatorProvisioningService{}
	org, err := svc.CreateOrganization(context.Background(), &platformModels.Organization{
		Name: "",
		Slug: "test",
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, org)
	var invalidErr *InvalidDataError
	require.ErrorAs(t, err, &invalidErr)
}

func TestCreateOrganization_FindBySlugError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{OrganizationRepo: &internalOrgRepoStub{
		findBySlugFn: func(context.Context, string) (*platformModels.Organization, error) {
			return nil, assert.AnError
		},
	}},
	}
	org, err := svc.CreateOrganization(context.Background(), &platformModels.Organization{
		Name: "Test", Slug: "test", Active: true,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, org)
	require.ErrorIs(t, err, assert.AnError)
}

func TestCreateOrganization_UniqueViolationOnCreate(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{OrganizationRepo: &internalOrgRepoStub{
		findBySlugFn: func(context.Context, string) (*platformModels.Organization, error) {
			return nil, nil
		},
		createFn: func(context.Context, *platformModels.Organization) error {
			return newPgError("23505")
		},
	}},
	}
	org, err := svc.CreateOrganization(context.Background(), &platformModels.Organization{
		Name: "Test", Slug: "test", Active: true,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, org)
	var conflictErr *ConflictError
	require.ErrorAs(t, err, &conflictErr)
}

func TestCreateOrganization_CreateError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{OrganizationRepo: &internalOrgRepoStub{
		findBySlugFn: func(context.Context, string) (*platformModels.Organization, error) {
			return nil, nil
		},
		createFn: func(context.Context, *platformModels.Organization) error {
			return assert.AnError
		},
	}},
	}
	org, err := svc.CreateOrganization(context.Background(), &platformModels.Organization{
		Name: "Test", Slug: "test", Active: true,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, org)
	require.ErrorIs(t, err, assert.AnError)
}

// ---------------------------------------------------------------------------
// CreateSchool — error paths
// ---------------------------------------------------------------------------

func TestCreateSchool_NilInput(t *testing.T) {
	svc := &operatorProvisioningService{}
	school, err := svc.CreateSchool(context.Background(), nil, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	var invalidErr *InvalidDataError
	require.ErrorAs(t, err, &invalidErr)
}

func TestCreateSchool_ValidationError(t *testing.T) {
	svc := &operatorProvisioningService{}
	school, err := svc.CreateSchool(context.Background(), &platformModels.School{
		Name:           "",
		Slug:           "test",
		Subdomain:      "test",
		OrganizationID: 1,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	var invalidErr *InvalidDataError
	require.ErrorAs(t, err, &invalidErr)
}

func TestCreateSchool_OrgFindByIDError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{OrganizationRepo: &internalOrgRepoStub{
		findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
			return nil, assert.AnError
		},
	}, SchoolRepo: &testpkg.SchoolRepoMock{}},
	}
	school, err := svc.CreateSchool(context.Background(), &platformModels.School{
		OrganizationID: 1,
		Name:           "Test",
		Slug:           "test",
		Subdomain:      "test",
		Active:         true,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	require.ErrorIs(t, err, assert.AnError)
}

func TestCreateSchool_UniqueViolationOnCreate(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{OrganizationRepo: &internalOrgRepoStub{
		findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
			return &platformModels.Organization{Model: modelBase.Model{ID: 1}, Name: "Org", Slug: "org", Active: true}, nil
		},
	}, SchoolRepo: &testpkg.SchoolRepoMock{
		CreateFn: func(context.Context, *platformModels.School) error {
			return newPgError("23505")
		},
	}},
	}
	school, err := svc.CreateSchool(context.Background(), &platformModels.School{
		OrganizationID: 1,
		Name:           "Test",
		Slug:           "test",
		Subdomain:      "test",
		Active:         true,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	var conflictErr *ConflictError
	require.ErrorAs(t, err, &conflictErr)
}

func TestCreateSchool_DeviceCreateError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{OrganizationRepo: &internalOrgRepoStub{
		findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
			return &platformModels.Organization{Model: modelBase.Model{ID: 1}, Name: "Org", Slug: "org", Active: true}, nil
		},
	}, SchoolRepo: &testpkg.SchoolRepoMock{
		CreateFn: func(_ context.Context, school *platformModels.School) error {
			school.ID = 55
			return nil
		},
	}, CategoryRepo: &internalCategoryRepoStub{}, DeviceRepo: &internalDeviceRepoStub{
		createFn: func(context.Context, *iotModels.Device) error {
			return assert.AnError
		},
	}},
	}
	school, err := svc.CreateSchool(context.Background(), &platformModels.School{
		OrganizationID: 1,
		Name:           "Test",
		Slug:           "test",
		Subdomain:      "test",
		Active:         true,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create web manual device for tenant 55")
}

func TestCreateSchool_WithDeviceSuccess(t *testing.T) {
	var deviceCreated bool
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{OrganizationRepo: &internalOrgRepoStub{
		findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
			return &platformModels.Organization{Model: modelBase.Model{ID: 1}, Name: "Org", Slug: "org", Active: true}, nil
		},
	}, SchoolRepo: &testpkg.SchoolRepoMock{
		CreateFn: func(_ context.Context, school *platformModels.School) error {
			school.ID = 55
			return nil
		},
	}, CategoryRepo: &internalCategoryRepoStub{}, DeviceRepo: &internalDeviceRepoStub{
		createFn: func(context.Context, *iotModels.Device) error {
			deviceCreated = true
			return nil
		},
	}, AuditLogRepo: &internalAuditLogRepoStub{}, Logger: slog.Default()},
	}
	school, err := svc.CreateSchool(context.Background(), &platformModels.School{
		OrganizationID: 1,
		Name:           "Test",
		Slug:           "test",
		Subdomain:      "test",
		Active:         true,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.NoError(t, err)
	require.NotNil(t, school)
	assert.Equal(t, int64(55), school.ID)
	assert.True(t, deviceCreated)
}

// ---------------------------------------------------------------------------
// InviteSchoolAdmin — error paths
// ---------------------------------------------------------------------------

func TestInviteSchoolAdmin_RoleRepoError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
			return &platformModels.School{
				Model:          modelBase.Model{ID: 9},
				OrganizationID: 3,
				Name:           "School",
				Slug:           "school",
				Subdomain:      "school",
				Active:         true,
			}, nil
		},
	}, RoleRepo: &internalRoleRepoStub{
		listFn: func(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
			return nil, assert.AnError
		},
	}},
	}
	invitation, err := svc.InviteSchoolAdmin(context.Background(), 9, 11, net.IPv4(127, 0, 0, 1), authSvc.InvitationRequest{
		Email: "test@example.com",
	})
	require.Nil(t, invitation)
	require.ErrorIs(t, err, assert.AnError)
}

func TestInviteSchoolAdmin_InvitationCreateError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
			return &platformModels.School{
				Model:          modelBase.Model{ID: 9},
				OrganizationID: 3,
				Name:           "School",
				Slug:           "school",
				Subdomain:      "school",
				Active:         true,
			}, nil
		},
	}, RoleRepo: &internalRoleRepoStub{
		listFn: func(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
			return []*authModels.Role{
				{Model: modelBase.Model{ID: 4}, Name: "admin", IsSystem: true},
			}, nil
		},
	}, InvitationService: &authtest.InvitationServiceMock{
		CreateInvitationFn: func(context.Context, authSvc.InvitationRequest) (*authModels.InvitationToken, error) {
			return nil, assert.AnError
		},
	}},
	}
	invitation, err := svc.InviteSchoolAdmin(context.Background(), 9, 11, net.IPv4(127, 0, 0, 1), authSvc.InvitationRequest{
		Email: "test@example.com",
	})
	require.Nil(t, invitation)
	require.ErrorIs(t, err, assert.AnError)
}

// ---------------------------------------------------------------------------
// ensureSchoolSlugAvailable / ensureSchoolSubdomainAvailable — repo errors
// ---------------------------------------------------------------------------

func TestEnsureSchoolSlugAvailable_RepoError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SchoolRepo: &testpkg.SchoolRepoMock{
		FindByOrganizationAndSlugFn: func(context.Context, int64, string) (*platformModels.School, error) {
			return nil, assert.AnError
		},
	}},
	}
	err := svc.ensureSchoolSlugAvailable(context.Background(), 1, "test")
	require.ErrorIs(t, err, assert.AnError)
}

func TestEnsureSchoolSubdomainAvailable_RepoError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SchoolRepo: &testpkg.SchoolRepoMock{
		FindBySubdomainFn: func(context.Context, string) (*platformModels.School, error) {
			return nil, assert.AnError
		},
	}},
	}
	err := svc.ensureSchoolSubdomainAvailable(context.Background(), "test")
	require.ErrorIs(t, err, assert.AnError)
}

// ---------------------------------------------------------------------------
// CreateSchool — non-unique create error
// ---------------------------------------------------------------------------

func TestCreateSchool_CreateNonUniqueError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{OrganizationRepo: &internalOrgRepoStub{
		findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
			return &platformModels.Organization{Model: modelBase.Model{ID: 1}, Name: "Org", Slug: "org", Active: true}, nil
		},
	}, SchoolRepo: &testpkg.SchoolRepoMock{
		CreateFn: func(context.Context, *platformModels.School) error {
			return assert.AnError
		},
	}},
	}
	school, err := svc.CreateSchool(context.Background(), &platformModels.School{
		OrganizationID: 1,
		Name:           "Test",
		Slug:           "test",
		Subdomain:      "test",
		Active:         true,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, school)
	require.ErrorIs(t, err, assert.AnError)
}

// ---------------------------------------------------------------------------
// withAdminTx — existing tx in context
// ---------------------------------------------------------------------------

func TestWithAdminTx_ExistingTxInContext(t *testing.T) {
	svc := &operatorProvisioningService{}
	// Put a non-nil tx pointer in context so TxFromContext returns true
	tx := bun.Tx{}
	ctx := modelBase.ContextWithTx(context.Background(), &tx)
	called := false
	err := tenant.WithAdminTxOrDirect(ctx, svc.adminDB(), func(context.Context) error {
		called = true
		return nil
	})
	require.NoError(t, err)
	assert.True(t, called)
}

// ---------------------------------------------------------------------------
// isSchoolLookupNotFound — DatabaseError with non-ErrNoRows
// ---------------------------------------------------------------------------

func TestIsSchoolLookupNotFound_DatabaseErrorNonNoRows(t *testing.T) {
	err := &modelBase.DatabaseError{Op: "find", Err: assert.AnError}
	assert.False(t, isSchoolLookupNotFound(err))
}

// ---------------------------------------------------------------------------
// maskAPIKey
// ---------------------------------------------------------------------------

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name string
		key  *string
		want string
	}{
		{"nil pointer", nil, ""},
		{"empty string", testpkg.StrPtr(""), ""},
		{"short key (5 chars)", testpkg.StrPtr("abcde"), "abcde"},
		{"exactly 10 chars", testpkg.StrPtr("1234567890"), "1234567890"},
		{"longer than 10 chars", testpkg.StrPtr("1234567890abcdef"), "1234567890..."},
		{"11 chars", testpkg.StrPtr("12345678901"), "1234567890..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, maskAPIKey(tt.key))
		})
	}
}

// ---------------------------------------------------------------------------
// enrichDeviceInfo
// ---------------------------------------------------------------------------

func TestEnrichDeviceInfo_EmptySlice(t *testing.T) {
	result := enrichDeviceInfo([]OperatorDeviceInfo{})
	assert.Empty(t, result)
}

func TestEnrichDeviceInfo_NilSlice(t *testing.T) {
	result := enrichDeviceInfo(nil)
	assert.Nil(t, result)
}

func TestEnrichDeviceInfo_NilLastSeen(t *testing.T) {
	apiKey := "abcdef1234567890"
	devices := []OperatorDeviceInfo{
		{ID: 1, DeviceID: "dev-1", APIKey: &apiKey, LastSeen: nil},
	}
	result := enrichDeviceInfo(devices)
	require.Len(t, result, 1)
	assert.False(t, result[0].IsOnline)
	assert.Equal(t, "abcdef1234...", result[0].MaskedAPIKey)
}

func TestEnrichDeviceInfo_LastSeenWithin5Min(t *testing.T) {
	recent := time.Now().Add(-2 * time.Minute)
	devices := []OperatorDeviceInfo{
		{ID: 1, DeviceID: "dev-1", LastSeen: &recent},
	}
	result := enrichDeviceInfo(devices)
	require.Len(t, result, 1)
	assert.True(t, result[0].IsOnline)
}

func TestEnrichDeviceInfo_LastSeenOlderThan5Min(t *testing.T) {
	old := time.Now().Add(-10 * time.Minute)
	devices := []OperatorDeviceInfo{
		{ID: 1, DeviceID: "dev-1", LastSeen: &old},
	}
	result := enrichDeviceInfo(devices)
	require.Len(t, result, 1)
	assert.False(t, result[0].IsOnline)
}

func TestEnrichDeviceInfo_MultipleDevices(t *testing.T) {
	recent := time.Now().Add(-1 * time.Minute)
	old := time.Now().Add(-10 * time.Minute)
	key1 := "shortkey"
	key2 := "verylongapikey1234567890"
	devices := []OperatorDeviceInfo{
		{ID: 1, DeviceID: "dev-1", APIKey: &key1, LastSeen: &recent},
		{ID: 2, DeviceID: "dev-2", APIKey: &key2, LastSeen: &old},
		{ID: 3, DeviceID: "dev-3", APIKey: nil, LastSeen: nil},
	}
	result := enrichDeviceInfo(devices)
	require.Len(t, result, 3)
	assert.True(t, result[0].IsOnline)
	assert.Equal(t, "shortkey", result[0].MaskedAPIKey)
	assert.False(t, result[1].IsOnline)
	assert.Equal(t, "verylongap...", result[1].MaskedAPIKey)
	assert.False(t, result[2].IsOnline)
	assert.Equal(t, "", result[2].MaskedAPIKey)
}

// ---------------------------------------------------------------------------
// queryDevices
// ---------------------------------------------------------------------------

func TestQueryDevices_NoWhereClause(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
	})

	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{
		"id", "device_id", "device_type", "name", "status", "api_key",
		"last_seen", "created_at", "updated_at",
		"school_id", "school_name", "organization_id", "organization_name",
	}))

	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SummariesRepo: platformRepo.NewOperatorSummariesRepository(bunDB)}, txHandler: modelBase.NewTxHandler(bunDB)}
	result, err := svc.queryDevices(context.Background(), platformModels.OperatorDeviceFilter{})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestQueryDevices_WithWhereClause(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
	})

	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{
		"id", "device_id", "device_type", "name", "status", "api_key",
		"last_seen", "created_at", "updated_at",
		"school_id", "school_name", "organization_id", "organization_name",
	}))

	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SummariesRepo: platformRepo.NewOperatorSummariesRepository(bunDB)}, txHandler: modelBase.NewTxHandler(bunDB)}
	result, err := svc.queryDevices(context.Background(), platformModels.OperatorDeviceFilter{SchoolID: testpkg.Int64Ptr(42)})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestQueryDevices_ScanError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
	})

	mock.ExpectQuery("SELECT").WillReturnError(assert.AnError)

	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SummariesRepo: platformRepo.NewOperatorSummariesRepository(bunDB)}, txHandler: modelBase.NewTxHandler(bunDB)}
	result, err := svc.queryDevices(context.Background(), platformModels.OperatorDeviceFilter{})
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestQueryDevices_UsesTxFromContext(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
	})

	// Start a tx, put it in context
	mock.ExpectBegin()
	tx, err := bunDB.Begin()
	require.NoError(t, err)
	ctx := modelBase.ContextWithTx(context.Background(), &tx)

	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{
		"id", "device_id", "device_type", "name", "status", "api_key",
		"last_seen", "created_at", "updated_at",
		"school_id", "school_name", "organization_id", "organization_name",
	}))

	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SummariesRepo: platformRepo.NewOperatorSummariesRepository(bunDB)}, txHandler: modelBase.NewTxHandler(bunDB)}
	result, err := svc.queryDevices(ctx, platformModels.OperatorDeviceFilter{})
	require.NoError(t, err)
	assert.Empty(t, result)

	mock.ExpectRollback()
	_ = tx.Rollback()
}

// ---------------------------------------------------------------------------
// ListAllDevices (via queryDevices)
// ---------------------------------------------------------------------------

func TestListAllDevices_Success(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
	})

	// withAdminTx starts a tx; then queryDevices runs the SELECT inside it
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{
		"id", "device_id", "device_type", "name", "status", "api_key",
		"last_seen", "created_at", "updated_at",
		"school_id", "school_name", "organization_id", "organization_name",
	}))
	mock.ExpectCommit()

	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SummariesRepo: platformRepo.NewOperatorSummariesRepository(bunDB)}, txHandler: modelBase.NewTxHandler(bunDB)}
	result, err := svc.ListAllDevices(context.Background())
	require.NoError(t, err)
	assert.Empty(t, result)
}

// ---------------------------------------------------------------------------
// ListSchoolDevices (success via queryDevices)
// ---------------------------------------------------------------------------

func TestListSchoolDevices_Success(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{
		"id", "device_id", "device_type", "name", "status", "api_key",
		"last_seen", "created_at", "updated_at",
		"school_id", "school_name", "organization_id", "organization_name",
	}))
	mock.ExpectCommit()

	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SummariesRepo: platformRepo.NewOperatorSummariesRepository(bunDB), SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
			return &platformModels.School{
				Model:          modelBase.Model{ID: 42},
				OrganizationID: 1,
				Name:           "School",
				Slug:           "school",
				Subdomain:      "school",
				Active:         true,
			}, nil
		},
	}}, txHandler: modelBase.NewTxHandler(bunDB),
	}
	result, err := svc.ListSchoolDevices(context.Background(), 42)
	require.NoError(t, err)
	assert.Empty(t, result)
}

// ---------------------------------------------------------------------------
// ListOrganizationDevices (success via queryDevices)
// ---------------------------------------------------------------------------

func TestListOrganizationDevices_Success(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{
		"id", "device_id", "device_type", "name", "status", "api_key",
		"last_seen", "created_at", "updated_at",
		"school_id", "school_name", "organization_id", "organization_name",
	}))
	mock.ExpectCommit()

	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SummariesRepo: platformRepo.NewOperatorSummariesRepository(bunDB), OrganizationRepo: &internalOrgRepoStub{
		findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
			return &platformModels.Organization{Model: modelBase.Model{ID: 5}, Name: "Org", Slug: "org", Active: true}, nil
		},
	}}, txHandler: modelBase.NewTxHandler(bunDB),
	}
	result, err := svc.ListOrganizationDevices(context.Background(), 5)
	require.NoError(t, err)
	assert.Empty(t, result)
}

// ---------------------------------------------------------------------------
// isAPIKeyConstraintViolation
// ---------------------------------------------------------------------------

func TestIsAPIKeyConstraintViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			"positive: devices_api_key_key constraint",
			newPgErrorWithConstraint("23505", "devices_api_key_key"),
			true,
		},
		{
			"other constraint: devices_device_id_key",
			newPgErrorWithConstraint("23505", "devices_device_id_key"),
			false,
		},
		{
			"non-pg error",
			assert.AnError,
			false,
		},
		{
			"DatabaseError wrapping api_key constraint",
			&modelBase.DatabaseError{Op: "create", Err: newPgErrorWithConstraint("23505", "devices_api_key_key")},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isAPIKeyConstraintViolation(tt.err))
		})
	}
}

// ---------------------------------------------------------------------------
// CreateDevice — validation errors
// ---------------------------------------------------------------------------

func TestCreateDevice_InvalidInput(t *testing.T) {
	svc := &operatorProvisioningService{}

	t.Run("schoolID <= 0", func(t *testing.T) {
		result, err := svc.CreateDevice(context.Background(), 0, "dev-1", "rfid", nil, nil, 1, net.IPv4(127, 0, 0, 1))
		require.Nil(t, result)
		var invalidErr *InvalidDataError
		require.ErrorAs(t, err, &invalidErr)
		assert.Contains(t, err.Error(), "school_id is required")
	})

	t.Run("empty deviceID", func(t *testing.T) {
		result, err := svc.CreateDevice(context.Background(), 1, "", "rfid", nil, nil, 1, net.IPv4(127, 0, 0, 1))
		require.Nil(t, result)
		var invalidErr *InvalidDataError
		require.ErrorAs(t, err, &invalidErr)
		assert.Contains(t, err.Error(), "device_id is required")
	})

	t.Run("whitespace-only deviceID", func(t *testing.T) {
		result, err := svc.CreateDevice(context.Background(), 1, "   ", "rfid", nil, nil, 1, net.IPv4(127, 0, 0, 1))
		require.Nil(t, result)
		var invalidErr *InvalidDataError
		require.ErrorAs(t, err, &invalidErr)
		assert.Contains(t, err.Error(), "device_id is required")
	})

	t.Run("empty deviceType", func(t *testing.T) {
		result, err := svc.CreateDevice(context.Background(), 1, "dev-1", "", nil, nil, 1, net.IPv4(127, 0, 0, 1))
		require.Nil(t, result)
		var invalidErr *InvalidDataError
		require.ErrorAs(t, err, &invalidErr)
		assert.Contains(t, err.Error(), "device_type is required")
	})
}

// ---------------------------------------------------------------------------
// CreateDevice — school not found
// ---------------------------------------------------------------------------

func TestCreateDevice_SchoolNotFound(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
			return nil, nil // nil school, no error
		},
	}},
	}
	result, err := svc.CreateDevice(context.Background(), 99, "dev-1", "rfid", nil, nil, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, result)
	var notFoundErr *SchoolNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
	assert.Equal(t, int64(99), notFoundErr.SchoolID)
}

func TestCreateDevice_SchoolNotFound_SqlErrNoRows(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
			return nil, sql.ErrNoRows
		},
	}},
	}
	result, err := svc.CreateDevice(context.Background(), 99, "dev-1", "rfid", nil, nil, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, result)
	var notFoundErr *SchoolNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

// ---------------------------------------------------------------------------
// CreateDevice — success with auto-generated key
// ---------------------------------------------------------------------------

func TestCreateDevice_Success_AutoKey(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
	})

	var createdDevice *iotModels.Device
	deviceName := "Test Reader"

	// withAdminTx opens tx; queryDeviceSingle runs SELECT inside it
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{
		"id", "device_id", "device_type", "name", "status", "api_key",
		"last_seen", "created_at", "updated_at",
		"school_id", "school_name", "organization_id", "organization_name",
	}).AddRow(
		int64(10), "dev-1", "rfid", "Test Reader", "active", "dev_autokey",
		nil, time.Now(), time.Now(),
		int64(42), "Test School", int64(1), "Test Org",
	))
	mock.ExpectCommit()

	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SummariesRepo: platformRepo.NewOperatorSummariesRepository(bunDB), SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
			return &platformModels.School{
				Model:          modelBase.Model{ID: 42},
				OrganizationID: 1,
				Name:           "Test School",
				Slug:           "test-school",
				Subdomain:      "test-school",
				Active:         true,
			}, nil
		},
	}, DeviceRepo: &internalDeviceRepoStub{
		createFn: func(_ context.Context, device *iotModels.Device) error {
			createdDevice = device
			device.ID = 10
			return nil
		},
	}, AuditLogRepo: &internalAuditLogRepoStub{}, Logger: slog.Default()}, txHandler: modelBase.NewTxHandler(bunDB),
	}

	result, err := svc.CreateDevice(context.Background(), 42, "dev-1", "rfid", &deviceName, nil, 1, net.IPv4(127, 0, 0, 1))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(10), result.ID)
	assert.Equal(t, "dev-1", result.DeviceID)

	// Verify the device was created with auto-generated key
	require.NotNil(t, createdDevice)
	assert.Equal(t, "dev-1", createdDevice.DeviceID)
	assert.Equal(t, "rfid", createdDevice.DeviceType)
	assert.Equal(t, iotModels.DeviceStatusActive, createdDevice.Status)
	require.NotNil(t, createdDevice.APIKey)
	assert.True(t, strings.HasPrefix(*createdDevice.APIKey, "dev_"), "auto key should start with dev_")
	assert.Len(t, *createdDevice.APIKey, 4+64, "dev_ prefix + 64 hex chars")
}

// ---------------------------------------------------------------------------
// CreateDevice — success with manual key
// ---------------------------------------------------------------------------

func TestCreateDevice_Success_ManualKey(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
	})

	var createdDevice *iotModels.Device
	manualKey := "my-custom-api-key-12345"

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{
		"id", "device_id", "device_type", "name", "status", "api_key",
		"last_seen", "created_at", "updated_at",
		"school_id", "school_name", "organization_id", "organization_name",
	}).AddRow(
		int64(11), "dev-2", "rfid", nil, "active", manualKey,
		nil, time.Now(), time.Now(),
		int64(42), "Test School", int64(1), "Test Org",
	))
	mock.ExpectCommit()

	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SummariesRepo: platformRepo.NewOperatorSummariesRepository(bunDB), SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
			return &platformModels.School{
				Model:          modelBase.Model{ID: 42},
				OrganizationID: 1,
				Name:           "Test School",
				Slug:           "test-school",
				Subdomain:      "test-school",
				Active:         true,
			}, nil
		},
	}, DeviceRepo: &internalDeviceRepoStub{
		createFn: func(_ context.Context, device *iotModels.Device) error {
			createdDevice = device
			device.ID = 11
			return nil
		},
	}, AuditLogRepo: &internalAuditLogRepoStub{}, Logger: slog.Default()}, txHandler: modelBase.NewTxHandler(bunDB),
	}

	result, err := svc.CreateDevice(context.Background(), 42, "dev-2", "rfid", nil, &manualKey, 1, net.IPv4(127, 0, 0, 1))
	require.NoError(t, err)
	require.NotNil(t, result)

	require.NotNil(t, createdDevice)
	require.NotNil(t, createdDevice.APIKey)
	assert.Equal(t, manualKey, *createdDevice.APIKey)
}

// ---------------------------------------------------------------------------
// CreateDevice — duplicate device_id (unique violation on device_id constraint)
// ---------------------------------------------------------------------------

func TestCreateDevice_DuplicateDeviceID(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
			return &platformModels.School{
				Model:          modelBase.Model{ID: 42},
				OrganizationID: 1,
				Name:           "School",
				Slug:           "school",
				Subdomain:      "school",
				Active:         true,
			}, nil
		},
	}, DeviceRepo: &internalDeviceRepoStub{
		createFn: func(context.Context, *iotModels.Device) error {
			// unique violation on device_id constraint (not api_key)
			return newPgErrorWithConstraint("23505", "devices_device_id_key")
		},
	}},
	}

	result, err := svc.CreateDevice(context.Background(), 42, "dev-dup", "rfid", nil, nil, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, result)
	var conflictErr *ConflictError
	require.ErrorAs(t, err, &conflictErr)
	assert.Contains(t, err.Error(), "device_id already exists for this school")
}

// ---------------------------------------------------------------------------
// CreateDevice — auto-key collision retry (first attempt fails, second succeeds)
// ---------------------------------------------------------------------------

func TestCreateDevice_AutoKeyCollisionRetry(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
	})

	attempt := 0
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{
		"id", "device_id", "device_type", "name", "status", "api_key",
		"last_seen", "created_at", "updated_at",
		"school_id", "school_name", "organization_id", "organization_name",
	}).AddRow(
		int64(10), "dev-1", "rfid", nil, "active", "dev_somekey",
		nil, time.Now(), time.Now(),
		int64(42), "School", int64(1), "Org",
	))
	mock.ExpectCommit()

	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SummariesRepo: platformRepo.NewOperatorSummariesRepository(bunDB), SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
			return &platformModels.School{
				Model:          modelBase.Model{ID: 42},
				OrganizationID: 1,
				Name:           "School",
				Slug:           "school",
				Subdomain:      "school",
				Active:         true,
			}, nil
		},
	}, DeviceRepo: &internalDeviceRepoStub{
		createFn: func(_ context.Context, device *iotModels.Device) error {
			attempt++
			if attempt == 1 {
				// First attempt: api_key collision
				return newPgErrorWithConstraint("23505", "devices_api_key_key")
			}
			// Second attempt: success
			device.ID = 10
			return nil
		},
	}, AuditLogRepo: &internalAuditLogRepoStub{}, Logger: slog.Default()}, txHandler: modelBase.NewTxHandler(bunDB),
	}

	result, err := svc.CreateDevice(context.Background(), 42, "dev-1", "rfid", nil, nil, 1, net.IPv4(127, 0, 0, 1))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, attempt, "should have retried after api_key collision")
}

// ---------------------------------------------------------------------------
// CreateDevice — manual key conflict (api_key collision with provided key)
// ---------------------------------------------------------------------------

func TestCreateDevice_ManualKeyConflict(t *testing.T) {
	manualKey := "my-conflicting-key"
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
			return &platformModels.School{
				Model:          modelBase.Model{ID: 42},
				OrganizationID: 1,
				Name:           "School",
				Slug:           "school",
				Subdomain:      "school",
				Active:         true,
			}, nil
		},
	}, DeviceRepo: &internalDeviceRepoStub{
		createFn: func(context.Context, *iotModels.Device) error {
			return newPgErrorWithConstraint("23505", "devices_api_key_key")
		},
	}},
	}

	result, err := svc.CreateDevice(context.Background(), 42, "dev-1", "rfid", nil, &manualKey, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, result)
	var conflictErr *ConflictError
	require.ErrorAs(t, err, &conflictErr)
	assert.Contains(t, err.Error(), "api_key already in use")
}

// ---------------------------------------------------------------------------
// SetDeviceAPIKey — invalid ID
// ---------------------------------------------------------------------------

func TestSetDeviceAPIKey_InvalidID(t *testing.T) {
	svc := &operatorProvisioningService{}

	t.Run("id zero", func(t *testing.T) {
		result, err := svc.SetDeviceAPIKey(context.Background(), 0, nil, 1, net.IPv4(127, 0, 0, 1))
		require.Nil(t, result)
		var invalidErr *InvalidDataError
		require.ErrorAs(t, err, &invalidErr)
		assert.Contains(t, err.Error(), "device id is required")
	})

	t.Run("id negative", func(t *testing.T) {
		result, err := svc.SetDeviceAPIKey(context.Background(), -1, nil, 1, net.IPv4(127, 0, 0, 1))
		require.Nil(t, result)
		var invalidErr *InvalidDataError
		require.ErrorAs(t, err, &invalidErr)
	})
}

// ---------------------------------------------------------------------------
// SetDeviceAPIKey — device not found
// ---------------------------------------------------------------------------

func TestSetDeviceAPIKey_DeviceNotFound(t *testing.T) {
	t.Run("nil device returned", func(t *testing.T) {
		svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{DeviceRepo: &internalDeviceRepoStub{
			findByIDFn: func(context.Context, interface{}) (*iotModels.Device, error) {
				return nil, nil
			},
		}},
		}
		result, err := svc.SetDeviceAPIKey(context.Background(), 99, nil, 1, net.IPv4(127, 0, 0, 1))
		require.Nil(t, result)
		var notFoundErr *OperatorDeviceNotFoundError
		require.ErrorAs(t, err, &notFoundErr)
		assert.Equal(t, int64(99), notFoundErr.DeviceID)
	})

	t.Run("sql.ErrNoRows", func(t *testing.T) {
		svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{DeviceRepo: &internalDeviceRepoStub{
			findByIDFn: func(context.Context, interface{}) (*iotModels.Device, error) {
				return nil, sql.ErrNoRows
			},
		}},
		}
		result, err := svc.SetDeviceAPIKey(context.Background(), 99, nil, 1, net.IPv4(127, 0, 0, 1))
		require.Nil(t, result)
		var notFoundErr *OperatorDeviceNotFoundError
		require.ErrorAs(t, err, &notFoundErr)
	})

	t.Run("DatabaseError wrapping sql.ErrNoRows", func(t *testing.T) {
		svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{DeviceRepo: &internalDeviceRepoStub{
			findByIDFn: func(context.Context, interface{}) (*iotModels.Device, error) {
				return nil, &modelBase.DatabaseError{Op: "find", Err: sql.ErrNoRows}
			},
		}},
		}
		result, err := svc.SetDeviceAPIKey(context.Background(), 99, nil, 1, net.IPv4(127, 0, 0, 1))
		require.Nil(t, result)
		var notFoundErr *OperatorDeviceNotFoundError
		require.ErrorAs(t, err, &notFoundErr)
	})
}

// ---------------------------------------------------------------------------
// SetDeviceAPIKey — success with auto-generated key
// ---------------------------------------------------------------------------

func TestSetDeviceAPIKey_Success_AutoKey(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
	})

	var updatedDevice *iotModels.Device

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{
		"id", "device_id", "device_type", "name", "status", "api_key",
		"last_seen", "created_at", "updated_at",
		"school_id", "school_name", "organization_id", "organization_name",
	}).AddRow(
		int64(10), "dev-1", "rfid", nil, "active", "dev_newkey",
		nil, time.Now(), time.Now(),
		int64(42), "School", int64(1), "Org",
	))
	mock.ExpectCommit()

	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{DeviceRepo: &internalDeviceRepoStub{
		findByIDFn: func(_ context.Context, id interface{}) (*iotModels.Device, error) {
			return &iotModels.Device{
				Model:      modelBase.Model{ID: 10},
				DeviceID:   "dev-1",
				DeviceType: "rfid",
				Status:     iotModels.DeviceStatusActive,
			}, nil
		},
		updateFn: func(_ context.Context, device *iotModels.Device) error {
			updatedDevice = device
			return nil
		},
	}, SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(_ context.Context, _ int64) (*platformModels.School, error) {
			return &platformModels.School{Active: true}, nil
		},
	}, AuditLogRepo: &internalAuditLogRepoStub{}, SummariesRepo: platformRepo.NewOperatorSummariesRepository(bunDB), Logger: slog.Default()}, txHandler: modelBase.NewTxHandler(bunDB),
	}

	result, err := svc.SetDeviceAPIKey(context.Background(), 10, nil, 1, net.IPv4(127, 0, 0, 1))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(10), result.ID)

	require.NotNil(t, updatedDevice)
	require.NotNil(t, updatedDevice.APIKey)
	assert.True(t, strings.HasPrefix(*updatedDevice.APIKey, "dev_"), "auto key should start with dev_")
	assert.Len(t, *updatedDevice.APIKey, 4+64)
}

// ---------------------------------------------------------------------------
// SetDeviceAPIKey — success with manual key
// ---------------------------------------------------------------------------

func TestSetDeviceAPIKey_Success_ManualKey(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
	})

	manualKey := "custom-key-abc"
	var updatedDevice *iotModels.Device

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{
		"id", "device_id", "device_type", "name", "status", "api_key",
		"last_seen", "created_at", "updated_at",
		"school_id", "school_name", "organization_id", "organization_name",
	}).AddRow(
		int64(10), "dev-1", "rfid", nil, "active", manualKey,
		nil, time.Now(), time.Now(),
		int64(42), "School", int64(1), "Org",
	))
	mock.ExpectCommit()

	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SummariesRepo: platformRepo.NewOperatorSummariesRepository(bunDB), DeviceRepo: &internalDeviceRepoStub{
		findByIDFn: func(_ context.Context, id interface{}) (*iotModels.Device, error) {
			return &iotModels.Device{
				Model:      modelBase.Model{ID: 10},
				DeviceID:   "dev-1",
				DeviceType: "rfid",
				Status:     iotModels.DeviceStatusActive,
			}, nil
		},
		updateFn: func(_ context.Context, device *iotModels.Device) error {
			updatedDevice = device
			return nil
		},
	}, SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(_ context.Context, _ int64) (*platformModels.School, error) {
			return &platformModels.School{Active: true}, nil
		},
	}, AuditLogRepo: &internalAuditLogRepoStub{}, Logger: slog.Default()}, txHandler: modelBase.NewTxHandler(bunDB),
	}

	result, err := svc.SetDeviceAPIKey(context.Background(), 10, &manualKey, 1, net.IPv4(127, 0, 0, 1))
	require.NoError(t, err)
	require.NotNil(t, result)

	require.NotNil(t, updatedDevice)
	require.NotNil(t, updatedDevice.APIKey)
	assert.Equal(t, manualKey, *updatedDevice.APIKey)
}

// ---------------------------------------------------------------------------
// SetDeviceAPIKey — manual key conflict (api_key collision)
// ---------------------------------------------------------------------------

func TestSetDeviceAPIKey_ManualKeyConflict(t *testing.T) {
	manualKey := "conflicting-key"
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{DeviceRepo: &internalDeviceRepoStub{
		findByIDFn: func(_ context.Context, id interface{}) (*iotModels.Device, error) {
			return &iotModels.Device{
				Model:      modelBase.Model{ID: 10},
				DeviceID:   "dev-1",
				DeviceType: "rfid",
				Status:     iotModels.DeviceStatusActive,
			}, nil
		},
		updateFn: func(context.Context, *iotModels.Device) error {
			return newPgErrorWithConstraint("23505", "devices_api_key_key")
		},
	}, SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(_ context.Context, _ int64) (*platformModels.School, error) {
			return &platformModels.School{Active: true}, nil
		},
	}},
	}

	result, err := svc.SetDeviceAPIKey(context.Background(), 10, &manualKey, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, result)
	var conflictErr *ConflictError
	require.ErrorAs(t, err, &conflictErr)
	assert.Contains(t, err.Error(), "api_key already in use")
}

// ---------------------------------------------------------------------------
// queryDeviceSingle
// ---------------------------------------------------------------------------

func TestQueryDeviceSingle_RequeryFailure(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
	})

	mock.ExpectQuery("SELECT").WillReturnError(assert.AnError)

	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SummariesRepo: platformRepo.NewOperatorSummariesRepository(bunDB)}, txHandler: modelBase.NewTxHandler(bunDB)}

	result, err := svc.queryDeviceSingle(context.Background(), "CreateDevice", int64(10))
	require.Nil(t, result)
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), "CreateDevice: re-query failed")
}

func TestQueryDeviceSingle_NoRows(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
	})

	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{
		"id", "device_id", "device_type", "name", "status", "api_key",
		"last_seen", "created_at", "updated_at",
		"school_id", "school_name", "organization_id", "organization_name",
	}))

	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SummariesRepo: platformRepo.NewOperatorSummariesRepository(bunDB), Logger: slog.Default()}, txHandler: modelBase.NewTxHandler(bunDB)}

	result, err := svc.queryDeviceSingle(context.Background(), "SetDeviceAPIKey", int64(10))
	require.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SetDeviceAPIKey: device not found after write")
}

// ---------------------------------------------------------------------------
// DeleteDevice
// ---------------------------------------------------------------------------

func TestDeleteDevice_InvalidID(t *testing.T) {
	svc := &operatorProvisioningService{}

	err := svc.DeleteDevice(context.Background(), 0, 1, net.IPv4(127, 0, 0, 1))
	var invalidErr *InvalidDataError
	require.ErrorAs(t, err, &invalidErr)
	assert.Contains(t, err.Error(), "device id is required")
}

func TestDeleteDevice_NotFound(t *testing.T) {
	tests := []struct {
		name     string
		findByID func(context.Context, interface{}) (*iotModels.Device, error)
	}{
		{
			name: "nil device returned",
			findByID: func(context.Context, interface{}) (*iotModels.Device, error) {
				return nil, nil
			},
		},
		{
			name: "sql.ErrNoRows",
			findByID: func(context.Context, interface{}) (*iotModels.Device, error) {
				return nil, sql.ErrNoRows
			},
		},
		{
			name: "DatabaseError wrapping sql.ErrNoRows",
			findByID: func(context.Context, interface{}) (*iotModels.Device, error) {
				return nil, &modelBase.DatabaseError{Op: "find", Err: sql.ErrNoRows}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{DeviceRepo: &internalDeviceRepoStub{
				findByIDFn: tt.findByID,
			}},
			}

			err := svc.DeleteDevice(context.Background(), 99, 1, net.IPv4(127, 0, 0, 1))
			var notFoundErr *OperatorDeviceNotFoundError
			require.ErrorAs(t, err, &notFoundErr)
			assert.Equal(t, int64(99), notFoundErr.DeviceID)
		})
	}
}

func TestDeleteDevice_ProtectedDevice(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{DeviceRepo: &internalDeviceRepoStub{
		findByIDFn: func(context.Context, interface{}) (*iotModels.Device, error) {
			return &iotModels.Device{
				Model:      modelBase.Model{ID: 10},
				DeviceID:   iotModels.WebManualDeviceID,
				DeviceType: iotModels.DeviceTypeVirtual,
			}, nil
		},
	}},
	}

	err := svc.DeleteDevice(context.Background(), 10, 1, net.IPv4(127, 0, 0, 1))
	var protectedErr *DeviceProtectedError
	require.ErrorAs(t, err, &protectedErr)
	assert.Equal(t, int64(10), protectedErr.DeviceID)
}

func TestDeleteDevice_DeleteErrors(t *testing.T) {
	t.Run("foreign key violation becomes device in use", func(t *testing.T) {
		device := &iotModels.Device{
			Model:      modelBase.Model{ID: 10},
			DeviceID:   "reader-10",
			DeviceType: "rfid",
		}
		device.SetTenantID(42)

		svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{DeviceRepo: &internalDeviceRepoStub{
			findByIDFn: func(context.Context, interface{}) (*iotModels.Device, error) {
				return device, nil
			},
			deleteFn: func(context.Context, interface{}) error {
				return newPgError("23503")
			},
		}},
		}

		err := svc.DeleteDevice(context.Background(), 10, 1, net.IPv4(127, 0, 0, 1))
		var inUseErr *DeviceInUseError
		require.ErrorAs(t, err, &inUseErr)
		assert.Equal(t, int64(10), inUseErr.DeviceID)
	})

	t.Run("generic delete error is wrapped", func(t *testing.T) {
		device := &iotModels.Device{
			Model:      modelBase.Model{ID: 10},
			DeviceID:   "reader-10",
			DeviceType: "rfid",
		}
		device.SetTenantID(42)

		svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{DeviceRepo: &internalDeviceRepoStub{
			findByIDFn: func(context.Context, interface{}) (*iotModels.Device, error) {
				return device, nil
			},
			deleteFn: func(context.Context, interface{}) error {
				return assert.AnError
			},
		}},
		}

		err := svc.DeleteDevice(context.Background(), 10, 1, net.IPv4(127, 0, 0, 1))
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "DeleteDevice")
	})
}

func TestDeleteDevice_Success(t *testing.T) {
	var deletedID interface{}
	var auditEntry *platformModels.OperatorAuditLog
	device := &iotModels.Device{
		Model:      modelBase.Model{ID: 10},
		DeviceID:   "reader-10",
		DeviceType: "rfid",
	}
	device.SetTenantID(42)

	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{DeviceRepo: &internalDeviceRepoStub{
		findByIDFn: func(context.Context, interface{}) (*iotModels.Device, error) {
			return device, nil
		},
		deleteFn: func(_ context.Context, id interface{}) error {
			deletedID = id
			return nil
		},
	}, AuditLogRepo: &internalAuditLogRepoStub{
		createFn: func(_ context.Context, entry *platformModels.OperatorAuditLog) error {
			auditEntry = entry
			return nil
		},
	}},
	}

	err := svc.DeleteDevice(context.Background(), 10, 7, net.IPv4(127, 0, 0, 1))
	require.NoError(t, err)
	assert.Equal(t, int64(10), deletedID)
	require.NotNil(t, auditEntry)
	assert.Equal(t, platformModels.ActionDelete, auditEntry.Action)
	assert.Equal(t, platformModels.ResourceDevice, auditEntry.ResourceType)
}

// ---------------------------------------------------------------------------
// SoftDeleteSchool / RestoreSchool
// ---------------------------------------------------------------------------

func TestSoftDeleteSchool_FindByIDError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
			return nil, assert.AnError
		},
	}},
	}

	err := svc.SoftDeleteSchool(context.Background(), 42, 7, net.IPv4(127, 0, 0, 1))
	require.ErrorIs(t, err, assert.AnError)
}

func TestSoftDeleteSchool_RowsAffectedMismatch(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
			return &platformModels.School{
				Model:          modelBase.Model{ID: 42},
				OrganizationID: 1,
				Name:           "School",
				Slug:           "school",
				Subdomain:      "school",
				Active:         true,
			}, nil
		},
		SoftDeleteFn: func(context.Context, int64) error {
			return &modelBase.DatabaseError{
				Op:  "soft delete school",
				Err: errors.New("expected 1 rows affected, got 0"),
			}
		},
	}},
	}

	err := svc.SoftDeleteSchool(context.Background(), 42, 7, net.IPv4(127, 0, 0, 1))
	var alreadyDeletedErr *SchoolAlreadyDeletedError
	require.ErrorAs(t, err, &alreadyDeletedErr)
}

func TestRestoreSchool_FindByIDError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
			return nil, assert.AnError
		},
	}},
	}

	err := svc.RestoreSchool(context.Background(), 42, 7, net.IPv4(127, 0, 0, 1))
	require.ErrorIs(t, err, assert.AnError)
}

func TestRestoreSchool_ConcurrentRestoreMapsToNotDeleted(t *testing.T) {
	deletedAt := time.Now()
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{SchoolRepo: &testpkg.SchoolRepoMock{
		FindByIDFn: func(context.Context, int64) (*platformModels.School, error) {
			return &platformModels.School{
				Model:          modelBase.Model{ID: 42},
				OrganizationID: 1,
				Name:           "School",
				Slug:           "school",
				Subdomain:      "school",
				Active:         true,
				DeletedAt:      &deletedAt,
			}, nil
		},
		RestoreFn: func(context.Context, int64) error {
			return &modelBase.DatabaseError{
				Op:  "restore school",
				Err: errors.New("expected 1 rows affected, got 0"),
			}
		},
	}, OrganizationRepo: &internalOrgRepoStub{
		findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
			return &platformModels.Organization{
				Model:  modelBase.Model{ID: 1},
				Name:   "Org",
				Slug:   "org",
				Active: true,
			}, nil
		},
	}},
	}

	err := svc.RestoreSchool(context.Background(), 42, 7, net.IPv4(127, 0, 0, 1))
	var notDeletedErr *SchoolNotDeletedError
	require.ErrorAs(t, err, &notDeletedErr)
}

// ---------------------------------------------------------------------------
// SoftDeleteOrganization / RestoreOrganization
// ---------------------------------------------------------------------------

func TestSoftDeleteOrganization_FindByIDError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{OrganizationRepo: &internalOrgRepoStub{
		findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
			return nil, assert.AnError
		},
	}, SchoolRepo: &testpkg.SchoolRepoMock{}},
	}

	err := svc.SoftDeleteOrganization(context.Background(), 42, 7, net.IPv4(127, 0, 0, 1))
	require.ErrorIs(t, err, assert.AnError)
}

func TestSoftDeleteOrganization_RowsAffectedMismatch(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{OrganizationRepo: &internalOrgRepoStub{
		findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
			return &platformModels.Organization{
				Model:  modelBase.Model{ID: 42},
				Name:   "Org",
				Slug:   "org",
				Active: true,
			}, nil
		},
		softDeleteFn: func(context.Context, int64) error {
			return &modelBase.DatabaseError{
				Op:  "soft delete organization",
				Err: errors.New("expected 1 rows affected, got 0"),
			}
		},
	}, SchoolRepo: &testpkg.SchoolRepoMock{
		CountNonDeletedByOrganizationIDFn: func(context.Context, int64) (int, error) {
			return 0, nil
		},
	}},
	}

	err := svc.SoftDeleteOrganization(context.Background(), 42, 7, net.IPv4(127, 0, 0, 1))
	var alreadyDeletedErr *OrganizationAlreadyDeletedError
	require.ErrorAs(t, err, &alreadyDeletedErr)
}

func TestRestoreOrganization_FindByIDError(t *testing.T) {
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{OrganizationRepo: &internalOrgRepoStub{
		findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
			return nil, assert.AnError
		},
	}, SchoolRepo: &testpkg.SchoolRepoMock{}},
	}

	err := svc.RestoreOrganization(context.Background(), 42, 7, net.IPv4(127, 0, 0, 1))
	require.ErrorIs(t, err, assert.AnError)
}

func TestRestoreOrganization_ConcurrentRestoreMapsToNotDeleted(t *testing.T) {
	deletedAt := time.Now()
	svc := &operatorProvisioningService{OperatorProvisioningServiceConfig: OperatorProvisioningServiceConfig{OrganizationRepo: &internalOrgRepoStub{
		findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
			return &platformModels.Organization{
				Model:     modelBase.Model{ID: 42},
				Name:      "Org",
				Slug:      "org",
				Active:    true,
				DeletedAt: &deletedAt,
			}, nil
		},
		restoreFn: func(context.Context, int64) error {
			return &modelBase.DatabaseError{
				Op:  "restore organization",
				Err: errors.New("expected 1 rows affected, got 0"),
			}
		},
	}, SchoolRepo: &testpkg.SchoolRepoMock{}},
	}

	err := svc.RestoreOrganization(context.Background(), 42, 7, net.IPv4(127, 0, 0, 1))
	var notDeletedErr *OrganizationNotDeletedError
	require.ErrorAs(t, err, &notDeletedErr)
}

// ---------------------------------------------------------------------------
// shouldCreateTeacher
// ---------------------------------------------------------------------------

func TestShouldCreateTeacher(t *testing.T) {
	tests := []struct {
		name     string
		roleName string
		want     bool
	}{
		{"user role", "user", true},
		{"teacher role", "teacher", false},
		{"admin role", "admin", false},
		{"uppercase Teacher", "Teacher", false},
		{"whitespace user", " user ", true},
		{"mixed case USER", "USER", true},
		{"empty string", "", false},
		{"unknown role", "moderator", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shouldCreateTeacher(tt.roleName))
		})
	}
}

// ---------------------------------------------------------------------------
// generateRandomSuffix
// ---------------------------------------------------------------------------

func TestGenerateRandomSuffix(t *testing.T) {
	const validChars = "abcdefghijklmnopqrstuvwxyz0123456789"

	t.Run("correct length", func(t *testing.T) {
		for _, length := range []int{0, 1, 6, 20} {
			result := generateRandomSuffix(length)
			assert.Len(t, result, length, "length=%d", length)
		}
	})

	t.Run("valid characters", func(t *testing.T) {
		result := generateRandomSuffix(100)
		for i, ch := range result {
			assert.Contains(t, validChars, string(ch), "invalid char at index %d: %c", i, ch)
		}
	})

	t.Run("two calls produce different results", func(t *testing.T) {
		a := generateRandomSuffix(12)
		b := generateRandomSuffix(12)
		// With 36^12 possible values, collision probability is negligible.
		assert.NotEqual(t, a, b, "two random suffixes should differ")
	})
}
