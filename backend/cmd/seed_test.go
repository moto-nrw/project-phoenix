package cmd

import (
	"bytes"
	"testing"

	contract "github.com/moto-nrw/project-phoenix/e2e/contract"
	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Command Registration Tests
// =============================================================================

func TestSeedCmd_Metadata(t *testing.T) {
	assert.Equal(t, "seed", seedCmd.Use)
	assert.Contains(t, seedCmd.Short, "Seed the database")
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

	assert.NotNil(t, f.Lookup("email"))
	assert.NotNil(t, f.Lookup("password"))
	assert.NotNil(t, f.Lookup("pin"))
	assert.NotNil(t, f.Lookup("url"))
	assert.NotNil(t, f.Lookup("verbose"))
	assert.NotNil(t, f.Lookup("scenario"))
	assert.NotNil(t, f.Lookup("tenant-slug"))
	assert.NotNil(t, f.Lookup("staff-password"))
	assert.NotNil(t, f.Lookup("admin-email"))
	assert.NotNil(t, f.Lookup("state-path"))
}

func TestSeedCmd_FlagDefaults(t *testing.T) {
	f := seedCmd.Flags()

	urlFlag := f.Lookup("url")
	require.NotNil(t, urlFlag)
	assert.Equal(t, "http://localhost:8080", urlFlag.DefValue)
}

func TestSeedCmd_UsageOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	seedCmd.SetOut(buf)
	seedCmd.SetErr(buf)

	err := seedCmd.Usage()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "seed")
	assert.Contains(t, output, "--email")
	assert.Contains(t, output, "--password")
	assert.Contains(t, output, "--pin")
	assert.Contains(t, output, "--url")
	assert.Contains(t, output, "--scenario")
}

func TestSeedCmd_LongDescription(t *testing.T) {
	assert.Contains(t, seedCmd.Long, "HTTP API")
	assert.Contains(t, seedCmd.Long, "REQUIRES")
	assert.Contains(t, seedCmd.Long, ".seed-state.json")
	assert.Contains(t, seedCmd.Long, contract.ManifestPath)
}

func TestSeedCmd_FlagTypes(t *testing.T) {
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

	scenarioFlag := f.Lookup("scenario")
	require.NotNil(t, scenarioFlag)
	assert.Equal(t, "", scenarioFlag.DefValue)

	tenantSlugFlag := f.Lookup("tenant-slug")
	require.NotNil(t, tenantSlugFlag)
	assert.Equal(t, "", tenantSlugFlag.DefValue)

	staffPasswordFlag := f.Lookup("staff-password")
	require.NotNil(t, staffPasswordFlag)
	assert.Equal(t, "", staffPasswordFlag.DefValue)

	adminEmailFlag := f.Lookup("admin-email")
	require.NotNil(t, adminEmailFlag)
	assert.Equal(t, "", adminEmailFlag.DefValue)

	statePathFlag := f.Lookup("state-path")
	require.NotNil(t, statePathFlag)
	assert.Equal(t, ".seed-state.json", statePathFlag.DefValue)
}

func TestSeedOptionsFromFlags_OmittedStatePathLeavesScenarioToChooseDefault(t *testing.T) {
	cmd := newSeedOptionsTestCommand(t)

	err := cmd.Flags().Set("scenario", seedapi.SeedScenarioE2E)
	require.NoError(t, err)

	opts := seedOptionsFromFlags(cmd)

	assert.Equal(t, seedapi.SeedScenarioE2E, opts.Scenario)
	assert.Empty(t, opts.StatePath)
}

func TestSeedOptionsFromFlags_ExplicitStatePathWins(t *testing.T) {
	cmd := newSeedOptionsTestCommand(t)

	err := cmd.Flags().Set("scenario", seedapi.SeedScenarioE2E)
	require.NoError(t, err)
	err = cmd.Flags().Set("state-path", "custom-state.json")
	require.NoError(t, err)

	opts := seedOptionsFromFlags(cmd)

	assert.Equal(t, "custom-state.json", opts.StatePath)
}

func TestValidateSeedFlagCombination_E2ERejectsDeterministicOverrides(t *testing.T) {
	cmd := newSeedOptionsTestCommand(t)

	err := cmd.Flags().Set("scenario", seedapi.SeedScenarioE2E)
	require.NoError(t, err)
	err = cmd.Flags().Set("tenant-slug", "custom-slug")
	require.NoError(t, err)

	err = validateSeedFlagCombination(cmd)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--tenant-slug")
	assert.Contains(t, err.Error(), seedapi.SeedScenarioE2E)
}

func TestValidateSeedFlagCombination_E2EAllowsStatePathOverride(t *testing.T) {
	cmd := newSeedOptionsTestCommand(t)

	err := cmd.Flags().Set("scenario", seedapi.SeedScenarioE2E)
	require.NoError(t, err)
	err = cmd.Flags().Set("state-path", "custom-state.json")
	require.NoError(t, err)

	err = validateSeedFlagCombination(cmd)

	require.NoError(t, err)
}

func TestValidateSeedFlagCombination_NonScenarioAllowsDeterministicOverrides(t *testing.T) {
	cmd := newSeedOptionsTestCommand(t)

	err := cmd.Flags().Set("tenant-slug", "custom-slug")
	require.NoError(t, err)
	err = cmd.Flags().Set("staff-password", "CustomPass123!")
	require.NoError(t, err)

	err = validateSeedFlagCombination(cmd)

	require.NoError(t, err)
}

func newSeedOptionsTestCommand(t *testing.T) *cobra.Command {
	t.Helper()

	cmd := &cobra.Command{Use: "seed"}
	cmd.Flags().String("scenario", "", "")
	cmd.Flags().String("tenant-slug", "", "")
	cmd.Flags().String("staff-password", "", "")
	cmd.Flags().String("admin-email", "", "")
	cmd.Flags().String("state-path", seedapi.DefaultSeedStatePath, "")

	return cmd
}
