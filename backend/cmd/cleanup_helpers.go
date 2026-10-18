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

	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"

	"github.com/moto-nrw/project-phoenix/api"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
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

type authCleanupService interface {
	CountExpiredTokens(context.Context) (int, error)
	CleanupExpiredTokens(context.Context) (int, error)
	DeleteStalePasswordResetWindows(context.Context) (int, error)
}

type invitationCleanupService interface {
	DeleteExpiredSchoolInvitations(context.Context) (int, error)
}

type sessionCleanupService interface {
	CleanupAbandonedSessions(context.Context, time.Duration) (int, error)
	EndDailySessions(context.Context) (*studentpresence.DailySessionCleanupResult, error)
}

type retentionCleanupService = studentpresence.PresenceCleanup
type timetableCleanupService = timetable.TimetableCleanup
type timeTrackingCleanupService = workforceCompose.TimeTrackingCleanupService

// cleanupRoot names how each cleanup command builds its one sweep. A command
// opens the database, builds the owners its sweep needs as local values and
// passes them on; nothing is held between the steps.
type cleanupRoot struct {
	openDatabase        func() (transactionruntime.Database, error)
	authCleanup         func(transactionruntime.Database, auditModels.Command) authCleanupService
	invitationCleanup   func(transactionruntime.Database) invitationCleanupService
	sessionCleanup      func(transactionruntime.Database, organizationtenancy.Capability, timetable.Capability) sessionCleanupService
	retentionCleanup    func(transactionruntime.Database, organizationtenancy.Capability, auditModels.Command) studentpresence.PresenceCleanup
	timetableCleanup    func(transactionruntime.Database, organizationtenancy.Capability, timetable.Capability, auditModels.Command) timetable.TimetableCleanup
	timeTrackingCleanup func(transactionruntime.Database, organizationtenancy.Capability, auditModels.Command) workforceCompose.TimeTrackingCleanupService
}

