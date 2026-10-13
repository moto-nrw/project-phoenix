package services_test

import (
	"log/slog"
	"reflect"
	"testing"

	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The booking consistency audit the Worker runs is composed over the Care Plan
// the service graph retains on the repository factory (ADR 0049).
func TestNewFactoryRetainsCarePlanForBookingConsistencyAudit(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := services.NewRepositoryTestFactory(db, services.NewUnobservedTimetableDependencies(db))
	_, err := services.NewFactoryForTestsWithConfig(repos, db, slog.Default(), testFactoryConfig())
	require.NoError(t, err)

	bookings, err := services.NewBookingConsistencyAudit(db, services.MustNewPeopleDirectory(db), repos.CarePlan())
	require.NoError(t, err)
	audit := reflect.ValueOf(bookings)
	auditDate := reflect.New(audit.Type().In(1)).Elem()
	auditDate.SetString("2030-09-01")
	result := audit.Call([]reflect.Value{reflect.ValueOf(testpkg.Ctx(t)), auditDate})
	require.Nil(t, result[1].Interface())
}
