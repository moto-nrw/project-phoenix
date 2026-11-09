package compose_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The public classifiers read a unit's error like the retry: on the
// operation's own result, never on an attached cleanup failure (#6316, PR
// #6380 table H). A failed savepoint rollback or RELEASE after it is
// diagnostics, as a failed outer ROLLBACK is.

type classification struct{ transient, constraint, unique bool }

func classify(err error) classification {
	return classification{
		transient:  transactionruntime.IsTransientDatabaseError(err),
		constraint: transactionruntime.IsConstraintViolation(err),
		unique:     transactionruntime.IsUniqueViolation(err),
	}
}

func TestClassifiersReadTheOperationErrorOnly(t *testing.T) {
	t.Parallel()
	rig := newRetryRig(t)
	cleanups := map[string]error{
		"bad_conn":  testpkg.InjectedFailure("bad_conn"),
		"08006":     testpkg.InjectedFailure("08006"),
		"23503":     testpkg.InjectedFailure("23503"),
		"23502":     testpkg.InjectedFailure("23502"),
		"23505":     testpkg.InjectedFailure("23505"),
		"text_conn": errors.New("driver: bad connection"),
		"text_fk":   errors.New(`ERROR: violates foreign key constraint "fk_x" (SQLSTATE=23503)`),
		"text_uq":   errors.New(`ERROR: duplicate key value violates unique constraint "uq_x" (SQLSTATE=23505)`),
	}
	mains := []struct {
		name string
		step step
		want classification
	}{
		{"no_sqlstate", failFirst("", 1), classification{}},
		{"23505", failFirst("23505", 1), classification{unique: true}},
		{"23503", failFirst("23503", 1), classification{constraint: true}},
		{"bad_conn", func(ctx context.Context, tx bun.Tx, _ int) error {
			_, err := tx.ExecContext(ctx, testpkg.ProbeStatement)
			return err
		}, classification{transient: true}},
	}
	sites := []testpkg.FaultSite{testpkg.FaultSavepointRestore, testpkg.FaultSavepointRelease, testpkg.FaultRollback}
	for _, main := range mains {
		for _, site := range sites {
			for name, cleanup := range cleanups {
				cell := fmt.Sprintf("main_%s/%s_%s", main.name, strings.ReplaceAll(string(site), " ", "_"), name)
				t.Run(cell, func(t *testing.T) {
					rig.faults.Arm(site, cleanup)
					if main.name == "bad_conn" {
						rig.faults.AlsoFail(testpkg.FaultProbeStatement, testpkg.InjectedFailure("bad_conn"))
					}
					inside := main.step
					ctx, cancel := context.WithTimeout(rig.ctx, 20*time.Second)
					defer cancel()
					_, err := rig.runOnce(t, ctx, site != testpkg.FaultRollback, inside)
					require.Error(t, err)
					require.True(t, rig.faults.Fired(), "every armed fault fired")
					got := classify(err)
					t.Logf("CELL H %s %+v", cell, got)
					require.Equal(t, main.want, got, "error: %v", err)
					if site != testpkg.FaultRollback {
						require.ErrorIs(t, err, tenant.ErrSavepointRollback)
					}
				})
			}
		}
	}
}

// runOnce runs one tenant unit without replay: the callback writes a probe
// row, then runs inside, in a savepoint when savepoint is set.
func (r *retryRig) runOnce(t *testing.T, ctx context.Context, savepoint bool, inside step) (retryOutcome, error) {
	t.Helper()
	_, err := r.super.ExecContext(context.Background(), `TRUNCATE public.retry_probe`)
	require.NoError(t, err)
	err = tenant.WithinTenant(ctx, r.tenant, func(ctx context.Context) error {
		tx, err := unitTx(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO public.retry_probe (attempt) VALUES (1)`); err != nil {
			return err
		}
		if !savepoint {
			return inside(ctx, tx, 1)
		}
		return tenant.WithSavepoint(ctx, func(ctx context.Context) error { return inside(ctx, tx, 1) })
	})
	var rows int
	require.NoError(t, r.super.NewRaw(`SELECT count(*) FROM public.retry_probe`).Scan(context.Background(), &rows))
	require.Zero(t, rows, "a failed unit commits nothing")
	return retryOutcome{failed: err != nil, attempts: 1, rows: rows}, err
}
