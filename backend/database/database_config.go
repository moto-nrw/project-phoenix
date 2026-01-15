package database

import (
	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/spf13/viper"
)

// GetDatabaseDSN returns the database connection string based on environment.
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
		return "postgres://postgres:postgres@localhost:5432/phoenix?sslmode=require"
	case "production":
		// Production requires explicit DB_DSN (fail fast if missing)
		logging.Logger.Fatal("APP_ENV=production requires explicit DB_DSN environment variable")
	}

	// 3. Legacy TEST_DB_DSN support (backwards compatibility)
	if testDSN := viper.GetString("test_db_dsn"); testDSN != "" {
		return testDSN
	}

	// 4. Fallback to development default
	// This allows: go run main.go serve (without setting APP_ENV explicitly)
	return "postgres://postgres:postgres@localhost:5432/phoenix?sslmode=require"
}
