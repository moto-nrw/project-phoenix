package services

import "github.com/moto-nrw/project-phoenix/modules/securityruntime"

// AnalyticsFingerprint hashes the pseudonymous analytics user ID with the
// Security Runtime's SHA-256 fingerprint, the same hash the browser uses
// (#3603). The root hands it to the tracker it builds.
func AnalyticsFingerprint(content []byte) string {
	return securityruntime.Fingerprint(content)
}
