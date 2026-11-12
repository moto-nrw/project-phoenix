package compose_test

import (
	"context"
	"database/sql"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// Force database/sql's automatic rollback to win before the callback returns.
// Commit then returns ErrTxDone rather than the ended context's own error.
func TestCommitAfterAutomaticContextRollback(t *testing.T) {
	t.Parallel()
	super := testpkg.SetupIsolatedTestDB(t)
	_, err := super.ExecContext(t.Context(), `CREATE TABLE public.cancelled_commit_probe (value text NOT NULL);
GRANT SELECT, INSERT ON public.cancelled_commit_probe TO phoenix_tenant, phoenix_admin`)
	require.NoError(t, err)
	for _, unit := range units {
		for _, expired := range []bool{false, true} {
			name := unit.name + "/cancelled"
			if expired {
				name = unit.name + "/deadline"
			}
			t.Run(name, func(t *testing.T) {
				db := testpkg.SetupClosableTestDB(t)
				t.Cleanup(func() { require.NoError(t, db.Close()) })
				require.NoError(t, db.PingContext(t.Context()))
				runtime, err := compose.NewTenantRuntime(db)
				require.NoError(t, err)
				schoolID := school(t, testpkg.Tenant(t))
				synctest.Test(t, func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					target := context.Canceled
					if expired {
						cancel()
						ctx, cancel = context.WithTimeout(context.Background(), time.Second)
						target = context.DeadlineExceeded
					}
					defer cancel()
					ctx = tenant.WithUnitOfWork(ctx, runtime)
					hookRan := false
					body := func(ctx context.Context) error {
						tenant.RegisterAfterCommit(ctx, func() { hookRan = true })
						tx, err := unitTx(ctx)
						require.NoError(t, err)
						_, err = tx.ExecContext(ctx, `INSERT INTO public.cancelled_commit_probe VALUES ('pending')`)
						require.NoError(t, err)
						if expired {
							time.Sleep(time.Second) // advance only the bubble's clock
						} else {
							cancel()
						}
						synctest.Wait()
						_, err = tx.ExecContext(context.Background(), `SELECT 1`)
						require.ErrorIs(t, err, sql.ErrTxDone, "automatic rollback finished before commit")
						return nil
					}
					if unit.admin {
						err = tenant.WithinAdmin(ctx, body)
					} else {
						err = tenant.WithinTenant(ctx, schoolID, body)
					}
					require.ErrorIs(t, err, target)
					if hookRan {
						t.Fatal("a cancelled transaction ran its after-commit hook")
					}
				})
				var rows int
				require.NoError(t, super.NewRaw(`SELECT count(*) FROM public.cancelled_commit_probe`).Scan(t.Context(), &rows))
				require.Zero(t, rows, "the automatic rollback discarded the write")
			})
		}
	}
}
