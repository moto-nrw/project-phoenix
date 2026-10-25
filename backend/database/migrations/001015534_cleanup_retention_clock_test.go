package migrations

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupRetentionUsesElapsedHoursAcrossClockChanges(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	serve := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, serve.Close()) })
	ctx := t.Context()
	school := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, school)

	functions := append([]string{"documents.delete_removed_files"}, finishedCleanupFunctions...)
	for _, function := range functions {
		var definition string
		require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef((? || '(bigint, timestamptz)')::regprocedure)`, function).Scan(ctx, &definition))
		require.Contains(t, definition, "clock_timestamp()")
		for _, instant := range []string{"2026-03-29T12:00:00+02:00", "2026-10-25T12:00:00+01:00"} {
			now, err := time.Parse(time.RFC3339, instant)
			require.NoError(t, err)
			// Freeze only the clock in this isolated database; keep the installed
			// function's authorization, cutoff calculation and configuration.
			frozen := strings.ReplaceAll(definition, "clock_timestamp()", fmt.Sprintf("TIMESTAMPTZ '%s'", instant))
			_, err = db.ExecContext(ctx, frozen)
			require.NoError(t, err)
			for _, zone := range []string{"Europe/Berlin", "UTC"} {
				t.Run(function+"/"+instant+"/"+zone, func(t *testing.T) {
					call := func(cutoff time.Time) error {
						return withPhoenixTenantTx(t, serve, school, func(ctx context.Context, tx testpkg.Tx) error {
							if _, err := tx.NewRaw(`SELECT set_config('TimeZone', ?, true)`, zone).Exec(ctx); err != nil {
								return err
							}
							var deleted int64
							return tx.NewRaw("SELECT "+function+"(?, ?)", school, cutoff).Scan(ctx, &deleted)
						})
					}
					assert.NoError(t, call(now.Add(-720*time.Hour)), "the worker's fixed 30-day window must be accepted")
					assert.ErrorContains(t, call(now.Add(-719*time.Hour)), "cutoff exceeds retention window", "a newer cutoff must still be rejected")
				})
			}
		}
	}
}

func TestCleanupRetentionClockMigrationRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	functions := append([]string{"documents.delete_removed_files"}, finishedCleanupFunctions...)
	definition := func(function string) string {
		var result string
		require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef((? || '(bigint, timestamptz)')::regprocedure)`, function).Scan(ctx, &result))
		return result
	}
	installed := make(map[string]string, len(functions))
	for _, function := range functions {
		installed[function] = definition(function)
		require.Contains(t, installed[function], "TimeZone")
	}

	require.NoError(t, cleanupRetentionClockDown(ctx, db))
	for _, function := range functions {
		assert.NotContains(t, definition(function), "TimeZone", "rollback restores the session timezone")
	}
	require.NoError(t, cleanupRetentionClockUp(ctx, db))
	for _, function := range functions {
		assert.Equal(t, installed[function], definition(function), "round trip preserves the function and its security configuration")
	}
}
