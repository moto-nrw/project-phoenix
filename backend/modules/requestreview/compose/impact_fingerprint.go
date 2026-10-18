package compose

import (
	"crypto/sha256"
	"fmt"
)

// pickupImpactFingerprint must stay byte-identical to securityruntime.Fingerprint,
// which the approval side compares the token against. This package may not
// import that owner (policy rule request-review-view.compose.impact-token);
// TestPickupImpactFingerprintIsLowercaseSHA256Hex pins the encoding.
func pickupImpactFingerprint(content []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(content))
}
