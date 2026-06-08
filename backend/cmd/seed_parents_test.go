package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssertLocalDevEnv(t *testing.T) {
	allowed := []string{"", "local", "development", "dev", "test", "DEV", "Test", "  development  "}
	for _, env := range allowed {
		t.Setenv("APP_ENV", env)
		require.NoErrorf(t, assertLocalDevEnv(), "APP_ENV=%q must be allowed", env)
	}

	rejected := []string{"production", "staging", "prod", "preview", "anything-else"}
	for _, env := range rejected {
		t.Setenv("APP_ENV", env)
		err := assertLocalDevEnv()
		require.Errorf(t, err, "APP_ENV=%q must be rejected", env)
		assert.Contains(t, err.Error(), "dev-only")
	}
}
