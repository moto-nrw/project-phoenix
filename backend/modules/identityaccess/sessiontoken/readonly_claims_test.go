package sessiontoken_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken/tokentest"
)

// ParseAccessJWT is how the staff-preview end call (#2893) proves which
// preview a request closes: the token arrives in the request BODY, so it must
// be verified as strictly as one that came through the Verifier middleware.
func TestParseAccessJWTVerifiesPreviewTokens(t *testing.T) {
	t.Parallel()

	tokenAuth, err := sessiontoken.NewTokenAuthWithDurations("parse-access-jwt-test-secret", 0, 0)
	require.NoError(t, err)
	tokenAuth.JwtExpiry = time.Hour

	minted := sessiontoken.AppClaims{
		ID:            42,
		Sub:           "target@example.com",
		Roles:         []string{"user"},
		TenantID:      77,
		ReadOnly:      true,
		ActingAdminID: 99,
	}
	token, err := tokentest.CreateJWT(tokenAuth, minted)
	require.NoError(t, err)

	parsed, err := tokenAuth.ParseAccessJWT(token)
	require.NoError(t, err)
	assert.True(t, parsed.IsReadOnlyPreview())
	assert.EqualValues(t, 99, parsed.ActingAdminID)
	assert.Equal(t, 42, parsed.ID)
	assert.EqualValues(t, 77, parsed.TenantID)

	t.Run("rejects a token signed with another secret", func(t *testing.T) {
		other, otherErr := sessiontoken.NewTokenAuthWithDurations("a-different-secret", 0, 0)
		require.NoError(t, otherErr)
		_, parseErr := other.ParseAccessJWT(token)
		require.Error(t, parseErr)
	})

	t.Run("rejects an expired token", func(t *testing.T) {
		expiring, expErr := sessiontoken.NewTokenAuthWithDurations("parse-access-jwt-test-secret", 0, 0)
		require.NoError(t, expErr)
		expiring.JwtExpiry = -time.Minute
		expired, createErr := tokentest.CreateJWT(expiring, minted)
		require.NoError(t, createErr)

		_, parseErr := tokenAuth.ParseAccessJWT(expired)
		require.Error(t, parseErr)
	})
}

// A preview left open past the access expiry must still end with an audit
// row, so the end path parses the expired token — signature and all other
// checks unchanged. It grants no access, it only identifies the preview.
func TestParseExpiredAccessJWTAcceptsExpiredPreviewTokens(t *testing.T) {
	t.Parallel()

	tokenAuth, err := sessiontoken.NewTokenAuthWithDurations("parse-expired-access-jwt-secret", 0, 0)
	require.NoError(t, err)

	expiring, err := sessiontoken.NewTokenAuthWithDurations("parse-expired-access-jwt-secret", 0, 0)
	require.NoError(t, err)
	expiring.JwtExpiry = -time.Minute
	expired, err := tokentest.CreateJWT(expiring, sessiontoken.AppClaims{
		ID:            42,
		Sub:           "target@example.com",
		Roles:         []string{"user"},
		TenantID:      77,
		ReadOnly:      true,
		ActingAdminID: 99,
		PreviewID:     "cafebabe",
	})

	require.NoError(t, err)

	_, err = tokenAuth.ParseAccessJWT(expired)
	require.Error(t, err, "the strict parse still rejects expired tokens")

	parsed, err := tokentest.ParseExpiredAccessJWT(tokenAuth, expired)
	require.NoError(t, err)
	assert.True(t, parsed.IsReadOnlyPreview())
	assert.EqualValues(t, 99, parsed.ActingAdminID)
	assert.Equal(t, "cafebabe", parsed.PreviewID)
	assert.Positive(t, parsed.ExpiresAt, "the expiry survives the parse")
	assert.Less(t, parsed.ExpiresAt, time.Now().Unix())

	t.Run("still rejects a foreign signature", func(t *testing.T) {
		other, otherErr := sessiontoken.NewTokenAuthWithDurations("a-different-secret", 0, 0)
		require.NoError(t, otherErr)
		_, parseErr := tokentest.ParseExpiredAccessJWT(other, expired)
		require.Error(t, parseErr)
	})
}
