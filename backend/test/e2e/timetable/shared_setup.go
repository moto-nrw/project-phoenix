package e2e_timetable

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"

	timetableAPI "github.com/moto-nrw/project-phoenix/api/timetable"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// These aliases keep the existing production-scoped legacy imports in one
// shim while the test-only scenario can use precise types. Moving the imports
// to _test.go would create new internal-test ratchet violations.
type (
	timetableTestResource     = timetableAPI.Resource
	timetableTestDependencies = timetableAPI.Dependencies
	timetableTestRouter       = chi.Router
	timetableTestTokenAuth    = jwt.TokenAuth
	timetableTestClaims       = jwt.AppClaims
)

var (
	newTimetableTestResource = timetableAPI.NewResource
	newTimetableTokenAuth    = jwt.NewTokenAuth
)

// Seed the JWT values before the test-only scenario creates a TokenAuth.
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
