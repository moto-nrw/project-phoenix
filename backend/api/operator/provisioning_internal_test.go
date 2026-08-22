package operator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	jwtPkg "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/seedtoken"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type mockProvisioningService struct {
	createOrganizationFn      func(context.Context, *platformModels.Organization, int64, net.IP) (*platformModels.Organization, error)
	listOrganizationsFn       func(context.Context) ([]*platformModels.Organization, error)
	updateOrganizationFn      func(context.Context, int64, platformSvc.UpdateOrganizationRequest, int64, net.IP) (*platformModels.Organization, error)
	createSchoolFn            func(context.Context, *platformModels.School, int64, net.IP) (*platformModels.School, error)
	listSchoolsFn             func(context.Context) ([]*platformModels.School, error)
	updateSchoolFn            func(context.Context, int64, platformSvc.UpdateSchoolRequest, int64, net.IP) (*platformModels.School, error)
	inviteSchoolAdminFn       func(context.Context, int64, int64, net.IP, authSvc.InvitationRequest) (*authModels.InvitationToken, error)
	createSchoolAccountFn     func(context.Context, int64, int64, net.IP, platformSvc.CreateSchoolAccountRequest) (*authModels.Account, error)
	listSchoolAccountsFn      func(context.Context, int64) ([]authModels.TenantAccountInfo, error)
	listOrgAccountsFn         func(context.Context, int64) ([]authModels.OrgAccountInfo, error)
	listAllAccountsFn         func(context.Context) ([]authModels.OrgAccountInfo, error)
	listSystemRolesFn         func(context.Context) ([]*authModels.Role, error)
	listAllDevicesFn          func(context.Context) ([]platformSvc.OperatorDeviceInfo, error)
	listSchoolDevicesFn       func(context.Context, int64) ([]platformSvc.OperatorDeviceInfo, error)
	listOrganizationDevicesFn func(context.Context, int64) ([]platformSvc.OperatorDeviceInfo, error)
	createDeviceFn            func(context.Context, int64, string, string, *string, *string, int64, net.IP) (*platformSvc.OperatorDeviceInfo, error)
	setDeviceAPIKeyFn         func(context.Context, int64, *string, int64, net.IP) (*platformSvc.OperatorDeviceInfo, error)
	getDeviceTransferStatusFn func(context.Context, int64) (*platformSvc.DeviceTransferStatus, error)
	transferDeviceFn          func(context.Context, int64, int64, int64, net.IP) (*platformSvc.OperatorDeviceInfo, error)
	softDeleteSchoolFn        func(int64) error
	restoreSchoolFn           func(int64) error
	softDeleteOrgFn           func(int64) error
	restoreOrgFn              func(int64) error
	deleteDeviceFn            func(context.Context, int64, int64, net.IP) error
	listSchoolPersonsFn       func(context.Context, int64) ([]platformSvc.OperatorPersonInfo, error)
	softDeletePersonFn        func(context.Context, int64, int64, net.IP) error
	getProvisioningStatsFn    func(context.Context) (*platformSvc.ProvisioningStats, error)
	getSchoolPWAUsageFn       func(context.Context, int64) (*platformSvc.SchoolPWAUsage, error)
	listOrgSummariesFn        func(context.Context) ([]*platformSvc.OrganizationSummary, error)
	listSchoolSummariesFn     func(context.Context) ([]*platformSvc.SchoolSummary, error)
	listOrgSchoolSummariesFn  func(context.Context, int64) ([]*platformSvc.SchoolSummary, error)
	listOrgPersonsFn          func(context.Context, int64) ([]platformSvc.OperatorPersonInfo, error)
	listTenantAccessFn        func(context.Context, int64) ([]platformSvc.AccountTenantAccessEntry, error)
	listAssignableRolesFn     func(context.Context, int64) ([]*authModels.Role, error)
	grantTenantAccessFn       func(context.Context, int64, int64, platformSvc.GrantAccountTenantAccessRequest, int64, net.IP) ([]platformSvc.AccountTenantAccessEntry, error)
	updateTenantRoleFn        func(context.Context, int64, int64, int64, int64, net.IP) ([]platformSvc.AccountTenantAccessEntry, error)
	revokeTenantAccessFn      func(context.Context, int64, int64, int64, net.IP) ([]platformSvc.AccountTenantAccessEntry, error)
}

func (m *mockProvisioningService) ListAccountTenantAccess(ctx context.Context, accountID int64) ([]platformSvc.AccountTenantAccessEntry, error) {
	if m.listTenantAccessFn != nil {
		return m.listTenantAccessFn(ctx, accountID)
	}
	return nil, nil
}

func (m *mockProvisioningService) ListAssignableSchoolRoles(ctx context.Context, schoolID int64) ([]*authModels.Role, error) {
	if m.listAssignableRolesFn != nil {
		return m.listAssignableRolesFn(ctx, schoolID)
	}
	return nil, nil
}

func (m *mockProvisioningService) GrantAccountTenantAccess(ctx context.Context, accountID, schoolID int64, req platformSvc.GrantAccountTenantAccessRequest, operatorID int64, clientIP net.IP) ([]platformSvc.AccountTenantAccessEntry, error) {
	if m.grantTenantAccessFn != nil {
		return m.grantTenantAccessFn(ctx, accountID, schoolID, req, operatorID, clientIP)
	}
	return nil, nil
}

func (m *mockProvisioningService) UpdateAccountTenantRole(ctx context.Context, accountID, schoolID, roleID, operatorID int64, clientIP net.IP) ([]platformSvc.AccountTenantAccessEntry, error) {
	if m.updateTenantRoleFn != nil {
		return m.updateTenantRoleFn(ctx, accountID, schoolID, roleID, operatorID, clientIP)
	}
	return nil, nil
}

func (m *mockProvisioningService) RevokeAccountTenantAccess(ctx context.Context, accountID, schoolID, operatorID int64, clientIP net.IP) ([]platformSvc.AccountTenantAccessEntry, error) {
	if m.revokeTenantAccessFn != nil {
		return m.revokeTenantAccessFn(ctx, accountID, schoolID, operatorID, clientIP)
	}
	return nil, nil
}