var defaultCleanupRoot = cleanupRoot{
	openDatabase:        transactionruntime.OpenDatabase,
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

// requireCleanupDependency rejects a service its builder could not compose,
// including a typed nil.
func requireCleanupDependency[T any](service T, capability string) (T, error) {
	var zero T
	value := reflect.ValueOf(service)
	if !value.IsValid() || ((value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) && value.IsNil()) {
		return zero, fmt.Errorf("%s cleanup service builder returned nil", capability)
	}
	return service, nil
}

// open opens the command's database and tenant transaction runtime.
// The caller must close it with closeCleanupDatabase when done.
func (root cleanupRoot) open() (transactionruntime.Database, error) {
	if root.openDatabase == nil {
		return transactionruntime.Database{}, fmt.Errorf("cleanup database opener is required")
	}
	db, err := root.openDatabase()
	if err != nil {
		return transactionruntime.Database{}, fmt.Errorf(errInitDB, err)
	}
	if db.Pool() == nil {
		return transactionruntime.Database{}, fmt.Errorf("cleanup database opener returned nil")
	}
	return db, nil
}

// openFor fails fast on a missing builder for capability before it opens the
// database.
func (root cleanupRoot) openFor(capability string) (transactionruntime.Database, error) {
	if err := root.validateCapability(capability); err != nil {
		return transactionruntime.Database{}, err
	}
	return root.open()
}

// openWithAudit opens the database for capability with the fail-closed Audit
// command its sweep appends through.
func (root cleanupRoot) openWithAudit(capability string) (transactionruntime.Database, auditModels.Command, error) {
	db, err := root.openFor(capability)
	if err != nil {
		return transactionruntime.Database{}, nil, err
	}
	auditCommand, err := newCleanupAuditCommand()
	if err != nil {
		_ = db.Close()
		return transactionruntime.Database{}, nil, err
	}
	return db, auditCommand, nil
}

// newCleanupSchools composes Organization & Tenancy over db; on failure db is
// closed.
func newCleanupSchools(db transactionruntime.Database) (organizationtenancy.Capability, error) {
	schools, err := api.NewOrganizationTenancy(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return schools, nil
}

// newCleanupTimetableOwner composes the Timetable owner over db; on failure db
// is closed.
func newCleanupTimetableOwner(db transactionruntime.Database) (timetable.Capability, error) {
	timetableCapability, err := api.NewCleanupTimetable(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return timetableCapability, nil
}

// requireBuiltCleanup closes db when the built service is missing.
func requireBuiltCleanup[T any](db transactionruntime.Database, service T, capability string) (T, error) {
	built, err := requireCleanupDependency(service, capability)
	if err != nil {
		closeCleanupDatabase(db, log.Default())
	}
	return built, err
}

func (root cleanupRoot) openAuthCleanup() (transactionruntime.Database, authCleanupService, error) {
	db, auditCommand, err := root.openWithAudit("auth")
	if err != nil {
		return transactionruntime.Database{}, nil, err
	}
	service, err := requireBuiltCleanup(db, root.authCleanup(db, auditCommand), "auth")
	if err != nil {
		return transactionruntime.Database{}, nil, err
	}
	return db, service, nil
}

// newCleanupAuditCommand builds the fail-closed Audit command every cleanup
// root appends through.
func newCleanupAuditCommand() (auditModels.Command, error) {
	return api.NewCleanupAuditCommand(slog.Default().With("service", "cleanup-cli"))
}

func buildAuthCleanupService(db transactionruntime.Database, auditCommand auditModels.Command) authCleanupService {
	service, err := api.NewAuthCleanupService(db, slog.Default().With("service", "auth-cleanup-cli"), auditCommand)
	if err != nil {
		slog.Default().Error("auth cleanup service composition failed", "error", err)
		return nil
	}
	return service
}

func (root cleanupRoot) openInvitationCleanup() (transactionruntime.Database, invitationCleanupService, error) {
	db, err := root.openFor("invitation")
	if err != nil {
		return transactionruntime.Database{}, nil, err
	}
	service, err := requireBuiltCleanup(db, root.invitationCleanup(db), "invitation")
	if err != nil {
		return transactionruntime.Database{}, nil, err
	}
	return db, service, nil
}

func buildInvitationCleanupService(db transactionruntime.Database) invitationCleanupService {
	service, err := identityaccessCompose.NewInvitationMaintenance(db.Pool())
	if err != nil {
		// requireCleanupDependency reports the nil as the missing capability,
		// the same way a builder that cannot compose its service does.
		slog.Default().Error("invitation cleanup service unavailable", slog.Any("error", err))
		return nil
	}
	return service
}

func (root cleanupRoot) openSessionCleanup() (transactionruntime.Database, organizationtenancy.Capability, sessionCleanupService, error) {
	db, err := root.openFor("session")
	if err != nil {
		return transactionruntime.Database{}, nil, nil, err
	}
	schools, err := newCleanupSchools(db)
	if err != nil {
		return transactionruntime.Database{}, nil, nil, err
	}
	timetableCapability, err := newCleanupTimetableOwner(db)
	if err != nil {
		return transactionruntime.Database{}, nil, nil, err
	}
	service, err := requireBuiltCleanup(db, root.sessionCleanup(db, schools, timetableCapability), "session")
	if err != nil {
		return transactionruntime.Database{}, nil, nil, err
	}
	return db, schools, service, nil
}

func buildSessionCleanupService(db transactionruntime.Database, schools organizationtenancy.Capability, timetableCapability timetable.Capability) sessionCleanupService {
	return newSessionCleanupService(db, schools, timetableCapability, slog.Default().With("service", "session-cleanup-cli"))
}

// openRetentionCleanup opens the database and builds the visit, attendance
// and supervisor retention sweep. The caller must close the database.
func (root cleanupRoot) openRetentionCleanup() (transactionruntime.Database, organizationtenancy.Capability, studentpresence.PresenceCleanup, error) {
	db, auditCommand, err := root.openWithAudit("retention")
	if err != nil {
		return transactionruntime.Database{}, nil, nil, err
	}
	schools, err := newCleanupSchools(db)
	if err != nil {
		return transactionruntime.Database{}, nil, nil, err
	}
	service, err := requireBuiltCleanup(db, root.retentionCleanup(db, schools, auditCommand), "retention")
	if err != nil {
		return transactionruntime.Database{}, nil, nil, err
	}
	return db, schools, service, nil
}

func buildRetentionCleanupService(db transactionruntime.Database, schools organizationtenancy.Capability, auditCommand auditModels.Command) studentpresence.PresenceCleanup {
	return newRetentionCleanupService(db, schools, slog.Default().With("service", "retention-cleanup-cli"), auditCommand)
}

// openTimetableCleanup opens the database and builds the timetable GDPR
// cleanup service (WP-B14), including its narrow settings graph. The caller
// must close the database.
func (root cleanupRoot) openTimetableCleanup() (transactionruntime.Database, organizationtenancy.Capability, timetable.TimetableCleanup, error) {
	db, auditCommand, err := root.openWithAudit("timetable")
	if err != nil {
		return transactionruntime.Database{}, nil, nil, err
	}
	schools, err := newCleanupSchools(db)
	if err != nil {
		return transactionruntime.Database{}, nil, nil, err
	}
	timetableCapability, err := newCleanupTimetableOwner(db)
	if err != nil {
		return transactionruntime.Database{}, nil, nil, err
	}
	service, err := requireBuiltCleanup(db, root.timetableCleanup(db, schools, timetableCapability, auditCommand), "timetable")
	if err != nil {
		return transactionruntime.Database{}, nil, nil, err
	}
	return db, schools, service, nil
}

func buildTimetableCleanupService(db transactionruntime.Database, schools organizationtenancy.Capability, timetableCapability timetable.Capability, auditCommand auditModels.Command) timetable.TimetableCleanup {
	logger := slog.Default().With("service", "timetable-cleanup-cli")
	cleanup, err := newTimetableCleanupService(db, schools, timetableCapability, logger, auditCommand)
	if err != nil {
		// requireCleanupDependency reports the missing service to the caller.
		logger.Error("compose timetable cleanup", slog.String("error", err.Error()))
		return nil
	}
	return cleanup
}

// openTimeTrackingCleanup opens the database and builds the time-tracking
// retention cleanup service (Tranche 0b). It reuses the same audit repo and
// settings service the scheduler version uses so CLI and cron stay in lock-
// step. The caller must close the database.
func (root cleanupRoot) openTimeTrackingCleanup() (transactionruntime.Database, organizationtenancy.Capability, workforceCompose.TimeTrackingCleanupService, error) {
	db, auditCommand, err := root.openWithAudit("time-tracking")
	if err != nil {
		return transactionruntime.Database{}, nil, nil, err
	}
	schools, err := newCleanupSchools(db)
	if err != nil {
		return transactionruntime.Database{}, nil, nil, err
	}
	service, err := requireBuiltCleanup(db, root.timeTrackingCleanup(db, schools, auditCommand), "time-tracking")
	if err != nil {
		return transactionruntime.Database{}, nil, nil, err
	}
	return db, schools, service, nil
}

func buildTimeTrackingCleanupService(db transactionruntime.Database, schools organizationtenancy.Capability, auditCommand auditModels.Command) workforceCompose.TimeTrackingCleanupService {
	workTime, err := api.NewUnobservedWorkforce(db)
	if err != nil {
		panic(fmt.Sprintf("time tracking cleanup: compose workforce: %v", err))
	}
	return newTimeTrackingCleanupService(db, schools, workTime, slog.Default().With("service", "time-tracking-cleanup-cli"), auditCommand)
}

// closeCleanupDatabase releases the database and reports a failure to
// logger; a nil logger discards it.
func closeCleanupDatabase(db transactionruntime.Database, logger *log.Logger) {
	if err := db.Close(); err != nil {
		if logger == nil {
			logger = log.New(io.Discard, "", 0)
		}
		logger.Printf(errCloseDB, err)
	}
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

	mustFprintf(output, "\n%s:\n", header)
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	_ = mustFprintf(w, "Student ID\t%s\n", countHeader)

	for studentID, count := range data {
		_ = mustFprintf(w, "%d\t%d\n", studentID, count)
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

	mustFprintln(output, "\nPer-date breakdown:")
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	_ = mustFprintln(w, "Date\tRecords")

	for date, count := range data {
		_ = mustFprintf(w, "%s\t%d\n", date, count)
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

	mustFprintln(output, "\nPer-student breakdown:")
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', tabwriter.AlignRight)
	_ = mustFprintf(w, "Student ID\t%s\t\n", countHeader)
	_ = mustFprintln(w, "----------\t----------------\t")

	total := 0
	for studentID, count := range data {
		_ = mustFprintf(w, "%d\t%d\t\n", studentID, count)
		total += count
	}

	_ = mustFprintln(w, "----------\t----------------\t")
	_ = mustFprintf(w, "TOTAL\t%d\t\n", total)

	if err := w.Flush(); err != nil {
		log.Printf(errFlushWriter, err)
	}
}

// printMonthlyBreakdownWithTotal prints a table with monthly data and totals.
func printMonthlyBreakdownWithTotal(output io.Writer, header string, data map[string]int64) {
	if len(data) == 0 {
		return
	}

	mustFprintf(output, "\n%s:\n", header)
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', tabwriter.AlignRight)
	_ = mustFprintln(w, "Month\tExpired Visits\t")
	_ = mustFprintln(w, "-------\t--------------\t")

	var total int64
	for month, count := range data {
		_ = mustFprintf(w, "%s\t%d\t\n", month, count)
		total += count
	}

	_ = mustFprintln(w, "-------\t--------------\t")
	_ = mustFprintf(w, "TOTAL\t%d\t\n", total)

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
	_ = mustFprintln(w, "Date\tRecords Deleted\tStudents\t")
	_ = mustFprintln(w, "----------\t---------------\t--------\t")

	for _, d := range deletions {
		_ = mustFprintf(w, "%s\t%d\t%d\t\n", d.Date, d.RecordsDeleted, d.StudentCount)
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
func queryRecentDeletions(ctx context.Context, db transactionruntime.Database) ([]recentDeletionRow, error) {
	summaries, err := listRecentAuditRetentionSummaries(ctx, db, time.Now().AddDate(0, 0, -30), 10)
	if err != nil {
		return nil, err
	}
	deletions := make([]recentDeletionRow, len(summaries))
	for index, summary := range summaries {
		deletions[index] = recentDeletionRow{
			Date: summary.Date, RecordsDeleted: summary.RecordsDeleted, StudentCount: summary.StudentCount,
		}
	}
	return deletions, nil
}
