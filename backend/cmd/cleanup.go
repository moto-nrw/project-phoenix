package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/spf13/cobra"
)

var (
	dryRun    bool
	verbose   bool
	logFile   string
	batchSize int
)

// cleanupCmd represents the cleanup command
var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up expired data based on retention policies",
	Long: `Clean up expired data based on retention policies configured in privacy consents.
This command will delete visit records that are older than the configured retention period for each student.`,
}

// cleanupVisitsCmd represents the visits subcommand
var cleanupVisitsCmd = &cobra.Command{
	Use:   "visits",
	Short: "Clean up expired visit records",
	Long: `Clean up expired visit records based on data retention settings in privacy consents.
Only completed visits (with exit_time set) that are older than the retention period will be deleted.
All deletions are logged in the audit.data_deletions table for GDPR compliance.`,
	RunE: runCleanupVisits,
}

// cleanupPreviewCmd shows what would be deleted
var cleanupPreviewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Preview what would be deleted without actually deleting",
	Long:  `Shows statistics about what data would be deleted if the cleanup command were run.`,
	RunE:  runCleanupPreview,
}

// cleanupStatsCmd shows retention statistics
var cleanupStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show data retention statistics",
	Long:  `Display statistics about expired data and retention policies.`,
	RunE:  runCleanupStats,
}

// cleanupTokensCmd represents the tokens subcommand
var cleanupTokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "Clean up expired authentication tokens",
	Long:  `Clean up expired refresh tokens from the database.
This helps maintain database performance and security by removing tokens that can no longer be used.`,
	RunE: runCleanupTokens,
}

func init() {
	RootCmd.AddCommand(cleanupCmd)
	cleanupCmd.AddCommand(cleanupVisitsCmd)
	cleanupCmd.AddCommand(cleanupPreviewCmd)
	cleanupCmd.AddCommand(cleanupStatsCmd)
	cleanupCmd.AddCommand(cleanupTokensCmd)

	// Flags for cleanup visits command
	cleanupVisitsCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be deleted without deleting")
	cleanupVisitsCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed logs")
	cleanupVisitsCmd.Flags().StringVar(&logFile, "log-file", "", "Write logs to file")
	cleanupVisitsCmd.Flags().IntVar(&batchSize, "batch-size", 100, "Number of students to process in each batch")

	// Flags for preview command
	cleanupPreviewCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed information")

	// Flags for stats command
	cleanupStatsCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed statistics")
}

func runCleanupVisits(cmd *cobra.Command, args []string) error {
	// Setup logging
	var logger *log.Logger
	if logFile != "" {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				log.Printf("failed to close log file: %v", err)
			}
		}()
		logger = log.New(file, "", log.LstdFlags)
	} else {
		logger = log.New(os.Stdout, "", log.LstdFlags)
	}

	logger.Println("Starting visit cleanup process...")

	// Initialize database
	db, err := database.InitDB()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	// Create repository factory
	repoFactory := repositories.NewFactory(db)

	// Create cleanup service
	cleanupService := active.NewCleanupService(
		repoFactory.ActiveVisit,
		repoFactory.PrivacyConsent,
		repoFactory.DataDeletion,
		db,
	)

	ctx := context.Background()

	// If dry run, show preview instead
	if dryRun {
		logger.Println("DRY RUN MODE - No data will be deleted")
		preview, err := cleanupService.PreviewCleanup(ctx)
		if err != nil {
			return fmt.Errorf("failed to preview cleanup: %w", err)
		}

		fmt.Println("\nCleanup Preview:")
		fmt.Printf("Total visits to delete: %d\n", preview.TotalVisits)
		if preview.OldestVisit != nil {
			fmt.Printf("Oldest visit: %s\n", preview.OldestVisit.Format("2006-01-02 15:04:05"))
		}
		fmt.Printf("Students affected: %d\n", len(preview.StudentVisitCounts))

		if verbose && len(preview.StudentVisitCounts) > 0 {
			fmt.Println("\nPer-student breakdown:")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "Student ID\tVisits to Delete")
			for studentID, count := range preview.StudentVisitCounts {
				_, _ = fmt.Fprintf(w, "%d\t%d\n", studentID, count)
			}
			if err := w.Flush(); err != nil {
				log.Printf("failed to flush writer: %v", err)
			}
		}

		return nil
	}

	// Run actual cleanup
	result, err := cleanupService.CleanupExpiredVisits(ctx)
	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	// Log results
	logger.Printf("Cleanup completed in %s\n", result.CompletedAt.Sub(result.StartedAt))
	logger.Printf("Students processed: %d\n", result.StudentsProcessed)
	logger.Printf("Records deleted: %d\n", result.RecordsDeleted)

	if len(result.Errors) > 0 {
		logger.Printf("Errors encountered: %d\n", len(result.Errors))
		if verbose {
			for _, err := range result.Errors {
				logger.Printf("  - Student %d: %s\n", err.StudentID, err.Error)
			}
		}
	}

	// Print summary
	fmt.Println("\nCleanup Summary:")
	fmt.Printf("Duration: %s\n", result.CompletedAt.Sub(result.StartedAt))
	fmt.Printf("Students processed: %d\n", result.StudentsProcessed)
	fmt.Printf("Records deleted: %d\n", result.RecordsDeleted)
	fmt.Printf("Status: %s\n", getStatusString(result.Success))

	if len(result.Errors) > 0 {
		fmt.Printf("Errors: %d\n", len(result.Errors))
	}

	return nil
}

