package cmd

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"text/tabwriter"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/spf13/cobra"
	"github.com/uptrace/bun"
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

Available subcommands: visits, preview, stats, tokens, invitations, rate-limits, attendance, sessions, supervisors, timetable.`,
}

// cleanupTimetableCmd represents the timetable subcommand (WP-B14).
var cleanupTimetableCmd = &cobra.Command{
	Use:   "timetable",
	Short: "Delete expired timetable instances + exceptions (GDPR retention)",
	Long: `Delete schedule.activity_instances (CASCADE → instance_staff + instance_students)
and schedule.activity_exceptions older than gdpr.timetable_retention_days for each
active tenant. Writes per-student audit rows to audit.data_deletions before the
deletes; exceptions carry no PII and are slog-only.`,
	RunE: runCleanupTimetable,
}

// cleanupTimetablePreviewCmd shows what would be deleted without deleting.
var cleanupTimetablePreviewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Preview timetable cleanup without deleting",
	Long:  `Count activity_instances, activity_exceptions, and distinct affected students per tenant.`,
	RunE:  runCleanupTimetablePreview,
}

// cleanupTimeTrackingCmd is the Tranche-0b counterpart to the timetable
// cleanup. Deletes active.work_sessions + active.staff_absences older than
// the tenant-configured retention window (default 730 days). CASCADE removes
// the children (work_session_breaks, audit.work_session_edits).
var cleanupTimeTrackingCmd = &cobra.Command{
	Use:   "time-tracking",
	Short: "Apply GDPR retention to time-tracking tables",
	Long: `Deletes work sessions and absences older than the configured retention window for each tenant.

Retention is read from setting gdpr.time_tracking_retention_days. The data_cleanup_enabled toggle is NOT consulted here — that gate only applies to the automatic scheduler run. CLI invocations always execute.

