package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/service/active"
	"github.com/spf13/cobra"
)

func runCleanupAttendance(_ *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithCleanupService()
	if err != nil {
		return err
	}
	defer ctx.Close()

	if dryRun {
		return runAttendanceDryRun(ctx)
	}

	return runAttendanceCleanup(ctx)
}

func runAttendanceDryRun(ctx *cleanupContext) error {
	fmt.Println("DRY RUN MODE - No data will be modified")

	preview, err := ctx.CleanupService.PreviewAttendanceCleanup(context.Background())
	if err != nil {
		return fmt.Errorf("failed to preview attendance cleanup: %w", err)
	}

	printAttendancePreviewHeader(preview)

	if verbose {
		printStudentBreakdown("Per-student breakdown", "Stale Records", preview.StudentRecords)
		printDateBreakdown(preview.RecordsByDate)
	}

	return nil
}

func printAttendancePreviewHeader(preview *active.AttendanceCleanupPreview) {
	fmt.Println("\nAttendance Cleanup Preview:")
	fmt.Printf("Total stale records: %d\n", preview.TotalRecords)

	if preview.OldestRecord != nil {
		daysAgo := time.Since(*preview.OldestRecord).Hours() / 24
		fmt.Printf("Oldest record: %s (%.0f days ago)\n",
			preview.OldestRecord.Format(dateFormat), daysAgo)
	}

	fmt.Printf(fmtStudentsAffected, len(preview.StudentRecords))
}

func runAttendanceCleanup(ctx *cleanupContext) error {
	result, err := ctx.CleanupService.CleanupStaleAttendance(context.Background())
	if err != nil {
		return fmt.Errorf("attendance cleanup failed: %w", err)
	}

	printAttendanceCleanupSummary(result)
	return nil
}

func printAttendanceCleanupSummary(result *active.AttendanceCleanupResult) {
	duration := result.CompletedAt.Sub(result.StartedAt)

	fmt.Println("\nAttendance Cleanup Summary:")
	fmt.Printf("Duration: %s\n", duration)
	fmt.Printf("Records closed: %d\n", result.RecordsClosed)
	fmt.Printf(fmtStudentsAffected, result.StudentsAffected)

	if result.OldestRecordDate != nil {
		fmt.Printf("Oldest record: %s\n", result.OldestRecordDate.Format(dateFormat))
	}

	fmt.Printf(fmtStatus, getStatusString(result.Success))
	printErrorList(result.Errors)
}

func printErrorList(errors []string) {
	if len(errors) == 0 {
		return
	}

	fmt.Printf("Errors: %d\n", len(errors))

	if !verbose {
		return
	}

	for _, errMsg := range errors {
		fmt.Printf("  - %s\n", errMsg)
	}
}
