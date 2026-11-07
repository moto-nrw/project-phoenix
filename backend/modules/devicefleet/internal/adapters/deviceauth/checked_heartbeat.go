package device

import (
	"context"
	"errors"
	"time"
)

// checkedHeartbeatDirectory authorizes a heartbeat inside the owner's existing
// tenant unit. It never retains the authentication callback for deferred work.
type checkedHeartbeatDirectory interface {
	RecordLastSeenChecked(context.Context, int64, func(context.Context) (time.Time, bool, error)) error
}

var errHeartbeatRejected = errors.New("device heartbeat authentication rejected")

func (d *LastSeenDebouncer) authenticateAndObserve(ctx context.Context, devices DeviceDirectory, device *AuthenticatedDevice, validate func(context.Context) *ErrResponse) *ErrResponse {
	checked, available := devices.(checkedHeartbeatDirectory)
	if !available || !d.heartbeatIsDue(device.ID) {
		return d.validateAndObserve(ctx, devices, device, validate)
	}

	var rejection *ErrResponse
	var observedAt time.Time
	var state *lastSeenDebounceState
	var writing, validated bool
	err := checked.RecordLastSeenChecked(ctx, device.ID, func(pinCtx context.Context) (time.Time, bool, error) {
		validated = true
		rejection = validate(pinCtx)
		if rejection != nil {
			return time.Time{}, false, errHeartbeatRejected
		}
		observedAt, state, writing = d.observeDevice(ctx, devices, device)
		return observedAt, writing, nil
	})
	if !validated {
		// A heartbeat-unit setup failure is not a PIN verdict. Preserve the
		// original resolver's 401 decision and best-effort heartbeat path.
		return d.validateAndObserve(ctx, devices, device, validate)
	}
	if writing {
		d.completeLastSeen(devices, device.ID, observedAt, state, err)
	}
	return rejection
}

func (d *LastSeenDebouncer) validateAndObserve(ctx context.Context, devices DeviceDirectory, device *AuthenticatedDevice, validate func(context.Context) *ErrResponse) *ErrResponse {
	if rejection := validate(ctx); rejection != nil {
		return rejection
	}
	d.updateDeviceLastSeen(ctx, devices, device)
	return nil
}

func (d *LastSeenDebouncer) heartbeatIsDue(id int64) bool {
	cached, exists := d.cache.Load(id)
	if !exists {
		return true
	}
	state := cached.(*lastSeenDebounceState)
	state.mu.Lock()
	defer state.mu.Unlock()
	return heartbeatDue(state, time.Now())
}
