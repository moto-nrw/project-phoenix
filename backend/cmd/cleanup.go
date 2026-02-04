package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/spf13/cobra"
)

var (
	dryRun    bool
	verbose   bool
	logFile   string
	batchSize int
)

const (
	flagDryRun          = "dry-run"
	flagDescShowDetails = "Show detailed information"
	flagDescDryRun      = "Show what would be cleaned without cleaning"
	fmtDuration         = "Duration: %s\n"
	fmtStudentsAffected = "Students affected: %d\n"
	fmtStatus           = "Status: %s\n"
)

// cleanupCmd represents the cleanup command
var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up expired data based on retention policies",
	Long: `Clean up expired data based on retention policies configured in privacy consents.
This command will delete visit records that are older than the configured retention period for each student.

Available subcommands: visits, preview, stats, tokens, invitations, rate-limits, attendance, sessions, supervisors.`,
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
	Long: `Clean up expired refresh tokens from the database.
This helps maintain database performance and security by removing tokens that can no longer be used.`,
	RunE: runCleanupTokens,
}

// cleanupInvitationsCmd represents the invitations subcommand
var cleanupInvitationsCmd = &cobra.Command{
	Use:   "invitations",
	Short: "Clean up expired or used invitation tokens",
	Long: `Clean up invitation tokens that are expired or already used.
This keeps the invitation table compact and ensures stale invitations are removed.`,
	RunE: runCleanupInvitations,
}

// cleanupRateLimitsCmd represents the rate-limits subcommand
var cleanupRateLimitsCmd = &cobra.Command{
	Use:   "rate-limits",
	Short: "Clean up expired password reset rate limit records",
	Long: `Remove password reset rate limit entries whose sliding window has expired.
This prevents the rate limit table from growing indefinitely.`,
	RunE: runCleanupRateLimits,
}

// cleanupAttendanceCmd represents the attendance subcommand
var cleanupAttendanceCmd = &cobra.Command{
	Use:   "attendance",
	Short: "Clean up stale attendance records from previous days",
	Long: `Clean up attendance records from previous days that don't have check-out times.
This fixes dashboard counting issues by closing attendance records that should have been checked out.
All cleanup actions are logged in the audit.data_deletions table for compliance.`,
	RunE: runCleanupAttendance,
}

// cleanupSessionsCmd represents the sessions subcommand
var cleanupSessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Clean up abandoned active sessions",
	Long: `Clean up abandoned active sessions and end daily sessions.
This command provides manual control over session cleanup that normally runs automatically.
It can clean up sessions that have exceeded their timeout or end all active sessions.`,
	RunE: runCleanupSessions,
}

// cleanupSupervisorsCmd represents the supervisors subcommand
var cleanupSupervisorsCmd = &cobra.Command{
	Use:   "supervisors",
	Short: "Clean up stale supervisor records from previous days",
	Long: `Clean up supervisor records from previous days that don't have end_date set.
This fixes supervisors showing as "Anwesend" after midnight by closing records that should have ended.
All cleanup actions are logged in the audit.data_deletions table for compliance.`,
	RunE: runCleanupSupervisors,
}

func init() {
	RootCmd.AddCommand(cleanupCmd)
	cleanupCmd.AddCommand(cleanupVisitsCmd)
	cleanupCmd.AddCommand(cleanupPreviewCmd)
	cleanupCmd.AddCommand(cleanupStatsCmd)
	cleanupCmd.AddCommand(cleanupTokensCmd)
	cleanupCmd.AddCommand(cleanupInvitationsCmd)
	cleanupCmd.AddCommand(cleanupRateLimitsCmd)
	cleanupCmd.AddCommand(cleanupAttendanceCmd)
	cleanupCmd.AddCommand(cleanupSessionsCmd)
	cleanupCmd.AddCommand(cleanupSupervisorsCmd)

	// Flags for cleanup visits command
	cleanupVisitsCmd.Flags().BoolVar(&dryRun, flagDryRun, false, "Show what would be deleted without deleting")
	cleanupVisitsCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed logs")
	cleanupVisitsCmd.Flags().StringVar(&logFile, "log-file", "", "Write logs to file")
	cleanupVisitsCmd.Flags().IntVar(&batchSize, "batch-size", 100, "Number of students to process in each batch")

	// Flags for preview command
	cleanupPreviewCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, flagDescShowDetails)

	// Flags for stats command
	cleanupStatsCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, flagDescShowDetails)

	// Flags for attendance command
	cleanupAttendanceCmd.Flags().BoolVar(&dryRun, flagDryRun, false, flagDescDryRun)
	cleanupAttendanceCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, flagDescShowDetails)

	// Flags for sessions command
	cleanupSessionsCmd.Flags().BoolVar(&dryRun, flagDryRun, false, flagDescDryRun)
	cleanupSessionsCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, flagDescShowDetails)
	cleanupSessionsCmd.Flags().String("mode", "abandoned", "Cleanup mode: 'abandoned' (timeout-based) or 'daily' (end all sessions)")
	cleanupSessionsCmd.Flags().Duration("threshold", 2*time.Hour, "Threshold for abandoned session cleanup (only used with --mode=abandoned)")

	// Flags for supervisors command
	cleanupSupervisorsCmd.Flags().BoolVar(&dryRun, flagDryRun, false, flagDescDryRun)
	cleanupSupervisorsCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, flagDescShowDetails)
}

