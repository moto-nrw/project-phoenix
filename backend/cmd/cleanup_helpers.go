package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"reflect"
	"text/tabwriter"
	"time"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Error message and format constants (S1192 fix - reduces string duplication)
const (
	errInitDB      = "failed to initialize database: %w"
	errCloseDB     = "failed to close database: %v"
	errFlushWriter = "failed to flush writer: %v"

	// dateFormat is the standard date format used for display (Go reference time layout)
	dateFormat = "2006-01-02"
	// dateTimeFormat is the standard date-time format used for display
	dateTimeFormat = "2006-01-02 15:04:05"
)

// cleanupContext holds only the resources selected by one cleanup command.
// Note: Context should be passed as a parameter to methods that need it,
// rather than stored in the struct (per Go best practices).
type cleanupContext struct {
	DB                         *bun.DB
	CleanupService             active.CleanupService
	AuthCleanupService         authCleanupService
	InvitationCleanupService   invitationCleanupService
	SessionCleanupService      sessionCleanupService
	TimetableCleanupService    schedule.TimetableCleanupService
	TimeTrackingCleanupService active.TimeTrackingCleanupService
	TenantRuntime              tenant.UnitOfWork
	Output                     io.Writer
	Logger                     *log.Logger
}

type authCleanupService interface {
	CleanupExpiredTokens(context.Context) (int, error)
	CleanupExpiredRateLimits(context.Context) (int, error)
}

type invitationCleanupService interface {
	CleanupExpiredInvitations(context.Context) (int, error)
}

type sessionCleanupService interface {
	CleanupAbandonedSessions(context.Context, time.Duration) (int, error)
	EndDailySessions(context.Context) (*active.DailySessionCleanupResult, error)
}

type retentionCleanupService = active.CleanupService
type timetableCleanupService = schedule.TimetableCleanupService
type timeTrackingCleanupService = active.TimeTrackingCleanupService

type cleanupRoot struct {
	openDatabase        func() (*bun.DB, error)
	authCleanup         func(*cleanupContext) authCleanupService
	invitationCleanup   func(*cleanupContext) invitationCleanupService
	sessionCleanup      func(*cleanupContext) sessionCleanupService
	retentionCleanup    func(*cleanupContext) active.CleanupService
	timetableCleanup    func(*cleanupContext) schedule.TimetableCleanupService
	timeTrackingCleanup func(*cleanupContext) active.TimeTrackingCleanupService
}

var defaultCleanupRoot = cleanupRoot{
	openDatabase:        database.InitDB,
	authCleanup:         buildAuthCleanupService,
	invitationCleanup:   buildInvitationCleanupService,
	sessionCleanup:      buildSessionCleanupService,
	retentionCleanup:    buildRetentionCleanupService,
	timetableCleanup:    buildTimetableCleanupService,
	timeTrackingCleanup: buildTimeTrackingCleanupService,
}

func (root cleanupRoot) validateCapability(capability string) error {
	if root.openDatabase == nil {
		return fmt.Errorf("cleanup database opener is required")
	}
	missing := false
	switch capability {
	case "auth":
		missing = root.authCleanup == nil
	case "invitation":
		missing = root.invitationCleanup == nil
	case "session":
		missing = root.sessionCleanup == nil
	case "retention":
		missing = root.retentionCleanup == nil
	case "timetable":
		missing = root.timetableCleanup == nil
	case "time-tracking":
		missing = root.timeTrackingCleanup == nil
	default:
		return fmt.Errorf("unknown cleanup capability %q", capability)
	}
	if missing {
		return fmt.Errorf("%s cleanup service builder is required", capability)
	}
	return nil
}

func buildCleanupDependency[T any](ctx *cleanupContext, builder func(*cleanupContext) T, capability string) (T, error) {
	var zero T
	if builder == nil {
		return zero, fmt.Errorf("%s cleanup service builder is required", capability)
	}
	service := builder(ctx)
	value := reflect.ValueOf(service)
	if !value.IsValid() || ((value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) && value.IsNil()) {
		return zero, fmt.Errorf("%s cleanup service builder returned nil", capability)
	}
	return service, nil
}

// newCleanupContext initializes the database and tenant transaction runtime.
// The caller must call Close() when done.
func newCleanupContext() (*cleanupContext, error) {
	return defaultCleanupRoot.newContext()
}

