package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The approval side verifies the token with securityruntime.Fingerprint, which
// is lowercase SHA-256 hex. A drift here rejects every pickup approval with
// pickup_change_impact_changed.
func TestPickupImpactFingerprintIsLowercaseSHA256Hex(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", pickupImpactFingerprint([]byte("abc")))
}
