package compose

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// visitorTraceTable is the first table a count of visitorTraceCounts reads.
var visitorTraceTable = regexp.MustCompile(`(?i)\bFROM\s+([a-z_]+\.[a-z_]+)`)

// rowsHoldingTheVisitor counts, per table of the schema, the rows whose text
// holds the visitor's last name: every column, JSON included. The database
// holds one school's traces only.
func rowsHoldingTheVisitor(t *testing.T, db *bun.DB) map[string]int {
	t.Helper()
	ctx := t.Context()
	var tables []string
	require.NoError(t, db.NewRaw(`
		SELECT format('%I.%I', n.nspname, c.relname)
		FROM pg_class AS c JOIN pg_namespace AS n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p') AND NOT c.relispartition
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')`).Scan(ctx, &tables))
	require.NotEmpty(t, tables, "schema discovery must not pass vacuously")
	rows := map[string]int{}
	for _, table := range tables {
		var count int
		require.NoError(t, db.NewRaw(fmt.Sprintf(`SELECT count(*) FROM %s AS row WHERE row::text LIKE ?`, table),
			"%"+visitorLast+"%").Scan(ctx, &count), table)
		if count > 0 {
			rows[table] = count
		}
	}
	return rows
}

// visitorTraceCounts is no free list: every row leaveVisitorTraces leaves
// with the visitor's name is counted by one of its entries. Removing an entry,
// or a new place in leaveVisitorTraces without one, fails here, so the
// erasure tests cannot lose a place silently (#6015).
func TestVisitorTraceCountsCountEveryRowHoldingTheVisitor(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	accounts := leaveVisitorTraces(t, db, schoolID)

	counted := map[string]int{}
	for place, query := range visitorTraceCounts {
		match := visitorTraceTable.FindStringSubmatch(query)
		require.NotNil(t, match, "the count of %s reads no table", place)
		var count int
		require.NoError(t, db.NewRaw(query, schoolID).Scan(t.Context(), &count), place)
		require.Equal(t, 1, count, "leaveVisitorTraces leaves one row for %s", place)
		counted[match[1]] += count
	}
	held := rowsHoldingTheVisitor(t, db)
	require.NotEmpty(t, held, "the visitor's name must be found somewhere; the scan would pass vacuously")
	require.Equal(t, held, counted,
		"rows holding the visitor's name per table (left) against the rows visitorTraceCounts counts (right): give every place a count")

	// The erasure leaves the name in no row of the database.
	queue, err := NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := NewDemoSchools(db)
	require.NoError(t, err)
	slug := orderDemoSchool(t, db, "OGS Nord")
	_, err = queue.ClaimDemoSchoolOrder(t.Context())
	require.NoError(t, err)
	require.NoError(t, schools.RememberDemoSchool(t.Context(), slug, organizationtenancy.DemoSchoolState{SchoolID: schoolID, SeedJSON: []byte(`{}`)}))
	require.NoError(t, queue.FinishDemoSchoolOrder(t.Context(), slug, accounts.caregiver, accounts.parent))
	_, erased := retireAtLeast(t, db, queue, []string{slug}, 0, 1)
	require.Equal(t, 1, erased)
	require.Empty(t, rowsHoldingTheVisitor(t, db), "rows still holding the visitor's name after the erasure")
}
