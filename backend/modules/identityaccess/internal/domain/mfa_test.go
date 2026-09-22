package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The code and cookie primitives the MFA flows sign with moved here with the
// services (#3331); these tests pin the shapes the login and the
// remember-device cookie depend on.

func TestGenerateEmailCode(t *testing.T) {
	t.Parallel()

	for i := 0; i < 50; i++ {
		code, err := GenerateEmailCode()
		require.NoError(t, err)
		assert.Len(t, code, MFAEmailCodeLength, "code must be 6 digits")
		for _, r := range code {
			assert.True(t, r >= '0' && r <= '9', "every char must be a decimal digit")
		}
	}
}

func TestTrustedDeviceToken_GenerateHashSignVerify(t *testing.T) {
	t.Parallel()

	raw, err := GenerateTrustedDeviceToken()
	require.NoError(t, err)
	assert.NotEmpty(t, raw)

	// SHA-256 hex is exactly 64 characters.
	dbHash := HashTrustedDeviceToken(raw)
	assert.Len(t, dbHash, 64)

	secret := DeriveMFASecret("a-very-long-test-secret-with-enough-entropy")
	require.NotEmpty(t, secret)

	signed := SignTrustedDeviceToken(raw, secret)
	assert.Contains(t, signed, ".", "signed token must contain a separator")

	got, ok := VerifyTrustedDeviceToken(signed, secret)
	require.True(t, ok, "expected signature verification to succeed")
	assert.Equal(t, raw, got)
}

func TestTrustedDeviceToken_RejectsTamperedSignature(t *testing.T) {
	t.Parallel()

	raw, err := GenerateTrustedDeviceToken()
	require.NoError(t, err)

	secret := DeriveMFASecret("test-jwt-secret-that-is-long-enough")
	signed := SignTrustedDeviceToken(raw, secret)

	// Flip a byte in the signature half.
	tamperIdx := strings.LastIndex(signed, ".") + 1
	require.Greater(t, tamperIdx, 0)
	tampered := signed[:tamperIdx] + flipByte(signed[tamperIdx:tamperIdx+1]) + signed[tamperIdx+1:]
	require.NotEqual(t, signed, tampered)

	_, ok := VerifyTrustedDeviceToken(tampered, secret)
	assert.False(t, ok, "tampered signature must be rejected")
}

func TestTrustedDeviceToken_RejectsWrongSecret(t *testing.T) {
	t.Parallel()

	raw, err := GenerateTrustedDeviceToken()
	require.NoError(t, err)

	signed := SignTrustedDeviceToken(raw, DeriveMFASecret("secret-A"))
	_, ok := VerifyTrustedDeviceToken(signed, DeriveMFASecret("secret-B"))
	assert.False(t, ok)
}

func TestTrustedDeviceToken_RejectsMalformed(t *testing.T) {
	t.Parallel()

	secret := DeriveMFASecret("any-secret")
	for _, in := range []string{"", "no-dot-at-all", ".", "raw.", ".sig"} {
		_, ok := VerifyTrustedDeviceToken(in, secret)
		assert.False(t, ok, "input %q must not verify", in)
	}
}

func TestDeriveMFASecret_DeterministicAndDistinct(t *testing.T) {
	t.Parallel()

	a := DeriveMFASecret("jwt-secret")
	b := DeriveMFASecret("jwt-secret")
	assert.Equal(t, a, b, "same input must produce same secret")

	c := DeriveMFASecret("different-secret")
	assert.NotEqual(t, a, c, "different inputs must produce different secrets")

	assert.Nil(t, DeriveMFASecret(""), "empty input must return nil")
}

func flipByte(s string) string {
	if len(s) == 0 {
		return s
	}
	b := []byte(s)
	if b[0] == 'A' {
		b[0] = 'B'
	} else {
		b[0] = 'A'
	}
	return string(b)
}

