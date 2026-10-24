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
	settingsAPI "github.com/moto-nrw/project-phoenix/modules/settings/inbound/operator"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorAuditBookingAuthorityImpact(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	student := testpkg.CreateTestStudent(t, db, "Audit", "Fixture", "2a")
	module, lifecycle, err := NewOperatorSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: buildOperatorProvisioning(t, db)})
	router := chi.NewRouter()
	mountOperatorSchoolRoutes(router, resource, nil, &operatorSchoolSettings{
		values: settingsAPI.NewSettingsResource(settingsAPI.SettingsConfig{
			Settings: module.OperatorSchoolSettings(module.SettingsSideEffects.Dispatch), OperatorID: operatorIDFromSession,
		}),
		bookingAuthorityImpact: newBookingAuthorityImpact(lifecycle),
	})
	request := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/schools/%d/settings/booking-authority-impact?search=private-filter", testpkg.Tenant(t)), nil)
		req.RemoteAddr = "198.51.100.46:12345"
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	response := request()
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), fmt.Sprintf(`"student_id":"%d"`, student.ID))
	var entries []struct {
		ResourceType string
		ResourceID   *int64
		RequestIP    string
		Changes      json.RawMessage
	}
	require.NoError(t, db.NewRaw(`SELECT resource_type, resource_id, host(request_ip) AS request_ip, changes
		FROM platform.operator_audit_log WHERE operator_id = ? AND action = 'view'`, operatorID).Scan(ctx, &entries))
	if assert.Len(t, entries, 1, "one audit entry must precede disclosure") {
		assert.Equal(t, "school", entries[0].ResourceType)
		require.NotNil(t, entries[0].ResourceID)
		assert.Equal(t, testpkg.Tenant(t), *entries[0].ResourceID)
		assert.Equal(t, "198.51.100.46", entries[0].RequestIP)
		assert.JSONEq(t, fmt.Sprintf(`{"view":"school_booking_authority_impact","school_id":%d}`, testpkg.Tenant(t)), string(entries[0].Changes))
	}
	_, err = db.ExecContext(ctx, `CREATE FUNCTION platform.reject_operator_audit() RETURNS trigger LANGUAGE plpgsql AS $$
	BEGIN RAISE EXCEPTION 'injected audit failure'; END $$;
	CREATE TRIGGER reject_operator_audit BEFORE INSERT ON platform.operator_audit_log
	FOR EACH ROW EXECUTE FUNCTION platform.reject_operator_audit()`)
	require.NoError(t, err)
	response = request()
	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	assert.Contains(t, response.Body.String(), `"code":"operator.audit_unavailable"`)
	for _, private := range []string{"student_id", "first_name", "last_name", "school_class", "blocking_children", "planned_completions", "private-filter", "injected audit failure"} {
		assert.NotContains(t, response.Body.String(), private)
	}
}
