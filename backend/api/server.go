package api

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/scheduler"
	"github.com/spf13/viper"
)

// Server provides an HTTP server for the API
type Server struct {
	*http.Server
	scheduler      *scheduler.Scheduler
	sessionCleanup *services.SessionCleanupService
}

// NewServer creates and configures a new API server
func NewServer() (*Server, error) {
	log.Println("Initializing API server...")

	api, err := New(viper.GetBool("enable_cors"))
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
	if api.Services != nil && api.Services.ActiveCleanup != nil && api.Services.Active != nil {
		srv.scheduler = scheduler.NewScheduler(api.Services.Active, api.Services.ActiveCleanup, api.Services.Auth)
	}

	// Initialize session cleanup service if active service is available
	if api.Services != nil && api.Services.Active != nil {
		srv.sessionCleanup = services.NewSessionCleanupService(
			api.Services.Active,
			log.New(os.Stdout, "[SessionCleanup] ", log.LstdFlags),
		)
	}

	return srv, nil
}

// Start runs the server with graceful shutdown
func (srv *Server) Start() {
	// Start scheduler if initialized
	if srv.scheduler != nil {
		srv.scheduler.Start()
	}

	// Start session cleanup service if initialized
	if srv.sessionCleanup != nil {
		srv.sessionCleanup.Start()
	}

	// Start server in a goroutine so that it doesn't block
	go func() {
		log.Printf("Server listening on %s\n", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Set up channel to listen for signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	// Block until we receive a signal
	sig := <-quit
	log.Printf("Server shutting down due to %s signal", sig)

	// Stop session cleanup service first if it's running
	if srv.sessionCleanup != nil {
		srv.sessionCleanup.Stop()
	}

	// Stop scheduler if it's running
	if srv.scheduler != nil {
		srv.scheduler.Stop()
	}

	// Create a deadline for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server gracefully stopped")
}
