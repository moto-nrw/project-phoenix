package migrations

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Stored accesses and queued orders keep the name the seeder showed: split at
// the first space, a single word with the child's family name, and the
// standing school's empty name stays empty. The down migration joins them.
func TestDemoVisitorFirstLastNameSplitsTheStoredPersonName(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.NoError(t, demoVisitorFirstLastNameDown(ctx, db))

	accesses := map[string]string{
		"split-two":    "Kim Beispiel",
		"split-many":   "  Anna \t Lena  von Berg ",
		"single-word":  "Kim",
		"particle-two": "Maria von Berg",
	}
	for hash, name := range accesses {
		_, err := db.NewRaw(`INSERT INTO auth.demo_accesses (email, person_name, school_name, token_hash, expires_at)
			VALUES ('kim@ogs-beispiel.de', ?, 'OGS Beispiel', ?, ?)`, name, hash, time.Now().Add(time.Hour)).Exec(ctx)
		require.NoError(t, err)
	}
	orders := map[string]string{"ogs-order-named": "Maria von Berg", "ogs-order-single": "Kim", "ogs-order-unnamed": ""}
	for slug, name := range orders {
		_, err := db.NewRaw(`INSERT INTO platform.demo_school_states (name, school_name, person_name) VALUES (?, 'OGS Beispiel', ?)`,
			slug, name).Exec(ctx)
		require.NoError(t, err)
	}

	require.NoError(t, demoVisitorFirstLastNameUp(ctx, db))

	type names struct {
		Key       string `bun:"key"`
		FirstName string `bun:"first_name"`
		LastName  string `bun:"last_name"`
	}
	var accessNames []names
	require.NoError(t, db.NewRaw(`SELECT token_hash AS key, first_name, last_name FROM auth.demo_accesses ORDER BY token_hash`).Scan(ctx, &accessNames))
	assert.Equal(t, []names{
		{Key: "particle-two", FirstName: "Maria", LastName: "von Berg"},
		{Key: "single-word", FirstName: "Kim", LastName: "Schneider"},
		{Key: "split-many", FirstName: "Anna", LastName: "Lena von Berg"},
		{Key: "split-two", FirstName: "Kim", LastName: "Beispiel"},
	}, accessNames)
	var orderNames []names
	require.NoError(t, db.NewRaw(`SELECT name AS key, first_name, last_name FROM platform.demo_school_states ORDER BY name`).Scan(ctx, &orderNames))
	assert.Equal(t, []names{
		{Key: "ogs-order-named", FirstName: "Maria", LastName: "von Berg"},
		{Key: "ogs-order-single", FirstName: "Kim", LastName: "Schneider"},
		{Key: "ogs-order-unnamed"},
	}, orderNames)

	var personNameColumns int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM information_schema.columns
		WHERE column_name = 'person_name' AND (table_schema, table_name) IN (('auth', 'demo_accesses'), ('platform', 'demo_school_states'))`).
		Scan(ctx, &personNameColumns))
	assert.Zero(t, personNameColumns, "person_name is gone from both tables")
	var canInsertFirst, canInsertLast bool
	require.NoError(t, db.NewRaw(`SELECT has_column_privilege('phoenix_admin', 'platform.demo_school_states', 'first_name', 'INSERT'),
		has_column_privilege('phoenix_admin', 'platform.demo_school_states', 'last_name', 'INSERT')`).Scan(ctx, &canInsertFirst, &canInsertLast))
	assert.True(t, canInsertFirst && canInsertLast, "the serving role queues an order with the visitor's name")
	_, err := db.NewRaw(`INSERT INTO auth.demo_accesses (email, school_name, token_hash, expires_at) VALUES ('x@ogs.de', 'OGS', 'nameless', now())`).Exec(ctx)
	require.Error(t, err, "an access always names its visitor")

	require.NoError(t, demoVisitorFirstLastNameDown(ctx, db))
	var restored string
	require.NoError(t, db.NewRaw(`SELECT person_name FROM auth.demo_accesses WHERE token_hash = 'particle-two'`).Scan(ctx, &restored))
	assert.Equal(t, "Maria von Berg", restored)
	require.NoError(t, db.NewRaw(`SELECT person_name FROM platform.demo_school_states WHERE name = 'ogs-order-unnamed'`).Scan(ctx, &restored))
	assert.Empty(t, restored)
	require.NoError(t, demoVisitorFirstLastNameUp(ctx, db))
}
