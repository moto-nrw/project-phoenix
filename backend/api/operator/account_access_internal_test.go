package operator

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
)

// The account school-access routes are Identity & Access routes (#3252)
// mounted by this router; these cases pin their request parsing and the
// operator error format they answer with.

type mockAccountAccess struct {
	identityaccess.OperatorAuthentication
	listFn   func(context.Context, int64) ([]identityaccess.AccountTenantAccess, error)
	grantFn  func(context.Context, identityaccess.GrantAccountTenantAccess) ([]identityaccess.AccountTenantAccess, error)
	updateFn func(ctx context.Context, accountID, schoolID, roleID, operatorID int64, clientIP string) ([]identityaccess.AccountTenantAccess, error)
	revokeFn func(ctx context.Context, accountID, schoolID, operatorID int64, clientIP string) ([]identityaccess.AccountTenantAccess, error)
}

func (m *mockAccountAccess) ListAccountTenantAccess(ctx context.Context, accountID int64) ([]identityaccess.AccountTenantAccess, error) {
	if m.listFn != nil {
		return m.listFn(ctx, accountID)
	}
	return nil, nil
}

func (m *mockAccountAccess) ListAssignableSchoolRoles(context.Context, int64) ([]identityaccess.AccountTenantRole, error) {
	return nil, nil
}

func (m *mockAccountAccess) GrantAccountTenantAccess(ctx context.Context, request identityaccess.GrantAccountTenantAccess) ([]identityaccess.AccountTenantAccess, error) {
	if m.grantFn != nil {
		return m.grantFn(ctx, request)
	}
	return nil, nil
}

func (m *mockAccountAccess) UpdateAccountTenantRole(ctx context.Context, accountID, schoolID, roleID, operatorID int64, clientIP string) ([]identityaccess.AccountTenantAccess, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, accountID, schoolID, roleID, operatorID, clientIP)
	}
	return nil, nil
}

func (m *mockAccountAccess) RevokeAccountTenantAccess(ctx context.Context, accountID, schoolID, operatorID int64, clientIP string) ([]identityaccess.AccountTenantAccess, error) {
	if m.revokeFn != nil {
		return m.revokeFn(ctx, accountID, schoolID, operatorID, clientIP)
	}
	return nil, nil
}

