package operator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/operator"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

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
}
