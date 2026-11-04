package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDevtoolsRootCmd_HasCommands(t *testing.T) {
	t.Parallel()
	names := make([]string, 0, len(RootCmd.Commands()))
	for _, command := range RootCmd.Commands() {
		names = append(names, command.Name())
	}
	assert.ElementsMatch(t, []string{"demo", "seed", "simulate"}, names)
	assert.NotNil(t, RootCmd.PersistentFlags().Lookup("config"))
	assert.NotNil(t, RootCmd.PersistentFlags().Lookup("demo-standing-school"))
}

func TestDevtoolsPropagatesDatabaseRuntimeConfig(t *testing.T) {
	t.Parallel()
	config := map[string]string{
		"app_env":               "development",
		"demo_db_dsn":           "postgres://phoenix_demo@localhost:5432/postgres?sslmode=disable",
		"phoenix_demo_password": "demo-password",
		"db_dsn":                "postgres://postgres@localhost:5432/postgres?sslmode=disable",
	}
	effective := make(map[string]string)

	propagateDatabaseConfigFrom(func(key string) string { return config[key] }, func(key, value string) error {
		effective[key] = value
		return nil
	})

	assert.Equal(t, "development", effective["APP_ENV"])
	assert.Equal(t, "postgres://postgres@localhost:5432/postgres?sslmode=disable", effective["DB_DSN"])
	assert.Equal(t, "postgres://phoenix_demo@localhost:5432/postgres?sslmode=disable", effective["DEMO_DB_DSN"])
	assert.Equal(t, "demo-password", effective["PHOENIX_DEMO_PASSWORD"])
}

func TestDevtoolsSecureRandomSourceFillsBuffer(t *testing.T) {
	t.Parallel()
	buffer := make([]byte, 32)
	n, err := secureRandomSource().Read(buffer)
	assert.NoError(t, err)
	assert.Equal(t, len(buffer), n)
	assert.NotEqual(t, make([]byte, 32), buffer)
}
