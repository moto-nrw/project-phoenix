package parent_test

import (
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/spf13/viper"
)

// The JWT config is process-global viper state and identical for every test in
// this package, so it is pinned once here, before the testpkg token-auth
// singleton is lazily created, instead of per router helper. Setting it from
// tests that run in parallel is a concurrent write to viper's override map
// (#2419).
func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	viper.Set("auth_jwt_secret", testJWTSecret)
	viper.Set("auth_jwt_expiry", time.Hour)
	testpkg.Run(m)
}
