package device

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/render"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/models/users"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun/driver/pgdriver"
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

// PINResolver resolves the device PIN for a given tenant.
// Returns the PIN string, or empty if not configured.
type PINResolver func(ctx context.Context, tenantID int64) string

// SchoolLookup is the narrow school read the device authenticators need
// (issue #584: handlers must not wire repositories; satisfied by
// services/platform.SchoolService, which returns repository results and
// errors verbatim).
type SchoolLookup interface {
	GetSchoolByID(ctx context.Context, id int64) (*platform.School, error)
}

func renderDeviceAuthError(w http.ResponseWriter, r *http.Request, errResp render.Renderer) {
	if err := render.Render(w, r, errResp); err != nil {
		slog.Error("failed to render device auth error", slog.String("error", err.Error()))
	}
}

func resolveDevicePIN(ctx context.Context, tenantID int64, pinResolver PINResolver) string {
	if pinResolver != nil && tenantID > 0 {
		if pin := pinResolver(ctx, tenantID); pin != "" {
			return pin
		}
	}

	slog.Warn("settings service returned no PIN, falling back to OGS_DEVICE_PIN env var",
		slog.Int64("tenant_id", tenantID),
	)
	return os.Getenv("OGS_DEVICE_PIN")
}

func validateDevicePIN(r *http.Request, device *iot.Device, pinResolver PINResolver) render.Renderer {
	staffPIN := r.Header.Get("X-Staff-PIN")
	if staffPIN == "" {
		slog.Warn("device authentication failed: missing X-Staff-PIN header")
		return ErrDeviceUnauthorized(ErrMissingPIN)
	}

	ogsPIN := resolveDevicePIN(r.Context(), device.TenantID, pinResolver)
	if ogsPIN == "" {
		slog.Error("OGS_DEVICE_PIN not configured")
		return ErrDeviceUnauthorized(ErrInvalidPIN)
	}

	if !SecureCompareStrings(staffPIN, ogsPIN) {
		slog.Warn("device authentication failed: invalid PIN")
		return ErrDeviceUnauthorized(ErrInvalidPIN)
	}

	return nil
}

func authenticatedDeviceContext(r *http.Request, device *iot.Device) context.Context {
	ctx := context.WithValue(r.Context(), CtxDevice, device)
	ctx = context.WithValue(ctx, CtxIsIoTDevice, true)

	// Device-auth routes don't use jwt.TenantMiddleware.
	if device.TenantID > 0 {
		ctx = tenant.WithTenantID(ctx, device.TenantID)
	}
	return ctx
}

func serveAuthenticatedDeviceRequest(
	w http.ResponseWriter,
	r *http.Request,
	next http.Handler,
	iotService iotSvc.Service,
	schools SchoolLookup,
	pinResolver PINResolver,
) {
	device, errResp := extractAndValidateAPIKey(r, iotService)
	if errResp != nil {
		renderDeviceAuthError(w, r, errResp)
		return
	}

	// Devices use long-lived API keys, so deleted schools must be blocked
	// immediately rather than waiting for token expiry.
	if errResp := rejectDeletedSchool(r.Context(), schools, device); errResp != nil {
		renderDeviceAuthError(w, r, errResp)
		return
	}

	if errResp := validateDevicePIN(r, device, pinResolver); errResp != nil {
		renderDeviceAuthError(w, r, errResp)
		return
	}

	ctx := authenticatedDeviceContext(r, device)
	slog.Debug("device authentication successful",
		slog.String("device_id", device.DeviceID),
	)
	updateDeviceLastSeen(r, iotService, device)
	next.ServeHTTP(w, r.WithContext(ctx))
}

// DeviceAuthenticator is a middleware that validates device API keys and the global OGS PIN.
// It requires both Authorization: Bearer <api_key> and X-Staff-PIN: <pin> headers.
// The middleware sets device context for downstream handlers.
// Rejects requests for devices belonging to soft-deleted schools.
// pinResolver is optional — if nil, falls back to OGS_DEVICE_PIN env var.
func DeviceAuthenticator(iotService iotSvc.Service, schools SchoolLookup, pinResolver PINResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serveAuthenticatedDeviceRequest(w, r, next, iotService, schools, pinResolver)
		})
	}
}

