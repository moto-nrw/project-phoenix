// Package testutil provides shared test utilities for API handler tests.
//
// # Test Pattern
//
// API tests follow the hermetic test pattern established in the codebase:
// - Real database with test fixtures
// - Real services via route/module-sized builders
// - httptest for HTTP request/response
// - Context injection for JWT claims and permissions
//
// Example:
//
//	func setupAuthRoute(t *testing.T) (*bun.DB, *Resource) {
//	    db, module := testutil.SetupAuthModule(t)
//	    return db, NewResource(module.Auth, module.Invitation)
//	}
//
//	func TestHandler(t *testing.T) {
//	    db, resource := setupAuthRoute(t)
//	    router := chi.NewRouter()
//	    router.Mount("/auth", resource.Router())
//
//	    req := testutil.NewAuthenticatedRequest("GET", "/auth/account", nil,
//	        testutil.WithPermissions("users:read"),
//	        testutil.WithClaims(sessiontoken.AppClaims{ID: 1, Username: "test"}),
//	    )
//
//	    rr := testutil.ExecuteRequest(router, req)
//	    testutil.AssertSuccessResponse(t, rr, http.StatusOK)
//	}
package testutil

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/devicetest"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// DeviceIdentity exposes the authenticated principal facts to HTTP adapter
// runtimes without making each adapter test import the device middleware.
func DeviceIdentity(ctx context.Context) (int64, string, bool) {
	principal := devicetest.DeviceFromContext(ctx)
	if principal == nil {
		return 0, "", false
	}
	return principal.ID, principal.DeviceID, true
}

// NewDeviceAuthenticators composes the production device authentication
// middleware for handler tests. settings may be nil to authenticate with
// fallbackPIN alone.
func NewDeviceAuthenticators(
	devices deviceauth.Fleet,
	schools deviceauth.SchoolDirectory,
	settings deviceauth.Settings,
	fallbackPIN string,
) *deviceauth.Authenticators {
	return deviceauth.New(deviceauth.Dependencies{
		Devices:      devices,
		Schools:      schools,
		SchoolErrors: devicetest.SchoolErrors(),
		Settings:     settings,
		FallbackPIN:  fallbackPIN,
	})
}

// AuthenticatedDeviceID reads the kiosk the device authenticator admitted on
// a request, as production binds it for routes that attribute the kiosk.
func AuthenticatedDeviceID(ctx context.Context) (string, bool) {
	return deviceauth.DeviceID(ctx)
}

// DevicePrincipal converts a device row into the principal the device auth
// middleware binds to a request, so handler tests see exactly what
// production handlers see. It never copies the API key.
func DevicePrincipal(d *testpkg.Device) *devicetest.Principal {
	if d == nil {
		return nil
	}
	return &devicetest.Principal{
		ID:         d.ID,
		TenantID:   d.TenantID,
		DeviceID:   d.DeviceID,
		DeviceType: d.DeviceType,
		Name:       d.Name,
		Status:     d.Status,
		LastSeen:   d.LastSeen,
	}
}

// WithDeviceActor binds the device and staff principals the device auth
// middleware would bind for a verified kiosk request, so a scenario can call
// a presence capability directly as that actor without naming the auth
// context keys itself. The staff member belongs to the device's school.
func WithDeviceActor(ctx context.Context, d *testpkg.Device, staffID int64) context.Context {
	return devicetest.WithStaff(devicetest.WithDevice(ctx, DevicePrincipal(d)), staffID, d.TenantID)
}

// WithDeviceContext adds an IoT device to the request context.
// This is used for testing device-authenticated endpoints.
// Also injects the device's tenant_id so TenantTxMiddleware can create
// a tenant-scoped transaction (mirrors production device auth middleware).
func WithDeviceContext(d *testpkg.Device) RequestOption {
	return func(req *http.Request) {
		ctx := devicetest.WithDevice(req.Context(), DevicePrincipal(d))
		if tid := d.TenantID; tid != 0 {
			ctx = tenant.WithTenantID(ctx, tid)
		}
		*req = *req.WithContext(ctx)
	}
}

// WithDeviceIdentity supplies a device principal to hermetic HTTP adapter tests
// that do not need a persisted device or a tenant transaction.
func WithDeviceIdentity(id int64, deviceID string) RequestOption {
	return func(req *http.Request) {
		principal := &devicetest.Principal{ID: id, DeviceID: deviceID}
		*req = *req.WithContext(devicetest.WithDevice(req.Context(), principal))
	}
}

// Rollback helpers exercise the same tenant marker as production middleware.
func MarkRollback(ctx context.Context)                       { tenant.MarkRollback(ctx) }
func WithRollbackMarker(ctx context.Context) context.Context { return tenant.WithRollbackMarker(ctx) }
func RollbackRequested(ctx context.Context) bool             { return tenant.RollbackRequested(ctx) }

// WithIoTDeviceRequest marks the request as a kiosk request, the way the
// device middleware does, so services authorize through the device session.
func WithIoTDeviceRequest() RequestOption {
	return func(req *http.Request) {
		*req = *req.WithContext(devicetest.WithIoTDevice(req.Context()))
	}
}

// WithStaffContext adds a staff member to the request context, as the
// principal a verified web boundary binds. This is used for testing endpoints
// that require staff authentication.
func WithStaffContext(staffID, tenantID int64) RequestOption {
	return func(req *http.Request) {
		*req = *req.WithContext(devicetest.WithStaff(req.Context(), staffID, tenantID))
	}
}
