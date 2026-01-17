package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/service/active"
	"github.com/spf13/cobra"
)

func runCleanupPreview(_ *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithCleanupService()
	if err != nil {
		return err
	}
	defer ctx.Close()

	preview, err := ctx.CleanupService.PreviewCleanup(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get cleanup preview: %w", err)
	}

	printPreviewHeader(preview)

	if verbose {
		printStudentBreakdownWithTotal("Visits to Delete", preview.StudentVisitCounts)
	}

	return nil
}

func printPreviewHeader(preview *active.CleanupPreview) {
	fmt.Println("Data Retention Cleanup Preview")
	fmt.Println("==============================")
	fmt.Printf("Total visits to delete: %d\n", preview.TotalVisits)

	if preview.OldestVisit != nil {
		daysAgo := time.Since(*preview.OldestVisit).Hours() / 24
		fmt.Printf("Oldest visit: %s (%.0f days ago)\n",
			preview.OldestVisit.Format(dateFormat), daysAgo)
	}

	fmt.Printf(fmtStudentsAffected, len(preview.StudentVisitCounts))
}
