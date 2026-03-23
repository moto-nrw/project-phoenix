package device

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type CtxKey int

const (
	CtxDevice CtxKey = iota
	CtxStaff
	CtxIsIoTDevice
)

const lastSeenDebounceWindow = 60 * time.Second

type lastSeenDebounceState struct {
	mu            sync.Mutex
	lastPersisted time.Time
	latestSeen    time.Time
	flushTimer    *time.Timer
	writeInFlight bool
}

var lastSeenWriteCache sync.Map

// DeviceFromCtx retrieves the authenticated device from request context.
func DeviceFromCtx(ctx context.Context) *iot.Device {
	device, ok := ctx.Value(CtxDevice).(*iot.Device)
	if !ok {
		return nil
	}
	return device
}

// StaffFromCtx retrieves the authenticated staff from request context.
func StaffFromCtx(ctx context.Context) *users.Staff {
	staff, ok := ctx.Value(CtxStaff).(*users.Staff)
	if !ok {
		return nil
	}
	return staff
}

// IsIoTDeviceRequest checks if the request is from an IoT device using global PIN.
// Returns true when a device has authenticated with API key + global OGS PIN.
func IsIoTDeviceRequest(ctx context.Context) bool {
	isIoT, ok := ctx.Value(CtxIsIoTDevice).(bool)
	return ok && isIoT
}

// extractAndValidateAPIKey extracts the API key from the Authorization header and validates the device.
// Returns the device if valid, or an error response to render.
func extractAndValidateAPIKey(r *http.Request, iotService iotSvc.Service) (*iot.Device, render.Renderer) {
	// Extract API key from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		slog.Warn("device authentication failed: missing Authorization header")
		return nil, ErrDeviceUnauthorized(ErrMissingAPIKey)
	}

	// Parse Bearer token
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		slog.Warn("device authentication failed: invalid Authorization header format")
		return nil, ErrDeviceUnauthorized(ErrInvalidAPIKeyFormat)
	}

	apiKey := strings.TrimPrefix(authHeader, bearerPrefix)
	if apiKey == "" {
		slog.Warn("device authentication failed: empty API key")
		return nil, ErrDeviceUnauthorized(ErrMissingAPIKey)
	}

	// Validate API key and get device
	device, err := iotService.GetDeviceByAPIKey(r.Context(), apiKey)
	if err != nil {
		slog.Warn("device authentication failed: invalid API key",
			slog.String("error", err.Error()),
		)
		return nil, ErrDeviceUnauthorized(ErrInvalidAPIKey)
	}

	if device == nil {
		slog.Warn("device authentication failed: device not found")
		return nil, ErrDeviceUnauthorized(ErrInvalidAPIKey)
	}

	// Check if device is active
	if !device.IsActive() {
		slog.Warn("device authentication failed: device not active",
			slog.String("status", string(device.Status)),
		)
		return nil, ErrDeviceForbidden(ErrDeviceInactive)
	}

	return device, nil
}

// updateDeviceLastSeen updates the device's last seen timestamp, logging any errors.
// Uses device.ID (PK, globally unique) for the debounce cache key and DB update
// to avoid cross-tenant collisions when device_id strings overlap.
func updateDeviceLastSeen(r *http.Request, iotService iotSvc.Service, device *iot.Device) {
	now := time.Now()
	device.LastSeen = &now

	state := getOrCreateLastSeenState(device.ID)
	state.mu.Lock()
	state.latestSeen = now

	shouldWriteNow := !state.writeInFlight && (state.lastPersisted.IsZero() || (now.Sub(state.lastPersisted) >= lastSeenDebounceWindow && state.flushTimer == nil))
	if shouldWriteNow {
		state.writeInFlight = true
		state.mu.Unlock()
		persistLastSeen(r.Context(), iotService, device.ID, now, state)
		return
	}

	if !state.writeInFlight {
		scheduleDeferredFlushLocked(iotService, device.ID, state, now)
	}
	state.mu.Unlock()
}

func getOrCreateLastSeenState(id int64) *lastSeenDebounceState {
	if existing, ok := lastSeenWriteCache.Load(id); ok {
		if state, ok := existing.(*lastSeenDebounceState); ok {
			return state
		}
	}

	state := &lastSeenDebounceState{}
	actual, _ := lastSeenWriteCache.LoadOrStore(id, state)
	actualState, _ := actual.(*lastSeenDebounceState)
	return actualState
}

