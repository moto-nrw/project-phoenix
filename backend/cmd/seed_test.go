package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Command Registration Tests
// =============================================================================

func TestSeedCmd_Metadata(t *testing.T) {
	assert.Equal(t, "seed", seedCmd.Use)
	assert.Contains(t, seedCmd.Short, "Seed the database")
	assert.Contains(t, seedCmd.Long, "API MODE")
	assert.NotNil(t, seedCmd.Run)
}

func TestSeedCmd_IsRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range RootCmd.Commands() {
		if cmd.Use == "seed" {
			found = true
			break
		}
	}
	assert.True(t, found, "seedCmd should be registered on RootCmd")
}

func TestSeedCmd_Flags(t *testing.T) {
	f := seedCmd.Flags()

	assert.NotNil(t, f.Lookup("reset"))
	assert.NotNil(t, f.Lookup("fixed-only"))
	assert.NotNil(t, f.Lookup("runtime-only"))
	assert.NotNil(t, f.Lookup("verbose"))
	assert.NotNil(t, f.Lookup("api"))
	assert.NotNil(t, f.Lookup("email"))
	assert.NotNil(t, f.Lookup("password"))
	assert.NotNil(t, f.Lookup("pin"))
	assert.NotNil(t, f.Lookup("url"))
}

func TestSeedCmd_FlagDefaults(t *testing.T) {
	f := seedCmd.Flags()

	// url flag should default to localhost:8080
	urlFlag := f.Lookup("url")
	require.NotNil(t, urlFlag)
	assert.Equal(t, "http://localhost:8080", urlFlag.DefValue)

	// Boolean flags should default to false
	resetFlag := f.Lookup("reset")
	require.NotNil(t, resetFlag)
	assert.Equal(t, "false", resetFlag.DefValue)
}

func TestSeedCmd_UsageOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	seedCmd.SetOut(buf)
	seedCmd.SetErr(buf)

	err := seedCmd.Usage()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "seed")
	assert.Contains(t, output, "--reset")
	assert.Contains(t, output, "--api")
	assert.Contains(t, output, "--email")
	assert.Contains(t, output, "--password")
	assert.Contains(t, output, "--pin")
	assert.Contains(t, output, "--url")
}
