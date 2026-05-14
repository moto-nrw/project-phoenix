package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/services/scheduler"
	"github.com/spf13/viper"
)

// Server provides an HTTP server for the API
type Server struct {
	*http.Server
	scheduler *scheduler.Scheduler
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
		scheduler: nil, // Will be initialized if cleanup is enabled
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
		if api.Services.Materialization != nil {
			srv.scheduler.SetMaterializer(api.Services.Materialization)
		}
		// WP-B14: timetable GDPR cleanup. Nil service → task does not register.
		if api.Services.TimetableCleanup != nil {
			srv.scheduler.SetTimetableCleanup(api.Services.TimetableCleanup)
		}
		// Tranche 0b: time-tracking GDPR cleanup. Same nil-safe wiring.
		if api.Services.TimeTrackingCleanup != nil {
			srv.scheduler.SetTimeTrackingCleanup(api.Services.TimeTrackingCleanup)
		}
		// WP-B9: overdue instance tick. Requires both the ActivityInstance
		// repo and a broadcaster — either missing disables the tick.
		if api.repos != nil && api.Services.RealtimeHub != nil {
			srv.scheduler.SetInstanceOverdueDeps(api.repos.ActivityInstance, api.Services.RealtimeHub)
		}
	}

	return srv, nil
}

// Start runs the server with graceful shutdown
func (srv *Server) Start() {
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
