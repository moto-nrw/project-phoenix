package webhooksig

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSecret      = "test-signing-secret"
	testOtherSecret = "rotated-signing-secret"
)

func signedNow(t *testing.T, secret string, body []byte, now time.Time) (string, string) {
	t.Helper()
	ts := strconv.FormatInt(now.Unix(), 10)
	return ts, Sign(secret, ts, body)
}

func TestVerify_Valid(t *testing.T) {
	v := NewVerifier(testSecret)
	body := []byte(`{"events":[]}`)
	now := time.Now()
	ts, sig := signedNow(t, testSecret, body, now)

	require.NoError(t, v.Verify(ts, sig, body, now))
}

func TestVerify_WrongSecret(t *testing.T) {
	v := NewVerifier(testSecret)
	body := []byte(`{"events":[]}`)
	now := time.Now()
	ts, sig := signedNow(t, "not-the-secret", body, now)

	assert.ErrorIs(t, v.Verify(ts, sig, body, now), ErrInvalidSignature)
}

func TestVerify_TamperedBody(t *testing.T) {
	v := NewVerifier(testSecret)
	body := []byte(`{"events":[{"type":"delivered"}]}`)
	now := time.Now()
	ts, sig := signedNow(t, testSecret, body, now)

	tampered := []byte(`{"events":[{"type":"bounced"}]}`)
	assert.ErrorIs(t, v.Verify(ts, sig, tampered, now), ErrInvalidSignature)
}

// The window is symmetric: a far-future timestamp is as suspicious as a stale
// one, and accepting it would extend an attacker's replay window arbitrarily.
func TestVerify_TimestampSkewBothDirections(t *testing.T) {
	v := NewVerifier(testSecret)
	body := []byte(`{"events":[]}`)
	now := time.Now()

	for _, offset := range []time.Duration{-6 * time.Minute, 6 * time.Minute} {
		signedAt := now.Add(offset)
		ts, sig := signedNow(t, testSecret, body, signedAt)
		assert.ErrorIs(t, v.Verify(ts, sig, body, now), ErrTimestampSkew,
			"offset %s should be rejected", offset)
	}

	// Just inside the window still passes.
	inside := now.Add(-4 * time.Minute)
	ts, sig := signedNow(t, testSecret, body, inside)
	assert.NoError(t, v.Verify(ts, sig, body, now))
}

func TestVerify_KeyRotation(t *testing.T) {
	v := NewVerifier(testSecret + " , " + testOtherSecret)
	body := []byte(`{"events":[]}`)
	now := time.Now()

	for _, secret := range []string{testSecret, testOtherSecret} {
		ts, sig := signedNow(t, secret, body, now)
		assert.NoError(t, v.Verify(ts, sig, body, now), "secret %q should be accepted", secret)
	}
}

func TestVerify_MalformedHeaders(t *testing.T) {
	v := NewVerifier(testSecret)
	body := []byte(`{"events":[]}`)
	now := time.Now()
	ts, sig := signedNow(t, testSecret, body, now)

	assert.ErrorIs(t, v.Verify("", sig, body, now), ErrMissingHeaders)
	assert.ErrorIs(t, v.Verify(ts, "", body, now), ErrMissingHeaders)
	assert.ErrorIs(t, v.Verify("not-a-number", sig, body, now), ErrMalformedHeader)
	assert.ErrorIs(t, v.Verify(ts, "v2=abcdef", body, now), ErrMalformedHeader)
	assert.ErrorIs(t, v.Verify(ts, "v1=nothex", body, now), ErrMalformedHeader)
	// Correct hex encoding but the wrong length is still malformed, not a
	// signature mismatch — it can never be a valid SHA-256 MAC.
	assert.ErrorIs(t, v.Verify(ts, "v1=aabb", body, now), ErrMalformedHeader)
}

// An unset secret must fail closed. A webhook endpoint that silently accepts
// everything because someone forgot an env var is the worst outcome here.
func TestVerify_NoSecretFailsClosed(t *testing.T) {
	v := NewVerifier("   ")
	assert.False(t, v.Configured())

	body := []byte(`{"events":[]}`)
	now := time.Now()
	ts, sig := signedNow(t, testSecret, body, now)
	assert.ErrorIs(t, v.Verify(ts, sig, body, now), ErrNoSecret)
}
