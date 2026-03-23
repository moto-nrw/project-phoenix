package platform

import (
	"context"
	"database/sql"
	"log/slog"
	"net"
	"reflect"
	"testing"
	"time"
	"unsafe"

	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

type internalSchoolRepoStub struct {
	createFn           func(context.Context, *platformModels.School) error
	findByIDFn         func(context.Context, int64) (*platformModels.School, error)
	findByOrgAndSlugFn func(context.Context, int64, string) (*platformModels.School, error)
	findBySubdomainFn  func(context.Context, string) (*platformModels.School, error)
}

func (s *internalSchoolRepoStub) Create(ctx context.Context, school *platformModels.School) error {
	if s.createFn != nil {
		return s.createFn(ctx, school)
	}
	return nil
}
func (s *internalSchoolRepoStub) FindByID(ctx context.Context, id int64) (*platformModels.School, error) {
	if s.findByIDFn != nil {
		return s.findByIDFn(ctx, id)
	}
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
func (s *internalSchoolRepoStub) Update(context.Context, *platformModels.School) error {
	return nil
}

type internalOrgRepoStub struct {
	findByIDFn   func(context.Context, int64) (*platformModels.Organization, error)
	findBySlugFn func(context.Context, string) (*platformModels.Organization, error)
	createFn     func(context.Context, *platformModels.Organization) error
	listFn       func(context.Context) ([]*platformModels.Organization, error)
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

type internalDeviceRepoStub struct {
	createFn func(context.Context, *iotModels.Device) error
}

func (s *internalDeviceRepoStub) Create(ctx context.Context, device *iotModels.Device) error {
	if s.createFn != nil {
		return s.createFn(ctx, device)
	}
	return nil
}
func (s *internalDeviceRepoStub) FindByID(context.Context, interface{}) (*iotModels.Device, error) {
	return nil, nil
}
func (s *internalDeviceRepoStub) Update(context.Context, *iotModels.Device) error { return nil }
func (s *internalDeviceRepoStub) Delete(context.Context, interface{}) error       { return nil }
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
func (s *internalDeviceRepoStub) UpdateStatus(context.Context, string, iotModels.DeviceStatus) error {
	return nil
}
func (s *internalDeviceRepoStub) FindActiveDevices(context.Context) ([]*iotModels.Device, error) {
	return nil, nil
}
func (s *internalDeviceRepoStub) FindDevicesRequiringMaintenance(context.Context) ([]*iotModels.Device, error) {
	return nil, nil
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
func (s *internalCategoryRepoStub) Update(context.Context, *activityModels.Category) error {
	return nil
}
func (s *internalCategoryRepoStub) Delete(context.Context, interface{}) error { return nil }
func (s *internalCategoryRepoStub) List(context.Context, *modelBase.QueryOptions) ([]*activityModels.Category, error) {
	return nil, nil
}
func (s *internalCategoryRepoStub) FindByName(context.Context, string) (*activityModels.Category, error) {
	return nil, nil
}
func (s *internalCategoryRepoStub) ListAll(context.Context) ([]*activityModels.Category, error) {
	return nil, nil
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

// ---------------------------------------------------------------------------
// createWebManualDevice
// ---------------------------------------------------------------------------

func TestCreateWebManualDevice_NilDeviceRepo(t *testing.T) {
	svc := &operatorProvisioningService{}
	require.NoError(t, svc.createWebManualDevice(context.Background(), 1))
}

func TestCreateWebManualDevice_ZeroTenantID(t *testing.T) {
	svc := &operatorProvisioningService{deviceRepo: &internalDeviceRepoStub{}}
	require.NoError(t, svc.createWebManualDevice(context.Background(), 0))
}

func TestCreateWebManualDevice_NegativeTenantID(t *testing.T) {
	svc := &operatorProvisioningService{deviceRepo: &internalDeviceRepoStub{}}
	require.NoError(t, svc.createWebManualDevice(context.Background(), -1))
}

func TestCreateWebManualDevice_Success(t *testing.T) {
	var created *iotModels.Device
	svc := &operatorProvisioningService{
		deviceRepo: &internalDeviceRepoStub{
			createFn: func(_ context.Context, device *iotModels.Device) error {
				created = device
				return nil
			},
		},
		logger: slog.Default(),
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
	svc := &operatorProvisioningService{
		deviceRepo: &internalDeviceRepoStub{
			createFn: func(context.Context, *iotModels.Device) error {
				return newPgError("23505")
			},
		},
	}
	require.NoError(t, svc.createWebManualDevice(context.Background(), 42))
}

func TestCreateWebManualDevice_CreateError(t *testing.T) {
	svc := &operatorProvisioningService{
		deviceRepo: &internalDeviceRepoStub{
			createFn: func(context.Context, *iotModels.Device) error {
				return assert.AnError
			},
		},
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
	svc := &operatorProvisioningService{categoryRepo: &internalCategoryRepoStub{}}
	require.NoError(t, svc.seedDefaultActivityCategories(context.Background(), 0))
}

func TestSeedDefaultActivityCategories_UniqueViolationSkipped(t *testing.T) {
	var count int
	svc := &operatorProvisioningService{
		categoryRepo: &internalCategoryRepoStub{
			createFn: func(context.Context, *activityModels.Category) error {
				count++
				if count <= 2 {
					return newPgError("23505")
				}
				return nil
			},
		},
	}

	err := svc.seedDefaultActivityCategories(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 9, count)
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
			assert.Equal(t, tt.want, isUniqueViolation(tt.err))
		})
	}
}

// ---------------------------------------------------------------------------
// resolveSystemRoleByName
// ---------------------------------------------------------------------------

func TestResolveSystemRoleByName_NilRoleSkipped(t *testing.T) {
	svc := &operatorProvisioningService{
		roleRepo: &internalRoleRepoStub{
			listFn: func(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
				return []*authModels.Role{nil}, nil
			},
		},
	}
	role, err := svc.resolveSystemRoleByName(context.Background(), "admin")
	require.NoError(t, err)
	assert.Nil(t, role)
}

func TestResolveSystemRoleByName_NameMismatch(t *testing.T) {
	svc := &operatorProvisioningService{
		roleRepo: &internalRoleRepoStub{
			listFn: func(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
				return []*authModels.Role{
					{Model: modelBase.Model{ID: 1}, Name: "teacher", IsSystem: true},
				}, nil
			},
		},
	}
	role, err := svc.resolveSystemRoleByName(context.Background(), "admin")
	require.NoError(t, err)
	assert.Nil(t, role)
}

func TestResolveSystemRoleByName_TenantSpecificSkipped(t *testing.T) {
	tenantID := int64(5)
	svc := &operatorProvisioningService{
		roleRepo: &internalRoleRepoStub{
			listFn: func(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
				return []*authModels.Role{
					{Model: modelBase.Model{ID: 1}, Name: "admin", IsSystem: true, TenantID: &tenantID},
				}, nil
			},
		},
	}
	role, err := svc.resolveSystemRoleByName(context.Background(), "admin")
	require.NoError(t, err)
	assert.Nil(t, role)
}

func TestResolveSystemRoleByName_NonSystemSkipped(t *testing.T) {
	svc := &operatorProvisioningService{
		roleRepo: &internalRoleRepoStub{
			listFn: func(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
				return []*authModels.Role{
					{Model: modelBase.Model{ID: 1}, Name: "admin", IsSystem: false},
				}, nil
			},
		},
	}
	role, err := svc.resolveSystemRoleByName(context.Background(), "admin")
	require.NoError(t, err)
	assert.Nil(t, role)
}

func TestResolveSystemRoleByName_RepoError(t *testing.T) {
	svc := &operatorProvisioningService{
		roleRepo: &internalRoleRepoStub{
			listFn: func(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
				return nil, assert.AnError
			},
		},
	}
	role, err := svc.resolveSystemRoleByName(context.Background(), "admin")
	require.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, role)
}

func TestResolveSystemRoleByName_Success(t *testing.T) {
	svc := &operatorProvisioningService{
		roleRepo: &internalRoleRepoStub{
			listFn: func(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
				return []*authModels.Role{
					{Model: modelBase.Model{ID: 7}, Name: "admin", IsSystem: true},
				}, nil
			},
		},
	}
	role, err := svc.resolveSystemRoleByName(context.Background(), "admin")
	require.NoError(t, err)
	require.NotNil(t, role)
	assert.Equal(t, int64(7), role.ID)
}

// ---------------------------------------------------------------------------
// logAction
// ---------------------------------------------------------------------------

func TestLogAction_AuditLogError(t *testing.T) {
	svc := &operatorProvisioningService{
		auditLogRepo: &internalAuditLogRepoStub{
			createFn: func(context.Context, *platformModels.OperatorAuditLog) error {
				return assert.AnError
			},
		},
		logger: slog.Default(),
	}
	resourceID := int64(1)
	// Must not panic; error is logged but not returned.
	svc.logAction(context.Background(), 1, "create", "school", &resourceID, net.IPv4(127, 0, 0, 1), map[string]any{
		"name": "test",
	})
}

func TestLogAction_EmptyChanges(t *testing.T) {
	var entry *platformModels.OperatorAuditLog
	svc := &operatorProvisioningService{
		auditLogRepo: &internalAuditLogRepoStub{
			createFn: func(_ context.Context, e *platformModels.OperatorAuditLog) error {
				entry = e
				return nil
			},
		},
	}
	resourceID := int64(1)
	svc.logAction(context.Background(), 1, "create", "school", &resourceID, net.IPv4(127, 0, 0, 1), nil)
	require.NotNil(t, entry)
	assert.Nil(t, entry.Changes)
}

func TestLogAction_WithChanges(t *testing.T) {
	var entry *platformModels.OperatorAuditLog
	svc := &operatorProvisioningService{
		auditLogRepo: &internalAuditLogRepoStub{
			createFn: func(_ context.Context, e *platformModels.OperatorAuditLog) error {
				entry = e
				return nil
			},
		},
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
	err := mapSchoolCreateConflict(context.Background(), &internalSchoolRepoStub{}, &platformModels.School{
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
	svc := &operatorProvisioningService{
		schoolRepo: &internalSchoolRepoStub{
			findByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return nil, sql.ErrNoRows
			},
		},
	}
	school, err := svc.loadActiveSchool(context.Background(), 1)
	require.Nil(t, school)
	var notFoundErr *SchoolNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestLoadActiveSchool_DatabaseError(t *testing.T) {
	svc := &operatorProvisioningService{
		schoolRepo: &internalSchoolRepoStub{
			findByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return nil, &modelBase.DatabaseError{Op: "find", Err: sql.ErrNoRows}
			},
		},
	}
	school, err := svc.loadActiveSchool(context.Background(), 1)
	require.Nil(t, school)
	var notFoundErr *SchoolNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestLoadActiveSchool_GenericError(t *testing.T) {
	svc := &operatorProvisioningService{
		schoolRepo: &internalSchoolRepoStub{
			findByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return nil, assert.AnError
			},
		},
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
	svc := &operatorProvisioningService{
		organizationRepo: &internalOrgRepoStub{
			findBySlugFn: func(context.Context, string) (*platformModels.Organization, error) {
				return nil, assert.AnError
			},
		},
	}
	org, err := svc.CreateOrganization(context.Background(), &platformModels.Organization{
		Name: "Test", Slug: "test", Active: true,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, org)
	require.ErrorIs(t, err, assert.AnError)
}

func TestCreateOrganization_UniqueViolationOnCreate(t *testing.T) {
	svc := &operatorProvisioningService{
		organizationRepo: &internalOrgRepoStub{
			findBySlugFn: func(context.Context, string) (*platformModels.Organization, error) {
				return nil, nil
			},
			createFn: func(context.Context, *platformModels.Organization) error {
				return newPgError("23505")
			},
		},
	}
	org, err := svc.CreateOrganization(context.Background(), &platformModels.Organization{
		Name: "Test", Slug: "test", Active: true,
	}, 1, net.IPv4(127, 0, 0, 1))
	require.Nil(t, org)
	var conflictErr *ConflictError
	require.ErrorAs(t, err, &conflictErr)
}

func TestCreateOrganization_CreateError(t *testing.T) {
	svc := &operatorProvisioningService{
		organizationRepo: &internalOrgRepoStub{
			findBySlugFn: func(context.Context, string) (*platformModels.Organization, error) {
				return nil, nil
			},
			createFn: func(context.Context, *platformModels.Organization) error {
				return assert.AnError
			},
		},
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
	svc := &operatorProvisioningService{
		organizationRepo: &internalOrgRepoStub{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return nil, assert.AnError
			},
		},
		schoolRepo: &internalSchoolRepoStub{},
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
	svc := &operatorProvisioningService{
		organizationRepo: &internalOrgRepoStub{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: modelBase.Model{ID: 1}, Name: "Org", Slug: "org", Active: true}, nil
			},
		},
		schoolRepo: &internalSchoolRepoStub{
			createFn: func(context.Context, *platformModels.School) error {
				return newPgError("23505")
			},
		},
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
	svc := &operatorProvisioningService{
		organizationRepo: &internalOrgRepoStub{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: modelBase.Model{ID: 1}, Name: "Org", Slug: "org", Active: true}, nil
			},
		},
		schoolRepo: &internalSchoolRepoStub{
			createFn: func(_ context.Context, school *platformModels.School) error {
				school.ID = 55
				return nil
			},
		},
		categoryRepo: &internalCategoryRepoStub{},
		deviceRepo: &internalDeviceRepoStub{
			createFn: func(context.Context, *iotModels.Device) error {
				return assert.AnError
			},
		},
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
	svc := &operatorProvisioningService{
		organizationRepo: &internalOrgRepoStub{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: modelBase.Model{ID: 1}, Name: "Org", Slug: "org", Active: true}, nil
			},
		},
		schoolRepo: &internalSchoolRepoStub{
			createFn: func(_ context.Context, school *platformModels.School) error {
				school.ID = 55
				return nil
			},
		},
		categoryRepo: &internalCategoryRepoStub{},
		deviceRepo: &internalDeviceRepoStub{
			createFn: func(context.Context, *iotModels.Device) error {
				deviceCreated = true
				return nil
			},
		},
		auditLogRepo: &internalAuditLogRepoStub{},
		logger:       slog.Default(),
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
	svc := &operatorProvisioningService{
		schoolRepo: &internalSchoolRepoStub{
			findByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          modelBase.Model{ID: 9},
					OrganizationID: 3,
					Name:           "School",
					Slug:           "school",
					Subdomain:      "school",
					Active:         true,
				}, nil
			},
		},
		roleRepo: &internalRoleRepoStub{
			listFn: func(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
				return nil, assert.AnError
			},
		},
	}
	invitation, err := svc.InviteSchoolAdmin(context.Background(), 9, 11, net.IPv4(127, 0, 0, 1), authSvc.InvitationRequest{
		Email: "test@example.com",
	})
	require.Nil(t, invitation)
	require.ErrorIs(t, err, assert.AnError)
}

func TestInviteSchoolAdmin_InvitationCreateError(t *testing.T) {
	svc := &operatorProvisioningService{
		schoolRepo: &internalSchoolRepoStub{
			findByIDFn: func(context.Context, int64) (*platformModels.School, error) {
				return &platformModels.School{
					Model:          modelBase.Model{ID: 9},
					OrganizationID: 3,
					Name:           "School",
					Slug:           "school",
					Subdomain:      "school",
					Active:         true,
				}, nil
			},
		},
		roleRepo: &internalRoleRepoStub{
			listFn: func(context.Context, map[string]interface{}) ([]*authModels.Role, error) {
				return []*authModels.Role{
					{Model: modelBase.Model{ID: 4}, Name: "admin", IsSystem: true},
				}, nil
			},
		},
		invitationService: &failingInvitationServiceStub{},
	}
	invitation, err := svc.InviteSchoolAdmin(context.Background(), 9, 11, net.IPv4(127, 0, 0, 1), authSvc.InvitationRequest{
		Email: "test@example.com",
	})
	require.Nil(t, invitation)
	require.ErrorIs(t, err, assert.AnError)
}

// ---------------------------------------------------------------------------
// Minimal invitation service stub that always fails
// ---------------------------------------------------------------------------

type failingInvitationServiceStub struct{}

func (f *failingInvitationServiceStub) WithTx(_ bun.Tx) interface{} { return f } //nolint:unused

// ---------------------------------------------------------------------------
// ensureSchoolSlugAvailable / ensureSchoolSubdomainAvailable — repo errors
// ---------------------------------------------------------------------------

func TestEnsureSchoolSlugAvailable_RepoError(t *testing.T) {
	svc := &operatorProvisioningService{
		schoolRepo: &internalSchoolRepoStub{
			findByOrgAndSlugFn: func(context.Context, int64, string) (*platformModels.School, error) {
				return nil, assert.AnError
			},
		},
	}
	err := svc.ensureSchoolSlugAvailable(context.Background(), 1, "test")
	require.ErrorIs(t, err, assert.AnError)
}

func TestEnsureSchoolSubdomainAvailable_RepoError(t *testing.T) {
	svc := &operatorProvisioningService{
		schoolRepo: &internalSchoolRepoStub{
			findBySubdomainFn: func(context.Context, string) (*platformModels.School, error) {
				return nil, assert.AnError
			},
		},
	}
	err := svc.ensureSchoolSubdomainAvailable(context.Background(), "test")
	require.ErrorIs(t, err, assert.AnError)
}

// ---------------------------------------------------------------------------
// CreateSchool — non-unique create error
// ---------------------------------------------------------------------------

func TestCreateSchool_CreateNonUniqueError(t *testing.T) {
	svc := &operatorProvisioningService{
		organizationRepo: &internalOrgRepoStub{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Model: modelBase.Model{ID: 1}, Name: "Org", Slug: "org", Active: true}, nil
			},
		},
		schoolRepo: &internalSchoolRepoStub{
			createFn: func(context.Context, *platformModels.School) error {
				return assert.AnError
			},
		},
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
	err := svc.withAdminTx(ctx, func(context.Context) error {
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
func (f *failingInvitationServiceStub) CreateInvitation(context.Context, authSvc.InvitationRequest) (*authModels.InvitationToken, error) {
	return nil, assert.AnError
}
func (f *failingInvitationServiceStub) ValidateInvitation(context.Context, string) (*authSvc.InvitationValidationResult, error) {
	return nil, nil
}
func (f *failingInvitationServiceStub) AcceptInvitation(context.Context, string, authSvc.UserRegistrationData) (*authModels.Account, error) {
	return nil, nil
}
func (f *failingInvitationServiceStub) ResendInvitation(context.Context, int64, int64) error {
	return nil
}
func (f *failingInvitationServiceStub) ListPendingInvitations(context.Context) ([]*authModels.InvitationToken, error) {
	return nil, nil
}
func (f *failingInvitationServiceStub) RevokeInvitation(context.Context, int64, int64) error {
	return nil
}
func (f *failingInvitationServiceStub) CleanupExpiredInvitations(context.Context) (int, error) {
	return 0, nil
}
