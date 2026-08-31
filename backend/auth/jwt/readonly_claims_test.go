package jwt

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	minted := AppClaims{
		ID:            42,
		Sub:           "target@example.com",
		Roles:         []string{"user"},
		Permissions:   []string{"students:read"},
		TenantID:      77,
		ReadOnly:      true,
		ActingAdminID: 99,
	}
	stamped, err := ParseStructToMap(minted)
	require.NoError(t, err)

	assert.Equal(t, true, stamped["read_only"])
	assert.EqualValues(t, 99, stamped["acting_admin_id"])

	var parsed AppClaims
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

	stamped, err := ParseStructToMap(AppClaims{
		ID:    42,
		Sub:   "user@example.com",
		Roles: []string{"admin"},
	})
	require.NoError(t, err)

	_, hasReadOnly := stamped["read_only"]
	_, hasActingAdmin := stamped["acting_admin_id"]
	assert.False(t, hasReadOnly)
	assert.False(t, hasActingAdmin)

	var parsed AppClaims
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
	var refresh RefreshClaims
	err := refresh.ParseClaims(claims)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read-only preview")
}
