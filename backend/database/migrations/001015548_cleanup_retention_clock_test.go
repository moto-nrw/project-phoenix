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

// retentionClockGuard is one retention function of 1.15.548 with the window
// its owner applies: days times 24 hours, or, without days, the same UTC clock
// time three years earlier.
type retentionClockGuard struct {
	function string
	// arguments is the function's signature for regprocedure.
	arguments string
	// call runs the function for the tenant ? up to the cutoff ?.
	call string
	days int
}

func (g retentionClockGuard) procedure() string { return g.function + g.arguments }

func retentionClockGuards() []retentionClockGuard {
	fixedDays := func(function string, days int) retentionClockGuard {
		return retentionClockGuard{
			function: function, arguments: "(bigint, timestamptz)",
			call: "SELECT " + function + "(?, ?)", days: days,
		}
	}
	guards := []retentionClockGuard{fixedDays("documents.delete_removed_files", 30)}
	for _, function := range finishedCleanupFunctions {
		guards = append(guards, fixedDays(function, 30))
	}
	return append(guards,
		fixedDays("audit.delete_expired_unregistered_tag_scans", 90),
		fixedDays("audit.delete_expired_auth_events", 90),
		retentionClockGuard{
			function: "audit.delete_expired_data_deletions", arguments: "(bigint, timestamptz)",
			call: "SELECT audit.delete_expired_data_deletions(?, ?)",
		},
		retentionClockGuard{
			function: "audit.delete_expired_protocol", arguments: "(bigint, text, timestamptz)",
			call: "SELECT audit.delete_expired_protocol(?, 'student_deletion_log_retention', ?)",
		},
	)
}

// retentionClockInstants are database clocks right after both clock changes
// in Berlin, where a day lasts 23 or 25 hours and three years back lands in
// the other offset, and on a leap day. threeYearsEarlier is the window
// boundary PostgreSQL and the Audit Platform both compute in UTC.
var retentionClockInstants = []struct{ at, threeYearsEarlier string }{
	{"2026-03-29T12:00:00+02:00", "2023-03-29T10:00:00Z"},
	{"2026-10-25T12:00:00+01:00", "2023-10-25T11:00:00Z"},
	{"2027-03-29T12:00:00+02:00", "2024-03-29T10:00:00Z"},
	{"2028-02-29T12:00:00+01:00", "2025-02-28T11:00:00Z"},
}

func TestCleanupRetentionUsesElapsedHoursAcrossClockChanges(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	serve := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, serve.Close()) })
	ctx := t.Context()
	school := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, school)

	for _, guard := range retentionClockGuards() {
		var definition string
		require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef(?::regprocedure)`, guard.procedure()).Scan(ctx, &definition))
		require.Contains(t, definition, "clock_timestamp()")
		for _, instant := range retentionClockInstants {
			now, err := time.Parse(time.RFC3339, instant.at)
			require.NoError(t, err)
			limit := now.Add(-time.Duration(guard.days) * 24 * time.Hour)
			if guard.days == 0 {
				limit, err = time.Parse(time.RFC3339, instant.threeYearsEarlier)
				require.NoError(t, err)
			}
			// Freeze only the clock in this isolated database; keep the installed
			// function's authorization, cutoff calculation and configuration.
			frozen := strings.ReplaceAll(definition, "clock_timestamp()", fmt.Sprintf("TIMESTAMPTZ '%s'", instant.at))
			_, err = db.ExecContext(ctx, frozen)
			require.NoError(t, err)
			for _, zone := range []string{"Europe/Berlin", "UTC"} {
				t.Run(guard.function+"/"+instant.at+"/"+zone, func(t *testing.T) {
					call := func(cutoff time.Time) error {
						return withPhoenixTenantTx(t, serve, school, func(ctx context.Context, tx testpkg.Tx) error {
							if _, err := tx.NewRaw(`SELECT set_config('TimeZone', ?, true)`, zone).Exec(ctx); err != nil {
								return err
							}
							var deleted int64
							return tx.NewRaw(guard.call, school, cutoff).Scan(ctx, &deleted)
						})
					}
					assert.NoError(t, call(limit), "the owner's fixed window must be accepted")
					assert.ErrorContains(t, call(limit.Add(time.Minute)), "cutoff exceeds retention window", "a newer cutoff must still be rejected")
				})
			}
		}
	}
}

func TestCleanupRetentionClockMigrationRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	guards := retentionClockGuards()
	definition := func(guard retentionClockGuard) string {
		var result string
		require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef(?::regprocedure)`, guard.procedure()).Scan(ctx, &result))
		return result
	}
	installed := make(map[string]string, len(guards))
	for _, guard := range guards {
		installed[guard.function] = definition(guard)
		require.Contains(t, installed[guard.function], "TimeZone")
	}

	require.NoError(t, cleanupRetentionClockDown(ctx, db))
	for _, guard := range guards {
		assert.NotContains(t, definition(guard), "TimeZone", "rollback restores the session timezone")
	}
	require.NoError(t, cleanupRetentionClockUp(ctx, db))
	for _, guard := range guards {
		assert.Equal(t, installed[guard.function], definition(guard), "round trip preserves the function and its security configuration")
	}
}
