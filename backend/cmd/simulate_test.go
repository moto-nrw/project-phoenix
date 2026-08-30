package cmd

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Command Registration Tests
// =============================================================================

func TestSimulateCmd_Metadata(t *testing.T) {
	assert.Equal(t, "simulate", simulateCmd.Use)
	assert.Contains(t, simulateCmd.Short, "simulation commands")
	assert.Contains(t, simulateCmd.Long, ".seed-state.json")
	assert.Nil(t, simulateCmd.Run, "simulate is a group command without its own Run")
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
// Subcommand Registration Tests
// =============================================================================

func TestSimulateCmd_HasSubcommands(t *testing.T) {
	subcommands := simulateCmd.Commands()
	names := make([]string, 0, len(subcommands))
	for _, cmd := range subcommands {
		names = append(names, cmd.Use)
	}
	assert.Contains(t, names, "full-day")
	assert.Contains(t, names, "status")
	assert.Contains(t, names, "live")
}

// =============================================================================
// FullDay Subcommand Tests
// =============================================================================

func TestSimulateFullDayCmd_Metadata(t *testing.T) {
	assert.Equal(t, "full-day", simulateFullDayCmd.Use)
	assert.Contains(t, simulateFullDayCmd.Short, "full-day simulation")
	assert.NotNil(t, simulateFullDayCmd.RunE)
}

func TestSimulateFullDayCmd_Flags(t *testing.T) {
	f := simulateFullDayCmd.Flags()
	assert.NotNil(t, f.Lookup("state"))
	assert.NotNil(t, f.Lookup("close"))
	assert.NotNil(t, f.Lookup("verbose"))
}

func TestSimulateFullDayCmd_FlagDefaults(t *testing.T) {
	f := simulateFullDayCmd.Flags()

	stateFlag := f.Lookup("state")
	require.NotNil(t, stateFlag)
	assert.Equal(t, ".seed-state.json", stateFlag.DefValue)

	closeFlag := f.Lookup("close")
	require.NotNil(t, closeFlag)
	assert.Equal(t, "false", closeFlag.DefValue)

	verboseFlag := f.Lookup("verbose")
	require.NotNil(t, verboseFlag)
	assert.Equal(t, "false", verboseFlag.DefValue)
}

// =============================================================================
// Status Subcommand Tests
// =============================================================================

func TestSimulateStatusCmd_Metadata(t *testing.T) {
	assert.Equal(t, "status", simulateStatusCmd.Use)
	assert.Contains(t, simulateStatusCmd.Short, "simulation state")
	assert.NotNil(t, simulateStatusCmd.RunE)
}

func TestSimulateStatusCmd_Flags(t *testing.T) {
	f := simulateStatusCmd.Flags()
	assert.NotNil(t, f.Lookup("state"))
	assert.NotNil(t, f.Lookup("verbose"))
}

func TestSimulateStatusCmd_FlagDefaults(t *testing.T) {
	f := simulateStatusCmd.Flags()

	stateFlag := f.Lookup("state")
	require.NotNil(t, stateFlag)
	assert.Equal(t, ".seed-state.json", stateFlag.DefValue)
}

// =============================================================================
// Live Subcommand Tests
// =============================================================================

func TestSimulateLiveCmd_Metadata(t *testing.T) {
	assert.Equal(t, "live", simulateLiveCmd.Use)
	assert.Contains(t, simulateLiveCmd.Short, "continuous live simulation")
	assert.NotNil(t, simulateLiveCmd.RunE)
}

func TestSimulateRootFailsFastWithoutClient(t *testing.T) {
	t.Parallel()

	require.EqualError(t, (simulateRoot{}).validate(), "simulation client factory is required")
}

func TestSimulateRootFailsFastForMissingRunners(t *testing.T) {
	t.Parallel()

	client := func(string, bool) (simulationClient, error) { return nil, nil }
	require.EqualError(t, (simulateRoot{client: client}).runFullDay(context.Background(), simulationFullDayOptions{}), "full-day simulation runner is required")
	require.EqualError(t, (simulateRoot{client: client}).runStatus(context.Background(), simulationStatusOptions{}), "status simulation runner is required")
	require.EqualError(t, (simulateRoot{client: client}).runLive(context.Background(), simulationLiveOptions{}), "live simulation runner is required")
}

func TestSimulateRootPreservesRunnerErrorContracts(t *testing.T) {
	t.Parallel()

	client := func(string, bool) (simulationClient, error) { return nil, nil }
	failure := errors.New("runner failed")

	err := (simulateRoot{client: client, fullDay: func(context.Context, simulationFullDayOptions) error { return failure }}).
		runFullDay(context.Background(), simulationFullDayOptions{})
	require.ErrorIs(t, err, failure)
	require.EqualError(t, err, "Full-day simulation failed: runner failed")

	err = (simulateRoot{client: client, status: func(context.Context, simulationStatusOptions) error { return failure }}).
		runStatus(context.Background(), simulationStatusOptions{})
	require.ErrorIs(t, err, failure)
	require.EqualError(t, err, "Status query failed: runner failed")

	err = (simulateRoot{client: client, live: func(context.Context, simulationLiveOptions) error { return failure }}).
		runLive(context.Background(), simulationLiveOptions{})
	require.ErrorIs(t, err, failure)
	require.EqualError(t, err, "Live simulation failed: runner failed")
}

func TestSimulateLiveCmd_Flags(t *testing.T) {
	f := simulateLiveCmd.Flags()
	assert.NotNil(t, f.Lookup("state"))
	assert.NotNil(t, f.Lookup("interval"))
	assert.NotNil(t, f.Lookup("verbose"))
}

func TestSimulateLiveCmd_FlagDefaults(t *testing.T) {
	f := simulateLiveCmd.Flags()

	stateFlag := f.Lookup("state")
	require.NotNil(t, stateFlag)
	assert.Equal(t, ".seed-state.json", stateFlag.DefValue)

	intervalFlag := f.Lookup("interval")
	require.NotNil(t, intervalFlag)
	assert.Equal(t, "10s", intervalFlag.DefValue)
}

// =============================================================================
// Usage Output Tests
// =============================================================================

func TestSimulateFullDayCmd_UsageOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	simulateFullDayCmd.SetOut(buf)
	simulateFullDayCmd.SetErr(buf)

	err := simulateFullDayCmd.Usage()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "full-day")
	assert.Contains(t, buf.String(), "--state")
}

func TestSimulateStatusCmd_UsageOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	simulateStatusCmd.SetOut(buf)
	simulateStatusCmd.SetErr(buf)

	err := simulateStatusCmd.Usage()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "status")
}

func TestSimulateLiveCmd_UsageOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	simulateLiveCmd.SetOut(buf)
	simulateLiveCmd.SetErr(buf)

	err := simulateLiveCmd.Usage()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "live")
	assert.Contains(t, buf.String(), "--interval")
}
