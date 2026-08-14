// Package rotation defines the shared security boundary for refresh-token
// rotation recovery across the tenant, parent, and operator portals.
package rotation

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"time"
)

// FamilyFingerprint returns a stable, non-reversible identifier suitable for
// correlating revocation audit rows without persisting the session family ID.
func FamilyFingerprint(familyID string) string {
	sum := sha256.Sum256([]byte(familyID))
	return hex.EncodeToString(sum[:])
}

const (
	// RecoveryProofHeader carries an independent, high-entropy secret that is
	// kept only in the encrypted, HTTP-only Auth.js session cookie and sent on
	// the server-to-server refresh request. It is deliberately not an access or
	// refresh token. The backend stores only its SHA-256 digest and never logs
	// either value.
	RecoveryProofHeader = "X-Refresh-Recovery-Proof"

	// RecoveryGrace matches the frontend's bounded old-token deduplication
	// window. Within it, a persisted successor may be returned after an
	// interrupted response or process replacement. Outside it, reuse revokes
	// the whole token family.
	RecoveryGrace = 5 * time.Minute

	// MaxRecoveryHops bounds database work and rejects corrupt/cyclic lineage.
	MaxRecoveryHops = 8
)

type recoveryProofContextKey struct{}

// WithRecoveryProof hashes proof before attaching it to the request context,
// keeping the bearer credential itself out of downstream service state.
func WithRecoveryProof(ctx context.Context, proof string) context.Context {
	if proof == "" {
		return ctx
	}
	sum := sha256.Sum256([]byte(proof))
	return context.WithValue(ctx, recoveryProofContextKey{}, sum)
}

// RecoveryProofHash returns a defensive copy of the request-bound proof hash.
func RecoveryProofHash(ctx context.Context) []byte {
	sum, ok := ctx.Value(recoveryProofContextKey{}).([sha256.Size]byte)
	if !ok {
		return nil
	}
	result := make([]byte, sha256.Size)
	copy(result, sum[:])
	return result
}

// MatchesRecoveryProof performs constant-time comparison. A missing proof or
// legacy handoff is deliberately not recoverable and remains replay-detected.
func MatchesRecoveryProof(ctx context.Context, expected []byte) bool {
	actual := RecoveryProofHash(ctx)
	return len(expected) == sha256.Size && len(actual) == sha256.Size && subtle.ConstantTimeCompare(expected, actual) == 1
}
