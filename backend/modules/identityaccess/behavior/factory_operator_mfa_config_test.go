package behavior_test

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestFactoryOperatorMFARefusesChallengeWithoutSMTP(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	cfg := owners.ServiceConfig{
		JWTSecret:        testpkg.NewTestSigningSecret(),
		JWTExpiry:        15 * time.Minute,
		JWTRefreshExpiry: 24 * time.Hour,
		FrontendURL:      "http://localhost:3000",
		PublicAPIURL:     "http://localhost:8080",
		ParentsURL:       "http://parents.localhost:3000",
		SchoolURL:        "http://schule.localhost:3000",
		CatererURL:       "http://caterer.localhost:3000",
		TraegerURL:       "http://traeger.localhost:3000",
		TenantDomain:     "localhost",
		OperatorHostname: "operator.localhost:3000",
	}

	identityModule, err := api.NewTestIdentityAccessWithConfig(testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)

	operator := testpkg.CreateTestOperator(t, db)
	t.Cleanup(func() {
		_, err := db.NewDelete().TableExpr("platform.email_outbox").Where("tenant_id IS NULL AND related_entity_type = ? AND related_entity_id = ?", "operator_mfa_email_code", operator.ID).Exec(context.Background())
		require.NoError(t, err)
	})
	_, err = identityModule.StartOperatorMFAChallenge(context.Background(), operator.ID, net.ParseIP("203.0.113.1"))
	require.ErrorIs(t, err, identityaccess.ErrMFAStatusUnavailable)
}
