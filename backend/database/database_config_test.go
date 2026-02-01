package database

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// TestGetDatabaseDSN_ExplicitDSN verifies that an explicit DB_DSN is returned when set
func TestGetDatabaseDSN_ExplicitDSN(t *testing.T) {
	defer viper.Reset()

	customDSN := "postgres://user:pass@custom-host:5555/custom_db?sslmode=verify-full"
	viper.Set("db_dsn", customDSN)

	result := GetDatabaseDSN()

	assert.Equal(t, customDSN, result, "Explicit db_dsn should be returned")
}

// TestGetDatabaseDSN_TestEnv verifies that APP_ENV=test returns the test database DSN
func TestGetDatabaseDSN_TestEnv(t *testing.T) {
	defer viper.Reset()

	viper.Set("app_env", "test")

	result := GetDatabaseDSN()

	expectedDSN := "postgres://postgres:postgres@localhost:5433/phoenix_test?sslmode=disable"
	assert.Equal(t, expectedDSN, result, "APP_ENV=test should return test DB DSN on port 5433")
}

// TestGetDatabaseDSN_DevelopmentEnv verifies that APP_ENV=development returns the development database DSN
func TestGetDatabaseDSN_DevelopmentEnv(t *testing.T) {
	defer viper.Reset()

	viper.Set("app_env", "development")

	result := GetDatabaseDSN()

	expectedDSN := "postgres://postgres:postgres@localhost:5432/postgres?sslmode=require"
	assert.Equal(t, expectedDSN, result, "APP_ENV=development should return dev DB DSN on port 5432")
}

// TestGetDatabaseDSN_LegacyTestDSN verifies that TEST_DB_DSN is returned when set (backwards compatibility)
func TestGetDatabaseDSN_LegacyTestDSN(t *testing.T) {
	defer viper.Reset()

	legacyDSN := "postgres://legacy:legacy@legacy-host:6543/legacy_test?sslmode=disable"
	viper.Set("test_db_dsn", legacyDSN)

	result := GetDatabaseDSN()

	assert.Equal(t, legacyDSN, result, "Legacy test_db_dsn should be returned for backwards compatibility")
}

// TestGetDatabaseDSN_FallbackDefault verifies that the development default is returned when no config is set
func TestGetDatabaseDSN_FallbackDefault(t *testing.T) {
	defer viper.Reset()

	// No configuration set at all

	result := GetDatabaseDSN()

	expectedDSN := "postgres://postgres:postgres@localhost:5432/postgres?sslmode=require"
	assert.Equal(t, expectedDSN, result, "Should fallback to development default when no config is set")
}

// TestGetDatabaseDSN_ExplicitDSN_OverridesAppEnv verifies that explicit DB_DSN takes precedence over APP_ENV
func TestGetDatabaseDSN_ExplicitDSN_OverridesAppEnv(t *testing.T) {
	defer viper.Reset()

	customDSN := "postgres://explicit:explicit@explicit-host:7777/explicit_db?sslmode=require"
	viper.Set("db_dsn", customDSN)
	viper.Set("app_env", "test") // Should be ignored

	result := GetDatabaseDSN()

	assert.Equal(t, customDSN, result, "Explicit db_dsn should override app_env")
}

// TestGetDatabaseDSN_ExplicitDSN_OverridesLegacy verifies that explicit DB_DSN takes precedence over TEST_DB_DSN
func TestGetDatabaseDSN_ExplicitDSN_OverridesLegacy(t *testing.T) {
	defer viper.Reset()

	customDSN := "postgres://explicit:explicit@explicit-host:8888/explicit_db?sslmode=require"
	legacyDSN := "postgres://legacy:legacy@legacy-host:6543/legacy_test?sslmode=disable"
	viper.Set("db_dsn", customDSN)
	viper.Set("test_db_dsn", legacyDSN) // Should be ignored

	result := GetDatabaseDSN()

	assert.Equal(t, customDSN, result, "Explicit db_dsn should override legacy test_db_dsn")
}
