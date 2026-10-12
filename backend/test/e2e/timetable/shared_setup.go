package e2e_timetable

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"

	timetableAPI "github.com/moto-nrw/project-phoenix/modules/timetable/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// These aliases keep the existing production-scoped legacy imports in one
// shim while the test-only scenario can use precise types. Moving the imports
// to _test.go would create new internal-test ratchet violations.
type (
	timetableTestResource     = timetableAPI.Resource
	timetableTestDependencies = timetableAPI.Dependencies
	timetableTestRouter       = chi.Router
	timetableTestClaims       = testpkg.Claims
)

var newTimetableTestResource = timetableAPI.NewResource

// Seed the JWT values before the scenario mints a token or mounts the
// verifier; both resolve the signer from this configuration.
func init() {
	if viper.GetString("auth_jwt_secret") == "" {
		viper.Set("auth_jwt_secret", "e2e-test-secret-abcdefghijklmnopqrstuvwxyz")
	}
	if viper.GetDuration("auth_jwt_expiry") == 0 {
		viper.Set("auth_jwt_expiry", 15*time.Minute)
	}
	if viper.GetDuration("auth_jwt_refresh_expiry") == 0 {
		viper.Set("auth_jwt_refresh_expiry", time.Hour)
	}
}
