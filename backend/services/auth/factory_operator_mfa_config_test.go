package auth_test

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestFactoryOperatorMFAUsesConfiguredTokenAuth(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	cfg := services.FactoryConfig{
		JWTSecret:        "configured-operator-mfa-secret-at-least-32-chars",
		JWTExpiry:        15 * time.Minute,
		JWTRefreshExpiry: 24 * time.Hour,
		FrontendURL:      "http://localhost:3000",
		ParentsURL:       "http://parents.localhost:3000",
		SchoolURL:        "http://schule.localhost:3000",
		TenantDomain:     "localhost",
		OperatorHostname: "operator.localhost:3000",
	}

	factory, err := services.NewFactoryForTestsWithConfig(repositories.NewFactory(db), db, slog.Default(), cfg)
	require.NoError(t, err)

	operator := testpkg.CreateTestOperator(t, db)
	token, err := factory.OperatorMFA.StartChallenge(context.Background(), operator.ID, net.ParseIP("203.0.113.1"))
	require.NoError(t, err)

	configuredTokenAuth, err := authjwt.NewTokenAuthWithDurations(cfg.JWTSecret, cfg.JWTExpiry, cfg.JWTRefreshExpiry)
	require.NoError(t, err)
	_, err = configuredTokenAuth.ParseMFAChallengeJWT(token)
	require.NoError(t, err)
}
