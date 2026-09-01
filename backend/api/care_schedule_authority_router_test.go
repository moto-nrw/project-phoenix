package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCareScheduleAuthorityHTTPFlow(t *testing.T) {
	t.Parallel()
	apiInstance := newGoldenAPI(t)
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	_, reviewer := testpkg.CreateTestStaffWithAccount(t, db, "Rita", "Review")
	parentToken := careScheduleParentToken(t, chain)
	staffToken := testutil.MintTestJWT(t, testutil.AdminTestClaimsForTenant(int(reviewer.ID), chain.TenantID))
	path := "/parent/me/children/" + strconv.FormatInt(chain.StudentID, 10) + "/care-schedule/requests"
	setAuthority := func(authoritative bool) {
		_, err := db.ExecContext(t.Context(), `
			INSERT INTO config.setting_values (tenant_id, setting_key, value)
			VALUES (?, ?, to_jsonb(?::boolean)), (?, ?, 'true'::jsonb), (?, ?, 'true'::jsonb)
			ON CONFLICT (tenant_id, setting_key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`,
			chain.TenantID, configModels.KeyEnrollmentBookingsAuthoritative, authoritative,
			chain.TenantID, configModels.KeyParentCarePickupRequestEnabled,
			chain.TenantID, configModels.KeyParentCareModeRequestEnabled,
		)
		require.NoError(t, err)
	}

	setAuthority(true)
	rejected := doCareScheduleJSON(t, apiInstance.Router, http.MethodPost, path, parentToken, careScheduleRequestBody())
	require.Equal(t, http.StatusForbidden, rejected.Code, rejected.Body.String())
	assert.Contains(t, rejected.Body.String(), `"code":"care_request_bookings_authoritative"`)

	setAuthority(false)
	created := doCareScheduleJSON(t, apiInstance.Router, http.MethodPost, path, parentToken, careScheduleRequestBody())
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	requestID := careSchedulePendingRequestID(t, created)

	decisionPath := "/api/students/care-schedule-change-requests/" + strconv.FormatInt(requestID, 10) + "/decide"
	// #2267: reason policy defaults to "both", so a staff approval carries a reason.
	decided := doCareScheduleJSON(t, apiInstance.Router, http.MethodPost, decisionPath, staffToken, map[string]any{"approve": true, "reason": "Passt so"})
	require.Equal(t, http.StatusOK, decided.Code, decided.Body.String())
	assert.Contains(t, decided.Body.String(), `"status":"approved"`)
}

func careScheduleParentToken(t *testing.T, chain testpkg.ParentChain) string {
	t.Helper()
	// Parent-scope principals are not tenant-bound; the security principal
	// seam rejects a parent token that carries a tenant (#2645).
	return testutil.MintTestJWT(t, jwt.AppClaims{
		ID: int(chain.AccountID), Sub: chain.Email, Roles: []string{"guardian"},
		Scope: tenant.ScopeParent,
	})
}

func careScheduleRequestBody() map[string]any {
	return map[string]any{"payload": map[string]any{"weekdays": []any{map[string]any{
		"weekday": 2, "scheduled": true, "pickup": "15:30", "mode": "pickup",
	}}}}
}

func doCareScheduleJSON(t *testing.T, handler http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func careSchedulePendingRequestID(t *testing.T, recorder *httptest.ResponseRecorder) int64 {
	t.Helper()
	var response struct {
		Data struct {
			PendingRequest struct {
				ID string `json:"id"`
			} `json:"pending_request"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	id, err := strconv.ParseInt(response.Data.PendingRequest.ID, 10, 64)
	require.NoError(t, err)
	return id
}
