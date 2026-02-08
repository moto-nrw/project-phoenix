package platform_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/platform"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// TestNewResource verifies that the platform resource can be constructed successfully.
func TestNewResource(t *testing.T) {
	t.Run("creates resource with nil services", func(t *testing.T) {
		cfg := platform.ResourceConfig{
			AnnouncementsService: nil,
			TokenAuth:            nil,
		}

		resource := platform.NewResource(cfg)
		require.NotNil(t, resource)
	})

	t.Run("creates resource with provided token auth", func(t *testing.T) {
		tokenAuth, err := jwt.NewTokenAuth()
		require.NoError(t, err)

		cfg := platform.ResourceConfig{
			AnnouncementsService: nil,
			TokenAuth:            tokenAuth,
		}

		resource := platform.NewResource(cfg)
		require.NotNil(t, resource)
	})

	t.Run("creates token auth internally when not provided", func(t *testing.T) {
		cfg := platform.ResourceConfig{}
		resource := platform.NewResource(cfg)
		require.NotNil(t, resource)
	})
}

// TestRouter verifies that the platform router can be constructed.
func TestRouter(t *testing.T) {
	t.Run("creates router successfully", func(t *testing.T) {
		cfg := platform.ResourceConfig{}
		resource := platform.NewResource(cfg)

		router := resource.Router()
		require.NotNil(t, router)
	})

	t.Run("router has expected routes", func(t *testing.T) {
		cfg := platform.ResourceConfig{}
		resource := platform.NewResource(cfg)

		router := resource.Router()
		require.NotNil(t, router)

		routes := router.Routes()
		assert.NotEmpty(t, routes)
	})
}
