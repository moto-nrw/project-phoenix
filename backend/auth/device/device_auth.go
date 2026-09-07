// Package device authenticates kiosk requests by device API key and staff
// PIN. It is the Device Fleet adapter for the IoT routes: it owns the wire
// contract (headers, status codes, error strings) and the principal it hands
// to handlers, while every fact it needs arrives through the ports below. The
// composition root binds those ports to the Device Fleet capability, the
// school directory, the staff PIN verification, and the tenant runtime.
package device

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/render"
)

type CtxKey int

const (
	CtxDevice CtxKey = iota
	CtxStaff
	CtxIsIoTDevice
)

// activeStatus is the one Device Fleet lifecycle state this adapter decides
// on. The vocabulary belongs to the owner; the composition root copies the
// owner's status string verbatim into the principal.
const activeStatus = "active"

// AuthenticatedDevice is the device principal handlers read from the request
// context. It never carries the API key.
type AuthenticatedDevice struct {
	ID         int64
	TenantID   int64
	DeviceID   string
	DeviceType string
	Name       *string
	Status     string
	LastSeen   *time.Time
}

// IsActive reports whether the device may serve requests.
func (d *AuthenticatedDevice) IsActive() bool { return d != nil && d.Status == activeStatus }

// AuthenticatedStaff is the staff principal bound to a request after the
// account PIN verified the caller. Only the identity is exposed; device
// credentials stay separate from the human record behind them.
type AuthenticatedStaff struct {
	ID       int64
	TenantID int64
}

// DeviceDirectory resolves device credentials and records device activity.
// The composition root serves it from the public Device Fleet capability.
type DeviceDirectory interface {
	// FindByAPIKey returns the device that owns apiKey. Any error, and a nil
	// device without an error, both reject the request as an invalid key.
	FindByAPIKey(ctx context.Context, apiKey string) (*AuthenticatedDevice, error)
	// RecordLastSeen writes the device's last_seen instant. It is addressed
	// by the globally unique primary key so a ping stays cross-tenant safe.
	RecordLastSeen(ctx context.Context, deviceID int64, seenAt time.Time) error
}

// Stable outcomes a SchoolLookup reports. Missing schools reject the device;
// an unavailable lookup fails open for the duration of the outage.
var (
	ErrSchoolNotFound          = errors.New("school not found")
	ErrSchoolLookupUnavailable = errors.New("school lookup unavailable")
)

// SchoolLookup answers the pre-tenant question whether the device's school
// still accepts devices. Devices use long-lived API keys, so a deleted school
// must be blocked immediately rather than waiting for token expiry.
//
// Implementations return ErrSchoolNotFound when the row is missing, wrap
// ErrSchoolLookupUnavailable for transient connectivity failures, and return
// any other error for failures that must reject the device.
type SchoolLookup interface {
	IsSchoolDeleted(ctx context.Context, schoolID int64) (bool, error)
}

// StaffPINAuthenticator verifies that a staff ID and account PIN belong to
// the device tenant before the middleware exposes staff identity to handlers.
type StaffPINAuthenticator interface {
	AuthenticateStaffPIN(ctx context.Context, tenantID, staffID int64, pin string) (*AuthenticatedStaff, error)
}

// PINResolver resolves the device PIN for a given tenant.
// Returns the PIN string, or empty if not configured.
type PINResolver func(ctx context.Context, tenantID int64) string

// TenantBinder makes the device's school the ambient tenant of the request.
// Device routes carry no session, so this is where the tenant boundary is
// established. An error marks the device row as unusable and the request is
// rejected like an invalid API key.
type TenantBinder func(ctx context.Context, tenantID int64) (context.Context, error)

