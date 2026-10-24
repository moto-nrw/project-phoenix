package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	identityAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	provisioningAPI "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	settingsAPI "github.com/moto-nrw/project-phoenix/modules/settings/inbound/operator"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorAuditSensitiveResponseRoutesFailClosed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := provisioningContext(t, db)
	service, identity := buildOperatorProvisioningWithIdentity(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	account := testpkg.CreateTestAccount(t, db, "audit-sensitive-response")
	organizationID, schoolID := provisionTestSchool(t, db, service, operatorID, "audit-response")
	module, lifecycle, err := NewOperatorSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service})
	router := chi.NewRouter()
	mountOperatorProvisioningRoutes(router, resource)
	mountOperatorSchoolRoutes(router, resource, nil, &operatorSchoolSettings{
		values: settingsAPI.NewSettingsResource(settingsAPI.SettingsConfig{
			Settings: module.OperatorSchoolSettings(module.SettingsSideEffects.Dispatch), OperatorID: operatorIDFromSession,
		}),
		bookingAuthorityImpact: newBookingAuthorityImpact(lifecycle),
	})
	(operatorIdentityRoutes{account: identityAPI.NewResource(identity, operatorIdentityResponses())}).mountAccountTenantAccessRoutes(router, resource)
	// The mounted scan reader/writer must never be invoked if its audit fails.
	scanCalls := 0
	scans := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		scanCalls++
		w.WriteHeader(http.StatusOK)
	})
	mountOperatorTagScanRoutes(router, scans, resource)
	school := fmt.Sprintf("/schools/%d", schoolID)
	access := fmt.Sprintf("/accounts/%d/tenants", account.ID)
	request := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path+"?search=private-filter", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "198.51.100.47:12345"
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	for _, tc := range []struct{ path, view string }{
		{"/schools/", "schools"},
		{"/schools/summaries", "school_summaries"},
		{fmt.Sprintf("/organizations/%d/schools", organizationID), "organization_schools"},
		{school + "/settings/schema", "school_settings_schema"},
		{school + "/settings/values/" + operatorBookingsAuthoritativeKey + "/reveal", "school_setting_reveal"},
		{"/unregistered-tag-scans/", "unregistered_tag_scans"},
	} {
		require.Equal(t, http.StatusOK, request(http.MethodGet, tc.path).Code)
		var entries []struct {
			RequestIP string
			Changes   json.RawMessage
		}
		require.NoError(t, db.NewRaw(`SELECT host(request_ip) AS request_ip, changes
			FROM platform.operator_audit_log WHERE operator_id = ? AND action = 'view' AND changes->>'view' = ?`, operatorID, tc.view).Scan(ctx, &entries))
		require.Len(t, entries, 1)
		assert.Equal(t, "198.51.100.47", entries[0].RequestIP)
		expected := map[string]any{"view": tc.view}
		switch tc.view {
		case "organization_schools":
			expected["organization_id"] = float64(organizationID)
		case "school_settings_schema", "school_setting_reveal":
			expected["school_id"] = float64(schoolID)
		}
		var changes map[string]any
		require.NoError(t, json.Unmarshal(entries[0].Changes, &changes))
		assert.Equal(t, expected, changes, "only fixed categories and IDs, never filters, setting values or scan contents")
	}
	require.Equal(t, 1, scanCalls)
	scanCalls = 0
	_, err = db.ExecContext(ctx, `CREATE FUNCTION platform.reject_operator_audit() RETURNS trigger LANGUAGE plpgsql AS $$
	BEGIN RAISE EXCEPTION 'injected audit failure'; END $$;
	CREATE TRIGGER reject_operator_audit BEFORE INSERT ON platform.operator_audit_log
	FOR EACH ROW EXECUTE FUNCTION platform.reject_operator_audit()`)
	require.NoError(t, err)
	cases := []struct{ method, path string }{
		{http.MethodGet, "/schools/"},
		{http.MethodGet, "/schools/summaries"},
		{http.MethodGet, fmt.Sprintf("/organizations/%d/schools", organizationID)},
		{http.MethodPost, "/schools/"},
		{http.MethodPut, school},
		{http.MethodPost, school + "/invite-admin"},
		{http.MethodPost, school + "/create-account"},
		{http.MethodPost, school + fmt.Sprintf("/accounts/%d/caregiver-capability/", account.ID)},
		{http.MethodDelete, school + fmt.Sprintf("/accounts/%d/caregiver-capability/", account.ID)},
		{http.MethodPost, access},
		{http.MethodPost, access + "/"},
		{http.MethodPut, access + fmt.Sprintf("/%d", schoolID)},
		{http.MethodDelete, access + fmt.Sprintf("/%d", schoolID)},
		{http.MethodGet, school + "/settings/schema"},
		{http.MethodGet, school + "/settings/values/sftp.password/reveal"},
		{http.MethodGet, "/unregistered-tag-scans/"},
		{http.MethodPost, fmt.Sprintf("/unregistered-tag-scans/%d/resolve", account.ID)},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			response := request(tc.method, tc.path)
			assert.Equal(t, http.StatusServiceUnavailable, response.Code)
			assert.Contains(t, response.Body.String(), `"code":"operator.audit_unavailable"`)
			for _, private := range []string{`"data"`, "private-filter", "injected audit failure"} {
				assert.NotContains(t, response.Body.String(), private)
			}
		})
	}
	assert.Zero(t, scanCalls, "no scan read or mutation is permitted without its audit")
}
