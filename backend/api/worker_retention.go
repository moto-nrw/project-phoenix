package api

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/cleanupruns"
	"github.com/moto-nrw/project-phoenix/services/scheduler"
)

// workerRetention is the retention capability of the nightly cleanup (#3950):
// the run over the owners' deletion steps and their deletion log, and the
// ledger of each school's last successful run.
type workerRetention struct {
	run  scheduler.RetentionRun
	runs scheduler.CleanupRunLedger
}

// Data kinds of the nightly retention steps. Each one is the deletion type
// the Audit Platform records for its step in audit.data_deletions.
const (
	retentionUnregisteredTagScans = "unregistered_tag_scan_retention"
)

// retentionOwners are the owner capabilities whose data the nightly cleanup
// deletes, and the deletion log. Each owner resolves its own window.
type retentionOwners struct {
	deletionLog interface {
		LogRetention(ctx context.Context, kind string, deleted, retentionDays int) error
	}
	unregisteredTagScans interface {
		DeleteExpired(ctx context.Context) (deleted, retentionDays int, err error)
	}
}

func newWorkerRetention(owners retentionOwners, logger *slog.Logger) (workerRetention, error) {
	if owners.deletionLog == nil {
		return workerRetention{}, fmt.Errorf("compose worker retention: deletion log is required")
	}
	deletions := owners.deletionLog
	run, err := scheduler.NewRetentionRun(retentionSteps(owners), func(ctx context.Context, kind string, outcome scheduler.RetentionOutcome) error {
		return deletions.LogRetention(ctx, kind, outcome.Deleted, outcome.RetentionDays)
	}, logger)
	if err != nil {
		return workerRetention{}, fmt.Errorf("compose worker retention: %w", err)
	}
	return workerRetention{run: run, runs: cleanupruns.NewStore()}, nil
}

// retentionSteps lists the deletions of the nightly cleanup of one school.
// A missing owner leaves its step out.
func retentionSteps(owners retentionOwners) []scheduler.RetentionStep {
	var steps []scheduler.RetentionStep
	if owners.unregisteredTagScans != nil {
		scans := owners.unregisteredTagScans
		steps = append(steps, scheduler.RetentionStep{
			Kind: retentionUnregisteredTagScans,
			Run: func(ctx context.Context) (scheduler.RetentionOutcome, error) {
				deleted, days, err := scans.DeleteExpired(ctx)
				return scheduler.RetentionOutcome{Deleted: deleted, RetentionDays: days}, err
			},
		})
	}
	return steps
}
