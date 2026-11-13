package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/modules/settings"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCareScheduleAuthorityHTTPFlow(t *testing.T) {
	t.Parallel()
	db, resource := setupCareScheduleRouter(t)
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/", resource)
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
			chain.TenantID, settings.KeyEnrollmentBookingsAuthoritative, authoritative,
			chain.TenantID, settings.KeyParentCarePickupRequestEnabled,
			chain.TenantID, settings.KeyParentCareModeRequestEnabled,
		)
		require.NoError(t, err)
	}

	setAuthority(true)
	rejected := doCareScheduleJSON(t, router, http.MethodPost, path, parentToken, careScheduleRequestBody())
	require.Equal(t, http.StatusForbidden, rejected.Code, rejected.Body.String())
	assert.Contains(t, rejected.Body.String(), `"code":"care.care_request_bookings_authoritative"`)

	setAuthority(false)
	created := doCareScheduleJSON(t, router, http.MethodPost, path, parentToken, careScheduleRequestBody())
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	requestID := careSchedulePendingRequestID(t, created)

	decisionPath := "/api/students/care-schedule-change-requests/" + strconv.FormatInt(requestID, 10) + "/decide"
	// #2267: reason policy defaults to "both", so a staff approval carries a reason.
	// #6843: an approval names the diff the reviewer saw; the decision path
	// with that token is covered by careplan/http TestWeeklyApprovalRefusesPlanChangedSinceReview.
	decided := doCareScheduleJSON(t, router, http.MethodPost, decisionPath, staffToken, map[string]any{"approve": true, "reason": "Passt so"})
	require.Equal(t, http.StatusBadRequest, decided.Code, decided.Body.String())
	rejected = doCareScheduleJSON(t, router, http.MethodPost, decisionPath, staffToken, map[string]any{"approve": false, "reason": "Passt nicht"})
	require.Equal(t, http.StatusOK, rejected.Code, rejected.Body.String())
	assert.Contains(t, rejected.Body.String(), `"status":"rejected"`)
}

func careScheduleParentToken(t *testing.T, chain testpkg.ParentChain) string {
	t.Helper()
	// Parent-scope principals are not tenant-bound; the security principal
	// seam rejects a parent token that carries a tenant (#2645).
	return testutil.MintTestJWT(t, sessiontoken.AppClaims{
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

// careScheduleImpactToken reads the impact token the open queue shows for one
// pending care request (#6843).
func careScheduleImpactToken(t *testing.T, handler http.Handler, token string, studentID, requestID int64) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/students/change-requests?view=open&types=care_schedule&student_id="+strconv.FormatInt(studentID, 10), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response struct {
		Data struct {
			Items []struct {
				Data struct {
					ID          string `json:"id"`
					ImpactToken string `json:"impact_token"`
				} `json:"data"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	for _, item := range response.Data.Items {
		if item.Data.ID == strconv.FormatInt(requestID, 10) {
			require.NotEmpty(t, item.Data.ImpactToken)
			return item.Data.ImpactToken
		}
	}
	t.Fatalf("care request %d missing from the open queue", requestID)
	return ""
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
