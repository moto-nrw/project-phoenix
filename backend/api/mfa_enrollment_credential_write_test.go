package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken/tokentest"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/tokenprimitives"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The enrollment confirm of every account portal hands the redeemed code's
// proof, credential generation included, to the enrollment write (#6518).
// A password change or MFA reset committed between the code and the write
// records no enrollment and mints nothing; the confirm answers 401
// identity.invalid_credentials like a refused enrollment start.

// TestSchoolMFAEnrollConfirmHoldsTheRedeemedCredentialGeneration drives the
// school confirm through the root's binding: the school scope and the
// generation of the redeemed code reach the enrollment, and its refusal
// answers 401 identity.invalid_credentials without a session.
func TestSchoolMFAEnrollConfirmHoldsTheRedeemedCredentialGeneration(t *testing.T) {
	t.Parallel()
	db, resource, tenantID, _ := setupSchoolRoute(t)
	_, accountID := registerLehrkraft(t, db, resource, tenantID, "school-enroll-generation")
	enrollmentToken, err := tokentest.CreateMFAEnrollmentJWT(testutil.TestTokenAuth(t), sessiontoken.MFAEnrollmentClaims{
		AccountID: accountID, Scope: tokenprimitives.PortalSchool, TenantID: tenantID, CredentialGeneration: 7,
	}, 5*time.Minute)
	require.NoError(t, err)

	var enrolled []identityaccess.VerifiedMFAChallenge
	mfa := SchoolPortalMFA(&MFAServiceMock{
		VerifyMFAChallengeForOwnerFn: func(_ context.Context, _, _, scope string, account, school int64) (identityaccess.VerifiedMFAChallenge, error) {
			return identityaccess.VerifiedMFAChallenge{AccountID: account, TenantID: school, Scope: scope, MFAResetGeneration: 7}, nil
		},
		EnrollMFAFn: func(_ context.Context, proof identityaccess.VerifiedMFAChallenge) error {
			enrolled = append(enrolled, proof)
			return identityaccess.ErrInvalidCredentials
		},
	})
	body := `{"challenge_token":"school-enroll-challenge","code":"123456","remember_device":true}`
	req := httptest.NewRequest(http.MethodPost, "/auth/mfa/enroll/confirm", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+enrollmentToken)
	rec := httptest.NewRecorder()
	newSchoolRouter(resource, mfa).ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"identity.invalid_credentials"`)
	assert.NotContains(t, rec.Body.String(), "access_token")
	assert.Empty(t, rec.Result().Cookies(), "no session and no remembered device")
	assert.Equal(t, []identityaccess.VerifiedMFAChallenge{{
		AccountID: accountID, TenantID: tenantID, Scope: identityaccess.MFAChallengeScopeSchool, MFAResetGeneration: 7,
	}}, enrolled, "the enrollment holds the redeemed code's generation")
	var tokens int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.tokens WHERE account_id = ?`, accountID).Scan(t.Context(), &tokens))
	assert.Zero(t, tokens, "a refused enrollment mints no session")
}

// writeAfterEnrollmentCode commits a credential write on its own connection
// right after the enrollment code was redeemed, before the confirm records
// the enrollment.
type writeAfterEnrollmentCode struct {
	identityaccess.Owner
	write func() error
}

func (m writeAfterEnrollmentCode) VerifyMFAChallengeForOwner(ctx context.Context, challengeToken, code, expectedScope string, accountID, tenantID int64) (identityaccess.VerifiedMFAChallenge, error) {
	verified, err := m.Owner.VerifyMFAChallengeForOwner(ctx, challengeToken, code, expectedScope, accountID, tenantID)
	if err == nil {
		done := make(chan error, 1)
		go func() { done <- m.write() }()
		err = <-done
	}
	return verified, err
}

// TestTraegerEnrollmentConfirmAfterACredentialWriteRecordsNothing runs the
// Träger sign-in, enrollment start and confirm against the database on an
// account past generation 0; a password change or MFA reset commits between
// the redeemed code and the enrollment write. Without a write the confirm
// enrolls, so the root's binding hands the code's generation on.
func TestTraegerEnrollmentConfirmAfterACredentialWriteRecordsNothing(t *testing.T) {
	t.Parallel()
	for _, write := range []string{"none", "password_change", "mfa_reset"} {
		t.Run(write, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			unit := testpkg.TenantRuntime(t, db)
			mail := testpkg.NewCapturingMailer()
			module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), unit, WithAuthTestMailer(mail))
			require.NoError(t, err)
			password := testpkg.NewTestPassword()
			organization := traegerOrganization(t, db, fmt.Sprintf("Träger Einrichtung %s", write), true)
			account := traegerAccount(t, db, password)
			testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleAdministration)
			_, err = db.NewRaw(`UPDATE auth.accounts SET mfa_reset_generation = 3 WHERE id = ?`, account.ID).Exec(context.Background())
			require.NoError(t, err)
			writes := map[string]func() error{
				"none": func() error { return nil },
				"password_change": func() error {
					return module.Auth.ChangeAccountPassword(context.Background(), identityaccess.OwnPasswordChange{
						AccountID: account.ID, CurrentPassword: password, NewPassword: testpkg.NewTestPassword(),
					})
				},
				"mfa_reset": func() error { return module.Auth.DisableMFA(context.Background(), account.ID) },
			}
			identity := writeAfterEnrollmentCode{Owner: module.Auth, write: writes[write]}
			portal := &traegerPortalHandler{Handler: traegerPortalRouter(t, db, identity, unit), mail: mail}

			code, login := traegerLogin(t, portal, account.Email, password, 0)
			require.Equal(t, http.StatusOK, code)
			require.Equal(t, "mfa_enrollment_required", login.Status)
			code, start := traegerPost(t, portal, "/traeger/auth/mfa/enroll/start", login.AccessToken, map[string]any{})
			require.Equal(t, http.StatusOK, code)
			code, answer := traegerPost(t, portal, "/traeger/auth/mfa/enroll/confirm", login.AccessToken, map[string]any{
				"challenge_token": start.ChallengeToken, "code": lastTraegerCode(t, portal, account.Email), "organization_id": organization,
			})

			enrolled, err := module.Auth.HasMFAEnrollment(context.Background(), account.ID)
			require.NoError(t, err)
			var tokens int
			require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.tokens WHERE account_id = ?`, account.ID).Scan(t.Context(), &tokens))
			if write == "none" {
				require.Equal(t, http.StatusOK, code, "the code of the current credential enrolls")
				assert.NotEmpty(t, answer.AccessToken)
				assert.True(t, enrolled)
				assert.Equal(t, 1, tokens)
				return
			}
			assert.Equal(t, http.StatusUnauthorized, code)
			assert.Equal(t, "identity.invalid_credentials", answer.Code)
			assert.Empty(t, answer.AccessToken)
			assert.False(t, enrolled, "the code of the old credential records no enrollment")
			assert.Zero(t, tokens, "and mints no session")
		})
	}
}
