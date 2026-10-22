package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConnector_InvalidDSNFailsWithoutPanicOrCredentials(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "unresolved reference",
			dsn:  "postgres://postgres:${POSTGRES_PASSWORD}@localhost:5432/postgres?sslmode=disable",
			want: "invalid database DSN: it contains an unresolved ${...} reference",
		},
		{
			name: "unsupported scheme",
			dsn:  "mysql://postgres:secret-value@localhost:5432/postgres",
			want: "invalid database DSN",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			connector, err := newConnector(test.dsn)

			require.EqualError(t, err, test.want)
			assert.Nil(t, connector)
		})
	}
}

func TestNewConnector_ValidDSN(t *testing.T) {
	t.Parallel()

	connector, err := newConnector("postgres://postgres:secret-value@localhost:5432/postgres?sslmode=disable")

	require.NoError(t, err)
	assert.NotNil(t, connector)
}
