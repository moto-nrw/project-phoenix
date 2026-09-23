package cmd

import (
	"context"
	"encoding/json"
	"errors"
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
		if report != nil {
			err = errors.Join(err, writeStaffOwnerReport(cmd.OutOrStdout(), report))
		}
		return err
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

// backfillReport is the shared shape of every storage backfill's per-tenant
// report: the JSON payload plus the Cutover verdict derived from it.
type backfillReport interface {
	Stable() bool
	Unstable() []int64
}

// writeBackfillReport prints the per-tenant checkpoints as JSON followed by the
// Cutover verdict. Unstable tenants make the command fail so deployment scripts
// cannot mistake an incomplete backfill for a finished one. label names the
// backfill in prose, command names its subcommand, and inspect lists the
// checkpoint fields that explain why a tenant is still unstable.
func writeBackfillReport(output io.Writer, label, command, inspect string, report backfillReport) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("write backfill report: %w", err)
	}
	if !report.Stable() {
		return fmt.Errorf("%s backfill is not stable for tenants %v; rerun `backfill %s` and inspect %s", label, report.Unstable(), command, inspect)
	}
	_, err := fmt.Fprintf(output, "%s backfill stable: every tenant reports equal counts and checksums\n", label)
	return err
}

func writeStaffOwnerReport(output io.Writer, report *migrations.StaffOwnerBackfillReport) error {
	return writeBackfillReport(output, "staff owner", "staff-owner", "rows_rejected and mismatch_count", report)
}

func writeStudentOwnerReport(output io.Writer, report *migrations.StudentOwnerBackfillReport) error {
	return writeBackfillReport(output, "student owner", "student-owner",
		"rows_rejected, mismatch_count, guardian_mismatch_count and care_state_mismatch_count", report)
}

func writeGuardianOwnerReport(output io.Writer, report *migrations.GuardianOwnerBackfillReport) error {
	return writeBackfillReport(output, "guardian owner", "guardian-owner",
		"rows_rejected, mismatch_count and guardian_access_mismatch_count", report)
}

func (root backfillRoot) guardianOwnerRun(cmd *cobra.Command, opts migrations.GuardianOwnerBackfillOptions) error {
	return root.run(cmd.Context(), func(ctx context.Context, db *bun.DB) error {
		opts.Logger = slog.Default().With("backfill", migrations.GuardianOwnerBackfillName)
		report, err := migrations.RunGuardianOwnerBackfill(ctx, db, opts)
		if report != nil {
			err = errors.Join(err, writeGuardianOwnerReport(cmd.OutOrStdout(), report))
		}
		return err
	})
}

func (root backfillRoot) guardianOwnerStatus(cmd *cobra.Command) error {
	return root.run(cmd.Context(), func(ctx context.Context, db *bun.DB) error {
		report, err := migrations.GuardianOwnerBackfillStatus(ctx, db)
		if err != nil {
			return err
		}
		return writeGuardianOwnerReport(cmd.OutOrStdout(), report)
	})
}

func (root backfillRoot) guardianOwnerReset(cmd *cobra.Command) error {
	return root.run(cmd.Context(), func(ctx context.Context, db *bun.DB) error {
		if err := migrations.ResetGuardianOwnerBackfill(ctx, db); err != nil {
			return err
		}
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "guardian owner backfill reset: target rows and checkpoints discarded; users.students_guardians untouched")
		return err
	})
}

func (root backfillRoot) studentOwnerRun(cmd *cobra.Command, opts migrations.StudentOwnerBackfillOptions) error {
	return root.run(cmd.Context(), func(ctx context.Context, db *bun.DB) error {
		opts.Logger = slog.Default().With("backfill", migrations.StudentOwnerBackfillName)
		report, err := migrations.RunStudentOwnerBackfill(ctx, db, opts)
		if report != nil {
			err = errors.Join(err, writeStudentOwnerReport(cmd.OutOrStdout(), report))
		}
		return err
	})
}

func (root backfillRoot) studentOwnerStatus(cmd *cobra.Command) error {
	return root.run(cmd.Context(), func(ctx context.Context, db *bun.DB) error {
		report, err := migrations.StudentOwnerBackfillStatus(ctx, db)
		if err != nil {
			return err
		}
		return writeStudentOwnerReport(cmd.OutOrStdout(), report)
	})
}

func (root backfillRoot) studentOwnerReset(cmd *cobra.Command) error {
	return root.run(cmd.Context(), func(ctx context.Context, db *bun.DB) error {
		if err := migrations.ResetStudentOwnerBackfill(ctx, db); err != nil {
			return err
		}
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "student owner backfill reset: target rows and checkpoints discarded; users.students untouched")
		return err
	})
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

