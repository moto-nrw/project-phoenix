package compose

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type sweepSchools struct {
	identityPorts.SchoolDirectory
}
type sweepPersons struct {
	identityPorts.PersonDirectory
}
type sweepPasswords struct {
	identityPorts.PasswordVerifier
}
type sweepCodec struct {
	identityPorts.TokenCodec
}
type sweepMFALock struct {
	identityPorts.MFAPolicyLock
}

// sweepAudit and sweepPush answer the session sweep's administrative
// follow-up with nothing to do.
type sweepAudit struct{ identityPorts.AuthAudit }

func (sweepAudit) ListPendingAccountWideWipes(context.Context) ([]identityaccess.PendingAccountWideWipe, error) {
	return nil, nil
}

type sweepPush struct {
	identityPorts.PushSubscriptionCleanup
}

func (sweepPush) DeleteOrphaned(context.Context) error { return nil }

// The unit of work passed in Dependencies is bound at construction: the
// session sweep opens every transaction under it. Without it the sweep
// fails before modifying tokens instead of performing an unscoped pool write.
func TestNew_BindsTheTenantRuntimeDependency(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)

	sweep := func(deps Dependencies) (string, error) {
		var logs bytes.Buffer
		deps.DB, deps.Observe = db, func(Observation) {}
		deps.Sessions = &identityPorts.SessionDependencies{
			PasswordLoginPolicy: func() (int, time.Duration, error) { return 5, time.Minute, nil },
			Schools:             sweepSchools{}, Persons: sweepPersons{}, Passwords: sweepPasswords{},
			Codec: sweepCodec{}, MFALock: sweepMFALock{}, Audit: sweepAudit{}, Push: sweepPush{}, PortalModules: sweepPortals{},
			Logger: slog.New(slog.NewTextHandler(&logs, nil)),
		}
		module, err := New(deps)
		require.NoError(t, err)
		_, err = module.CleanupExpiredTokens(context.Background())
		return logs.String(), err
	}

	logs, err := sweep(Dependencies{})
	require.ErrorContains(t, err, "runtime is required")
	require.Empty(t, logs, "no unscoped sweep or deferred reconciliation may run")
	logs, err = sweep(Dependencies{TenantRuntime: testpkg.TenantRuntime(t, db)})
	require.NoError(t, err)
	require.NotContains(t, logs, "reconciliation failed")
}

// sweepPortals stands in for the portal module booking; the sweep mints no
// session.
type sweepPortals struct{}

func (sweepPortals) BookedParentPortalSchools(_ context.Context, tenantIDs []int64) ([]int64, error) {
	return tenantIDs, nil
}

func (sweepPortals) RequireParentPortal(context.Context, int64) error { return nil }

func (sweepPortals) RequireSchoolPortal(context.Context, int64) error { return nil }
