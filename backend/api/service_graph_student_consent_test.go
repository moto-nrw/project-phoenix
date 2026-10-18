package api

import (
	"log/slog"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceGraphStudentConsentUsesAuditRoutedRepository(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	carePlan, err := NewUnobservedCarePlan(testpkg.NewDatabase(t, db), MustNewUnobservedTimetable(testpkg.NewDatabase(t, db)), MustNewPeopleDirectory(testpkg.NewDatabase(t, db)))
	require.NoError(t, err)
	owners, err := NewOwnerCapabilitiesForTests(testpkg.NewDatabase(t, db))
	require.NoError(t, err)

	cfg := ServiceConfig{
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
	identity, err := IdentityRootForTests(testpkg.NewDatabase(t, db), cfg)
	require.NoError(t, err)

	people, err := owners.PeopleWiring(testpkg.NewDatabase(t, db), carePlan, slog.Default())
	require.NoError(t, err)
	platform, err := owners.PlatformWiring(testpkg.NewDatabase(t, db), people)
	require.NoError(t, err)
	presence, err := owners.PresenceWiring(testpkg.NewDatabase(t, db), carePlan, people, slog.Default())
	require.NoError(t, err)
	timetable, err := owners.TimetableWiring(testpkg.NewDatabase(t, db), people, slog.Default())
	require.NoError(t, err)
	carePlanWiring, err := owners.CarePlanWiring(testpkg.NewDatabase(t, db), carePlan, people, identity.AuditActor, slog.Default())
	require.NoError(t, err)

	observedAppends := 0
	graph, err := newServiceGraph(serviceGraphInputs{
		DB: testpkg.NewDatabase(t, db), Logger: slog.Default(), Config: cfg, AllowAuditRootWrites: true,
		CarePlan: carePlan, Organizations: owners.Organizations, Persons: owners.Persons, Groups: owners.Groups,
		Rooms: owners.Rooms, Membership: owners.Membership, Calendar: owners.Calendar, Timetable: owners.Timetable,
		Appointments: owners.Appointments, WorkTime: owners.Workforce,
		ObserveAuditAppend:     func(_ string, _ time.Duration, _ int, _ error) { observedAppends++ },
		ObserveDelivery:        func(string, string, string, time.Duration, error) {},
		ObserveDurableDelivery: func(string, string, string, time.Duration, int, error) {},
		ObserveDeviceFleet:     func(string, time.Duration, int64, int64, time.Duration, string, error) {},
		ObserveIdentityAccess:  func(string, time.Duration, int64, int64, time.Duration, string, error) {},
		Workforce:              owners.WorkforceWiring(testpkg.NewDatabase(t, db)), Identity: identity,
		People: people, Platform: platform, Presence: presence, TimetableWiring: timetable,
		CarePlanWiring: carePlanWiring, Enrollment: owners.EnrollmentWiring(testpkg.NewDatabase(t, db), carePlan, people),
	})
	require.NoError(t, err)

	fixture := testpkg.CreateTestStudent(t, db, "Audit", "Routing", "1a")
	student, err := NewUnboundStudentRepository(testpkg.NewDatabase(t, db)).FindByID(testpkg.Ctx(t), fixture.ID)
	require.NoError(t, err)
	after := *student
	consentedAt := time.Date(2026, time.September, 2, 8, 30, 0, 0, time.UTC)
	after.PhotoConsentGivenAt = &consentedAt

	err = graph.StudentConsents.RecordTransitions(
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