func (root cleanupRoot) newContext() (*cleanupContext, error) {
	if root.openDatabase == nil {
		return nil, fmt.Errorf("cleanup database opener is required")
	}
	db, err := root.openDatabase()
	if err != nil {
		return nil, fmt.Errorf(errInitDB, err)
	}
	if db == nil {
		return nil, fmt.Errorf("cleanup database opener returned nil")
	}

	postgresRuntime, err := database.NewPostgresUnitOfWork(db, tenant.ObservePoolWait)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	tenantRuntime, err := tenant.NewUnitOfWork(
		postgresRuntime.WithinTenant,
		postgresRuntime.WithinAdmin,
		tenant.SavepointFunc(postgresRuntime),
		database.IsRetryableTransactionError,
	)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	tenantRuntime = tenantRuntime.WithTransactionDetacher(postgresRuntime.ContextWithoutTransaction)
	tenantRuntime = tenantRuntime.WithContextAdapters(postgresRuntime.ContextWithTenant, postgresRuntime.ContextWithTransaction)

	return &cleanupContext{
		DB:            db,
		TenantRuntime: tenantRuntime,
		Output:        os.Stdout,
		Logger:        log.Default(),
	}, nil
}

func newCleanupContextWithAuthCleanup() (*cleanupContext, error) {
	if err := defaultCleanupRoot.validateCapability("auth"); err != nil {
		return nil, err
	}
	ctx, err := newCleanupContext()
	if err != nil {
		return nil, err
	}
	ctx.AuthCleanupService, err = buildCleanupDependency(ctx, defaultCleanupRoot.authCleanup, "auth")
	if err != nil {
		ctx.Close()
		return nil, err
	}
	return ctx, nil
}

func buildAuthCleanupService(ctx *cleanupContext) authCleanupService {
	return services.NewAuthCleanupService(ctx.DB, ctx.TenantRuntime, slog.Default().With("service", "auth-cleanup-cli"))
}

func newCleanupContextWithInvitationCleanup() (*cleanupContext, error) {
	if err := defaultCleanupRoot.validateCapability("invitation"); err != nil {
		return nil, err
	}
	ctx, err := newCleanupContext()
	if err != nil {
		return nil, err
	}
	ctx.InvitationCleanupService, err = buildCleanupDependency(ctx, defaultCleanupRoot.invitationCleanup, "invitation")
	if err != nil {
		ctx.Close()
		return nil, err
	}
	return ctx, nil
}

func buildInvitationCleanupService(ctx *cleanupContext) invitationCleanupService {
	return services.NewInvitationCleanupService(ctx.DB, slog.Default().With("service", "invitation-cleanup-cli"))
}

func newCleanupContextWithSessionCleanup() (*cleanupContext, error) {
	if err := defaultCleanupRoot.validateCapability("session"); err != nil {
		return nil, err
	}
	ctx, err := newCleanupContext()
	if err != nil {
		return nil, err
	}
	ctx.SessionCleanupService, err = buildCleanupDependency(ctx, defaultCleanupRoot.sessionCleanup, "session")
	if err != nil {
		ctx.Close()
		return nil, err
	}
	return ctx, nil
}

func buildSessionCleanupService(ctx *cleanupContext) sessionCleanupService {
	return services.NewSessionCleanupService(ctx.DB, ctx.TenantRuntime, slog.Default().With("service", "session-cleanup-cli"))
}

// newCleanupContextWithCleanupService initializes database and cleanup service.
// Use this for visit/attendance cleanup commands.
// The caller must call Close() when done.
func newCleanupContextWithCleanupService() (*cleanupContext, error) {
	if err := defaultCleanupRoot.validateCapability("retention"); err != nil {
		return nil, err
	}
	ctx, err := newCleanupContext()
	if err != nil {
		return nil, err
	}

	ctx.CleanupService, err = buildCleanupDependency(ctx, defaultCleanupRoot.retentionCleanup, "retention")
	if err != nil {
		ctx.Close()
		return nil, err
	}
	return ctx, nil
}

func buildRetentionCleanupService(ctx *cleanupContext) active.CleanupService {
	return services.NewRetentionCleanupService(ctx.DB, slog.Default().With("service", "retention-cleanup-cli"))

}

