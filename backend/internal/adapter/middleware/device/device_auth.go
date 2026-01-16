package device

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/iot"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	iotSvc "github.com/moto-nrw/project-phoenix/internal/core/service/iot"
	usersSvc "github.com/moto-nrw/project-phoenix/internal/core/service/users"
)

type CtxKey = port.DeviceContextKey

const (
	CtxDevice      = port.CtxDevice
	CtxStaff       = port.CtxStaff
	CtxIsIoTDevice = port.CtxIsIoTDevice
)

// DeviceFromCtx retrieves the authenticated device from request context.
func DeviceFromCtx(ctx context.Context) *iot.Device {
	return port.DeviceFromCtx(ctx)
}

// StaffFromCtx retrieves the authenticated staff from request context.
func StaffFromCtx(ctx context.Context) *users.Staff {
	return port.StaffFromCtx(ctx)
}

// IsIoTDeviceRequest checks if the request context is marked as IoT device.
func IsIoTDeviceRequest(ctx context.Context) bool {
	return port.IsIoTDeviceRequest(ctx)
}

// RequireOGSPIN returns the configured device PIN or an error if missing.
func RequireOGSPIN() (string, error) {
	ogsPin := strings.TrimSpace(os.Getenv("OGS_DEVICE_PIN"))
	if ogsPin == "" {
		return "", fmt.Errorf("OGS_DEVICE_PIN environment variable is required")
	}
	return ogsPin, nil
}

// extractAndValidateAPIKey extracts the API key from the Authorization header and validates the device.
// Returns the device if valid, or an error response to render.
func extractAndValidateAPIKey(r *http.Request, iotService iotSvc.Service) (*iot.Device, render.Renderer) {
	// Extract API key from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		logger.Logger.Warn("Device authentication failed: missing Authorization header")
		return nil, ErrDeviceUnauthorized(ErrMissingAPIKey)
	}

	// Parse Bearer token
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		logger.Logger.Warn("Device authentication failed: invalid Authorization header format")
		return nil, ErrDeviceUnauthorized(ErrInvalidAPIKeyFormat)
	}

	apiKey := strings.TrimPrefix(authHeader, bearerPrefix)
	if apiKey == "" {
		logger.Logger.Warn("Device authentication failed: empty API key")
		return nil, ErrDeviceUnauthorized(ErrMissingAPIKey)
	}

	// Validate API key and get device
	device, err := iotService.GetDeviceByAPIKey(r.Context(), apiKey)
	if err != nil {
		logger.Logger.Warn("Device authentication failed: invalid API key:", err)
		return nil, ErrDeviceUnauthorized(ErrInvalidAPIKey)
	}

	if device == nil {
		logger.Logger.Warn("Device authentication failed: device not found")
		return nil, ErrDeviceUnauthorized(ErrInvalidAPIKey)
	}

	// Check if device is active
	if !device.IsActive() {
		logger.Logger.Warn("Device authentication failed: device not active, status:", device.Status)
		return nil, ErrDeviceForbidden(ErrDeviceInactive)
	}

	return device, nil
}

// updateDeviceLastSeen updates the device's last seen timestamp, logging any errors.
func updateDeviceLastSeen(r *http.Request, iotService iotSvc.Service, device *iot.Device) {
	device.UpdateLastSeen()
	if err := iotService.UpdateDevice(r.Context(), device); err != nil {
		logger.Logger.Warn("Failed to update device last seen time:", err)
	}
}

// DeviceAuthenticator is a middleware that validates device API keys and the global OGS PIN.
// It requires both Authorization: Bearer <api_key> and X-Staff-PIN: <pin> headers.
// The middleware sets device context for downstream handlers.
func DeviceAuthenticator(iotService iotSvc.Service, _ usersSvc.PersonService, ogsPin string) func(http.Handler) http.Handler {
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
				logger.Logger.Warn("Device authentication failed: missing X-Staff-PIN header")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrMissingPIN))
				return
			}

			if ogsPin == "" {
				logger.Logger.Error("OGS_DEVICE_PIN not configured at startup")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrInvalidPIN))
				return
			}

			// Validate PIN using constant-time comparison
			if !SecureCompareStrings(staffPIN, ogsPin) {
				logger.Logger.Warn("Device authentication failed: invalid PIN")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrInvalidPIN))
				return
			}

			// Authentication successful - set device context
			ctx := context.WithValue(r.Context(), CtxDevice, device)
			ctx = context.WithValue(ctx, CtxIsIoTDevice, true)

			logger.Logger.Info("Device authentication successful", "device_id", device.DeviceID)
			updateDeviceLastSeen(r, iotService, device)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
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

			logger.Logger.Info("Device-only authentication successful", "device_id", device.DeviceID)
			updateDeviceLastSeen(r, iotService, device)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SecureCompareStrings performs a constant-time comparison of two strings to prevent timing attacks
func SecureCompareStrings(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
