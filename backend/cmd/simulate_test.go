package cmd

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Command Registration Tests
// =============================================================================

func TestSimulateCmd_Metadata(t *testing.T) {
	assert.Equal(t, "simulate", simulateCmd.Use)
	assert.Contains(t, simulateCmd.Short, "IoT simulator")
	assert.Contains(t, simulateCmd.Long, "discovery loop")
	assert.NotNil(t, simulateCmd.Run)
}

func TestSimulateCmd_IsRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range RootCmd.Commands() {
		if cmd.Use == "simulate" {
			found = true
			break
		}
	}
	assert.True(t, found, "simulateCmd should be registered on RootCmd")
}

func TestSimulateCmd_UsageOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	simulateCmd.SetOut(buf)
	simulateCmd.SetErr(buf)

	err := simulateCmd.Usage()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "simulate")
}

// =============================================================================
// Constants Tests
// =============================================================================

func TestSimulateConstants(t *testing.T) {
	assert.Equal(t, "simulator/iot/simulator.yaml", defaultSimulatorConfig)
	assert.Equal(t, "SIMULATOR_CONFIG", envSimulatorConfig)
}

// =============================================================================
// resolveSimulatorConfigPath Tests
// =============================================================================

func TestResolveSimulatorConfigPath_EnvVarTakesPriority(t *testing.T) {
	t.Setenv(envSimulatorConfig, "/custom/path/config.yaml")

	result := resolveSimulatorConfigPath()
	assert.Equal(t, "/custom/path/config.yaml", result)
}

func TestResolveSimulatorConfigPath_UsesDefaultWhenNoEnv(t *testing.T) {
	t.Setenv(envSimulatorConfig, "")

	result := resolveSimulatorConfigPath()
	assert.Equal(t, filepath.Clean(defaultSimulatorConfig), result)
}

func TestResolveSimulatorConfigPath_DefaultPathIsCleaned(t *testing.T) {
	t.Setenv(envSimulatorConfig, "")

	result := resolveSimulatorConfigPath()
	// filepath.Clean should normalize the path
	assert.Equal(t, result, filepath.Clean(result))
}
