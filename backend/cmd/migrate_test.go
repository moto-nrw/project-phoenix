package cmd

import (
	"bytes"
	"errors"
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
	t.Parallel()
	assert.Equal(t, "migrate", migrateCmd.Use)
	assert.Equal(t, "use bun migration tool", migrateCmd.Short)
	assert.Equal(t, "run bun migrations", migrateCmd.Long)
	assert.NotNil(t, migrateCmd.RunE)
}

func TestMigrateRootFailsFastWithoutDatabaseDependency(t *testing.T) {
	t.Parallel()
	err := (migrateRoot{}).run(t.Context(), nil)

	require.ErrorContains(t, err, "database opener is required")
}

func TestMigrateResetCmd_Metadata(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "reset", migrateResetCmd.Use)
	assert.Equal(t, "reset database and run all migrations", migrateResetCmd.Short)
	assert.Contains(t, migrateResetCmd.Long, "WARNING")
	assert.NotNil(t, migrateResetCmd.RunE)
}

func TestMigrateStatusCmd_Metadata(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "status", migrateStatusCmd.Use)
	assert.Equal(t, "show migration status", migrateStatusCmd.Short)
	assert.Contains(t, migrateStatusCmd.Long, "status of all migrations")
	assert.NotNil(t, migrateStatusCmd.RunE)
}

func TestMigrateValidateCmd_Metadata(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "validate", migrateValidateCmd.Use)
	assert.Equal(t, "validate migration dependencies", migrateValidateCmd.Short)
	assert.Contains(t, migrateValidateCmd.Long, "dependencies")
	assert.NotNil(t, migrateValidateCmd.RunE)
}

func TestMigrateValidateCmd_Run_Success(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	runErr := runMigrationValidation(&output, func() error { return nil }, migrations.ValidateMigrations, migrations.PrintMigrationPlanTo)

	assert.NoError(t, runErr)
	assert.Contains(t, output.String(), "All migrations validated successfully!")
	assert.Contains(t, output.String(), "Migration Plan:")
}

func TestMigrateValidateCmd_Run_ValidationError(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	runErr := runMigrationValidation(&output, func() error { return nil }, func() error {
		return errors.New("broken dependency")
	}, migrations.PrintMigrationPlanTo)

	assert.Error(t, runErr)
	assert.Contains(t, runErr.Error(), "migration validation failed")
	assert.NotContains(t, output.String(), "All migrations validated successfully!")
}

// =============================================================================
// Command Type Tests
// =============================================================================

func TestMigrateCommands_AreCobraCommands(t *testing.T) {
	t.Parallel()
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
