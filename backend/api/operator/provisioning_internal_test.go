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
	createOrganizationFn func(context.Context, *platformModels.Organization, int64, net.IP) (*platformModels.Organization, error)
	listOrganizationsFn  func(context.Context) ([]*platformModels.Organization, error)
	updateOrganizationFn func(context.Context, int64, platformSvc.UpdateOrganizationRequest, int64, net.IP) (*platformModels.Organization, error)
	createSchoolFn       func(context.Context, *platformModels.School, int64, net.IP) (*platformModels.School, error)
	listSchoolsFn        func(context.Context) ([]*platformModels.School, error)
	updateSchoolFn       func(context.Context, int64, platformSvc.UpdateSchoolRequest, int64, net.IP) (*platformModels.School, error)
	inviteSchoolAdminFn  func(context.Context, int64, int64, net.IP, authSvc.InvitationRequest) (*authModels.InvitationToken, error)
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

func ptrInt64(v int64) *int64 { return &v }

var _ platformSvc.OperatorProvisioningService = (*mockProvisioningService)(nil)
var _ interface{ WithTx(bun.Tx) interface{} } = (*mockProvisioningService)(nil)

func (m *mockProvisioningService) WithTx(_ bun.Tx) interface{} { return m }
