package database

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"

	"github.com/spf13/viper"
)

// GetDatabaseDSN returns the explicitly configured database connection string.
// Used by CLI commands (migrate, seed, cleanup) which run as the postgres superuser.
//
// Selection rule:
// 1. APP_ENV=test requires TEST_DB_DSN and never falls through to DB_DSN.
// 2. Every other environment requires DB_DSN.
//
// Missing config is fatal by design; do not add localhost fallbacks here.
func GetDatabaseDSN() string {
	dsn, err := resolveDatabaseDSN()
	if err == nil {
		return dsn
	}

	slog.Error(err.Error())
	os.Exit(1)
	return ""
}

func resolveDatabaseDSN() (string, error) {
	appEnv := viper.GetString("app_env")
	if appEnv == "test" {
		if testDSN := viper.GetString("test_db_dsn"); testDSN != "" {
			return testDSN, nil
		}
		return "", fmt.Errorf("APP_ENV=test requires TEST_DB_DSN environment variable")
	}

	if dsn := viper.GetString("db_dsn"); dsn != "" {
		return dsn, nil
	}

	return "", fmt.Errorf("DB_DSN environment variable is required")
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
