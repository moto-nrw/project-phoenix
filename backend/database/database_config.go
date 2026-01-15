package database

import (
	"fmt"
	"os"

	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/spf13/viper"
)

// GetDatabaseDSN returns the database connection string based on environment.
//
// 12-Factor Compliant: All configuration comes from environment variables.
// The app fails fast if required configuration is missing.
//
// Required environment variables:
//   - DB_DSN: Full database connection string (required for production)
//
// Optional environment variables:
//   - APP_ENV: Application environment (development|test|production)
//   - TEST_DB_DSN: Legacy support for test database connection
//
// Behavior by APP_ENV:
//   - production: DB_DSN is REQUIRED, app fails if missing
//   - test: Uses TEST_DB_DSN or DB_DSN, fails if neither set
//   - development: Uses DB_DSN, fails if missing (no hardcoded defaults)
//   - (unset): Treated as development
//
// Examples:
//   - Development: DB_DSN="postgres://..." go run main.go serve
//   - Test: APP_ENV=test TEST_DB_DSN="postgres://..." go test ./...
//   - Production: APP_ENV=production DB_DSN="postgres://..." ./main serve
func GetDatabaseDSN() string {
	appEnv := viper.GetString("app_env")
	if appEnv == "" {
		appEnv = "development"
	}

	// 1. Explicit DB_DSN - highest priority (12-Factor: config from environment)
	if dsn := viper.GetString("db_dsn"); dsn != "" {
		return dsn
	}

	// 2. APP_ENV-specific handling
	switch appEnv {
	case "test":
		// Test environment: check TEST_DB_DSN for backwards compatibility
		if testDSN := viper.GetString("test_db_dsn"); testDSN != "" {
			return testDSN
		}
		// 12-Factor: Fail fast if test database not configured
		failMissingConfig("test", "TEST_DB_DSN or DB_DSN")
		return "" // unreachable

	case "production":
		// 12-Factor: Production MUST have explicit configuration
		failMissingConfig("production", "DB_DSN")
		return "" // unreachable

	case "development":
		// 12-Factor: Even development should not have hardcoded secrets
		failMissingConfig("development", "DB_DSN")
		return "" // unreachable

	default:
		// Unknown environment - fail fast
		if logger.Logger != nil {
			logger.Logger.Fatalf("Unknown APP_ENV value: %s (expected: development, test, or production)", appEnv)
		} else {
			fmt.Fprintf(os.Stderr, "FATAL: Unknown APP_ENV value: %s (expected: development, test, or production)\n", appEnv)
			os.Exit(1)
		}
		return "" // unreachable
	}
}

// failMissingConfig logs a fatal error for missing required configuration.
// This enforces 12-Factor principle: fail fast when config is missing.
func failMissingConfig(env, requiredVar string) {
	msg := fmt.Sprintf("APP_ENV=%s requires %s environment variable to be set", env, requiredVar)
	if logger.Logger != nil {
		logger.Logger.Fatal(msg)
	} else {
		fmt.Fprintf(os.Stderr, "FATAL: %s\n", msg)
		os.Exit(1)
	}
}
