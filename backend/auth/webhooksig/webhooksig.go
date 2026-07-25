// Package webhooksig verifies HMAC signatures on inbound provider webhooks.
//
// The endpoints this guards are unauthenticated with respect to JWTs — the
// signature IS the credential. Everything here is therefore deliberately
// conservative: verification runs on the raw body before any decoding, an
// absent secret fails closed, and no error distinguishes "wrong secret" from
// "tampered body" to the caller.
package webhooksig

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Header names carrying the signature material.
const (
	HeaderTimestamp = "X-Phoenix-Webhook-Timestamp"
	HeaderSignature = "X-Phoenix-Webhook-Signature"
)

// signaturePrefix versions the scheme so a future rotation to a different MAC
// can coexist with in-flight requests.
const signaturePrefix = "v1="

// DefaultMaxSkew bounds how far a request timestamp may be from our clock.
// Symmetric on purpose: a far-future timestamp is as suspicious as a stale
// one, and accepting it would extend an attacker's replay window arbitrarily.
const DefaultMaxSkew = 5 * time.Minute

// Verification failures. Callers must map all of them to one opaque 401 —
// distinguishing them for the client would build a signing oracle.
var (
	ErrNoSecret         = errors.New("webhook signing secret is not configured")
	ErrMissingHeaders   = errors.New("webhook signature headers are missing")
	ErrMalformedHeader  = errors.New("webhook signature header is malformed")
	ErrTimestampSkew    = errors.New("webhook timestamp is outside the accepted window")
	ErrInvalidSignature = errors.New("webhook signature does not match")
)

// Verifier holds the accepted signing secrets.
type Verifier struct {
	secrets []string
	maxSkew time.Duration
}

// NewVerifier parses a comma-separated secret list. Multiple secrets exist so
// a key can be rotated with an overlap window instead of a flag day; any
// member matching is accepted. Returns a Verifier with no secrets when the
// input is blank — Verify then fails with ErrNoSecret, never open.
func NewVerifier(secretList string) *Verifier {
	var secrets []string
	for part := range strings.SplitSeq(secretList, ",") {
		if s := strings.TrimSpace(part); s != "" {
			secrets = append(secrets, s)
		}
	}
	return &Verifier{secrets: secrets, maxSkew: DefaultMaxSkew}
}

// Configured reports whether any secret is available. Handlers use this to
// answer 503 instead of 401, so an operator can tell a misconfiguration from
// an attack.
func (v *Verifier) Configured() bool {
	return v != nil && len(v.secrets) > 0
}

// Verify checks the signature over the raw request body.
//
// The signed string is "v1:<timestamp>:<body>" — the timestamp is inside the
// MAC, so it cannot be adjusted to widen the replay window without breaking
// the signature.
func (v *Verifier) Verify(timestampHeader, signatureHeader string, body []byte, now time.Time) error {
	if !v.Configured() {
		return ErrNoSecret
	}
	timestampHeader = strings.TrimSpace(timestampHeader)
	signatureHeader = strings.TrimSpace(signatureHeader)
	if timestampHeader == "" || signatureHeader == "" {
		return ErrMissingHeaders
	}

	unix, err := strconv.ParseInt(timestampHeader, 10, 64)
	if err != nil {
		return ErrMalformedHeader
	}
	if delta := now.Sub(time.Unix(unix, 0)); delta > v.maxSkew || delta < -v.maxSkew {
		return ErrTimestampSkew
	}

	if !strings.HasPrefix(signatureHeader, signaturePrefix) {
		return ErrMalformedHeader
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signatureHeader, signaturePrefix))
	if err != nil || len(provided) != sha256.Size {
		return ErrMalformedHeader
	}

	payload := append([]byte("v1:"+timestampHeader+":"), body...)
	for _, secret := range v.secrets {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		if hmac.Equal(mac.Sum(nil), provided) {
			return nil
		}
	}
	return ErrInvalidSignature
}

// Sign produces the signature header value for a body and timestamp. Used by
// tests and by the local verification script; production only verifies.
func Sign(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(append([]byte("v1:"+timestamp+":"), body...))
	return signaturePrefix + hex.EncodeToString(mac.Sum(nil))
}
