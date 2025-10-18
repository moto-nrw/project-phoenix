package iot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	// ErrPartialAuthentication indicates that at least one device failed authentication.
	ErrPartialAuthentication = errors.New("one or more devices failed to authenticate")
)

// Run executes the minimal simulator bootstrap: authenticate each device once.
func Run(ctx context.Context, cfg *Config) error {
	globalPIN := getGlobalPIN()
	if globalPIN == "" {
		return fmt.Errorf("OGS_DEVICE_PIN environment variable is required")
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
	log.Printf("[simulator] Starting bootstrap for %d device(s) against %s", len(cfg.Devices), baseURL)

	var failed []string
	for _, device := range cfg.Devices {
		device := device // capture range variable

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := authenticateDevice(ctx, httpClient, baseURL, globalPIN, device); err != nil {
			log.Printf("[simulator] Device %s authentication FAILED: %v", device.DeviceID, err)
			failed = append(failed, device.DeviceID)
			continue
		}

		log.Printf("[simulator] Device %s authentication OK", device.DeviceID)
	}

	if len(failed) > 0 {
		return fmt.Errorf("%w: %s", ErrPartialAuthentication, strings.Join(failed, ", "))
	}

	log.Printf("[simulator] Bootstrap complete. All devices authenticated successfully.")
	return nil
}

func authenticateDevice(ctx context.Context, client *http.Client, baseURL, pin string, device DeviceConfig) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/iot/status", nil)
	if err != nil {
		return fmt.Errorf("build status request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+device.APIKey)
	req.Header.Set("X-Staff-PIN", pin)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "project-phoenix-simulator/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call status endpoint: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from /api/iot/status", resp.StatusCode)
	}

	return nil
}

func getGlobalPIN() string {
	return strings.TrimSpace(os.Getenv("OGS_DEVICE_PIN"))
}
