package migrations

import (
	"bytes"
	"context"
	"errors"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// studentOwnerCutoverBunName is the name bun knows 1.15.397 under: the numeric
// prefix of the file that registers it.
const studentOwnerCutoverBunName = "001015397"

func TestPendingPreconditionsSelectsPendingMigrationsThatDeclareOne(t *testing.T) {
	t.Parallel()
	checks := pendingPreconditions(migrate.MigrationSlice{
		{Name: studentOwnerCutoverBunName},
	})
	require.Len(t, checks, 1)
	require.Equal(t, studentOwnerCutoverVersion, checks[0].version)
	require.NotNil(t, checks[0].migration.Precondition)
}

// An applied migration's precondition describes a state its own Up has already
// consumed, so asking it again would report a failure about finished work. The
// filtering itself is the migrator's `Unapplied()`; what this pins is that
// preflight feeds on that set. The test database has every migration applied,
// so nothing may be selected.
func TestMigratePreflightAsksNothingWhenEveryMigrationIsApplied(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	output := &bytes.Buffer{}
	require.NoError(t, migratePreflightTo(t.Context(), db, output))
	require.Contains(t, output.String(), "no pending migration declares a data precondition")
}

func TestPendingPreconditionsSkipsMigrationsWithoutOne(t *testing.T) {
	t.Parallel()
	require.Empty(t, pendingPreconditions(migrate.MigrationSlice{
		{Name: "001015394"},             // registered, declares no precondition
		{Name: "001015025"},             // registered with bun only, absent from the registry
		{Name: "000000000000_not_real"}, // unknown to both
	}))
}

func TestMigratePreflightRequiresADatabase(t *testing.T) {
	t.Parallel()
	var db *bun.DB
	require.ErrorContains(t, MigratePreflight(context.Background(), db), "database is required")
}

func TestMigratePreflightReportsEachFailingMigrationOnce(t *testing.T) {
	t.Parallel()
	failing := errors.New("tenant 4: 2 unreconciled guardian values")
	output := &bytes.Buffer{}
	err := reportPreflightChecks(context.Background(), output, []preconditionCheck{
		{version: "1.15.397", migration: &Migration{
			Description:  "Cut over student storage",
			Precondition: func(context.Context, *bun.DB) error { return failing },
		}},
		{version: "1.15.400", migration: &Migration{
			Description:  "Something else",
			Precondition: func(context.Context, *bun.DB) error { return nil },
		}},
	}, nil)

	require.ErrorIs(t, err, failing)
	require.ErrorContains(t, err, "failed for 1 pending migration(s)")
	require.Contains(t, output.String(), "FAILED 1.15.397 - Cut over student storage")
	require.Contains(t, output.String(), "OK 1.15.400 - Something else")
}

func TestMigratePreflightPassesWhenEveryPreconditionHolds(t *testing.T) {
	t.Parallel()
	output := &bytes.Buffer{}
	require.NoError(t, reportPreflightChecks(context.Background(), output, []preconditionCheck{
		{version: "1.15.397", migration: &Migration{
			Description:  "Cut over student storage",
			Precondition: func(context.Context, *bun.DB) error { return nil },
		}},
	}, nil))
	require.Contains(t, output.String(), "1 pending migration(s) checked")
}

func TestMigratePreflightSaysSoWhenNothingDeclaresAPrecondition(t *testing.T) {
	t.Parallel()
	output := &bytes.Buffer{}
	require.NoError(t, reportPreflightChecks(context.Background(), output, nil, nil))
	require.Contains(t, output.String(), "no pending migration declares a data precondition")
}
