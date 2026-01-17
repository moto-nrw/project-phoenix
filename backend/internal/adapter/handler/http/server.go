package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/service/scheduler"
	"github.com/spf13/viper"
)

// Server provides an HTTP server for the API
type Server struct {
	*http.Server
	scheduler *scheduler.Scheduler
}

// NewServer creates and configures a new API server
func NewServer() (*Server, error) {
	logger.Logger.Info("Initializing API server")

	if err := requireServiceMetadata(); err != nil {
		return nil, err
	}

	api, err := New(viper.GetBool("enable_cors"))
	if err != nil {
		return nil, err
	}

	var addr string
	port := strings.TrimSpace(viper.GetString("port"))
	if port == "" {
		return nil, fmt.Errorf("PORT environment variable is required")
	}

	// Allow port to be set as host:port in env during development
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
		schedulerConfig, err := buildSchedulerConfig()
		if err != nil {
			return nil, err
		}
		srv.scheduler = scheduler.NewScheduler(api.Services.Active, api.Services.ActiveCleanup, api.Services.Auth, api.Services.Invitation, schedulerConfig)
	}

	return srv, nil
}

func requireServiceMetadata() error {
	serviceName := strings.TrimSpace(viper.GetString("service_name"))
	if serviceName == "" {
		return fmt.Errorf("SERVICE_NAME environment variable is required")
	}
	serviceVersion := strings.TrimSpace(viper.GetString("service_version"))
	if serviceVersion == "" {
		return fmt.Errorf("SERVICE_VERSION environment variable is required")
	}
	return nil
}

// Start runs the server with graceful shutdown
func (srv *Server) Start() {
	// Start scheduler if initialized (includes session cleanup task)
	if srv.scheduler != nil {
		srv.scheduler.Start()
	}

	// Start server in a goroutine so that it doesn't block
	go func() {
		logger.Logger.WithField("addr", srv.Addr).Info("Server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Logger.WithError(err).Fatal("Server error")
		}
	}()

	// Set up channel to listen for signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	// Block until we receive a signal
	sig := <-quit
	logger.Logger.WithField("signal", sig.String()).Info("Server shutting down")

	// Stop scheduler if it's running (includes session cleanup task)
	if srv.scheduler != nil {
		srv.scheduler.Stop()
	}

	// Create a deadline for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		logger.Logger.WithError(err).Fatal("Server forced to shutdown")
	}

	logger.Logger.Info("Server gracefully stopped")
}
