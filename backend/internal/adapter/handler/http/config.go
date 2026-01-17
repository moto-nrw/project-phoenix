package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// HTTPServerConfig holds HTTP server configuration from environment variables
type HTTPServerConfig struct {
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	ShutdownTimeout time.Duration
}

// LoadHTTPServerConfig loads and validates HTTP server configuration from environment
func LoadHTTPServerConfig() (*HTTPServerConfig, error) {
	cfg := &HTTPServerConfig{}

	// HTTP_READ_TIMEOUT_SECONDS (default: 15)
	readTimeoutStr := strings.TrimSpace(viper.GetString("http_read_timeout_seconds"))
	if readTimeoutStr == "" {
		cfg.ReadTimeout = 15 * time.Second
	} else {
		readTimeoutSeconds := viper.GetInt("http_read_timeout_seconds")
		if readTimeoutSeconds <= 0 {
			return nil, fmt.Errorf("HTTP_READ_TIMEOUT_SECONDS must be a positive integer, got %d", readTimeoutSeconds)
		}
		if readTimeoutSeconds > 300 {
			return nil, fmt.Errorf("HTTP_READ_TIMEOUT_SECONDS must be <= 300 seconds (5 minutes), got %d", readTimeoutSeconds)
		}
		cfg.ReadTimeout = time.Duration(readTimeoutSeconds) * time.Second
	}

	// HTTP_WRITE_TIMEOUT_SECONDS (default: 0, must stay 0 for SSE)
	writeTimeoutStr := strings.TrimSpace(viper.GetString("http_write_timeout_seconds"))
	if writeTimeoutStr == "" {
		cfg.WriteTimeout = 0 // Required for SSE long-lived streams
	} else {
		writeTimeoutSeconds := viper.GetInt("http_write_timeout_seconds")
		// Only 0 is valid for SSE compatibility
		if writeTimeoutSeconds != 0 {
			return nil, fmt.Errorf("HTTP_WRITE_TIMEOUT_SECONDS must be 0 for SSE compatibility, got %d", writeTimeoutSeconds)
		}
		cfg.WriteTimeout = 0
	}

	// HTTP_IDLE_TIMEOUT_SECONDS (default: 0, disabled)
	idleTimeoutStr := strings.TrimSpace(viper.GetString("http_idle_timeout_seconds"))
	if idleTimeoutStr == "" {
		cfg.IdleTimeout = 0 // Disabled by default
	} else {
		idleTimeoutSeconds := viper.GetInt("http_idle_timeout_seconds")
		if idleTimeoutSeconds < 0 {
			return nil, fmt.Errorf("HTTP_IDLE_TIMEOUT_SECONDS must be >= 0, got %d", idleTimeoutSeconds)
		}
		cfg.IdleTimeout = time.Duration(idleTimeoutSeconds) * time.Second
	}

	// HTTP_SHUTDOWN_TIMEOUT_SECONDS (default: 30)
	shutdownTimeoutStr := strings.TrimSpace(viper.GetString("http_shutdown_timeout_seconds"))
	if shutdownTimeoutStr == "" {
		cfg.ShutdownTimeout = 30 * time.Second
	} else {
		shutdownTimeoutSeconds := viper.GetInt("http_shutdown_timeout_seconds")
		if shutdownTimeoutSeconds <= 0 {
			return nil, fmt.Errorf("HTTP_SHUTDOWN_TIMEOUT_SECONDS must be a positive integer, got %d", shutdownTimeoutSeconds)
		}
		if shutdownTimeoutSeconds > 300 {
			return nil, fmt.Errorf("HTTP_SHUTDOWN_TIMEOUT_SECONDS must be <= 300 seconds (5 minutes), got %d", shutdownTimeoutSeconds)
		}
		cfg.ShutdownTimeout = time.Duration(shutdownTimeoutSeconds) * time.Second
	}

	return cfg, nil
}
