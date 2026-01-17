package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/service/active"
	"github.com/spf13/cobra"
)

func runCleanupStats(_ *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithCleanupService()
	if err != nil {
		return err
	}
	defer ctx.Close()

	stats, err := ctx.CleanupService.GetRetentionStatistics(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get retention statistics: %w", err)
	}

	printRetentionStats(stats)

	if !verbose {
		return nil
	}

	printMonthlyBreakdownWithTotal("Expired visits by month", stats.ExpiredVisitsByMonth)
	printVerboseRecentDeletions(ctx)

	return nil
}

func printRetentionStats(stats *active.RetentionStats) {
	fmt.Println("Data Retention Statistics")
	fmt.Println("========================")
	fmt.Printf("Total expired visits: %d\n", stats.TotalExpiredVisits)
	fmt.Printf(fmtStudentsAffected, stats.StudentsAffected)

	if stats.OldestExpiredVisit != nil {
		daysAgo := time.Since(*stats.OldestExpiredVisit).Hours() / 24
		fmt.Printf("Oldest expired visit: %s (%.0f days ago)\n",
			stats.OldestExpiredVisit.Format(dateFormat), daysAgo)
	}
}

func printVerboseRecentDeletions(ctx *cleanupContext) {
	fmt.Println("\nRecent deletion activity:")

	deletions, err := queryRecentDeletions(context.Background(), ctx.DB)
	if err != nil || len(deletions) == 0 {
		return
	}

	printRecentDeletions(deletions)
}
