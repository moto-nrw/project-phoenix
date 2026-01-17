package device

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/render"
	adaptermiddleware "github.com/moto-nrw/project-phoenix/internal/adapter/middleware"
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
		recordDeviceAuthError(r.Context(), ErrMissingAPIKey)
		return nil, ErrDeviceUnauthorized(ErrMissingAPIKey)
	}

	// Parse Bearer token
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		recordDeviceAuthError(r.Context(), ErrInvalidAPIKeyFormat)
		return nil, ErrDeviceUnauthorized(ErrInvalidAPIKeyFormat)
	}

	apiKey := strings.TrimPrefix(authHeader, bearerPrefix)
	if apiKey == "" {
		recordDeviceAuthError(r.Context(), ErrMissingAPIKey)
		return nil, ErrDeviceUnauthorized(ErrMissingAPIKey)
	}

	// Validate API key and get device
	device, err := iotService.GetDeviceByAPIKey(r.Context(), apiKey)
	if err != nil {
		recordDeviceAuthError(r.Context(), ErrInvalidAPIKey)
		return nil, ErrDeviceUnauthorized(ErrInvalidAPIKey)
	}

	if device == nil {
		recordDeviceAuthError(r.Context(), ErrInvalidAPIKey)
		return nil, ErrDeviceUnauthorized(ErrInvalidAPIKey)
	}

	// Check if device is active
	if !device.IsActive() {
		recordDeviceAuthError(r.Context(), ErrDeviceInactive)
		return nil, ErrDeviceForbidden(ErrDeviceInactive)
	}

	return device, nil
}

// updateDeviceLastSeen updates the device's last seen timestamp, logging any errors.
func updateDeviceLastSeen(r *http.Request, iotService iotSvc.Service, device *iot.Device) {
	device.UpdateLastSeen()
	if err := iotService.UpdateDevice(r.Context(), device); err != nil {
		recordDeviceSideEffectError(r.Context(), err)
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
				recordDeviceAuthError(r.Context(), ErrMissingPIN)
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrMissingPIN))
				return
			}

			if ogsPin == "" {
				recordDeviceAuthErrorMessage(r.Context(), "OGS_DEVICE_PIN not configured at startup", "pin_not_configured")
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrInvalidPIN))
				return
			}

			// Validate PIN using constant-time comparison
			if !SecureCompareStrings(staffPIN, ogsPin) {
				recordDeviceAuthError(r.Context(), ErrInvalidPIN)
				_ = render.Render(w, r, ErrDeviceUnauthorized(ErrInvalidPIN))
				return
			}

			// Authentication successful - set device context
			ctx := context.WithValue(r.Context(), CtxDevice, device)
			ctx = context.WithValue(ctx, CtxIsIoTDevice, true)

			recordDeviceActor(ctx, device)
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

			recordDeviceActor(ctx, device)
			updateDeviceLastSeen(r, iotService, device)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SecureCompareStrings performs a constant-time comparison of two strings to prevent timing attacks
func SecureCompareStrings(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func recordDeviceActor(ctx context.Context, device *iot.Device) {
	if device == nil {
		return
	}
	event := adaptermiddleware.GetWideEvent(ctx)
	if event == nil || event.Timestamp.IsZero() {
		return
	}
	if event.UserID == "" {
		event.UserID = device.DeviceID
	}
	if event.UserRole == "" {
		event.UserRole = "device"
	}
}

func recordDeviceAuthError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	recordDeviceAuthErrorMessage(ctx, err.Error(), deviceAuthErrorCode(err))
}

func recordDeviceAuthErrorMessage(ctx context.Context, message string, code string) {
	event := adaptermiddleware.GetWideEvent(ctx)
	if event == nil || event.Timestamp.IsZero() || event.ErrorType != "" {
		return
	}
	event.ErrorType = "device_auth"
	if code != "" {
		event.ErrorCode = code
	}
	event.ErrorMessage = message
}

func recordDeviceSideEffectError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	event := adaptermiddleware.GetWideEvent(ctx)
	if event == nil || event.Timestamp.IsZero() || event.ErrorType != "" {
		return
	}
	event.ErrorType = "device_last_seen_update"
	event.ErrorCode = "last_seen_update_failed"
	event.ErrorMessage = err.Error()
}

func deviceAuthErrorCode(err error) string {
	switch err {
	case ErrMissingAPIKey:
		return "missing_api_key"
	case ErrInvalidAPIKey:
		return "invalid_api_key"
	case ErrInvalidAPIKeyFormat:
		return "invalid_api_key_format"
	case ErrMissingPIN:
		return "missing_pin"
	case ErrInvalidPIN:
		return "invalid_pin"
	case ErrMissingStaffID:
		return "missing_staff_id"
	case ErrInvalidStaffID:
		return "invalid_staff_id"
	case ErrDeviceInactive:
		return "device_inactive"
	case ErrStaffAccountLocked:
		return "staff_account_locked"
	case ErrDeviceOffline:
		return "device_offline"
	case ErrPINAttemptsExceeded:
		return "pin_attempts_exceeded"
	default:
		return ""
	}
}
