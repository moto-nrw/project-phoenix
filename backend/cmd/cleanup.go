package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"text/tabwriter"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/spf13/cobra"
)

const (
	flagDryRun          = "dry-run"
	flagDescShowDetails = "Show detailed information"
	flagDescDryRun      = "Show what would be cleaned without cleaning"
	fmtDuration         = "Duration: %s\n"
	fmtStudentsAffected = "Students affected: %d\n"
	fmtStatus           = "Status: %s\n"
)

func mustFprintf(output io.Writer, format string, args ...any) int {
	n, err := fmt.Fprintf(output, format, args...)
	if err != nil {
		panic(fmt.Errorf("write cleanup output: %w", err))
	}
	return n
}

func mustFprintln(output io.Writer, args ...any) int {
	n, err := fmt.Fprintln(output, args...)
	if err != nil {
		panic(fmt.Errorf("write cleanup output: %w", err))
	}
	return n
}

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
	cleanupTimetableCmd.Flags().Bool(flagDryRun, false, flagDescDryRun)
	cleanupTimetableCmd.Flags().BoolP("verbose", "v", false, flagDescShowDetails)
	cleanupTimetablePreviewCmd.Flags().BoolP("verbose", "v", false, flagDescShowDetails)
	cleanupTimetableStatsCmd.Flags().BoolP("verbose", "v", false, flagDescShowDetails)

	// Flags for time-tracking cleanup commands (Tranche 0b)
	cleanupTimeTrackingCmd.Flags().Bool(flagDryRun, false, flagDescDryRun)
	cleanupTimeTrackingCmd.Flags().BoolP("verbose", "v", false, flagDescShowDetails)
	cleanupTimeTrackingPreviewCmd.Flags().BoolP("verbose", "v", false, flagDescShowDetails)
	cleanupTimeTrackingStatsCmd.Flags().BoolP("verbose", "v", false, flagDescShowDetails)

	// Flags for cleanup visits command
	cleanupVisitsCmd.Flags().Bool(flagDryRun, false, "Show what would be deleted without deleting")
	cleanupVisitsCmd.Flags().BoolP("verbose", "v", false, "Show detailed logs")
	cleanupVisitsCmd.Flags().String("log-file", "", "Write logs to file")
	cleanupVisitsCmd.Flags().Int("batch-size", 100, "Number of students to process in each batch")

	// Flags for preview command
	cleanupPreviewCmd.Flags().BoolP("verbose", "v", false, flagDescShowDetails)

	// Flags for stats command
	cleanupStatsCmd.Flags().BoolP("verbose", "v", false, flagDescShowDetails)

	// Flags for attendance command
	cleanupAttendanceCmd.Flags().Bool(flagDryRun, false, flagDescDryRun)
	cleanupAttendanceCmd.Flags().BoolP("verbose", "v", false, flagDescShowDetails)

	// Flags for sessions command
	cleanupSessionsCmd.Flags().Bool(flagDryRun, false, flagDescDryRun)
	cleanupSessionsCmd.Flags().BoolP("verbose", "v", false, flagDescShowDetails)
	cleanupSessionsCmd.Flags().String("mode", "abandoned", "Cleanup mode: 'abandoned' (timeout-based) or 'daily' (end all sessions)")
	cleanupSessionsCmd.Flags().Duration("threshold", 2*time.Hour, "Threshold for abandoned session cleanup (only used with --mode=abandoned)")

	// Flags for supervisors command
	cleanupSupervisorsCmd.Flags().Bool(flagDryRun, false, flagDescDryRun)
	cleanupSupervisorsCmd.Flags().BoolP("verbose", "v", false, flagDescShowDetails)
}

func runCleanupVisits(cmd *cobra.Command, _ []string) error {
	logFile, _ := cmd.Flags().GetString("log-file")
	logger, cleanup, err := setupLogger(logFile, cmd.OutOrStdout())
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
	ctx.Output = cmd.OutOrStdout()
	ctx.Logger = logger

	dryRun, _ := cmd.Flags().GetBool(flagDryRun)
	verbose, _ := cmd.Flags().GetBool("verbose")
	if dryRun {
		return runVisitsDryRun(logger, ctx, verbose)
	}

	return runVisitsCleanup(logger, ctx, verbose)
}

