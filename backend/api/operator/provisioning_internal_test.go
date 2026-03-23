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

	"github.com/go-chi/chi/v5"
	jwtPkg "github.com/moto-nrw/project-phoenix/auth/jwt"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type mockProvisioningService struct {
	createOrganizationFn      func(context.Context, *platformModels.Organization, int64, net.IP) (*platformModels.Organization, error)
	listOrganizationsFn       func(context.Context) ([]*platformModels.Organization, error)
	updateOrganizationFn      func(context.Context, int64, platformSvc.UpdateOrganizationRequest, int64, net.IP) (*platformModels.Organization, error)
	createSchoolFn            func(context.Context, *platformModels.School, int64, net.IP) (*platformModels.School, error)
	listSchoolsFn             func(context.Context) ([]*platformModels.School, error)
	updateSchoolFn            func(context.Context, int64, platformSvc.UpdateSchoolRequest, int64, net.IP) (*platformModels.School, error)
	inviteSchoolAdminFn       func(context.Context, int64, int64, net.IP, authSvc.InvitationRequest) (*authModels.InvitationToken, error)
	listSchoolAccountsFn      func(context.Context, int64) ([]authModels.TenantAccountInfo, error)
	listOrgAccountsFn         func(context.Context, int64) ([]authModels.OrgAccountInfo, error)
	listAllAccountsFn         func(context.Context) ([]authModels.OrgAccountInfo, error)
	listAllDevicesFn          func(context.Context) ([]platformSvc.OperatorDeviceInfo, error)
	listSchoolDevicesFn       func(context.Context, int64) ([]platformSvc.OperatorDeviceInfo, error)
	listOrganizationDevicesFn func(context.Context, int64) ([]platformSvc.OperatorDeviceInfo, error)
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
	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodPost, "/operator/organizations", bytes.NewBufferString(`{"name":`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.CreateOrganization(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_ListOrganizations(t *testing.T) {
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
	resource := NewProvisioningResource(&mockProvisioningService{})
	req := httptest.NewRequest(http.MethodPost, "/operator/schools", bytes.NewBufferString(`{"organization_id":`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	rr := httptest.NewRecorder()

	resource.CreateSchool(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_ListSchools_Error(t *testing.T) {
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
			return &authModels.InvitationToken{
				Model:      modelBase.Model{ID: 5},
				Email:      req.Email,
				RoleID:     9,
				Token:      token,
				ExpiresAt:  expiresAt,
				FirstName:  &first,
				LastName:   &last,
				Position:   &position,
				CreatedBy:  nil,
				Role:       &authModels.Role{Name: roleName},
				Creator:    &authModels.Account{Email: creatorEmail},
				EmailError: nil,
			}, nil
		},
	})

	viper.Set("app_env", "development")
	req := httptest.NewRequest(http.MethodPost, "/operator/schools/12/invite-admin", bytes.NewBufferString(`{"email":" PRINCIPAL@example.com ","first_name":" Ada ","last_name":" Lovelace ","position":" Principal "}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(seedTokenHeader, "true")
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
}

func TestProvisioningResource_InviteSchoolAdmin_InvalidSchoolID(t *testing.T) {
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

func TestProvisioningHelpers(t *testing.T) {
	errText := "boom"
	now := time.Now()

	assert.Equal(t, int64(0), operatorInvitationCreatedByValue(nil))
	assert.Equal(t, int64(9), operatorInvitationCreatedByValue(ptrInt64(9)))
	assert.Equal(t, "pending", operatorInvitationDeliveryStatus(nil, nil))
	assert.Equal(t, "failed", operatorInvitationDeliveryStatus(nil, &errText))
	assert.Equal(t, "sent", operatorInvitationDeliveryStatus(&now, &errText))

	viper.Set("app_env", "development")
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	assert.False(t, shouldExposeSeedInvitationToken(req))
	req.Header.Set(seedTokenHeader, "true")
	assert.True(t, shouldExposeSeedInvitationToken(req))
	viper.Set("app_env", "production")
	assert.False(t, shouldExposeSeedInvitationToken(req))
}

func TestProvisioningErrorRenderer_NotFoundAndFallbacks(t *testing.T) {
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

func TestProvisioningResource_UpdateSchool_InvalidRequest(t *testing.T) {
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
	req := &updateOrganizationRequest{Name: "", Slug: "valid-slug", Active: true}
	err := req.Bind(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestUpdateOrganizationRequest_Bind_MissingSlug(t *testing.T) {
	req := &updateOrganizationRequest{Name: "Valid Name", Slug: "", Active: true}
	err := req.Bind(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slug is required")
}

func TestUpdateOrganizationRequest_Bind_TrimWhitespace(t *testing.T) {
	req := &updateOrganizationRequest{Name: "  Org Name  ", Slug: "  org-slug  ", Active: true}
	err := req.Bind(nil)
	require.NoError(t, err)
	assert.Equal(t, "Org Name", req.Name)
	assert.Equal(t, "org-slug", req.Slug)
}

func TestUpdateSchoolRequest_Bind_MissingName(t *testing.T) {
	req := &updateSchoolRequest{Name: "", Slug: "valid", Subdomain: "valid"}
	err := req.Bind(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestUpdateSchoolRequest_Bind_MissingSlug(t *testing.T) {
	req := &updateSchoolRequest{Name: "Valid", Slug: "", Subdomain: "valid"}
	err := req.Bind(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slug is required")
}

func TestUpdateSchoolRequest_Bind_MissingSubdomain(t *testing.T) {
	req := &updateSchoolRequest{Name: "Valid", Slug: "valid", Subdomain: ""}
	err := req.Bind(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subdomain is required")
}

func TestUpdateSchoolRequest_Bind_TrimAndLowercaseEmail(t *testing.T) {
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
	req := &createOrganizationRequest{Name: "  Org  ", Slug: "  org-slug  "}
	err := req.Bind(nil)
	require.NoError(t, err)
	assert.Equal(t, "Org", req.Name)
	assert.Equal(t, "org-slug", req.Slug)
}

func TestCreateSchoolRequest_Bind_TrimAndLowercaseEmail(t *testing.T) {
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
	req := &inviteSchoolAdminRequest{
		Email:     "  ADMIN@EXAMPLE.COM  ",
		FirstName: "  Ada  ",
		LastName:  "  Lovelace  ",
		Position:  "  Principal  ",
	}
	err := req.Bind(nil)
	require.NoError(t, err)
	assert.Equal(t, "admin@example.com", req.Email)
	assert.Equal(t, "Ada", req.FirstName)
	assert.Equal(t, "Lovelace", req.LastName)
	assert.Equal(t, "Principal", req.Position)
}

// --- Account listing handler tests ---

func TestProvisioningResource_ListSchoolAccounts(t *testing.T) {
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

func ptrInt64(v int64) *int64 { return &v }

var _ platformSvc.OperatorProvisioningService = (*mockProvisioningService)(nil)
var _ interface{ WithTx(bun.Tx) interface{} } = (*mockProvisioningService)(nil)

func (m *mockProvisioningService) WithTx(_ bun.Tx) interface{} { return m }
