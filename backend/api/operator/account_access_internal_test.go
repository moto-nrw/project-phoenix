package operator

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

func accountTenantRequest(t *testing.T, method, body string, params map[string]string) *http.Request {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, "/operator/accounts/7/tenants", nil)
	} else {
		req = httptest.NewRequest(method, "/operator/accounts/7/tenants", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req.RemoteAddr = "203.0.113.7:5555"
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	for key, value := range params {
		routeCtx.URLParams.Add(key, value)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}

func TestProvisioningResource_ListAccountTenantAccess(t *testing.T) {
	resource := NewProvisioningResource(&mockProvisioningService{
		listTenantAccessFn: func(_ context.Context, accountID int64) ([]platformSvc.AccountTenantAccessEntry, error) {
			assert.Equal(t, int64(7), accountID)
			return []platformSvc.AccountTenantAccessEntry{{
				Roles: []platformSvc.AccountTenantRole{{ID: 1, Name: "admin", IsSystem: true}},
			}}, nil
		},
	})

	rr := httptest.NewRecorder()
	resource.ListAccountTenantAccess(rr, accountTenantRequest(t, http.MethodGet, "", map[string]string{"accountId": "7"}))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	body := decodeBody(t, rr)
	entries := body["data"].([]any)
	require.Len(t, entries, 1)
	roles := entries[0].(map[string]any)["roles"].([]any)
	require.Len(t, roles, 1)
	assert.Equal(t, true, roles[0].(map[string]any)["is_system"])
}

func TestProvisioningResource_ListAccountTenantAccess_InvalidID(t *testing.T) {
	resource := NewProvisioningResource(&mockProvisioningService{})

	rr := httptest.NewRecorder()
	resource.ListAccountTenantAccess(rr, accountTenantRequest(t, http.MethodGet, "", map[string]string{"accountId": "nope"}))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_GrantAccountTenantAccess(t *testing.T) {
	resource := NewProvisioningResource(&mockProvisioningService{
		grantTenantAccessFn: func(_ context.Context, accountID, schoolID int64, req platformSvc.GrantAccountTenantAccessRequest, operatorID int64, clientIP net.IP) ([]platformSvc.AccountTenantAccessEntry, error) {
			assert.Equal(t, int64(7), accountID)
			assert.Equal(t, int64(12), schoolID)
			assert.Equal(t, int64(3), req.RoleID)
			assert.Equal(t, "Erika", req.FirstName)
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, "203.0.113.7", clientIP.String())
			return []platformSvc.AccountTenantAccessEntry{}, nil
		},
	})

	rr := httptest.NewRecorder()
	resource.GrantAccountTenantAccess(rr, accountTenantRequest(t, http.MethodPost,
		`{"school_id":12,"role_id":3,"first_name":"  Erika  ","last_name":"Muster"}`,
		map[string]string{"accountId": "7"}))

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
}

func TestProvisioningResource_GrantAccountTenantAccess_RequiresSchoolAndRole(t *testing.T) {
	resource := NewProvisioningResource(&mockProvisioningService{
		grantTenantAccessFn: func(context.Context, int64, int64, platformSvc.GrantAccountTenantAccessRequest, int64, net.IP) ([]platformSvc.AccountTenantAccessEntry, error) {
			t.Fatal("service must not be called for an invalid payload")
			return nil, nil
		},
	})

	for _, body := range []string{`{"role_id":3}`, `{"school_id":12}`} {
		rr := httptest.NewRecorder()
		resource.GrantAccountTenantAccess(rr, accountTenantRequest(t, http.MethodPost, body, map[string]string{"accountId": "7"}))
		assert.Equal(t, http.StatusBadRequest, rr.Code, body)
	}
}

func TestProvisioningResource_GrantAccountTenantAccess_ConflictIsSurfaced(t *testing.T) {
	resource := NewProvisioningResource(&mockProvisioningService{
		grantTenantAccessFn: func(context.Context, int64, int64, platformSvc.GrantAccountTenantAccessRequest, int64, net.IP) ([]platformSvc.AccountTenantAccessEntry, error) {
			return nil, &platformSvc.ConflictError{Err: assert.AnError}
		},
	})

	rr := httptest.NewRecorder()
	resource.GrantAccountTenantAccess(rr, accountTenantRequest(t, http.MethodPost,
		`{"school_id":12,"role_id":3}`, map[string]string{"accountId": "7"}))

	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestProvisioningResource_GrantAccountTenantAccess_InvalidRoleMessageSurvives(t *testing.T) {
	resource := NewProvisioningResource(&mockProvisioningService{
		grantTenantAccessFn: func(context.Context, int64, int64, platformSvc.GrantAccountTenantAccessRequest, int64, net.IP) ([]platformSvc.AccountTenantAccessEntry, error) {
			return nil, &platformSvc.InvalidDataError{Err: assert.AnError}
		},
	})

	rr := httptest.NewRecorder()
	resource.GrantAccountTenantAccess(rr, accountTenantRequest(t, http.MethodPost,
		`{"school_id":12,"role_id":3}`, map[string]string{"accountId": "7"}))

	require.Equal(t, http.StatusBadRequest, rr.Code)
	body := decodeBody(t, rr)
	// The generic provisioning renderer collapses this to "invalid input data";
	// the access renderer has to keep the concrete reason.
	assert.Contains(t, body["message"], assert.AnError.Error())
}

func TestProvisioningResource_UpdateAccountTenantRole(t *testing.T) {
	resource := NewProvisioningResource(&mockProvisioningService{
		updateTenantRoleFn: func(_ context.Context, accountID, schoolID, roleID, operatorID int64, _ net.IP) ([]platformSvc.AccountTenantAccessEntry, error) {
			assert.Equal(t, int64(7), accountID)
			assert.Equal(t, int64(12), schoolID)
			assert.Equal(t, int64(4), roleID)
			assert.Equal(t, int64(42), operatorID)
			return []platformSvc.AccountTenantAccessEntry{}, nil
		},
	})

	rr := httptest.NewRecorder()
	resource.UpdateAccountTenantRole(rr, accountTenantRequest(t, http.MethodPut, `{"role_id":4}`,
		map[string]string{"accountId": "7", "tenantId": "12"}))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestProvisioningResource_UpdateAccountTenantRole_RequiresRole(t *testing.T) {
	resource := NewProvisioningResource(&mockProvisioningService{})

	rr := httptest.NewRecorder()
	resource.UpdateAccountTenantRole(rr, accountTenantRequest(t, http.MethodPut, `{}`,
		map[string]string{"accountId": "7", "tenantId": "12"}))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_RevokeAccountTenantAccess(t *testing.T) {
	called := false
	resource := NewProvisioningResource(&mockProvisioningService{
		revokeTenantAccessFn: func(_ context.Context, accountID, schoolID, operatorID int64, _ net.IP) ([]platformSvc.AccountTenantAccessEntry, error) {
			called = true
			assert.Equal(t, int64(7), accountID)
			assert.Equal(t, int64(12), schoolID)
			assert.Equal(t, int64(42), operatorID)
			return []platformSvc.AccountTenantAccessEntry{}, nil
		},
	})

	rr := httptest.NewRecorder()
	resource.RevokeAccountTenantAccess(rr, accountTenantRequest(t, http.MethodDelete, "",
		map[string]string{"accountId": "7", "tenantId": "12"}))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.True(t, called)
}

func TestProvisioningResource_RevokeAccountTenantAccess_UnknownMapping(t *testing.T) {
	resource := NewProvisioningResource(&mockProvisioningService{
		revokeTenantAccessFn: func(context.Context, int64, int64, int64, net.IP) ([]platformSvc.AccountTenantAccessEntry, error) {
			return nil, &platformSvc.AccountTenantAccessNotFoundError{AccountID: 7, SchoolID: 12}
		},
	})

	rr := httptest.NewRecorder()
	resource.RevokeAccountTenantAccess(rr, accountTenantRequest(t, http.MethodDelete, "",
		map[string]string{"accountId": "7", "tenantId": "12"}))

	assert.Equal(t, http.StatusNotFound, rr.Code)
}
