package tokenprimitives

import (
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jsonWire turns a wire value into what a verified token decodes to:
// numbers become float64.
func jsonWire(t *testing.T, claims any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(claims)
	require.NoError(t, err)
	var wire map[string]any
	require.NoError(t, json.Unmarshal(encoded, &wire))
	return wire
}

func TestAccessWireRoundTrip(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0)
	claims := AccessClaims{
		AccountID: 42, Email: "a@example.com", Username: "anna", FirstName: "Anna", LastName: "Muster",
		Roles: []string{"admin"}, RoleIsSystem: []bool{true}, Permissions: []string{"users:read"},
		IsAdmin: true, Scope: "school", TenantID: 7, OrgID: 3, FamilyID: "fam", ReadOnly: true,
		ActingAdminID: 9, PreviewID: "prev", ViaTraeger: true, ExpiresAt: now.Add(time.Hour).Unix(),
	}
	got, err := decodeAccess(jsonWire(t, EncodeAccess(claims, now, now.Add(time.Hour))))
	require.NoError(t, err)
	assert.Equal(t, claims, got)
}

func TestRefreshWireRoundTripAndDefaultExpiry(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0)
	claims := RefreshClaims{AccountID: 42, Token: "raw", TenantID: 7, OrgID: 3, Scope: "org", FamilyID: "fam", LineageID: "lin", ExpiresAt: 1_900_000_000}
	got, err := decodeRefresh(jsonWire(t, EncodeRefresh(claims, now, now.Add(time.Hour))))
	require.NoError(t, err)
	assert.Equal(t, claims, got)

	claims.ExpiresAt = 0
	got, err = decodeRefresh(jsonWire(t, EncodeRefresh(claims, now, now.Add(time.Hour))))
	require.NoError(t, err)
	assert.Equal(t, now.Add(time.Hour).Unix(), got.ExpiresAt)
}

func TestMFAWireRoundTrips(t *testing.T) {
	t.Parallel()
	now := time.Now()
	challenge := ChallengeClaims{AccountID: 42, Scope: "caterer", ChallengeID: 5}
	gotChallenge, err := decodeChallenge(jsonWire(t, EncodeChallenge(challenge, now, time.Minute)), now)
	require.NoError(t, err)
	assert.Equal(t, challenge, gotChallenge)

	wire, err := EncodeEnrollment(EnrollmentClaims{AccountID: 42, Scope: "tenant", TenantID: 7}, now, time.Minute)
	require.NoError(t, err)
	gotEnrollment, err := decodeEnrollment(jsonWire(t, wire), now)
	require.NoError(t, err)
	assert.Equal(t, EnrollmentClaims{AccountID: 42, Scope: "tenant", TenantID: 7, ExpiresAt: now.Add(time.Minute).Unix()}, gotEnrollment)
}

func TestEnrollmentWireRefusesPortalsWithoutEnrollment(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"caterer", "parent", "", "kiosk"} {
		_, err := EncodeEnrollment(EnrollmentClaims{AccountID: 1, Scope: scope}, time.Now(), time.Minute)
		assert.Error(t, err, scope)
	}
}

