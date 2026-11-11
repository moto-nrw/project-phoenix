package tokenprimitives

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHolidayParentSetupWireRoundTrip(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0)
	claims := HolidayParentSetupClaims{Email: "eltern@example.org", OrganizationID: 7}
	got, err := decodeHolidayParentSetup(jsonWire(t, EncodeHolidayParentSetup(claims, now, 30*24*time.Hour)), now)
	require.NoError(t, err)
	assert.Equal(t, "eltern@example.org", got.Email)
	assert.Equal(t, claims.OrganizationID, got.OrganizationID)
	assert.Equal(t, now.Add(30*24*time.Hour).Unix(), got.ExpiresAt)
}

func TestHolidayParentSetupWireRefusesExpiredAndIncompleteTokens(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0)
	wire := func(mutate func(map[string]any)) map[string]any {
		claims := jsonWire(t, EncodeHolidayParentSetup(HolidayParentSetupClaims{Email: "eltern@example.org", OrganizationID: 7}, now, time.Hour))
		mutate(claims)
		return claims
	}
	for name, claims := range map[string]map[string]any{
		"expired":           jsonWire(t, EncodeHolidayParentSetup(HolidayParentSetupClaims{Email: "eltern@example.org", OrganizationID: 7}, now.Add(-2*time.Hour), time.Hour)),
		"without purpose":   wire(func(w map[string]any) { delete(w, holidayParentSetupFlag) }),
		"without expiry":    wire(func(w map[string]any) { delete(w, "exp") }),
		"without Träger":    wire(func(w map[string]any) { delete(w, "org_id") }),
		"without address":   wire(func(w map[string]any) { w["email"] = "" }),
		"with an account":   wire(func(w map[string]any) { w["id"] = float64(42) }),
		"with a scope":      wire(func(w map[string]any) { w["scope"] = "parent" }),
		"pending challenge": wire(func(w map[string]any) { w[pendingChallengeFlag] = true }),
	} {
		_, err := decodeHolidayParentSetup(claims, now)
		assert.Error(t, err, name)
	}
}

// The setup token is no session, no refresh, no MFA token, and none of
// those is a setup token (ADR 0216 point 5).
func TestHolidayParentSetupIsBoundToItsPurpose(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0)
	setup := jsonWire(t, EncodeHolidayParentSetup(HolidayParentSetupClaims{Email: "eltern@example.org", OrganizationID: 7}, now, time.Hour))
	// Even with every claim the other shapes require, the purpose flag
	// refuses it.
	setup["id"], setup["sub"], setup["roles"], setup["token"] = float64(42), "eltern@example.org", []any{}, "raw"
	setup["account_id"], setup["scope"] = float64(42), "tenant"
	_, err := decodeAccess(setup)
	assert.Error(t, err, "a setup token is no access token")
	_, err = decodeRefresh(setup)
	assert.Error(t, err, "a setup token is no refresh token")
	_, err = decodeChallenge(setup, now)
	assert.Error(t, err, "a setup token is no challenge")
	_, err = decodeEnrollment(setup, now)
	assert.Error(t, err, "a setup token is no enrollment token")

	access := jsonWire(t, EncodeAccess(AccessClaims{AccountID: 42, Email: "eltern@example.org", Roles: []string{"guardian"}, Scope: "parent", OrgID: 7}, now, now.Add(time.Hour)))
	refresh := jsonWire(t, EncodeRefresh(RefreshClaims{AccountID: 42, Token: "raw", Scope: "parent", OrgID: 7}, now, now.Add(time.Hour)))
	challenge := jsonWire(t, EncodeChallenge(ChallengeClaims{AccountID: 42, Scope: "tenant"}, now, time.Minute))
	for name, claims := range map[string]map[string]any{"access": access, "refresh": refresh, "challenge": challenge} {
		claims["email"] = "eltern@example.org"
		_, err := decodeHolidayParentSetup(claims, now)
		assert.Error(t, err, "a %s token is no setup token", name)
	}
}
