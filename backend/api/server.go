package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/services/scheduler"
	"github.com/spf13/viper"
)

// Server provides an HTTP server for the API
type Server struct {
	*http.Server
	scheduler      *scheduler.Scheduler
	capacityLogger *observability.CapacityLogger
}

// NewServer creates and configures a new API server
func NewServer(logger *slog.Logger) (*Server, error) {
	slog.Info("Initializing API server")

	api, err := New(viper.GetBool("enable_cors"), logger)
	if err != nil {
		return nil, err
	}

	var addr string
	port := viper.GetString("port")

	// Allow port to be set as localhost:8080 in env during development
	if strings.Contains(port, ":") {
		addr = port
	} else {
		addr = ":" + port
	}

	srv := &Server{
		Server: &http.Server{
			Addr:    addr,
			Handler: api,
			// ReadTimeout stays modest to protect against slowloris attacks,
			// but WriteTimeout must be disabled to allow long-lived SSE streams.
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 0,
			IdleTimeout:  0,
		},
		scheduler:      nil, // Will be initialized if cleanup is enabled
		capacityLogger: observability.NewCapacityLogger(api.db.DB, api.Services.RealtimeHub, api.Metrics, logger.With("component", "capacity")),
	}

	// Initialize scheduler if cleanup is enabled
	// Note: Session cleanup is now handled by the scheduler's scheduleSessionCleanupTask()
	if api.Services != nil && api.Services.ActiveCleanup != nil && api.Services.Active != nil {
		// OperatorAuth implements EmailChangeTokenCleaner (5th arg).
		// OperatorInvitation implements OperatorInvitationCleaner (6th arg).
		// Both are backed by the same concrete struct exposed through the
		// two narrower interfaces defined in services/platform.
		srv.scheduler = scheduler.NewScheduler(api.Services.Active, api.Services.ActiveCleanup, api.Services.Auth, api.Services.Invitation, api.Services.OperatorAuth, api.Services.OperatorInvitation, logger.With("service", "scheduler"))
		srv.scheduler.SetDB(api.db)
		srv.scheduler.SetSchoolRepo(api.repos.School)
		if api.Services.Settings != nil {
			srv.scheduler.SetSettingsService(api.Services.Settings)
		}
		if api.Services.WorkSession != nil {
			srv.scheduler.SetWorkSessionCleaner(api.Services.WorkSession)
			srv.scheduler.SetBreakAutoEnder(api.Services.WorkSession)
		}
		if api.Services.Feedback != nil {
			srv.scheduler.SetFeedbackCleaner(api.Services.Feedback)
		}
		if api.Services.UnregisteredTagScans != nil {
			srv.scheduler.SetUnregisteredTagScanCleaner(api.Services.UnregisteredTagScans)
		}
		if api.Services.Materialization != nil {
			srv.scheduler.SetMaterializer(api.Services.Materialization)
		}
		if api.Services.AutoStart != nil {
			srv.scheduler.SetAutoStartService(api.Services.AutoStart)
		}
		// WP-B14: timetable GDPR cleanup. Nil service → task does not register.
		if api.Services.TimetableCleanup != nil {
			srv.scheduler.SetTimetableCleanup(api.Services.TimetableCleanup)
		}
		// Tranche 0b: time-tracking GDPR cleanup. Same nil-safe wiring.
		if api.Services.TimeTrackingCleanup != nil {
			srv.scheduler.SetTimeTrackingCleanup(api.Services.TimeTrackingCleanup)
		}
		// Issue #1455: per-child change-history GDPR cleanup. Same nil-safe wiring.
		if api.Services.StudentChangeLogCleanup != nil {
			srv.scheduler.SetStudentChangeLogCleanup(api.Services.StudentChangeLogCleanup)
		}
		// WP-B9: overdue instance tick. Requires both the ActivityInstance
		// repo and a broadcaster — either missing disables the tick.
		if api.repos != nil && api.Services.RealtimeHub != nil {
			srv.scheduler.SetInstanceOverdueDeps(api.repos.ActivityInstance, api.Services.RealtimeHub)
		}
		// Daily session-end bridge: closes schedule-side rows for ended
		// active.groups via repositories (issue #585 layering).
		if api.repos != nil {
			srv.scheduler.SetTimetableBridgeRepos(api.repos.InstanceStudent, api.repos.ActivityInstance)
			srv.scheduler.SetStudentStatusDayRepo(api.repos.StudentStatusDay)
		}
		// Parent-enrollment PR 2: activate-students tick.
		if api.repos != nil && api.repos.Student != nil {
			srv.scheduler.SetStudentLifecycleRepo(api.repos.Student)
			if api.Services.StudentAudit != nil {
				srv.scheduler.SetStudentLifecycleAudit(api.Services.StudentAudit)
			}
		}
		// Parent-enrollment PR 5: platform email outbox worker.
		if api.Services.EmailOutboxWorker != nil {
			srv.scheduler.SetOutboxWorker(api.Services.EmailOutboxWorker)
		}
		// Phase rollover slice 1: per-tenant deadline resolver tick.
		// The adapter narrows the typed return value behind `any` so
		// the scheduler doesn't import the enrollment package.
		if api.Services.EnrollmentRollover != nil {
			rolloverSvc := api.Services.EnrollmentRollover
			srv.scheduler.SetRolloverDeadlineRunner(scheduler.NewRolloverDeadlineRunner(
				func(ctx context.Context, asOf time.Time) (any, error) {
					return rolloverSvc.RunDeadlineWorker(ctx, asOf)
				},
			))
		}
	}

	return srv, nil
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

	slog.Info("Server gracefully stopped")
}
