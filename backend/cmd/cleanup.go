package cmd

import (
	"time"

	"github.com/spf13/cobra"
)

var (
	dryRun    bool
	verbose   bool
	batchSize int
)

const (
	flagDryRun          = "dry-run"
	flagDescShowDetails = "Show detailed information"
	fmtStudentsAffected = "Students affected: %d\n"
	fmtStatus           = "Status: %s\n"
)

// cleanupCmd represents the cleanup command
var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up expired data based on retention policies",
	Long: `Clean up expired data based on retention policies configured in privacy consents.
This command will delete visit records that are older than the configured retention period for each student.

Available subcommands: visits, preview, stats, tokens, invitations, rate-limits, attendance, sessions.`,
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

	// Flags for cleanup visits command
	cleanupVisitsCmd.Flags().BoolVar(&dryRun, flagDryRun, false, "Show what would be deleted without deleting")
	cleanupVisitsCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed logs")
	cleanupVisitsCmd.Flags().IntVar(&batchSize, "batch-size", 100, "Number of students to process in each batch")

	// Flags for preview command
	cleanupPreviewCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, flagDescShowDetails)

	// Flags for stats command
	cleanupStatsCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, flagDescShowDetails)

	// Flags for attendance command
	cleanupAttendanceCmd.Flags().BoolVar(&dryRun, flagDryRun, false, "Show what would be cleaned without cleaning")
	cleanupAttendanceCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, flagDescShowDetails)

	// Flags for sessions command
	cleanupSessionsCmd.Flags().BoolVar(&dryRun, flagDryRun, false, "Show what would be cleaned without cleaning")
	cleanupSessionsCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, flagDescShowDetails)
	cleanupSessionsCmd.Flags().String("mode", "abandoned", "Cleanup mode: 'abandoned' (timeout-based) or 'daily' (end all sessions)")
	cleanupSessionsCmd.Flags().Duration("threshold", 2*time.Hour, "Threshold for abandoned session cleanup (only used with --mode=abandoned)")
}
