package cmd

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/service/active"
	"github.com/spf13/cobra"
)

func runCleanupVisits(_ *cobra.Command, _ []string) error {
	logger.Logger.Info("Starting visit cleanup process...")

	ctx, err := newCleanupContextWithCleanupService()
	if err != nil {
		return err
	}
	defer ctx.Close()

	if dryRun {
		return runVisitsDryRun(ctx)
	}

	return runVisitsCleanup(ctx)
}

func runVisitsDryRun(ctx *cleanupContext) error {
	logger.Logger.Info("DRY RUN MODE - No data will be deleted")

	preview, err := ctx.CleanupService.PreviewCleanup(context.Background())
	if err != nil {
		return fmt.Errorf("failed to preview cleanup: %w", err)
	}

	fmt.Println("\nCleanup Preview:")
	fmt.Printf("Total visits to delete: %d\n", preview.TotalVisits)
	if preview.OldestVisit != nil {
		fmt.Printf("Oldest visit: %s\n", preview.OldestVisit.Format(dateTimeFormat))
	}
	fmt.Printf(fmtStudentsAffected, len(preview.StudentVisitCounts))

	if verbose {
		printStudentBreakdown("Per-student breakdown", "Visits to Delete", preview.StudentVisitCounts)
	}

	return nil
}

func runVisitsCleanup(ctx *cleanupContext) error {
	result, err := ctx.CleanupService.CleanupExpiredVisits(context.Background())
	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	logVisitCleanupResult(result)
	printVisitCleanupSummary(result)

	return nil
}

func logVisitCleanupResult(result *active.CleanupResult) {
	duration := result.CompletedAt.Sub(result.StartedAt)
	logger.Logger.WithFields(map[string]interface{}{
		"duration":           duration.String(),
		"students_processed": result.StudentsProcessed,
		"records_deleted":    result.RecordsDeleted,
	}).Info("Cleanup completed")

	if len(result.Errors) == 0 {
		return
	}

	logger.Logger.WithField("error_count", len(result.Errors)).Warn("Errors encountered during cleanup")
	if verbose {
		for _, e := range result.Errors {
			logger.Logger.WithFields(map[string]interface{}{
				"student_id": e.StudentID,
				"error":      e.Error,
			}).Warn("Student cleanup error")
		}
	}
}

func printVisitCleanupSummary(result *active.CleanupResult) {
	duration := result.CompletedAt.Sub(result.StartedAt)

	fmt.Println("\nCleanup Summary:")
	fmt.Printf("Duration: %s\n", duration)
	fmt.Printf("Students processed: %d\n", result.StudentsProcessed)
	fmt.Printf("Records deleted: %d\n", result.RecordsDeleted)
	fmt.Printf(fmtStatus, getStatusString(result.Success))

	if len(result.Errors) > 0 {
		fmt.Printf("Errors: %d\n", len(result.Errors))
	}
}
