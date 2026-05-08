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
			break
		}
	}
	assert.True(t, found, "e2eCmd should be registered on RootCmd")
}

func TestE2EPrepareCmd_Metadata(t *testing.T) {
	assert.Equal(t, "prepare", e2ePrepareCmd.Use)
	assert.Contains(t, e2ePrepareCmd.Short, "canonical Playwright world")
	assert.Contains(t, e2ePrepareCmd.Long, ".e2e-state.json")
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
}

func TestE2EUpCmd_Metadata(t *testing.T) {
	assert.Equal(t, "up", e2eUpCmd.Use)
	assert.Contains(t, e2eUpCmd.Short, "manual testing")
	assert.NotNil(t, e2eUpCmd.Run)
}