func runVisitsDryRun(logger *log.Logger, ctx *cleanupContext, verbose bool) error {
	logger.Println("DRY RUN MODE - No data will be deleted")

	preview, err := ctx.CleanupService.PreviewCleanup(context.Background())
	if err != nil {
		return fmt.Errorf("failed to preview cleanup: %w", err)
	}

	mustFprintln(ctx.output(), "\nCleanup Preview:")
	mustFprintf(ctx.output(), "Total visits to delete: %d\n", preview.TotalVisits)
	if preview.OldestVisit != nil {
		mustFprintf(ctx.output(), "Oldest visit: %s\n", preview.OldestVisit.Format(dateTimeFormat))
	}
	mustFprintf(ctx.output(), fmtStudentsAffected, len(preview.StudentVisitCounts))

	if verbose {
		printStudentBreakdown(ctx.output(), "Per-student breakdown", "Visits to Delete", preview.StudentVisitCounts)
	}

	return nil
}

func runVisitsCleanup(logger *log.Logger, ctx *cleanupContext, verbose bool) error {
	result, err := ctx.CleanupService.CleanupExpiredVisits(context.Background())
	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	logVisitCleanupResult(logger, result, verbose)
	printVisitCleanupSummary(ctx.output(), result)

	return nil
}

func logVisitCleanupResult(logger *log.Logger, result *active.CleanupResult, verbose bool) {
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

func printVisitCleanupSummary(output io.Writer, result *active.CleanupResult) {
	duration := result.CompletedAt.Sub(result.StartedAt)

	mustFprintln(output, "\nCleanup Summary:")
	mustFprintf(output, fmtDuration, duration)
	mustFprintf(output, "Students processed: %d\n", result.StudentsProcessed)
	mustFprintf(output, "Records deleted: %d\n", result.RecordsDeleted)
	mustFprintf(output, fmtStatus, getStatusString(result.Success))

	if len(result.Errors) > 0 {
		mustFprintf(output, "Errors: %d\n", len(result.Errors))
	}
}

func runCleanupPreview(cmd *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithCleanupService()
	if err != nil {
		return err
	}
	defer ctx.Close()
	ctx.Output = cmd.OutOrStdout()

	preview, err := ctx.CleanupService.PreviewCleanup(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get cleanup preview: %w", err)
	}

	printPreviewHeader(ctx.output(), preview)

	verbose, _ := cmd.Flags().GetBool("verbose")
	if verbose {
		printStudentBreakdownWithTotal(ctx.output(), "Visits to Delete", preview.StudentVisitCounts)
	}

	return nil
}

func printPreviewHeader(output io.Writer, preview *active.CleanupPreview) {
	mustFprintln(output, "Data Retention Cleanup Preview")
	mustFprintln(output, "==============================")
	mustFprintf(output, "Total visits to delete: %d\n", preview.TotalVisits)

	if preview.OldestVisit != nil {
		daysAgo := time.Since(*preview.OldestVisit).Hours() / 24
		mustFprintf(output, "Oldest visit: %s (%.0f days ago)\n",
			preview.OldestVisit.Format(dateFormat), daysAgo)
	}

	mustFprintf(output, fmtStudentsAffected, len(preview.StudentVisitCounts))
}

func runCleanupStats(cmd *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithCleanupService()
	if err != nil {
		return err
	}
	defer ctx.Close()
	ctx.Output = cmd.OutOrStdout()

	stats, err := ctx.CleanupService.GetRetentionStatistics(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get retention statistics: %w", err)
	}

	printRetentionStats(ctx.output(), stats)

	verbose, _ := cmd.Flags().GetBool("verbose")
	if !verbose {
		return nil
	}

	printMonthlyBreakdownWithTotal(ctx.output(), "Expired visits by month", stats.ExpiredVisitsByMonth)
	printVerboseRecentDeletions(ctx)

	return nil
}

func printRetentionStats(output io.Writer, stats *active.RetentionStats) {
	mustFprintln(output, "Data Retention Statistics")
	mustFprintln(output, "========================")
	mustFprintf(output, "Total expired visits: %d\n", stats.TotalExpiredVisits)
	mustFprintf(output, fmtStudentsAffected, stats.StudentsAffected)

	if stats.OldestExpiredVisit != nil {
		daysAgo := time.Since(*stats.OldestExpiredVisit).Hours() / 24
		mustFprintf(output, "Oldest expired visit: %s (%.0f days ago)\n",
			stats.OldestExpiredVisit.Format(dateFormat), daysAgo)
	}
}

func printVerboseRecentDeletions(ctx *cleanupContext) {
	mustFprintln(ctx.output(), "\nRecent deletion activity:")

	deletions, err := queryRecentDeletions(context.Background(), ctx.DB)
	if err != nil || len(deletions) == 0 {
		return
	}

	printRecentDeletions(ctx.output(), deletions)
}

func getStatusString(success bool) string {
	if success {
		return "SUCCESS"
	}
	return "COMPLETED WITH ERRORS"
}

func runCleanupTokens(cmd *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithAuthCleanup()
	if err != nil {
		return err
	}
	defer ctx.Close()
	ctx.Output = cmd.OutOrStdout()

	count, err := countExpiredTokens(ctx)
	if err != nil {
		return fmt.Errorf("failed to count expired tokens: %w", err)
	}

	mustFprintf(ctx.output(), "Found %d expired tokens to clean up\n", count)

	if count == 0 {
		mustFprintln(ctx.output(), "No expired tokens to clean up")
		return nil
	}

	deletedCount, err := ctx.AuthCleanupService.CleanupExpiredTokens(context.Background())
	if err != nil {
		return fmt.Errorf("failed to delete expired tokens: %w", err)
	}

	mustFprintf(ctx.output(), "Successfully deleted %d expired tokens\n", deletedCount)
	return nil
}

func countExpiredTokens(ctx *cleanupContext) (int, error) {
	return ctx.DB.NewSelect().
		TableExpr("auth.tokens").
		Where("expiry < ?", time.Now()).
		Count(context.Background())
}

func runCleanupInvitations(cmd *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithInvitationCleanup()
	if err != nil {
		return err
	}
	defer ctx.Close()
	ctx.Output = cmd.OutOrStdout()

	if ctx.InvitationCleanupService == nil {
		mustFprintln(ctx.output(), "Invitation service is not available; nothing to clean.")
		return nil
	}

	count, err := ctx.InvitationCleanupService.CleanupExpiredInvitations(context.Background())
	if err != nil {
		return fmt.Errorf("failed to clean up invitations: %w", err)
	}

	mustFprintf(ctx.output(), "Invitation cleanup removed %d records\n", count)
	return nil
}

func runCleanupRateLimits(cmd *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithAuthCleanup()
	if err != nil {
		return err
	}
	defer ctx.Close()
	ctx.Output = cmd.OutOrStdout()

	count, err := ctx.AuthCleanupService.CleanupExpiredRateLimits(context.Background())
	if err != nil {
		return fmt.Errorf("failed to clean up password reset rate limits: %w", err)
	}

	mustFprintf(ctx.output(), "Password reset rate limit cleanup removed %d records\n", count)
	return nil
}

func runCleanupAttendance(cmd *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithCleanupService()
	if err != nil {
		return err
	}
	defer ctx.Close()
	ctx.Output = cmd.OutOrStdout()

	dryRun, _ := cmd.Flags().GetBool(flagDryRun)
	verbose, _ := cmd.Flags().GetBool("verbose")
	if dryRun {
		return runAttendanceDryRun(ctx, verbose)
	}

	return runAttendanceCleanup(ctx, verbose)
}

func runAttendanceDryRun(ctx *cleanupContext, verbose bool) error {
	mustFprintln(ctx.output(), "DRY RUN MODE - No data will be modified")

	preview, err := ctx.CleanupService.PreviewAttendanceCleanup(context.Background())
	if err != nil {
		return fmt.Errorf("failed to preview attendance cleanup: %w", err)
	}

	printAttendancePreviewHeader(ctx.output(), preview)

	if verbose {
		printStudentBreakdown(ctx.output(), "Per-student breakdown", "Stale Records", preview.StudentRecords)
		printDateBreakdown(ctx.output(), preview.RecordsByDate)
	}

	return nil
}

func printAttendancePreviewHeader(output io.Writer, preview *active.AttendanceCleanupPreview) {
	mustFprintln(output, "\nAttendance Cleanup Preview:")
	mustFprintf(output, "Total stale records: %d\n", preview.TotalRecords)

	if preview.OldestRecord != nil {
		daysAgo := preview.OldestRecord.DaysUntil(timezone.TodayDate())
		mustFprintf(output, "Oldest record: %s (%d days ago)\n",
			preview.OldestRecord.Format(dateFormat), daysAgo)
	}

	mustFprintf(output, fmtStudentsAffected, len(preview.StudentRecords))
}

func runAttendanceCleanup(ctx *cleanupContext, verbose bool) error {
	result, err := ctx.CleanupService.CleanupStaleAttendance(context.Background())
	if err != nil {
		return fmt.Errorf("attendance cleanup failed: %w", err)
	}

	printAttendanceCleanupSummary(ctx.output(), result, verbose)
	return nil
}

func printAttendanceCleanupSummary(output io.Writer, result *active.AttendanceCleanupResult, verbose bool) {
	duration := result.CompletedAt.Sub(result.StartedAt)

	mustFprintln(output, "\nAttendance Cleanup Summary:")
	mustFprintf(output, fmtDuration, duration)
	mustFprintf(output, "Records closed: %d\n", result.RecordsClosed)
	mustFprintf(output, fmtStudentsAffected, result.StudentsAffected)

	if result.OldestRecordDate != nil {
		mustFprintf(output, "Oldest record: %s\n", result.OldestRecordDate.Format(dateFormat))
	}

	mustFprintf(output, fmtStatus, getStatusString(result.Success))
	printErrorList(output, result.Errors, verbose)
}

func printErrorList(output io.Writer, errors []string, verbose bool) {
	if len(errors) == 0 {
		return
	}

	mustFprintf(output, "Errors: %d\n", len(errors))

	if !verbose {
		return
	}

	for _, errMsg := range errors {
		mustFprintf(output, "  - %s\n", errMsg)
	}
}

func runCleanupSessions(cmd *cobra.Command, _ []string) error {
	mode, _ := cmd.Flags().GetString("mode")
	threshold, _ := cmd.Flags().GetDuration("threshold")
	dryRun, _ := cmd.Flags().GetBool(flagDryRun)
	verbose, _ := cmd.Flags().GetBool("verbose")

	log.Printf("Starting session cleanup process (mode: %s)...", mode)

	ctx, err := newCleanupContextWithSessionCleanup()
	if err != nil {
		return err
	}
	defer ctx.Close()
	ctx.Output = cmd.OutOrStdout()
	ctx.Logger = log.Default()

	switch mode {
	case "abandoned":
		return runAbandonedSessionCleanup(ctx, threshold, dryRun)
	case "daily":
		return runDailySessionCleanup(ctx, dryRun, verbose)
	default:
		return fmt.Errorf("invalid mode: %s (must be 'abandoned' or 'daily')", mode)
	}
}

func runAbandonedSessionCleanup(ctx *cleanupContext, threshold time.Duration, dryRun bool) error {
	if dryRun {
		ctx.logger().Printf("DRY RUN MODE - Would clean up sessions abandoned for more than %v", threshold)
		return nil
	}

	count, err := ctx.SessionCleanupService.CleanupAbandonedSessions(context.Background(), threshold)
	if err != nil {
		return fmt.Errorf("abandoned session cleanup failed: %w", err)
	}

	printAbandonedSessionSummary(ctx.output(), threshold, count)
	return nil
}

func printAbandonedSessionSummary(output io.Writer, threshold time.Duration, count int) {
	mustFprintln(output, "\nAbandoned Session Cleanup Summary:")
	mustFprintf(output, "Threshold: %v\n", threshold)
	mustFprintf(output, "Sessions cleaned: %d\n", count)
	mustFprintln(output, "Status: SUCCESS")
}

func runDailySessionCleanup(ctx *cleanupContext, dryRun bool, verbose bool) error {
	if dryRun {
		ctx.logger().Println("DRY RUN MODE - Would end all active sessions")
		return nil
	}

	result, err := ctx.SessionCleanupService.EndDailySessions(context.Background())
	if err != nil {
		return fmt.Errorf("daily session cleanup failed: %w", err)
	}

	printDailySessionSummary(ctx.output(), result, verbose)
	return nil
}

func printDailySessionSummary(output io.Writer, result *active.DailySessionCleanupResult, verbose bool) {
	mustFprintln(output, "\nDaily Session Cleanup Summary:")
	mustFprintf(output, "Sessions ended: %d\n", result.SessionsEnded)
	mustFprintf(output, "Visits ended: %d\n", result.VisitsEnded)
	mustFprintf(output, "Supervisors ended: %d\n", result.SupervisorsEnded)
	mustFprintf(output, fmtStatus, getStatusString(result.Success))
	printErrorList(output, result.Errors, verbose)
}

func runCleanupSupervisors(cmd *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithCleanupService()
	if err != nil {
		return err
	}
	defer ctx.Close()
	ctx.Output = cmd.OutOrStdout()

	dryRun, _ := cmd.Flags().GetBool(flagDryRun)
	verbose, _ := cmd.Flags().GetBool("verbose")
	if dryRun {
		return runSupervisorsDryRun(ctx, verbose)
	}

	return runSupervisorsCleanup(ctx, verbose)
}

func runSupervisorsDryRun(ctx *cleanupContext, verbose bool) error {
	mustFprintln(ctx.output(), "DRY RUN MODE - No data will be modified")

	preview, err := ctx.CleanupService.PreviewSupervisorCleanup(context.Background())
	if err != nil {
		return fmt.Errorf("failed to preview supervisor cleanup: %w", err)
	}

	printSupervisorPreviewHeader(ctx.output(), preview)

	if verbose {
		printStaffBreakdown(ctx.output(), "Per-staff breakdown", "Stale Records", preview.StaffRecords)
		printDateBreakdown(ctx.output(), preview.RecordsByDate)
	}

	return nil
}

func printSupervisorPreviewHeader(output io.Writer, preview *active.SupervisorCleanupPreview) {
	mustFprintln(output, "\nSupervisor Cleanup Preview:")
	mustFprintf(output, "Total stale records: %d\n", preview.TotalRecords)

	if preview.OldestRecord != nil {
		daysAgo := preview.OldestRecord.DaysUntil(timezone.TodayDate())
		mustFprintf(output, "Oldest record: %s (%d days ago)\n",
			preview.OldestRecord.Format(dateFormat), daysAgo)
	}

	mustFprintf(output, "Staff affected: %d\n", len(preview.StaffRecords))
}

func runSupervisorsCleanup(ctx *cleanupContext, verbose bool) error {
	result, err := ctx.CleanupService.CleanupStaleSupervisors(context.Background())
	if err != nil {
		return fmt.Errorf("supervisor cleanup failed: %w", err)
	}

	printSupervisorCleanupSummary(ctx.output(), result, verbose)
	return nil
}

func printSupervisorCleanupSummary(output io.Writer, result *active.SupervisorCleanupResult, verbose bool) {
	duration := result.CompletedAt.Sub(result.StartedAt)

	mustFprintln(output, "\nSupervisor Cleanup Summary:")
	mustFprintf(output, fmtDuration, duration)
	mustFprintf(output, "Records closed: %d\n", result.RecordsClosed)
	mustFprintf(output, "Staff affected: %d\n", result.StaffAffected)

	if result.OldestRecordDate != nil {
		mustFprintf(output, "Oldest record: %s\n", result.OldestRecordDate.Format(dateFormat))
	}

	mustFprintf(output, fmtStatus, getStatusString(result.Success))
	printErrorList(output, result.Errors, verbose)
}

// printStaffBreakdown prints a table of staff IDs and their counts.
func printStaffBreakdown(output io.Writer, header string, countHeader string, data map[int64]int) {
	if len(data) == 0 {
		return
	}

	mustFprintf(output, "\n%s:\n", header)
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	_ = mustFprintf(w, "Staff ID\t%s\n", countHeader)

	for staffID, count := range data {
		_ = mustFprintf(w, "%d\t%d\n", staffID, count)
	}

	if err := w.Flush(); err != nil {
		log.Printf(errFlushWriter, err)
	}
}

// --- Timetable GDPR cleanup (WP-B14) ---

func runCleanupTimetable(cmd *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithTimetableCleanup()
	if err != nil {
		return err
	}
	defer ctx.Close()
	ctx.Output = cmd.OutOrStdout()

	dryRun, _ := cmd.Flags().GetBool(flagDryRun)
	verbose, _ := cmd.Flags().GetBool("verbose")
	if dryRun {
		return forEachTenantTimetablePreview(ctx)
	}
	return forEachTenantTimetableCleanup(ctx, verbose)
}

func runCleanupTimetablePreview(cmd *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithTimetableCleanup()
	if err != nil {
		return err
	}
	defer ctx.Close()
	ctx.Output = cmd.OutOrStdout()
	return forEachTenantTimetablePreview(ctx)
}

func runCleanupTimetableStats(cmd *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithTimetableCleanup()
	if err != nil {
		return err
	}
	defer ctx.Close()
	ctx.Output = cmd.OutOrStdout()
	return forEachTenantTimetableStats(ctx)
}

// forEachTenantTimetableCleanup runs the actual DELETE per tenant.
func forEachTenantTimetableCleanup(cc *cleanupContext, verbose bool) error {
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
			printTimetableCleanupLine(cc.output(), tenantID, result)
			return nil
		})
	if err != nil {
		return fmt.Errorf("list active tenants: %w", err)
	}

	mustFprintln(cc.output(), "\nTimetable Cleanup Summary")
	mustFprintln(cc.output(), "=========================")
	mustFprintf(cc.output(), "Tenants processed:    %d\n", tenantCount)
	mustFprintf(cc.output(), "Instances deleted:    %d\n", totalInstances)
	mustFprintf(cc.output(), "Exceptions deleted:   %d\n", totalExceptions)
	mustFprintf(cc.output(), "Students affected:    %d\n", totalStudents)
	mustFprintf(cc.output(), "Errors:               %d\n", len(errs))
	if verbose {
		for _, e := range errs {
			mustFprintf(cc.output(), "  - %s\n", e.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d tenant(s) failed; see output above", len(errs))
	}
	return nil
}

func forEachTenantTimetablePreview(cc *cleanupContext) error {
	mustFprintln(cc.output(), "DRY RUN MODE - No data will be deleted")
	mustFprintln(cc.output(), "\nTimetable Cleanup Preview")
	mustFprintln(cc.output(), "=========================")

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
			printTimetablePreviewLine(cc.output(), tenantID, p)
			return nil
		})
	if err != nil {
		return fmt.Errorf("list active tenants: %w", err)
	}

	mustFprintf(cc.output(), "\nTOTAL: %d instances, %d exceptions, %d students across all tenants\n",
		totalInstances, totalExceptions, totalStudents)
	return nil
}

