package device

import (
	"context"
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

type CtxKey int

const (
	CtxDevice CtxKey = iota
	CtxStaff
)

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

// DeviceAuthenticator is a middleware that validates device API keys and the global OGS PIN.
// It requires both Authorization: Bearer <api_key> and X-Staff-PIN: <pin> headers.
// The middleware sets device context for downstream handlers.
func DeviceAuthenticator(iotService iotSvc.Service, usersService usersSvc.PersonService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract API key from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logging.GetLogEntry(r).Warn("Device authentication failed: missing Authorization header")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrMissingAPIKey))
				return
			}

			// Parse Bearer token
			const bearerPrefix = "Bearer "
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				logging.GetLogEntry(r).Warn("Device authentication failed: invalid Authorization header format")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrInvalidAPIKeyFormat))
				return
			}

			apiKey := strings.TrimPrefix(authHeader, bearerPrefix)
			if apiKey == "" {
				logging.GetLogEntry(r).Warn("Device authentication failed: empty API key")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrMissingAPIKey))
				return
			}

			// Extract staff PIN from X-Staff-PIN header
			staffPIN := r.Header.Get("X-Staff-PIN")
			if staffPIN == "" {
				logging.GetLogEntry(r).Warn("Device authentication failed: missing X-Staff-PIN header")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrMissingPIN))
				return
			}

			// Get global OGS PIN from environment
			ogsPin := os.Getenv("OGS_DEVICE_PIN")
			if ogsPin == "" {
				logging.GetLogEntry(r).Error("OGS_DEVICE_PIN not configured in environment")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrInvalidPIN))
				return
			}

			// Validate PIN using constant-time comparison
			if !SecureCompareStrings(staffPIN, ogsPin) {
				logging.GetLogEntry(r).Warn("Device authentication failed: invalid PIN")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrInvalidPIN))
				return
			}

			// Validate API key and get device
			device, err := iotService.GetDeviceByAPIKey(r.Context(), apiKey)
			if err != nil {
				logging.GetLogEntry(r).Warn("Device authentication failed: invalid API key:", err)
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrInvalidAPIKey))
				return
			}

			if device == nil {
				logging.GetLogEntry(r).Warn("Device authentication failed: device not found")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrInvalidAPIKey))
				return
			}

			// Check if device is active
			if !device.IsActive() {
				logging.GetLogEntry(r).Warn("Device authentication failed: device not active, status:", device.Status)
				_ = render.Render(w, r, ErrDeviceForbidden(ErrDeviceInactive))
				return
			}

			// Authentication successful - set device context only (no staff context needed)
			ctx := context.WithValue(r.Context(), CtxDevice, device)

			// Log successful authentication for audit trail
			logging.GetLogEntry(r).Info("Device authentication successful",
				"device_id", device.DeviceID)

			// Update device last seen time
			device.UpdateLastSeen()
			if err := iotService.UpdateDevice(r.Context(), device); err != nil {
				// Log error but don't fail the request
				logging.GetLogEntry(r).Warn("Failed to update device last seen time:", err)
			}

			// Call the next handler with updated context
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
			// Extract API key from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logging.GetLogEntry(r).Warn("Device authentication failed: missing Authorization header")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrMissingAPIKey))
				return
			}

			// Parse Bearer token
			const bearerPrefix = "Bearer "
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				logging.GetLogEntry(r).Warn("Device authentication failed: invalid Authorization header format")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrInvalidAPIKeyFormat))
				return
			}

			apiKey := strings.TrimPrefix(authHeader, bearerPrefix)
			if apiKey == "" {
				logging.GetLogEntry(r).Warn("Device authentication failed: empty API key")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrMissingAPIKey))
				return
			}

			// Validate API key and get device
			device, err := iotService.GetDeviceByAPIKey(r.Context(), apiKey)
			if err != nil {
				logging.GetLogEntry(r).Warn("Device authentication failed: invalid API key:", err)
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrInvalidAPIKey))
				return
			}

			if device == nil {
				logging.GetLogEntry(r).Warn("Device authentication failed: device not found")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrInvalidAPIKey))
				return
			}

			// Check if device is active
			if !device.IsActive() {
				logging.GetLogEntry(r).Warn("Device authentication failed: device not active, status:", device.Status)
				_ = render.Render(w, r, ErrDeviceForbidden(ErrDeviceInactive))
				return
			}

			// Authentication successful - set device context only
			ctx := context.WithValue(r.Context(), CtxDevice, device)

			// Log successful device authentication for audit trail
			logging.GetLogEntry(r).Info("Device-only authentication successful",
				"device_id", device.DeviceID)

			// Update device last seen time
			device.UpdateLastSeen()
			if err := iotService.UpdateDevice(r.Context(), device); err != nil {
				// Log error but don't fail the request
				logging.GetLogEntry(r).Warn("Failed to update device last seen time:", err)
			}

			// Call the next handler with updated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SecureCompareStrings performs a constant-time comparison of two strings to prevent timing attacks
func SecureCompareStrings(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