func newAccountAccessResource(access *mockAccountAccess) *identityoperator.Resource {
	return identityoperator.NewResource(access, IdentityResponses())
}

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
	t.Parallel()

	resource := newAccountAccessResource(&mockAccountAccess{
		listFn: func(_ context.Context, accountID int64) ([]identityaccess.AccountTenantAccess, error) {
			assert.Equal(t, int64(7), accountID)
			return []identityaccess.AccountTenantAccess{{
				Roles: []identityaccess.AccountTenantRole{{ID: 9007199254740993, Name: "admin", IsSystem: true}},
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
	assert.Equal(t, "9007199254740993", roles[0].(map[string]any)["id"])
	assert.Equal(t, true, roles[0].(map[string]any)["is_system"])
}

func TestProvisioningResource_ListAccountTenantAccess_InvalidID(t *testing.T) {
	t.Parallel()

	resource := newAccountAccessResource(&mockAccountAccess{})

	rr := httptest.NewRecorder()
	resource.ListAccountTenantAccess(rr, accountTenantRequest(t, http.MethodGet, "", map[string]string{"accountId": "nope"}))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_GrantAccountTenantAccess(t *testing.T) {
	t.Parallel()

	resource := newAccountAccessResource(&mockAccountAccess{
		grantFn: func(_ context.Context, request identityaccess.GrantAccountTenantAccess) ([]identityaccess.AccountTenantAccess, error) {
			assert.Equal(t, int64(7), request.AccountID)
			assert.Equal(t, int64(12), request.SchoolID)
			assert.Equal(t, int64(9007199254740993), request.RoleID)
			assert.Equal(t, "Erika", request.FirstName)
			assert.Equal(t, int64(42), request.OperatorID)
			assert.Equal(t, "203.0.113.7", request.ClientIP)
			return []identityaccess.AccountTenantAccess{}, nil
		},
	})

	rr := httptest.NewRecorder()
	resource.GrantAccountTenantAccess(rr, accountTenantRequest(t, http.MethodPost,
		`{"school_id":12,"role_id":"9007199254740993","first_name":"  Erika  ","last_name":"Muster"}`,
		map[string]string{"accountId": "7"}))

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
}

func TestProvisioningResource_GrantAccountTenantAccess_RequiresSchoolAndRole(t *testing.T) {
	t.Parallel()

	resource := newAccountAccessResource(&mockAccountAccess{
		grantFn: func(context.Context, identityaccess.GrantAccountTenantAccess) ([]identityaccess.AccountTenantAccess, error) {
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
	t.Parallel()

	resource := newAccountAccessResource(&mockAccountAccess{
		grantFn: func(context.Context, identityaccess.GrantAccountTenantAccess) ([]identityaccess.AccountTenantAccess, error) {
			return nil, identityaccess.ErrAccountTenantAccessExists
		},
	})

	rr := httptest.NewRecorder()
	resource.GrantAccountTenantAccess(rr, accountTenantRequest(t, http.MethodPost,
		`{"school_id":12,"role_id":3}`, map[string]string{"accountId": "7"}))

	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestProvisioningResource_GrantAccountTenantAccess_InvalidRoleMessageSurvives(t *testing.T) {
	t.Parallel()

	resource := newAccountAccessResource(&mockAccountAccess{
		grantFn: func(context.Context, identityaccess.GrantAccountTenantAccess) ([]identityaccess.AccountTenantAccess, error) {
			return nil, &identityaccess.InvalidInputError{Err: assert.AnError}
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
	t.Parallel()

	resource := newAccountAccessResource(&mockAccountAccess{
		updateFn: func(_ context.Context, accountID, schoolID, roleID, operatorID int64, _ string) ([]identityaccess.AccountTenantAccess, error) {
			assert.Equal(t, int64(7), accountID)
			assert.Equal(t, int64(12), schoolID)
			assert.Equal(t, int64(9007199254740993), roleID)
			assert.Equal(t, int64(42), operatorID)
			return []identityaccess.AccountTenantAccess{}, nil
		},
	})

	rr := httptest.NewRecorder()
	resource.UpdateAccountTenantRole(rr, accountTenantRequest(t, http.MethodPut, `{"role_id":"9007199254740993"}`,
		map[string]string{"accountId": "7", "tenantId": "12"}))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestProvisioningResource_UpdateAccountTenantRole_RequiresRole(t *testing.T) {
	t.Parallel()

	resource := newAccountAccessResource(&mockAccountAccess{})

	rr := httptest.NewRecorder()
	resource.UpdateAccountTenantRole(rr, accountTenantRequest(t, http.MethodPut, `{}`,
		map[string]string{"accountId": "7", "tenantId": "12"}))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProvisioningResource_RevokeAccountTenantAccess(t *testing.T) {
	t.Parallel()

	called := false
	resource := newAccountAccessResource(&mockAccountAccess{
		revokeFn: func(_ context.Context, accountID, schoolID, operatorID int64, _ string) ([]identityaccess.AccountTenantAccess, error) {
			called = true
			assert.Equal(t, int64(7), accountID)
			assert.Equal(t, int64(12), schoolID)
			assert.Equal(t, int64(42), operatorID)
			return []identityaccess.AccountTenantAccess{}, nil
		},
	})

	rr := httptest.NewRecorder()
	resource.RevokeAccountTenantAccess(rr, accountTenantRequest(t, http.MethodDelete, "",
		map[string]string{"accountId": "7", "tenantId": "12"}))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.True(t, called)
}

func TestProvisioningResource_RevokeAccountTenantAccess_UnknownMapping(t *testing.T) {
	t.Parallel()

	resource := newAccountAccessResource(&mockAccountAccess{
		revokeFn: func(context.Context, int64, int64, int64, string) ([]identityaccess.AccountTenantAccess, error) {
			return nil, identityaccess.ErrAccountTenantAccessNotFound
		},
	})

	rr := httptest.NewRecorder()
	resource.RevokeAccountTenantAccess(rr, accountTenantRequest(t, http.MethodDelete, "",
		map[string]string{"accountId": "7", "tenantId": "12"}))

	assert.Equal(t, http.StatusNotFound, rr.Code)
}