// ShortenUserAgent labels the device in the notification mail and in the
// trusted-device list, so both name one device the same way.
func TestShortenUserAgent(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		userAgent string
		want      string
	}{
		"empty":            {"", "Unbekanntes Gerät"},
		"whitespace only":  {"   ", "Unbekanntes Gerät"},
		"chrome on macOS":  {"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36", "Chrome auf macOS"},
		"firefox on linux": {"Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0", "Firefox auf Linux"},
		"edge on windows":  {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36 Edg/120.0", "Edge auf Windows"},
		"safari on iPhone": {"Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1", "Safari auf iPhone"},
		"safari on iPad":   {"Mozilla/5.0 (iPad; CPU OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/604.1", "Safari auf iPad"},
		"chrome android":   {"Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Mobile Safari/537.36", "Chrome auf Android"},
		"unknown browser":  {"SomeCrawler/1.0 (Android 14)", "Browser auf Android"},
		"unknown all":      {"SomeCrawler/1.0", "Browser"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ShortenUserAgent(tt.userAgent))
		})
	}
}

// A Chrome user agent also names Safari; the browser detection must not
// report Safari for it.
func TestShortenUserAgent_DoesNotMisidentifyChromeAsSafari(t *testing.T) {
	t.Parallel()

	chrome := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"
	assert.Equal(t, "Chrome auf macOS", ShortenUserAgent(chrome))
}

// The admin override allow list is the last line of defence before the
// database CHECK constraint.
func TestIsValidMFAAdminOverride(t *testing.T) {
	t.Parallel()

	assert.True(t, IsValidMFAAdminOverride(MFAAdminOverrideNone))
	assert.True(t, IsValidMFAAdminOverride(MFAAdminOverrideForceOff))
	assert.True(t, IsValidMFAAdminOverride(MFAAdminOverrideForceOn))
	assert.False(t, IsValidMFAAdminOverride("force_maybe"))
	assert.False(t, IsValidMFAAdminOverride(""))
}

// The resolved policy applies the school's mode to a role set, and an
// override wins outright.
func TestMFAPolicyRequiredFor(t *testing.T) {
	t.Parallel()

	assert.True(t, MFAPolicyForced(true).RequiredFor(nil))
	assert.False(t, MFAPolicyForced(false).RequiredFor([]string{AdminRoleName}))
	assert.True(t, MFAPolicyForMode(MFAModeRequiredAll).RequiredFor(nil))
	assert.True(t, MFAPolicyForMode(MFAModeRequiredAdmins).RequiredFor([]string{AdminRoleName}))
	assert.False(t, MFAPolicyForMode(MFAModeRequiredAdmins).RequiredFor([]string{"lehrkraft"}))
	assert.False(t, MFAPolicyForMode(MFAModeOff).RequiredFor([]string{AdminRoleName}))
	assert.False(t, MFAPolicy{}.RequiredFor([]string{AdminRoleName}), "the zero policy requires nothing")
	assert.False(t, MFAPolicyForMode("something-else").RequiredFor([]string{AdminRoleName}))
}

// A school names its own roles. The retained gate matched the admin role
// case-insensitively, and a case-sensitive compare would let a role a school
// created as "Admin" log in without a second factor under required_admins.
func TestMFAPolicyRequiredForMatchesTheAdminRoleCaseInsensitively(t *testing.T) {
	t.Parallel()

	admins := MFAPolicyForMode(MFAModeRequiredAdmins)
	for _, name := range []string{"admin", "Admin", "ADMIN", "AdMiN"} {
		assert.True(t, admins.RequiredFor([]string{name}), "%q is the admin role", name)
	}
	assert.False(t, admins.RequiredFor([]string{"administrator"}), "a different role is not the admin role")
}

// The permission gate honours the area and admin wildcards.
func TestHasPermissionMatch(t *testing.T) {
	t.Parallel()

	assert.True(t, HasPermissionMatch([]string{"users:manage"}, "users:manage"))
	assert.True(t, HasPermissionMatch([]string{"users:*"}, "users:manage"))
	assert.True(t, HasPermissionMatch([]string{"admin:*"}, "users:manage"))
	assert.False(t, HasPermissionMatch([]string{"users:read"}, "users:manage"))
	assert.False(t, HasPermissionMatch(nil, "users:manage"))
}

// The lockout decision left the model in #586 (Rule 12): it is a pure read
// of the stamp with the clock injected. Moved here with the flows (#3331).
func TestMFALocked(t *testing.T) {
	t.Parallel()

	now := time.Now()
	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)

	assert.False(t, MFALocked(nil, now), "no stamp is not locked")
	assert.False(t, MFALocked(&past, now), "a stamp in the past is not locked")
	assert.True(t, MFALocked(&future, now), "a stamp in the future is locked")
}
