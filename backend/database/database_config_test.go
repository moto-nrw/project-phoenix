package database

import (
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testResolvePoolConfig(t *testing.T) {
	t.Run("accepts explicit values", func(t *testing.T) {
		defer viper.Reset()
		viper.Set("db_max_open_conns", 40)
		viper.Set("db_max_idle_conns", 20)
		viper.Set("db_conn_max_lifetime", "30m")
		viper.Set("db_conn_max_idle_time", "10m")

		config, err := resolvePoolConfig()

		require.NoError(t, err)
		assert.Equal(t, 40, config.maxOpen)
		assert.Equal(t, 20, config.maxIdle)
		assert.Equal(t, 30*time.Minute, config.lifetime)
		assert.Equal(t, 10*time.Minute, config.idleTime)
	})

	t.Run("rejects missing or invalid values", func(t *testing.T) {
		for _, test := range []struct {
			name  string
			key   string
			value string
			want  string
		}{
			{name: "missing open connections", want: "DB_MAX_OPEN_CONNS"},
			{name: "invalid idle connections", key: "db_max_idle_conns", value: "many", want: "DB_MAX_IDLE_CONNS"},
			{name: "zero lifetime", key: "db_conn_max_lifetime", value: "0s", want: "DB_CONN_MAX_LIFETIME"},
			{name: "invalid idle time", key: "db_conn_max_idle_time", value: "later", want: "DB_CONN_MAX_IDLE_TIME"},
		} {
			t.Run(test.name, func(t *testing.T) {
				defer viper.Reset()
				viper.Set("db_max_open_conns", 40)
				viper.Set("db_max_idle_conns", 20)
				viper.Set("db_conn_max_lifetime", "30m")
				viper.Set("db_conn_max_idle_time", "10m")
				if test.key == "" {
					viper.Reset()
				} else {
					viper.Set(test.key, test.value)
				}

				_, err := resolvePoolConfig()

				require.Error(t, err)
				assert.Contains(t, err.Error(), test.want)
			})
		}
	})
}

// TestGetDatabaseDSN_ExplicitDSN verifies that an explicit DB_DSN is returned when set
// Deliberately NOT parallel: mutates process-global configuration.
func TestGetDatabaseDSN_ExplicitDSN(t *testing.T) {
	defer viper.Reset()

	customDSN := "postgres://user:pass@custom-host:5555/custom_db?sslmode=verify-full"
	viper.Set("db_dsn", customDSN)

	result, err := resolveDatabaseDSN()

	require.NoError(t, err)
	assert.Equal(t, customDSN, result, "Explicit db_dsn should be returned")
}

// TestGetDatabaseDSN_TestEnv verifies that APP_ENV=test requires an explicit test database DSN.
// Deliberately NOT parallel: mutates process-global configuration.
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
// Deliberately NOT parallel: mutates process-global configuration.
func TestGetDatabaseDSN_DevelopmentEnvRequiresDSN(t *testing.T) {
	defer viper.Reset()

	viper.Set("app_env", "development")

	result, err := resolveDatabaseDSN()

	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "DB_DSN")
}

// TestGetDatabaseDSN_TestDSNRequiresTestEnv verifies that TEST_DB_DSN is not a fallback outside test mode.
// Deliberately NOT parallel: mutates process-global configuration.
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
// Deliberately NOT parallel: mutates process-global configuration.
func TestGetDatabaseDSN_MissingConfigFails(t *testing.T) {
	defer viper.Reset()

	result, err := resolveDatabaseDSN()

	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "DB_DSN")

	testResolvePoolConfig(t)
}

// TestGetDatabaseDSN_TestEnvNeverUsesDevelopmentDSN proves the documented
// APP_ENV=test migration command cannot reset the development database merely
// because dev.env also contains DB_DSN.
// Deliberately NOT parallel: mutates process-global configuration.
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

// Deliberately NOT parallel: mutates process-global configuration.
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
// Deliberately NOT parallel: mutates process-global configuration.
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
