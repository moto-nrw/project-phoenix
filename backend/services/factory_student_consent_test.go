package services_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/services"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFactoryStudentConsentUsesAuditRoutedRepository(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	carePlan, err := api.NewUnobservedCarePlan(testpkg.NewDatabase(t, db), api.NewUnobservedTimetableDependencies(testpkg.NewDatabase(t, db)))
	require.NoError(t, err)
	owners, err := api.NewOwnerCapabilitiesForTests(testpkg.NewDatabase(t, db))
	require.NoError(t, err)

	cfg := services.FactoryConfig{
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
	identity, err := api.IdentityRootForTests(testpkg.NewDatabase(t, db), cfg)
	require.NoError(t, err)

	people, err := owners.PeopleWiring(testpkg.NewDatabase(t, db), carePlan, slog.Default())
	require.NoError(t, err)
	timetable, err := owners.TimetableWiring(testpkg.NewDatabase(t, db), people, slog.Default())
	require.NoError(t, err)

	observedAppends := 0
	factory, err := services.NewFactoryWithConfig(
		carePlan,
		db,
		slog.Default(),
		cfg,
		api.UnboundTenantRuntime(),
		owners.Organizations,
		owners.Persons,
		owners.Groups,
		owners.Rooms,
		owners.Membership,
		owners.Calendar,
		owners.Timetable,
		owners.Appointments,
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
		owners.Workforce,
		owners.WorkforceWiring(testpkg.NewDatabase(t, db)),
		func(services.DataImportObservation) {},
		nil,
		services.FileStorageWiring{},
		identity,
		people,
		timetable,
		true,
	)
	require.NoError(t, err)

	fixture := testpkg.CreateTestStudent(t, db, "Audit", "Routing", "1a")
	student, err := api.NewUnboundStudentRepository(testpkg.NewDatabase(t, db)).FindByID(testpkg.Ctx(t), fixture.ID)
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
