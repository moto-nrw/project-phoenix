package jwt

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/stretchr/testify/require"
)

func TestNativeSessionCodecPreservesTokenWire(t *testing.T) {
	t.Parallel()
	legacy, err := NewTokenAuthWithDurations("session-codec-test-key-not-for-deployment", time.Hour, 24*time.Hour)
	require.NoError(t, err)
	native, err := identityCompose.NewSessionTokenCodec(legacy.JwtAuth, time.Hour, 24*time.Hour)
	require.NoError(t, err)
	for _, scope := range []string{"", "org", "parent", "school", "platform"} {
		t.Run(scope, func(t *testing.T) {
			claims := identityaccess.SessionClaims{AccountID: 42, Email: "codec@test.local", Username: "codec", FirstName: "Test", LastName: "Account", Roles: []string{"user"}, Permissions: []string{"students:read"}, IsAdmin: true, Scope: scope, TenantID: 73, OrgID: 8, FamilyID: "family"}
			refresh := identityaccess.RefreshClaims{AccountID: claims.AccountID, Token: "persisted-refresh", Scope: scope, TenantID: claims.TenantID, ExpiresAt: time.Now().Add(2 * time.Hour).Unix()}
			accessToken, refreshToken, err := native.IssueTokenPair(claims, refresh)
			require.NoError(t, err)
			oldClaims, err := legacy.ParseAccessJWT(accessToken)
			require.NoError(t, err)
			require.Equal(t, scope, oldClaims.Scope)
			require.Equal(t, int(claims.AccountID), oldClaims.ID)
			require.Equal(t, claims.Roles, oldClaims.Roles)
			require.Equal(t, claims.Permissions, oldClaims.Permissions)
			decoded, err := native.ParseAccessToken(accessToken)
			require.NoError(t, err)
			claims.ExpiresAt = decoded.ExpiresAt
			require.Equal(t, claims, decoded)
			decodedRefresh, err := native.ParseRefreshToken(refreshToken)
			require.NoError(t, err)
			require.Equal(t, refresh, decodedRefresh, "persisted refresh expiry must not be extended")
			oldToken, err := legacy.CreateJWT(*oldClaims)
			require.NoError(t, err)
			decodedOld, err := native.ParseAccessToken(oldToken)
			require.NoError(t, err)
			decodedOld.ExpiresAt = decoded.ExpiresAt
			require.Equal(t, decoded, decodedOld)
			oldRefresh, err := legacy.CreateRefreshJWT(RefreshClaims{ID: int(refresh.AccountID), Token: refresh.Token, TenantID: refresh.TenantID, Scope: scope, CommonClaims: CommonClaims{ExpiresAt: refresh.ExpiresAt}})
			require.NoError(t, err)
			decodedRefresh, err = native.ParseRefreshToken(oldRefresh)
			require.NoError(t, err)
			require.Equal(t, refresh, decodedRefresh)
		})
	}
}

func TestNativeSessionCodecTokenBoundaries(t *testing.T) {
	t.Parallel()
	legacy, err := NewTokenAuthWithDurations("session-codec-boundary-test-key-only", time.Hour, 24*time.Hour)
	require.NoError(t, err)
	native, err := identityCompose.NewSessionTokenCodec(legacy.JwtAuth, time.Hour, 24*time.Hour)
	require.NoError(t, err)
	for _, kind := range []string{"mfa_pending", "mfa_enrollment_pending", "read_only", "malformed-token"} {
		t.Run(kind, func(t *testing.T) {
			wire := map[string]any{"id": 42, "sub": "codec@test.local", "roles": []string{}, "token": "refresh", "exp": time.Now().Add(time.Hour).Unix()}
			if kind == "malformed-token" {
				wire["token"] = 42
			} else {
				wire[kind] = true
			}
			_, token, err := legacy.JwtAuth.Encode(wire)
			require.NoError(t, err)
			require.NotPanics(t, func() { _, err = native.ParseRefreshToken(token) })
			require.Error(t, err)
			if kind == "mfa_pending" || kind == "mfa_enrollment_pending" {
				_, err = native.ParseAccessToken(token)
				require.Error(t, err)
			}
		})
	}
	_, expired, err := legacy.JwtAuth.Encode(map[string]any{"id": 42, "sub": "codec@test.local", "roles": []string{}, "read_only": true, "acting_admin_id": 9, "preview_id": "preview", "exp": time.Now().Add(-time.Hour).Unix()})
	require.NoError(t, err)
	_, err = native.ParseAccessToken(expired)
	require.ErrorContains(t, err, "access token expired")
	preview, err := native.ParseAccessTokenAllowExpired(expired)
	require.NoError(t, err)
	require.True(t, preview.ReadOnly)
	require.EqualValues(t, 9, preview.ActingAdminID)
	require.Equal(t, "preview", preview.PreviewID)
	other, err := NewTokenAuthWithDurations("different-session-codec-test-key-only", time.Hour, time.Hour)
	require.NoError(t, err)
	_, forged, err := other.JwtAuth.Encode(map[string]any{"id": 42, "sub": "codec@test.local", "roles": []string{}})
	require.NoError(t, err)
	_, err = native.ParseAccessTokenAllowExpired(forged)
	require.Error(t, err, "the expired-token evidence path still verifies the signature")
	for _, scope := range []string{"tenant", "school", "platform"} {
		challenge := identityaccess.MFAChallengeClaims{AccountID: 42, TenantID: 73, Scope: scope, ChallengeID: 91}
		challengeToken, err := native.IssueChallengeToken(challenge, time.Hour)
		require.NoError(t, err)
		oldChallenge, err := legacy.ParseMFAChallengeJWT(challengeToken)
		require.NoError(t, err)
		require.Equal(t, scope, oldChallenge.Scope)
		require.EqualValues(t, 91, oldChallenge.ChallengeID)
		decodedChallenge, err := native.ParseChallengeToken(challengeToken)
		require.NoError(t, err)
		require.Equal(t, challenge, decodedChallenge)
		oldChallengeToken, err := legacy.CreateMFAChallengeJWT(*oldChallenge, time.Hour)
		require.NoError(t, err)
		decodedChallenge, err = native.ParseChallengeToken(oldChallengeToken)
		require.NoError(t, err)
		require.Equal(t, challenge, decodedChallenge)
		_, err = native.ParseAccessToken(challengeToken)
		require.Error(t, err)
		_, err = native.ParseRefreshToken(challengeToken)
		require.Error(t, err)
		token, err := native.IssueMFAEnrollmentToken(42, 73, scope, time.Hour)
		require.NoError(t, err)
		parsed, err := legacy.ParseMFAEnrollmentJWT(token)
		require.NoError(t, err)
		require.Equal(t, scope, parsed.Scope)
		require.EqualValues(t, 42, parsed.AccountID)
		require.EqualValues(t, 73, parsed.TenantID)
		_, err = native.ParseAccessToken(token)
		require.Error(t, err)
		_, err = native.ParseChallengeToken(token)
		require.Error(t, err)
	}
}
