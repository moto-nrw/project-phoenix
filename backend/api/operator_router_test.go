package api

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	provisioningoperator "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	settingsoperator "github.com/moto-nrw/project-phoenix/modules/settings/inbound/operator"
)

// chiWalk traverses all routes in the chi.Router and calls fn for each
// concrete pattern. Used to assert mount points in the operator router.
func chiWalk(router chi.Router, fn func(pattern string)) error {
	return chi.Walk(router, func(_ string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		fn(route)
		return nil
	})
}

// stubSchoolSettings stands in for the school settings so that the school
// settings routes get wired. Handler behavior is exercised with
// Settings Platform's operator routes.
type stubSchoolSettings struct {
	settings.OperatorSchoolSettings
}

type stubBillingReport struct {
	organizationtenancy.BillingReport
}

type protectedRouteProvisioningService struct {
	organizationtenancy.Provisioning
	createSchoolFn func(context.Context, *organizationtenancy.CreateSchool, int64, net.IP) (*organizationtenancy.School, error)
}

func (s *protectedRouteProvisioningService) CreateSchool(ctx context.Context, school *organizationtenancy.CreateSchool, operatorID int64, clientIP net.IP) (*organizationtenancy.School, error) {
	if s.createSchoolFn != nil {
		return s.createSchoolFn(ctx, school, operatorID, clientIP)
	}
	return &organizationtenancy.School{OrganizationID: school.OrganizationID, Name: school.Name, Slug: school.Slug, Subdomain: school.Subdomain, Email: school.Email}, nil
}

// TestOperatorIdentityRoutes verifies that the Identity & Access routes of
// the operator router can be built.
func TestOperatorIdentityRoutes(t *testing.T) {
	t.Parallel()

	t.Run("builds the routes with nil services", func(t *testing.T) {
		routes := operatorIdentity{
			access:   nil,
			sessions: identityoperator.Sessions{},
		}.routes()
		require.NotNil(t, routes.mfa)
	})

	t.Run("builds the routes with provided sessions", func(t *testing.T) {
		routes := operatorIdentity{
			access:   nil,
			sessions: operatorTestSessions(t, nil),
		}.routes()
		require.NotNil(t, routes.mfa)
	})

	t.Run("accepts inputs whose sessions the root provides later", func(t *testing.T) {
		routes := operatorIdentity{}.routes()
		require.NotNil(t, routes.mfa)
	})
}

