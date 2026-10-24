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
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type resetAfterVerification struct {
	identityaccess.AccountMFA
	reset func() error
}

func (m resetAfterVerification) VerifyMFAChallenge(ctx context.Context, token, code string) (identityaccess.VerifiedMFAChallenge, error) {
	return m.VerifyMFAChallengeForScope(ctx, token, code, identityaccess.MFAChallengeScopeTenant)
}

func (m resetAfterVerification) VerifyMFAChallengeForScope(ctx context.Context, token, code, scope string) (identityaccess.VerifiedMFAChallenge, error) {
	verified, err := m.AccountMFA.VerifyMFAChallengeForScope(ctx, token, code, scope)
	if err == nil {
		// The verifier has completed; a concurrent reset commits before mint.
		done := make(chan error, 1)
		go func() { done <- m.reset() }()
		err = <-done
	}
	return verified, err
}

type resetAfterMint struct {
	identityaccess.Owner
	reset func() error
}

func (s resetAfterMint) IssueTokensForVerifiedMFA(ctx context.Context, proof identityaccess.VerifiedMFAChallenge, ip, ua string) (string, string, error) {
	access, refresh, err := s.Owner.IssueTokensForVerifiedMFA(ctx, proof, ip, ua)
	if err == nil {
		err = s.reset()
	}
	return access, refresh, err
}
func (s resetAfterMint) IssueSchoolTokensForVerifiedMFA(ctx context.Context, proof identityaccess.VerifiedMFAChallenge, ip, ua string) (string, string, error) {
	access, refresh, err := s.Owner.IssueSchoolTokensForVerifiedMFA(ctx, proof, ip, ua)
	if err == nil {
		err = s.reset()
	}
	return access, refresh, err
}

func TestMFAResetBetweenVerificationAndExchangeRefusesBothPortals(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"before_verification", "before_mint", "after_mint"} {
		afterMint := phase == "after_mint"
		for _, scope := range []string{identityaccess.MFAChallengeScopeTenant, identityaccess.MFAChallengeScopeSchool} {
			t.Run(scope+"/reset_"+phase, func(t *testing.T) {
				testpkg.OwnTenant(t)
				db, school, _, _ := setupSchoolRoute(t)
				tenantID := testpkg.Tenant(t)
				_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Reset", "Race")
				if scope == identityaccess.MFAChallengeScopeSchool {
					testpkg.AssignLehrkraftSystemRole(t, db, account.ID, tenantID)
				}
				mail := testpkg.NewCapturingMailer()
				module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), WithAuthTestMailer(mail))
				require.NoError(t, err)
				require.NoError(t, module.MFA.EnrollMFA(t.Context(), account.ID))
				token, err := module.MFA.StartMFAChallenge(t.Context(), account.ID, tenantID, scope, nil)
				require.NoError(t, err)
				messages := mail.Messages()
				require.Equal(t, 1, len(messages))
				content, err := json.Marshal(messages[0].Content)
				require.NoError(t, err)
				var code struct{ Code string }
				require.NoError(t, json.Unmarshal(content, &code))
				reset := func() error {
					if err := module.MFA.DisableMFA(testpkg.Ctx(t), account.ID); err != nil {
						return err
					}
					// Even re-establishing enrollment cannot revive proof from the old generation.
					return module.MFA.EnrollMFA(t.Context(), account.ID)
				}
				mfa := module.MFA
				sessions := module.Auth
				if phase == "before_verification" {
					require.NoError(t, reset())
				} else if afterMint {
					sessions = resetAfterMint{Owner: module.Auth, reset: reset}
				} else {
					mfa = resetAfterVerification{AccountMFA: module.MFA, reset: reset}
				}

				var router http.Handler
				if scope == identityaccess.MFAChallengeScopeTenant {
					root := chi.NewRouter()
					resource := &accountHTTP.Resource{Sessions: sessions, MFAService: mfa}
					root.Mount("/auth", resource.Router(TenantSessionChain))
					router = root
				} else {
					school.AuthService = SchoolPortalAuth(sessions)
					school.MFAService = SchoolPortalMFA(mfa)
					router = newSchoolRouter(school, school.MFAService)
				}
				body := fmt.Sprintf(`{"challenge_token":%q,"code":%q,"remember_device":true}`, token, code.Code)
				request := httptest.NewRequest(http.MethodPost, "/auth/mfa/verify", strings.NewReader(body))
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				if afterMint {
					require.Equal(t, http.StatusOK, response.Code)
				} else {
					require.Equal(t, http.StatusUnauthorized, response.Code)
				}
				require.Zero(t, len(response.Result().Cookies()))
				if !afterMint {
					require.False(t, strings.Contains(response.Body.String(), "access_token"))
				}
				stored, err := module.Auth.ListAccountSessions(t.Context(), identityaccess.AccountSessionFilter{AccountID: account.ID})
				require.NoError(t, err)
				if afterMint {
					require.Equal(t, 1, len(stored))
					require.False(t, stored[0].MFAVerified)
				} else {
					require.Zero(t, len(stored))
				}
				devices, err := module.MFA.ListTrustedDevices(t.Context(), account.ID, tenantID)
				require.NoError(t, err)
				require.Zero(t, len(devices))
			})
		}
	}
}