func runCleanupPreview(cmd *cobra.Command, args []string) error {
	// Initialize database
	db, err := database.InitDB()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	// Create repository factory
	repoFactory := repositories.NewFactory(db)

	// Create cleanup service
	cleanupService := active.NewCleanupService(
		repoFactory.ActiveVisit,
		repoFactory.PrivacyConsent,
		repoFactory.DataDeletion,
		db,
	)

	ctx := context.Background()

	// Get preview
	preview, err := cleanupService.PreviewCleanup(ctx)
	if err != nil {
		return fmt.Errorf("failed to get cleanup preview: %w", err)
	}

	// Display preview
	fmt.Println("Data Retention Cleanup Preview")
	fmt.Println("==============================")
	fmt.Printf("Total visits to delete: %d\n", preview.TotalVisits)
	if preview.OldestVisit != nil {
		fmt.Printf("Oldest visit: %s (%.0f days ago)\n",
			preview.OldestVisit.Format("2006-01-02"),
			time.Since(*preview.OldestVisit).Hours()/24)
	}
	fmt.Printf("Students affected: %d\n", len(preview.StudentVisitCounts))

	if verbose && len(preview.StudentVisitCounts) > 0 {
		fmt.Println("\nPer-student breakdown:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
		_, _ = fmt.Fprintln(w, "Student ID\tVisits to Delete\t")
		_, _ = fmt.Fprintln(w, "----------\t----------------\t")

		total := 0
		for studentID, count := range preview.StudentVisitCounts {
			_, _ = fmt.Fprintf(w, "%d\t%d\t\n", studentID, count)
			total += count
		}
		_, _ = fmt.Fprintln(w, "----------\t----------------\t")
		_, _ = fmt.Fprintf(w, "TOTAL\t%d\t\n", total)
		if err := w.Flush(); err != nil {
			log.Printf("failed to flush writer: %v", err)
		}
	}

	return nil
}

func runCleanupStats(cmd *cobra.Command, args []string) error {
	// Initialize database
	db, err := database.InitDB()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	// Create repository factory
	repoFactory := repositories.NewFactory(db)

	// Create cleanup service
	cleanupService := active.NewCleanupService(
		repoFactory.ActiveVisit,
		repoFactory.PrivacyConsent,
		repoFactory.DataDeletion,
		db,
	)

	ctx := context.Background()

	// Get statistics
	stats, err := cleanupService.GetRetentionStatistics(ctx)
	if err != nil {
		return fmt.Errorf("failed to get retention statistics: %w", err)
	}

	// Display statistics
	fmt.Println("Data Retention Statistics")
	fmt.Println("========================")
	fmt.Printf("Total expired visits: %d\n", stats.TotalExpiredVisits)
	fmt.Printf("Students affected: %d\n", stats.StudentsAffected)

	if stats.OldestExpiredVisit != nil {
		fmt.Printf("Oldest expired visit: %s (%.0f days ago)\n",
			stats.OldestExpiredVisit.Format("2006-01-02"),
			time.Since(*stats.OldestExpiredVisit).Hours()/24)
	}

	if verbose && len(stats.ExpiredVisitsByMonth) > 0 {
		fmt.Println("\nExpired visits by month:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
		_, _ = fmt.Fprintln(w, "Month\tExpired Visits\t")
		_, _ = fmt.Fprintln(w, "-------\t--------------\t")

		total := int64(0)
		for month, count := range stats.ExpiredVisitsByMonth {
			_, _ = fmt.Fprintf(w, "%s\t%d\t\n", month, count)
			total += count
		}
		_, _ = fmt.Fprintln(w, "-------\t--------------\t")
		_, _ = fmt.Fprintf(w, "TOTAL\t%d\t\n", total)
		if err := w.Flush(); err != nil {
			log.Printf("failed to flush writer: %v", err)
		}
	}

	// Get historical deletion statistics
	if verbose {
		fmt.Println("\nRecent deletion activity:")

		// Query recent deletions
		var recentDeletions []struct {
			Date           string `bun:"date"`
			RecordsDeleted int64  `bun:"records_deleted"`
			StudentCount   int64  `bun:"student_count"`
		}

		err = db.NewRaw(`
			SELECT 
				TO_CHAR(deleted_at, 'YYYY-MM-DD') as date,
				SUM(records_deleted) as records_deleted,
				COUNT(DISTINCT student_id) as student_count
			FROM audit.data_deletions
			WHERE deletion_type = 'visit_retention'
				AND deleted_at >= NOW() - INTERVAL '30 days'
			GROUP BY TO_CHAR(deleted_at, 'YYYY-MM-DD')
			ORDER BY date DESC
			LIMIT 10
		`).Scan(ctx, &recentDeletions)

		if err == nil && len(recentDeletions) > 0 {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
			_, _ = fmt.Fprintln(w, "Date\tRecords Deleted\tStudents\t")
			_, _ = fmt.Fprintln(w, "----------\t---------------\t--------\t")

			for _, d := range recentDeletions {
				_, _ = fmt.Fprintf(w, "%s\t%d\t%d\t\n", d.Date, d.RecordsDeleted, d.StudentCount)
			}
			if err := w.Flush(); err != nil {
				log.Printf("failed to flush writer: %v", err)
			}
		}
	}

	return nil
}

func getStatusString(success bool) string {
	if success {
		return "SUCCESS"
	}
	return "COMPLETED WITH ERRORS"
}

func runCleanupTokens(cmd *cobra.Command, args []string) error {
	// Initialize database
	db, err := database.InitDB()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	// Create repository factory
	repoFactory := repositories.NewFactory(db)

	// Create service factory (no mailer needed for token cleanup)
	serviceFactory, err := services.NewFactory(repoFactory, db)
	if err != nil {
		return fmt.Errorf("failed to create service factory: %w", err)
	}
	authService := serviceFactory.Auth

	ctx := context.Background()

	// Get count of expired tokens first
	count, err := db.NewSelect().
		TableExpr("auth.tokens").
		Where("expiry < ?", time.Now()).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to count expired tokens: %w", err)
	}

	fmt.Printf("Found %d expired tokens to clean up\n", count)

	if count == 0 {
		fmt.Println("No expired tokens to clean up")
		return nil
	}

	// Delete expired tokens
	deletedCount, err := authService.CleanupExpiredTokens(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete expired tokens: %w", err)
	}

	fmt.Printf("Successfully deleted %d expired tokens\n", deletedCount)
	return nil
}
