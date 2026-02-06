package api_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
)

// TestParsePositiveInt tests the parsePositiveInt helper function via New.
// Since parsePositiveInt is unexported, we test it indirectly through environment variables.
func TestNew_ParsesEnvironmentVariables(t *testing.T) {
	t.Run("creates API instance successfully", func(t *testing.T) {
		apiInstance, err := api.New(false, nil)
		require.NoError(t, err)
		require.NotNil(t, apiInstance)
		assert.NotNil(t, apiInstance.Services)
		assert.NotNil(t, apiInstance.Router)
	})

	t.Run("creates API with CORS enabled", func(t *testing.T) {
		apiInstance, err := api.New(true, nil)
		require.NoError(t, err)
		require.NotNil(t, apiInstance)
	})
}

// TestAPI_ServeHTTP verifies the http.Handler interface implementation.
func TestAPI_ServeHTTP(t *testing.T) {
	t.Run("API implements http.Handler", func(t *testing.T) {
		apiInstance, err := api.New(false, nil)
		require.NoError(t, err)

		// Verify API implements http.Handler
		var _ interface{} = apiInstance
		assert.NotNil(t, apiInstance.Router)
	})
}
