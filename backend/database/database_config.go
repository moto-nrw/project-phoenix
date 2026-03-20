package database

import (
	"database/sql"
	"log"
	"log/slog"
	"net/url"
	"os"

	"github.com/spf13/viper"
	"github.com/uptrace/bun/driver/pgdriver"
)

// GetDatabaseDSN returns the database connection string based on environment.
// Used by CLI commands (migrate, seed, cleanup) which run as the postgres superuser.
//
// Precedence order:
// 1. Explicit DB_DSN environment variable (for production/Docker overrides)
// 2. APP_ENV environment variable (test/development/production smart defaults)
// 3. Legacy TEST_DB_DSN variable (backwards compatibility)
// 4. Fallback to development default (localhost:5432)
//
// Examples:
//   - Development (default): go run main.go serve
//   - Test database: APP_ENV=test go run main.go migrate reset
//   - Production: DB_DSN="postgres://..." go run main.go serve
func GetDatabaseDSN() string {
	// 1. Explicit DB_DSN (production/Docker override) - highest priority
	if dsn := viper.GetString("db_dsn"); dsn != "" {
		return dsn
	}

	// 2. APP_ENV-based smart defaults
	appEnv := viper.GetString("app_env")
	switch appEnv {
	case "test":
		// Test database on port 5433 (separate from dev on 5432)
		return "postgres://postgres:postgres@localhost:5433/phoenix_test?sslmode=disable"
	case "development":
		// Development database with SSL (sslmode=require for GDPR compliance)
		// Database name "postgres" matches Docker Compose default (no POSTGRES_DB override)
		return "postgres://postgres:postgres@localhost:5432/postgres?sslmode=require"
	case "production":
		// Production requires explicit DB_DSN (fail fast if missing)
		log.Fatal("APP_ENV=production requires explicit DB_DSN environment variable")
	}

	// 3. Legacy TEST_DB_DSN support (backwards compatibility)
	if testDSN := viper.GetString("test_db_dsn"); testDSN != "" {
		return testDSN
	}

	// 4. Fallback to development default
	// This allows: go run main.go serve (without setting APP_ENV explicitly)
	return "postgres://postgres:postgres@localhost:5432/postgres?sslmode=require"
}

// GetServeDSN returns the database connection string for the HTTP server.
// Connects as the phoenix_auth role (NOINHERIT, can SET ROLE to phoenix_tenant/phoenix_admin)
// instead of the postgres superuser. This enforces least-privilege at the connection level.
//
// The DSN is auto-constructed by replacing the user/password in the base DSN with
// phoenix_auth and the PHOENIX_AUTH_PASSWORD environment variable. If the password
// is not set, falls back to the superuser DSN with a warning. If the password is set
// but the phoenix_auth role doesn't exist yet (migrations haven't run), it also falls
// back gracefully so the server can start and run migrations.
func GetServeDSN() string {
	baseDSN := GetDatabaseDSN()

	password := os.Getenv("PHOENIX_AUTH_PASSWORD")
	if password == "" {
		password = viper.GetString("phoenix_auth_password")
	}
	if password == "" {
		slog.Warn("PHOENIX_AUTH_PASSWORD not set — serve command falling back to superuser connection. " +
			"Set PHOENIX_AUTH_PASSWORD in your env file after running migration V1.14.1.")
		return baseDSN
	}

	parsed, err := url.Parse(baseDSN)
	if err != nil {
		slog.Warn("failed to parse DB_DSN for phoenix_auth substitution, using superuser connection",
			"error", err)
		return baseDSN
	}

	parsed.User = url.UserPassword("phoenix_auth", password)
	phoenixDSN := parsed.String()

	// Verify the phoenix_auth role can actually connect. If the role doesn't exist
	// yet (migrations haven't run), fall back to the superuser DSN so the server
	// can start and run migrations that create the role.
	testDB := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(phoenixDSN)))
	err = testDB.Ping()
	_ = testDB.Close()

	if err != nil {
		slog.Warn("phoenix_auth connection failed — falling back to superuser connection. "+
			"Run migrations to create the phoenix_auth role.",
			"error", err)
		return baseDSN
	}

	return phoenixDSN
}