var backfillStudentOwnerCmd = &cobra.Command{
	Use:   "student-owner",
	Short: "Backfill People, School Membership and Care Plan student storage from users.students (#2758)",
	Long: `Copy users.students into users.student_profiles, users.student_school_memberships and
users.student_care_profiles. Re-reads changed old rows until every tenant is stable, then prints
per-tenant counts, checksums, batch timings, retries and the final-delta checkpoint that Cutover
#2759 consumes. The legacy guardian columns and the sick/excused flags have no target: they are
reconciled against users.guardian_profiles / users.guardian_phone_numbers and
active.student_status_days and reported as guardian_mismatch_count / care_state_mismatch_count
instead of being dropped. Exits non-zero while any tenant is unstable.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		batchSize, err := cmd.Flags().GetInt(flagBackfillBatchSize)
		if err != nil {
			return err
		}
		maxPasses, err := cmd.Flags().GetInt(flagBackfillMaxPasses)
		if err != nil {
			return err
		}
		return defaultBackfillRoot.studentOwnerRun(cmd, migrations.StudentOwnerBackfillOptions{BatchSize: batchSize, MaxPasses: maxPasses})
	},
}

var backfillStudentOwnerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the persisted per-tenant checkpoints without copying",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return defaultBackfillRoot.studentOwnerStatus(cmd)
	},
}

var backfillStudentOwnerResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Discard target rows and checkpoints to restart from zero (refused after Cutover)",
	Long: `Truncate users.student_profiles, users.student_school_memberships and
users.student_care_profiles and delete the student-owner checkpoints. users.students is never
modified. The command refuses once users.students is no longer a base table, because the targets
are then authoritative.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return defaultBackfillRoot.studentOwnerReset(cmd)
	},
}

var backfillGuardianOwnerCmd = &cobra.Command{
	Use:   "guardian-owner",
	Short: "Backfill People, Care Plan and Identity guardian storage from users.students_guardians (#2755)",
	Long: `Copy users.students_guardians into users.student_guardian_relationships,
users.student_guardian_pickup_permissions and auth.guardian_student_access. Re-reads changed old rows
until every tenant is stable, then prints per-tenant counts, checksums, batch timings, retries and the
final-delta checkpoint that Cutover #2756 consumes. The Identity row binds the guardian's account as
users.guardian_profiles names it; a drifted binding is reported as guardian_access_mismatch_count and
closed by the next pass. Exits non-zero while any tenant is unstable.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		batchSize, err := cmd.Flags().GetInt(flagBackfillBatchSize)
		if err != nil {
			return err
		}
		maxPasses, err := cmd.Flags().GetInt(flagBackfillMaxPasses)
		if err != nil {
			return err
		}
		return defaultBackfillRoot.guardianOwnerRun(cmd, migrations.GuardianOwnerBackfillOptions{BatchSize: batchSize, MaxPasses: maxPasses})
	},
}

var backfillGuardianOwnerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the persisted per-tenant checkpoints without copying",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return defaultBackfillRoot.guardianOwnerStatus(cmd)
	},
}

var backfillGuardianOwnerResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Discard target rows and checkpoints to restart from zero (refused after Cutover)",
	Long: `Truncate users.student_guardian_relationships, users.student_guardian_pickup_permissions and
auth.guardian_student_access and delete the guardian-owner checkpoints. users.students_guardians is never
modified. The command refuses once Cutover #2756 has installed the compatibility mirror, because the
targets are then authoritative.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return defaultBackfillRoot.guardianOwnerReset(cmd)
	},
}

func init() {
	for _, cmd := range []*cobra.Command{backfillStaffOwnerCmd, backfillStudentOwnerCmd, backfillGuardianOwnerCmd} {
		cmd.Flags().Int(flagBackfillBatchSize, 0, "rows per batch transaction (default 500)")
		cmd.Flags().Int(flagBackfillMaxPasses, 0, "re-read passes per tenant before giving up on stability (default 5)")
	}
	RootCmd.AddCommand(backfillCmd)
	backfillCmd.AddCommand(backfillStaffOwnerCmd)
	backfillStaffOwnerCmd.AddCommand(backfillStaffOwnerStatusCmd)
	backfillStaffOwnerCmd.AddCommand(backfillStaffOwnerResetCmd)
	backfillCmd.AddCommand(backfillStudentOwnerCmd)
	backfillStudentOwnerCmd.AddCommand(backfillStudentOwnerStatusCmd)
	backfillStudentOwnerCmd.AddCommand(backfillStudentOwnerResetCmd)
	backfillCmd.AddCommand(backfillGuardianOwnerCmd)
	backfillGuardianOwnerCmd.AddCommand(backfillGuardianOwnerStatusCmd)
	backfillGuardianOwnerCmd.AddCommand(backfillGuardianOwnerResetCmd)
}
