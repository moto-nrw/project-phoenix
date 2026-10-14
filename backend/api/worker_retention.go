package api

import (
	"context"
	"fmt"
	"log/slog"

	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/cleanupruns"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/services/scheduler"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	retentionStatusDays           = "status_day_retention"
	retentionParentMessages       = "parent_message_retention"
)

// Retention windows the moto team records per school (ADR 0048). The Settings
// Platform owns the keys; newWorkerRetention refuses a key it does not define.
const (
	settingStatusDayRetentionDays = "gdpr.status_day_retention_days"
)

var retentionSettingKeys = []string{
	settingStatusDayRetentionDays,
}

// retentionOwners are the owner capabilities whose data the nightly cleanup
// deletes, and the deletion log. Each owner resolves its own window.
type retentionOwners struct {
	deletionLog interface {
		LogRetention(ctx context.Context, kind string, deleted, retentionDays int) error
	}
	// settings resolves the retention windows of the school in context.
	settings interface {
		ResolveInt(ctx context.Context, key string) (int, error)
	}
	unregisteredTagScans interface {
		DeleteExpired(ctx context.Context) (deleted, retentionDays int, err error)
	}
	statusDays     carePlanModule.StudentStatusDayRetention
	parentMessages communicationModule.ParentMessageCleanup
}

func newWorkerRetention(owners retentionOwners, logger *slog.Logger) (workerRetention, error) {
	if owners.deletionLog == nil || owners.settings == nil {
		return workerRetention{}, fmt.Errorf("compose worker retention: deletion log and settings are required")
	}
	if err := settingsCompose.VerifyPreloadKeys(retentionSettingKeys); err != nil {
		return workerRetention{}, fmt.Errorf("compose worker retention: %w", err)
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
	if owners.statusDays != nil {
		statusDays := owners.statusDays
		steps = append(steps, scheduler.RetentionStep{
			Kind: retentionStatusDays,
			Run: func(ctx context.Context) (scheduler.RetentionOutcome, error) {
				days, err := retentionDays(ctx, owners.settings, settingStatusDayRetentionDays)
				if err != nil {
					return scheduler.RetentionOutcome{}, err
				}
				deleted, err := statusDays.DeleteStudentStatusDaysBefore(ctx, carePlanModule.Date(retentionCutoff(days)))
				return scheduler.RetentionOutcome{Deleted: deleted, RetentionDays: days}, err
			},
		})
	}
	if owners.parentMessages != nil {
		messages := owners.parentMessages
		steps = append(steps, scheduler.RetentionStep{
			Kind: retentionParentMessages,
			Run: func(ctx context.Context) (scheduler.RetentionOutcome, error) {
				result, err := messages.CleanupExpiredParentMessages(ctx)
				return scheduler.RetentionOutcome{Deleted: result.MessagesDeleted, RetentionDays: result.RetentionDays}, err
			},
		})
	}
	return steps
}

// retentionDays resolves a school's retention window. It never guesses: an
// unreadable or non-positive window fails the step, so nothing is deleted
// on a wrong period and the run is caught up once the setting is fixed.
func retentionDays(ctx context.Context, settings interface {
	ResolveInt(ctx context.Context, key string) (int, error)
}, key string) (int, error) {
	days, err := settings.ResolveInt(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", key, err)
	}
	if days <= 0 {
		return 0, fmt.Errorf("resolve %s: retention window must be positive, got %d", key, days)
	}
	return days, nil
}

// retentionCutoff is the first Berlin calendar day a window of days keeps:
// data dated before it is past its retention.
func retentionCutoff(days int) calendar.Date {
	return calendar.TodayDate().AddDays(-days)
}
