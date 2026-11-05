package migrations

import (
	"context"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

var germanNameAlter = regexp.MustCompile(`ALTER TABLE (\w+)\.(\w+) ALTER COLUMN (\w+) TYPE`)

// germanNameCollations reads the collation of every column the up migration
// alters.
func germanNameCollations(t *testing.T, ctx context.Context, db *bun.DB) map[string]string {
	t.Helper()
	matches := germanNameAlter.FindAllStringSubmatch(germanNameCollationUpSQL, -1)
	require.Len(t, matches, 29)
	got := make(map[string]string, len(matches))
	for _, m := range matches {
		var collation string
		require.NoError(t, db.NewRaw(`SELECT coalesce(collation_name, 'default') FROM information_schema.columns
  WHERE table_schema = ? AND table_name = ? AND column_name = ?`, m[1], m[2], m[3]).Scan(ctx, &collation))
		got[m[1]+"."+m[2]+"."+m[3]] = collation
	}
	return got
}

// TestGermanNameCollationSortsNamesInDictionaryOrder pins #4949: the name
// columns sort in German dictionary order whatever the database's libc
// locale does, so an accent, an umlaut or a lowercase particle no longer
// lands behind Z. Equality stays byte-exact, and the down migration
// restores the default collation.
func TestGermanNameCollationSortsNamesInDictionaryOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

	for column, collation := range germanNameCollations(t, ctx, db) {
		assert.Equal(t, "german_name", collation, column)
	}

	for _, last := range []string{"Zimmermann", "Özdemir", "von Bergen", "adler", "Becker", "Ábel", "Oswald", "Raum 10", "Raum 2"} {
		testpkg.CreateTestPerson(t, db, "Kim", last)
	}
	var sorted []string
	require.NoError(t, db.NewRaw(`SELECT last_name FROM users.persons WHERE tenant_id = ? ORDER BY last_name`,
		testpkg.Tenant(t)).Scan(ctx, &sorted))
	assert.Equal(t, []string{"Ábel", "adler", "Becker", "Oswald", "Özdemir", "Raum 2", "Raum 10", "von Bergen", "Zimmermann"}, sorted)

	var equal, insensitive bool
	require.NoError(t, db.NewRaw(`SELECT last_name = 'adler', last_name ILIKE 'ADLER' FROM users.persons
  WHERE tenant_id = ? AND last_name = 'adler'`, testpkg.Tenant(t)).Scan(ctx, &equal, &insensitive))
	assert.True(t, equal)
	assert.True(t, insensitive, "ILIKE keeps working on the collation")
	var upper int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.persons WHERE tenant_id = ? AND last_name = 'Adler'`,
		testpkg.Tenant(t)).Scan(ctx, &upper))
	assert.Zero(t, upper, "equality stays case-sensitive")

	relfilenode := func() int64 {
		var node int64
		require.NoError(t, db.NewRaw(`SELECT relfilenode FROM pg_class WHERE oid = 'users.guardian_profiles'::regclass`).Scan(ctx, &node))
		return node
	}
	before := relfilenode()
	require.NoError(t, germanNameCollationDown(ctx, db))
	for column, collation := range germanNameCollations(t, ctx, db) {
		assert.Equal(t, "default", collation, column)
	}
	require.NoError(t, germanNameCollationUp(ctx, db))
	assert.Equal(t, before, relfilenode(), "changing the collation does not rewrite the table")
	for column, collation := range germanNameCollations(t, ctx, db) {
		assert.Equal(t, "german_name", collation, column)
	}
}
