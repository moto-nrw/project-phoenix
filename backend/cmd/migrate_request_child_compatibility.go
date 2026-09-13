package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/moto-nrw/project-phoenix/database/migrations"
	"github.com/spf13/cobra"
	"github.com/uptrace/bun"
)

var migrateRequestChildCompatibilityCmd = &cobra.Command{
	Use:   "request-child-compatibility",
	Short: "repair or verify rollback compatibility after request-child cutover",
	Long:  "Resumes bounded repairs from authoritative targets to rollback-only metadata. Never changes submitted choices, effective bookings, submission notes, or hit counters. Prints JSON evidence and exits nonzero on remaining drift.",
	RunE: func(cmd *cobra.Command, _ []string) error {
		batchSize, err := cmd.Flags().GetInt("batch-size")
		if err != nil {
			return err
		}
		tenants, err := cmd.Flags().GetInt64Slice("tenant")
		if err != nil {
			return err
		}
		verifyOnly, err := cmd.Flags().GetBool("verify-only")
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return defaultMigrateRoot.run(ctx, func(ctx context.Context, db *bun.DB) error {
			report, runErr := migrations.RepairRequestChildStorageCompatibility(ctx, db, migrations.CompatibilityRepairOptions{BatchSize: batchSize, TenantIDs: tenants, VerifyOnly: verifyOnly})
			return errors.Join(runErr, json.NewEncoder(cmd.OutOrStdout()).Encode(report))
		})
	},
}

func init() {
	migrateRequestChildCompatibilityCmd.Flags().Int("batch-size", 500, "maximum rollback metadata rows per committed batch")
	migrateRequestChildCompatibilityCmd.Flags().Int64Slice("tenant", nil, "limit metadata repair and verification to these school IDs")
	migrateRequestChildCompatibilityCmd.Flags().Bool("verify-only", false, "verify the live compatibility view without repairing schema or metadata")
	migrateBackfillCmd.AddCommand(migrateRequestChildCompatibilityCmd)
}
