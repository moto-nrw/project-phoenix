package operator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	jwtPkg "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ organizationtenancy.Provisioning = (*mockProvisioningService)(nil)

type mockProvisioningService struct {
	createOrganizationFn      func(context.Context, *organizationtenancy.CreateOrganization, int64, net.IP) (*organizationtenancy.Organization, error)
	listOrganizationsFn       func(context.Context) ([]organizationtenancy.Organization, error)
	updateOrganizationFn      func(context.Context, int64, organizationtenancy.OrganizationChanges, int64, net.IP) (*organizationtenancy.Organization, error)
	createSchoolFn            func(context.Context, *organizationtenancy.CreateSchool, int64, net.IP) (*organizationtenancy.School, error)
	listSchoolsFn             func(context.Context) ([]*organizationtenancy.School, error)
	updateSchoolFn            func(context.Context, int64, organizationtenancy.SchoolChanges, int64, net.IP) (*organizationtenancy.School, error)
	inviteSchoolAdminFn       func(context.Context, int64, int64, net.IP, organizationtenancy.SchoolAdminInvitationInput) (*organizationtenancy.SchoolAdminInvitation, error)
	createSchoolAccountFn     func(context.Context, int64, int64, net.IP, organizationtenancy.SchoolAccountInput) (*organizationtenancy.CreatedAccount, error)
	listSchoolAccountsFn      func(context.Context, int64) ([]organizationtenancy.SchoolAccount, error)
	listOrgAccountsFn         func(context.Context, int64) ([]organizationtenancy.OrganizationAccount, error)
	listAllAccountsFn         func(context.Context) ([]organizationtenancy.OrganizationAccount, error)
	listSystemRolesFn         func(context.Context) ([]organizationtenancy.SystemRole, error)
	listAllDevicesFn          func(context.Context) ([]organizationtenancy.OperatorDevice, error)
	listSchoolDevicesFn       func(context.Context, int64) ([]organizationtenancy.OperatorDevice, error)
	listOrganizationDevicesFn func(context.Context, int64) ([]organizationtenancy.OperatorDevice, error)
	createDeviceFn            func(context.Context, int64, string, string, *string, *string, int64, net.IP) (*organizationtenancy.OperatorDevice, error)
	setDeviceAPIKeyFn         func(context.Context, int64, *string, int64, net.IP) (*organizationtenancy.OperatorDevice, error)
	getDeviceTransferStatusFn func(context.Context, int64) (*organizationtenancy.DeviceTransferStatus, error)
	transferDeviceFn          func(context.Context, int64, int64, int64, net.IP) (*organizationtenancy.OperatorDevice, error)
	softDeleteSchoolFn        func(int64) error
	restoreSchoolFn           func(int64) error
	softDeleteOrgFn           func(int64) error
	restoreOrgFn              func(int64) error
	deleteDeviceFn            func(context.Context, int64, int64, net.IP) error
	listSchoolPersonsFn       func(context.Context, int64) ([]organizationtenancy.OperatorPerson, error)
	softDeletePersonFn        func(context.Context, int64, int64, net.IP) error
	getProvisioningStatsFn    func(context.Context) (*organizationtenancy.ProvisioningStats, error)
	getSchoolPWAUsageFn       func(context.Context, int64) (*organizationtenancy.SchoolPWAUsage, error)
	listPWAUsageFn            func(context.Context, time.Duration) ([]organizationtenancy.SchoolPWAUsageRow, error)
	listOrgSummariesFn        func(context.Context) ([]*organizationtenancy.OrganizationSummary, error)
	listSchoolSummariesFn     func(context.Context) ([]*organizationtenancy.SchoolSummary, error)
	listOrgSchoolSummariesFn  func(context.Context, int64) ([]*organizationtenancy.SchoolSummary, error)
	listOrgPersonsFn          func(context.Context, int64) ([]organizationtenancy.OperatorPerson, error)
}

