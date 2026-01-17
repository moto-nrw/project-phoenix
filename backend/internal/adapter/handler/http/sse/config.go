package sse

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// SSEConfig holds SSE server configuration from environment variables
type SSEConfig struct {
	HeartbeatInterval time.Duration
}

// LoadSSEConfig loads and validates SSE configuration from environment
func LoadSSEConfig() (*SSEConfig, error) {
	cfg := &SSEConfig{}

	// SSE_HEARTBEAT_SECONDS (default: 30)
	heartbeatStr := strings.TrimSpace(viper.GetString("sse_heartbeat_seconds"))
	if heartbeatStr == "" {
		cfg.HeartbeatInterval = 30 * time.Second
	} else {
		heartbeatSeconds := viper.GetInt("sse_heartbeat_seconds")
		if heartbeatSeconds <= 0 {
			return nil, fmt.Errorf("SSE_HEARTBEAT_SECONDS must be a positive integer, got %d", heartbeatSeconds)
		}
		if heartbeatSeconds > 300 {
			return nil, fmt.Errorf("SSE_HEARTBEAT_SECONDS must be <= 300 seconds (5 minutes), got %d", heartbeatSeconds)
		}
		cfg.HeartbeatInterval = time.Duration(heartbeatSeconds) * time.Second
	}

	return cfg, nil
}