func (m *mockProvisioningService) CreateOrganization(ctx context.Context, org *platformModels.Organization, operatorID int64, clientIP net.IP) (*platformModels.Organization, error) {
	return m.createOrganizationFn(ctx, org, operatorID, clientIP)
}
func (m *mockProvisioningService) ListOrganizations(ctx context.Context) ([]*platformModels.Organization, error) {
	return m.listOrganizationsFn(ctx)
}
func (m *mockProvisioningService) UpdateOrganization(ctx context.Context, id int64, req platformSvc.UpdateOrganizationRequest, operatorID int64, clientIP net.IP) (*platformModels.Organization, error) {
	if m.updateOrganizationFn != nil {
		return m.updateOrganizationFn(ctx, id, req, operatorID, clientIP)
	}
	return nil, nil
}
func (m *mockProvisioningService) CreateSchool(ctx context.Context, school *platformModels.School, operatorID int64, clientIP net.IP) (*platformModels.School, error) {
	return m.createSchoolFn(ctx, school, operatorID, clientIP)
}
func (m *mockProvisioningService) ListSchools(ctx context.Context) ([]*platformModels.School, error) {
	return m.listSchoolsFn(ctx)
}
func (m *mockProvisioningService) UpdateSchool(ctx context.Context, id int64, req platformSvc.UpdateSchoolRequest, operatorID int64, clientIP net.IP) (*platformModels.School, error) {
	if m.updateSchoolFn != nil {
		return m.updateSchoolFn(ctx, id, req, operatorID, clientIP)
	}
	return nil, nil
}
func (m *mockProvisioningService) InviteSchoolAdmin(ctx context.Context, schoolID, operatorID int64, clientIP net.IP, req authSvc.InvitationRequest) (*authModels.InvitationToken, error) {
	return m.inviteSchoolAdminFn(ctx, schoolID, operatorID, clientIP, req)
}
func (m *mockProvisioningService) CreateSchoolAccount(ctx context.Context, schoolID, operatorID int64, clientIP net.IP, req platformSvc.CreateSchoolAccountRequest) (*authModels.Account, error) {
	if m.createSchoolAccountFn != nil {
		return m.createSchoolAccountFn(ctx, schoolID, operatorID, clientIP, req)
	}
	return nil, errors.New("not implemented")
}
func (m *mockProvisioningService) ListSystemRoles(ctx context.Context) ([]*authModels.Role, error) {
	if m.listSystemRolesFn != nil {
		return m.listSystemRolesFn(ctx)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListSchoolAccounts(ctx context.Context, schoolID int64) ([]authModels.TenantAccountInfo, error) {
	if m.listSchoolAccountsFn != nil {
		return m.listSchoolAccountsFn(ctx, schoolID)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListOrganizationAccounts(ctx context.Context, orgID int64) ([]authModels.OrgAccountInfo, error) {
	if m.listOrgAccountsFn != nil {
		return m.listOrgAccountsFn(ctx, orgID)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListAllAccounts(ctx context.Context) ([]authModels.OrgAccountInfo, error) {
	if m.listAllAccountsFn != nil {
		return m.listAllAccountsFn(ctx)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListAllDevices(ctx context.Context) ([]platformSvc.OperatorDeviceInfo, error) {
	if m.listAllDevicesFn != nil {
		return m.listAllDevicesFn(ctx)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListSchoolDevices(ctx context.Context, schoolID int64) ([]platformSvc.OperatorDeviceInfo, error) {
	if m.listSchoolDevicesFn != nil {
		return m.listSchoolDevicesFn(ctx, schoolID)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListOrganizationDevices(ctx context.Context, orgID int64) ([]platformSvc.OperatorDeviceInfo, error) {
	if m.listOrganizationDevicesFn != nil {
		return m.listOrganizationDevicesFn(ctx, orgID)
	}
	return nil, nil
}
func (m *mockProvisioningService) CreateDevice(ctx context.Context, schoolID int64, deviceID, deviceType string, name, apiKey *string, operatorID int64, clientIP net.IP) (*platformSvc.OperatorDeviceInfo, error) {
	if m.createDeviceFn != nil {
		return m.createDeviceFn(ctx, schoolID, deviceID, deviceType, name, apiKey, operatorID, clientIP)
	}
	return nil, nil
}
func (m *mockProvisioningService) SetDeviceAPIKey(ctx context.Context, deviceID int64, apiKey *string, operatorID int64, clientIP net.IP) (*platformSvc.OperatorDeviceInfo, error) {
	if m.setDeviceAPIKeyFn != nil {
		return m.setDeviceAPIKeyFn(ctx, deviceID, apiKey, operatorID, clientIP)
	}
	return nil, nil
}
func (m *mockProvisioningService) GetDeviceTransferStatus(ctx context.Context, deviceID int64) (*platformSvc.DeviceTransferStatus, error) {
	if m.getDeviceTransferStatusFn != nil {
		return m.getDeviceTransferStatusFn(ctx, deviceID)
	}
	return nil, nil
}
func (m *mockProvisioningService) TransferDevice(ctx context.Context, deviceID, targetSchoolID, operatorID int64, clientIP net.IP) (*platformSvc.OperatorDeviceInfo, error) {
	if m.transferDeviceFn != nil {
		return m.transferDeviceFn(ctx, deviceID, targetSchoolID, operatorID, clientIP)
	}
	return nil, nil
}
func (m *mockProvisioningService) SoftDeleteSchool(_ context.Context, schoolID, _ int64, _ net.IP) error {
	if m.softDeleteSchoolFn != nil {
		return m.softDeleteSchoolFn(schoolID)
	}
	return nil
}
func (m *mockProvisioningService) RestoreSchool(_ context.Context, schoolID, _ int64, _ net.IP) error {
	if m.restoreSchoolFn != nil {
		return m.restoreSchoolFn(schoolID)
	}
	return nil
}
func (m *mockProvisioningService) SoftDeleteOrganization(_ context.Context, orgID int64, _ int64, _ net.IP) error {
	if m.softDeleteOrgFn != nil {
		return m.softDeleteOrgFn(orgID)
	}
	return nil
}
func (m *mockProvisioningService) RestoreOrganization(_ context.Context, orgID int64, _ int64, _ net.IP) error {
	if m.restoreOrgFn != nil {
		return m.restoreOrgFn(orgID)
	}
	return nil
}
func (m *mockProvisioningService) DeleteDevice(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
	if m.deleteDeviceFn != nil {
		return m.deleteDeviceFn(ctx, id, operatorID, clientIP)
	}
	return nil
}
func (m *mockProvisioningService) ListSchoolPersons(ctx context.Context, schoolID int64) ([]platformSvc.OperatorPersonInfo, error) {
	if m.listSchoolPersonsFn != nil {
		return m.listSchoolPersonsFn(ctx, schoolID)
	}
	return nil, nil
}
func (m *mockProvisioningService) SoftDeletePerson(ctx context.Context, personID int64, operatorID int64, clientIP net.IP) error {
	if m.softDeletePersonFn != nil {
		return m.softDeletePersonFn(ctx, personID, operatorID, clientIP)
	}
	return nil
}
func (m *mockProvisioningService) GetProvisioningStats(ctx context.Context) (*platformSvc.ProvisioningStats, error) {
	if m.getProvisioningStatsFn != nil {
		return m.getProvisioningStatsFn(ctx)
	}
	return nil, nil
}
func (m *mockProvisioningService) GetSchoolPWAUsage(ctx context.Context, schoolID int64) (*platformSvc.SchoolPWAUsage, error) {
	if m.getSchoolPWAUsageFn != nil {
		return m.getSchoolPWAUsageFn(ctx, schoolID)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListOrganizationSummaries(ctx context.Context) ([]*platformSvc.OrganizationSummary, error) {
	if m.listOrgSummariesFn != nil {
		return m.listOrgSummariesFn(ctx)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListSchoolSummaries(ctx context.Context) ([]*platformSvc.SchoolSummary, error) {
	if m.listSchoolSummariesFn != nil {
		return m.listSchoolSummariesFn(ctx)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListOrganizationSchoolSummaries(ctx context.Context, organizationID int64) ([]*platformSvc.SchoolSummary, error) {
	if m.listOrgSchoolSummariesFn != nil {
		return m.listOrgSchoolSummariesFn(ctx, organizationID)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListOrganizationPersons(ctx context.Context, organizationID int64) ([]platformSvc.OperatorPersonInfo, error) {
	if m.listOrgPersonsFn != nil {
		return m.listOrgPersonsFn(ctx, organizationID)
	}
	return nil, nil
}

type mockCaregiverCapabilityService struct {
	getFn     func(context.Context, int64) (*userModels.CaregiverCapabilityState, error)
	enableFn  func(context.Context, int64, userModels.EnableCaregiverCapabilityInput) (*userModels.CaregiverCapabilityState, error)
	disableFn func(context.Context, int64) (*userModels.CaregiverCapabilityState, error)
}

func (m *mockCaregiverCapabilityService) GetCaregiverCapability(ctx context.Context, accountID int64) (*userModels.CaregiverCapabilityState, error) {
	if m.getFn != nil {
		return m.getFn(ctx, accountID)
	}
	return nil, nil
}

func newMockAdminDB(t *testing.T) (*bun.DB, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	return bun.NewDB(sqlDB, pgdialect.New()), mock
}

func (m *mockCaregiverCapabilityService) EnableCaregiverCapability(ctx context.Context, accountID int64, input userModels.EnableCaregiverCapabilityInput) (*userModels.CaregiverCapabilityState, error) {
	if m.enableFn != nil {
		return m.enableFn(ctx, accountID, input)
	}
	return nil, nil
}

func (m *mockCaregiverCapabilityService) DisableCaregiverCapability(ctx context.Context, accountID int64) (*userModels.CaregiverCapabilityState, error) {
	if m.disableFn != nil {
		return m.disableFn(ctx, accountID)
	}
	return nil, nil
}

func withOperatorClaims(req *http.Request, operatorID int) *http.Request {
	claims := jwtPkg.AppClaims{ID: operatorID, Scope: "platform"}
	return req.WithContext(context.WithValue(req.Context(), jwtPkg.CtxClaims, claims))
}

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), rr.Body.String())
	return body
}

func TestProvisioningResource_CreateOrganization(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		createOrganizationFn: func(_ context.Context, org *platformModels.Organization, operatorID int64, clientIP net.IP) (*platformModels.Organization, error) {
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, "Stadt Koeln", org.Name)
			assert.Equal(t, "stadt-koeln", org.Slug)
			assert.Equal(t, "203.0.113.10", clientIP.String())
			org.ID = 55
			return org, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/operator/organizations", bytes.NewBufferString(`{"name":"  Stadt Koeln ","slug":" stadt-koeln "}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.CreateOrganization(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	body := decodeBody(t, rr)
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(55), data["id"])
}

func TestProvisioningResource_CreateOrganization_InvalidRequest(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodPost, "/operator/organizations", bytes.NewBufferString(`{"name":`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.CreateOrganization(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_ListOrganizations(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listOrganizationsFn: func(context.Context) ([]*platformModels.Organization, error) {
			return []*platformModels.Organization{{Name: "Org", Slug: "org"}}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/operator/organizations", nil)
	rr := httptest.NewRecorder()

	resource.ListOrganizations(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestProvisioningResource_ListOrganizations_Error(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listOrganizationsFn: func(context.Context) ([]*platformModels.Organization, error) {
			return nil, errors.New("db fail")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/operator/organizations", nil)
	rr := httptest.NewRecorder()

	resource.ListOrganizations(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestProvisioningResource_CreateSchool(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		createSchoolFn: func(_ context.Context, school *platformModels.School, operatorID int64, clientIP net.IP) (*platformModels.School, error) {
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, int64(7), school.OrganizationID)
			assert.Equal(t, "school@example.com", school.Email)
			assert.Equal(t, "198.51.100.20", clientIP.String())
			school.ID = 88
			return school, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/operator/schools", bytes.NewBufferString(`{"organization_id":7,"name":" Test School ","slug":" test-school ","subdomain":" test-sub ","email":" SCHOOL@EXAMPLE.COM "}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.20:4444"
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.CreateSchool(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	body := decodeBody(t, rr)
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(88), data["id"])
}

func TestProvisioningResource_ListSchools(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listSchoolsFn: func(context.Context) ([]*platformModels.School, error) {
			return []*platformModels.School{{Name: "School", Slug: "school"}}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/operator/schools", nil)
	rr := httptest.NewRecorder()

	resource.ListSchools(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestProvisioningResource_CreateSchool_InvalidRequest(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodPost, "/operator/schools", bytes.NewBufferString(`{"organization_id":`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.CreateSchool(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_ListSchools_Error(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listSchoolsFn: func(context.Context) ([]*platformModels.School, error) {
			return nil, errors.New("db fail")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/operator/schools", nil)
	rr := httptest.NewRecorder()

	resource.ListSchools(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestProvisioningResource_InviteSchoolAdmin(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour).UTC()
	first := "Ada"
	last := "Lovelace"
	position := "Principal"
	token := "seed-token"
	roleName := "admin"
	creatorEmail := "operator@example.com"

	resource := NewProvisioningResource(&mockProvisioningService{
		inviteSchoolAdminFn: func(_ context.Context, schoolID, operatorID int64, clientIP net.IP, req authSvc.InvitationRequest) (*authModels.InvitationToken, error) {
			assert.Equal(t, int64(12), schoolID)
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, "203.0.113.5", clientIP.String())
			require.NotNil(t, req.FirstName)
			require.NotNil(t, req.LastName)
			require.NotNil(t, req.Position)
			assert.Equal(t, "principal@example.com", req.Email)
			assert.True(t, req.CaregiverEnabled)
			return &authModels.InvitationToken{
				Model:            modelBase.Model{ID: 5},
				Email:            req.Email,
				RoleID:           9,
				Token:            token,
				ExpiresAt:        expiresAt,
				FirstName:        &first,
				LastName:         &last,
				Position:         &position,
				CaregiverEnabled: req.CaregiverEnabled,
				CreatedBy:        nil,
				Role:             &authModels.Role{Name: roleName},
				Creator:          &authModels.Account{Email: creatorEmail},
				EmailError:       nil,
			}, nil
		},
	})

	prevEnv := viper.GetString("app_env")
	viper.Set("app_env", "development")
	t.Cleanup(func() { viper.Set("app_env", prevEnv) })

	req := httptest.NewRequest(http.MethodPost, "http://localhost/operator/schools/12/invite-admin", bytes.NewBufferString(`{"email":" PRINCIPAL@example.com ","first_name":" Ada ","last_name":" Lovelace ","position":" Principal ","caregiver_enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(seedtoken.Header, "true")
	req.RemoteAddr = "203.0.113.5:9999"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "12")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.InviteSchoolAdmin(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	body := decodeBody(t, rr)
	data := body["data"].(map[string]any)
	assert.Equal(t, token, data["token"])
	assert.Equal(t, float64(0), data["created_by"])
	assert.Equal(t, roleName, data["role_name"])
	assert.Equal(t, creatorEmail, data["creator"])
	assert.Equal(t, true, data["caregiver_enabled"])
}

func TestProvisioningResource_InviteSchoolAdmin_InvalidSchoolID(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodPost, "/operator/schools/nope/invite-admin", bytes.NewBufferString(`{"email":"principal@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "nope")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.InviteSchoolAdmin(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_InviteSchoolAdmin_ServiceValidationError(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		inviteSchoolAdminFn: func(context.Context, int64, int64, net.IP, authSvc.InvitationRequest) (*authModels.InvitationToken, error) {
			return nil, &authSvc.AuthError{Op: "create invitation", Err: errors.New("invalid email")}
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/operator/schools/12/invite-admin", bytes.NewBufferString(`{"email":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "12")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.InviteSchoolAdmin(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestProvisioningHelpers(t *testing.T) {
	errText := "boom"
	now := time.Now()
	prevEnv := viper.GetString("app_env")
	t.Cleanup(func() { viper.Set("app_env", prevEnv) })

	assert.Equal(t, int64(0), operatorInvitationCreatedByValue(nil))
	assert.Equal(t, int64(9), operatorInvitationCreatedByValue(ptrInt64(9)))
	assert.Equal(t, "pending", operatorInvitationDeliveryStatus(nil, nil))
	assert.Equal(t, "failed", operatorInvitationDeliveryStatus(nil, &errText))
	assert.Equal(t, "sent", operatorInvitationDeliveryStatus(&now, &errText))

	viper.Set("app_env", "development")
	req := httptest.NewRequest(http.MethodPost, "http://localhost/", nil)
	assert.False(t, shouldExposeSeedInvitationToken(req))
	req.Header.Set(seedtoken.Header, "true")
	assert.True(t, shouldExposeSeedInvitationToken(req))
	viper.Set("app_env", "production")
	assert.False(t, shouldExposeSeedInvitationToken(req))
	viper.Set("app_env", "staging")
	assert.False(t, shouldExposeSeedInvitationToken(req))
	viper.Set("app_env", "developement")
	assert.False(t, shouldExposeSeedInvitationToken(req))

	viper.Set("app_env", "development")
	publicReq := httptest.NewRequest(http.MethodPost, "https://api-staging.moto-app.de/", nil)
	publicReq.Header.Set(seedtoken.Header, "true")
	assert.False(t, shouldExposeSeedInvitationToken(publicReq))
}

func TestProvisioningErrorRenderer_NotFoundAndFallbacks(t *testing.T) {
	t.Parallel()

	cases := []struct {
		err        error
		statusCode int
	}{
		{err: &platformSvc.OrganizationNotFoundError{OrganizationID: 1}, statusCode: http.StatusNotFound},
		{err: &platformSvc.SchoolNotFoundError{SchoolID: 1}, statusCode: http.StatusNotFound},
		{err: &platformSvc.InvalidDataError{Err: errors.New("bad")}, statusCode: http.StatusBadRequest},
		{err: errors.New("plain"), statusCode: http.StatusInternalServerError},
	}

	for _, tc := range cases {
		renderer := ProvisioningErrorRenderer(tc.err)
		resp, ok := renderer.(*ErrResponse)
		require.True(t, ok)
		assert.Equal(t, tc.statusCode, resp.HTTPStatusCode)
	}
}

func TestProvisioningResource_UpdateOrganization(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		updateOrganizationFn: func(_ context.Context, id int64, req platformSvc.UpdateOrganizationRequest, operatorID int64, clientIP net.IP) (*platformModels.Organization, error) {
			assert.Equal(t, int64(5), id)
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, "Updated Org", req.Name)
			assert.Equal(t, "updated-org", req.Slug)
			assert.True(t, req.Active)
			assert.Equal(t, "203.0.113.10", clientIP.String())
			return &platformModels.Organization{Model: modelBase.Model{ID: 5}, Name: "Updated Org", Slug: "updated-org", Active: true}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPut, "/operator/organizations/5", bytes.NewBufferString(`{"name":"Updated Org","slug":"updated-org","active":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "5")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.UpdateOrganization(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	body := decodeBody(t, rr)
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(5), data["id"])
	assert.Equal(t, "Updated Org", data["name"])
}

func TestProvisioningResource_UpdateOrganization_InvalidRequest(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodPut, "/operator/organizations/5", bytes.NewBufferString(`{"name":`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "5")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.UpdateOrganization(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_UpdateOrganization_InvalidID(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodPut, "/operator/organizations/abc", bytes.NewBufferString(`{"name":"Updated Org","slug":"updated-org","active":true}`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.UpdateOrganization(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_UpdateOrganization_NotFound(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		updateOrganizationFn: func(context.Context, int64, platformSvc.UpdateOrganizationRequest, int64, net.IP) (*platformModels.Organization, error) {
			return nil, &platformSvc.OrganizationNotFoundError{OrganizationID: 5}
		},
	})

	req := httptest.NewRequest(http.MethodPut, "/operator/organizations/5", bytes.NewBufferString(`{"name":"Updated Org","slug":"updated-org","active":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "5")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.UpdateOrganization(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestProvisioningResource_UpdateOrganization_Conflict(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		updateOrganizationFn: func(context.Context, int64, platformSvc.UpdateOrganizationRequest, int64, net.IP) (*platformModels.Organization, error) {
			return nil, &platformSvc.ConflictError{Err: errors.New("slug taken")}
		},
	})

	req := httptest.NewRequest(http.MethodPut, "/operator/organizations/5", bytes.NewBufferString(`{"name":"Updated Org","slug":"updated-org","active":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "5")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.UpdateOrganization(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestProvisioningResource_UpdateSchool(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		updateSchoolFn: func(_ context.Context, id int64, req platformSvc.UpdateSchoolRequest, operatorID int64, clientIP net.IP) (*platformModels.School, error) {
			assert.Equal(t, int64(10), id)
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, "Updated School", req.Name)
			assert.Equal(t, "updated-school", req.Slug)
			assert.Equal(t, "updated-sub", req.Subdomain)
			assert.Equal(t, "school@example.com", req.Email)
			assert.Equal(t, "198.51.100.20", clientIP.String())
			return &platformModels.School{Model: modelBase.Model{ID: 10}, Name: "Updated School", Slug: "updated-school", Subdomain: "updated-sub"}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPut, "/operator/schools/10", bytes.NewBufferString(`{"organization_id":7,"name":"Updated School","slug":"updated-school","subdomain":"updated-sub","address":"Main St 1","city":"Cologne","zip":"50667","phone":"0221-1234","email":"school@example.com","active":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.20:4444"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "10")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.UpdateSchool(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	body := decodeBody(t, rr)
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(10), data["id"])
}

func TestProvisioningResource_UpdateSchool_Hidden(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		updateSchoolFn: func(_ context.Context, id int64, req platformSvc.UpdateSchoolRequest, operatorID int64, _ net.IP) (*platformModels.School, error) {
			assert.Equal(t, int64(10), id)
			assert.True(t, req.Hidden, "Hidden field must be passed through from handler")
			assert.Equal(t, "Hidden School", req.Name)
			return &platformModels.School{Model: modelBase.Model{ID: 10}, Name: "Hidden School", Slug: "hidden", Subdomain: "hidden", Hidden: true}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPut, "/operator/schools/10", bytes.NewBufferString(`{"organization_id":7,"name":"Hidden School","slug":"hidden","subdomain":"hidden","active":true,"hidden":true}`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "10")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.UpdateSchool(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	body := decodeBody(t, rr)
	data := body["data"].(map[string]any)
	assert.Equal(t, true, data["hidden"])
}

func TestProvisioningResource_CreateSchool_Hidden(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		createSchoolFn: func(_ context.Context, school *platformModels.School, operatorID int64, _ net.IP) (*platformModels.School, error) {
			assert.True(t, school.Hidden, "Hidden field must be passed through from create handler")
			assert.Equal(t, "Demo School", school.Name)
			school.ID = 99
			return school, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/operator/schools", bytes.NewBufferString(`{"organization_id":7,"name":"Demo School","slug":"demo","subdomain":"demo","hidden":true}`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.CreateSchool(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	body := decodeBody(t, rr)
	data := body["data"].(map[string]any)
	assert.Equal(t, true, data["hidden"])
}

func TestProvisioningResource_UpdateSchool_InvalidRequest(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodPut, "/operator/schools/10", bytes.NewBufferString(`{"organization_id":`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "10")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.UpdateSchool(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_UpdateSchool_InvalidID(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodPut, "/operator/schools/nope", bytes.NewBufferString(`{"organization_id":7,"name":"School","slug":"school","subdomain":"sub","email":"a@b.com","active":true}`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "nope")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.UpdateSchool(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_UpdateSchool_NotFound(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		updateSchoolFn: func(context.Context, int64, platformSvc.UpdateSchoolRequest, int64, net.IP) (*platformModels.School, error) {
			return nil, &platformSvc.SchoolNotFoundError{SchoolID: 10}
		},
	})

	req := httptest.NewRequest(http.MethodPut, "/operator/schools/10", bytes.NewBufferString(`{"organization_id":7,"name":"School","slug":"school","subdomain":"sub","email":"a@b.com","active":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.20:4444"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "10")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.UpdateSchool(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestProvisioningResource_UpdateSchool_Conflict(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		updateSchoolFn: func(context.Context, int64, platformSvc.UpdateSchoolRequest, int64, net.IP) (*platformModels.School, error) {
			return nil, &platformSvc.ConflictError{Err: errors.New("subdomain taken")}
		},
	})

	req := httptest.NewRequest(http.MethodPut, "/operator/schools/10", bytes.NewBufferString(`{"organization_id":7,"name":"School","slug":"school","subdomain":"sub","email":"a@b.com","active":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.20:4444"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "10")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.UpdateSchool(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

// --- Bind method tests ---

func TestUpdateOrganizationRequest_Bind_MissingName(t *testing.T) {
	t.Parallel()

	req := &updateOrganizationRequest{Name: "", Slug: "valid-slug", Active: true}
	err := req.Bind(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestUpdateOrganizationRequest_Bind_MissingSlug(t *testing.T) {
	t.Parallel()

	req := &updateOrganizationRequest{Name: "Valid Name", Slug: "", Active: true}
	err := req.Bind(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slug is required")
}

func TestUpdateOrganizationRequest_Bind_TrimWhitespace(t *testing.T) {
	t.Parallel()

	req := &updateOrganizationRequest{Name: "  Org Name  ", Slug: "  org-slug  ", Active: true}
	err := req.Bind(nil)
	require.NoError(t, err)
	assert.Equal(t, "Org Name", req.Name)
	assert.Equal(t, "org-slug", req.Slug)
}

func TestUpdateSchoolRequest_Bind_MissingName(t *testing.T) {
	t.Parallel()

	req := &updateSchoolRequest{Name: "", Slug: "valid", Subdomain: "valid"}
	err := req.Bind(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestUpdateSchoolRequest_Bind_MissingSlug(t *testing.T) {
	t.Parallel()

	req := &updateSchoolRequest{Name: "Valid", Slug: "", Subdomain: "valid"}
	err := req.Bind(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slug is required")
}

func TestUpdateSchoolRequest_Bind_MissingSubdomain(t *testing.T) {
	t.Parallel()

	req := &updateSchoolRequest{Name: "Valid", Slug: "valid", Subdomain: ""}
	err := req.Bind(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subdomain is required")
}

func TestUpdateSchoolRequest_Bind_TrimAndLowercaseEmail(t *testing.T) {
	t.Parallel()

	req := &updateSchoolRequest{
		Name:      "  School  ",
		Slug:      "  school  ",
		Subdomain: "  school-sub  ",
		Address:   "  123 Main St  ",
		City:      "  Cologne  ",
		Zip:       "  50667  ",
		Phone:     "  +49 221  ",
		Email:     "  SCHOOL@EXAMPLE.COM  ",
		Active:    true,
	}
	err := req.Bind(nil)
	require.NoError(t, err)
	assert.Equal(t, "School", req.Name)
	assert.Equal(t, "school", req.Slug)
	assert.Equal(t, "school-sub", req.Subdomain)
	assert.Equal(t, "123 Main St", req.Address)
	assert.Equal(t, "Cologne", req.City)
	assert.Equal(t, "50667", req.Zip)
	assert.Equal(t, "+49 221", req.Phone)
	assert.Equal(t, "school@example.com", req.Email)
}

func TestCreateOrganizationRequest_Bind_TrimWhitespace(t *testing.T) {
	t.Parallel()

	req := &createOrganizationRequest{Name: "  Org  ", Slug: "  org-slug  "}
	err := req.Bind(nil)
	require.NoError(t, err)
	assert.Equal(t, "Org", req.Name)
	assert.Equal(t, "org-slug", req.Slug)
}

func TestCreateSchoolRequest_Bind_TrimAndLowercaseEmail(t *testing.T) {
	t.Parallel()

	req := &createSchoolRequest{
		Name:      "  School  ",
		Slug:      "  school  ",
		Subdomain: "  school-sub  ",
		Email:     "  SCHOOL@EXAMPLE.COM  ",
	}
	err := req.Bind(nil)
	require.NoError(t, err)
	assert.Equal(t, "School", req.Name)
	assert.Equal(t, "school", req.Slug)
	assert.Equal(t, "school-sub", req.Subdomain)
	assert.Equal(t, "school@example.com", req.Email)
}

func TestInviteSchoolAdminRequest_Bind_TrimAndLowercaseEmail(t *testing.T) {
	t.Parallel()

	req := &inviteSchoolAdminRequest{
		Email:            "  ADMIN@EXAMPLE.COM  ",
		FirstName:        "  Ada  ",
		LastName:         "  Lovelace  ",
		Position:         "  Principal  ",
		CaregiverEnabled: true,
	}
	err := req.Bind(nil)
	require.NoError(t, err)
	assert.Equal(t, "admin@example.com", req.Email)
	assert.Equal(t, "Ada", req.FirstName)
	assert.Equal(t, "Lovelace", req.LastName)
	assert.Equal(t, "Principal", req.Position)
	assert.True(t, req.CaregiverEnabled)
}

func TestCreateDeviceRequest_Bind_TrimWhitespace(t *testing.T) {
	t.Parallel()

	req := &createDeviceRequest{
		SchoolID:   9,
		DeviceID:   "  DEV-123  ",
		DeviceType: "  terminal  ",
		Name:       "  Eingang  ",
		APIKey:     "  custom-key  ",
	}

	err := req.Bind(nil)

	require.NoError(t, err)
	assert.Equal(t, "DEV-123", req.DeviceID)
	assert.Equal(t, "terminal", req.DeviceType)
	assert.Equal(t, "Eingang", req.Name)
	assert.Equal(t, "custom-key", req.APIKey)
}

func TestCreateDeviceRequest_Bind_RequiresSchoolID(t *testing.T) {
	t.Parallel()

	req := &createDeviceRequest{DeviceID: "DEV-123", DeviceType: "terminal"}

	err := req.Bind(nil)

	require.EqualError(t, err, "school_id is required")
}

func TestCreateDeviceRequest_Bind_RequiresDeviceID(t *testing.T) {
	t.Parallel()

	req := &createDeviceRequest{SchoolID: 9, DeviceType: "terminal"}

	err := req.Bind(nil)

	require.EqualError(t, err, "device_id is required")
}

func TestCreateDeviceRequest_Bind_RequiresDeviceType(t *testing.T) {
	t.Parallel()

	req := &createDeviceRequest{SchoolID: 9, DeviceID: "DEV-123"}

	err := req.Bind(nil)

	require.EqualError(t, err, "device_type is required")
}

func TestSetDeviceAPIKeyRequest_Bind_TrimWhitespace(t *testing.T) {
	t.Parallel()

	req := &setDeviceAPIKeyRequest{APIKey: "  manual-key  "}

	err := req.Bind(nil)

	require.NoError(t, err)
	assert.Equal(t, "manual-key", req.APIKey)
}

// --- Account listing handler tests ---

func TestProvisioningResource_ListSchoolAccounts(t *testing.T) {
	t.Parallel()

	expected := []authModels.TenantAccountInfo{
		{AccountID: 1, Email: "admin@example.com", Active: true, RoleName: "admin"},
	}
	resource := NewProvisioningResource(&mockProvisioningService{
		listSchoolAccountsFn: func(_ context.Context, schoolID int64) ([]authModels.TenantAccountInfo, error) {
			assert.Equal(t, int64(7), schoolID)
			return expected, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/schools/7/accounts", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "7")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListSchoolAccounts(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestProvisioningResource_ListSchoolAccounts_InvalidID(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodGet, "/operator/schools/abc/accounts", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListSchoolAccounts(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_ListSchoolAccounts_ServiceError(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listSchoolAccountsFn: func(_ context.Context, _ int64) ([]authModels.TenantAccountInfo, error) {
			return nil, &platformSvc.SchoolNotFoundError{SchoolID: 7}
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/schools/7/accounts", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "7")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListSchoolAccounts(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestProvisioningResource_ListOrganizationAccounts(t *testing.T) {
	t.Parallel()

	expected := []authModels.OrgAccountInfo{
		{TenantAccountInfo: authModels.TenantAccountInfo{AccountID: 1, Email: "admin@example.com"}, SchoolID: 9, SchoolName: "School A"},
	}
	resource := NewProvisioningResource(&mockProvisioningService{
		listOrgAccountsFn: func(_ context.Context, orgID int64) ([]authModels.OrgAccountInfo, error) {
			assert.Equal(t, int64(3), orgID)
			return expected, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/organizations/3/accounts", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "3")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListOrganizationAccounts(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestProvisioningResource_ListOrganizationAccounts_InvalidID(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodGet, "/operator/organizations/nope/accounts", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "nope")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListOrganizationAccounts(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_ListOrganizationAccounts_ServiceError(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listOrgAccountsFn: func(_ context.Context, _ int64) ([]authModels.OrgAccountInfo, error) {
			return nil, &platformSvc.OrganizationNotFoundError{OrganizationID: 3}
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/organizations/3/accounts", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "3")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListOrganizationAccounts(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestProvisioningResource_ListAllAccounts(t *testing.T) {
	t.Parallel()

	expected := []authModels.OrgAccountInfo{
		{TenantAccountInfo: authModels.TenantAccountInfo{AccountID: 1, Email: "admin@example.com"}, SchoolID: 9},
	}
	resource := NewProvisioningResource(&mockProvisioningService{
		listAllAccountsFn: func(_ context.Context) ([]authModels.OrgAccountInfo, error) {
			return expected, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/accounts", nil)
	rr := httptest.NewRecorder()

	resource.ListAllAccounts(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestProvisioningResource_ListAllAccounts_Error(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listAllAccountsFn: func(_ context.Context) ([]authModels.OrgAccountInfo, error) {
			return nil, errors.New("db fail")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/accounts", nil)
	rr := httptest.NewRecorder()

	resource.ListAllAccounts(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// --- Device listing handler tests ---

func TestProvisioningResource_ListAllDevices(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listAllDevicesFn: func(_ context.Context) ([]platformSvc.OperatorDeviceInfo, error) {
			return []platformSvc.OperatorDeviceInfo{{ID: 1, DeviceID: "dev-1"}}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/devices", nil)
	rr := httptest.NewRecorder()

	resource.ListAllDevices(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestProvisioningResource_ListAllDevices_Error(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listAllDevicesFn: func(_ context.Context) ([]platformSvc.OperatorDeviceInfo, error) {
			return nil, errors.New("db fail")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/devices", nil)
	rr := httptest.NewRecorder()

	resource.ListAllDevices(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestProvisioningResource_ListSchoolDevices(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listSchoolDevicesFn: func(_ context.Context, schoolID int64) ([]platformSvc.OperatorDeviceInfo, error) {
			assert.Equal(t, int64(7), schoolID)
			return []platformSvc.OperatorDeviceInfo{{ID: 1, DeviceID: "dev-1"}}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/schools/7/devices", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "7")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListSchoolDevices(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestProvisioningResource_ListSchoolDevices_InvalidID(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodGet, "/operator/schools/nope/devices", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "nope")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListSchoolDevices(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_ListSchoolDevices_Error(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listSchoolDevicesFn: func(_ context.Context, _ int64) ([]platformSvc.OperatorDeviceInfo, error) {
			return nil, &platformSvc.SchoolNotFoundError{SchoolID: 7}
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/schools/7/devices", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "7")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListSchoolDevices(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestProvisioningResource_ListOrganizationDevices(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listOrganizationDevicesFn: func(_ context.Context, orgID int64) ([]platformSvc.OperatorDeviceInfo, error) {
			assert.Equal(t, int64(3), orgID)
			return []platformSvc.OperatorDeviceInfo{{ID: 1, DeviceID: "dev-1"}}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/organizations/3/devices", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "3")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListOrganizationDevices(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestProvisioningResource_ListOrganizationDevices_InvalidID(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodGet, "/operator/organizations/nope/devices", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "nope")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListOrganizationDevices(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_ListOrganizationDevices_Error(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listOrganizationDevicesFn: func(_ context.Context, _ int64) ([]platformSvc.OperatorDeviceInfo, error) {
			return nil, &platformSvc.OrganizationNotFoundError{OrganizationID: 3}
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/organizations/3/devices", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "3")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListOrganizationDevices(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestProvisioningResource_CreateDevice(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		createDeviceFn: func(_ context.Context, schoolID int64, deviceID, deviceType string, name, apiKey *string, operatorID int64, clientIP net.IP) (*platformSvc.OperatorDeviceInfo, error) {
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, int64(9), schoolID)
			assert.Equal(t, "DEV-123", deviceID)
			assert.Equal(t, "terminal", deviceType)
			require.NotNil(t, name)
			require.NotNil(t, apiKey)
			assert.Equal(t, "Eingang", *name)
			assert.Equal(t, "manual-key", *apiKey)
			assert.Equal(t, "203.0.113.77", clientIP.String())
			return &platformSvc.OperatorDeviceInfo{ID: 5, DeviceID: deviceID, APIKey: apiKey}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/operator/devices", bytes.NewBufferString(`{"school_id":9,"device_id":"  DEV-123  ","device_type":"  terminal  ","name":"  Eingang  ","api_key":"  manual-key  "}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.77:9876"
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.CreateDevice(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	body := decodeBody(t, rr)
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(5), data["id"])
	assert.Equal(t, "DEV-123", data["device_id"])
	assert.Equal(t, "manual-key", data["api_key"])
}

func TestProvisioningResource_CreateDevice_InvalidRequest(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodPost, "/operator/devices", bytes.NewBufferString(`{"school_id":0,"device_id":"","device_type":""}`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.CreateDevice(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_CreateDevice_Conflict(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		createDeviceFn: func(_ context.Context, _ int64, _, _ string, _, _ *string, _ int64, _ net.IP) (*platformSvc.OperatorDeviceInfo, error) {
			return nil, &platformSvc.ConflictError{Err: errors.New("api_key already exists")}
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/operator/devices", bytes.NewBufferString(`{"school_id":9,"device_id":"DEV-123","device_type":"terminal"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.CreateDevice(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestProvisioningResource_SetDeviceAPIKey(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		setDeviceAPIKeyFn: func(_ context.Context, deviceID int64, apiKey *string, operatorID int64, clientIP net.IP) (*platformSvc.OperatorDeviceInfo, error) {
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, int64(17), deviceID)
			require.NotNil(t, apiKey)
			assert.Equal(t, "manual-key", *apiKey)
			assert.Equal(t, "198.51.100.30", clientIP.String())
			return &platformSvc.OperatorDeviceInfo{ID: deviceID, APIKey: apiKey}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/operator/devices/17/set-api-key", bytes.NewBufferString(`{"api_key":"  manual-key  "}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.30:4444"
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "17")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.SetDeviceAPIKey(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	body := decodeBody(t, rr)
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(17), data["id"])
	assert.Equal(t, "manual-key", data["api_key"])
}

func TestProvisioningResource_SetDeviceAPIKey_AutoGenerated(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		setDeviceAPIKeyFn: func(_ context.Context, deviceID int64, apiKey *string, _ int64, _ net.IP) (*platformSvc.OperatorDeviceInfo, error) {
			assert.Equal(t, int64(17), deviceID)
			assert.Nil(t, apiKey)
			return &platformSvc.OperatorDeviceInfo{ID: deviceID, APIKey: ptrString("generated-key")}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/operator/devices/17/set-api-key", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "17")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.SetDeviceAPIKey(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestProvisioningResource_SetDeviceAPIKey_InvalidID(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodPost, "/operator/devices/nope/set-api-key", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "nope")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.SetDeviceAPIKey(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_SetDeviceAPIKey_NotFound(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		setDeviceAPIKeyFn: func(_ context.Context, _ int64, _ *string, _ int64, _ net.IP) (*platformSvc.OperatorDeviceInfo, error) {
			return nil, &platformSvc.OperatorDeviceNotFoundError{DeviceID: 17}
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/operator/devices/17/set-api-key", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "17")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.SetDeviceAPIKey(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestProvisioningResource_GetDeviceTransferStatus(t *testing.T) {
	t.Parallel()

	lastSeen := time.Now().Add(-10 * time.Minute)
	resource := NewProvisioningResource(&mockProvisioningService{
		getDeviceTransferStatusFn: func(_ context.Context, deviceID int64) (*platformSvc.DeviceTransferStatus, error) {
			assert.Equal(t, int64(17), deviceID)
			return &platformSvc.DeviceTransferStatus{CanTransfer: true, LastSeen: &lastSeen}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/devices/17/transfer-status", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "17")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.GetDeviceTransferStatus(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	body := decodeBody(t, rr)
	data := body["data"].(map[string]any)
	assert.Equal(t, true, data["can_transfer"])
	assert.Equal(t, false, data["is_protected"])
}

func TestProvisioningResource_TransferDevice(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		transferDeviceFn: func(_ context.Context, deviceID, targetSchoolID, operatorID int64, clientIP net.IP) (*platformSvc.OperatorDeviceInfo, error) {
			assert.Equal(t, int64(17), deviceID)
			assert.Equal(t, int64(23), targetSchoolID)
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, "198.51.100.30", clientIP.String())
			return &platformSvc.OperatorDeviceInfo{ID: 99, DeviceID: "DEV-17", SchoolID: targetSchoolID}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/operator/devices/17/transfer", bytes.NewBufferString(`{"target_school_id":23}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.30:4444"
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "17")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.TransferDevice(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	body := decodeBody(t, rr)
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(99), data["id"])
	assert.Equal(t, float64(23), data["school_id"])
}

func TestProvisioningResource_TransferDevice_Blocked(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		transferDeviceFn: func(_ context.Context, deviceID, _ int64, _ int64, _ net.IP) (*platformSvc.OperatorDeviceInfo, error) {
			return nil, &platformSvc.DeviceTransferBlockedError{DeviceID: deviceID, Reason: platformSvc.DeviceTransferBlockedOnline}
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/operator/devices/17/transfer", bytes.NewBufferString(`{"target_school_id":23}`))
	req.Header.Set("Content-Type", "application/json")
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "17")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.TransferDevice(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
}

// --- CreateSchoolAccount handler tests ---

func TestProvisioningResource_CreateSchoolAccount(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		createSchoolAccountFn: func(_ context.Context, schoolID, operatorID int64, clientIP net.IP, req platformSvc.CreateSchoolAccountRequest) (*authModels.Account, error) {
			assert.Equal(t, int64(7), schoolID)
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, "203.0.113.50", clientIP.String())
			assert.Equal(t, "teacher@example.com", req.Email)
			assert.Equal(t, "Ada", req.FirstName)
			assert.Equal(t, "Lovelace", req.LastName)
			assert.Equal(t, "Secure123!", req.Password)
			assert.Equal(t, "Lehrerin", req.Position)
			require.NotNil(t, req.RoleID)
			assert.Equal(t, int64(3), *req.RoleID)
			return &authModels.Account{Model: modelBase.Model{ID: 99}, Email: "teacher@example.com"}, nil
		},
	})

	body := `{"email":" TEACHER@example.com ","first_name":" Ada ","last_name":" Lovelace ","password":"Secure123!","confirm_password":"Secure123!","role_id":3,"position":" Lehrerin "}`
	req := httptest.NewRequest(http.MethodPost, "/operator/schools/7/create-account", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.50:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "7")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.CreateSchoolAccount(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	body2 := decodeBody(t, rr)
	data := body2["data"].(map[string]any)
	assert.Equal(t, float64(99), data["id"])
	assert.Equal(t, "teacher@example.com", data["email"])
}

func TestProvisioningResource_CreateSchoolAccount_InvalidSchoolID(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodPost, "/operator/schools/abc/create-account", bytes.NewBufferString(`{"email":"a@b.com","first_name":"A","last_name":"B","password":"Secure123!","confirm_password":"Secure123!"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.CreateSchoolAccount(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_CreateSchoolAccount_InvalidRequest(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodPost, "/operator/schools/7/create-account", bytes.NewBufferString(`{"email":"","first_name":"","last_name":"","password":"x","confirm_password":"y"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "7")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.CreateSchoolAccount(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_CreateSchoolAccount_ServiceError(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		createSchoolAccountFn: func(context.Context, int64, int64, net.IP, platformSvc.CreateSchoolAccountRequest) (*authModels.Account, error) {
			return nil, &authSvc.AuthError{Op: "create account", Err: authSvc.ErrEmailAlreadyExists}
		},
	})

	body := `{"email":"taken@example.com","first_name":"A","last_name":"B","password":"Secure123!","confirm_password":"Secure123!"}`
	req := httptest.NewRequest(http.MethodPost, "/operator/schools/7/create-account", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.50:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "7")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.CreateSchoolAccount(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

// --- ListSystemRoles handler tests ---

func TestProvisioningResource_ListSystemRoles(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listSystemRolesFn: func(_ context.Context) ([]*authModels.Role, error) {
			return []*authModels.Role{{Name: "admin"}, {Name: "teacher"}}, nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/operator/roles", nil)
	rr := httptest.NewRecorder()

	resource.ListSystemRoles(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestProvisioningResource_ListSystemRoles_Error(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listSystemRolesFn: func(_ context.Context) ([]*authModels.Role, error) {
			return nil, errors.New("db fail")
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/operator/roles", nil)
	rr := httptest.NewRecorder()

	resource.ListSystemRoles(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// --- createSchoolAccountRequest Bind tests ---

func TestCreateSchoolAccountRequest_Bind_TrimAndLowercaseEmail(t *testing.T) {
	t.Parallel()

	roleID := int64(3)
	req := &createSchoolAccountRequest{
		Email:           "  TEACHER@EXAMPLE.COM  ",
		FirstName:       "  Ada  ",
		LastName:        "  Lovelace  ",
		Password:        "Secure123!",
		ConfirmPassword: "Secure123!",
		RoleID:          &roleID,
		Position:        "  Lehrerin  ",
	}
	err := req.Bind(nil)
	require.NoError(t, err)
	assert.Equal(t, "teacher@example.com", req.Email)
	assert.Equal(t, "Ada", req.FirstName)
	assert.Equal(t, "Lovelace", req.LastName)
	assert.Equal(t, "Lehrerin", req.Position)
}

func TestCreateSchoolAccountRequest_Bind_RequiresEmail(t *testing.T) {
	t.Parallel()

	req := &createSchoolAccountRequest{Email: "", FirstName: "A", LastName: "B", Password: "Secure123!", ConfirmPassword: "Secure123!"}
	err := req.Bind(nil)
	require.EqualError(t, err, "email is required")
}

func TestCreateSchoolAccountRequest_Bind_RequiresFirstName(t *testing.T) {
	t.Parallel()

	req := &createSchoolAccountRequest{Email: "a@b.com", FirstName: "", LastName: "B", Password: "Secure123!", ConfirmPassword: "Secure123!"}
	err := req.Bind(nil)
	require.EqualError(t, err, "first name is required")
}

func TestCreateSchoolAccountRequest_Bind_RequiresLastName(t *testing.T) {
	t.Parallel()

	req := &createSchoolAccountRequest{Email: "a@b.com", FirstName: "A", LastName: "", Password: "Secure123!", ConfirmPassword: "Secure123!"}
	err := req.Bind(nil)
	require.EqualError(t, err, "last name is required")
}

func TestCreateSchoolAccountRequest_Bind_RequiresPassword(t *testing.T) {
	t.Parallel()

	req := &createSchoolAccountRequest{Email: "a@b.com", FirstName: "A", LastName: "B", Password: "", ConfirmPassword: ""}
	err := req.Bind(nil)
	require.EqualError(t, err, "password is required")
}

func TestCreateSchoolAccountRequest_Bind_PasswordsMustMatch(t *testing.T) {
	t.Parallel()

	req := &createSchoolAccountRequest{Email: "a@b.com", FirstName: "A", LastName: "B", Password: "Secure123!", ConfirmPassword: "Different!"}
	err := req.Bind(nil)
	require.EqualError(t, err, "passwords do not match")
}

// --- Additional ProvisioningErrorRenderer cases ---

func TestProvisioningErrorRenderer_SchoolInactive(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&platformSvc.SchoolInactiveError{SchoolID: 1})
	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusForbidden, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "inactive")
}

func TestProvisioningErrorRenderer_DeviceNotFound(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&platformSvc.OperatorDeviceNotFoundError{DeviceID: 42})
	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
}

func TestProvisioningErrorRenderer_AuthEmailAlreadyExists(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&authSvc.AuthError{Op: "create", Err: authSvc.ErrEmailAlreadyExists})
	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
}

func TestProvisioningErrorRenderer_AuthUsernameAlreadyExists(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&authSvc.AuthError{Op: "create", Err: authSvc.ErrUsernameAlreadyExists})
	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
}

func TestProvisioningErrorRenderer_AuthPasswordMismatch(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&authSvc.AuthError{Op: "create", Err: authSvc.ErrPasswordMismatch})
	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
}

func TestProvisioningErrorRenderer_AuthPasswordTooWeak(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&authSvc.AuthError{Op: "create", Err: authSvc.ErrPasswordTooWeak})
	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
}

func TestProvisioningErrorRenderer_AuthInvitationNameRequired(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&authSvc.AuthError{Op: "create invitation", Err: authSvc.ErrInvitationNameRequired})
	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
}

func TestProvisioningErrorRenderer_AuthGenericInvitationError(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&authSvc.AuthError{Op: "create invitation", Err: errors.New("some validation error")})
	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
}

func TestProvisioningErrorRenderer_AuthDefaultError(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&authSvc.AuthError{Op: "some op", Err: errors.New("db error")})
	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
}

// --- Person handler tests ---

func TestProvisioningResource_ListSchoolPersons_Success(t *testing.T) {
	t.Parallel()

	expected := []platformSvc.OperatorPersonInfo{
		{ID: 10, FirstName: "Ada", LastName: "Lovelace", SchoolID: 7, SchoolName: "Test School"},
		{ID: 11, FirstName: "Grace", LastName: "Hopper", SchoolID: 7, SchoolName: "Test School"},
	}
	resource := NewProvisioningResource(&mockProvisioningService{
		listSchoolPersonsFn: func(_ context.Context, schoolID int64) ([]platformSvc.OperatorPersonInfo, error) {
			assert.Equal(t, int64(7), schoolID)
			return expected, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/schools/7/persons", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "7")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListSchoolPersons(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	body := decodeBody(t, rr)
	data := body["data"].([]any)
	assert.Len(t, data, 2)
	first := data[0].(map[string]any)
	assert.Equal(t, float64(10), first["id"])
	assert.Equal(t, "Ada", first["first_name"])
}

func TestProvisioningResource_ListSchoolPersons_SchoolNotFound(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listSchoolPersonsFn: func(_ context.Context, _ int64) ([]platformSvc.OperatorPersonInfo, error) {
			return nil, &platformSvc.SchoolNotFoundError{SchoolID: 7}
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/schools/7/persons", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "7")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListSchoolPersons(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestProvisioningResource_ListSchoolPersons_InvalidID(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodGet, "/operator/schools/nope/persons", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "nope")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListSchoolPersons(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_SoftDeletePerson_Success(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		softDeletePersonFn: func(_ context.Context, personID int64, operatorID int64, clientIP net.IP) error {
			assert.Equal(t, int64(15), personID)
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, "203.0.113.10", clientIP.String())
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/operator/persons/15", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "15")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.SoftDeletePerson(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	body := decodeBody(t, rr)
	assert.Equal(t, "Person deleted successfully", body["message"])
}

func TestProvisioningResource_SoftDeletePerson_NotFound(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		softDeletePersonFn: func(_ context.Context, _ int64, _ int64, _ net.IP) error {
			return &platformSvc.PersonNotFoundError{PersonID: 15}
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/operator/persons/15", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "15")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.SoftDeletePerson(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestProvisioningResource_SoftDeletePerson_ActiveSupervisions(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		softDeletePersonFn: func(_ context.Context, _ int64, _ int64, _ net.IP) error {
			return &platformSvc.PersonHasActiveSupervisionsError{PersonID: 15, Count: 2}
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/operator/persons/15", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "15")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.SoftDeletePerson(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestProvisioningResource_SoftDeletePerson_InvalidID(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodDelete, "/operator/persons/abc", nil)
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.SoftDeletePerson(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningErrorRenderer_PersonNotFound(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&platformSvc.PersonNotFoundError{PersonID: 42})
	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
	assert.Equal(t, "Person not found", resp.ErrorText)
}

func TestProvisioningErrorRenderer_PersonActiveSupervisors(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&platformSvc.PersonHasActiveSupervisionsError{PersonID: 42, Count: 3})
	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Equal(t, "Person has active supervisions and cannot be deleted", resp.ErrorText)
}

func ptrInt64(v int64) *int64    { return &v }
func ptrString(v string) *string { return &v }

var _ platformSvc.OperatorProvisioningService = (*mockProvisioningService)(nil)

// --- SoftDeleteSchool handler tests ---

func TestProvisioningResource_SoftDeleteSchool(t *testing.T) {
	t.Parallel()

	var deletedID int64
	resource := NewProvisioningResource(&mockProvisioningService{
		softDeleteSchoolFn: func(schoolID int64) error {
			deletedID = schoolID
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/operator/schools/55", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "55")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.SoftDeleteSchool(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, int64(55), deletedID)
}

func TestProvisioningResource_SoftDeleteSchool_InvalidID(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})

	req := httptest.NewRequest(http.MethodDelete, "/operator/schools/abc", nil)
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.SoftDeleteSchool(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_SoftDeleteSchool_NotFound(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		softDeleteSchoolFn: func(_ int64) error {
			return &platformSvc.SchoolNotFoundError{SchoolID: 99}
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/operator/schools/99", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "99")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.SoftDeleteSchool(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestProvisioningResource_SoftDeleteSchool_AlreadyDeleted(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		softDeleteSchoolFn: func(_ int64) error {
			return &platformSvc.SchoolAlreadyDeletedError{SchoolID: 55}
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/operator/schools/55", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "55")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.SoftDeleteSchool(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

// --- RestoreSchool handler tests ---

func TestProvisioningResource_RestoreSchool(t *testing.T) {
	t.Parallel()

	var restoredID int64
	resource := NewProvisioningResource(&mockProvisioningService{
		restoreSchoolFn: func(schoolID int64) error {
			restoredID = schoolID
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/operator/schools/55/restore", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "55")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.RestoreSchool(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(55), restoredID)
}

func TestProvisioningResource_RestoreSchool_InvalidID(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})

	req := httptest.NewRequest(http.MethodPost, "/operator/schools/abc/restore", nil)
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.RestoreSchool(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_RestoreSchool_NotFound(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		restoreSchoolFn: func(_ int64) error {
			return &platformSvc.SchoolNotFoundError{SchoolID: 99}
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/operator/schools/99/restore", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "99")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.RestoreSchool(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestProvisioningResource_RestoreSchool_NotDeleted(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		restoreSchoolFn: func(_ int64) error {
			return &platformSvc.SchoolNotDeletedError{SchoolID: 55}
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/operator/schools/55/restore", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "55")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.RestoreSchool(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

// --- Error renderer tests for soft-delete/restore ---

func TestProvisioningErrorRenderer_SchoolAlreadyDeleted(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&platformSvc.SchoolAlreadyDeletedError{SchoolID: 55})
	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "already deleted")
}

func TestProvisioningErrorRenderer_SchoolNotDeleted(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&platformSvc.SchoolNotDeletedError{SchoolID: 55})
	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "not deleted")
}

// --- SoftDeleteOrganization handler tests ---

func TestProvisioningResource_SoftDeleteOrganization(t *testing.T) {
	t.Parallel()

	var deletedID int64
	resource := NewProvisioningResource(&mockProvisioningService{
		softDeleteOrgFn: func(orgID int64) error {
			deletedID = orgID
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/operator/organizations/10", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "10")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.SoftDeleteOrganization(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, int64(10), deletedID)
}

func TestProvisioningResource_SoftDeleteOrganization_HasSchools(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		softDeleteOrgFn: func(_ int64) error {
			return &platformSvc.OrganizationHasSchoolsError{OrganizationID: 10, SchoolCount: 3}
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/operator/organizations/10", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "10")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.SoftDeleteOrganization(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestProvisioningResource_SoftDeleteOrganization_AlreadyDeleted(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		softDeleteOrgFn: func(_ int64) error {
			return &platformSvc.OrganizationAlreadyDeletedError{OrganizationID: 10}
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/operator/organizations/10", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "10")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.SoftDeleteOrganization(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

// --- RestoreOrganization handler tests ---

func TestProvisioningResource_RestoreOrganization(t *testing.T) {
	t.Parallel()

	var restoredID int64
	resource := NewProvisioningResource(&mockProvisioningService{
		restoreOrgFn: func(orgID int64) error {
			restoredID = orgID
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/operator/organizations/10/restore", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "10")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.RestoreOrganization(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(10), restoredID)
}

func TestProvisioningResource_RestoreOrganization_NotDeleted(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		restoreOrgFn: func(_ int64) error {
			return &platformSvc.OrganizationNotDeletedError{OrganizationID: 10}
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/operator/organizations/10/restore", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "10")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.RestoreOrganization(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestProvisioningResource_SoftDeleteOrganization_InvalidID(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodDelete, "/operator/organizations/abc", nil)
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.SoftDeleteOrganization(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_RestoreOrganization_InvalidID(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodPost, "/operator/organizations/abc/restore", nil)
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.RestoreOrganization(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- Error renderer tests for organization soft-delete/restore ---

func TestProvisioningErrorRenderer_OrganizationAlreadyDeleted(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&platformSvc.OrganizationAlreadyDeletedError{OrganizationID: 10})
	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "already deleted")
}

func TestProvisioningErrorRenderer_OrganizationNotDeleted(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&platformSvc.OrganizationNotDeletedError{OrganizationID: 10})
	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "not deleted")
}

func TestProvisioningErrorRenderer_OrganizationHasSchools(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&platformSvc.OrganizationHasSchoolsError{OrganizationID: 10, SchoolCount: 3})
	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "3 existing school(s)")
}

func TestProvisioningErrorRenderer_OrganizationDeleted(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&platformSvc.OrganizationDeletedError{OrganizationID: 10})
	resp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "deleted")
}

func TestUpdateCaregiverCapabilityRequest_Bind_TrimWhitespace(t *testing.T) {
	t.Parallel()

	req := &updateCaregiverCapabilityRequest{
		FirstName: "  Ada ",
		LastName:  " Lovelace  ",
		Position:  "  Springer ",
	}

	require.NoError(t, req.Bind(nil))
	assert.Equal(t, "Ada", req.FirstName)
	assert.Equal(t, "Lovelace", req.LastName)
	assert.Equal(t, "Springer", req.Position)
}

func TestCaregiverCapabilityProvisioningErrorRenderer(t *testing.T) {
	t.Parallel()

	t.Run("renders blocker details", func(t *testing.T) {
		renderer := caregiverCapabilityProvisioningErrorRenderer(&usersSvc.CaregiverCapabilityBlockedError{
			Reasons: []userModels.CaregiverCapabilityBlockerCode{
				userModels.CaregiverCapabilityBlockerActiveGroupSupervisions,
				userModels.CaregiverCapabilityBlockerGroupAssignments,
			},
		})

		blocked, ok := renderer.(*common.CaregiverCapabilityBlockedResponse)
		require.True(t, ok)
		assert.Equal(t, http.StatusConflict, blocked.HTTPStatusCode)
		assert.Equal(
			t,
			[]userModels.CaregiverCapabilityBlockerCode{
				userModels.CaregiverCapabilityBlockerActiveGroupSupervisions,
				userModels.CaregiverCapabilityBlockerGroupAssignments,
			},
			blocked.Blockers,
		)
	})

	t.Run("maps missing account to not found", func(t *testing.T) {
		renderer := caregiverCapabilityProvisioningErrorRenderer(&usersSvc.AccountNotAssignedToTenantError{
			AccountID: 77,
			TenantID:  4,
		})

		resp, ok := renderer.(*ErrResponse)
		require.True(t, ok)
		assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
		assert.Equal(t, "Account not found", resp.ErrorText)
	})

	t.Run("maps invalid data to bad request", func(t *testing.T) {
		renderer := caregiverCapabilityProvisioningErrorRenderer(&platformSvc.InvalidDataError{
			Err: errors.New("invalid caregiver input"),
		})

		resp, ok := renderer.(*ErrResponse)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
		assert.Equal(t, "invalid caregiver input", resp.ErrorText)
	})

	t.Run("maps wrapped validation errors to bad request", func(t *testing.T) {
		renderer := caregiverCapabilityProvisioningErrorRenderer(&usersSvc.UsersError{
			Op:  "enable caregiver capability",
			Err: &usersSvc.ValidationError{Err: errors.New("first_name is required")},
		})

		resp, ok := renderer.(*ErrResponse)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
		assert.Equal(t, "first_name is required", resp.ErrorText)
	})

	t.Run("preserves internal users errors as internal server errors", func(t *testing.T) {
		renderer := caregiverCapabilityProvisioningErrorRenderer(&usersSvc.UsersError{
			Op:  "enable caregiver capability",
			Err: errors.New("audit write failed"),
		})

		resp, ok := renderer.(*ErrResponse)
		require.True(t, ok)
		assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
		assert.Equal(t, "An error occurred", resp.ErrorText)
	})
}

func TestCaregiverCapabilityBlockedResponse_Render(t *testing.T) {
	t.Parallel()

	resp := common.NewCaregiverCapabilityBlockedResponse(
		http.StatusConflict,
		"blocked",
		nil,
	)
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	require.NoError(t, resp.Render(httptest.NewRecorder(), req))
	assert.Equal(t, http.StatusConflict, req.Context().Value(render.StatusCtxKey))
}

func TestProvisioningResource_CaregiverCapabilityServiceNotConfigured(t *testing.T) {
	t.Parallel()

	t.Run("GetSchoolAccountCaregiverCapability returns internal error", func(t *testing.T) {
		resource := &ProvisioningResource{}
		req := httptest.NewRequest(http.MethodGet, "/operator/schools/12/accounts/34/caregiver-capability", nil)
		rr := httptest.NewRecorder()

		resource.GetSchoolAccountCaregiverCapability(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("EnableSchoolAccountCaregiverCapability returns internal error", func(t *testing.T) {
		resource := &ProvisioningResource{}
		req := httptest.NewRequest(http.MethodPut, "/operator/schools/12/accounts/34/caregiver-capability", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		resource.EnableSchoolAccountCaregiverCapability(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("DisableSchoolAccountCaregiverCapability returns internal error", func(t *testing.T) {
		resource := &ProvisioningResource{}
		req := httptest.NewRequest(http.MethodDelete, "/operator/schools/12/accounts/34/caregiver-capability", nil)
		rr := httptest.NewRecorder()

		resource.DisableSchoolAccountCaregiverCapability(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestProvisioningResource_DeleteDevice(t *testing.T) {
	t.Parallel()

	t.Run("deletes device successfully", func(t *testing.T) {
		resource := NewProvisioningResource(&mockProvisioningService{
			deleteDeviceFn: func(_ context.Context, id int64, operatorID int64, clientIP net.IP) error {
				assert.Equal(t, int64(55), id)
				assert.Equal(t, int64(42), operatorID)
				assert.Equal(t, "203.0.113.50", clientIP.String())
				return nil
			},
		})

		req := httptest.NewRequest(http.MethodDelete, "/operator/devices/55", nil)
		req.RemoteAddr = "203.0.113.50:4242"
		req = withOperatorClaims(req, 42)
		routeCtx := chi.NewRouteContext()
		routeCtx.URLParams.Add("id", "55")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
		rr := httptest.NewRecorder()

		resource.DeleteDevice(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("rejects invalid device id", func(t *testing.T) {
		resource := NewProvisioningResource(&mockProvisioningService{})
		req := httptest.NewRequest(http.MethodDelete, "/operator/devices/nope", nil)
		req = withOperatorClaims(req, 42)
		routeCtx := chi.NewRouteContext()
		routeCtx.URLParams.Add("id", "nope")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
		rr := httptest.NewRecorder()

		resource.DeleteDevice(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("renders service errors", func(t *testing.T) {
		resource := NewProvisioningResource(&mockProvisioningService{
			deleteDeviceFn: func(context.Context, int64, int64, net.IP) error {
				return &platformSvc.OperatorDeviceNotFoundError{DeviceID: 55}
			},
		})

		req := httptest.NewRequest(http.MethodDelete, "/operator/devices/55", nil)
		req = withOperatorClaims(req, 42)
		routeCtx := chi.NewRouteContext()
		routeCtx.URLParams.Add("id", "55")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
		rr := httptest.NewRecorder()

		resource.DeleteDevice(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

func TestProvisioningResource_GetSchoolAccountCaregiverCapability(t *testing.T) {
	t.Parallel()

	db, mock := newMockAdminDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	resource := &ProvisioningResource{
		CaregiverCapabilityService: &mockCaregiverCapabilityService{
			getFn: func(ctx context.Context, accountID int64) (*userModels.CaregiverCapabilityState, error) {
				assert.Equal(t, int64(12), tenant.FromContext(ctx))
				assert.Equal(t, int64(34), accountID)
				return &userModels.CaregiverCapabilityState{AccountID: accountID, HasUserRole: true}, nil
			},
		},
		db: db,
	}

	req := httptest.NewRequest(http.MethodGet, "/operator/schools/12/accounts/34/caregiver-capability", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "12")
	routeCtx.URLParams.Add("accountId", "34")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.GetSchoolAccountCaregiverCapability(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProvisioningResource_EnableSchoolAccountCaregiverCapability(t *testing.T) {
	t.Parallel()

	db, mock := newMockAdminDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	resource := &ProvisioningResource{
		CaregiverCapabilityService: &mockCaregiverCapabilityService{
			enableFn: func(ctx context.Context, accountID int64, input userModels.EnableCaregiverCapabilityInput) (*userModels.CaregiverCapabilityState, error) {
				assert.Equal(t, int64(12), tenant.FromContext(ctx))
				assert.Equal(t, int64(34), accountID)
				tx, ok := modelBase.TxFromContext(ctx)
				require.True(t, ok)
				require.NotNil(t, tx)
				assert.Equal(t, userModels.EnableCaregiverCapabilityInput{
					FirstName: "Ada",
					LastName:  "Lovelace",
					Position:  "Springer",
				}, input)
				return &userModels.CaregiverCapabilityState{AccountID: accountID, HasUserRole: true}, nil
			},
		},
		db: db,
	}

	req := httptest.NewRequest(http.MethodPut, "/operator/schools/12/accounts/34/caregiver-capability", bytes.NewBufferString(`{"first_name":" Ada ","last_name":" Lovelace ","position":" Springer "}`))
	req.Header.Set("Content-Type", "application/json")
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "12")
	routeCtx.URLParams.Add("accountId", "34")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.EnableSchoolAccountCaregiverCapability(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProvisioningResource_EnableSchoolAccountCaregiverCapability_EmptyBody(t *testing.T) {
	t.Parallel()

	resource := &ProvisioningResource{
		CaregiverCapabilityService: &mockCaregiverCapabilityService{},
	}

	req := httptest.NewRequest(http.MethodPut, "/operator/schools/12/accounts/34/caregiver-capability", http.NoBody)
	req.Header.Set("Content-Type", "application/json")
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "12")
	routeCtx.URLParams.Add("accountId", "34")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.EnableSchoolAccountCaregiverCapability(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var responseBody map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &responseBody))
	assert.Equal(t, "error", responseBody["status"])
	assert.Equal(t, "EOF", responseBody["message"])
}

func TestProvisioningResource_DisableSchoolAccountCaregiverCapability(t *testing.T) {
	t.Parallel()

	db, mock := newMockAdminDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	resource := &ProvisioningResource{
		CaregiverCapabilityService: &mockCaregiverCapabilityService{
			disableFn: func(ctx context.Context, accountID int64) (*userModels.CaregiverCapabilityState, error) {
				assert.Equal(t, int64(12), tenant.FromContext(ctx))
				assert.Equal(t, int64(34), accountID)
				tx, ok := modelBase.TxFromContext(ctx)
				require.True(t, ok)
				require.NotNil(t, tx)
				return nil, &usersSvc.CaregiverCapabilityBlockedError{
					Reasons: []userModels.CaregiverCapabilityBlockerCode{
						userModels.CaregiverCapabilityBlockerActiveGroupSupervisions,
					},
				}
			},
		},
		db: db,
	}

	req := httptest.NewRequest(http.MethodDelete, "/operator/schools/12/accounts/34/caregiver-capability", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "12")
	routeCtx.URLParams.Add("accountId", "34")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.DisableSchoolAccountCaregiverCapability(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- Provisioning summaries (drill-in refactor) ---

func TestProvisioningResource_GetProvisioningStats(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		getProvisioningStatsFn: func(context.Context) (*platformSvc.ProvisioningStats, error) {
			return &platformSvc.ProvisioningStats{
				TraegerCount: 3,
				SchulenCount: 7,
				KontenCount:  42,
				GeraeteCount: 11,
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/stats", nil)
	rr := httptest.NewRecorder()

	resource.GetProvisioningStats(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	body := decodeBody(t, rr)
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(3), data["traeger_count"])
	assert.Equal(t, float64(7), data["schulen_count"])
	assert.Equal(t, float64(42), data["konten_count"])
	assert.Equal(t, float64(11), data["geraete_count"])
}

func TestProvisioningResource_GetProvisioningStats_Error(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		getProvisioningStatsFn: func(context.Context) (*platformSvc.ProvisioningStats, error) {
			return nil, errors.New("db fail")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/stats", nil)
	rr := httptest.NewRecorder()

	resource.GetProvisioningStats(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestProvisioningResource_ListOrganizationSummaries(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listOrgSummariesFn: func(context.Context) ([]*platformSvc.OrganizationSummary, error) {
			return []*platformSvc.OrganizationSummary{
				{ID: 1, Name: "Org One", Slug: "org-one", Active: true, SchulenCount: 2, KontenCount: 5, GeraeteCount: 3, PersonenCount: 18},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/organizations/summaries", nil)
	rr := httptest.NewRecorder()

	resource.ListOrganizationSummaries(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	body := decodeBody(t, rr)
	data := body["data"].([]any)
	require.Len(t, data, 1)
	first := data[0].(map[string]any)
	assert.Equal(t, "org-one", first["slug"])
	assert.Equal(t, float64(2), first["schulen_count"])
	assert.Equal(t, float64(5), first["konten_count"])
	assert.Equal(t, float64(3), first["geraete_count"])
	assert.Equal(t, float64(18), first["personen_count"])
}

func TestProvisioningResource_ListOrganizationSummaries_Error(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listOrgSummariesFn: func(context.Context) ([]*platformSvc.OrganizationSummary, error) {
			return nil, errors.New("db fail")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/organizations/summaries", nil)
	rr := httptest.NewRecorder()

	resource.ListOrganizationSummaries(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestProvisioningResource_ListSchoolSummaries(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listSchoolSummariesFn: func(context.Context) ([]*platformSvc.SchoolSummary, error) {
			return []*platformSvc.SchoolSummary{
				{ID: 10, OrganizationID: 1, OrganizationName: "Org One", Name: "School A", Slug: "school-a", Subdomain: "a", Active: true, KontenCount: 4, GeraeteCount: 2, PersonenCount: 30},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/schools/summaries", nil)
	rr := httptest.NewRecorder()

	resource.ListSchoolSummaries(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	body := decodeBody(t, rr)
	data := body["data"].([]any)
	require.Len(t, data, 1)
	first := data[0].(map[string]any)
	assert.Equal(t, "school-a", first["slug"])
	assert.Equal(t, "Org One", first["organization_name"])
	assert.Equal(t, float64(4), first["konten_count"])
	assert.Equal(t, float64(30), first["personen_count"])
}

func TestProvisioningResource_ListSchoolSummaries_Error(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listSchoolSummariesFn: func(context.Context) ([]*platformSvc.SchoolSummary, error) {
			return nil, errors.New("db fail")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/schools/summaries", nil)
	rr := httptest.NewRecorder()

	resource.ListSchoolSummaries(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestProvisioningResource_ListOrganizationSchoolSummaries(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listOrgSchoolSummariesFn: func(_ context.Context, orgID int64) ([]*platformSvc.SchoolSummary, error) {
			assert.Equal(t, int64(7), orgID)
			return []*platformSvc.SchoolSummary{
				{ID: 99, OrganizationID: 7, Name: "Schule X", Slug: "schule-x"},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/organizations/7/schools", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "7")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListOrganizationSchoolSummaries(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	body := decodeBody(t, rr)
	data := body["data"].([]any)
	require.Len(t, data, 1)
	first := data[0].(map[string]any)
	assert.Equal(t, "schule-x", first["slug"])
}

func TestProvisioningResource_ListOrganizationSchoolSummaries_InvalidID(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodGet, "/operator/organizations/nope/schools", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "nope")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListOrganizationSchoolSummaries(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_ListOrganizationSchoolSummaries_OrganizationNotFound(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listOrgSchoolSummariesFn: func(_ context.Context, orgID int64) ([]*platformSvc.SchoolSummary, error) {
			return nil, &platformSvc.OrganizationNotFoundError{OrganizationID: orgID}
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/operator/organizations/9999/schools", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "9999")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListOrganizationSchoolSummaries(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestProvisioningResource_ListOrganizationPersons(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listOrgPersonsFn: func(_ context.Context, orgID int64) ([]platformSvc.OperatorPersonInfo, error) {
			assert.Equal(t, int64(7), orgID)
			return []platformSvc.OperatorPersonInfo{
				{ID: 1, FirstName: "Ada", LastName: "Lovelace", SchoolID: 10, SchoolName: "School A", OrganizationID: 7, OrganizationName: "Org One"},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/operator/organizations/7/persons", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "7")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListOrganizationPersons(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	body := decodeBody(t, rr)
	data := body["data"].([]any)
	require.Len(t, data, 1)
	first := data[0].(map[string]any)
	assert.Equal(t, "Ada", first["first_name"])
	assert.Equal(t, "Lovelace", first["last_name"])
	assert.Equal(t, float64(7), first["organization_id"])
}

func TestProvisioningResource_ListOrganizationPersons_InvalidID(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodGet, "/operator/organizations/abc/persons", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListOrganizationPersons(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_ListOrganizationPersons_OrganizationNotFound(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(&mockProvisioningService{
		listOrgPersonsFn: func(_ context.Context, orgID int64) ([]platformSvc.OperatorPersonInfo, error) {
			return nil, &platformSvc.OrganizationNotFoundError{OrganizationID: orgID}
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/operator/organizations/9999/persons", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "9999")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListOrganizationPersons(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