// The refusals reach logs and 401 details; their texts are part of the
// contract.
func TestDecodersRefuseForeignAndMalformedTokens(t *testing.T) {
	t.Parallel()
	access := func(extra map[string]any) map[string]any {
		wire := map[string]any{"id": float64(42), "sub": "a@example.com", "roles": []any{"admin"}}
		for k, v := range extra {
			wire[k] = v
		}
		return wire
	}
	refresh := func(extra map[string]any) map[string]any {
		wire := map[string]any{"id": float64(42), "token": "raw"}
		for k, v := range extra {
			wire[k] = v
		}
		return wire
	}
	pending := func(flag string, extra map[string]any) map[string]any {
		wire := map[string]any{"account_id": float64(42), "scope": "tenant", flag: true}
		for k, v := range extra {
			wire[k] = v
		}
		return wire
	}
	now := time.Now()
	accessErr := func(w map[string]any) error { _, err := decodeAccess(w); return err }
	refreshErr := func(w map[string]any) error { _, err := decodeRefresh(w); return err }
	challengeErr := func(w map[string]any) error { _, err := decodeChallenge(w, now); return err }
	enrollmentErr := func(w map[string]any) error { _, err := decodeEnrollment(w, now); return err }

	for name, tc := range map[string]struct {
		err  error
		want string
	}{
		"access: challenge":           {accessErr(access(map[string]any{"mfa_pending": true})), "token is a pending-MFA challenge, not a session token"},
		"access: enrollment":          {accessErr(access(map[string]any{"mfa_enrollment_pending": true})), "token is a pending-MFA-enrollment token, not a session token"},
		"access: no id":               {accessErr(map[string]any{"sub": "a", "roles": []any{}}), "missing required claim: id"},
		"access: id not a number":     {accessErr(access(map[string]any{"id": "42"})), "claim id is not a number"},
		"access: no sub":              {accessErr(map[string]any{"id": float64(1), "roles": []any{}}), "missing required claim: sub"},
		"access: sub not a string":    {accessErr(access(map[string]any{"sub": float64(1)})), "claim sub is not a string"},
		"access: no roles":            {accessErr(map[string]any{"id": float64(1), "sub": "a"}), "missing required claim: roles"},
		"access: roles not an array":  {accessErr(access(map[string]any{"roles": "admin"})), "claim roles: value is not an array"},
		"access: role not a string":   {accessErr(access(map[string]any{"roles": []any{"admin", float64(1)}})), "claim roles: element 1 is not a string"},
		"refresh: challenge":          {refreshErr(refresh(map[string]any{"mfa_pending": true})), "token is a pending-MFA challenge, not a refresh token"},
		"refresh: enrollment":         {refreshErr(refresh(map[string]any{"mfa_enrollment_pending": true})), "token is a pending-MFA-enrollment token, not a refresh token"},
		"refresh: preview":            {refreshErr(refresh(map[string]any{"read_only": true})), "token is a read-only preview token, not a refresh token"},
		"refresh: no id":              {refreshErr(map[string]any{"token": "raw"}), "missing required claim: id"},
		"refresh: no token":           {refreshErr(map[string]any{"id": float64(42)}), "missing required claim: token"},
		"refresh: token not a string": {refreshErr(refresh(map[string]any{"token": float64(7)})), "claim token is not a string"},
		"challenge: enrollment flag":  {challengeErr(pending("mfa_pending", map[string]any{"mfa_enrollment_pending": true})), "token is a pending-MFA-enrollment token, not a challenge"},
		"challenge: no account":       {challengeErr(pending("mfa_pending", map[string]any{"account_id": float64(0)})), "missing required claim: account_id"},
		"challenge: unknown scope":    {challengeErr(pending("mfa_pending", map[string]any{"scope": "unknown"})), `invalid scope claim: "unknown"`},
		"challenge: not pending":      {challengeErr(pending("other", nil)), "token is not a pending-MFA challenge"},
		"challenge: expired":          {challengeErr(pending("mfa_pending", map[string]any{"exp": float64(now.Add(-time.Minute).Unix())})), "challenge token expired"},
		"enrollment: challenge flag":  {enrollmentErr(pending("mfa_enrollment_pending", map[string]any{"mfa_pending": true})), "token is a pending-MFA challenge, not an enrollment token"},
		"enrollment: no account":      {enrollmentErr(pending("mfa_enrollment_pending", map[string]any{"account_id": nil})), "missing required claim: account_id"},
		"enrollment: caterer scope":   {enrollmentErr(pending("mfa_enrollment_pending", map[string]any{"scope": "caterer"})), `invalid scope claim: "caterer"`},
		"enrollment: not pending":     {enrollmentErr(pending("other", nil)), "token is not a pending-MFA-enrollment token"},
		"enrollment: expired":         {enrollmentErr(pending("mfa_enrollment_pending", map[string]any{"exp": float64(now.Add(-time.Minute).Unix())})), "enrollment token expired"},
	} {
		require.Error(t, tc.err, name)
		assert.Equal(t, tc.want, tc.err.Error(), name)
	}
}

func TestDecodeAccessKeepsValidPermissionsOnly(t *testing.T) {
	t.Parallel()
	got, err := decodeAccess(map[string]any{
		"id": float64(1), "sub": "a", "roles": []any{}, "permissions": []any{"users:read", float64(3), nil, "groups:read"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"users:read", "groups:read"}, got.Permissions)
	assert.Empty(t, got.Roles)
}

func TestOpaqueCapabilityToken(t *testing.T) {
	t.Parallel()
	raw, fingerprint, err := NewOpaqueCapabilityToken()
	require.NoError(t, err)
	assert.Len(t, raw, 43)
	assert.Len(t, fingerprint, 64)
	_, err = hex.DecodeString(fingerprint)
	require.NoError(t, err)
	assert.Equal(t, fingerprint, OpaqueCapabilityFingerprint(raw))
	assert.NotContains(t, fingerprint, raw)
	other, _, err := NewOpaqueCapabilityToken()
	require.NoError(t, err)
	assert.NotEqual(t, raw, other)
}

func TestDecodeVerifiedReadsEveryShapeOnce(t *testing.T) {
	t.Parallel()
	now := time.Now()
	token := jwt.New()
	for key, value := range jsonWire(t, EncodeRefresh(RefreshClaims{AccountID: 42, Token: "raw", Scope: "platform"}, now, now.Add(time.Hour))) {
		require.NoError(t, token.Set(key, value))
	}
	verified := DecodeVerified(token, now)
	assert.False(t, verified.Expired)
	require.NoError(t, verified.RefreshErr)
	assert.Equal(t, "raw", verified.Refresh.Token)
	assert.Equal(t, now.Add(time.Hour).Unix(), verified.Refresh.ExpiresAt)
	assert.EqualError(t, verified.AccessErr, "missing required claim: sub")
	assert.EqualError(t, verified.EnrollmentErr, "missing required claim: account_id")

	expired := jwt.New()
	require.NoError(t, expired.Set("exp", now.Add(-time.Minute)))
	assert.True(t, DecodeVerified(expired, now).Expired)
}
