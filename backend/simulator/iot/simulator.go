package iot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
)

var (
	// ErrPartialAuthentication indicates that at least one device failed authentication.
	ErrPartialAuthentication = errors.New("one or more devices failed to authenticate")
)

// Run executes the simulator discovery phase: authenticate devices and keep their state in sync.
func Run(ctx context.Context, cfg *Config) error {
	globalPIN := getGlobalPIN()
	if globalPIN == "" {
		return fmt.Errorf("OGS_DEVICE_PIN environment variable is required")
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	client := NewClient(cfg.BaseURL, globalPIN, httpClient)
	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"device_count": len(cfg.Devices),
			"base_url":     strings.TrimSuffix(cfg.BaseURL, "/"),
		}).Info("Starting state sync")
	}

	states := make(map[string]*DeviceState, len(cfg.Devices))
	stateMu := &sync.RWMutex{}

	// Phase 1: Authenticate all devices
	failed := authenticateDevices(ctx, client, cfg, states, stateMu)
	if len(failed) > 0 {
		return fmt.Errorf("%w: %s", ErrPartialAuthentication, strings.Join(failed, ", "))
	}

	// Phase 2: Start event engine if configured
	eventTicker := startEventEngine(ctx, cfg, client, stateMu, states)
	if eventTicker != nil {
		defer eventTicker.Stop()
	}

	// Phase 3: Run refresh loop
	return runRefreshLoop(ctx, cfg, client, states, stateMu)
}

// authenticateDevices authenticates all configured devices and performs initial state sync.
func authenticateDevices(ctx context.Context, client *Client, cfg *Config, states map[string]*DeviceState, stateMu *sync.RWMutex) []string {
	var failed []string

	for _, device := range cfg.Devices {
		if ctx.Err() != nil {
			break
		}

		if err := client.Authenticate(ctx, device); err != nil {
			if logger.Logger != nil {
				logger.Logger.WithFields(map[string]interface{}{
					"device_id": device.DeviceID,
					"error":     err.Error(),
				}).Error("Device authentication failed")
			}
			failed = append(failed, device.DeviceID)
			continue
		}

		if logger.Logger != nil {
			logger.Logger.WithField("device_id", device.DeviceID).Info("Device authentication OK")
		}
		syncDeviceState(ctx, client, cfg, device, states, stateMu)
	}

	return failed
}

// startEventEngine initializes the event engine if configured.
func startEventEngine(ctx context.Context, cfg *Config, client *Client, stateMu *sync.RWMutex, states map[string]*DeviceState) *time.Ticker {
	if cfg.Event.Interval <= 0 || cfg.Event.MaxEventsPerTick <= 0 {
		return nil
	}

	engine := NewEngine(cfg, client, stateMu, states)
	ticker := time.NewTicker(cfg.Event.Interval)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				engine.Tick(ctx)
			}
		}
	}()

	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"interval":   cfg.Event.Interval.String(),
			"max_events": cfg.Event.MaxEventsPerTick,
		}).Info("Event loop running")
	}
	return ticker
}

// runRefreshLoop periodically refreshes device states.
func runRefreshLoop(ctx context.Context, cfg *Config, client *Client, states map[string]*DeviceState, stateMu *sync.RWMutex) error {
	if cfg.RefreshInterval <= 0 {
		if logger.Logger != nil {
			logger.Logger.Info("Initial authentication complete; no refresh interval configured, exiting")
		}
		return nil
	}

	if logger.Logger != nil {
		logger.Logger.WithField("interval", cfg.RefreshInterval.String()).Info("State sync running. Press Ctrl+C to stop")
	}

	ticker := time.NewTicker(cfg.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if logger.Logger != nil {
				logger.Logger.Info("Context cancelled, shutting down state sync")
			}
			return nil
		case <-ticker.C:
			refreshAllDevices(ctx, client, cfg, states, stateMu)
		}
	}
}

// refreshAllDevices refreshes state for all configured devices.
func refreshAllDevices(ctx context.Context, client *Client, cfg *Config, states map[string]*DeviceState, stateMu *sync.RWMutex) {
	for _, device := range cfg.Devices {
		if ctx.Err() != nil {
			if logger.Logger != nil {
				logger.Logger.Info("Context cancelled, shutting down state sync")
			}
			return
		}
		syncDeviceState(ctx, client, cfg, device, states, stateMu)
	}
}