// Dependencies are the ports the authenticator needs.
type Dependencies struct {
	// Devices resolves API keys and records activity. Required.
	Devices DeviceDirectory
	// BindTenant establishes the tenant boundary. Required.
	BindTenant TenantBinder
	// Schools rejects devices of deleted schools. A nil lookup skips the
	// check.
	Schools SchoolLookup
	// StaffPIN verifies personal staff credentials. A nil authenticator
	// rejects requests that present them.
	StaffPIN StaffPINAuthenticator
	// PIN resolves the tenant device PIN; FallbackPIN is used when it is nil
	// or returns an empty PIN.
	PIN         PINResolver
	FallbackPIN string
	// LastSeen debounces last_seen writes. Authenticators built from one
	// Dependencies value share it; a nil debouncer gets a fresh one.
	LastSeen *LastSeenDebouncer
}

// Authenticator builds the device middlewares over one set of ports.
type Authenticator struct{ deps Dependencies }

// NewAuthenticator validates the required ports and shares one last-seen
// debouncer between the middlewares it produces.
func NewAuthenticator(deps Dependencies) *Authenticator {
	if deps.Devices == nil || deps.BindTenant == nil {
		panic("device authenticator: device directory and tenant binder are required")
	}
	if deps.LastSeen == nil {
		deps.LastSeen = NewLastSeenDebouncer()
	}
	return &Authenticator{deps: deps}
}

// Device authenticates API key plus device PIN and, when a personal
// credential is supplied, binds the verified staff identity.
func (a *Authenticator) Device() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, errResp := a.authenticateDevice(r.Context(), credentialsFromRequest(r))
			if errResp != nil {
				renderDeviceAuthError(w, r, errResp)
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// DeviceOnly authenticates the API key alone. It sets only the device
// principal and the tenant; handlers behind it must not attribute actions to
// a staff member.
func (a *Authenticator) DeviceOnly() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, errResp := a.authenticateDeviceOnly(r.Context(), credentialsFromRequest(r))
			if errResp != nil {
				renderDeviceAuthError(w, r, errResp)
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RejectAll is the fail-closed stand-in for a device route group whose
// authenticator was never composed. Every request receives the missing-key
// rejection, so a misconfigured graph can never serve kiosk routes
// unauthenticated.
func RejectAll() func(http.Handler) http.Handler {
	return func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slog.Warn("device authentication failed: no device authenticator configured")
			renderDeviceAuthError(w, r, ErrDeviceUnauthorized(ErrMissingAPIKey))
		})
	}
}

// Required returns middleware when a composition root supplied it and the
// fail-closed RejectAll otherwise. Resources call it while mounting their
// device route groups, so a bare handler test or a broken graph logs the
// misconfiguration and never serves kiosk routes unauthenticated.
func Required(name string, middleware func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	if middleware != nil {
		return middleware
	}
	slog.Error("device route group mounted without an authenticator; every request is rejected",
		slog.String("middleware", name),
	)
	return RejectAll()
}

// credentials are the request headers the authenticator reads.
type credentials struct {
	authorization string
	devicePIN     string
	staffID       string
	staffPIN      string
}

func credentialsFromRequest(r *http.Request) credentials {
	return credentials{
		authorization: r.Header.Get("Authorization"),
		devicePIN:     r.Header.Get("X-Staff-PIN"),
		staffID:       r.Header.Get("X-Staff-ID"),
		staffPIN:      r.Header.Get("X-Staff-Auth-PIN"),
	}
}

func (a *Authenticator) authenticateDeviceOnly(ctx context.Context, c credentials) (context.Context, *ErrResponse) {
	device, errResp := a.resolveDevice(ctx, c.authorization)
	if errResp != nil {
		return nil, errResp
	}
	tenantCtx, err := a.deps.BindTenant(ctx, device.TenantID)
	if err != nil {
		return nil, ErrDeviceUnauthorized(ErrInvalidAPIKey)
	}
	tenantCtx = context.WithValue(tenantCtx, CtxDevice, device)

	slog.Info("device-only authentication successful",
		slog.String("device_id", device.DeviceID),
	)
	a.deps.LastSeen.updateDeviceLastSeen(ctx, a.deps.Devices, device)
	return tenantCtx, nil
}

