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

var backfillStudentCompatibilityCmd = &cobra.Command{
	Use:   "compatibility",
	Short: "Repair or verify the rollback compatibility of users.students after Cutover (#2759)",
	Long: `Restore the rollback-only users.students view, its routing and the repointed foreign keys from
the authoritative owner storage, and report what a previous image would read. It never writes
users.student_profiles, users.student_school_memberships or users.student_care_profiles and never
resets the hit counters. Prints JSON evidence — compatibility reads and writes, per-school student
counts, children hidden by a retired enrollment, broken care chains, unreachable archive rows and
any foreign key whose existing rows are still unconfirmed — and exits non-zero on remaining drift.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
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
		return defaultBackfillRoot.run(ctx, func(ctx context.Context, db *bun.DB) error {
			report, runErr := migrations.RepairStudentOwnerCompatibility(ctx, db,
				migrations.StudentCompatibilityOptions{TenantIDs: tenants, VerifyOnly: verifyOnly})
			return errors.Join(runErr, json.NewEncoder(cmd.OutOrStdout()).Encode(report))
		})
	},
}

func init() {
	backfillStudentCompatibilityCmd.Flags().Int64Slice("tenant", nil, "limit the report and repair to these school IDs")
	backfillStudentCompatibilityCmd.Flags().Bool("verify-only", false, "report the rollback shape without repairing definitions, constraints or archive rows")
	backfillStudentOwnerCmd.AddCommand(backfillStudentCompatibilityCmd)
}
