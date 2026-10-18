package api

import (
	"log/slog"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The booking consistency audit the Worker runs is composed by the root over
// the Care Plan the service graph retains (ADR 0049, #4012).
func TestServiceGraphRetainsCarePlanForBookingConsistencyAudit(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	_, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), testServiceConfig())
	require.NoError(t, err)

	bookings, err := NewBookingConsistencyAudit(testpkg.NewDatabase(t, db), MustNewPeopleDirectory(testpkg.NewDatabase(t, db)), repos.CarePlan())
	require.NoError(t, err)
	audit := reflect.ValueOf(bookings)
	auditDate := reflect.New(audit.Type().In(1)).Elem()
	auditDate.SetString("2030-09-01")
	result := audit.Call([]reflect.Value{reflect.ValueOf(testpkg.Ctx(t)), auditDate})
	require.Nil(t, result[1].Interface())
}
