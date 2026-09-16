package platform_test

import (
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"github.com/spf13/viper"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testJWTSecret is generated at runtime to avoid hardcoded secret literals
// that trigger secret scanners. All tests in this package share the value.
var testJWTSecret = func() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}()

// The operator auth/MFA flows under test hash passwords constantly; cheap
// Argon2id params keep that off the test suite's critical path.
//
// The JWT config is process-global viper state and identical for every test
// in this package, so it is pinned once here instead of per test. Restoring it
// from a t.Cleanup would hand a different secret to still-running parallel
// tests (#2419).
func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.UseCheapArgon2Params()
	viper.Set("auth_jwt_secret", testJWTSecret)
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", time.Hour)
	testpkg.Run(m)
}
