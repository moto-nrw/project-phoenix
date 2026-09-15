package cmd

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Command Registration Tests
// =============================================================================

func TestSeedCmd_Metadata(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "seed", seedCmd.Use)
	assert.Contains(t, seedCmd.Short, "Seed the database")
	assert.NotNil(t, seedCmd.RunE)
}

func TestSeedRootFailsFastWithoutDependencies(t *testing.T) {
	t.Parallel()

	root := defaultSeedRoot
	root.newAdapter = nil
	err := root.validate()
	require.EqualError(t, err, "seed API adapter factory is required")

	root = defaultSeedRoot
	root.random = nil
	err = root.validate()
	require.EqualError(t, err, "seed random source is required")

	root = defaultSeedRoot
	root.seed = nil
	err = root.validate()
	require.EqualError(t, err, "seed runner is required")
}

func TestSeedRootRejectsNilBuiltAdapter(t *testing.T) {
	t.Parallel()

	root := defaultSeedRoot
	root.newAdapter = func(string, bool) seedAdapter { return nil }
	err := root.run(context.Background(), "http://localhost", false, seedOptions{}, "email", "password", "pin")
	require.EqualError(t, err, "seed API adapter factory returned nil")
}

func TestSeedRootInvokesOnlyItsSelectedDependencies(t *testing.T) {
	t.Parallel()

	adapterCalls := 0
	runnerCalls := 0
	random := strings.NewReader("random")
	root := seedRoot{
		newAdapter: func(baseURL string, verbose bool) seedAdapter {
			adapterCalls++
			return defaultSeedRoot.newAdapter(baseURL, verbose)
		},
		random: random,
		seed: func(_ context.Context, adapter seedAdapter, source io.Reader, _ bool, _ seedOptions, _, _, _ string) error {
			runnerCalls++
			assert.NotNil(t, adapter)
			assert.Same(t, random, source)
			return nil
		},
	}

	require.NoError(t, root.run(context.Background(), "http://localhost", false, seedOptions{}, "email", "password", "pin"))
	assert.Equal(t, 1, adapterCalls)
	assert.Equal(t, 1, runnerCalls)
}

func TestSeedRootPreservesRunnerError(t *testing.T) {
	t.Parallel()

	failure := errors.New("runner failed")
	root := defaultSeedRoot
	root.seed = func(context.Context, seedAdapter, io.Reader, bool, seedOptions, string, string, string) error {
		return failure
	}

	err := root.run(context.Background(), "http://localhost", false, seedOptions{}, "email", "password", "pin")
	require.ErrorIs(t, err, failure)
	require.EqualError(t, err, "runner failed")
}

func TestSeedCmd_Flags(t *testing.T) {
	t.Parallel()
	f := seedCmd.Flags()

	assert.NotNil(t, f.Lookup("email"))
	assert.NotNil(t, f.Lookup("password"))
	assert.NotNil(t, f.Lookup("pin"))
	assert.NotNil(t, f.Lookup("url"))
	assert.NotNil(t, f.Lookup("verbose"))
	assert.NotNil(t, f.Lookup("tenant-slug"))
	assert.NotNil(t, f.Lookup("staff-password"))
	assert.NotNil(t, f.Lookup("admin-email"))
	assert.NotNil(t, f.Lookup("randomize"))
}

func TestSeedCmd_FlagDefaults(t *testing.T) {
	t.Parallel()
	f := seedCmd.Flags()

	urlFlag := f.Lookup("url")
	require.NotNil(t, urlFlag)
	assert.Equal(t, "http://localhost:8080", urlFlag.DefValue)
}

func TestSeedCmd_LongDescription(t *testing.T) {
	t.Parallel()
	assert.Contains(t, seedCmd.Long, "HTTP API")
	assert.Contains(t, seedCmd.Long, "REQUIRES")
	assert.Contains(t, seedCmd.Long, ".seed-state.json")
}

func TestSeedCmd_FlagTypes(t *testing.T) {
	t.Parallel()
	f := seedCmd.Flags()

	emailFlag := f.Lookup("email")
	require.NotNil(t, emailFlag)
	assert.Equal(t, "", emailFlag.DefValue)

	passwordFlag := f.Lookup("password")
	require.NotNil(t, passwordFlag)
	assert.Equal(t, "", passwordFlag.DefValue)

	pinFlag := f.Lookup("pin")
	require.NotNil(t, pinFlag)
	assert.Equal(t, "", pinFlag.DefValue)

	verboseFlag := f.Lookup("verbose")
	require.NotNil(t, verboseFlag)
	assert.Equal(t, "false", verboseFlag.DefValue)

	tenantSlugFlag := f.Lookup("tenant-slug")
	require.NotNil(t, tenantSlugFlag)
	assert.Equal(t, "", tenantSlugFlag.DefValue)

	staffPasswordFlag := f.Lookup("staff-password")
	require.NotNil(t, staffPasswordFlag)
	assert.Equal(t, "", staffPasswordFlag.DefValue)

	adminEmailFlag := f.Lookup("admin-email")
	require.NotNil(t, adminEmailFlag)
	assert.Equal(t, "", adminEmailFlag.DefValue)

	randomizeFlag := f.Lookup("randomize")
	require.NotNil(t, randomizeFlag)
	assert.Equal(t, "false", randomizeFlag.DefValue)
}
