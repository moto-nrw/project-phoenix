package migrations

// Coverage for the 1.15.260 backfill (issue #2131): every existing school must
// end up with a "Mensa" category so Essenszeiten have a fitting
// Pflichtkategorie, without duplicating one for schools that already have it.

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// countCategoriesNamed returns how many rows of the tenant carry the given
// name, case-insensitively — the same predicate the migration uses.
func countCategoriesNamed(t *testing.T, db *bun.DB, tenantID int64, name string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var count int
	err := db.NewRaw(`
		SELECT COUNT(*) FROM activities.categories
		WHERE tenant_id = ? AND LOWER(name) = LOWER(?);
	`, tenantID, name).Scan(ctx, &count)
	require.NoError(t, err, "count categories named %q", name)
	return count
}

func insertMensaTestCategory(t *testing.T, db *bun.DB, tenantID int64, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewRaw(`
		INSERT INTO activities.categories (tenant_id, name) VALUES (?, ?);
	`, tenantID, name).Exec(ctx)
	require.NoError(t, err, "insert category %q", name)
}

func TestBackfillMensaCategory(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	// A school provisioned via the operator portal: it got the hard-coded
	// default list, which never contained Mensa.
	const withoutMensa int64 = 21311
	// A school that already has one, spelled differently — the migration must
	// not add a second.
	const withMensa int64 = 21312

	testpkg.EnsureTestTenant(t, db, withoutMensa)
	testpkg.EnsureTestTenant(t, db, withMensa)
	defer testpkg.CleanupTenantTestData(t, db, withoutMensa, withMensa)

	insertMensaTestCategory(t, db, withoutMensa, "Sport")
	insertMensaTestCategory(t, db, withMensa, "mensa")

	require.NoError(t, backfillMensaCategoryUp(ctx, db))

	assert.Equal(t, 1, countCategoriesNamed(t, db, withoutMensa, "Mensa"),
		"a school without a Mensa category must get exactly one")
	assert.Equal(t, 1, countCategoriesNamed(t, db, withMensa, "Mensa"),
		"an existing Mensa (any casing) must not be duplicated")

	// Idempotency: re-running must not add a second row anywhere.
	require.NoError(t, backfillMensaCategoryUp(ctx, db))
	assert.Equal(t, 1, countCategoriesNamed(t, db, withoutMensa, "Mensa"))
	assert.Equal(t, 1, countCategoriesNamed(t, db, withMensa, "Mensa"))
}
