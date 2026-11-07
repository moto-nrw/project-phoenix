package sessiontoken_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken/tokentest"
)

// jsonRoundTrip pushes the stamped claim map through JSON so numbers arrive
// as float64 — exactly the shape ParseClaims sees after jwx decoding.
func jsonRoundTrip(t *testing.T, claims map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(claims)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

// A read-only preview token (#2893) must survive the stamp → parse round
// trip with both preview claims intact.
func TestReadOnlyPreviewClaimsRoundTrip(t *testing.T) {
	t.Parallel()

	minted := sessiontoken.AppClaims{
		ID:            42,
		Sub:           "target@example.com",
		Roles:         []string{"user"},
		Permissions:   []string{"students:read"},
		TenantID:      77,
		ReadOnly:      true,
		ActingAdminID: 99,
	}
	stamped, err := tokentest.ParseStructToMap(minted)
	require.NoError(t, err)

	assert.Equal(t, true, stamped["read_only"])
	assert.EqualValues(t, 99, stamped["acting_admin_id"])

	var parsed sessiontoken.AppClaims
	require.NoError(t, parsed.ParseClaims(jsonRoundTrip(t, stamped)))
	assert.True(t, parsed.ReadOnly)
	assert.True(t, parsed.IsReadOnlyPreview())
	assert.EqualValues(t, 99, parsed.ActingAdminID)
	assert.Equal(t, 42, parsed.ID)
	assert.EqualValues(t, 77, parsed.TenantID)
}

// Regular tokens must not grow preview claims: the stamped map omits them
// and the parsed struct stays zero.
func TestRegularClaimsCarryNoPreviewFields(t *testing.T) {
	t.Parallel()

	stamped, err := tokentest.ParseStructToMap(sessiontoken.AppClaims{
		ID:    42,
		Sub:   "user@example.com",
		Roles: []string{"admin"},
	})

	require.NoError(t, err)

	_, hasReadOnly := stamped["read_only"]
	_, hasActingAdmin := stamped["acting_admin_id"]
	assert.False(t, hasReadOnly)
	assert.False(t, hasActingAdmin)

	var parsed sessiontoken.AppClaims
	require.NoError(t, parsed.ParseClaims(jsonRoundTrip(t, stamped)))
	assert.False(t, parsed.IsReadOnlyPreview())
	assert.Zero(t, parsed.ActingAdminID)
}

// A preview token must never pass as a refresh token, even if a future
// change ever adds the fields RefreshClaims wants (mirrors the MFA gates).
func TestRefreshClaimsRejectReadOnlyPreviewToken(t *testing.T) {
	t.Parallel()

	claims := map[string]any{
		"id":        float64(42),
		"token":     "some-refresh-token",
		"read_only": true,
	}
	var refresh sessiontoken.RefreshClaims
	err := refresh.ParseClaims(claims)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read-only preview")
}

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

// The preview id identifies ONE preview instance across every re-mint, so it
// has to survive the stamp → parse round trip and stay absent elsewhere.
func TestPreviewIDClaimRoundTrip(t *testing.T) {
	t.Parallel()

	stamped, err := tokentest.ParseStructToMap(sessiontoken.AppClaims{
		ID:            42,
		Sub:           "target@example.com",
		Roles:         []string{"user"},
		ReadOnly:      true,
		ActingAdminID: 99,
		PreviewID:     "0123456789abcdef",
	})

	require.NoError(t, err)
	assert.Equal(t, "0123456789abcdef", stamped["preview_id"])

	var parsed sessiontoken.AppClaims
	require.NoError(t, parsed.ParseClaims(jsonRoundTrip(t, stamped)))
	assert.Equal(t, "0123456789abcdef", parsed.PreviewID)

	regular, err := tokentest.ParseStructToMap(sessiontoken.AppClaims{ID: 42, Sub: "u@example.com", Roles: []string{"admin"}})
	require.NoError(t, err)
	_, hasPreviewID := regular["preview_id"]
	assert.False(t, hasPreviewID)
}
