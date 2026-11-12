package httpintegration_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/modules/timetable/compose"
	"github.com/moto-ogs/project-phoenix/modules/timetable/ports"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestOperationCallerDatabaseFailureReachesDirectoryFallback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Operation", "CallerFailure")
	callerRows, err := api.NewUserContextTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	fallbackErr := errors.New("original directory failure")
	fakes := newTimetableOpsDeps()
	fakes.personService.accountErr = fallbackErr
	deps := fakes.dependencies()
	identity := &observedCallerStaff{source: callerRows.Caller()}
	deps.CallerStaff = identity
	operations, err := compose.NewOperations(deps, owners.TimetableOperationCaller{RequestSession: deps.Session})
	require.NoError(t, err)
	for _, table := range []string{"users.persons", "users.staff_school_memberships"} {
		t.Run(table, func(t *testing.T) {
			request := httptest.NewRequest("GET", "/", nil)
			testutil.WithClaims(t, testpkg.Claims{ID: int(account.ID)})(request)
			ctx := testpkg.WithRequestIdentityCache(request.Context())
			blocker, err := db.BeginTx(ctx, &sql.TxOptions{})
			require.NoError(t, err)
			defer func() { require.NoError(t, blocker.Rollback()) }()
			_, err = blocker.ExecContext(ctx, "LOCK TABLE "+table+" IN ACCESS EXCLUSIVE MODE")
			require.NoError(t, err)
			err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
				_, err := tx.ExecContext(txCtx, "SET LOCAL lock_timeout = '10ms'")
				require.NoError(t, err)
				_, err = operations.PlannedNow(txCtx, account.ID, false, calendar.NewDate(2026, 11, 6), time.Date(2026, time.November, 6, 12, 0, 0, 0, time.UTC), timetable.PlannedNowOptions{})
				return err
			})
			require.Equal(t, "55P03", testpkg.PostgresSQLState(identity.err))
			require.ErrorIs(t, err, fallbackErr)
		})
	}
}

type observedCallerStaff struct {
	source ports.OperationStaffIdentity
	err    error
}

func (s *observedCallerStaff) StaffID(ctx context.Context) (int64, error) {
	staffID, err := s.source.StaffID(ctx)
	s.err = err
	return staffID, err
}