func forEachTenantTimetableStats(cc *cleanupContext) error {
	mustFprintln(cc.output(), "Timetable Retention Statistics")
	mustFprintln(cc.output(), "==============================")

	err := forEachActiveTenant(context.Background(), cc, "cli-timetable-stats",
		func(txCtx context.Context, tenantID int64) error {
			stats, err := cc.TimetableCleanupService.GetStats(txCtx)
			if err != nil {
				return err
			}
			printTimetableStatsLine(cc.output(), tenantID, stats)
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
	ctx = tenant.WithUnitOfWork(ctx, cc.TenantRuntime)
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

func printTimetableCleanupLine(output io.Writer, tenantID int64, r *schedule.TimetableCleanupResult) {
	mustFprintf(output, "[tenant %d] instances=%d exceptions=%d students=%d retention=%dd cutoff=%s duration_ms=%d\n",
		tenantID, r.InstancesDeleted, r.ExceptionsDeleted, r.StudentsAffected,
		r.RetentionDays, r.CutoffDate.Format(dateFormat), r.DurationMS)
}

func printTimetablePreviewLine(output io.Writer, tenantID int64, p *schedule.TimetableCleanupPreview) {
	mustFprintf(output, "[tenant %d] would-delete instances=%d exceptions=%d students=%d retention=%dd cutoff=%s",
		tenantID, p.InstancesToDelete, p.ExceptionsToDelete, p.StudentsAffected,
		p.RetentionDays, p.CutoffDate.Format(dateFormat))
	if p.OldestInstance != nil {
		mustFprintf(output, " oldest_instance=%s", p.OldestInstance.Format(dateFormat))
	}
	if p.OldestException != nil {
		mustFprintf(output, " oldest_exception=%s", p.OldestException.Format(dateFormat))
	}
	mustFprintln(output)
}

func printTimetableStatsLine(output io.Writer, tenantID int64, s *schedule.TimetableCleanupStats) {
	mustFprintf(output, "[tenant %d] instances_total=%d exceptions_total=%d retention=%dd cutoff=%s",
		tenantID, s.TotalInstances, s.TotalExceptions,
		s.RetentionDays, s.CutoffDate.Format(dateFormat))
	if s.OldestInstance != nil {
		mustFprintf(output, " oldest_instance=%s", s.OldestInstance.Format(dateFormat))
	}
	if s.OldestException != nil {
		mustFprintf(output, " oldest_exception=%s", s.OldestException.Format(dateFormat))
	}
	mustFprintln(output)
}

// --- Time-tracking GDPR cleanup (Tranche 0b) -----------------------------
//
// Mirrors the timetable trio: top-level command toggles between cleanup and
// dry-run via the --dry-run flag, the "preview" subcommand is the explicit
// dry-run alias, and "stats" reads the current row counts. All three iterate
// the same tenant runtime seam the scheduler uses, so CLI and cron
// agree on tenant ordering and isolation.

func runCleanupTimeTracking(cmd *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithTimeTrackingCleanup()
	if err != nil {
		return err
	}
	defer ctx.Close()
	ctx.Output = cmd.OutOrStdout()
	dryRun, _ := cmd.Flags().GetBool(flagDryRun)
	verbose, _ := cmd.Flags().GetBool("verbose")
	if dryRun {
		return forEachTenantTimeTrackingPreview(ctx)
	}
	return forEachTenantTimeTrackingCleanup(ctx, verbose)
}

func runCleanupTimeTrackingPreview(cmd *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithTimeTrackingCleanup()
	if err != nil {
		return err
	}
	defer ctx.Close()
	ctx.Output = cmd.OutOrStdout()
	return forEachTenantTimeTrackingPreview(ctx)
}

func runCleanupTimeTrackingStats(cmd *cobra.Command, _ []string) error {
	ctx, err := newCleanupContextWithTimeTrackingCleanup()
	if err != nil {
		return err
	}
	defer ctx.Close()
	ctx.Output = cmd.OutOrStdout()
	return forEachTenantTimeTrackingStats(ctx)
}

// listActiveTenantIDsForCLI returns the IDs of all active, non-deleted tenants.
func listActiveTenantIDsForCLI(ctx context.Context, cc *cleanupContext) ([]int64, error) {
	ctx = tenant.WithUnitOfWork(ctx, cc.TenantRuntime)
	schools, err := cc.Schools.ListActiveSchools(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(schools))
	for _, school := range schools {
		ids = append(ids, school.ID)
	}
	return ids, nil
}

func forEachTenantTimeTrackingCleanup(cc *cleanupContext, verbose bool) error {
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
		printTimeTrackingCleanupLine(cc.output(), tenantID, result)
		return nil
	})
	if err != nil {
		return fmt.Errorf("list active tenants: %w", err)
	}

	mustFprintln(cc.output(), "\nTime-Tracking Cleanup Summary")
	mustFprintln(cc.output(), "=============================")
	mustFprintf(cc.output(), "Tenants processed:    %d\n", tenantCount)
	mustFprintf(cc.output(), "Sessions deleted:     %d\n", totalSessions)
	mustFprintf(cc.output(), "Absences deleted:     %d\n", totalAbsences)
	mustFprintf(cc.output(), "Staff affected:       %d\n", totalStaff)
	mustFprintf(cc.output(), "Errors:               %d\n", len(errs))
	if verbose {
		for _, e := range errs {
			mustFprintf(cc.output(), "  - %s\n", e.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d tenant(s) failed; see output above", len(errs))
	}
	return nil
}

func forEachTenantTimeTrackingPreview(cc *cleanupContext) error {
	mustFprintln(cc.output(), "DRY RUN MODE - No data will be deleted")
	mustFprintln(cc.output(), "\nTime-Tracking Cleanup Preview")
	mustFprintln(cc.output(), "=============================")

	totalSessions, totalAbsences, totalStaff := 0, 0, 0
	err := forEachActiveTenant(context.Background(), cc, "cli-time-tracking-preview", func(txCtx context.Context, tenantID int64) error {
		p, previewErr := cc.TimeTrackingCleanupService.PreviewExpiredTimeTrackingData(txCtx)
		if previewErr != nil {
			return previewErr
		}
		totalSessions += p.SessionsToDelete
		totalAbsences += p.AbsencesToDelete
		totalStaff += p.StaffAffected
		printTimeTrackingPreviewLine(cc.output(), tenantID, p)
		return nil
	})
	if err != nil {
		return fmt.Errorf("list active tenants: %w", err)
	}

	mustFprintf(cc.output(), "\nTOTAL: %d sessions, %d absences, %d staff across all tenants\n",
		totalSessions, totalAbsences, totalStaff)
	return nil
}

func forEachTenantTimeTrackingStats(cc *cleanupContext) error {
	mustFprintln(cc.output(), "Time-Tracking Retention Statistics")
	mustFprintln(cc.output(), "==================================")

	err := forEachActiveTenant(context.Background(), cc, "cli-time-tracking-stats", func(txCtx context.Context, tenantID int64) error {
		stats, statsErr := cc.TimeTrackingCleanupService.GetStats(txCtx)
		if statsErr != nil {
			return statsErr
		}
		printTimeTrackingStatsLine(cc.output(), tenantID, stats)
		return nil
	})
	if err != nil {
		return fmt.Errorf("list active tenants: %w", err)
	}
	return nil
}

func printTimeTrackingCleanupLine(output io.Writer, tenantID int64, r *active.TimeTrackingCleanupResult) {
	mustFprintf(output, "[tenant %d] sessions=%d absences=%d staff=%d retention=%dd cutoff=%s duration_ms=%d\n",
		tenantID, r.SessionsDeleted, r.AbsencesDeleted, r.StaffAffected,
		r.RetentionDays, r.CutoffDate.Format(dateFormat), r.DurationMS)
}

func printTimeTrackingPreviewLine(output io.Writer, tenantID int64, p *active.TimeTrackingCleanupPreview) {
	mustFprintf(output, "[tenant %d] would-delete sessions=%d absences=%d staff=%d retention=%dd cutoff=%s",
		tenantID, p.SessionsToDelete, p.AbsencesToDelete, p.StaffAffected,
		p.RetentionDays, p.CutoffDate.Format(dateFormat))
	if p.OldestSession != nil {
		mustFprintf(output, " oldest_session=%s", p.OldestSession.Format(dateFormat))
	}
	if p.OldestAbsence != nil {
		mustFprintf(output, " oldest_absence=%s", p.OldestAbsence.Format(dateFormat))
	}
	mustFprintln(output)
}

func printTimeTrackingStatsLine(output io.Writer, tenantID int64, s *active.TimeTrackingCleanupStats) {
	mustFprintf(output, "[tenant %d] sessions_total=%d absences_total=%d retention=%dd cutoff=%s",
		tenantID, s.TotalSessions, s.TotalAbsences,
		s.RetentionDays, s.CutoffDate.Format(dateFormat))
	if s.OldestSession != nil {
		mustFprintf(output, " oldest_session=%s", s.OldestSession.Format(dateFormat))
	}
	if s.OldestAbsence != nil {
		mustFprintf(output, " oldest_absence=%s", s.OldestAbsence.Format(dateFormat))
	}
	mustFprintln(output)
}
