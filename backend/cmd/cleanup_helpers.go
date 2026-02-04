package cmd

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"text/tabwriter"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/uptrace/bun"
)

// Error message and format constants (S1192 fix - reduces string duplication)
const (
	errInitDB         = "failed to initialize database: %w"
	errCloseDB        = "failed to close database: %v"
	errServiceFactory = "failed to create service factory: %w"
	errFlushWriter    = "failed to flush writer: %v"

	// dateFormat is the standard date format used for display (Go reference time layout)
	dateFormat = "2006-01-02"
	// dateTimeFormat is the standard date-time format used for display
	dateTimeFormat = "2006-01-02 15:04:05"
)

// cleanupContext holds shared resources for cleanup commands.
// It provides a consistent way to initialize and close database connections
// and service factories across all cleanup subcommands.
// Note: Context should be passed as a parameter to methods that need it,
// rather than stored in the struct (per Go best practices).
type cleanupContext struct {
	DB             *bun.DB
	RepoFactory    *repositories.Factory
	ServiceFactory *services.Factory
	CleanupService active.CleanupService
}

// newCleanupContext initializes database and repository factory.
// The caller must call Close() when done.
func newCleanupContext() (*cleanupContext, error) {
	db, err := database.InitDB()
	if err != nil {
		return nil, fmt.Errorf(errInitDB, err)
	}

	repoFactory := repositories.NewFactory(db)

	return &cleanupContext{
		DB:          db,
		RepoFactory: repoFactory,
	}, nil
}

// newCleanupContextWithServices initializes database, repository factory, and service factory.
// Use this when you need access to services (Auth, Invitation, Active).
// The caller must call Close() when done.
func newCleanupContextWithServices() (*cleanupContext, error) {
	ctx, err := newCleanupContext()
	if err != nil {
		return nil, err
	}

	serviceFactory, err := services.NewFactory(ctx.RepoFactory, ctx.DB, slog.Default())
	if err != nil {
		ctx.Close()
		return nil, fmt.Errorf(errServiceFactory, err)
	}

	ctx.ServiceFactory = serviceFactory
	return ctx, nil
}

// newCleanupContextWithCleanupService initializes database and cleanup service.
// Use this for visit/attendance cleanup commands.
// The caller must call Close() when done.
func newCleanupContextWithCleanupService() (*cleanupContext, error) {
	ctx, err := newCleanupContext()
	if err != nil {
		return nil, err
	}

	ctx.CleanupService = active.NewCleanupService(
		ctx.RepoFactory.ActiveVisit,
		ctx.RepoFactory.PrivacyConsent,
		ctx.RepoFactory.DataDeletion,
		ctx.DB,
	)

	return ctx, nil
}

// Close releases database resources.
func (c *cleanupContext) Close() {
	if c.DB != nil {
		if err := c.DB.Close(); err != nil {
			log.Printf(errCloseDB, err)
		}
	}
}

// setupLogger creates a logger that writes to the specified file or stdout.
// Returns: logger, cleanup function (call when done), error.
func setupLogger(logFilePath string) (*log.Logger, func(), error) {
	if logFilePath == "" {
		// No cleanup needed for stdout logger - return no-op function
		return log.New(os.Stdout, "", log.LstdFlags), func() { /* no cleanup needed for stdout */ }, nil
	}

	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log file: %w", err)
	}

	cleanup := func() {
		if err := file.Close(); err != nil {
			log.Printf("failed to close log file: %v", err)
		}
	}

	return log.New(file, "", log.LstdFlags), cleanup, nil
}

// printStudentBreakdown prints a table of student IDs and their counts.
func printStudentBreakdown(header string, countHeader string, data map[int64]int) {
	if len(data) == 0 {
		return
	}

	fmt.Printf("\n%s:\n", header)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "Student ID\t%s\n", countHeader)

	for studentID, count := range data {
		_, _ = fmt.Fprintf(w, "%d\t%d\n", studentID, count)
	}

	if err := w.Flush(); err != nil {
		log.Printf(errFlushWriter, err)
	}
}

// printDateBreakdown prints a table of dates and their counts.
func printDateBreakdown(data map[string]int) {
	if len(data) == 0 {
		return
	}

	fmt.Println("\nPer-date breakdown:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "Date\tRecords")

	for date, count := range data {
		_, _ = fmt.Fprintf(w, "%s\t%d\n", date, count)
	}

	if err := w.Flush(); err != nil {
		log.Printf(errFlushWriter, err)
	}
}

// printStudentBreakdownWithTotal prints a table with student data and a total row.
func printStudentBreakdownWithTotal(countHeader string, data map[int64]int) {
	if len(data) == 0 {
		return
	}

	fmt.Println("\nPer-student breakdown:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	_, _ = fmt.Fprintf(w, "Student ID\t%s\t\n", countHeader)
	_, _ = fmt.Fprintln(w, "----------\t----------------\t")

	total := 0
	for studentID, count := range data {
		_, _ = fmt.Fprintf(w, "%d\t%d\t\n", studentID, count)
		total += count
	}

	_, _ = fmt.Fprintln(w, "----------\t----------------\t")
	_, _ = fmt.Fprintf(w, "TOTAL\t%d\t\n", total)

	if err := w.Flush(); err != nil {
		log.Printf(errFlushWriter, err)
	}
}

// printMonthlyBreakdownWithTotal prints a table with monthly data and totals.
func printMonthlyBreakdownWithTotal(header string, data map[string]int64) {
	if len(data) == 0 {
		return
	}

	fmt.Printf("\n%s:\n", header)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	_, _ = fmt.Fprintln(w, "Month\tExpired Visits\t")
	_, _ = fmt.Fprintln(w, "-------\t--------------\t")

	var total int64
	for month, count := range data {
		_, _ = fmt.Fprintf(w, "%s\t%d\t\n", month, count)
		total += count
	}

	_, _ = fmt.Fprintln(w, "-------\t--------------\t")
	_, _ = fmt.Fprintf(w, "TOTAL\t%d\t\n", total)

	if err := w.Flush(); err != nil {
		log.Printf(errFlushWriter, err)
	}
}

// printRecentDeletions prints a table of recent deletion activity.
func printRecentDeletions(deletions []recentDeletionRow) {
	if len(deletions) == 0 {
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	_, _ = fmt.Fprintln(w, "Date\tRecords Deleted\tStudents\t")
	_, _ = fmt.Fprintln(w, "----------\t---------------\t--------\t")

	for _, d := range deletions {
		_, _ = fmt.Fprintf(w, "%s\t%d\t%d\t\n", d.Date, d.RecordsDeleted, d.StudentCount)
	}

	if err := w.Flush(); err != nil {
		log.Printf(errFlushWriter, err)
	}
}

// recentDeletionRow represents a row of recent deletion activity.
type recentDeletionRow struct {
	Date           string `bun:"date"`
	RecordsDeleted int64  `bun:"records_deleted"`
	StudentCount   int64  `bun:"student_count"`
}

// queryRecentDeletions fetches recent deletion activity from the audit table.
func queryRecentDeletions(ctx context.Context, db *bun.DB) ([]recentDeletionRow, error) {
	var deletions []recentDeletionRow

	err := db.NewRaw(`
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
	`).Scan(ctx, &deletions)

	return deletions, err
}
