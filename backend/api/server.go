package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/analytics"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/scheduler"
	"github.com/spf13/viper"
)

// Server provides an HTTP server for the API
type Server struct {
	*http.Server
	scheduler      *scheduler.Scheduler
	capacityLogger *observability.CapacityLogger
	tracker        analytics.Tracker
}

// NewServer creates and configures a new API server
func NewServer(logger *slog.Logger) (*Server, error) {
	slog.Info("Initializing API server")

	api, err := New(viper.GetBool("enable_cors"), logger)
	if err != nil {
		return nil, err
	}

	srv := &Server{
		Server: &http.Server{
			Addr:    resolveListenAddr(viper.GetString("port")),
			Handler: api,
			// ReadTimeout stays modest to protect against slowloris attacks,
			// but WriteTimeout must be disabled to allow long-lived SSE streams.
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 0,
			IdleTimeout:  0,
		},
		scheduler:      newScheduler(api, logger),
		capacityLogger: observability.NewCapacityLogger(api.db.DB, api.Services.RealtimeHub, api.Metrics, logger.With("component", "capacity")),
		tracker:        api.Services.Tracker,
	}

	return srv, nil
}

// resolveListenAddr turns a configured port into a listen address. A value
// containing ":" (e.g. "localhost:8080" in dev) is used verbatim; a bare port
// is prefixed with ":".
func resolveListenAddr(port string) string {
	if strings.Contains(port, ":") {
		return port
	}
	return ":" + port
}

// newScheduler builds and wires the background scheduler, or returns nil when
// cleanup dependencies are absent (session cleanup is one of its tasks).
func newScheduler(api *API, logger *slog.Logger) *scheduler.Scheduler {
	if api.Services == nil || api.Services.ActiveCleanup == nil || api.Services.Active == nil {
		return nil
	}

	// OperatorAuth implements EmailChangeTokenCleaner (5th arg).
	// OperatorInvitation implements OperatorInvitationCleaner (6th arg).
	// Both are backed by the same concrete struct exposed through the
	// two narrower interfaces defined in services/platform.
	sched := scheduler.NewScheduler(api.Services.Active, api.Services.ActiveCleanup, api.Services.Auth, api.Services.Invitation, api.Services.OperatorAuth, api.Services.OperatorInvitation, logger.With("service", "scheduler"))
	sched.SetDB(api.db)
	sched.SetSchoolRepo(api.repos.School)

	configureSchedulerServices(sched, api.Services)
	configureSchedulerRepos(sched, api)
	if api.Staff != nil {
		sched.SetStaffDocumentFileCleaner(api.Staff)
	}
	if api.Students != nil {
		sched.SetStudentDocumentFileCleaner(api.Students)
	}

	return sched
}

// configureSchedulerServices attaches the optional service-backed scheduler
// tasks; each is skipped when its backing service is nil.
func configureSchedulerServices(sched *scheduler.Scheduler, svc *services.Factory) {
	if svc.Settings != nil {
		sched.SetSettingsService(svc.Settings)
	}
	if svc.WorkSession != nil {
		sched.SetWorkSessionCleaner(svc.WorkSession)
		sched.SetBreakAutoEnder(svc.WorkSession)
		// #1798: auto-checkout at planned shift end (per-tenant opt-in).
		sched.SetAutoCheckouter(svc.WorkSession)
	}
	if svc.Feedback != nil {
		sched.SetFeedbackCleaner(svc.Feedback)
	}
	if svc.UnregisteredTagScans != nil {
		sched.SetUnregisteredTagScanCleaner(svc.UnregisteredTagScans)
	}
	if svc.Materialization != nil {
		sched.SetMaterializer(svc.Materialization)
	}
	if svc.AutoStart != nil {
		sched.SetAutoStartService(svc.AutoStart)
	}
	// WP-B14: timetable GDPR cleanup. Nil service → task does not register.
	if svc.TimetableCleanup != nil {
		sched.SetTimetableCleanup(svc.TimetableCleanup)
	}
	// Tranche 0b: time-tracking GDPR cleanup. Same nil-safe wiring.
	if svc.TimeTrackingCleanup != nil {
		sched.SetTimeTrackingCleanup(svc.TimeTrackingCleanup)
	}
	// Issue #1455: per-child change-history GDPR cleanup. Same nil-safe wiring.
	if svc.StudentChangeLogCleanup != nil {
		sched.SetStudentChangeLogCleanup(svc.StudentChangeLogCleanup)
	}
	// Issue #2189: PWA standalone-usage GDPR cleanup. Same nil-safe wiring.
	if svc.PWAUsage != nil {
		sched.SetPWAUsageCleanup(svc.PWAUsage)
		sched.SetStaffMessageCleanup(svc.StaffMessaging)
	}
	if svc.EnrollmentRejectedCleanup != nil {
		sched.SetEnrollmentRejectedCleanup(svc.EnrollmentRejectedCleanup)
	}
	// Parent-enrollment PR 5: platform email outbox worker.
	if svc.EmailOutboxWorker != nil {
		sched.SetOutboxWorker(svc.EmailOutboxWorker)
	}
	// Issue #1671: per-tenant guardian appointment reminders. The calendar
	// service already satisfies the narrow queuer interface.
	if svc.Calendar != nil {
		sched.SetAppointmentReminderQueuer(svc.Calendar)
	}
	// Phase rollover slice 1: per-tenant deadline resolver tick.
	// The adapter narrows the typed return value behind `any` so
	// the scheduler doesn't import the enrollment package.
	if svc.EnrollmentRollover != nil {
		rolloverSvc := svc.EnrollmentRollover
		sched.SetRolloverDeadlineRunner(scheduler.NewRolloverDeadlineRunner(
			func(ctx context.Context, asOf time.Time) (any, error) {
				return rolloverSvc.RunDeadlineWorker(ctx, asOf)
			},
		))
	}
}

