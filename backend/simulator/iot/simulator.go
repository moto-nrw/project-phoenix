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

const sessionStartRetryInterval = 30 * time.Second

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

	log.Printf("[simulator] Starting state sync for %d device(s) against %s", len(cfg.Devices), strings.TrimSuffix(cfg.BaseURL, "/"))

	var failed []string
	states := make(map[string]*DeviceState, len(cfg.Devices))

	for _, device := range cfg.Devices {
		device := device // capture range variable

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := client.Authenticate(ctx, device); err != nil {
			log.Printf("[simulator] Device %s authentication FAILED: %v", device.DeviceID, err)
			failed = append(failed, device.DeviceID)
			continue
		}

		log.Printf("[simulator] Device %s authentication OK", device.DeviceID)

		if state, err := refreshDeviceState(ctx, client, device); err != nil {
			log.Printf("[simulator] Device %s initial sync failed: %v", device.DeviceID, err)
		} else {
			if prev := states[device.DeviceID]; prev != nil {
				state.SessionManaged = prev.SessionManaged
				state.ManagedSessionID = prev.ManagedSessionID
				state.LastSessionStartAttempt = prev.LastSessionStartAttempt
				if !state.sessionActive() && prev.SessionManaged {
					state.SessionManaged = false
					state.ManagedSessionID = nil
				}
			}
			states[device.DeviceID] = state
			maybeStartDefaultSession(ctx, client, device, state)
			logDeviceState(device.DeviceID, state)
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("%w: %s", ErrPartialAuthentication, strings.Join(failed, ", "))
	}

	if cfg.RefreshInterval <= 0 {
		log.Printf("[simulator] Initial authentication complete; no refresh interval configured, exiting.")
		return nil
	}

	log.Printf("[simulator] State sync running (interval %s). Press Ctrl+C to stop.", cfg.RefreshInterval)

	ticker := time.NewTicker(cfg.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[simulator] Context cancelled, shutting down state sync.")
			return nil
		case <-ticker.C:
			for _, device := range cfg.Devices {
				select {
				case <-ctx.Done():
					log.Printf("[simulator] Context cancelled, shutting down state sync.")
					return nil
				default:
				}

				state, err := refreshDeviceState(ctx, client, device)
				if err != nil {
					log.Printf("[simulator] Device %s refresh failed: %v", device.DeviceID, err)
					continue
				}

				if prev := states[device.DeviceID]; prev != nil {
					state.SessionManaged = prev.SessionManaged
					state.ManagedSessionID = prev.ManagedSessionID
					state.LastSessionStartAttempt = prev.LastSessionStartAttempt
					if !state.sessionActive() && prev.SessionManaged {
						state.SessionManaged = false
						state.ManagedSessionID = nil
					}
				}

				states[device.DeviceID] = state
				maybeStartDefaultSession(ctx, client, device, state)
				logDeviceState(device.DeviceID, state)
			}
		}
	}
}

func refreshDeviceState(ctx context.Context, client *Client, device DeviceConfig) (*DeviceState, error) {
	session, err := client.FetchSession(ctx, device)
	if err != nil {
		return nil, fmt.Errorf("fetch session: %w", err)
	}

	rooms, err := client.FetchRooms(ctx, device)
	if err != nil {
		return nil, fmt.Errorf("fetch rooms: %w", err)
	}

	activities, err := client.FetchActivities(ctx, device)
	if err != nil {
		return nil, fmt.Errorf("fetch activities: %w", err)
	}

	students, err := client.FetchStudents(ctx, device)
	if err != nil {
		return nil, fmt.Errorf("fetch students: %w", err)
	}

	return &DeviceState{
		Session:       session,
		Rooms:         rooms,
		Activities:    activities,
		Students:      students,
		LastRefreshed: time.Now(),
	}, nil
}

func logDeviceState(deviceID string, state *DeviceState) {
	if state == nil {
		log.Printf("[simulator] Device %s state unavailable", deviceID)
		return
	}

	roomName := "<none>"
	sessionStatus := "inactive"

	if state.Session != nil {
		if state.Session.RoomName != nil && *state.Session.RoomName != "" {
			roomName = *state.Session.RoomName
		}
		if state.Session.IsActive {
			sessionStatus = "active"
		}
	}

	log.Printf(
		"[simulator] Device %s -> session=%s room=%s students=%d rooms=%d activities=%d refreshed=%s",
		deviceID,
		sessionStatus,
		roomName,
		len(state.Students),
		len(state.Rooms),
		len(state.Activities),
		state.LastRefreshed.Format(time.RFC3339),
	)
}

func getGlobalPIN() string {
	return strings.TrimSpace(os.Getenv("OGS_DEVICE_PIN"))
}

func maybeStartDefaultSession(ctx context.Context, client *Client, device DeviceConfig, state *DeviceState) {
	if device.DefaultSession == nil {
		return
	}

	if state.sessionActive() {
		return
	}

	if time.Since(state.LastSessionStartAttempt) < sessionStartRetryInterval {
		return
	}

	state.LastSessionStartAttempt = time.Now()

	resp, err := client.StartSession(ctx, device, device.DefaultSession)
	if err != nil {
		log.Printf("[simulator] Device %s session start failed: %v", device.DeviceID, err)
		return
	}

	log.Printf("[simulator] Device %s session started (room=%d activity=%d supervisors=%v)", device.DeviceID, device.DefaultSession.RoomID, device.DefaultSession.ActivityID, device.DefaultSession.SupervisorIDs)

	id := resp.ActiveGroupID
	state.SessionManaged = true
	state.ManagedSessionID = &id
	state.LastSessionStartAttempt = time.Now()

	session, err := client.FetchSession(ctx, device)
	if err != nil {
		log.Printf("[simulator] Device %s failed to refresh session after start: %v", device.DeviceID, err)
		return
	}
	state.Session = session
}
