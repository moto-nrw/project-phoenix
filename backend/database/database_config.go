package database

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
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
	return resolveDatabaseDSNFrom(os.Getenv)
}

func resolveDatabaseDSNFrom(getenv func(string) string) (string, error) {
	appEnv := getenv("APP_ENV")
	if appEnv == "test" {
		if testDSN := getenv("TEST_DB_DSN"); testDSN != "" {
			return testDSN, nil
		}
		return "", fmt.Errorf("APP_ENV=test requires TEST_DB_DSN environment variable")
	}

	if dsn := getenv("DB_DSN"); dsn != "" {
		return dsn, nil
	}

	return "", fmt.Errorf("DB_DSN environment variable is required")
}

// GetServeDSN returns the database connection string for the HTTP server.
// Connects as the phoenix_auth role (NOINHERIT, can SET ROLE to phoenix_tenant/phoenix_admin)
// instead of the postgres superuser. This enforces least-privilege at the connection level.
//
// Deployed services supply a credential-free DB_DSN endpoint, not the privileged
// migration DSN. PHOENIX_AUTH_PASSWORD is mandatory.
// Run migration V1.14.1 to create the phoenix_auth role, then set the password
// in your env file (dev.env for local, .env for Docker).
func GetServeDSN() string {
	dsn, err := resolveServeDSNFrom(os.Getenv)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	return dsn
}

func resolveServeDSNFrom(getenv func(string) string) (string, error) {
	baseDSN, err := resolveDatabaseDSNFrom(getenv)
	if err != nil {
		return "", err
	}

	password := getenv("PHOENIX_AUTH_PASSWORD")
	if password == "" {
		return "", fmt.Errorf("PHOENIX_AUTH_PASSWORD is required for serve")
	}

	parsed, err := url.Parse(baseDSN)
	if err != nil {
		// URL parser errors can include credentials from the original input.
		return "", fmt.Errorf("invalid database DSN for serve")
	}

	parsed.User = url.UserPassword("phoenix_auth", password)
	return parsed.String(), nil
}