// TestRouter verifies that the operator router can be constructed.
func TestRouter(t *testing.T) {
	t.Parallel()

	t.Run("creates router successfully", func(t *testing.T) {
		router := testOperatorRouter(operatorIdentity{sessions: operatorTestSessions(t, nil)}, nil)
		require.NotNil(t, router)
	})

	t.Run("names the missing sessions instead of dereferencing nil", func(t *testing.T) {
		require.PanicsWithValue(t,
			"operator api: the operator session chains are required to mount the operator routes",
			func() { testOperatorRouter(operatorIdentity{}, nil) },
		)
	})

	t.Run("router has expected routes", func(t *testing.T) {
		router := testOperatorRouter(operatorIdentity{sessions: operatorTestSessions(t, nil)}, nil)
		require.NotNil(t, router)

		routes := router.Routes()
		assert.NotEmpty(t, routes)
	})

	t.Run("settings routes are mounted when SchoolSettings is provided", func(t *testing.T) {
		schoolSettings := &operatorSchoolSettings{
			values: settingsoperator.NewSettingsResource(settingsoperator.SettingsConfig{
				Settings:   stubSchoolSettings{},
				OperatorID: func(context.Context) int64 { return 0 },
			}),
			bookingAuthorityImpact: http.NotFoundHandler(),
		}
		router := operatorRouter(operatorIdentity{sessions: operatorTestSessions(t, nil)},
			provisioningoperator.ProvisioningConfig{}, nil, schoolSettings, nil, nil, authRateLimiters{}, http.NotFoundHandler())
		require.NotNil(t, router)

		// Walk the route tree and assert the settings subroute exists under /schools.
		found := false
		bookingAuthority := false
		err := chiWalk(router, func(pattern string) {
			if pattern == "/schools/{id}/settings/schema" ||
				pattern == "/schools/{id}/settings/values/{key}" ||
				pattern == "/schools/{id}/settings/values/{key}/reveal" {
				found = true
			}
			if pattern == "/schools/{id}/settings/booking-authority-impact" {
				bookingAuthority = true
			}
		})
		require.NoError(t, err)
		assert.True(t, found, "expected /schools/{id}/settings/* routes to be mounted when SchoolSettings is provided")
		assert.True(t, bookingAuthority, "expected Care Plan's booking authority review below the school settings")
	})

	t.Run("school settings require Care Plan's booking authority review", func(t *testing.T) {
		schoolSettings := &operatorSchoolSettings{
			values: settingsoperator.NewSettingsResource(settingsoperator.SettingsConfig{
				Settings:   stubSchoolSettings{},
				OperatorID: func(context.Context) int64 { return 0 },
			}),
		}
		assert.Panics(t, func() {
			operatorRouter(operatorIdentity{sessions: operatorTestSessions(t, nil)},
				provisioningoperator.ProvisioningConfig{}, nil, schoolSettings, nil, nil, authRateLimiters{}, http.NotFoundHandler())
		})
	})

	t.Run("settings routes are NOT mounted when SchoolSettings is nil", func(t *testing.T) {
		router := testOperatorRouter(operatorIdentity{sessions: operatorTestSessions(t, nil)}, nil)

		found := false
		err := chiWalk(router, func(pattern string) {
			if pattern == "/schools/{id}/settings/schema" {
				found = true
			}
		})
		require.NoError(t, err)
		assert.False(t, found, "expected settings routes NOT to be mounted when SchoolSettings is nil")
	})

	t.Run("billing seed route is mounted with the billing report", func(t *testing.T) {
		billing := provisioningoperator.NewBillingResource(stubBillingReport{}, nil, nil)
		router := operatorRouter(operatorIdentity{sessions: operatorTestSessions(t, nil)},
			provisioningoperator.ProvisioningConfig{}, billing, nil, nil, nil, authRateLimiters{}, http.NotFoundHandler())

		found := false
		err := chiWalk(router, func(pattern string) {
			if pattern == "/billing/key-date-counts/seed" {
				found = true
			}
		})
		require.NoError(t, err)
		assert.True(t, found, "expected local billing seed route when billing is configured")
	})
}

// TestUnregisteredTagScanRoutesSitBehindOperatorAuth pins the mount of the
// Device Fleet scan review (#3232): the router hands it the rest of the path
// only after the operator auth chain accepted the caller.
func TestUnregisteredTagScanRoutesSitBehindOperatorAuth(t *testing.T) {
	t.Parallel()

	var reached []string
	review := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = append(reached, r.Method+" "+chi.RouteContext(r.Context()).RoutePath)
		w.WriteHeader(http.StatusTeapot)
	})
	authService := &mockOperatorAuthService{
		getOperatorFn: func(_ context.Context, id int64) (*identityaccess.Operator, error) {
			op := &identityaccess.Operator{Active: true}
			op.ID = id
			return op, nil
		},
	}
	router := operatorRouter(operatorIdentity{access: authService, sessions: operatorTestSessions(t, authService)},
		provisioningoperator.ProvisioningConfig{}, nil, nil, nil, review, authRateLimiters{}, http.NotFoundHandler())

	anonymous := httptest.NewRecorder()
	router.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, "/unregistered-tag-scans/", nil))
	assert.Equal(t, http.StatusUnauthorized, anonymous.Code)
	assert.Empty(t, reached)

	for _, target := range []struct{ method, path, routePath string }{
		{method: http.MethodGet, path: "/unregistered-tag-scans/", routePath: "/"},
		{method: http.MethodPost, path: "/unregistered-tag-scans/123/resolve", routePath: "/123/resolve"},
	} {
		req := httptest.NewRequest(target.method, target.path, nil)
		req.Header.Set("Authorization", "Bearer "+operatorRouteAccessToken(t, 42))
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusTeapot, rr.Code, target.path)
		assert.Contains(t, reached, target.method+" "+target.routePath)
	}
}

