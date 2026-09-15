package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testEnvironment map[string]string

func (e testEnvironment) getenv(key string) string { return e[key] }

func validPoolEnvironment() testEnvironment {
	return testEnvironment{
		"DB_MAX_OPEN_CONNS":     "40",
		"DB_MAX_IDLE_CONNS":     "20",
		"DB_CONN_MAX_LIFETIME":  "30m",
		"DB_CONN_MAX_IDLE_TIME": "10m",
	}
}

func testPoolConfigFromEnv(t *testing.T) {
	t.Run("accepts explicit values", func(t *testing.T) {
		env := validPoolEnvironment()

		config, err := poolConfigFrom(env.getenv)

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
				env := validPoolEnvironment()
				env[test.key] = test.value

				_, err := poolConfigFrom(env.getenv)

				require.Error(t, err)
				assert.Contains(t, err.Error(), test.want)
			})
		}
	})
}

// TestGetDatabaseDSN_ExplicitDSN verifies that an explicit DB_DSN is returned when set
func TestGetDatabaseDSN_ExplicitDSN(t *testing.T) {
	t.Parallel()
	env := testEnvironment{}

	customDSN := "postgres://user:pass@custom-host:5555/custom_db?sslmode=verify-full"
	env["DB_DSN"] = customDSN

	result, err := resolveDatabaseDSNFrom(env.getenv)

	require.NoError(t, err)
	assert.Equal(t, customDSN, result, "Explicit db_dsn should be returned")
}

// TestGetDatabaseDSN_TestEnv verifies that APP_ENV=test requires an explicit test database DSN.
func TestGetDatabaseDSN_TestEnv(t *testing.T) {
	t.Parallel()
	env := testEnvironment{}

	testDSN := "postgres://postgres:postgres@localhost:5433/phoenix_test?sslmode=disable"
	env["APP_ENV"] = "test"
	env["TEST_DB_DSN"] = testDSN

	result, err := resolveDatabaseDSNFrom(env.getenv)

	require.NoError(t, err)
	assert.Equal(t, testDSN, result, "APP_ENV=test should return explicit test_db_dsn")
}

// TestGetDatabaseDSN_DevelopmentEnvRequiresDSN verifies that development no longer invents a localhost DSN.
func TestGetDatabaseDSN_DevelopmentEnvRequiresDSN(t *testing.T) {
	t.Parallel()
	env := testEnvironment{}
	env["APP_ENV"] = "development"

	result, err := resolveDatabaseDSNFrom(env.getenv)

	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "DB_DSN")
}

// TestGetDatabaseDSN_TestDSNRequiresTestEnv verifies that TEST_DB_DSN is not a fallback outside test mode.
func TestGetDatabaseDSN_TestDSNRequiresTestEnv(t *testing.T) {
	t.Parallel()
	env := testEnvironment{}

	legacyDSN := "postgres://legacy:legacy@legacy-host:6543/legacy_test?sslmode=disable"
	env["TEST_DB_DSN"] = legacyDSN

	result, err := resolveDatabaseDSNFrom(env.getenv)

	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "DB_DSN")
}

// TestGetDatabaseDSN_MissingConfigFails verifies that no localhost fallback is returned.
func TestGetDatabaseDSN_MissingConfigFails(t *testing.T) {
	t.Parallel()
	env := testEnvironment{}

	result, err := resolveDatabaseDSNFrom(env.getenv)

	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "DB_DSN")

	testPoolConfigFromEnv(t)
}

// TestGetDatabaseDSN_TestEnvNeverUsesDevelopmentDSN proves the documented
// APP_ENV=test migration command cannot reset the development database merely
// because dev.env also contains DB_DSN.
func TestGetDatabaseDSN_TestEnvNeverUsesDevelopmentDSN(t *testing.T) {
	t.Parallel()
	env := testEnvironment{}

	developmentDSN := "postgres://development:development@localhost:5432/phoenix?sslmode=disable"
	testDSN := "postgres://test:test@localhost:5433/phoenix_test?sslmode=disable"
	env["DB_DSN"] = developmentDSN
	env["TEST_DB_DSN"] = testDSN
	env["APP_ENV"] = "test"

	result, err := resolveDatabaseDSNFrom(env.getenv)

	require.NoError(t, err)
	assert.Equal(t, testDSN, result)
	assert.NotEqual(t, developmentDSN, result)
}

func TestGetDatabaseDSN_TestEnvWithoutTestDSNFailsEvenWhenDevelopmentDSNExists(t *testing.T) {
	t.Parallel()
	env := testEnvironment{}

	env["DB_DSN"] = "postgres://development:development@localhost:5432/phoenix?sslmode=disable"
	env["APP_ENV"] = "test"

	result, err := resolveDatabaseDSNFrom(env.getenv)

	require.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "TEST_DB_DSN")
}

// TestGetDatabaseDSN_ExplicitDSN_OverridesLegacy verifies that explicit DB_DSN takes precedence over TEST_DB_DSN
func TestGetDatabaseDSN_ExplicitDSN_OverridesLegacy(t *testing.T) {
	t.Parallel()
	env := testEnvironment{}

	customDSN := "postgres://explicit:explicit@explicit-host:8888/explicit_db?sslmode=require"
	legacyDSN := "postgres://legacy:legacy@legacy-host:6543/legacy_test?sslmode=disable"
	env["DB_DSN"] = customDSN
	env["TEST_DB_DSN"] = legacyDSN // Should be ignored

	result, err := resolveDatabaseDSNFrom(env.getenv)

	require.NoError(t, err)
	assert.Equal(t, customDSN, result, "Explicit db_dsn should override legacy test_db_dsn")
}