// DeviceOnlyAuthenticator is a middleware that validates only device API keys.
// It requires only Authorization: Bearer <api_key> header (no staff PIN required).
// The middleware sets device context for downstream handlers.
// This is used for endpoints that need device authentication but not staff authentication,
// such as getting the list of available teachers for login selection.
// Rejects requests for devices belonging to soft-deleted schools.
func DeviceOnlyAuthenticator(iotService iotSvc.Service, schools SchoolLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Validate API key and get device
			device, errResp := extractAndValidateAPIKey(r, iotService)
			if errResp != nil {
				if err := render.Render(w, r, errResp); err != nil {
					slog.Error("failed to render device auth error", slog.String("error", err.Error()))
				}
				return
			}

			// Reject requests for devices belonging to soft-deleted schools.
			if errResp := rejectDeletedSchool(r.Context(), schools, device); errResp != nil {
				if err := render.Render(w, r, errResp); err != nil {
					slog.Error("failed to render device auth error", slog.String("error", err.Error()))
				}
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

// rejectDeletedSchool checks if the device's school has been soft-deleted.
// Returns an error renderer if the school is deleted, nil otherwise.
// Runs before TenantTxMiddleware, so there is no tenant transaction in
// context; the underlying repository falls back to the raw *bun.DB connection
// which is fine because platform.schools is not behind RLS. Fails open only
// on transient DB errors (connection failures) to avoid breaking all IoT
// devices during outages. "Not found" errors (school row deleted/missing)
// are treated as rejection.
func rejectDeletedSchool(ctx context.Context, schools SchoolLookup, device *iot.Device) render.Renderer {
	if schools == nil || device.TenantID <= 0 {
		return nil
	}
	school, err := schools.GetSchoolByID(ctx, device.TenantID)
	if err != nil {
		// Distinguish "not found" from transient connectivity errors.
		// The repository wraps sql.ErrNoRows in a DatabaseError — unwrap and
		// check.
		if isNotFoundErr(err) {
			slog.Warn("device authentication rejected: school not found",
				slog.String("device_id", device.DeviceID),
				slog.Int64("tenant_id", device.TenantID),
			)
			return ErrDeviceForbidden(ErrDeviceInactive)
		}
		// Fail open ONLY on transient connectivity errors (net timeouts,
		// connection resets, driver-level connection failures) to avoid
		// breaking all IoT devices during brief outages. All other errors
		// (permission issues, serialization failures, bad queries) fail
		// closed to prevent bypassing the soft-delete guard.
		if isTransientDBErr(err) {
			slog.Warn("school lookup failed during device auth, failing open (transient)",
				slog.String("device_id", device.DeviceID),
				slog.Int64("tenant_id", device.TenantID),
				slog.String("error", err.Error()),
			)
			return nil
		}
		// Non-transient, non-not-found error — fail closed.
		slog.Error("school lookup failed during device auth, rejecting device",
			slog.String("device_id", device.DeviceID),
			slog.Int64("tenant_id", device.TenantID),
			slog.String("error", err.Error()),
		)
		return ErrDeviceForbidden(ErrDeviceInactive)
	}
	if school == nil {
		// School genuinely doesn't exist — reject the device.
		return ErrDeviceForbidden(ErrDeviceInactive)
	}
	if school.IsDeleted() {
		slog.Warn("device authentication rejected: school is soft-deleted",
			slog.String("device_id", device.DeviceID),
			slog.Int64("tenant_id", device.TenantID),
		)
		return ErrDeviceForbidden(ErrDeviceInactive)
	}
	return nil
}

// isTransientDBErr returns true for errors that indicate a temporary
// connectivity problem (net timeouts, connection resets, context
// cancellation, PostgreSQL connection-class SQLSTATE 08xxx).
// Everything else (bad queries, permission errors, serialization
// failures) returns false so the caller can fail closed.
func isTransientDBErr(err error) bool {
	if err == nil {
		return false
	}

	// Context-level timeouts / cancellations.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	// Net-level errors (timeouts, connection resets).
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// pgdriver connection-class errors (SQLSTATE 08xxx).
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		code := pgErr.Field('C')
		if len(code) >= 2 && code[:2] == "08" {
			return true
		}
	}

	// Unwrap DatabaseError and check the inner error.
	var dbErr *modelBase.DatabaseError
	if errors.As(err, &dbErr) {
		return isTransientDBErr(dbErr.Err)
	}

	return false
}

// isNotFoundErr checks if an error represents a "not found" condition,
// unwrapping DatabaseError if necessary.
func isNotFoundErr(err error) bool {
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	var dbErr *modelBase.DatabaseError
	if errors.As(err, &dbErr) {
		return errors.Is(dbErr.Err, sql.ErrNoRows)
	}
	return false
}

// SecureCompareStrings performs a constant-time comparison of two strings to prevent timing attacks
func SecureCompareStrings(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
