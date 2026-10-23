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
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	accountHTTP "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	"github.com/moto-nrw/project-phoenix/modules/schoolportal"
	settings "github.com/moto-nrw/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A valid code must buy a session even when remembering the device is
// unavailable. Drive real settings, challenge consumption and session issuance
// through both portal routers, including the root's school-scope binding.
func TestMFASecurityPolicyFailuresBothPortals(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{identityaccess.MFAChallengeScopeTenant, identityaccess.MFAChallengeScopeSchool} {
		for _, key := range []string{settings.KeyMFATrustedDeviceEnabled, settings.KeyMFATrustedDeviceDays, settings.KeyAccountLockoutThreshold, settings.KeyAccountLockoutDurationMinutes} {
			t.Run(scope+"/"+key, func(t *testing.T) {
				testpkg.OwnTenant(t)
				db, school, _, _ := setupSchoolRoute(t)
				tenantID := testpkg.Tenant(t)
				_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Policy", "Failure")
				if scope == identityaccess.MFAChallengeScopeSchool {
					testpkg.AssignLehrkraftSystemRole(t, db, account.ID, tenantID)
				}
				mail := testpkg.NewCapturingMailer()
				module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), WithAuthTestMailer(mail))
				require.NoError(t, err)
				token, err := module.MFA.StartMFAChallenge(context.Background(), account.ID, tenantID, scope, nil)
				require.NoError(t, err)
				messages := mail.Messages()
				require.Len(t, messages, 1)
				content, err := json.Marshal(messages[0].Content)
				require.NoError(t, err)
				var code struct{ Code string }
				require.NoError(t, json.Unmarshal(content, &code))
				require.NotEmpty(t, code.Code)

				// A malformed stored override makes the typed resolver fail,
				// without breaking the independent lockout policy or sessions.
				_, err = db.NewRaw(`INSERT INTO config.setting_values (tenant_id, setting_key, value)
					VALUES (?, ?, '"unreadable-policy"'::jsonb)`, tenantID, key).Exec(context.Background())
				require.NoError(t, err)
				var router http.Handler
				if scope == identityaccess.MFAChallengeScopeTenant {
					resource := &accountHTTP.Resource{Sessions: module.Auth, MFAService: module.MFA}
					root := chi.NewRouter()
					root.Mount("/auth", resource.Router(TenantSessionChain))
					router = root
				} else {
					school.AuthService = SchoolPortalAuth(module.Auth)
					school.MFAService = SchoolPortalMFA(module.MFA)
					router = newSchoolRouter(school, school.MFAService)
				}
				body := fmt.Sprintf(`{"challenge_token":%q,"code":%q,"remember_device":true}`, token, code.Code)
				verify := func() *httptest.ResponseRecorder {
					r := httptest.NewRequest(http.MethodPost, "/auth/mfa/verify", strings.NewReader(body))
					r.Header.Set("Content-Type", "application/json")
					recorder := httptest.NewRecorder()
					router.ServeHTTP(recorder, r)
					return recorder
				}
				response := verify()
				lockoutFailure := key == settings.KeyAccountLockoutThreshold || key == settings.KeyAccountLockoutDurationMinutes
				if lockoutFailure {
					require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
					assert.Contains(t, response.Body.String(), `"code":"general.unavailable"`)
					assert.NotContains(t, response.Body.String(), "access_token")
					assert.NotContains(t, response.Body.String(), "refresh_token")
					assert.Empty(t, response.Result().Cookies())
					reader := NewAuthTestSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
					require.NoError(t, reader.ResetValue(testpkg.Ctx(t), key, nil, nil))
					// Recovery must accept the very same code and challenge.
					response = verify()
				}
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				var tokens schoolportal.TokenResponse
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &tokens))
				assert.NotEmpty(t, tokens.AccessToken)
				assert.NotEmpty(t, tokens.RefreshToken)
				if lockoutFailure {
					require.Len(t, response.Result().Cookies(), 1)
				} else {
					assert.Empty(t, response.Result().Cookies())
				}
				devices, err := module.MFA.ListTrustedDevices(context.Background(), account.ID, tenantID)
				require.NoError(t, err)
				if lockoutFailure {
					assert.Len(t, devices, 1)
				} else {
					assert.Empty(t, devices)
				}
				// The successful login, not a failed optional policy read,
				// consumed the challenge. Replay retains the existing code.
				replayed := verify()
				assert.Equal(t, http.StatusUnauthorized, replayed.Code, replayed.Body.String())
				assert.Contains(t, replayed.Body.String(), `"code":"identity.mfa_code_invalid"`)
			})
		}
	}
}