func (m *mockProvisioningService) CreateOrganization(ctx context.Context, org *organizationtenancy.CreateOrganization, operatorID int64, clientIP net.IP) (*organizationtenancy.Organization, error) {
	return m.createOrganizationFn(ctx, org, operatorID, clientIP)
}
func (m *mockProvisioningService) ListOrganizations(ctx context.Context) ([]organizationtenancy.Organization, error) {
	return m.listOrganizationsFn(ctx)
}
func (m *mockProvisioningService) UpdateOrganization(ctx context.Context, id int64, req organizationtenancy.OrganizationChanges, operatorID int64, clientIP net.IP) (*organizationtenancy.Organization, error) {
	if m.updateOrganizationFn != nil {
		return m.updateOrganizationFn(ctx, id, req, operatorID, clientIP)
	}
	return nil, nil
}
func (m *mockProvisioningService) CreateSchool(ctx context.Context, school *organizationtenancy.CreateSchool, operatorID int64, clientIP net.IP) (*organizationtenancy.School, error) {
	return m.createSchoolFn(ctx, school, operatorID, clientIP)
}
func (m *mockProvisioningService) ListSchools(ctx context.Context) ([]*organizationtenancy.School, error) {
	return m.listSchoolsFn(ctx)
}
func (m *mockProvisioningService) UpdateSchool(ctx context.Context, id int64, req organizationtenancy.SchoolChanges, operatorID int64, clientIP net.IP) (*organizationtenancy.School, error) {
	if m.updateSchoolFn != nil {
		return m.updateSchoolFn(ctx, id, req, operatorID, clientIP)
	}
	return nil, nil
}
func (m *mockProvisioningService) InviteSchoolAdmin(ctx context.Context, schoolID, operatorID int64, clientIP net.IP, req organizationtenancy.SchoolAdminInvitationInput) (*organizationtenancy.SchoolAdminInvitation, error) {
	return m.inviteSchoolAdminFn(ctx, schoolID, operatorID, clientIP, req)
}
func (m *mockProvisioningService) CreateSchoolAccount(ctx context.Context, schoolID, operatorID int64, clientIP net.IP, req organizationtenancy.SchoolAccountInput) (*organizationtenancy.CreatedAccount, error) {
	if m.createSchoolAccountFn != nil {
		return m.createSchoolAccountFn(ctx, schoolID, operatorID, clientIP, req)
	}
	return nil, errors.New("not implemented")
}
func (m *mockProvisioningService) ListSystemRoles(ctx context.Context) ([]organizationtenancy.SystemRole, error) {
	if m.listSystemRolesFn != nil {
		return m.listSystemRolesFn(ctx)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListSchoolAccounts(ctx context.Context, schoolID int64) ([]organizationtenancy.SchoolAccount, error) {
	if m.listSchoolAccountsFn != nil {
		return m.listSchoolAccountsFn(ctx, schoolID)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListOrganizationAccounts(ctx context.Context, orgID int64) ([]organizationtenancy.OrganizationAccount, error) {
	if m.listOrgAccountsFn != nil {
		return m.listOrgAccountsFn(ctx, orgID)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListAllAccounts(ctx context.Context) ([]organizationtenancy.OrganizationAccount, error) {
	if m.listAllAccountsFn != nil {
		return m.listAllAccountsFn(ctx)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListAllDevices(ctx context.Context) ([]organizationtenancy.OperatorDevice, error) {
	if m.listAllDevicesFn != nil {
		return m.listAllDevicesFn(ctx)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListSchoolDevices(ctx context.Context, schoolID int64) ([]organizationtenancy.OperatorDevice, error) {
	if m.listSchoolDevicesFn != nil {
		return m.listSchoolDevicesFn(ctx, schoolID)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListOrganizationDevices(ctx context.Context, orgID int64) ([]organizationtenancy.OperatorDevice, error) {
	if m.listOrganizationDevicesFn != nil {
		return m.listOrganizationDevicesFn(ctx, orgID)
	}
	return nil, nil
}
func (m *mockProvisioningService) CreateDevice(ctx context.Context, schoolID int64, deviceID, deviceType string, name, apiKey *string, operatorID int64, clientIP net.IP) (*organizationtenancy.OperatorDevice, error) {
	if m.createDeviceFn != nil {
		return m.createDeviceFn(ctx, schoolID, deviceID, deviceType, name, apiKey, operatorID, clientIP)
	}
	return nil, nil
}
func (m *mockProvisioningService) SetDeviceAPIKey(ctx context.Context, deviceID int64, apiKey *string, operatorID int64, clientIP net.IP) (*organizationtenancy.OperatorDevice, error) {
	if m.setDeviceAPIKeyFn != nil {
		return m.setDeviceAPIKeyFn(ctx, deviceID, apiKey, operatorID, clientIP)
	}
	return nil, nil
}
func (m *mockProvisioningService) GetDeviceTransferStatus(ctx context.Context, deviceID int64) (*organizationtenancy.DeviceTransferStatus, error) {
	if m.getDeviceTransferStatusFn != nil {
		return m.getDeviceTransferStatusFn(ctx, deviceID)
	}
	return nil, nil
}
func (m *mockProvisioningService) TransferDevice(ctx context.Context, deviceID, targetSchoolID, operatorID int64, clientIP net.IP) (*organizationtenancy.OperatorDevice, error) {
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
func (m *mockProvisioningService) ListSchoolPersons(ctx context.Context, schoolID int64) ([]organizationtenancy.OperatorPerson, error) {
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
func (m *mockProvisioningService) GetProvisioningStats(ctx context.Context) (*organizationtenancy.ProvisioningStats, error) {
	if m.getProvisioningStatsFn != nil {
		return m.getProvisioningStatsFn(ctx)
	}
	return nil, nil
}
func (m *mockProvisioningService) GetSchoolPWAUsage(ctx context.Context, schoolID int64) (*organizationtenancy.SchoolPWAUsage, error) {
	if m.getSchoolPWAUsageFn != nil {
		return m.getSchoolPWAUsageFn(ctx, schoolID)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListPWAUsage(ctx context.Context, window time.Duration) ([]organizationtenancy.SchoolPWAUsageRow, error) {
	if m.listPWAUsageFn != nil {
		return m.listPWAUsageFn(ctx, window)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListOrganizationSummaries(ctx context.Context) ([]*organizationtenancy.OrganizationSummary, error) {
	if m.listOrgSummariesFn != nil {
		return m.listOrgSummariesFn(ctx)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListSchoolSummaries(ctx context.Context) ([]*organizationtenancy.SchoolSummary, error) {
	if m.listSchoolSummariesFn != nil {
		return m.listSchoolSummariesFn(ctx)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListOrganizationSchoolSummaries(ctx context.Context, organizationID int64) ([]*organizationtenancy.SchoolSummary, error) {
	if m.listOrgSchoolSummariesFn != nil {
		return m.listOrgSchoolSummariesFn(ctx, organizationID)
	}
	return nil, nil
}
func (m *mockProvisioningService) ListOrganizationPersons(ctx context.Context, organizationID int64) ([]organizationtenancy.OperatorPerson, error) {
	if m.listOrgPersonsFn != nil {
		return m.listOrgPersonsFn(ctx, organizationID)
	}
	return nil, nil
}

// seedInvitationTokenHeader is the header a seeding client sets to read
// invitation tokens outside production.
const seedInvitationTokenHeader = "X-Phoenix-Seed-Token"

var _ SchoolAccountCaregivers = (*fakeSchoolAccountCaregivers)(nil)

// fakeSchoolAccountCaregivers stands in for the root's binding of People
// Directory's caregiver capability, which owns the admin transaction and the
// tenant scoping (covered in services/users/caregiver_capability_views_test.go).
type fakeSchoolAccountCaregivers struct {
	getFn     func(ctx context.Context, schoolID, accountID int64) (json.RawMessage, error)
	enableFn  func(ctx context.Context, schoolID, accountID int64, firstName, lastName, position string) (json.RawMessage, error)
	disableFn func(ctx context.Context, schoolID, accountID int64) (json.RawMessage, error)
}

func (f *fakeSchoolAccountCaregivers) GetSchoolAccountCaregiverCapability(ctx context.Context, schoolID, accountID int64) (json.RawMessage, error) {
	if f.getFn != nil {
		return f.getFn(ctx, schoolID, accountID)
	}
	return nil, nil
}

func (f *fakeSchoolAccountCaregivers) EnableSchoolAccountCaregiverCapability(ctx context.Context, schoolID, accountID int64, firstName, lastName, position string) (json.RawMessage, error) {
	if f.enableFn != nil {
		return f.enableFn(ctx, schoolID, accountID, firstName, lastName, position)
	}
	return nil, nil
}

func (f *fakeSchoolAccountCaregivers) DisableSchoolAccountCaregiverCapability(ctx context.Context, schoolID, accountID int64) (json.RawMessage, error) {
	if f.disableFn != nil {
		return f.disableFn(ctx, schoolID, accountID)
	}
	return nil, nil
}

// The fake caregiver failures carry the behaviours the routes classify on.
type fakeCaregiverBlockedError struct{ blockers []string }

func (e fakeCaregiverBlockedError) Error() string {
	return "caregiver capability cannot be removed while active bindings exist"
}
func (e fakeCaregiverBlockedError) CaregiverCapabilityBlockers() []string { return e.blockers }

type fakeCaregiverAccountMissingError struct{ error }

func (e fakeCaregiverAccountMissingError) CaregiverAccountMissing() bool { return true }

// fakeCaregiverRequestInvalidError reports invalid, an error whose cause is
// the rejection text, as People Directory's validation error does.
type fakeCaregiverRequestInvalidError struct {
	error
	invalid error
}

func (e fakeCaregiverRequestInvalidError) CaregiverRequestInvalid() error { return e.invalid }

type fakeCaregiverFailureError struct {
	error
	cause error
}

func (e fakeCaregiverFailureError) CaregiverFailure() error { return e.cause }

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

// createdSchool mirrors a persisted school for a create input, as the
// provisioning capability returns it.
func createdSchool(id int64, input *organizationtenancy.CreateSchool) *organizationtenancy.School {
	return &organizationtenancy.School{
		ID:             id,
		OrganizationID: input.OrganizationID,
		Name:           input.Name,
		Slug:           input.Slug,
		Subdomain:      input.Subdomain,
		Active:         input.Active,
		Hidden:         input.Hidden,
		Address:        input.Address,
		City:           input.City,
		Zip:            input.Zip,
		Phone:          input.Phone,
		Email:          input.Email,
	}
}

func TestProvisioningResource_CreateOrganization(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		createOrganizationFn: func(_ context.Context, org *organizationtenancy.CreateOrganization, operatorID int64, clientIP net.IP) (*organizationtenancy.Organization, error) {
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, "Stadt Koeln", org.Name)
			assert.Equal(t, "stadt-koeln", org.Slug)
			assert.Equal(t, "203.0.113.10", clientIP.String())
			return &organizationtenancy.Organization{ID: 55, Name: org.Name, Slug: org.Slug, Active: org.Active}, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
	req := httptest.NewRequest(http.MethodPost, "/operator/organizations", bytes.NewBufferString(`{"name":`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.CreateOrganization(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_ListOrganizations(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listOrganizationsFn: func(context.Context) ([]organizationtenancy.Organization, error) {
			return []organizationtenancy.Organization{{Name: "Org", Slug: "org"}}, nil
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/operator/organizations", nil)
	rr := httptest.NewRecorder()

	resource.ListOrganizations(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestProvisioningResource_ListOrganizations_Error(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listOrganizationsFn: func(context.Context) ([]organizationtenancy.Organization, error) {
			return nil, errors.New("db fail")
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/operator/organizations", nil)
	rr := httptest.NewRecorder()

	resource.ListOrganizations(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestProvisioningResource_CreateSchool(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		createSchoolFn: func(_ context.Context, school *organizationtenancy.CreateSchool, operatorID int64, clientIP net.IP) (*organizationtenancy.School, error) {
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, int64(7), school.OrganizationID)
			assert.Equal(t, "school@example.com", school.Email)
			assert.Equal(t, "198.51.100.20", clientIP.String())
			return createdSchool(88, school), nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listSchoolsFn: func(context.Context) ([]*organizationtenancy.School, error) {
			return []*organizationtenancy.School{{Name: "School", Slug: "school"}}, nil
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/operator/schools", nil)
	rr := httptest.NewRecorder()

	resource.ListSchools(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestProvisioningResource_CreateSchool_InvalidRequest(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
	req := httptest.NewRequest(http.MethodPost, "/operator/schools", bytes.NewBufferString(`{"organization_id":`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.CreateSchool(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_ListSchools_Error(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listSchoolsFn: func(context.Context) ([]*organizationtenancy.School, error) {
			return nil, errors.New("db fail")
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/operator/schools", nil)
	rr := httptest.NewRecorder()

	resource.ListSchools(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestProvisioningResource_InviteSchoolAdmin(t *testing.T) {
	t.Parallel()
	expiresAt := time.Now().Add(time.Hour).UTC()
	first := "Ada"
	last := "Lovelace"
	position := "Principal"
	token := "seed-token"
	roleName := "admin"
	creatorEmail := "operator@example.com"

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		inviteSchoolAdminFn: func(_ context.Context, schoolID, operatorID int64, clientIP net.IP, req organizationtenancy.SchoolAdminInvitationInput) (*organizationtenancy.SchoolAdminInvitation, error) {
			assert.Equal(t, int64(12), schoolID)
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, "203.0.113.5", clientIP.String())
			require.NotNil(t, req.FirstName)
			require.NotNil(t, req.LastName)
			require.NotNil(t, req.Position)
			assert.Equal(t, "principal@example.com", req.Email)
			assert.True(t, req.CaregiverEnabled)
			return &organizationtenancy.SchoolAdminInvitation{
				ID:               5,
				Email:            req.Email,
				RoleID:           9,
				Token:            token,
				ExpiresAt:        expiresAt,
				FirstName:        &first,
				LastName:         &last,
				Position:         &position,
				CaregiverEnabled: req.CaregiverEnabled,
				CreatedBy:        nil,
				RoleName:         roleName,
				CreatorEmail:     creatorEmail,
				EmailError:       nil,
			}, nil
		},
	}})

	resource.appEnv = "development"

	req := httptest.NewRequest(http.MethodPost, "http://localhost/operator/schools/12/invite-admin", bytes.NewBufferString(`{"email":" PRINCIPAL@example.com ","first_name":" Ada ","last_name":" Lovelace ","position":" Principal ","caregiver_enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(seedInvitationTokenHeader, "true")
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		inviteSchoolAdminFn: func(context.Context, int64, int64, net.IP, organizationtenancy.SchoolAdminInvitationInput) (*organizationtenancy.SchoolAdminInvitation, error) {
			return nil, &organizationtenancy.ProvisioningIdentityError{Op: "create invitation", Err: errors.New("invalid email")}
		},
	}})

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

func TestProvisioningHelpers(t *testing.T) {
	t.Parallel()
	errText := "boom"
	now := time.Now()

	assert.Equal(t, int64(0), operatorInvitationCreatedByValue(nil))
	assert.Equal(t, int64(9), operatorInvitationCreatedByValue(ptrInt64(9)))
	assert.Equal(t, "pending", operatorInvitationDeliveryStatus(nil, nil))
	assert.Equal(t, "failed", operatorInvitationDeliveryStatus(nil, &errText))
	assert.Equal(t, "sent", operatorInvitationDeliveryStatus(&now, &errText))

	req := httptest.NewRequest(http.MethodPost, "http://localhost/", nil)
	assert.False(t, shouldExposeSeedInvitationToken(req, "development"))
	req.Header.Set(seedInvitationTokenHeader, "true")
	assert.True(t, shouldExposeSeedInvitationToken(req, "development"))
	assert.False(t, shouldExposeSeedInvitationToken(req, "production"))
	assert.False(t, shouldExposeSeedInvitationToken(req, "staging"))
	assert.False(t, shouldExposeSeedInvitationToken(req, "developement"))

	publicReq := httptest.NewRequest(http.MethodPost, "https://api-staging.moto-app.de/", nil)
	publicReq.Header.Set(seedInvitationTokenHeader, "true")
	assert.False(t, shouldExposeSeedInvitationToken(publicReq, "development"))
}

func TestProvisioningErrorRenderer_NotFoundAndFallbacks(t *testing.T) {
	t.Parallel()

	cases := []struct {
		err        error
		statusCode int
	}{
		{err: &organizationtenancy.OrganizationNotFoundError{OrganizationID: 1}, statusCode: http.StatusNotFound},
		{err: &organizationtenancy.SchoolNotFoundError{SchoolID: 1}, statusCode: http.StatusNotFound},
		{err: &organizationtenancy.InvalidProvisioningDataError{Err: errors.New("bad")}, statusCode: http.StatusBadRequest},
		{err: &organizationtenancy.InvalidProvisioningDataError{Err: errors.New("bad")}, statusCode: http.StatusBadRequest},
		{err: errors.New("plain"), statusCode: http.StatusInternalServerError},
	}

	for _, tc := range cases {
		renderer := ProvisioningErrorRenderer(tc.err)
		resp, ok := renderer.(*common.OperatorErrResponse)
		require.True(t, ok)
		assert.Equal(t, tc.statusCode, resp.HTTPStatusCode)
	}
}

func TestProvisioningResource_UpdateOrganization(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		updateOrganizationFn: func(_ context.Context, id int64, req organizationtenancy.OrganizationChanges, operatorID int64, clientIP net.IP) (*organizationtenancy.Organization, error) {
			assert.Equal(t, int64(5), id)
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, "Updated Org", req.Name)
			assert.Equal(t, "updated-org", req.Slug)
			assert.True(t, req.Active)
			assert.Equal(t, "203.0.113.10", clientIP.String())
			return &organizationtenancy.Organization{ID: 5, Name: "Updated Org", Slug: "updated-org", Active: true}, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		updateOrganizationFn: func(context.Context, int64, organizationtenancy.OrganizationChanges, int64, net.IP) (*organizationtenancy.Organization, error) {
			return nil, &organizationtenancy.OrganizationNotFoundError{OrganizationID: 5}
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		updateOrganizationFn: func(context.Context, int64, organizationtenancy.OrganizationChanges, int64, net.IP) (*organizationtenancy.Organization, error) {
			return nil, &organizationtenancy.ProvisioningConflictError{Err: errors.New("slug taken")}
		},
	}})

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

func TestProvisioningResource_UpdateOrganization_ProvisioningConflict(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		updateOrganizationFn: func(context.Context, int64, organizationtenancy.OrganizationChanges, int64, net.IP) (*organizationtenancy.Organization, error) {
			return nil, &organizationtenancy.ProvisioningConflictError{Err: errors.New("slug taken")}
		},
	}})

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
	assert.Equal(t, "slug taken", decodeBody(t, rr)["message"])
}

func TestProvisioningResource_UpdateSchool(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		updateSchoolFn: func(_ context.Context, id int64, req organizationtenancy.SchoolChanges, operatorID int64, clientIP net.IP) (*organizationtenancy.School, error) {
			assert.Equal(t, int64(10), id)
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, "Updated School", req.Name)
			assert.Equal(t, "updated-school", req.Slug)
			assert.Equal(t, "updated-sub", req.Subdomain)
			assert.Equal(t, "school@example.com", req.Email)
			assert.Equal(t, "198.51.100.20", clientIP.String())
			return &organizationtenancy.School{ID: 10, Name: "Updated School", Slug: "updated-school", Subdomain: "updated-sub"}, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		updateSchoolFn: func(_ context.Context, id int64, req organizationtenancy.SchoolChanges, operatorID int64, _ net.IP) (*organizationtenancy.School, error) {
			assert.Equal(t, int64(10), id)
			assert.True(t, req.Hidden, "Hidden field must be passed through from handler")
			assert.Equal(t, "Hidden School", req.Name)
			return &organizationtenancy.School{ID: 10, Name: "Hidden School", Slug: "hidden", Subdomain: "hidden", Hidden: true}, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		createSchoolFn: func(_ context.Context, school *organizationtenancy.CreateSchool, operatorID int64, _ net.IP) (*organizationtenancy.School, error) {
			assert.True(t, school.Hidden, "Hidden field must be passed through from create handler")
			assert.Equal(t, "Demo School", school.Name)
			return createdSchool(99, school), nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		updateSchoolFn: func(context.Context, int64, organizationtenancy.SchoolChanges, int64, net.IP) (*organizationtenancy.School, error) {
			return nil, &organizationtenancy.SchoolNotFoundError{SchoolID: 10}
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		updateSchoolFn: func(context.Context, int64, organizationtenancy.SchoolChanges, int64, net.IP) (*organizationtenancy.School, error) {
			return nil, &organizationtenancy.ProvisioningConflictError{Err: errors.New("subdomain taken")}
		},
	}})

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

func TestProvisioningResource_UpdateSchool_ProvisioningConflict(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		updateSchoolFn: func(context.Context, int64, organizationtenancy.SchoolChanges, int64, net.IP) (*organizationtenancy.School, error) {
			return nil, &organizationtenancy.ProvisioningConflictError{Err: errors.New("subdomain taken")}
		},
	}})

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
	assert.Equal(t, "subdomain taken", decodeBody(t, rr)["message"])
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

	expected := []organizationtenancy.SchoolAccount{
		{AccountID: 1, Email: "admin@example.com", Active: true, RoleName: "admin"},
	}
	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listSchoolAccountsFn: func(_ context.Context, schoolID int64) ([]organizationtenancy.SchoolAccount, error) {
			assert.Equal(t, int64(7), schoolID)
			return expected, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listSchoolAccountsFn: func(_ context.Context, _ int64) ([]organizationtenancy.SchoolAccount, error) {
			return nil, &organizationtenancy.SchoolNotFoundError{SchoolID: 7}
		},
	}})

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

	expected := []organizationtenancy.OrganizationAccount{
		{SchoolAccount: organizationtenancy.SchoolAccount{AccountID: 1, Email: "admin@example.com"}, SchoolID: 9, SchoolName: "School A"},
	}
	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listOrgAccountsFn: func(_ context.Context, orgID int64) ([]organizationtenancy.OrganizationAccount, error) {
			assert.Equal(t, int64(3), orgID)
			return expected, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listOrgAccountsFn: func(_ context.Context, _ int64) ([]organizationtenancy.OrganizationAccount, error) {
			return nil, &organizationtenancy.OrganizationNotFoundError{OrganizationID: 3}
		},
	}})

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

	expected := []organizationtenancy.OrganizationAccount{
		{SchoolAccount: organizationtenancy.SchoolAccount{AccountID: 1, Email: "admin@example.com"}, SchoolID: 9},
	}
	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listAllAccountsFn: func(_ context.Context) ([]organizationtenancy.OrganizationAccount, error) {
			return expected, nil
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/operator/accounts", nil)
	rr := httptest.NewRecorder()

	resource.ListAllAccounts(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestProvisioningResource_ListAllAccounts_Error(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listAllAccountsFn: func(_ context.Context) ([]organizationtenancy.OrganizationAccount, error) {
			return nil, errors.New("db fail")
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/operator/accounts", nil)
	rr := httptest.NewRecorder()

	resource.ListAllAccounts(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// --- Device listing handler tests ---

func TestProvisioningResource_ListAllDevices(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listAllDevicesFn: func(_ context.Context) ([]organizationtenancy.OperatorDevice, error) {
			return []organizationtenancy.OperatorDevice{{ID: 1, DeviceID: "dev-1"}}, nil
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/operator/devices", nil)
	rr := httptest.NewRecorder()

	resource.ListAllDevices(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestProvisioningResource_ListAllDevices_Error(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listAllDevicesFn: func(_ context.Context) ([]organizationtenancy.OperatorDevice, error) {
			return nil, errors.New("db fail")
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/operator/devices", nil)
	rr := httptest.NewRecorder()

	resource.ListAllDevices(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestProvisioningResource_ListSchoolDevices(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listSchoolDevicesFn: func(_ context.Context, schoolID int64) ([]organizationtenancy.OperatorDevice, error) {
			assert.Equal(t, int64(7), schoolID)
			return []organizationtenancy.OperatorDevice{{ID: 1, DeviceID: "dev-1"}}, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listSchoolDevicesFn: func(_ context.Context, _ int64) ([]organizationtenancy.OperatorDevice, error) {
			return nil, &organizationtenancy.SchoolNotFoundError{SchoolID: 7}
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listOrganizationDevicesFn: func(_ context.Context, orgID int64) ([]organizationtenancy.OperatorDevice, error) {
			assert.Equal(t, int64(3), orgID)
			return []organizationtenancy.OperatorDevice{{ID: 1, DeviceID: "dev-1"}}, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listOrganizationDevicesFn: func(_ context.Context, _ int64) ([]organizationtenancy.OperatorDevice, error) {
			return nil, &organizationtenancy.OrganizationNotFoundError{OrganizationID: 3}
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		createDeviceFn: func(_ context.Context, schoolID int64, deviceID, deviceType string, name, apiKey *string, operatorID int64, clientIP net.IP) (*organizationtenancy.OperatorDevice, error) {
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, int64(9), schoolID)
			assert.Equal(t, "DEV-123", deviceID)
			assert.Equal(t, "terminal", deviceType)
			require.NotNil(t, name)
			require.NotNil(t, apiKey)
			assert.Equal(t, "Eingang", *name)
			assert.Equal(t, "manual-key", *apiKey)
			assert.Equal(t, "203.0.113.77", clientIP.String())
			return &organizationtenancy.OperatorDevice{ID: 5, DeviceID: deviceID, APIKey: apiKey}, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
	req := httptest.NewRequest(http.MethodPost, "/operator/devices", bytes.NewBufferString(`{"school_id":0,"device_id":"","device_type":""}`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.CreateDevice(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_CreateDevice_Conflict(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		createDeviceFn: func(_ context.Context, _ int64, _, _ string, _, _ *string, _ int64, _ net.IP) (*organizationtenancy.OperatorDevice, error) {
			return nil, &organizationtenancy.ProvisioningConflictError{Err: errors.New("api_key already exists")}
		},
	}})

	req := httptest.NewRequest(http.MethodPost, "/operator/devices", bytes.NewBufferString(`{"school_id":9,"device_id":"DEV-123","device_type":"terminal"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.CreateDevice(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestProvisioningResource_CreateDevice_ProvisioningConflict(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		createDeviceFn: func(_ context.Context, _ int64, _, _ string, _, _ *string, _ int64, _ net.IP) (*organizationtenancy.OperatorDevice, error) {
			return nil, &organizationtenancy.ProvisioningConflictError{Err: errors.New("api_key already exists")}
		},
	}})

	req := httptest.NewRequest(http.MethodPost, "/operator/devices", bytes.NewBufferString(`{"school_id":9,"device_id":"DEV-123","device_type":"terminal"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.CreateDevice(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Equal(t, "api_key already exists", decodeBody(t, rr)["message"])
}

func TestProvisioningResource_SetDeviceAPIKey(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		setDeviceAPIKeyFn: func(_ context.Context, deviceID int64, apiKey *string, operatorID int64, clientIP net.IP) (*organizationtenancy.OperatorDevice, error) {
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, int64(17), deviceID)
			require.NotNil(t, apiKey)
			assert.Equal(t, "manual-key", *apiKey)
			assert.Equal(t, "198.51.100.30", clientIP.String())
			return &organizationtenancy.OperatorDevice{ID: deviceID, APIKey: apiKey}, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		setDeviceAPIKeyFn: func(_ context.Context, deviceID int64, apiKey *string, _ int64, _ net.IP) (*organizationtenancy.OperatorDevice, error) {
			assert.Equal(t, int64(17), deviceID)
			assert.Nil(t, apiKey)
			return &organizationtenancy.OperatorDevice{ID: deviceID, APIKey: ptrString("generated-key")}, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		setDeviceAPIKeyFn: func(_ context.Context, _ int64, _ *string, _ int64, _ net.IP) (*organizationtenancy.OperatorDevice, error) {
			return nil, &organizationtenancy.OperatorDeviceNotFoundError{DeviceID: 17}
		},
	}})

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
	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		getDeviceTransferStatusFn: func(_ context.Context, deviceID int64) (*organizationtenancy.DeviceTransferStatus, error) {
			assert.Equal(t, int64(17), deviceID)
			return &organizationtenancy.DeviceTransferStatus{CanTransfer: true, LastSeen: &lastSeen}, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		transferDeviceFn: func(_ context.Context, deviceID, targetSchoolID, operatorID int64, clientIP net.IP) (*organizationtenancy.OperatorDevice, error) {
			assert.Equal(t, int64(17), deviceID)
			assert.Equal(t, int64(23), targetSchoolID)
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, "198.51.100.30", clientIP.String())
			return &organizationtenancy.OperatorDevice{ID: 99, DeviceID: "DEV-17", SchoolID: targetSchoolID}, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		transferDeviceFn: func(_ context.Context, deviceID, _ int64, _ int64, _ net.IP) (*organizationtenancy.OperatorDevice, error) {
			return nil, &organizationtenancy.DeviceTransferBlockedError{DeviceID: deviceID, Reason: organizationtenancy.DeviceTransferBlockedOnline}
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		createSchoolAccountFn: func(_ context.Context, schoolID, operatorID int64, clientIP net.IP, req organizationtenancy.SchoolAccountInput) (*organizationtenancy.CreatedAccount, error) {
			assert.Equal(t, int64(7), schoolID)
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, "203.0.113.50", clientIP.String())
			assert.Equal(t, "teacher@example.com", req.Email)
			assert.Equal(t, "Ada", req.FirstName)
			assert.Equal(t, "Lovelace", req.LastName)
			assert.Equal(t, "Secure123!", req.Password)
			assert.Equal(t, "Lehrerin", req.Position)
			require.NotNil(t, req.RoleID)
			assert.Equal(t, int64(9007199254740993), *req.RoleID)
			return &organizationtenancy.CreatedAccount{ID: 99, Email: "teacher@example.com"}, nil
		},
	}})

	body := `{"email":" TEACHER@example.com ","first_name":" Ada ","last_name":" Lovelace ","password":"Secure123!","confirm_password":"Secure123!","role_id":"9007199254740993","position":" Lehrerin "}`
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		createSchoolAccountFn: func(context.Context, int64, int64, net.IP, organizationtenancy.SchoolAccountInput) (*organizationtenancy.CreatedAccount, error) {
			return nil, &organizationtenancy.ProvisioningIdentityError{Op: "create account", Err: organizationtenancy.ErrAccountEmailExists}
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listSystemRolesFn: func(_ context.Context) ([]organizationtenancy.SystemRole, error) {
			return []organizationtenancy.SystemRole{{ID: 9007199254740993, Name: "admin"}, {Name: "teacher"}}, nil
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/operator/roles", nil)
	rr := httptest.NewRecorder()

	resource.ListSystemRoles(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	body := decodeBody(t, rr)
	roles := body["data"].([]any)
	assert.Equal(t, "9007199254740993", roles[0].(map[string]any)["id"])
}

func TestProvisioningResource_ListSystemRoles_Error(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listSystemRolesFn: func(_ context.Context) ([]organizationtenancy.SystemRole, error) {
			return nil, errors.New("db fail")
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/operator/roles", nil)
	rr := httptest.NewRecorder()

	resource.ListSystemRoles(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// --- createSchoolAccountRequest Bind tests ---

func TestCreateSchoolAccountRequest_Bind_TrimAndLowercaseEmail(t *testing.T) {
	t.Parallel()

	roleID := common.JSONID(3)
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

	renderer := ProvisioningErrorRenderer(&organizationtenancy.SchoolInactiveError{SchoolID: 1})
	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusForbidden, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "inactive")
}

func TestProvisioningErrorRenderer_DeviceNotFound(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.OperatorDeviceNotFoundError{DeviceID: 42})
	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
}

func TestProvisioningErrorRenderer_AuthEmailAlreadyExists(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.ProvisioningIdentityError{Op: "create", Err: organizationtenancy.ErrAccountEmailExists})
	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
}

func TestProvisioningErrorRenderer_AuthUsernameAlreadyExists(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.ProvisioningIdentityError{Op: "create", Err: organizationtenancy.ErrAccountUsernameExists})
	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
}

func TestProvisioningErrorRenderer_AuthPasswordMismatch(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.ProvisioningIdentityError{Op: "create", Err: organizationtenancy.ErrPasswordMismatch})
	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
}

func TestProvisioningErrorRenderer_AuthPasswordTooWeak(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.ProvisioningIdentityError{Op: "create", Err: organizationtenancy.ErrPasswordTooWeak})
	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
}

func TestProvisioningErrorRenderer_AuthInvitationNameRequired(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.ProvisioningIdentityError{Op: "create invitation", Err: organizationtenancy.ErrInvitationNameRequired})
	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
}

func TestProvisioningErrorRenderer_AuthGenericInvitationError(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.ProvisioningIdentityError{Op: "create invitation", Err: errors.New("some validation error")})
	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
}

func TestProvisioningErrorRenderer_AuthDefaultError(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.ProvisioningIdentityError{Op: "some op", Err: errors.New("db error")})
	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
}

// --- Person handler tests ---

func TestProvisioningResource_ListSchoolPersons_Success(t *testing.T) {
	t.Parallel()

	expected := []organizationtenancy.OperatorPerson{
		{ID: 10, FirstName: "Ada", LastName: "Lovelace", SchoolID: 7, SchoolName: "Test School"},
		{ID: 11, FirstName: "Grace", LastName: "Hopper", SchoolID: 7, SchoolName: "Test School"},
	}
	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listSchoolPersonsFn: func(_ context.Context, schoolID int64) ([]organizationtenancy.OperatorPerson, error) {
			assert.Equal(t, int64(7), schoolID)
			return expected, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listSchoolPersonsFn: func(_ context.Context, _ int64) ([]organizationtenancy.OperatorPerson, error) {
			return nil, &organizationtenancy.SchoolNotFoundError{SchoolID: 7}
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		softDeletePersonFn: func(_ context.Context, personID int64, operatorID int64, clientIP net.IP) error {
			assert.Equal(t, int64(15), personID)
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, "203.0.113.10", clientIP.String())
			return nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		softDeletePersonFn: func(_ context.Context, _ int64, _ int64, _ net.IP) error {
			return &organizationtenancy.PersonNotFoundError{PersonID: 15}
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		softDeletePersonFn: func(_ context.Context, _ int64, _ int64, _ net.IP) error {
			return &organizationtenancy.PersonHasActiveSupervisionsError{PersonID: 15, Count: 2}
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	renderer := ProvisioningErrorRenderer(&organizationtenancy.PersonNotFoundError{PersonID: 42})
	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
	assert.Equal(t, "Person not found", resp.ErrorText)
}

func TestProvisioningErrorRenderer_PersonActiveSupervisors(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.PersonHasActiveSupervisionsError{PersonID: 42, Count: 3})
	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Equal(t, "Person has active supervisions and cannot be deleted", resp.ErrorText)
}

func ptrInt64(v int64) *int64    { return &v }
func ptrString(v string) *string { return &v }

var _ organizationtenancy.Provisioning = (*mockProvisioningService)(nil)

// --- SoftDeleteSchool handler tests ---

func TestProvisioningResource_SoftDeleteSchool(t *testing.T) {
	t.Parallel()

	var deletedID int64
	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		softDeleteSchoolFn: func(schoolID int64) error {
			deletedID = schoolID
			return nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		softDeleteSchoolFn: func(_ int64) error {
			return &organizationtenancy.SchoolNotFoundError{SchoolID: 99}
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		softDeleteSchoolFn: func(_ int64) error {
			return &organizationtenancy.SchoolAlreadyDeletedError{SchoolID: 55}
		},
	}})

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
	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		restoreSchoolFn: func(schoolID int64) error {
			restoredID = schoolID
			return nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		restoreSchoolFn: func(_ int64) error {
			return &organizationtenancy.SchoolNotFoundError{SchoolID: 99}
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		restoreSchoolFn: func(_ int64) error {
			return &organizationtenancy.SchoolNotDeletedError{SchoolID: 55}
		},
	}})

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

	renderer := ProvisioningErrorRenderer(&organizationtenancy.SchoolAlreadyDeletedError{SchoolID: 55})
	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "already deleted")
}

func TestProvisioningErrorRenderer_SchoolNotDeleted(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.SchoolNotDeletedError{SchoolID: 55})
	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "not deleted")
}

// --- SoftDeleteOrganization handler tests ---

func TestProvisioningResource_SoftDeleteOrganization(t *testing.T) {
	t.Parallel()

	var deletedID int64
	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		softDeleteOrgFn: func(orgID int64) error {
			deletedID = orgID
			return nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		softDeleteOrgFn: func(_ int64) error {
			return &organizationtenancy.OrganizationHasSchoolsError{SchoolCount: 3}
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		softDeleteOrgFn: func(_ int64) error {
			return &organizationtenancy.OrganizationAlreadyDeletedError{OrganizationID: 10}
		},
	}})

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
	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		restoreOrgFn: func(orgID int64) error {
			restoredID = orgID
			return nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		restoreOrgFn: func(_ int64) error {
			return &organizationtenancy.OrganizationNotDeletedError{OrganizationID: 10}
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	renderer := ProvisioningErrorRenderer(&organizationtenancy.OrganizationAlreadyDeletedError{OrganizationID: 10})
	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "already deleted")
}

func TestProvisioningErrorRenderer_OrganizationNotDeleted(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.OrganizationNotDeletedError{OrganizationID: 10})
	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "not deleted")
}

func TestProvisioningErrorRenderer_OrganizationHasSchools(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.OrganizationHasSchoolsError{SchoolCount: 3})
	resp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
	assert.Contains(t, resp.ErrorText, "3 existing school(s)")
}

func TestProvisioningErrorRenderer_OrganizationDeleted(t *testing.T) {
	t.Parallel()

	renderer := ProvisioningErrorRenderer(&organizationtenancy.OrganizationDeletedError{OrganizationID: 10})
	resp, ok := renderer.(*common.OperatorErrResponse)
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
		renderer := caregiverCapabilityProvisioningErrorRenderer(fakeCaregiverBlockedError{
			blockers: []string{"active_group_supervisions", "group_assignments"},
		})

		blocked, ok := renderer.(*common.CaregiverCapabilityBlockedResponse)
		require.True(t, ok)
		assert.Equal(t, http.StatusConflict, blocked.HTTPStatusCode)
		assert.Equal(
			t,
			[]string{"active_group_supervisions", "group_assignments"},
			blocked.Blockers,
		)
	})

	t.Run("maps missing account to not found", func(t *testing.T) {
		renderer := caregiverCapabilityProvisioningErrorRenderer(fakeCaregiverAccountMissingError{
			error: errors.New("account 77 is not assigned to tenant 4"),
		})

		resp, ok := renderer.(*common.OperatorErrResponse)
		require.True(t, ok)
		assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
		assert.Equal(t, "Account not found", resp.ErrorText)
	})

	t.Run("maps invalid data to bad request", func(t *testing.T) {
		renderer := caregiverCapabilityProvisioningErrorRenderer(&organizationtenancy.InvalidProvisioningDataError{
			Err: errors.New("invalid caregiver input"),
		})

		resp, ok := renderer.(*common.OperatorErrResponse)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
		assert.Equal(t, "invalid caregiver input", resp.ErrorText)
	})

	t.Run("maps invalid provisioning data to bad request", func(t *testing.T) {
		renderer := caregiverCapabilityProvisioningErrorRenderer(&organizationtenancy.InvalidProvisioningDataError{
			Err: errors.New("invalid caregiver input"),
		})

		resp, ok := renderer.(*common.OperatorErrResponse)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
		assert.Equal(t, "invalid caregiver input", resp.ErrorText)
	})

	t.Run("maps wrapped validation errors to bad request", func(t *testing.T) {
		invalid := fmt.Errorf("validation: %w", errors.New("first_name is required"))
		renderer := caregiverCapabilityProvisioningErrorRenderer(fakeCaregiverRequestInvalidError{
			error:   fmt.Errorf("users.enable caregiver capability: %w", invalid),
			invalid: invalid,
		})

		resp, ok := renderer.(*common.OperatorErrResponse)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
		assert.Equal(t, "first_name is required", resp.ErrorText)
	})

	t.Run("preserves internal users errors as internal server errors", func(t *testing.T) {
		cause := errors.New("audit write failed")
		renderer := caregiverCapabilityProvisioningErrorRenderer(fakeCaregiverFailureError{
			error: fmt.Errorf("users.enable caregiver capability: %w", cause),
			cause: cause,
		})

		resp, ok := renderer.(*common.OperatorErrResponse)
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
		resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
			deleteDeviceFn: func(_ context.Context, id int64, operatorID int64, clientIP net.IP) error {
				assert.Equal(t, int64(55), id)
				assert.Equal(t, int64(42), operatorID)
				assert.Equal(t, "203.0.113.50", clientIP.String())
				return nil
			},
		}})

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
		resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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
		resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
			deleteDeviceFn: func(context.Context, int64, int64, net.IP) error {
				return &organizationtenancy.OperatorDeviceNotFoundError{DeviceID: 55}
			},
		}})

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

	called := false
	resource := &ProvisioningResource{
		CaregiverCapabilityService: &fakeSchoolAccountCaregivers{
			getFn: func(_ context.Context, schoolID, accountID int64) (json.RawMessage, error) {
				called = true
				assert.Equal(t, int64(12), schoolID)
				assert.Equal(t, int64(34), accountID)
				return json.RawMessage(`{"account_id":34,"has_user_role":true}`), nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/operator/schools/12/accounts/34/caregiver-capability", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "12")
	routeCtx.URLParams.Add("accountId", "34")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.GetSchoolAccountCaregiverCapability(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	require.True(t, called)
}

func TestProvisioningResource_EnableSchoolAccountCaregiverCapability(t *testing.T) {
	t.Parallel()

	called := false
	resource := &ProvisioningResource{
		CaregiverCapabilityService: &fakeSchoolAccountCaregivers{
			enableFn: func(_ context.Context, schoolID, accountID int64, firstName, lastName, position string) (json.RawMessage, error) {
				called = true
				assert.Equal(t, int64(12), schoolID)
				assert.Equal(t, int64(34), accountID)
				assert.Equal(t, "Ada", firstName)
				assert.Equal(t, "Lovelace", lastName)
				assert.Equal(t, "Springer", position)
				return json.RawMessage(`{"account_id":34,"has_user_role":true}`), nil
			},
		},
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
	require.True(t, called)
}

func TestProvisioningResource_EnableSchoolAccountCaregiverCapability_EmptyBody(t *testing.T) {
	t.Parallel()

	resource := &ProvisioningResource{
		CaregiverCapabilityService: &fakeSchoolAccountCaregivers{},
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

	called := false
	resource := &ProvisioningResource{
		CaregiverCapabilityService: &fakeSchoolAccountCaregivers{
			disableFn: func(_ context.Context, schoolID, accountID int64) (json.RawMessage, error) {
				called = true
				assert.Equal(t, int64(12), schoolID)
				assert.Equal(t, int64(34), accountID)
				return nil, fakeCaregiverBlockedError{blockers: []string{"active_group_supervisions"}}
			},
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/operator/schools/12/accounts/34/caregiver-capability", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "12")
	routeCtx.URLParams.Add("accountId", "34")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.DisableSchoolAccountCaregiverCapability(rr, req)
	assert.Equal(t, http.StatusConflict, rr.Code)
	require.True(t, called)
}

// --- Provisioning summaries (drill-in refactor) ---

func TestProvisioningResource_GetProvisioningStats(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		getProvisioningStatsFn: func(context.Context) (*organizationtenancy.ProvisioningStats, error) {
			return &organizationtenancy.ProvisioningStats{
				TraegerCount: 3,
				SchulenCount: 7,
				KontenCount:  42,
				GeraeteCount: 11,
			}, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		getProvisioningStatsFn: func(context.Context) (*organizationtenancy.ProvisioningStats, error) {
			return nil, errors.New("db fail")
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/operator/stats", nil)
	rr := httptest.NewRecorder()

	resource.GetProvisioningStats(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestProvisioningResource_ListOrganizationSummaries(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listOrgSummariesFn: func(context.Context) ([]*organizationtenancy.OrganizationSummary, error) {
			return []*organizationtenancy.OrganizationSummary{
				{ID: 1, Name: "Org One", Slug: "org-one", Active: true, SchulenCount: 2, KontenCount: 5, GeraeteCount: 3, PersonenCount: 18},
			}, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listOrgSummariesFn: func(context.Context) ([]*organizationtenancy.OrganizationSummary, error) {
			return nil, errors.New("db fail")
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/operator/organizations/summaries", nil)
	rr := httptest.NewRecorder()

	resource.ListOrganizationSummaries(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestProvisioningResource_ListSchoolSummaries(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listSchoolSummariesFn: func(context.Context) ([]*organizationtenancy.SchoolSummary, error) {
			return []*organizationtenancy.SchoolSummary{
				{ID: 10, OrganizationID: 1, OrganizationName: "Org One", Name: "School A", Slug: "school-a", Subdomain: "a", Active: true, KontenCount: 4, GeraeteCount: 2, PersonenCount: 30},
			}, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listSchoolSummariesFn: func(context.Context) ([]*organizationtenancy.SchoolSummary, error) {
			return nil, errors.New("db fail")
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/operator/schools/summaries", nil)
	rr := httptest.NewRecorder()

	resource.ListSchoolSummaries(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestProvisioningResource_ListOrganizationSchoolSummaries(t *testing.T) {
	t.Parallel()

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listOrgSchoolSummariesFn: func(_ context.Context, orgID int64) ([]*organizationtenancy.SchoolSummary, error) {
			assert.Equal(t, int64(7), orgID)
			return []*organizationtenancy.SchoolSummary{
				{ID: 99, OrganizationID: 7, Name: "Schule X", Slug: "schule-x"},
			}, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listOrgSchoolSummariesFn: func(_ context.Context, orgID int64) ([]*organizationtenancy.SchoolSummary, error) {
			return nil, &organizationtenancy.OrganizationNotFoundError{OrganizationID: orgID}
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listOrgPersonsFn: func(_ context.Context, orgID int64) ([]organizationtenancy.OperatorPerson, error) {
			assert.Equal(t, int64(7), orgID)
			return []organizationtenancy.OperatorPerson{
				{ID: 1, FirstName: "Ada", LastName: "Lovelace", SchoolID: 10, SchoolName: "School A", OrganizationID: 7, OrganizationName: "Org One"},
			}, nil
		},
	}})

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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
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

	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
		listOrgPersonsFn: func(_ context.Context, orgID int64) ([]organizationtenancy.OperatorPerson, error) {
			return nil, &organizationtenancy.OrganizationNotFoundError{OrganizationID: orgID}
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/operator/organizations/9999/persons", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "9999")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.ListOrganizationPersons(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
