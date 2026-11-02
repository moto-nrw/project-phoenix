package test

import (
	"encoding/base64"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// NewTestPassword creates a throwaway credential satisfying the password policy.
// Reuse it within a scenario (or a package fixture) so setup and login agree;
// never use it for assertions about a particular weak or malformed password.
func NewTestPassword() string {
	return "Aa1!" + randomTestCredential(24)
}

// NewTestSigningSecret creates an opaque HMAC signing credential for a test.
// Share one result between that scenario's issuer and verifier; generate another
// result when the test needs an unrelated signing key.
func NewTestSigningSecret() string {
	return randomTestCredential(64)
}

func randomTestCredential(length int) string {
	random, err := securityruntime.RandomLowerAlphanumeric(length)
	if err != nil {
		panic("generate test credential: " + err.Error())
	}
	return random
}

// NewTestHealthDataKeys returns a HEALTH_DATA_KEYS key ring with one random
// key, so a test seals the health information of the holiday registrations
// like the server does (#4722).
func NewTestHealthDataKeys() string {
	key := make([]byte, 32)
	if err := securityruntime.FillRandom(key); err != nil {
		panic("generate test health data key: " + err.Error())
	}
	return "test:" + base64.StdEncoding.EncodeToString(key)
}

// TestHealthDataKeys is the key ring of the test binary; the test
// configuration serves it as HEALTH_DATA_KEYS.
var TestHealthDataKeys = NewTestHealthDataKeys()
