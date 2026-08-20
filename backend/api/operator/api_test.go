package operator_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/operator"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// chiWalk traverses all routes in the chi.Router and calls fn for each
// concrete pattern. Used to assert mount points in the operator router.
func chiWalk(router chi.Router, fn func(pattern string)) error {
	return chi.Walk(router, func(_ string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		fn(route)
		return nil
	})
}

// stubSettingsService is a no-op SettingsService used only to satisfy the
// non-nil check in NewResource so that the settings routes get wired.
// Handler behavior is exercised in settings_integration_test.go with a real
// service.
func stubSettingsService() *configtest.Mock { return &configtest.Mock{} }

type protectedRouteProvisioningService struct {
	platformSvc.OperatorProvisioningService
	createSchoolFn func(context.Context, *platformModels.School, int64, net.IP) (*platformModels.School, error)
}

func (s *protectedRouteProvisioningService) CreateSchool(ctx context.Context, school *platformModels.School, operatorID int64, clientIP net.IP) (*platformModels.School, error) {
	if s.createSchoolFn != nil {
		return s.createSchoolFn(ctx, school, operatorID, clientIP)
	}
	return school, nil
}

// TestNewResource verifies that the operator resource can be constructed successfully.
func TestNewResource(t *testing.T) {
	t.Parallel()

	t.Run("creates resource with nil services", func(t *testing.T) {
		cfg := operator.ResourceConfig{
			AuthService:          nil,
			SuggestionsService:   nil,
			AnnouncementsService: nil,
			TokenAuth:            nil,
		}

		resource := operator.NewResource(cfg)
		require.NotNil(t, resource)
	})

	t.Run("creates resource with provided token auth", func(t *testing.T) {
		tokenAuth, err := jwt.NewTokenAuth()
		require.NoError(t, err)

		cfg := operator.ResourceConfig{
			AuthService:          nil,
			SuggestionsService:   nil,
			AnnouncementsService: nil,
			TokenAuth:            tokenAuth,
		}

		resource := operator.NewResource(cfg)
		require.NotNil(t, resource)
	})

	t.Run("creates token auth internally when not provided", func(t *testing.T) {
		cfg := operator.ResourceConfig{}
		resource := operator.NewResource(cfg)
		require.NotNil(t, resource)
	})
}

// TestRouter verifies that the operator router can be constructed.
func TestRouter(t *testing.T) {
	t.Parallel()

	t.Run("creates router successfully", func(t *testing.T) {
		cfg := operator.ResourceConfig{}
		resource := operator.NewResource(cfg)

		router := resource.Router()
		require.NotNil(t, router)
	})

	t.Run("router has expected routes", func(t *testing.T) {
		cfg := operator.ResourceConfig{}
		resource := operator.NewResource(cfg)

		router := resource.Router()
		require.NotNil(t, router)

		routes := router.Routes()
		assert.NotEmpty(t, routes)
	})

	t.Run("settings routes are mounted when SettingsService is provided", func(t *testing.T) {
		cfg := operator.ResourceConfig{
			SettingsService: stubSettingsService(),
		}
		resource := operator.NewResource(cfg)
		require.NotNil(t, resource)

		router := resource.Router()
		require.NotNil(t, router)

		// Walk the route tree and assert the settings subroute exists under /schools.
		found := false
		err := chiWalk(router, func(pattern string) {
			if pattern == "/schools/{id}/settings/schema" ||
				pattern == "/schools/{id}/settings/values/{key}" ||
				pattern == "/schools/{id}/settings/values/{key}/reveal" {
				found = true
			}
		})
		require.NoError(t, err)
		assert.True(t, found, "expected /schools/{id}/settings/* routes to be mounted when SettingsService is provided")
	})

	t.Run("settings routes are NOT mounted when SettingsService is nil", func(t *testing.T) {
		cfg := operator.ResourceConfig{}
		resource := operator.NewResource(cfg)
		require.NotNil(t, resource)

		router := resource.Router()

		found := false
		err := chiWalk(router, func(pattern string) {
			if pattern == "/schools/{id}/settings/schema" {
				found = true
			}
		})
		require.NoError(t, err)
		assert.False(t, found, "expected settings routes NOT to be mounted when SettingsService is nil")
	})
}

func TestProtectedOperatorRoutesRejectInactiveOperator(t *testing.T) {
	t.Parallel()

	tokenAuth := newOperatorRouteTokenAuth(t)
	accessToken := operatorRouteAccessToken(t, tokenAuth, 42)
	authService := &mockOperatorAuthService{
		getOperatorFn: func(_ context.Context, id int64) (*platformModels.Operator, error) {
			assert.Equal(t, int64(42), id)
			op := &platformModels.Operator{Active: false}
			op.ID = id
			return op, nil
		},
	}
	provisioningService := &protectedRouteProvisioningService{
		createSchoolFn: func(context.Context, *platformModels.School, int64, net.IP) (*platformModels.School, error) {
			t.Fatal("inactive operator reached protected provisioning handler")
			return nil, nil
		},
	}
	router := operator.NewResource(operator.ResourceConfig{
		AuthService:         authService,
		ProvisioningService: provisioningService,
		TokenAuth:           tokenAuth,
	}).Router()

	req := newCreateSchoolRequest(accessToken, 7)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "Operator account is inactive")
}

func TestProtectedOperatorRoutesAllowActiveOperator(t *testing.T) {
	t.Parallel()

	tokenAuth := newOperatorRouteTokenAuth(t)
	accessToken := operatorRouteAccessToken(t, tokenAuth, 42)
	organizationID := 7
	authService := &mockOperatorAuthService{
		getOperatorFn: func(_ context.Context, id int64) (*platformModels.Operator, error) {
			assert.Equal(t, int64(42), id)
			op := &platformModels.Operator{Active: true}
			op.ID = id
			return op, nil
		},
	}
	createSchoolCalled := false
	provisioningService := &protectedRouteProvisioningService{
		createSchoolFn: func(_ context.Context, school *platformModels.School, operatorID int64, clientIP net.IP) (*platformModels.School, error) {
			createSchoolCalled = true
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, int64(organizationID), school.OrganizationID)
			assert.Equal(t, "school@example.com", school.Email)
			school.ID = 88
			return school, nil
		},
	}
	router := operator.NewResource(operator.ResourceConfig{
		AuthService:         authService,
		ProvisioningService: provisioningService,
		TokenAuth:           tokenAuth,
	}).Router()

	req := newCreateSchoolRequest(accessToken, organizationID)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	assert.True(t, createSchoolCalled, "active operator should reach protected provisioning handler")
}

func newOperatorRouteTokenAuth(t *testing.T) *jwt.TokenAuth {
	t.Helper()
	tokenAuth, err := jwt.NewTokenAuthWithSecret("operator-route-test-secret-123456")
	require.NoError(t, err)
	tokenAuth.JwtExpiry = 15 * time.Minute
	return tokenAuth
}

func operatorRouteAccessToken(t *testing.T, tokenAuth *jwt.TokenAuth, operatorID int) string {
	t.Helper()
	token, err := tokenAuth.CreateJWT(jwt.AppClaims{
		ID:    operatorID,
		Sub:   "operator-route-test",
		Roles: []string{"operator"},
		Scope: "platform",
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