// newCleanupContextWithTimetableCleanup initializes database + timetable
// GDPR cleanup service (WP-B14), including its narrow settings graph.
// The caller must call Close() when done.
func newCleanupContextWithTimetableCleanup() (*cleanupContext, error) {
	if err := defaultCleanupRoot.validateCapability("timetable"); err != nil {
		return nil, err
	}
	ctx, err := newCleanupContext()
	if err != nil {
		return nil, err
	}
	ctx.TimetableCleanupService, err = buildCleanupDependency(ctx, defaultCleanupRoot.timetableCleanup, "timetable")
	if err != nil {
		ctx.Close()
		return nil, err
	}
	return ctx, nil
}

func buildTimetableCleanupService(ctx *cleanupContext) schedule.TimetableCleanupService {
	return services.NewTimetableCleanupService(ctx.DB, ctx.TenantRuntime, slog.Default().With("service", "timetable-cleanup-cli"))
}

// newCleanupContextWithTimeTrackingCleanup initializes database + time-tracking
// retention cleanup service (Tranche 0b). Reuses the same audit repo and
// settings service the scheduler version uses so CLI and cron stay in lock-
// step. Caller must Close().
func newCleanupContextWithTimeTrackingCleanup() (*cleanupContext, error) {
	if err := defaultCleanupRoot.validateCapability("time-tracking"); err != nil {
		return nil, err
	}
	ctx, err := newCleanupContext()
	if err != nil {
		return nil, err
	}
	ctx.TimeTrackingCleanupService, err = buildCleanupDependency(ctx, defaultCleanupRoot.timeTrackingCleanup, "time-tracking")
	if err != nil {
		ctx.Close()
		return nil, err
	}
	return ctx, nil
}

func buildTimeTrackingCleanupService(ctx *cleanupContext) active.TimeTrackingCleanupService {
	return services.NewTimeTrackingCleanupService(ctx.DB, ctx.TenantRuntime, slog.Default().With("service", "time-tracking-cleanup-cli"))
}

// Close releases database resources.
func (c *cleanupContext) Close() {
	if c.DB != nil {
		if err := c.DB.Close(); err != nil {
			c.logger().Printf(errCloseDB, err)
		}
	}
}

func (c *cleanupContext) output() io.Writer {
	if c.Output != nil {
		return c.Output
	}
	return io.Discard
}

func (c *cleanupContext) logger() *log.Logger {
	if c.Logger != nil {
		return c.Logger
	}
	return log.New(io.Discard, "", 0)
}

// setupLogger creates a logger that writes to the specified file or stdout.
// Returns: logger, cleanup function (call when done), error.
func setupLogger(logFilePath string, output io.Writer) (*log.Logger, func(), error) {
	if logFilePath == "" {
		return log.New(output, "", log.LstdFlags), func() {}, nil
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
func printStudentBreakdown(output io.Writer, header string, countHeader string, data map[int64]int) {
	if len(data) == 0 {
		return
	}

	fmt.Fprintf(output, "\n%s:\n", header)
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "Student ID\t%s\n", countHeader)

	for studentID, count := range data {
		_, _ = fmt.Fprintf(w, "%d\t%d\n", studentID, count)
	}

	if err := w.Flush(); err != nil {
		log.Printf(errFlushWriter, err)
	}
}

// printDateBreakdown prints a table of dates and their counts.
func printDateBreakdown(output io.Writer, data map[string]int) {
	if len(data) == 0 {
		return
	}

	fmt.Fprintln(output, "\nPer-date breakdown:")
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "Date\tRecords")

	for date, count := range data {
		_, _ = fmt.Fprintf(w, "%s\t%d\n", date, count)
	}

	if err := w.Flush(); err != nil {
		log.Printf(errFlushWriter, err)
	}
}

// printStudentBreakdownWithTotal prints a table with student data and a total row.
func printStudentBreakdownWithTotal(output io.Writer, countHeader string, data map[int64]int) {
	if len(data) == 0 {
		return
	}

	fmt.Fprintln(output, "\nPer-student breakdown:")
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', tabwriter.AlignRight)
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
func printMonthlyBreakdownWithTotal(output io.Writer, header string, data map[string]int64) {
	if len(data) == 0 {
		return
	}

	fmt.Fprintf(output, "\n%s:\n", header)
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', tabwriter.AlignRight)
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
func printRecentDeletions(output io.Writer, deletions []recentDeletionRow) {
	if len(deletions) == 0 {
		return
	}

	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', tabwriter.AlignRight)
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