func (a *Authenticator) authenticateDevice(ctx context.Context, c credentials) (context.Context, *ErrResponse) {
	device, errResp := a.resolveDevice(ctx, c.authorization)
	if errResp != nil {
		return nil, errResp
	}
	tenantCtx, err := a.deps.BindTenant(ctx, device.TenantID)
	if err != nil {
		return nil, ErrDeviceUnauthorized(ErrInvalidAPIKey)
	}
	if errResp := a.validateDevicePIN(ctx, device, c.devicePIN); errResp != nil {
		return nil, errResp
	}
	staff, errResp := a.authenticateStaff(ctx, device, c.staffID, c.staffPIN)
	if errResp != nil {
		return nil, errResp
	}

	tenantCtx = context.WithValue(tenantCtx, CtxDevice, device)
	tenantCtx = context.WithValue(tenantCtx, CtxIsIoTDevice, true)
	if staff != nil {
		tenantCtx = context.WithValue(tenantCtx, CtxStaff, staff)
	}
	slog.Debug("device authentication successful",
		slog.String("device_id", device.DeviceID),
	)
	a.deps.LastSeen.updateDeviceLastSeen(ctx, a.deps.Devices, device)
	return tenantCtx, nil
}

// resolveDevice turns the Authorization header into an active device whose
// school still exists.
func (a *Authenticator) resolveDevice(ctx context.Context, authorization string) (*AuthenticatedDevice, *ErrResponse) {
	device, errResp := a.extractAndValidateAPIKey(ctx, authorization)
	if errResp != nil {
		return nil, errResp
	}
	if errResp := a.rejectDeletedSchool(ctx, device); errResp != nil {
		return nil, errResp
	}
	return device, nil
}

// extractAndValidateAPIKey parses the bearer token and resolves the device.
func (a *Authenticator) extractAndValidateAPIKey(ctx context.Context, authorization string) (*AuthenticatedDevice, *ErrResponse) {
	if authorization == "" {
		slog.Warn("device authentication failed: missing Authorization header")
		return nil, ErrDeviceUnauthorized(ErrMissingAPIKey)
	}

	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authorization, bearerPrefix) {
		slog.Warn("device authentication failed: invalid Authorization header format")
		return nil, ErrDeviceUnauthorized(ErrInvalidAPIKeyFormat)
	}

	apiKey := strings.TrimPrefix(authorization, bearerPrefix)
	if apiKey == "" {
		slog.Warn("device authentication failed: empty API key")
		return nil, ErrDeviceUnauthorized(ErrMissingAPIKey)
	}

	device, err := a.deps.Devices.FindByAPIKey(ctx, apiKey)
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
	if !device.IsActive() {
		slog.Warn("device authentication failed: device not active",
			slog.String("status", device.Status),
		)
		return nil, ErrDeviceForbidden(ErrDeviceInactive)
	}
	return device, nil
}

// rejectDeletedSchool blocks devices whose school is gone or soft-deleted.
// It runs before any tenant transaction exists. Only a transient lookup
// failure fails open, so a brief outage does not take every kiosk offline;
// every other failure fails closed so the soft-delete guard cannot be
// bypassed.
func (a *Authenticator) rejectDeletedSchool(ctx context.Context, device *AuthenticatedDevice) *ErrResponse {
	if a.deps.Schools == nil || device.TenantID <= 0 {
		return nil
	}
	deleted, err := a.deps.Schools.IsSchoolDeleted(ctx, device.TenantID)
	switch {
	case errors.Is(err, ErrSchoolNotFound):
		slog.Warn("device authentication rejected: school not found",
			slog.String("device_id", device.DeviceID),
			slog.Int64("tenant_id", device.TenantID),
		)
		return ErrDeviceForbidden(ErrDeviceInactive)
	case errors.Is(err, ErrSchoolLookupUnavailable):
		slog.Warn("school lookup failed during device auth, failing open (transient)",
			slog.String("device_id", device.DeviceID),
			slog.Int64("tenant_id", device.TenantID),
			slog.String("error", err.Error()),
		)
		return nil
	case err != nil:
		slog.Error("school lookup failed during device auth, rejecting device",
			slog.String("device_id", device.DeviceID),
			slog.Int64("tenant_id", device.TenantID),
			slog.String("error", err.Error()),
		)
		return ErrDeviceForbidden(ErrDeviceInactive)
	case deleted:
		slog.Warn("device authentication rejected: school is soft-deleted",
			slog.String("device_id", device.DeviceID),
			slog.Int64("tenant_id", device.TenantID),
		)
		return ErrDeviceForbidden(ErrDeviceInactive)
	}
	return nil
}

