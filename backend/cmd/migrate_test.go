package cmd

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/migrations"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateRootMapsCommandsToStableOperations(t *testing.T) {
	t.Parallel()

	root := defaultMigrateRoot
	expected := map[string]migrationOperation{
		"migrate": migrations.Migrate,
		"reset":   migrations.Reset,
		"status":  migrations.MigrateStatus,
	}
	for command, want := range expected {
		op, err := root.operation(command)
		require.NoError(t, err)
		assert.Equal(t, reflect.ValueOf(want).Pointer(), reflect.ValueOf(op).Pointer())
	}
	_, err := root.operation("unknown")
	require.EqualError(t, err, `unknown migration operation "unknown"`)
}

// =============================================================================
// migrateCmd Tests
// =============================================================================

func TestMigrateCmd_Metadata(t *testing.T) {
	assert.Equal(t, "migrate", migrateCmd.Use)
	assert.Equal(t, "use bun migration tool", migrateCmd.Short)
	assert.Equal(t, "run bun migrations", migrateCmd.Long)
	assert.NotNil(t, migrateCmd.RunE)
}

func TestMigrateRootFailsFastWithoutDatabaseDependency(t *testing.T) {
	err := (migrateRoot{}).run(t.Context(), nil)

	require.ErrorContains(t, err, "database opener is required")
}

func TestMigrateResetCmd_Metadata(t *testing.T) {
	assert.Equal(t, "reset", migrateResetCmd.Use)
	assert.Equal(t, "reset database and run all migrations", migrateResetCmd.Short)
	assert.Contains(t, migrateResetCmd.Long, "WARNING")
	assert.NotNil(t, migrateResetCmd.RunE)
}

func TestMigrateStatusCmd_Metadata(t *testing.T) {
	assert.Equal(t, "status", migrateStatusCmd.Use)
	assert.Equal(t, "show migration status", migrateStatusCmd.Short)
	assert.Contains(t, migrateStatusCmd.Long, "status of all migrations")
	assert.NotNil(t, migrateStatusCmd.RunE)
}

func TestMigrateValidateCmd_Metadata(t *testing.T) {
	assert.Equal(t, "validate", migrateValidateCmd.Use)
	assert.Equal(t, "validate migration dependencies", migrateValidateCmd.Short)
	assert.Contains(t, migrateValidateCmd.Long, "dependencies")
	assert.NotNil(t, migrateValidateCmd.RunE)
}

// =============================================================================
// Subcommand Registration Tests (verifies init() ran correctly)
// =============================================================================

func TestMigrateCmd_IsRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range RootCmd.Commands() {
		if cmd.Use == "migrate" {
			found = true
			break
		}
	}
	assert.True(t, found, "migrateCmd should be registered on RootCmd")
}

func TestMigrateCmd_HasSubcommands(t *testing.T) {
	subcommands := migrateCmd.Commands()
	names := make([]string, 0, len(subcommands))
	for _, cmd := range subcommands {
		names = append(names, cmd.Use)
	}

	assert.Contains(t, names, "reset", "migrateCmd should have 'reset' subcommand")
	assert.Contains(t, names, "status", "migrateCmd should have 'status' subcommand")
	assert.Contains(t, names, "validate", "migrateCmd should have 'validate' subcommand")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestMigrateCmd_SubcommandCount(t *testing.T) {
	assert.Len(t, migrateCmd.Commands(), 3, "migrateCmd should have exactly 3 subcommands")
}

// captureStdout captures stdout output during a function call.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestMigrateValidateCmd_Run_Success(t *testing.T) {
	// ValidateMigrations() and PrintMigrationPlan() are pure in-memory checks
	// that operate on the global MigrationRegistry — no database needed.
	var runErr error
	output := captureStdout(t, func() {
		runErr = migrateValidateCmd.RunE(migrateValidateCmd, []string{})
	})

	assert.NoError(t, runErr)
	assert.Contains(t, output, "All migrations validated successfully!")
	assert.Contains(t, output, "Migration Plan:")
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestMigrateValidateCmd_Run_ValidationError(t *testing.T) {
	// Inject a migration with a broken dependency to trigger the error path.
	migrations.MigrationRegistry["test_broken_dep"] = &migrations.Migration{
		Version:     "999.0.0",
		Description: "broken test migration",
		DependsOn:   []string{"nonexistent_migration"},
	}
	defer delete(migrations.MigrationRegistry, "test_broken_dep")

	var runErr error
	output := captureStdout(t, func() {
		runErr = migrateValidateCmd.RunE(migrateValidateCmd, []string{})
	})

	assert.Error(t, runErr)
	assert.Contains(t, runErr.Error(), "migration validation failed")
	assert.NotContains(t, output, "All migrations validated successfully!")
}

// =============================================================================
// Command Type Tests
// =============================================================================

func TestMigrateCommands_AreCobraCommands(t *testing.T) {
	commands := []*cobra.Command{
		migrateCmd,
		migrateResetCmd,
		migrateStatusCmd,
		migrateValidateCmd,
	}

	for _, cmd := range commands {
		assert.IsType(t, &cobra.Command{}, cmd, "should be a *cobra.Command")
	}
}

func TestMigrateCmd_ParentChildRelationship(t *testing.T) {
	assert.Equal(t, migrateCmd, migrateResetCmd.Parent())
	assert.Equal(t, migrateCmd, migrateStatusCmd.Parent())
	assert.Equal(t, migrateCmd, migrateValidateCmd.Parent())
}

// =============================================================================
// Usage Output Tests (ensures commands are fully wired)
// =============================================================================

func TestMigrateCmd_UsageOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	migrateCmd.SetOut(buf)
	migrateCmd.SetErr(buf)

	err := migrateCmd.Usage()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "migrate")
	assert.Contains(t, output, "Available Commands")
	assert.Contains(t, output, "reset")
	assert.Contains(t, output, "status")
	assert.Contains(t, output, "validate")
}

func TestMigrateResetCmd_UsageOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	migrateResetCmd.SetOut(buf)
	migrateResetCmd.SetErr(buf)

	err := migrateResetCmd.Usage()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "reset")
}

func TestMigrateStatusCmd_UsageOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	migrateStatusCmd.SetOut(buf)
	migrateStatusCmd.SetErr(buf)

	err := migrateStatusCmd.Usage()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "status")
}

func TestMigrateValidateCmd_UsageOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	migrateValidateCmd.SetOut(buf)
	migrateValidateCmd.SetErr(buf)

	err := migrateValidateCmd.Usage()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "validate")
}
