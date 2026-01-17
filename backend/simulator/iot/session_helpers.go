package iot

import (
	"context"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

const sessionStartRetryInterval = 30 * time.Second

func generateAGHopTarget(eventCfg EventConfig) int {
	min := eventCfg.Rotation.MinAGHops
	max := eventCfg.Rotation.MaxAGHops
	if min <= 0 && max <= 0 {
		return defaultMinAGHops
	}
	if min <= 0 {
		min = defaultMinAGHops
	}
	if max < min {
		max = min
	}
	if max == min {
		return min
	}

	span := max - min + 1
	if span <= 1 {
		return min
	}
	return min + rng.Intn(span)
}

func logDeviceState(deviceID string, state *DeviceState) {
	if logger.Logger == nil {
		return
	}

	if state == nil {
		logger.Logger.WithField("device_id", deviceID).Warn("Device state unavailable")
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

	logger.Logger.WithFields(map[string]interface{}{
		"device_id":      deviceID,
		"session_status": sessionStatus,
		"room_name":      roomName,
		"student_count":  len(state.Students),
		"room_count":     len(state.Rooms),
		"activity_count": len(state.Activities),
		"refreshed":      state.LastRefreshed.Format(time.RFC3339),
	}).Debug("Device state synced")
}

func getGlobalPIN() string {
	return strings.TrimSpace(os.Getenv("OGS_DEVICE_PIN"))
}

func maybeStartDefaultSession(ctx context.Context, client *Client, device DeviceConfig, stateMu *sync.RWMutex, state *DeviceState) {
	if device.DefaultSession == nil || state == nil {
		return
	}

	stateMu.Lock()

	if state.sessionActive() {
		stateMu.Unlock()
		return
	}

	if time.Since(state.LastSessionStartAttempt) < sessionStartRetryInterval {
		stateMu.Unlock()
		return
	}

	state.LastSessionStartAttempt = time.Now()
	stateMu.Unlock()

	resp, err := client.StartSession(ctx, device, device.DefaultSession)
	if err != nil {
		if logger.Logger != nil {
			logger.Logger.WithFields(map[string]interface{}{
				"device_id": device.DeviceID,
				"error":     err.Error(),
			}).Error("Device session start failed")
		}
		return
	}

	session, err := client.FetchSession(ctx, device)
	if err != nil {
		if logger.Logger != nil {
			logger.Logger.WithFields(map[string]interface{}{
				"device_id": device.DeviceID,
				"error":     err.Error(),
			}).Warn("Device failed to refresh session after start")
		}
	}

	stateMu.Lock()
	defer stateMu.Unlock()

	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"device_id":      device.DeviceID,
			"room_id":        device.DefaultSession.RoomID,
			"activity_id":    device.DefaultSession.ActivityID,
			"supervisor_ids": device.DefaultSession.SupervisorIDs,
		}).Info("Device session started")
	}

	id := resp.ActiveGroupID
	state.SessionManaged = true
	state.ManagedSessionID = &id
	state.LastSessionStartAttempt = time.Now()
	if session != nil {
		state.Session = session
	}
}
