package test

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/spf13/viper"
)

// ConfiguredTokenAuth resolves the signer from the seeded test configuration,
// as the API root does from the process configuration, so tokens minted by
// the test helpers and the mounted verifier always share one secret.
func ConfiguredTokenAuth() (*jwt.TokenAuth, error) {
	return jwt.NewTokenAuthWithDurations(
		viper.GetString("auth_jwt_secret"),
		viper.GetDuration("auth_jwt_expiry"),
		viper.GetDuration("auth_jwt_refresh_expiry"),
	)
}

// SessionVerifier mirrors the verifier the production API root mounts once for
// every route. A domain router served in isolation has none, so its
// Authenticator rejects each token until a test mounts this one.
func SessionVerifier(next http.Handler) http.Handler {
	tokenAuth, err := ConfiguredTokenAuth()
	if err != nil {
		panic(err)
	}
	return tokenAuth.Verifier()(next)
}
