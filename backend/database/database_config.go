package database

import (
	"log/slog"
	"net/url"
	"os"

	"github.com/spf13/viper"
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
		slog.Error("APP_ENV=production requires explicit DB_DSN environment variable")
		os.Exit(1)
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
// PHOENIX_AUTH_PASSWORD is mandatory. The server will refuse to start without it.
// Run migration V1.14.1 to create the phoenix_auth role, then set the password
// in your env file (dev.env for local, .env for Docker).
func GetServeDSN() string {
	baseDSN := GetDatabaseDSN()

	password := os.Getenv("PHOENIX_AUTH_PASSWORD")
	if password == "" {
		password = viper.GetString("phoenix_auth_password")
	}
	if password == "" {
		slog.Error("PHOENIX_AUTH_PASSWORD is required for serve — set it in your env file after running migration V1.14.1")
		os.Exit(1)
	}

	parsed, err := url.Parse(baseDSN)
	if err != nil {
		slog.Error("failed to parse DB_DSN for phoenix_auth substitution", slog.String("error", err.Error()))
		os.Exit(1)
	}

	parsed.User = url.UserPassword("phoenix_auth", password)
	return parsed.String()
}
