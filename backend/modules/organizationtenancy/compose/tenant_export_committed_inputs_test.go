package compose_test

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Real exporter assertions must see committed two-school fixtures and nonempty
// secret sentinels. Otherwise isolation/absence checks could pass vacuously.
// This independent reader never calls the fixture column/row builders or the
// production exporter predicates. The identical test runs on B before C.
func TestTenantExportFixtureHasCommittedInputs(t *testing.T) {
	t.Parallel()
	db, catalog, a, b := exportFixture(t)
	require.NotEmpty(t, catalog.Tables)
	require.Positive(t, a)
	require.Positive(t, b)
	require.NotEqual(t, a, b)
	want := []int64{a, b}
	slices.Sort(want)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	anchor, err := db.Conn(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, anchor.Close()) }()
	reader, err := db.Conn(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, reader.Close()) }()
	var anchorPID, readerPID int
	require.NoError(t, anchor.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &anchorPID))
	require.NoError(t, reader.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &readerPID))
	require.NotEqual(t, anchorPID, readerPID, "the observer needs a separate committed connection")
	err = reader.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, "SET LOCAL ROLE phoenix_tenant_export")
		require.NoError(t, err)
		schemas := map[string]bool{}
		for _, table := range catalog.Tables {
			parts := strings.SplitN(table.Name, ".", 2)
			require.Len(t, parts, 2)
			qualified := exportIdent(parts[0]) + "." + exportIdent(parts[1])
			if !schemas[parts[0]] {
				var canRead bool
				require.NoError(t, tx.NewRaw("SELECT has_schema_privilege('phoenix_admin', ?, 'USAGE')", parts[0]).Scan(ctx, &canRead))
				require.True(t, canRead, "admin schema grant: %s", parts[0])
				schemas[parts[0]] = true
			}
			var adminRead, exportRead bool
			require.NoError(t, tx.NewRaw("SELECT has_table_privilege('phoenix_admin', ?, 'SELECT'), has_table_privilege('phoenix_tenant_export', ?, 'SELECT')", table.Name, table.Name).Scan(ctx, &adminRead, &exportRead))
			require.True(t, adminRead, "admin table grant: %s", table.Name)
			require.True(t, exportRead, "export table grant: %s", table.Name)
			var schools []int64
			require.NoError(t, tx.NewRaw("SELECT fixture_school FROM "+qualified+" ORDER BY fixture_school").Scan(ctx, &schools))
			require.Equal(t, want, schools, "both schools must be visible after fixture return: %s", table.Name)
			var emptyLabels int
			require.NoError(t, tx.NewRaw("SELECT count(*) FROM "+qualified+" WHERE fixture_label IS NULL OR fixture_label = ''").Scan(ctx, &emptyLabels))
			require.Zero(t, emptyLabels, "nonempty original marker: %s", table.Name)
			for column := range table.ExcludeSecretColumns {
				var sentinels int
				ident := exportIdent(column)
				require.NoError(t, tx.NewRaw("SELECT count(*) FROM "+qualified+" WHERE "+ident+" IS NOT NULL AND "+ident+" <> ''").Scan(ctx, &sentinels))
				require.Equal(t, 2, sentinels, "secret exclusion must not pass on empty fixture input: %s.%s", table.Name, column)
			}
		}
		return nil
	})
	require.NoError(t, err)
}
