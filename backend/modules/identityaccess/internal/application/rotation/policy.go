// Package rotation holds the bounds of refresh-token rotation recovery that
// the account portals and the operator portal share. The hand-off engine and
// the recovery proof live in the domain (domain.HandoffChain).
package rotation

import (
	"crypto/sha256"
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
	// RecoveryGrace matches the frontend's bounded old-token deduplication
	// window. Within it, a persisted successor may be returned after an
	// interrupted response or process replacement. Outside it, reuse revokes
	// the whole token family.
	RecoveryGrace = 5 * time.Minute
	// MaxRecoveryHops bounds database work and rejects corrupt/cyclic lineage.
	MaxRecoveryHops = 8
)
