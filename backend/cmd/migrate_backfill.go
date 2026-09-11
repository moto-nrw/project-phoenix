package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/moto-nrw/project-phoenix/database/migrations"
	"github.com/spf13/cobra"
	"github.com/uptrace/bun"
)

// migrateBackfillCmd groups resumable data backfills that migration 1.15.383
// and its successors run during deployment. The subcommands re-run the same
// batches by hand: to resume after a stop, to re-read changed legacy rows
// before Cutover, to refresh verification, or to restart from zero.
var migrateBackfillCmd = &cobra.Command{
	Use:   "backfill",
	Short: "run or verify resumable data backfills",
	Long:  `Re-run, verify, or restart the resumable tenant-batch backfills that populate Expand target tables from their legacy source.`,
}

var migrateBackfillRequestChildStorageCmd = &cobra.Command{
	Use:   "request-child-storage",
	Short: "copy enrollment.request_child_offerings into selections and bookings (#2713)",
	Long: `Copies enrollment.request_child_offerings into enrollment.request_child_offering_selections
and enrollment.care_offering_bookings in resumable per-tenant id batches, then verifies counts and
checksums. Legacy rows are only read. Ctrl-C or SIGTERM lets the running batch commit, then stops
before the next one; the checkpoint keeps every committed batch and the next run resumes there.

Exit status is non-zero when the run was stopped or any tenant did not reach equal counts and checksums.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		options, err := requestChildStorageOptionsFromFlags(cmd)
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return defaultMigrateRoot.run(ctx, func(ctx context.Context, db *bun.DB) error {
			return runRequestChildStorageBackfill(ctx, db, options, cmd.OutOrStdout())
		})
	},
}

func requestChildStorageOptionsFromFlags(cmd *cobra.Command) (migrations.RequestChildStorageBackfillOptions, error) {
	var options migrations.RequestChildStorageBackfillOptions
	var err error
	flags := cmd.Flags()
	if options.BatchSize, err = flags.GetInt("batch-size"); err != nil {
		return options, err
	}
	if options.TenantIDs, err = flags.GetInt64Slice("tenant"); err != nil {
		return options, err
	}
	if options.MaxPasses, err = flags.GetInt("max-passes"); err != nil {
		return options, err
	}
	if options.LockTimeout, err = flags.GetDuration("lock-timeout"); err != nil {
		return options, err
	}
	if options.VerifyOnly, err = flags.GetBool("verify-only"); err != nil {
		return options, err
	}
	if options.Restart, err = flags.GetBool("restart"); err != nil {
		return options, err
	}
	return options, nil
}

// runRequestChildStorageBackfill prints one evidence row per tenant after the
// run and fails when a tenant is incomplete, so operators and scripts see the
// same result the checkpoint table records.
func runRequestChildStorageBackfill(ctx context.Context, db *bun.DB, options migrations.RequestChildStorageBackfillOptions, output io.Writer) error {
	report, runErr := migrations.RunRequestChildStorageBackfill(ctx, db, options)
	if len(report.Tenants) > 0 {
		writeRequestChildStorageReport(output, report)
	}
	if errors.Is(runErr, migrations.ErrRequestChildStorageBackfillStopped) {
		mustFprintln(output, "Stopped at a batch boundary; every committed batch is checkpointed. Run the command again to resume.")
	}
	if runErr != nil {
		return runErr
	}
	if incomplete := report.IncompleteTenants(); len(incomplete) > 0 {
		return fmt.Errorf("request child storage backfill incomplete for tenants %v", incomplete)
	}
	mustFprintf(output, "All %d tenants report equal counts and checksums.\n", len(report.Tenants))
	return nil
}

func writeRequestChildStorageReport(output io.Writer, report migrations.RequestChildStorageReport) {
	table := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	mustFprintln(table, "tenant\tcomplete\thwm\tscanned\tcopied\tskipped\tdeleted\tbatches\tretried\tdeadlocks\tserialization\tlock_timeouts\tbatch_p95\tbatch_max\tpool_wait\tpasses\tmismatches\torphans\toldest_unmigrated")
	for _, tenant := range report.Tenants {
		v := tenant.Verification
		mustFprintf(table, "%d\t%t\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s\t%s\t%s\t%d\t%d\t%d\t%s\n",
			tenant.TenantID, tenant.Complete, tenant.HighWaterMark, tenant.RowsScanned, tenant.RowsCopied, tenant.RowsSkipped,
			tenant.RowsDeleted, tenant.BatchesCompleted, tenant.BatchesRetried, tenant.Deadlocks, tenant.SerializationFailures,
			tenant.LockTimeouts, tenant.BatchP95.Round(time.Millisecond), tenant.BatchMax.Round(time.Millisecond),
			tenant.PoolWait.Round(time.Millisecond), tenant.Passes, v.Mismatches(), v.Orphans, v.OldestUnmigrated.Round(time.Second))
	}
	if err := table.Flush(); err != nil {
		panic(fmt.Errorf("write backfill report: %w", err))
	}
	for _, tenant := range report.Tenants {
		mustFprintf(output, "Tenant %d provenance=%s day_differences=%d lock_wait=%s snapshot=%s\n",
			tenant.TenantID, tenant.Verification.ProvenancePolicy, tenant.Verification.DayDifferences,
			tenant.LockWait, tenant.Verification.Snapshot)
	}
	if report.DatabaseDeadlocksKnown {
		mustFprintf(output, "Database deadlocks during run: %d; duration %s\n", report.DatabaseDeadlocks, report.Duration.Round(time.Millisecond))
	} else {
		mustFprintf(output, "Database deadlocks during run: unavailable; duration %s\n", report.Duration.Round(time.Millisecond))
	}
}

func registerRequestChildStorageFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.Int("batch-size", 0, "legacy ids per committed batch (default 500)")
	flags.Int64Slice("tenant", nil, "limit the run to these school IDs (default: every school)")
	flags.Int("max-passes", 0, "reconcile passes before giving up on a changing tenant (default 5)")
	flags.Duration("lock-timeout", 0, "lock_timeout per batch transaction (default 5s)")
	flags.Bool("verify-only", false, "refresh verification and acceptance without modifying source or target rows")
	flags.Bool("restart", false, "truncate copied target rows and checkpoints, then copy from zero (pre-Cutover rollback)")
}

func init() {
	registerRequestChildStorageFlags(migrateBackfillRequestChildStorageCmd)
	migrateBackfillCmd.AddCommand(migrateBackfillRequestChildStorageCmd)
	migrateCmd.AddCommand(migrateBackfillCmd)
}
