package sessiontoken

import "github.com/moto-nrw/project-phoenix/modules/identityaccess/tokenprimitives"

// NewOpaqueCapabilityToken retains the existing session capability-token API.
func NewOpaqueCapabilityToken() (string, string, error) {
	return tokenprimitives.NewOpaqueCapabilityToken()
}

// OpaqueCapabilityFingerprint returns the unchanged opaque-token database fingerprint.
func OpaqueCapabilityFingerprint(raw string) string {
	return tokenprimitives.OpaqueCapabilityFingerprint(raw)
}
