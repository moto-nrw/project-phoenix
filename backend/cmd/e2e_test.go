package cmd

import (
	"testing"

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
	assert.Contains(t, e2ePrepareCmd.Long, ".e2e-manifest.json")
	assert.NotNil(t, e2ePrepareCmd.Run)
}

func TestE2EPrepareCmd_FlagDefaults(t *testing.T) {
	f := e2ePrepareCmd.Flags()

	urlFlag := f.Lookup("url")
	require.NotNil(t, urlFlag)
	assert.Equal(t, "http://localhost:8080", urlFlag.DefValue)

	assert.Nil(t, f.Lookup("operator-email"))
	assert.Nil(t, f.Lookup("operator-password"))
	assert.Nil(t, f.Lookup("operator-display-name"))
	assert.Nil(t, f.Lookup("staff-pin"))
}
