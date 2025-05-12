package api

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Server provides an HTTP server for the API
type Server struct {
	*http.Server
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
		&http.Server{
			Addr:         addr,
			Handler:      api,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}

	return srv, nil
}

// Start runs the server with graceful shutdown
func (srv *Server) Start() {
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

	// Create a deadline for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server gracefully stopped")
}
