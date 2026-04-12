package operator_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/operator"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
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
type stubSettingsService struct{}

func (stubSettingsService) GetSchema(context.Context, []string) (*configSvc.SettingsSchema, error) {
	return nil, nil
}
func (stubSettingsService) Resolve(context.Context, string) (any, error) { return nil, nil }
func (stubSettingsService) ResolveString(context.Context, string) (string, error) {
	return "", nil
}
func (stubSettingsService) ResolveStringForTenant(context.Context, int64, string) (string, error) {
	return "", nil
}
func (stubSettingsService) ResolveBool(context.Context, string) (bool, error) { return false, nil }
func (stubSettingsService) ResolveInt(context.Context, string) (int, error)   { return 0, nil }
func (stubSettingsService) HasTenantOverride(context.Context, string) (bool, error) {
	return false, nil
}
func (stubSettingsService) SetValue(context.Context, string, any, *int64, []string) error {
	return nil
}
func (stubSettingsService) ResetValue(context.Context, string, *int64, []string) error {
	return nil
}
func (stubSettingsService) GetLoginImageURL(context.Context, int64) (string, error) {
	return "", nil
}
func (stubSettingsService) SetLoginImageURL(context.Context, int64, string) (string, error) {
	return "", nil
}
func (stubSettingsService) ClearLoginImageURL(context.Context, int64) (string, error) {
	return "", nil
}

// TestNewResource verifies that the operator resource can be constructed successfully.
func TestNewResource(t *testing.T) {
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
			SettingsService: stubSettingsService{},
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