// configureSchedulerRepos attaches the repository-backed scheduler tasks; each
// is skipped when its repositories (or the broadcaster) are absent.
func configureSchedulerRepos(sched *scheduler.Scheduler, api *API) {
	repos := api.repos
	if repos == nil {
		return
	}
	// WP-B9: overdue instance tick. Requires the activity-instance repo,
	// room repo, and broadcaster; partial wiring disables the tick.
	if api.Services.RealtimeHub != nil {
		sched.SetInstanceOverdueDeps(repos.ActivityInstance, repos.Room, api.Services.RealtimeHub)
	}
	// Personal reminder notifications: dispatches one event per person and
	// reminder kind through the channel-agnostic abstraction. Gated per tenant
	// by notifications.dispatch_enabled and the reminders.* settings, and per
	// person by their own consent plus notifications.on_duty_only.
	sched.SetReminderNotificationDeps(scheduler.ReminderNotificationDeps{
		Computer:     api.Services.Reminders,
		Notifier:     api.Services.Notifications,
		Preferences:  api.Services.NotificationPreferences,
		Staff:        repos.Staff,
		Accounts:     repos.Account,
		WorkSessions: repos.WorkSession,
	})
	// Daily session-end bridge: closes schedule-side rows for ended
	// active.groups via repositories (issue #585 layering).
	sched.SetTimetableBridgeRepos(repos.InstanceStudent, repos.ActivityInstance, api.Services.TimetableBridge)
	sched.SetStudentStatusDayRepo(repos.StudentStatusDay)
	sched.SetBookingConsistencyAudit(repos.BookingConsistency)
	// Parent-enrollment PR 2: activate-students tick.
	if repos.Student != nil {
		sched.SetStudentLifecycleRepo(repos.Student)
		if api.Services.StudentAudit != nil {
			sched.SetStudentLifecycleAudit(api.Services.StudentAudit)
		}
		// Effect day of "Betreuung beenden" (#2487): runs inside the same tick,
		// before the status transition it shares its candidate set with.
		if api.Services.CareLifecycle != nil {
			sched.SetCareExitEffector(api.Services.CareLifecycle)
		}
	}
}

// Start runs the server with graceful shutdown
func (srv *Server) Start() {
	capacityCtx, stopCapacityLogger := context.WithCancel(context.Background())
	defer stopCapacityLogger()
	if srv.capacityLogger != nil {
		srv.capacityLogger.LogSnapshot()
		go srv.capacityLogger.Start(capacityCtx)
	}

	// Start scheduler if initialized (includes session cleanup task)
	if srv.scheduler != nil {
		srv.scheduler.Start()
	}

	// Start server in a goroutine so that it doesn't block
	go func() {
		slog.Info("Server listening", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Set up channel to listen for signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	// Block until we receive a signal
	sig := <-quit
	slog.Info("Server shutting down", slog.String("signal", sig.String()))

	// Stop scheduler if it's running (includes session cleanup task)
	if srv.scheduler != nil {
		srv.scheduler.Stop()
	}

	// Create a deadline for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Flush buffered analytics events before exit
	if srv.tracker != nil {
		if err := srv.tracker.Close(); err != nil {
			slog.Warn("analytics tracker close failed", slog.String("error", err.Error()))
		}
	}

	slog.Info("Server gracefully stopped")
}
