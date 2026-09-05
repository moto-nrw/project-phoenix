package database

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeDSNCredentialFreeEndpoint(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"disable", "require"} {
		t.Run(mode, func(t *testing.T) {
			env := testEnvironment{
				"DB_DSN":                "postgres://phoenix_auth@postgres:5432/postgres?sslmode=" + mode,
				"PHOENIX_AUTH_PASSWORD": "fixture:p@ss/word?#%",
			}
			dsn, err := resolveServeDSNFrom(env.getenv)
			require.NoError(t, err)
			parsed, err := url.Parse(dsn)
			require.NoError(t, err)
			assert.Equal(t, "phoenix_auth", parsed.User.Username())
			password, present := parsed.User.Password()
			assert.True(t, present)
			assert.Equal(t, env["PHOENIX_AUTH_PASSWORD"], password)
			assert.Equal(t, "postgres:5432", parsed.Host)
			assert.Equal(t, "/postgres", parsed.Path)
			assert.Equal(t, mode, parsed.Query().Get("sslmode"))
		})
	}
}

func TestServeDSNMissingAndInvalidConfig(t *testing.T) {
	t.Parallel()
	for _, env := range []testEnvironment{
		{},
		{"DB_DSN": "postgres://phoenix_auth@postgres/postgres"},
		{"APP_ENV": "test", "DB_DSN": "postgres://postgres:fixture-secret@postgres/postgres", "PHOENIX_AUTH_PASSWORD": "fixture"},
		{"DB_DSN": "postgres://postgres:fixture-secret@host:invalid/db", "PHOENIX_AUTH_PASSWORD": "fixture"},
	} {
		dsn, err := resolveServeDSNFrom(env.getenv)
		require.Error(t, err)
		assert.Empty(t, dsn)
		assert.NotContains(t, err.Error(), "fixture-secret")
	}
}
