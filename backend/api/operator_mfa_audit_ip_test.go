package api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	identityAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorMFAAuditsPersistRequestIP(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	_, identity := buildOperatorProvisioningWithIdentity(t, db)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	account := testpkg.CreateTestAccount(t, db, "audit-request-ip")
	schoolID := testpkg.Tenant(t)
	testpkg.EnsureAccountTenant(t, db, account.ID, schoolID)
	require.NoError(t, identity.EnrollMFA(ctx, account.ID))
	resource := &identityAPI.SchoolAccountMFAResource{TenantMFAService: identity}
	router := chi.NewRouter()
	router.Use(chimiddleware.ClientIPFromXFFTrustedProxies(1))
	router.Put("/schools/{id}/accounts/{accountId}/mfa/override", resource.SetSchoolAccountMFAOverride)
	router.Delete("/schools/{id}/accounts/{accountId}/mfa", resource.ResetSchoolAccountMFA)
	router.Put("/accounts/{accountId}/mfa/global-override", resource.SetAccountMFAGlobalOverride)
	for _, tc := range []struct{ action, method, path, forwardedFor, wantIP string }{
		{"set_override", http.MethodPut, fmt.Sprintf("/schools/%d/accounts/%d/mfa/override", schoolID, account.ID), "", "198.51.100.43"},
		{"set_global_override", http.MethodPut, fmt.Sprintf("/accounts/%d/mfa/global-override", account.ID), "203.0.113.99, 2001:db8::43", "2001:db8::43"},
		{"disable", http.MethodDelete, fmt.Sprintf("/schools/%d/accounts/%d/mfa", schoolID, account.ID), "", "198.51.100.43"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(`{"override":"force_on","reason":"test transition"}`))
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = "198.51.100.43:12345"
			if tc.forwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tc.forwardedFor)
			}
			req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			require.Equal(t, http.StatusNoContent, response.Code)
			var storedIP string
			require.NoError(t, db.NewRaw(`SELECT COALESCE(host(request_ip), '') FROM platform.operator_audit_log
				WHERE operator_id = ? AND resource_id = ? AND action = 'mfa_admin_override' AND changes->>'action' = ?`,
				operatorID, account.ID, tc.action).Scan(ctx, &storedIP))
			assert.Equal(t, tc.wantIP, storedIP)
		})
	}
}