func (a *Authenticator) resolveDevicePIN(ctx context.Context, tenantID int64) string {
	if a.deps.PIN != nil && tenantID > 0 {
		if pin := a.deps.PIN(ctx, tenantID); pin != "" {
			return pin
		}
	}

	slog.Warn("settings service returned no PIN, falling back to OGS_DEVICE_PIN env var",
		slog.Int64("tenant_id", tenantID),
	)
	return a.deps.FallbackPIN
}

func (a *Authenticator) validateDevicePIN(ctx context.Context, device *AuthenticatedDevice, staffPIN string) *ErrResponse {
	if staffPIN == "" {
		slog.Warn("device authentication failed: missing X-Staff-PIN header")
		return ErrDeviceUnauthorized(ErrMissingPIN)
	}

	ogsPIN := a.resolveDevicePIN(ctx, device.TenantID)
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

func (a *Authenticator) authenticateStaff(ctx context.Context, device *AuthenticatedDevice, staffIDHeader, staffPIN string) (*AuthenticatedStaff, *ErrResponse) {
	if staffIDHeader == "" && staffPIN == "" {
		return nil, nil
	}
	// Deployed PyrePortal versions send X-Staff-ID without a personal PIN.
	// Keep those requests working, but do not trust or expose the ID.
	if staffPIN == "" {
		return nil, nil
	}
	if staffIDHeader == "" || a.deps.StaffPIN == nil {
		slog.Warn("device staff authentication failed: incomplete credentials",
			slog.String("device_id", device.DeviceID),
		)
		return nil, ErrDeviceUnauthorized(ErrInvalidPIN)
	}

	staffID, err := strconv.ParseInt(staffIDHeader, 10, 64)
	if err != nil || staffID <= 0 {
		slog.Warn("device staff authentication failed: invalid staff ID",
			slog.String("device_id", device.DeviceID),
		)
		return nil, ErrDeviceUnauthorized(ErrInvalidPIN)
	}

	staff, err := a.deps.StaffPIN.AuthenticateStaffPIN(ctx, device.TenantID, staffID, staffPIN)
	if err != nil || staff == nil || staff.TenantID != device.TenantID {
		slog.Warn("device staff authentication failed",
			slog.String("device_id", device.DeviceID),
			slog.Int64("staff_id", staffID),
		)
		return nil, ErrDeviceUnauthorized(ErrInvalidPIN)
	}
	return staff, nil
}

func renderDeviceAuthError(w http.ResponseWriter, r *http.Request, errResp *ErrResponse) {
	if err := render.Render(w, r, errResp); err != nil {
		slog.Error("failed to render device auth error", slog.String("error", err.Error()))
	}
}

// DeviceFromCtx retrieves the authenticated device from request context.
func DeviceFromCtx(ctx context.Context) *AuthenticatedDevice {
	device, ok := ctx.Value(CtxDevice).(*AuthenticatedDevice)
	if !ok {
		return nil
	}
	return device
}

// StaffFromCtx retrieves the authenticated staff from request context.
func StaffFromCtx(ctx context.Context) *AuthenticatedStaff {
	staff, ok := ctx.Value(CtxStaff).(*AuthenticatedStaff)
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

// SecureCompareStrings performs a constant-time comparison of two strings to prevent timing attacks
func SecureCompareStrings(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// =============================================================================
// Last-seen debouncing
// =============================================================================

const lastSeenDebounceWindow = 60 * time.Second

type lastSeenDebounceState struct {
	mu            sync.Mutex
	lastPersisted time.Time
	latestSeen    time.Time
	flushTimer    *time.Timer
	writeInFlight bool
}

// LastSeenDebouncer coalesces last_seen writes per device so a chatty kiosk
// produces at most one write per debounce window.
type LastSeenDebouncer struct{ cache sync.Map }

func NewLastSeenDebouncer() *LastSeenDebouncer { return &LastSeenDebouncer{} }

// updateDeviceLastSeen updates the device's last seen timestamp, logging any errors.
// Uses device.ID (PK, globally unique) for the debounce cache key and DB update
// to avoid cross-tenant collisions when device_id strings overlap.
func (d *LastSeenDebouncer) updateDeviceLastSeen(ctx context.Context, devices DeviceDirectory, device *AuthenticatedDevice) {
	now := time.Now()
	device.LastSeen = &now

	state := d.getOrCreateLastSeenState(device.ID)
	state.mu.Lock()
	state.latestSeen = now

	shouldWriteNow := !state.writeInFlight && (state.lastPersisted.IsZero() || (now.Sub(state.lastPersisted) >= lastSeenDebounceWindow && state.flushTimer == nil))
	if shouldWriteNow {
		state.writeInFlight = true
		state.mu.Unlock()
		d.persistLastSeen(ctx, devices, device.ID, now, state)
		return
	}

	if !state.writeInFlight {
		d.scheduleDeferredFlushLocked(devices, device.ID, state, now)
	}
	state.mu.Unlock()
}

func (d *LastSeenDebouncer) getOrCreateLastSeenState(id int64) *lastSeenDebounceState {
	if existing, ok := d.cache.Load(id); ok {
		if state, ok := existing.(*lastSeenDebounceState); ok {
			return state
		}
	}

	state := &lastSeenDebounceState{}
	actual, _ := d.cache.LoadOrStore(id, state)
	actualState, _ := actual.(*lastSeenDebounceState)
	return actualState
}

func (d *LastSeenDebouncer) persistLastSeen(ctx context.Context, devices DeviceDirectory, id int64, observedAt time.Time, state *lastSeenDebounceState) {
	if err := devices.RecordLastSeen(ctx, id, observedAt); err != nil {
		slog.Warn("failed to update device last seen time",
			slog.Int64("device_pk", id),
			slog.String("error", err.Error()),
		)
		state.mu.Lock()
		state.writeInFlight = false
		if state.latestSeen.After(observedAt) {
			state.lastPersisted = observedAt
		}
		d.scheduleDeferredFlushLocked(devices, id, state, time.Now())
		state.mu.Unlock()
		return
	}

	state.mu.Lock()
	state.lastPersisted = observedAt
	state.writeInFlight = false
	d.scheduleDeferredFlushLocked(devices, id, state, time.Now())
	state.mu.Unlock()
}

func (d *LastSeenDebouncer) flushDeferredLastSeen(devices DeviceDirectory, id int64, state *lastSeenDebounceState) {
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

	d.persistLastSeen(context.Background(), devices, id, latestSeen, state)
}

func (d *LastSeenDebouncer) scheduleDeferredFlushLocked(devices DeviceDirectory, id int64, state *lastSeenDebounceState, now time.Time) {
	if state.writeInFlight || state.flushTimer != nil || state.lastPersisted.IsZero() || !state.latestSeen.After(state.lastPersisted) {
		return
	}

	delay := state.lastPersisted.Add(lastSeenDebounceWindow).Sub(now)
	if delay < 0 {
		delay = 0
	}

	state.flushTimer = time.AfterFunc(delay, func() {
		d.flushDeferredLastSeen(devices, id, state)
	})
}
