package cmd

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/e2e/scenarios"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2ECmd_IsRegisteredOnRoot(t *testing.T) {
	var found bool
	for _, cmd := range RootCmd.Commands() {
		if cmd.Use == "e2e" {
			found = true
			assert.True(t, cmd.Hidden, "backend e2e command is an implementation detail behind frontend pnpm e2e")
			break
		}
	}
	assert.True(t, found, "e2eCmd should be registered on RootCmd")
}

func TestE2EPrepareCmd_Metadata(t *testing.T) {
	assert.Equal(t, "prepare", e2ePrepareCmd.Use)
	assert.Contains(t, e2ePrepareCmd.Short, "internal building block")
	assert.Contains(t, e2ePrepareCmd.Long, ".e2e-state.json")
	assert.Contains(t, e2ePrepareCmd.Long, "pnpm e2e")
	assert.True(t, e2ePrepareCmd.Hidden)
	assert.NotNil(t, e2ePrepareCmd.Run)
}

func TestE2EPrepareCmd_FlagDefaults(t *testing.T) {
	f := e2ePrepareCmd.Flags()

	scenarioFlag := f.Lookup("scenario")
	require.NotNil(t, scenarioFlag)
	assert.Equal(t, scenarios.DefaultPrepareScenario().Name, scenarioFlag.DefValue)

	assert.Nil(t, f.Lookup("url"))
	assert.Nil(t, f.Lookup("operator-email"))
	assert.Nil(t, f.Lookup("operator-password"))
	assert.Nil(t, f.Lookup("operator-display-name"))
	assert.Nil(t, f.Lookup("staff-pin"))
}

func TestE2EHostsCmd_Metadata(t *testing.T) {
	assert.Equal(t, "hosts", e2eHostsCmd.Use)
	assert.Contains(t, e2eHostsCmd.Short, "hostnames")
	assert.NotNil(t, e2eHostsCmd.Run)
}

func TestE2EHostsCmd_FlagDefaults(t *testing.T) {
	f := e2eHostsCmd.Flags()

	scenarioFlag := f.Lookup("scenario")
	require.NotNil(t, scenarioFlag)
	assert.Equal(t, scenarios.DefaultPrepareScenario().Name, scenarioFlag.DefValue)
}

func TestE2ERunCmd_Metadata(t *testing.T) {
	assert.Equal(t, "run", e2eRunCmd.Use)
	assert.Contains(t, e2eRunCmd.Short, "end-to-end")
	assert.NotNil(t, e2eRunCmd.Run)
	require.NotNil(t, e2eRunCmd.Args)
	assert.NoError(t, e2eRunCmd.Args(e2eRunCmd, nil))
	assert.Error(t, e2eRunCmd.Args(e2eRunCmd, []string{"--project=chromium-admin"}))
}

func TestE2EUpCmd_IsNotRegistered(t *testing.T) {
	for _, cmd := range e2eCmd.Commands() {
		assert.NotEqual(t, "up", cmd.Use)
	}
}
