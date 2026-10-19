package api

import (
	"context"
	"log/slog"
	"reflect"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

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

	bookings, err := owners.NewBookingConsistencyAudit(testpkg.NewDatabase(t, db), owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, db)), repos.CarePlan())
	require.NoError(t, err)
	audit := reflect.ValueOf(bookings)
	auditDate := reflect.New(audit.Type().In(1)).Elem()
	auditDate.SetString("2030-09-01")
	require.NoError(t, tenant.NewTransactionRunner().RunInTx(testpkg.Ctx(t), func(ctx context.Context) error {
		result := audit.Call([]reflect.Value{reflect.ValueOf(ctx), auditDate})
		require.Nil(t, result[1].Interface())
		return nil
	}))
}
