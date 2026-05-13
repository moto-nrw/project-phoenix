package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/moto-nrw/project-phoenix/auth/userpass"
)

// MFA constants enforced across the service. Kept in one place so the model,
// service, and tests agree on lengths and formats.
const (
	// MFAEmailCodeLength is the number of decimal digits in a 6-digit email code.
	MFAEmailCodeLength = 6

	// MFATrustedDeviceTokenBytes is the random-source size for trusted-device tokens.
	// 32 bytes -> 256 bits of entropy, far above the OWASP "≥128 bits" recommendation.
	MFATrustedDeviceTokenBytes = 32

	// trustedDeviceHKDFInfo derives a per-purpose secret from AUTH_JWT_SECRET so
	// the MFA cookie HMAC and the JWT signing key never share a key.
	trustedDeviceHKDFInfo = "mfa-trusted-device-v1"
)

// GenerateEmailCode returns a freshly generated 6-digit numeric code suitable
// for emailing to a user. It is sourced from crypto/rand and zero-padded so
// every returned string has exactly MFAEmailCodeLength characters.
func GenerateEmailCode() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("read random bytes for mfa email code: %w", err)
	}
	// Modulo bias for 10^6 over 2^32 is ~10^-3 — negligible given the 10-min
	// TTL, single-use semantics, and 5-attempt lockout. If this ever moves to
	// a higher-stakes flow (e.g. step-up for transaction signing) switch to a
	// rejection-sampling approach.
	n := binary.BigEndian.Uint32(buf[:]) % 1_000_000
	return fmt.Sprintf("%06d", n), nil
}

// HashShortCode wraps userpass.HashPassword so MFA email-code hashing goes
// through the project-wide Argon2id helper. That keeps the algorithm +
// parameters consistent across token types and means that any future tuning
// of the Argon2 parameters automatically applies to MFA hashes too.
func HashShortCode(plain string) (string, error) {
	return userpass.HashPassword(plain, nil)
}

// VerifyShortCode returns true iff `plain` matches `encodedHash`. The
// underlying userpass.VerifyPassword uses subtle.ConstantTimeCompare, so this
// helper is safe to call on user-supplied input.
func VerifyShortCode(plain, encodedHash string) (bool, error) {
	return userpass.VerifyPassword(plain, encodedHash)
}

// GenerateTrustedDeviceToken returns a fresh URL-safe random token for use as
// the cookie value. The DB hash is computed separately via HashTrustedDeviceToken.
func GenerateTrustedDeviceToken() (string, error) {
	buf := make([]byte, MFATrustedDeviceTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random bytes for trusted-device token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashTrustedDeviceToken returns a hex-encoded SHA-256 of the raw token.
// SHA-256 is sufficient here (not Argon2) because the token itself is
// 256 bits of cryptographic randomness — there is no offline-guessing
// surface to defend against.
func HashTrustedDeviceToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

// SignTrustedDeviceToken returns base64(rawToken) + "." + base64(HMAC-SHA256(rawToken, secret)).
// The cookie ships the signed form so that an attacker who guesses the
// rawToken bytes still can't construct a valid cookie without the server
// secret. Verification re-derives the HMAC and rejects on mismatch via
// constant-time compare.
func SignTrustedDeviceToken(rawToken string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(rawToken))
	sig := mac.Sum(nil)
	return rawToken + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// VerifyTrustedDeviceToken splits a signed cookie value into its raw-token
// and signature parts and returns the raw token iff the signature verifies
// against `secret`. Returns ("", false) on any malformed or tampered input.
func VerifyTrustedDeviceToken(signedToken string, secret []byte) (string, bool) {
	dot := -1
	for i := len(signedToken) - 1; i >= 0; i-- {
		if signedToken[i] == '.' {
			dot = i
			break
		}
	}
	if dot <= 0 || dot == len(signedToken)-1 {
		return "", false
	}
	raw := signedToken[:dot]
	sigB64 := signedToken[dot+1:]

	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return "", false
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(raw))
	expected := mac.Sum(nil)

	if subtle.ConstantTimeCompare(sig, expected) != 1 {
		return "", false
	}
	return raw, true
}

// DeriveMFASecret produces a per-purpose secret from the JWT secret using
// HMAC-SHA256 with a fixed info string. This avoids requiring a separate
// env var while still ensuring the trusted-device HMAC key is independent
// from the JWT signing key (cryptographic domain separation).
func DeriveMFASecret(jwtSecret string) []byte {
	if jwtSecret == "" {
		return nil
	}
	mac := hmac.New(sha256.New, []byte(jwtSecret))
	mac.Write([]byte(trustedDeviceHKDFInfo))
	return mac.Sum(nil)
}
