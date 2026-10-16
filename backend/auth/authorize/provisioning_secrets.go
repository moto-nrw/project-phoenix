package authorize

import "github.com/moto-nrw/project-phoenix/internal/randstr"

// DeviceAPIKey returns a fresh device API key: 32 random bytes hex-encoded
// with the "dev_" prefix.
func DeviceAPIKey() (string, error) {
	return randstr.APIKey()
}

// RandomLowerAlphanumeric returns n uniformly random lowercase letters and
// digits from the cryptographic entropy source.
func RandomLowerAlphanumeric(n int) (string, error) {
	return randstr.String(n, randstr.LowerAlphanumeric)
}
