package database

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetDatabaseDSN_ExplicitDSN verifies that an explicit DB_DSN is returned when set
func TestGetDatabaseDSN_ExplicitDSN(t *testing.T) {
	defer viper.Reset()

	customDSN := "postgres://user:pass@custom-host:5555/custom_db?sslmode=verify-full"
	viper.Set("db_dsn", customDSN)

	result, err := resolveDatabaseDSN()

	require.NoError(t, err)
	assert.Equal(t, customDSN, result, "Explicit db_dsn should be returned")
}

// TestGetDatabaseDSN_TestEnv verifies that APP_ENV=test requires an explicit test database DSN.
func TestGetDatabaseDSN_TestEnv(t *testing.T) {
	defer viper.Reset()

	testDSN := "postgres://postgres:postgres@localhost:5433/phoenix_test?sslmode=disable"
	viper.Set("app_env", "test")
	viper.Set("test_db_dsn", testDSN)

	result, err := resolveDatabaseDSN()

	require.NoError(t, err)
	assert.Equal(t, testDSN, result, "APP_ENV=test should return explicit test_db_dsn")
}

// TestGetDatabaseDSN_DevelopmentEnvRequiresDSN verifies that development no longer invents a localhost DSN.
func TestGetDatabaseDSN_DevelopmentEnvRequiresDSN(t *testing.T) {
	defer viper.Reset()

	viper.Set("app_env", "development")

	result, err := resolveDatabaseDSN()

	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "DB_DSN")
}

// TestGetDatabaseDSN_TestDSNRequiresTestEnv verifies that TEST_DB_DSN is not a fallback outside test mode.
func TestGetDatabaseDSN_TestDSNRequiresTestEnv(t *testing.T) {
	defer viper.Reset()

	legacyDSN := "postgres://legacy:legacy@legacy-host:6543/legacy_test?sslmode=disable"
	viper.Set("test_db_dsn", legacyDSN)

	result, err := resolveDatabaseDSN()

	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "DB_DSN")
}

// TestGetDatabaseDSN_MissingConfigFails verifies that no localhost fallback is returned.
func TestGetDatabaseDSN_MissingConfigFails(t *testing.T) {
	defer viper.Reset()

	result, err := resolveDatabaseDSN()

	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "DB_DSN")
}

// TestGetDatabaseDSN_TestEnvNeverUsesDevelopmentDSN proves the documented
// APP_ENV=test migration command cannot reset the development database merely
// because dev.env also contains DB_DSN.
func TestGetDatabaseDSN_TestEnvNeverUsesDevelopmentDSN(t *testing.T) {
	defer viper.Reset()

	developmentDSN := "postgres://development:development@localhost:5432/phoenix?sslmode=disable"
	testDSN := "postgres://test:test@localhost:5433/phoenix_test?sslmode=disable"
	viper.Set("db_dsn", developmentDSN)
	viper.Set("test_db_dsn", testDSN)
	viper.Set("app_env", "test")

	result, err := resolveDatabaseDSN()

	require.NoError(t, err)
	assert.Equal(t, testDSN, result)
	assert.NotEqual(t, developmentDSN, result)
}

func TestGetDatabaseDSN_TestEnvWithoutTestDSNFailsEvenWhenDevelopmentDSNExists(t *testing.T) {
	defer viper.Reset()

	viper.Set("db_dsn", "postgres://development:development@localhost:5432/phoenix?sslmode=disable")
	viper.Set("app_env", "test")

	result, err := resolveDatabaseDSN()

	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "TEST_DB_DSN")
}

// TestGetDatabaseDSN_ExplicitDSN_OverridesLegacy verifies that explicit DB_DSN takes precedence over TEST_DB_DSN
func TestGetDatabaseDSN_ExplicitDSN_OverridesLegacy(t *testing.T) {
	defer viper.Reset()

	customDSN := "postgres://explicit:explicit@explicit-host:8888/explicit_db?sslmode=require"
	legacyDSN := "postgres://legacy:legacy@legacy-host:6543/legacy_test?sslmode=disable"
	viper.Set("db_dsn", customDSN)
	viper.Set("test_db_dsn", legacyDSN) // Should be ignored

	result, err := resolveDatabaseDSN()

	require.NoError(t, err)
	assert.Equal(t, customDSN, result, "Explicit db_dsn should override legacy test_db_dsn")
}
