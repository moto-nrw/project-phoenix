package services

import (
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFactoryStudentConsentUsesAuditRoutedRepository(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	owners, err := newOwnerCapabilitiesForTests(db)
	require.NoError(t, err)

	observedAppends := 0
	factory, err := newFactory(
		repos,
		db,
		slog.Default(),
		FactoryConfig{
			JWTSecret:        "test-secret-must-be-at-least-32-chars-long-for-real",
			JWTExpiry:        15 * time.Minute,
			JWTRefreshExpiry: 24 * time.Hour,
			FrontendURL:      "http://localhost:3000",
			ParentsURL:       "http://parents.localhost:3000",
			SchoolURL:        "http://schule.localhost:3000",
			TenantDomain:     "localhost",
			OperatorHostname: "operator.localhost:3000",
		},
		owners.organizations,
		owners.persons,
		owners.groups,
		owners.rooms,
		owners.membership,
		owners.calendar,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		func(_ string, _ time.Duration, _ int, _ error) { observedAppends++ },
		func(string, string, string, time.Duration, error) {},
		func(string, string, string, time.Duration, int, error) {},
		true,
	)
	require.NoError(t, err)

	student := testpkg.CreateTestStudent(t, db, "Audit", "Routing", "1a")
	after := *student
	consentedAt := time.Date(2026, time.September, 2, 8, 30, 0, 0, time.UTC)
	after.PhotoConsentGivenAt = &consentedAt

	err = factory.StudentConsents.RecordTransitions(
		testpkg.Ctx(t),
		student,
		&after,
		"tenant_portal",
		nil,
		consentedAt,
	)
	require.NoError(t, err)
	assert.Equal(t, 1, observedAppends)
}