Use --dry-run to preview without deleting.`,
	RunE: runCleanupTimeTracking,
}

var cleanupTimeTrackingPreviewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Preview time-tracking cleanup without deleting",
	RunE:  runCleanupTimeTrackingPreview,
}

var cleanupTimeTrackingStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show current time-tracking table sizes and oldest rows",
	RunE:  runCleanupTimeTrackingStats,
}

// cleanupTimetableStatsCmd shows table totals and oldest dates.
var cleanupTimetableStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show timetable retention statistics",
	Long:  `Total row counts and oldest timestamps across schedule.activity_instances and schedule.activity_exceptions per tenant.`,
	RunE:  runCleanupTimetableStats,
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
	cleanupCmd.AddCommand(cleanupTimetableCmd)
	cleanupTimetableCmd.AddCommand(cleanupTimetablePreviewCmd)
	cleanupTimetableCmd.AddCommand(cleanupTimetableStatsCmd)

	cleanupCmd.AddCommand(cleanupTimeTrackingCmd)
	cleanupTimeTrackingCmd.AddCommand(cleanupTimeTrackingPreviewCmd)
	cleanupTimeTrackingCmd.AddCommand(cleanupTimeTrackingStatsCmd)

	// Flags for timetable commands
	cleanupTimetableCmd.Flags().BoolVar(&dryRun, flagDryRun, false, flagDescDryRun)
	cleanupTimetableCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, flagDescShowDetails)
	cleanupTimetablePreviewCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, flagDescShowDetails)
	cleanupTimetableStatsCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, flagDescShowDetails)

	// Flags for time-tracking cleanup commands (Tranche 0b)
	cleanupTimeTrackingCmd.Flags().BoolVar(&dryRun, flagDryRun, false, flagDescDryRun)
	cleanupTimeTrackingCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, flagDescShowDetails)
	cleanupTimeTrackingPreviewCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, flagDescShowDetails)
	cleanupTimeTrackingStatsCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, flagDescShowDetails)

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
		daysAgo := preview.OldestRecord.DaysUntil(timezone.TodayDate())
		fmt.Printf("Oldest record: %s (%d days ago)\n",
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
		daysAgo := preview.OldestRecord.DaysUntil(timezone.TodayDate())
		fmt.Printf("Oldest record: %s (%d days ago)\n",
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

// --- Timetable GDPR cleanup (WP-B14) ---

func runCleanupTimetable(_ *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithTimetableCleanup()
	if err != nil {
		return err
	}
	defer ctx.Close()

	if dryRun {
		return forEachTenantTimetablePreview(ctx)
	}
	return forEachTenantTimetableCleanup(ctx)
}

func runCleanupTimetablePreview(_ *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithTimetableCleanup()
	if err != nil {
		return err
	}
	defer ctx.Close()
	return forEachTenantTimetablePreview(ctx)
}

func runCleanupTimetableStats(_ *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithTimetableCleanup()
	if err != nil {
		return err
	}
	defer ctx.Close()
	return forEachTenantTimetableStats(ctx)
}

// forEachTenantTimetableCleanup runs the actual DELETE per tenant.
func forEachTenantTimetableCleanup(cc *cleanupContext) error {
	totalInstances, totalExceptions, totalStudents := 0, 0, 0
	tenantCount := 0
	errs := make([]error, 0)

	err := forEachActiveTenant(context.Background(), cc, "cli-timetable-cleanup",
		func(txCtx context.Context, tenantID int64) error {
			tenantCount++
			result, err := cc.TimetableCleanupService.CleanupExpiredTimetableData(txCtx)
			if err != nil {
				errs = append(errs, fmt.Errorf("tenant %d: %w", tenantID, err))
				return err
			}
			totalInstances += result.InstancesDeleted
			totalExceptions += result.ExceptionsDeleted
			totalStudents += result.StudentsAffected
			printTimetableCleanupLine(tenantID, result)
			return nil
		})
	if err != nil {
		return fmt.Errorf("list active tenants: %w", err)
	}

	fmt.Println("\nTimetable Cleanup Summary")
	fmt.Println("=========================")
	fmt.Printf("Tenants processed:    %d\n", tenantCount)
	fmt.Printf("Instances deleted:    %d\n", totalInstances)
	fmt.Printf("Exceptions deleted:   %d\n", totalExceptions)
	fmt.Printf("Students affected:    %d\n", totalStudents)
	fmt.Printf("Errors:               %d\n", len(errs))
	if verbose {
		for _, e := range errs {
			fmt.Printf("  - %s\n", e.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d tenant(s) failed; see output above", len(errs))
	}
	return nil
}

func forEachTenantTimetablePreview(cc *cleanupContext) error {
	fmt.Println("DRY RUN MODE - No data will be deleted")
	fmt.Println("\nTimetable Cleanup Preview")
	fmt.Println("=========================")

	totalInstances, totalExceptions, totalStudents := 0, 0, 0
	err := forEachActiveTenant(context.Background(), cc, "cli-timetable-preview",
		func(txCtx context.Context, tenantID int64) error {
			p, err := cc.TimetableCleanupService.PreviewExpiredTimetableData(txCtx)
			if err != nil {
				return err
			}
			totalInstances += p.InstancesToDelete
			totalExceptions += p.ExceptionsToDelete
			totalStudents += p.StudentsAffected
			printTimetablePreviewLine(tenantID, p)
			return nil
		})
	if err != nil {
		return fmt.Errorf("list active tenants: %w", err)
	}

	fmt.Printf("\nTOTAL: %d instances, %d exceptions, %d students across all tenants\n",
		totalInstances, totalExceptions, totalStudents)
	return nil
}

func forEachTenantTimetableStats(cc *cleanupContext) error {
	fmt.Println("Timetable Retention Statistics")
	fmt.Println("==============================")

	err := forEachActiveTenant(context.Background(), cc, "cli-timetable-stats",
		func(txCtx context.Context, tenantID int64) error {
			stats, err := cc.TimetableCleanupService.GetStats(txCtx)
			if err != nil {
				return err
			}
			printTimetableStatsLine(tenantID, stats)
			return nil
		})
	if err != nil {
		return fmt.Errorf("list active tenants: %w", err)
	}
	return nil
}

func forEachActiveTenant(
	ctx context.Context,
	cc *cleanupContext,
	operation string,
	fn func(context.Context, int64) error,
) error {
	ctx = tenant.WithRuntime(ctx, cc.TenantRuntime)
	tenantIDs, err := listActiveTenantIDsForCLI(ctx, cc)
	if err != nil {
		return err
	}
	for _, rawID := range tenantIDs {
		id, idErr := tenant.NewTenantID(rawID)
		if idErr != nil {
			return fmt.Errorf("%s: %w", operation, idErr)
		}
		if tenantErr := tenant.WithinTenant(ctx, id, func(txCtx context.Context) error {
			return fn(txCtx, id.Int64())
		}); tenantErr != nil {
			slog.Error("tenant operation failed, continuing to next tenant",
				"entry_point", "worker",
				"operation", operation,
				"tenant_id", id.Int64(),
				"error", tenantErr,
			)
		}
	}
	return nil
}

func printTimetableCleanupLine(tenantID int64, r *schedule.TimetableCleanupResult) {
	fmt.Printf("[tenant %d] instances=%d exceptions=%d students=%d retention=%dd cutoff=%s duration_ms=%d\n",
		tenantID, r.InstancesDeleted, r.ExceptionsDeleted, r.StudentsAffected,
		r.RetentionDays, r.CutoffDate.Format(dateFormat), r.DurationMS)
}

func printTimetablePreviewLine(tenantID int64, p *schedule.TimetableCleanupPreview) {
	fmt.Printf("[tenant %d] would-delete instances=%d exceptions=%d students=%d retention=%dd cutoff=%s",
		tenantID, p.InstancesToDelete, p.ExceptionsToDelete, p.StudentsAffected,
		p.RetentionDays, p.CutoffDate.Format(dateFormat))
	if p.OldestInstance != nil {
		fmt.Printf(" oldest_instance=%s", p.OldestInstance.Format(dateFormat))
	}
	if p.OldestException != nil {
		fmt.Printf(" oldest_exception=%s", p.OldestException.Format(dateFormat))
	}
	fmt.Println()
}

func printTimetableStatsLine(tenantID int64, s *schedule.TimetableCleanupStats) {
	fmt.Printf("[tenant %d] instances_total=%d exceptions_total=%d retention=%dd cutoff=%s",
		tenantID, s.TotalInstances, s.TotalExceptions,
		s.RetentionDays, s.CutoffDate.Format(dateFormat))
	if s.OldestInstance != nil {
		fmt.Printf(" oldest_instance=%s", s.OldestInstance.Format(dateFormat))
	}
	if s.OldestException != nil {
		fmt.Printf(" oldest_exception=%s", s.OldestException.Format(dateFormat))
	}
	fmt.Println()
}

// --- Time-tracking GDPR cleanup (Tranche 0b) -----------------------------
//
// Mirrors the timetable trio: top-level command toggles between cleanup and
// dry-run via the --dry-run flag, the "preview" subcommand is the explicit
// dry-run alias, and "stats" reads the current row counts. All three iterate
// the same tenant runtime seam the scheduler uses, so CLI and cron
// agree on tenant ordering and isolation.

func runCleanupTimeTracking(_ *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithTimeTrackingCleanup()
	if err != nil {
		return err
	}
	defer ctx.Close()
	if dryRun {
		return forEachTenantTimeTrackingPreview(ctx)
	}
	return forEachTenantTimeTrackingCleanup(ctx)
}

func runCleanupTimeTrackingPreview(_ *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithTimeTrackingCleanup()
	if err != nil {
		return err
	}
	defer ctx.Close()
	return forEachTenantTimeTrackingPreview(ctx)
}

func runCleanupTimeTrackingStats(_ *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithTimeTrackingCleanup()
	if err != nil {
		return err
	}
	defer ctx.Close()
	return forEachTenantTimeTrackingStats(ctx)
}

// listActiveTenantIDsForCLI returns the IDs of all active, non-deleted tenants
// via a direct SQL query against platform.schools. We bypass
// the repository ListActive method on purpose: that path drives a bun.Relation() join
// which mis-resolves the search_path in the CLI context and produces
// "relation organizations does not exist". The same bug bites the timetable
// CLI today — fixing it cleanly is out of scope for Tranche 0b.
func listActiveTenantIDsForCLI(ctx context.Context, cc *cleanupContext) ([]int64, error) {
	ctx = tenant.WithRuntime(ctx, cc.TenantRuntime)
	var ids []int64
	err := tenant.WithAdminTx(ctx, cc.DB, func(txCtx context.Context, tx bun.Tx) error {
		rows, err := tx.QueryContext(txCtx, `
			SELECT id FROM platform.schools
			WHERE active = true AND deleted_at IS NULL
			ORDER BY name
		`)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	return ids, err
}

func forEachTenantTimeTrackingCleanup(cc *cleanupContext) error {
	totalSessions, totalAbsences, totalStaff := 0, 0, 0
	errs := make([]error, 0)
	tenantCount := 0
	err := forEachActiveTenant(context.Background(), cc, "cli-time-tracking-cleanup", func(txCtx context.Context, tenantID int64) error {
		tenantCount++
		result, cleanupErr := cc.TimeTrackingCleanupService.CleanupExpiredTimeTrackingData(txCtx)
		if cleanupErr != nil {
			errs = append(errs, fmt.Errorf("tenant %d: %w", tenantID, cleanupErr))
			return cleanupErr
		}
		totalSessions += result.SessionsDeleted
		totalAbsences += result.AbsencesDeleted
		totalStaff += result.StaffAffected
		printTimeTrackingCleanupLine(tenantID, result)
		return nil
	})
	if err != nil {
		return fmt.Errorf("list active tenants: %w", err)
	}

	fmt.Println("\nTime-Tracking Cleanup Summary")
	fmt.Println("=============================")
	fmt.Printf("Tenants processed:    %d\n", tenantCount)
	fmt.Printf("Sessions deleted:     %d\n", totalSessions)
	fmt.Printf("Absences deleted:     %d\n", totalAbsences)
	fmt.Printf("Staff affected:       %d\n", totalStaff)
	fmt.Printf("Errors:               %d\n", len(errs))
	if verbose {
		for _, e := range errs {
			fmt.Printf("  - %s\n", e.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d tenant(s) failed; see output above", len(errs))
	}
	return nil
}

func forEachTenantTimeTrackingPreview(cc *cleanupContext) error {
	fmt.Println("DRY RUN MODE - No data will be deleted")
	fmt.Println("\nTime-Tracking Cleanup Preview")
	fmt.Println("=============================")

	totalSessions, totalAbsences, totalStaff := 0, 0, 0
	err := forEachActiveTenant(context.Background(), cc, "cli-time-tracking-preview", func(txCtx context.Context, tenantID int64) error {
		p, previewErr := cc.TimeTrackingCleanupService.PreviewExpiredTimeTrackingData(txCtx)
		if previewErr != nil {
			return previewErr
		}
		totalSessions += p.SessionsToDelete
		totalAbsences += p.AbsencesToDelete
		totalStaff += p.StaffAffected
		printTimeTrackingPreviewLine(tenantID, p)
		return nil
	})
	if err != nil {
		return fmt.Errorf("list active tenants: %w", err)
	}

	fmt.Printf("\nTOTAL: %d sessions, %d absences, %d staff across all tenants\n",
		totalSessions, totalAbsences, totalStaff)
	return nil
}

func forEachTenantTimeTrackingStats(cc *cleanupContext) error {
	fmt.Println("Time-Tracking Retention Statistics")
	fmt.Println("==================================")

	err := forEachActiveTenant(context.Background(), cc, "cli-time-tracking-stats", func(txCtx context.Context, tenantID int64) error {
		stats, statsErr := cc.TimeTrackingCleanupService.GetStats(txCtx)
		if statsErr != nil {
			return statsErr
		}
		printTimeTrackingStatsLine(tenantID, stats)
		return nil
	})
	if err != nil {
		return fmt.Errorf("list active tenants: %w", err)
	}
	return nil
}

func printTimeTrackingCleanupLine(tenantID int64, r *active.TimeTrackingCleanupResult) {
	fmt.Printf("[tenant %d] sessions=%d absences=%d staff=%d retention=%dd cutoff=%s duration_ms=%d\n",
		tenantID, r.SessionsDeleted, r.AbsencesDeleted, r.StaffAffected,
		r.RetentionDays, r.CutoffDate.Format(dateFormat), r.DurationMS)
}

func printTimeTrackingPreviewLine(tenantID int64, p *active.TimeTrackingCleanupPreview) {
	fmt.Printf("[tenant %d] would-delete sessions=%d absences=%d staff=%d retention=%dd cutoff=%s",
		tenantID, p.SessionsToDelete, p.AbsencesToDelete, p.StaffAffected,
		p.RetentionDays, p.CutoffDate.Format(dateFormat))
	if p.OldestSession != nil {
		fmt.Printf(" oldest_session=%s", p.OldestSession.Format(dateFormat))
	}
	if p.OldestAbsence != nil {
		fmt.Printf(" oldest_absence=%s", p.OldestAbsence.Format(dateFormat))
	}
	fmt.Println()
}

func printTimeTrackingStatsLine(tenantID int64, s *active.TimeTrackingCleanupStats) {
	fmt.Printf("[tenant %d] sessions_total=%d absences_total=%d retention=%dd cutoff=%s",
		tenantID, s.TotalSessions, s.TotalAbsences,
		s.RetentionDays, s.CutoffDate.Format(dateFormat))
	if s.OldestSession != nil {
		fmt.Printf(" oldest_session=%s", s.OldestSession.Format(dateFormat))
	}
	if s.OldestAbsence != nil {
		fmt.Printf(" oldest_absence=%s", s.OldestAbsence.Format(dateFormat))
	}
	fmt.Println()
}
