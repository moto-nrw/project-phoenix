package credentials

import (
	"crypto/sha256"
	"encoding/hex"
)

// DeviceKeyHash hashes the exact bearer credential, including legacy keys.
// New device keys contain 256 bits of cryptographic randomness. Unlike a
// password, they do not need a slow password KDF or a per-record salt.
func (DeviceKeys) Hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// MaskDeviceKey never reveals a complete credential, including short legacy
// values. Only four trailing characters of longer keys remain visible.
func (DeviceKeys) Mask(raw string) string {
	chars := []rune(raw)
	if len(chars) <= 8 {
		return "..."
	}
	return "..." + string(chars[len(chars)-4:])
}

// DeviceKeys combines the root's key generator with this owner's storage
// credential policy. The application receives only its consumer-owned port.
type DeviceKeys struct {
	Minter interface{ Mint() (string, error) }
}

func (k DeviceKeys) Mint() (string, error) { return k.Minter.Mint() }
