package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearDatabaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "")
	t.Setenv("DB_DSN", "")
	t.Setenv("TEST_DB_DSN", "")
}

func testPoolConfigFromEnv(t *testing.T) {
	setValidPoolConfig := func(t *testing.T) {
		t.Helper()
		t.Setenv("DB_MAX_OPEN_CONNS", "40")
		t.Setenv("DB_MAX_IDLE_CONNS", "20")
		t.Setenv("DB_CONN_MAX_LIFETIME", "30m")
		t.Setenv("DB_CONN_MAX_IDLE_TIME", "10m")
	}

	t.Run("accepts explicit values", func(t *testing.T) {
		setValidPoolConfig(t)

		config, err := poolConfigFromEnv()

		require.NoError(t, err)
		assert.Equal(t, 40, config.maxOpen)
		assert.Equal(t, 20, config.maxIdle)
		assert.Equal(t, 30*time.Minute, config.maxLifetime)
		assert.Equal(t, 10*time.Minute, config.maxIdleTime)
	})

	t.Run("rejects missing or invalid values", func(t *testing.T) {
		for _, test := range []struct {
			name  string
			key   string
			value string
			want  string
		}{
			{name: "missing open connections", key: "DB_MAX_OPEN_CONNS", want: "DB_MAX_OPEN_CONNS"},
			{name: "invalid idle connections", key: "DB_MAX_IDLE_CONNS", value: "many", want: "DB_MAX_IDLE_CONNS"},
			{name: "too many idle connections", key: "DB_MAX_IDLE_CONNS", value: "41", want: "DB_MAX_IDLE_CONNS"},
			{name: "zero lifetime", key: "DB_CONN_MAX_LIFETIME", value: "0s", want: "DB_CONN_MAX_LIFETIME"},
			{name: "invalid idle time", key: "DB_CONN_MAX_IDLE_TIME", value: "later", want: "DB_CONN_MAX_IDLE_TIME"},
		} {
			t.Run(test.name, func(t *testing.T) {
				setValidPoolConfig(t)
				t.Setenv(test.key, test.value)

				_, err := poolConfigFromEnv()

				require.Error(t, err)
				assert.Contains(t, err.Error(), test.want)
			})
		}
	})
}

// TestGetDatabaseDSN_ExplicitDSN verifies that an explicit DB_DSN is returned when set
// Deliberately NOT parallel: mutates process-global configuration.
func TestGetDatabaseDSN_ExplicitDSN(t *testing.T) {
	clearDatabaseEnv(t)

	customDSN := "postgres://user:pass@custom-host:5555/custom_db?sslmode=verify-full"
	t.Setenv("DB_DSN", customDSN)

	result, err := resolveDatabaseDSN()

	require.NoError(t, err)
	assert.Equal(t, customDSN, result, "Explicit db_dsn should be returned")
}

// TestGetDatabaseDSN_TestEnv verifies that APP_ENV=test requires an explicit test database DSN.
// Deliberately NOT parallel: mutates process-global configuration.
func TestGetDatabaseDSN_TestEnv(t *testing.T) {
	clearDatabaseEnv(t)

	testDSN := "postgres://postgres:postgres@localhost:5433/phoenix_test?sslmode=disable"
	t.Setenv("APP_ENV", "test")
	t.Setenv("TEST_DB_DSN", testDSN)

	result, err := resolveDatabaseDSN()

	require.NoError(t, err)
	assert.Equal(t, testDSN, result, "APP_ENV=test should return explicit test_db_dsn")
}

// TestGetDatabaseDSN_DevelopmentEnvRequiresDSN verifies that development no longer invents a localhost DSN.
// Deliberately NOT parallel: mutates process-global configuration.
func TestGetDatabaseDSN_DevelopmentEnvRequiresDSN(t *testing.T) {
	clearDatabaseEnv(t)
	t.Setenv("APP_ENV", "development")

	result, err := resolveDatabaseDSN()

	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "DB_DSN")
}

// TestGetDatabaseDSN_TestDSNRequiresTestEnv verifies that TEST_DB_DSN is not a fallback outside test mode.
// Deliberately NOT parallel: mutates process-global configuration.
func TestGetDatabaseDSN_TestDSNRequiresTestEnv(t *testing.T) {
	clearDatabaseEnv(t)

	legacyDSN := "postgres://legacy:legacy@legacy-host:6543/legacy_test?sslmode=disable"
	t.Setenv("TEST_DB_DSN", legacyDSN)

	result, err := resolveDatabaseDSN()

	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "DB_DSN")
}

// TestGetDatabaseDSN_MissingConfigFails verifies that no localhost fallback is returned.
// Deliberately NOT parallel: mutates process-global configuration.
func TestGetDatabaseDSN_MissingConfigFails(t *testing.T) {
	clearDatabaseEnv(t)

	result, err := resolveDatabaseDSN()

	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "DB_DSN")

	testPoolConfigFromEnv(t)
}

// TestGetDatabaseDSN_TestEnvNeverUsesDevelopmentDSN proves the documented
// APP_ENV=test migration command cannot reset the development database merely
// because dev.env also contains DB_DSN.
// Deliberately NOT parallel: mutates process-global configuration.
func TestGetDatabaseDSN_TestEnvNeverUsesDevelopmentDSN(t *testing.T) {
	clearDatabaseEnv(t)

	developmentDSN := "postgres://development:development@localhost:5432/phoenix?sslmode=disable"
	testDSN := "postgres://test:test@localhost:5433/phoenix_test?sslmode=disable"
	t.Setenv("DB_DSN", developmentDSN)
	t.Setenv("TEST_DB_DSN", testDSN)
	t.Setenv("APP_ENV", "test")

	result, err := resolveDatabaseDSN()

	require.NoError(t, err)
	assert.Equal(t, testDSN, result)
	assert.NotEqual(t, developmentDSN, result)
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestGetDatabaseDSN_TestEnvWithoutTestDSNFailsEvenWhenDevelopmentDSNExists(t *testing.T) {
	clearDatabaseEnv(t)

	t.Setenv("DB_DSN", "postgres://development:development@localhost:5432/phoenix?sslmode=disable")
	t.Setenv("APP_ENV", "test")

	result, err := resolveDatabaseDSN()

	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "TEST_DB_DSN")
}

// TestGetDatabaseDSN_ExplicitDSN_OverridesLegacy verifies that explicit DB_DSN takes precedence over TEST_DB_DSN
// Deliberately NOT parallel: mutates process-global configuration.
func TestGetDatabaseDSN_ExplicitDSN_OverridesLegacy(t *testing.T) {
	clearDatabaseEnv(t)

	customDSN := "postgres://explicit:explicit@explicit-host:8888/explicit_db?sslmode=require"
	legacyDSN := "postgres://legacy:legacy@legacy-host:6543/legacy_test?sslmode=disable"
	t.Setenv("DB_DSN", customDSN)
	t.Setenv("TEST_DB_DSN", legacyDSN) // Should be ignored

	result, err := resolveDatabaseDSN()

	require.NoError(t, err)
	assert.Equal(t, customDSN, result, "Explicit db_dsn should override legacy test_db_dsn")
}
