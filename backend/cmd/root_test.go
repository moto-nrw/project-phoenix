package cmd

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// RootCmd Tests
// =============================================================================

func TestRootCmd_Metadata(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "phoenix", RootCmd.Use)
	assert.Contains(t, RootCmd.Short, "RFID-based")
	assert.Contains(t, RootCmd.Long, "Project Phoenix")
}

func TestRootCmd_HasCommands(t *testing.T) {
	t.Parallel()
	commands := RootCmd.Commands()
	assert.NotEmpty(t, commands, "RootCmd should have subcommands")

	names := make([]string, 0, len(commands))
	for _, cmd := range commands {
		names = append(names, cmd.Use)
	}

	assert.Contains(t, names, "serve")
	assert.Contains(t, names, "migrate")
	assert.Contains(t, names, "cleanup")
	assert.Contains(t, names, "seed")
	assert.Contains(t, names, "gendoc")
	assert.Contains(t, names, "simulate")
	t.Run("composition-inventory", assertCommandComposition)
}

func TestRootCmd_PersistentFlags(t *testing.T) {
	t.Parallel()
	f := RootCmd.PersistentFlags()
	assert.NotNil(t, f.Lookup("config"))
	assert.NotNil(t, f.Lookup("db_debug"))
}

// =============================================================================
// initConfig Tests
// =============================================================================

func TestInitConfig_DefaultConfig(t *testing.T) {
	t.Parallel()
	config := viper.New()
	loadConfig(config, "", true)
	assert.Empty(t, config.ConfigFileUsed())
}

func TestInitConfig_WithConfigFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := tmpDir + "/test.env"
	err := os.WriteFile(configPath, []byte("TEST_KEY=test_value"), 0644)
	require.NoError(t, err)

	config := viper.New()
	loadConfig(config, configPath, false)

	assert.Equal(t, configPath, config.ConfigFileUsed())
	assert.Equal(t, "test_value", config.GetString("TEST_KEY"))
}

func TestInitConfig_WithNonExistentConfigFile(t *testing.T) {
	t.Parallel()
	config := viper.New()
	const path = "/nonexistent/path/config.env"
	loadConfig(config, path, false)
	assert.Equal(t, path, config.ConfigFileUsed())
}