func TestProtectedOperatorRoutesRejectInactiveOperator(t *testing.T) {
	t.Parallel()

	accessToken := operatorRouteAccessToken(t, 42)
	authService := &mockOperatorAuthService{
		getOperatorFn: func(_ context.Context, id int64) (*identityaccess.Operator, error) {
			assert.Equal(t, int64(42), id)
			op := &identityaccess.Operator{Active: false}
			op.ID = id
			return op, nil
		},
	}
	provisioningService := &protectedRouteProvisioningService{
		createSchoolFn: func(context.Context, *organizationtenancy.CreateSchool, int64, net.IP) (*organizationtenancy.School, error) {
			t.Fatal("inactive operator reached protected provisioning handler")
			return nil, nil
		},
	}
	router := testOperatorRouter(operatorIdentity{access: authService, sessions: operatorTestSessions(t, authService)}, provisioningService)

	req := newCreateSchoolRequest(accessToken, 7)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "Operator account is inactive")
}

func TestProtectedOperatorRoutesAllowActiveOperator(t *testing.T) {
	t.Parallel()

	accessToken := operatorRouteAccessToken(t, 42)
	organizationID := 7
	authService := &mockOperatorAuthService{
		getOperatorFn: func(_ context.Context, id int64) (*identityaccess.Operator, error) {
			assert.Equal(t, int64(42), id)
			op := &identityaccess.Operator{Active: true}
			op.ID = id
			return op, nil
		},
	}
	createSchoolCalled := false
	provisioningService := &protectedRouteProvisioningService{
		createSchoolFn: func(_ context.Context, school *organizationtenancy.CreateSchool, operatorID int64, clientIP net.IP) (*organizationtenancy.School, error) {
			createSchoolCalled = true
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, int64(organizationID), school.OrganizationID)
			assert.Equal(t, "school@example.com", school.Email)
			return &organizationtenancy.School{ID: 88, OrganizationID: school.OrganizationID, Name: school.Name, Slug: school.Slug, Subdomain: school.Subdomain, Email: school.Email}, nil
		},
	}
	router := testOperatorRouter(operatorIdentity{access: authService, sessions: operatorTestSessions(t, authService)}, provisioningService)

	req := newCreateSchoolRequest(accessToken, organizationID)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	assert.True(t, createSchoolCalled, "active operator should reach protected provisioning handler")
}

// testOperatorRouter mounts the operator routes over the given Identity &
// Access inputs and provisioning service, without billing, school settings,
// announcements, the tag scan review or rate limits.
func testOperatorRouter(identity operatorIdentity, provisioning organizationtenancy.Provisioning) chi.Router {
	return operatorRouter(identity, provisioningoperator.ProvisioningConfig{Service: provisioning}, nil, nil, nil, nil, authRateLimiters{}, http.NotFoundHandler())
}

// operatorTestSessions binds the operator session chains to the seeded test
// signer and the given active-operator lookup.
func operatorTestSessions(t *testing.T, operators identityoperator.OperatorLookup) identityoperator.Sessions {
	t.Helper()
	return identityoperator.NewSessions(routetest.TestTokenAuth(t), operators)
}

func operatorRouteAccessToken(t *testing.T, operatorID int) string {
	t.Helper()
	token, err := routetest.TestTokenAuth(t).CreateJWT(routetest.Claims{
		ID:       operatorID,
		Sub:      "operator-route-test",
		Roles:    []string{"operator"},
		Scope:    "platform",
		FamilyID: "operator-route-test-family",
	})
	require.NoError(t, err)
	return token
}

func newCreateSchoolRequest(accessToken string, organizationID int) *http.Request {
	body := fmt.Sprintf(`{"organization_id":%d,"name":"Test School","slug":"test-school","subdomain":"test-sub","email":"school@example.com"}`, organizationID)
	req := httptest.NewRequest(http.MethodPost, "/schools", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.RemoteAddr = "198.51.100.20:4444"
	return req
}
