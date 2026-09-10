package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/moto-nrw/project-phoenix/database/migrations"
	"github.com/spf13/cobra"
	"github.com/uptrace/bun"
)

type presenceBackfillOptions struct {
	tenantID   int64
	allTenants bool
	batchSize  int
	maxBatches int
}

func newPresenceBackfillCommand(root migrateRoot) *cobra.Command {
	options := presenceBackfillOptions{}
	command := &cobra.Command{
		Use:   "presence-backfill [run|status|restart]",
		Short: "Backfill Presence targets while old Timetable rows remain authoritative",
		Long:  "Run resumable tenant batches, inspect checkpoints, or restart target-only data before cutover. No caller switch or dual write. Restart must not be used after #2762 cutover.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			action := "run"
			if len(args) != 0 {
				action = args[0]
			}
			if err := options.validate(action); err != nil {
				return err
			}
			return root.run(cmd.Context(), func(ctx context.Context, db *bun.DB) error {
				return runPresenceBackfillCommand(ctx, db, cmd.OutOrStdout(), action, options)
			})
		},
	}
	command.Flags().Int64Var(&options.tenantID, "tenant-id", 0, "School ID (required unless --all-tenants)")
	command.Flags().BoolVar(&options.allTenants, "all-tenants", false, "Process all schools in ID order; unavailable for restart")
	command.Flags().IntVar(&options.batchSize, "batch-size", 500, "Source rows per independently committed batch (1..10000)")
	command.Flags().IntVar(&options.maxBatches, "max-batches", 0, "Stop after this many batches per tenant with a nonzero exit; 0 runs until verified")
	return command
}

func (o presenceBackfillOptions) validate(action string) error {
	if action != "run" && action != "status" && action != "restart" {
		return fmt.Errorf("unknown Presence backfill action %q", action)
	}
	if o.tenantID < 0 || (o.tenantID > 0) == o.allTenants {
		return fmt.Errorf("select exactly one of --tenant-id (positive) or --all-tenants")
	}
	if action == "restart" && o.allTenants {
		return fmt.Errorf("restart requires a single --tenant-id")
	}
	if o.batchSize < 1 || o.batchSize > 10000 || o.maxBatches < 0 {
		return fmt.Errorf("batch size must be 1..10000 and max batches must be nonnegative")
	}
	return nil
}

func runPresenceBackfillCommand(ctx context.Context, db *bun.DB, output io.Writer, action string, options presenceBackfillOptions) error {
	ids := []int64{options.tenantID}
	if options.allTenants {
		var err error
		ids, err = migrations.PresenceBackfillTenants(ctx, db)
		if err != nil {
			return err
		}
	}
	for _, tenantID := range ids {
		if action == "restart" {
			if err := migrations.RestartPresenceBackfill(ctx, db, tenantID); err != nil {
				return err
			}
		}
		step := func(ctx context.Context) (migrations.PresenceBackfillReport, *migrations.PresenceBackfillTelemetry, error) {
			var report migrations.PresenceBackfillReport
			var err error
			if action == "run" {
				report, err = migrations.PresenceBackfillBatch(ctx, db, tenantID, options.batchSize)
			} else {
				report, err = migrations.PresenceBackfillStatus(ctx, db, tenantID)
			}
			if err != nil {
				return report, nil, err
			}
			// Source-age inspection scans the tenant. Keep ordinary copy batches
			// bounded; live aggregate metrics are available via status and at finish.
			if action == "run" && !report.Complete {
				return report, nil, nil
			}
			metrics, err := migrations.PresenceBackfillMetrics(ctx, db, tenantID)
			return report, &metrics, err
		}
		if action != "run" {
			report, metrics, err := step(ctx)
			if err != nil {
				return err
			}
			if err := encodePresenceProgress(output, report, metrics); err != nil {
				return err
			}
			continue
		}
		if err := writePresenceBackfillProgress(ctx, output, options.maxBatches, step); err != nil {
			return err
		}
	}
	return nil
}

type presenceBackfillStep func(context.Context) (migrations.PresenceBackfillReport, *migrations.PresenceBackfillTelemetry, error)

func writePresenceBackfillProgress(ctx context.Context, output io.Writer, maxBatches int, step presenceBackfillStep) error {
	for batch := 0; maxBatches == 0 || batch < maxBatches; batch++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		report, metrics, err := step(ctx)
		if err != nil {
			return err
		}
		if err := encodePresenceProgress(output, report, metrics); err != nil {
			return err
		}
		if report.Complete {
			return nil
		}
	}
	return fmt.Errorf("batch limit reached; checkpoint retained, rerun to resume")
}

func encodePresenceProgress(output io.Writer, report migrations.PresenceBackfillReport, metrics *migrations.PresenceBackfillTelemetry) error {
	return json.NewEncoder(output).Encode(struct {
		Checkpoint migrations.PresenceBackfillReport     `json:"checkpoint"`
		Metrics    *migrations.PresenceBackfillTelemetry `json:"metrics,omitempty"`
	}{report, metrics})
}

func init() { migrateCmd.AddCommand(newPresenceBackfillCommand(defaultMigrateRoot)) }
