// Router-level tests for the Träger-Portal's password reset (#5149): the link
// reaches Träger-Büro members only and lands on the Träger host.
package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// traegerResetLink reads the reset link of the newest reset mail to address.
func traegerResetLink(t *testing.T, portal *traegerPortalHandler, address string) string {
	t.Helper()
	messages := portal.mail.Messages()
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].To.Address != address || messages[i].Template != "password-reset.html" {
			continue
		}
		content, ok := messages[i].Content.(map[string]any)
		require.True(t, ok, "the reset mail carries its template content")
		link, _ := content["ResetURL"].(string)
		return link
	}
	return ""
}

func forgetTraegerReset(t *testing.T, db *bun.DB, accountID int64, address string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_, err := db.NewRaw(`DELETE FROM auth.password_reset_tokens WHERE account_id = ?`, accountID).Exec(ctx)
		require.NoError(t, err)
		_, err = db.NewRaw(`DELETE FROM auth.password_reset_rate_limits WHERE email = ?`, address).Exec(ctx)
		require.NoError(t, err)
	})
}

// The link reaches Träger-Büro members only, opens the Träger host, and the
// new password ends the open sessions and signs the member in again.
func TestTraegerPortalPasswordResetLinksMembersToTheTraegerHost(t *testing.T) {
	t.Parallel()
	db, portal, identity, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Rücksetzen", true)
	member := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, member.ID, identityaccess.TraegerRoleAdministration)
	stranger := traegerAccount(t, db, password)
	forgetTraegerReset(t, db, member.ID, member.Email)
	forgetTraegerReset(t, db, stranger.ID, stranger.Email)
	code, before := traegerSignIn(t, portal, member.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	require.NotEmpty(t, before.RefreshToken)

	code, _ = traegerPost(t, portal, "/traeger/auth/password-reset", "", map[string]string{"email": stranger.Email})
	assert.Equal(t, http.StatusOK, code, "an account without Träger-Büro access gets the same answer")
	code, _ = traegerPost(t, portal, "/traeger/auth/password-reset", "", map[string]string{"email": strings.ToUpper(member.Email)})
	require.Equal(t, http.StatusOK, code)

	require.Eventually(t, func() bool { return traegerResetLink(t, portal, member.Email) != "" }, 2*time.Second, 10*time.Millisecond)
	assert.Empty(t, traegerResetLink(t, portal, stranger.Email), "the Träger-Portal mails no link to a non-member")
	link := traegerResetLink(t, portal, member.Email)
	require.True(t, strings.HasPrefix(link, "http://traeger.invalid/reset-password?token="), "the link opens the Träger host: %s", link)
	token := strings.TrimPrefix(link, "http://traeger.invalid/reset-password?token=")

	renewed := testpkg.NewTestPassword()
	code, answer := traegerPost(t, portal, "/traeger/auth/password-reset/confirm", "", map[string]string{
		"token": token, "new_password": renewed, "confirm_password": renewed + "x",
	})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, "identity.password_mismatch", answer.Code)
	code, _ = traegerPost(t, portal, "/traeger/auth/password-reset/confirm", "", map[string]string{
		"token": token, "new_password": renewed, "confirm_password": renewed,
	})
	require.Equal(t, http.StatusOK, code)
	code, answer = traegerPost(t, portal, "/traeger/auth/password-reset/confirm", "", map[string]string{
		"token": token, "new_password": renewed, "confirm_password": renewed,
	})
	assert.Equal(t, http.StatusGone, code, "a spent link cannot be used again")
	assert.Equal(t, "identity.password_reset_link_invalid", answer.Code)
	_, _, err := identity.RefreshTokenWithAudit(context.Background(), before.RefreshToken, "", "", "")
	require.Error(t, err, "the new password ends the Träger session that was open before")

	code, signedIn := traegerSignIn(t, portal, member.Email, renewed, 0)
	require.Equal(t, http.StatusOK, code)
	assert.NotEmpty(t, signedIn.AccessToken, "the new password signs the member in to the Träger-Portal")
}

// The Träger-Portal keeps the reset limit of the other portals: three
// requests an hour, then 429 with the Retry-After the dialog counts down.
func TestTraegerPortalPasswordResetIsRateLimited(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	portal, _ := traegerPortalOn(t, db, testpkg.TenantRuntime(t, db), WithAuthTestPasswordResetRateLimit(true))
	organization := traegerOrganization(t, db, "Träger Grenze", true)
	member := traegerAccount(t, db, testpkg.NewTestPassword())
	testpkg.CreateTestTraegerMember(t, db, organization, member.ID, identityaccess.TraegerRoleReadOnly)
	forgetTraegerReset(t, db, member.ID, member.Email)

	for range 3 {
		code, _ := traegerPost(t, portal, "/traeger/auth/password-reset", "", map[string]string{"email": member.Email})
		require.Equal(t, http.StatusOK, code)
	}
	rec := traegerRequest(t, portal, "/traeger/auth/password-reset", map[string]string{"email": member.Email})
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"), "the countdown of the reset dialog reads Retry-After")
	assert.Contains(t, rec.Body.String(), "identity.password_reset_rate_limited")
}
