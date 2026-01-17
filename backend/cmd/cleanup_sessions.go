package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/service/active"
	"github.com/spf13/cobra"
)

func runCleanupSessions(cmd *cobra.Command, _ []string) error {
	mode, _ := cmd.Flags().GetString("mode")
	threshold, _ := cmd.Flags().GetDuration("threshold")

	logger.Logger.WithField("mode", mode).Info("Starting session cleanup process")

	ctx, err := newCleanupContextWithServices()
	if err != nil {
		return err
	}
	defer ctx.Close()

	switch mode {
	case "abandoned":
		return runAbandonedSessionCleanup(ctx, threshold)
	case "daily":
		return runDailySessionCleanup(ctx)
	default:
		return fmt.Errorf("invalid mode: %s (must be 'abandoned' or 'daily')", mode)
	}
}

func runAbandonedSessionCleanup(ctx *cleanupContext, threshold time.Duration) error {
	if dryRun {
		logger.Logger.WithField("threshold", threshold.String()).Info("DRY RUN MODE - Would clean up abandoned sessions")
		return nil
	}

	count, err := ctx.ServiceFactory.Active.CleanupAbandonedSessions(context.Background(), threshold)
	if err != nil {
		return fmt.Errorf("abandoned session cleanup failed: %w", err)
	}

	printAbandonedSessionSummary(threshold, count)
	return nil
}

func printAbandonedSessionSummary(threshold time.Duration, count int) {
	fmt.Printf("\nAbandoned Session Cleanup Summary:\n")
	fmt.Printf("Threshold: %v\n", threshold)
	fmt.Printf("Sessions cleaned: %d\n", count)
	fmt.Printf("Status: SUCCESS\n")
}

func runDailySessionCleanup(ctx *cleanupContext) error {
	if dryRun {
		logger.Logger.Info("DRY RUN MODE - Would end all active sessions")
		return nil
	}

	result, err := ctx.ServiceFactory.Active.EndDailySessions(context.Background())
	if err != nil {
		return fmt.Errorf("daily session cleanup failed: %w", err)
	}

	printDailySessionSummary(result)
	return nil
}

func printDailySessionSummary(result *active.DailySessionCleanupResult) {
	fmt.Printf("\nDaily Session Cleanup Summary:\n")
	fmt.Printf("Sessions ended: %d\n", result.SessionsEnded)
	fmt.Printf("Visits ended: %d\n", result.VisitsEnded)
	fmt.Printf(fmtStatus, getStatusString(result.Success))
	printErrorList(result.Errors)
}
