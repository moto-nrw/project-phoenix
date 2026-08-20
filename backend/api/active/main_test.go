package active_test

import (
	"testing"
	"time"

	"github.com/spf13/viper"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The JWT config is process-global viper state and identical for every test in
// this package, so it is pinned once here instead of per test. Setting it from
// tests that run in parallel is a concurrent write to viper's override map.
// Per-test tenants keep those parallel tests from sharing rows (#2419).
func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	viper.Set("auth_jwt_secret", testJWTSecret)
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)
	testpkg.Run(m)
}
