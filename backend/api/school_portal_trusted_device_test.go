// Router-level tests for the own remembered devices of the school portal
// (#6955): a Lehrkraft lists and revokes the devices of the account at the
// school of the session, behind the school session only.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/modules/schoolportal"
	settings "github.com/moto-ogs/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

const schoolTrustedDevicesPath = "/school/auth/mfa/trusted-devices"

// schoolDeviceRequest sends a request to the school router with an optional
// bearer token, remember-device cookie and JSON body.
func schoolDeviceRequest(t *testing.T, router http.Handler, method, path, bearer, cookie string, body any, counter *testpkg.QueryCounter) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
	}
	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: common.TrustedDeviceCookie, Value: cookie})
	}
	if counter != nil {
		req = req.WithContext(counter.Context(req.Context()))
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestSchoolPortalListsAndRevokesItsRememberedDevices(t *testing.T) {
	t.Parallel()
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.Tenant(t)
	password := testpkg.NewTestPassword()
	account := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("lehrkraft-%d@test.local", testpkg.UniqueTestTenantID(t)), password)
	testpkg.AssignLehrkraftSystemRole(t, db, account.ID, tenantID)
	_, err := db.NewRaw(`INSERT INTO config.setting_values (tenant_id, setting_key, value) VALUES (?, ?, '"required_all"'::jsonb)`,
		tenantID, settings.KeyMFAMode).Exec(ctx)
	require.NoError(t, err)
	mail := testpkg.NewCapturingMailer()
	module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), WithAuthTestMailer(mail))
	require.NoError(t, err)
	require.NoError(t, enrollMFA(ctx, t, db, module.MFA, account.ID))
	resource := schoolportal.NewResource(SchoolPortalAuth(module.Auth), SchoolPortalMFA(module.MFA), common.PasswordResetRoutes{},
		emptySchoolMessagingRouter{}, nil, nil, nil, nil, nil)
	mounted := chi.NewRouter()
	mounted.Mount("/school", resource.Router(SchoolSessionChain, ProtectedSchoolRoutes))
	router := testpkg.SessionVerifier(mounted)
	credentials := map[string]any{"email": account.Email, "password": password}
	login := func(cookie string) map[string]any {
		rec := schoolDeviceRequest(t, router, http.MethodPost, "/school/auth/login", "", cookie, credentials, nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var answer map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer))
		return answer
	}
	code := func() string {
		messages := mail.Messages()
		require.NotEmpty(t, messages)
		content, err := json.Marshal(messages[len(messages)-1].Content)
		require.NoError(t, err)
		var sent struct{ Code string }
		require.NoError(t, json.Unmarshal(content, &sent))
		return sent.Code
	}

	challenge := login("")
	require.Equal(t, "mfa_required", challenge["status"])
	rec := schoolDeviceRequest(t, router, http.MethodPost, "/school/auth/mfa/verify", "", "", map[string]any{
		"challenge_token": challenge["challenge_token"], "code": code(), "remember_device": true,
	}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	cookie := traegerDeviceCookie(rec)
	require.NotEmpty(t, cookie)
	var session struct {
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &session))
	assert.Equal(t, "authenticated", login(cookie)["status"], "the remembered device replaces the code")

	counter := testpkg.CaptureQueriesForContext(t, db)
	rec = schoolDeviceRequest(t, router, http.MethodGet, schoolTrustedDevicesPath, session.AccessToken, "", nil, counter)
	counter.Stop()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	querybudgets.AssertQueryBudget(t, "api.school.auth.mfa.trusted_devices", counter.Measurement(counter.Queries()))
	var devices []common.TrustedDeviceDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &devices))
	require.Len(t, devices, 1, "the device of the account at the session's school")

	for name, token := range map[string]string{
		"tenant":  testpkg.MintTestJWT(t, sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email, TenantID: tenantID}),
		"Träger":  testpkg.MintTestJWT(t, sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email, Scope: "org", OrgID: 1}),
		"caterer": testpkg.MintTestJWT(t, sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email, Scope: "caterer"}),
	} {
		rec = schoolDeviceRequest(t, router, http.MethodGet, schoolTrustedDevicesPath, token, "", nil, nil)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "a %s token lists no school devices", name)
		rec = schoolDeviceRequest(t, router, http.MethodDelete, fmt.Sprintf("%s/%d", schoolTrustedDevicesPath, devices[0].ID), token, "", nil, nil)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "a %s token revokes no school device", name)
	}
	rec = schoolDeviceRequest(t, router, http.MethodDelete, schoolTrustedDevicesPath+"/abc", session.AccessToken, "", nil, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	rec = schoolDeviceRequest(t, router, http.MethodDelete, fmt.Sprintf("%s/%d", schoolTrustedDevicesPath, devices[0].ID+1_000_000), session.AccessToken, "", nil, nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, "a foreign id answers like the tenant route: %s", rec.Body.String())

	rec = schoolDeviceRequest(t, router, http.MethodDelete, fmt.Sprintf("%s/%d", schoolTrustedDevicesPath, devices[0].ID), session.AccessToken, "", nil, nil)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	rec = schoolDeviceRequest(t, router, http.MethodGet, schoolTrustedDevicesPath, session.AccessToken, "", nil, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, "[]", rec.Body.String())
	assert.Equal(t, "mfa_required", login(cookie)["status"], "a revoked device asks for the code again")
}
