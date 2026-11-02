package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	provisioningAPI "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorPersonAndAccountRoutesAuditOneAttemptPerRequest(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service := buildOperatorProvisioning(t, db)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, schoolID := provisionTestSchool(t, db, service, operatorID, "audit-reads")
	resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service})
	router := chi.NewRouter()
	mountOperatorProvisioningRoutes(router, resource, nil)
	mountOperatorSchoolRoutes(router, resource, nil, nil, nil)
	cases := []struct {
		path, view, resource string
		scope                map[string]any
	}{
		{"/accounts", "accounts", "account", nil},
		{fmt.Sprintf("/schools/%d/accounts", schoolID), "school_accounts", "account", map[string]any{"school_id": float64(schoolID)}},
		{fmt.Sprintf("/schools/%d/persons", schoolID), "school_persons", "person", map[string]any{"school_id": float64(schoolID)}},
		{fmt.Sprintf("/organizations/%d/accounts", organizationID), "organization_accounts", "account", map[string]any{"organization_id": float64(organizationID)}},
		{fmt.Sprintf("/organizations/%d/persons", organizationID), "organization_persons", "person", map[string]any{"organization_id": float64(organizationID)}},
	}
	request := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path+"?search=untrusted-filter", nil)
		req.RemoteAddr = "198.51.100.44:12345"
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	for _, tc := range cases {
		t.Run(tc.view, func(t *testing.T) {
			assert.Equal(t, http.StatusOK, request(tc.path).Code)
			var entries []struct {
				ResourceType string
				RequestIP    string
				Changes      json.RawMessage
			}
			err := db.NewRaw(`SELECT resource_type, COALESCE(host(request_ip), '') AS request_ip, changes FROM platform.operator_audit_log
			WHERE operator_id = ? AND action = 'view' AND changes->>'view' = ?`, operatorID, tc.view).Scan(ctx, &entries)
			require.NoError(t, err)
			require.Len(t, entries, 1)
			assert.Equal(t, tc.resource, entries[0].ResourceType)
			assert.Equal(t, "198.51.100.44", entries[0].RequestIP)
			expected := map[string]any{"view": tc.view}
			for key, value := range tc.scope {
				expected[key] = value
			}
			var metadata map[string]any
			require.NoError(t, json.Unmarshal(entries[0].Changes, &metadata))
			assert.Equal(t, expected, metadata, "only fixed categories and IDs may be stored")
		})
	}
	_, err := db.ExecContext(ctx, `CREATE FUNCTION platform.reject_operator_audit() RETURNS trigger LANGUAGE plpgsql AS $$
	BEGIN RAISE EXCEPTION 'injected audit failure'; END $$;
	CREATE TRIGGER reject_operator_audit BEFORE INSERT ON platform.operator_audit_log
	FOR EACH ROW EXECUTE FUNCTION platform.reject_operator_audit()`)
	require.NoError(t, err)
	for _, tc := range cases {
		response := request(tc.path)
		assert.Equal(t, http.StatusServiceUnavailable, response.Code)
		assert.Contains(t, response.Body.String(), `"code":"operator.audit_unavailable"`)
		assert.NotContains(t, response.Body.String(), "untrusted-filter")
	}
}
