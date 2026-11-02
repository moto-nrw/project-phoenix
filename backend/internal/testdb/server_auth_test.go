package testdb

import (
	"context"
	"errors"
	"net"
	"os"
	"syscall"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Only injected probes run here: no connection, Docker command or lease can
// reach a shared server, even when the inherited TEST_DB_DSN points at one.
func TestEnsureServerRejectsWrongCredentialsWithoutMutation(t *testing.T) {
	t.Parallel()
	for _, initial := range []string{"authentication failure", "starting", "connection refused"} {
		t.Run(initial, func(t *testing.T) {
			t.Parallel()
			cfg, err := NewConfig("postgres://postgres:wrong@localhost:6543/phoenix_test?sslmode=disable")
			require.NoError(t, err)
			authErr := errors.New("authentication probe")
			starts, pings := 0, 0
			err = ensureServerWithDependencies(context.Background(), cfg,
				func(context.Context) error { starts++; return nil },
				func(context.Context, *Config) error {
					pings++
					if pings == 1 && initial == "connection refused" {
						return syscall.ECONNREFUSED
					}
					if pings == 1 && initial == "starting" {
						return &net.OpError{Op: "read", Err: os.ErrDeadlineExceeded}
					}
					return authErr
				},
				func(err error) bool { return errors.Is(err, authErr) },
			)
			require.ErrorContains(t, err, "test database authentication failed")
			assert.Contains(t, err.Error(), "root .env")
			assert.Contains(t, err.Error(), "docs/agents/operations.md")
			assert.NotContains(t, err.Error(), "postgres://")
			if initial == "connection refused" {
				assert.Equal(t, 1, starts)
			} else {
				assert.Zero(t, starts)
			}
		})
	}
}

func TestVerifyAuthRolePasswordNeverMutatesExistingRole(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"missing", "valid", "wrong password"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(state != "missing"))
			cfg, err := NewConfig("postgres://postgres@example.invalid:6543/phoenix_test?sslmode=disable")
			require.NoError(t, err)
			pings := 0
			err = verifyAuthRolePassword(context.Background(), cfg, db, func(_ context.Context, roleCfg *Config) error {
				pings++
				assert.Equal(t, "phoenix_auth", roleCfg.templateURL.User.Username())
				assert.Equal(t, cfg.templateURL.Host, roleCfg.templateURL.Host)
				if state == "wrong password" {
					return errors.New("private driver diagnostic")
				}
				return nil
			})
			if state == "wrong password" {
				require.ErrorContains(t, err, "existing role was not changed")
				assert.NotContains(t, err.Error(), "private driver diagnostic")
			} else {
				require.NoError(t, err)
			}
			if state == "missing" {
				assert.Zero(t, pings)
			} else {
				assert.Equal(t, 1, pings)
			}
			require.NoError(t, mock.ExpectationsWereMet())
			assert.Equal(t, "postgres", cfg.templateURL.User.Username(), "caller config stays unchanged")
		})
	}
}
