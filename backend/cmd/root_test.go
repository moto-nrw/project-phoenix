package cmd

import (
	"bytes"
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
	assert.Equal(t, "phoenix", RootCmd.Use)
	assert.Contains(t, RootCmd.Short, "RFID-based")
	assert.Contains(t, RootCmd.Long, "Project Phoenix")
}

func TestRootCmd_HasCommands(t *testing.T) {
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
}

func TestRootCmd_PersistentFlags(t *testing.T) {
	f := RootCmd.PersistentFlags()
	assert.NotNil(t, f.Lookup("config"))
	assert.NotNil(t, f.Lookup("db_debug"))
}

func TestRootCmd_UsageOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	RootCmd.SetOut(buf)
	RootCmd.SetErr(buf)

	err := RootCmd.Usage()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "phoenix")
	assert.Contains(t, output, "Available Commands")
}

// =============================================================================
// initConfig Tests
// =============================================================================

func TestInitConfig_DefaultConfig(t *testing.T) {
	oldCfgFile := cfgFile
	cfgFile = ""
	defer func() { cfgFile = oldCfgFile }()

	// initConfig should not panic even without config files
	initConfig()

	assert.NotNil(t, viper.GetViper())
}

func TestInitConfig_WithConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/test.env"
	err := os.WriteFile(configPath, []byte("TEST_KEY=test_value"), 0644)
	require.NoError(t, err)

	oldCfgFile := cfgFile
	cfgFile = configPath
	defer func() { cfgFile = oldCfgFile }()

	initConfig()

	assert.Equal(t, configPath, viper.ConfigFileUsed())
}

func TestInitConfig_WithNonExistentConfigFile(t *testing.T) {
	oldCfgFile := cfgFile
	cfgFile = "/nonexistent/path/config.env"
	defer func() { cfgFile = oldCfgFile }()

	// Should not panic
	initConfig()
}
