package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type sweepSchools struct{ identityPorts.SchoolDirectory }
type sweepPersons struct{ PersonDirectory }
type sweepAudit struct{ identityPorts.AuthAudit }
type sweepPush struct{ PushSubscriptionCleanup }

var errRecordedAdminTx = errors.New("recorded administrative transaction")

// The unit of work passed in Dependencies is bound at construction: the
// session sweep's administrative follow-up opens its transaction under it,
// which without the binding fails for want of a runtime.
func TestNew_BindsTheTenantRuntimeDependency(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)

	adminTransactions := 0
	runtime, err := tenant.NewUnitOfWork(
		func(context.Context, int64, func(context.Context, any) error) error { return errRecordedAdminTx },
		func(context.Context, func(context.Context, any) error) error {
			adminTransactions++
			return errRecordedAdminTx
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)

	sweep := func(runtime tenant.UnitOfWork) {
		module, err := New(Dependencies{
			DB: db, Observe: func(Observation) {}, TenantRuntime: runtime,
			Sessions: &SessionDependencies{
				Schools: sweepSchools{}, Persons: sweepPersons{}, Passwords: application.UnusablePasswords{},
				Codec: application.UnsignedSessions{}, MFALock: application.UnlockedMFAPolicy{},
				Audit: sweepAudit{}, Push: sweepPush{},
			},
		})
		require.NoError(t, err)
		_, err = module.CleanupExpiredTokens(context.Background())
		require.NoError(t, err)
	}

	sweep(tenant.UnitOfWork{})
	require.Zero(t, adminTransactions, "a zero runtime binds nothing")
	sweep(runtime)
	require.Equal(t, 1, adminTransactions)
}
