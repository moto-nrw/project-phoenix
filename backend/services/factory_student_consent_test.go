package services

import (
	"log/slog"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFactoryStudentConsentUsesAuditRoutedRepository(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	carePlan, err := NewUnobservedCarePlan(db, NewUnobservedTimetableDependencies(db))
	require.NoError(t, err)
	owners, err := newOwnerCapabilitiesForTests(db)
	require.NoError(t, err)

	cfg := FactoryConfig{
		JWTSecret:        "test-secret-must-be-at-least-32-chars-long-for-real",
		JWTExpiry:        15 * time.Minute,
		JWTRefreshExpiry: 24 * time.Hour,
		FrontendURL:      "http://localhost:3000",
		PublicAPIURL:     "http://localhost:8080",
		ParentsURL:       "http://parents.localhost:3000",
		SchoolURL:        "http://schule.localhost:3000",
		TenantDomain:     "localhost",
		OperatorHostname: "operator.localhost:3000",
	}
	tokens, err := testTokensFor(cfg)
	require.NoError(t, err)

	observedAppends := 0
	factory, err := NewFactoryWithConfig(
		carePlan,
		db,
		slog.Default(),
		cfg,
		unboundTenantRuntime(),
		owners.organizations,
		owners.persons,
		owners.groups,
		owners.rooms,
		owners.membership,
		owners.calendar,
		owners.timetable,
		owners.appointments,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		func(_ string, _ time.Duration, _ int, _ error) { observedAppends++ },
		func(string, string, string, time.Duration, error) {},
		func(string, string, string, time.Duration, int, error) {},
		func(string, time.Duration, int64, int64, time.Duration, string, error) {},
		func(string, time.Duration, int64, int64, time.Duration, string, error) {},
		owners.workforce,
		func(DataImportObservation) {},
		nil,
		FileStorageWiring{},
		tokens,
		true,
	)
	require.NoError(t, err)

	fixture := testpkg.CreateTestStudent(t, db, "Audit", "Routing", "1a")
	student, err := NewUnboundStudentRepository(db).FindByID(testpkg.Ctx(t), fixture.ID)
	require.NoError(t, err)
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