func runCleanupVisits(_ *cobra.Command, _ []string) error {
	logger, cleanup, err := setupLogger(logFile)
	if err != nil {
		return err
	}
	defer cleanup()

	logger.Println("Starting visit cleanup process...")

	ctx, err := newCleanupContextWithCleanupService()
	if err != nil {
		return err
	}
	defer ctx.Close()

	if dryRun {
		return runVisitsDryRun(logger, ctx)
	}

	return runVisitsCleanup(logger, ctx)
}

func runVisitsDryRun(logger *log.Logger, ctx *cleanupContext) error {
	logger.Println("DRY RUN MODE - No data will be deleted")

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

func runVisitsCleanup(logger *log.Logger, ctx *cleanupContext) error {
	result, err := ctx.CleanupService.CleanupExpiredVisits(context.Background())
	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	logVisitCleanupResult(logger, result)
	printVisitCleanupSummary(result)

	return nil
}

func logVisitCleanupResult(logger *log.Logger, result *active.CleanupResult) {
	duration := result.CompletedAt.Sub(result.StartedAt)
	logger.Printf("Cleanup completed in %s\n", duration)
	logger.Printf("Students processed: %d\n", result.StudentsProcessed)
	logger.Printf("Records deleted: %d\n", result.RecordsDeleted)

	if len(result.Errors) == 0 {
		return
	}

	logger.Printf("Errors encountered: %d\n", len(result.Errors))
	if verbose {
		for _, e := range result.Errors {
			logger.Printf("  - Student %d: %s\n", e.StudentID, e.Error)
		}
	}
}

func printVisitCleanupSummary(result *active.CleanupResult) {
	duration := result.CompletedAt.Sub(result.StartedAt)

	fmt.Println("\nCleanup Summary:")
	fmt.Printf(fmtDuration, duration)
	fmt.Printf("Students processed: %d\n", result.StudentsProcessed)
	fmt.Printf("Records deleted: %d\n", result.RecordsDeleted)
	fmt.Printf(fmtStatus, getStatusString(result.Success))

	if len(result.Errors) > 0 {
		fmt.Printf("Errors: %d\n", len(result.Errors))
	}
}

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

func getStatusString(success bool) string {
	if success {
		return "SUCCESS"
	}
	return "COMPLETED WITH ERRORS"
}

func runCleanupTokens(_ *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithServices()
	if err != nil {
		return err
	}
	defer ctx.Close()

	count, err := countExpiredTokens(ctx)
	if err != nil {
		return fmt.Errorf("failed to count expired tokens: %w", err)
	}

	fmt.Printf("Found %d expired tokens to clean up\n", count)

	if count == 0 {
		fmt.Println("No expired tokens to clean up")
		return nil
	}

	deletedCount, err := ctx.ServiceFactory.Auth.CleanupExpiredTokens(context.Background())
	if err != nil {
		return fmt.Errorf("failed to delete expired tokens: %w", err)
	}

	fmt.Printf("Successfully deleted %d expired tokens\n", deletedCount)
	return nil
}

func countExpiredTokens(ctx *cleanupContext) (int, error) {
	return ctx.DB.NewSelect().
		TableExpr("auth.tokens").
		Where("expiry < ?", time.Now()).
		Count(context.Background())
}

func runCleanupInvitations(_ *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithServices()
	if err != nil {
		return err
	}
	defer ctx.Close()

	if ctx.ServiceFactory.Invitation == nil {
		fmt.Println("Invitation service is not available; nothing to clean.")
		return nil
	}

	count, err := ctx.ServiceFactory.Invitation.CleanupExpiredInvitations(context.Background())
	if err != nil {
		return fmt.Errorf("failed to clean up invitations: %w", err)
	}

	fmt.Printf("Invitation cleanup removed %d records\n", count)
	return nil
}

func runCleanupRateLimits(_ *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithServices()
	if err != nil {
		return err
	}
	defer ctx.Close()

	count, err := ctx.ServiceFactory.Auth.CleanupExpiredRateLimits(context.Background())
	if err != nil {
		return fmt.Errorf("failed to clean up password reset rate limits: %w", err)
	}

	fmt.Printf("Password reset rate limit cleanup removed %d records\n", count)
	return nil
}

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
	fmt.Printf(fmtDuration, duration)
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

func runCleanupSessions(cmd *cobra.Command, _ []string) error {
	mode, _ := cmd.Flags().GetString("mode")
	threshold, _ := cmd.Flags().GetDuration("threshold")

	log.Printf("Starting session cleanup process (mode: %s)...", mode)

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
		log.Printf("DRY RUN MODE - Would clean up sessions abandoned for more than %v", threshold)
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
		log.Println("DRY RUN MODE - Would end all active sessions")
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
	fmt.Printf("Supervisors ended: %d\n", result.SupervisorsEnded)
	fmt.Printf(fmtStatus, getStatusString(result.Success))
	printErrorList(result.Errors)
}

func runCleanupSupervisors(_ *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithCleanupService()
	if err != nil {
		return err
	}
	defer ctx.Close()

	if dryRun {
		return runSupervisorsDryRun(ctx)
	}

	return runSupervisorsCleanup(ctx)
}

func runSupervisorsDryRun(ctx *cleanupContext) error {
	fmt.Println("DRY RUN MODE - No data will be modified")

	preview, err := ctx.CleanupService.PreviewSupervisorCleanup(context.Background())
	if err != nil {
		return fmt.Errorf("failed to preview supervisor cleanup: %w", err)
	}

	printSupervisorPreviewHeader(preview)

	if verbose {
		printStaffBreakdown("Per-staff breakdown", "Stale Records", preview.StaffRecords)
		printDateBreakdown(preview.RecordsByDate)
	}

	return nil
}

func printSupervisorPreviewHeader(preview *active.SupervisorCleanupPreview) {
	fmt.Println("\nSupervisor Cleanup Preview:")
	fmt.Printf("Total stale records: %d\n", preview.TotalRecords)

	if preview.OldestRecord != nil {
		daysAgo := time.Since(*preview.OldestRecord).Hours() / 24
		fmt.Printf("Oldest record: %s (%.0f days ago)\n",
			preview.OldestRecord.Format(dateFormat), daysAgo)
	}

	fmt.Printf("Staff affected: %d\n", len(preview.StaffRecords))
}

func runSupervisorsCleanup(ctx *cleanupContext) error {
	result, err := ctx.CleanupService.CleanupStaleSupervisors(context.Background())
	if err != nil {
		return fmt.Errorf("supervisor cleanup failed: %w", err)
	}

	printSupervisorCleanupSummary(result)
	return nil
}

func printSupervisorCleanupSummary(result *active.SupervisorCleanupResult) {
	duration := result.CompletedAt.Sub(result.StartedAt)

	fmt.Println("\nSupervisor Cleanup Summary:")
	fmt.Printf(fmtDuration, duration)
	fmt.Printf("Records closed: %d\n", result.RecordsClosed)
	fmt.Printf("Staff affected: %d\n", result.StaffAffected)

	if result.OldestRecordDate != nil {
		fmt.Printf("Oldest record: %s\n", result.OldestRecordDate.Format(dateFormat))
	}

	fmt.Printf(fmtStatus, getStatusString(result.Success))
	printErrorList(result.Errors)
}

// printStaffBreakdown prints a table of staff IDs and their counts.
func printStaffBreakdown(header string, countHeader string, data map[int64]int) {
	if len(data) == 0 {
		return
	}

	fmt.Printf("\n%s:\n", header)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "Staff ID\t%s\n", countHeader)

	for staffID, count := range data {
		_, _ = fmt.Fprintf(w, "%d\t%d\n", staffID, count)
	}

	if err := w.Flush(); err != nil {
		log.Printf(errFlushWriter, err)
	}
}
