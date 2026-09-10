package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/database/migrations"
	"github.com/spf13/cobra"
	"github.com/uptrace/bun"
)

const (
	flagBackfillBatchSize = "batch-size"
	flagBackfillMaxPasses = "max-passes"
)

// backfillRoot injects the superuser database and output so the resumable
// storage backfills of the architecture migration (#2580) can be driven from
// the CLI and tested without a real connection.
type backfillRoot struct {
	// openDatabase returns the pool and a release function; tests hand in
	// the package pool with a no-op release instead of closing it.
	openDatabase func() (*bun.DB, func(), error)
}

var defaultBackfillRoot = backfillRoot{
	openDatabase: func() (*bun.DB, func(), error) {
		db, err := database.DBConn()
		if err != nil {
			return nil, nil, err
		}
		return db, func() { _ = db.Close() }, nil
	},
}

func (root backfillRoot) run(ctx context.Context, operation func(context.Context, *bun.DB) error) error {
	if root.openDatabase == nil {
		return fmt.Errorf("database opener is required")
	}
	db, release, err := root.openDatabase()
	if err != nil {
		return fmt.Errorf(errInitDB, err)
	}
	if db == nil {
		return fmt.Errorf("database opener returned nil")
	}
	if release != nil {
		defer release()
	}
	return operation(ctx, db)
}

func (root backfillRoot) staffOwnerRun(cmd *cobra.Command, opts migrations.StaffOwnerBackfillOptions) error {
	return root.run(cmd.Context(), func(ctx context.Context, db *bun.DB) error {
		opts.Logger = slog.Default().With("backfill", migrations.StaffOwnerBackfillName)
		report, err := migrations.RunStaffOwnerBackfill(ctx, db, opts)
		if err != nil {
			return err
		}
		return writeStaffOwnerReport(cmd.OutOrStdout(), report)
	})
}

func (root backfillRoot) staffOwnerStatus(cmd *cobra.Command) error {
	return root.run(cmd.Context(), func(ctx context.Context, db *bun.DB) error {
		report, err := migrations.StaffOwnerBackfillStatus(ctx, db)
		if err != nil {
			return err
		}
		return writeStaffOwnerReport(cmd.OutOrStdout(), report)
	})
}

func (root backfillRoot) staffOwnerReset(cmd *cobra.Command) error {
	return root.run(cmd.Context(), func(ctx context.Context, db *bun.DB) error {
		if err := migrations.ResetStaffOwnerBackfill(ctx, db); err != nil {
			return err
		}
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "staff owner backfill reset: target rows and checkpoints discarded; users.staff untouched")
		return err
	})
}

// writeStaffOwnerReport prints the per-tenant checkpoints as JSON followed by
// the Cutover verdict. Unstable tenants make the command fail so deployment
// scripts cannot mistake an incomplete backfill for a finished one.
func writeStaffOwnerReport(output io.Writer, report *migrations.StaffOwnerBackfillReport) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("write backfill report: %w", err)
	}
	if !report.Stable() {
		return fmt.Errorf("staff owner backfill is not stable for tenants %v; rerun `backfill staff-owner` and inspect rows_rejected and mismatch_count", report.Unstable())
	}
	_, err := fmt.Fprintln(output, "staff owner backfill stable: every tenant reports equal counts and checksums")
	return err
}

var backfillCmd = &cobra.Command{
	Use:   "backfill",
	Short: "Run resumable storage backfills of the architecture migration (#2580)",
	Long: `Copy old-table rows into new owner storage in tenant/id batches with a persisted high-water mark.
Each batch commits on its own; interrupting a run and starting it again resumes at the checkpoint.
The old tables stay authoritative until their Cutover ticket switches the callers.`,
}

var backfillStaffOwnerCmd = &cobra.Command{
	Use:   "staff-owner",
	Short: "Backfill Membership and Workforce staff storage from users.staff (#2752)",
	Long: `Copy users.staff into users.staff_school_memberships and users.staff_employment_profiles.
Re-reads changed old rows until every tenant is stable, then prints per-tenant counts, checksums,
batch timings, retries and the final-delta checkpoint that Cutover #2753 consumes.
Exits non-zero while any tenant is unstable.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		batchSize, err := cmd.Flags().GetInt(flagBackfillBatchSize)
		if err != nil {
			return err
		}
		maxPasses, err := cmd.Flags().GetInt(flagBackfillMaxPasses)
		if err != nil {
			return err
		}
		return defaultBackfillRoot.staffOwnerRun(cmd, migrations.StaffOwnerBackfillOptions{BatchSize: batchSize, MaxPasses: maxPasses})
	},
}

var backfillStaffOwnerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the persisted per-tenant checkpoints without copying",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return defaultBackfillRoot.staffOwnerStatus(cmd)
	},
}

var backfillStaffOwnerResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Discard target rows and checkpoints to restart from zero (refused after Cutover)",
	Long: `Truncate users.staff_school_memberships and users.staff_employment_profiles and delete the
staff-owner checkpoints. users.staff is never modified. The command refuses once users.staff is no
longer a base table, because the targets are then authoritative.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return defaultBackfillRoot.staffOwnerReset(cmd)
	},
}

func init() {
	backfillStaffOwnerCmd.Flags().Int(flagBackfillBatchSize, 0, "rows per batch transaction (default 500)")
	backfillStaffOwnerCmd.Flags().Int(flagBackfillMaxPasses, 0, "re-read passes per tenant before giving up on stability (default 5)")
	RootCmd.AddCommand(backfillCmd)
	backfillCmd.AddCommand(backfillStaffOwnerCmd)
	backfillStaffOwnerCmd.AddCommand(backfillStaffOwnerStatusCmd)
	backfillStaffOwnerCmd.AddCommand(backfillStaffOwnerResetCmd)
}
