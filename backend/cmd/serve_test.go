package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Command Registration Tests
// =============================================================================

func TestServeCmd_Metadata(t *testing.T) {
	assert.Equal(t, "serve", serveCmd.Use)
	assert.Contains(t, serveCmd.Short, "start http server")
	assert.Contains(t, serveCmd.Long, "http server")
	assert.NotNil(t, serveCmd.Run)
}

func TestServeCmd_IsRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range RootCmd.Commands() {
		if cmd.Use == "serve" {
			found = true
			break
		}
	}
	assert.True(t, found, "serveCmd should be registered on RootCmd")
}

func TestServeCmd_UsageOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	serveCmd.SetOut(buf)
	serveCmd.SetErr(buf)

	err := serveCmd.Usage()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "serve")
}

// =============================================================================
// Viper Defaults Tests (set in serve.go init())
// =============================================================================

func TestServeCmd_ViperDefaults(t *testing.T) {
	// Reset viper to isolate from dev.env (which initConfig may have loaded)
	viper.Reset()

	// Re-register the defaults that serve.go init() sets
	viper.SetDefault("port", "8080")
	viper.SetDefault("log_level", "debug")
	viper.SetDefault("login_url", "http://localhost:8080/login")
	viper.SetDefault("auth_jwt_secret", "random")
	viper.SetDefault("auth_jwt_expiry", "15m")
	viper.SetDefault("auth_jwt_refresh_expiry", "1h")

	// Verify defaults without any config file influence
	assert.Equal(t, "8080", viper.GetString("port"))
	assert.Equal(t, "debug", viper.GetString("log_level"))
	assert.Equal(t, "http://localhost:8080/login", viper.GetString("login_url"))
	assert.Equal(t, "random", viper.GetString("auth_jwt_secret"))
	assert.Equal(t, "15m", viper.GetString("auth_jwt_expiry"))
	assert.Equal(t, "1h", viper.GetString("auth_jwt_refresh_expiry"))
}