func persistLastSeen(ctx context.Context, iotService iotSvc.Service, id int64, observedAt time.Time, state *lastSeenDebounceState) {
	if err := iotService.UpdateDeviceLastSeenAt(ctx, id, observedAt); err != nil {
		slog.Warn("failed to update device last seen time",
			slog.Int64("device_pk", id),
			slog.String("error", err.Error()),
		)
		state.mu.Lock()
		state.writeInFlight = false
		if state.latestSeen.After(observedAt) {
			state.lastPersisted = observedAt
		}
		scheduleDeferredFlushLocked(iotService, id, state, time.Now())
		state.mu.Unlock()
		return
	}

	state.mu.Lock()
	state.lastPersisted = observedAt
	state.writeInFlight = false
	scheduleDeferredFlushLocked(iotService, id, state, time.Now())
	state.mu.Unlock()
}

func flushDeferredLastSeen(iotService iotSvc.Service, id int64, state *lastSeenDebounceState) {
	state.mu.Lock()
	latestSeen := state.latestSeen
	lastPersisted := state.lastPersisted
	state.flushTimer = nil
	if state.writeInFlight || latestSeen.IsZero() || !latestSeen.After(lastPersisted) {
		state.mu.Unlock()
		return
	}
	state.writeInFlight = true
	state.mu.Unlock()

	persistLastSeen(context.Background(), iotService, id, latestSeen, state)
}

func scheduleDeferredFlushLocked(iotService iotSvc.Service, id int64, state *lastSeenDebounceState, now time.Time) {
	if state.writeInFlight || state.flushTimer != nil || state.lastPersisted.IsZero() || !state.latestSeen.After(state.lastPersisted) {
		return
	}

	delay := state.lastPersisted.Add(lastSeenDebounceWindow).Sub(now)
	if delay < 0 {
		delay = 0
	}

	state.flushTimer = time.AfterFunc(delay, func() {
		flushDeferredLastSeen(iotService, id, state)
	})
}

func deviceAuthenticator(iotService iotSvc.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Validate API key and get device
			device, errResp := extractAndValidateAPIKey(r, iotService)
			if errResp != nil {
				_ = render.Render(w, r, errResp)
				return
			}

			// Extract staff PIN from X-Staff-PIN header
			staffPIN := r.Header.Get("X-Staff-PIN")
			if staffPIN == "" {
				slog.Warn("device authentication failed: missing X-Staff-PIN header")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrMissingPIN))
				return
			}

			// Get global OGS PIN from environment
			ogsPin := os.Getenv("OGS_DEVICE_PIN")
			if ogsPin == "" {
				slog.Error("OGS_DEVICE_PIN not configured in environment")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrInvalidPIN))
				return
			}

			// Validate PIN using constant-time comparison
			if !SecureCompareStrings(staffPIN, ogsPin) {
				slog.Warn("device authentication failed: invalid PIN")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrInvalidPIN))
				return
			}

			// Authentication successful - set device context
			ctx := context.WithValue(r.Context(), CtxDevice, device)
			ctx = context.WithValue(ctx, CtxIsIoTDevice, true)

			// Inject tenant context from the authenticated device so
			// tenant.FromContext(ctx) works in downstream services (e.g., CreateVisit).
			// Device-auth routes don't use jwt.TenantMiddleware.
			if device.TenantID > 0 {
				ctx = tenant.WithTenantID(ctx, device.TenantID)
			}

			slog.Debug("device authentication successful",
				slog.String("device_id", device.DeviceID),
			)
			updateDeviceLastSeen(r, iotService, device)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// DeviceAuthenticator is a middleware that validates device API keys and the global OGS PIN.
// It requires both Authorization: Bearer <api_key> and X-Staff-PIN: <pin> headers.
// The middleware sets device context for downstream handlers.
func DeviceAuthenticator(iotService iotSvc.Service, _ usersSvc.PersonService) func(http.Handler) http.Handler {
	return deviceAuthenticator(iotService)
}

// DeviceOnlyAuthenticator is a middleware that validates only device API keys.
// It requires only Authorization: Bearer <api_key> header (no staff PIN required).
// The middleware sets device context for downstream handlers.
// This is used for endpoints that need device authentication but not staff authentication,
// such as getting the list of available teachers for login selection.
func DeviceOnlyAuthenticator(iotService iotSvc.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Validate API key and get device
			device, errResp := extractAndValidateAPIKey(r, iotService)
			if errResp != nil {
				_ = render.Render(w, r, errResp)
				return
			}

			// Authentication successful - set device context only
			ctx := context.WithValue(r.Context(), CtxDevice, device)

			// Inject tenant context from the authenticated device
			if device.TenantID > 0 {
				ctx = tenant.WithTenantID(ctx, device.TenantID)
			}

			slog.Info("device-only authentication successful",
				slog.String("device_id", device.DeviceID),
			)
			updateDeviceLastSeen(r, iotService, device)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SecureCompareStrings performs a constant-time comparison of two strings to prevent timing attacks
func SecureCompareStrings(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
