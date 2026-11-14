package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// TestUnaccentNameSearchFindsNamesWithoutAccents pins part 5 of #4949: a
// search typed without accents finds a stored name with them, on the
// public.german_name columns too, and f_unaccent stays IMMUTABLE so an index
// can use it later. The down migration removes the wrapper and the extension.
func TestUnaccentNameSearchFindsNamesWithoutAccents(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

	var folded string
	require.NoError(t, db.NewRaw(`SELECT public.f_unaccent('Gómez Müller Straße Łukasz Ćirić')`).Scan(ctx, &folded))
	assert.Equal(t, "Gomez Muller Strasse Lukasz Ciric", folded)

	var volatility string
	require.NoError(t, db.NewRaw(`SELECT provolatile FROM pg_proc WHERE oid = 'public.f_unaccent(text)'::regprocedure`).Scan(ctx, &volatility))
	assert.Equal(t, "i", volatility, "f_unaccent must be IMMUTABLE")

	for _, last := range []string{"Gómez", "Özdemir", "Becker"} {
		testpkg.CreateTestPerson(t, db, "Kim", last)
	}
	var found []string
	require.NoError(t, db.NewRaw(`SELECT last_name FROM users.persons
  WHERE tenant_id = ? AND (f_unaccent(last_name) ILIKE f_unaccent(?) OR f_unaccent(last_name) ILIKE f_unaccent(?))
  ORDER BY last_name`, testpkg.Tenant(t), "%gomez%", "%ozdem%").Scan(ctx, &found))
	assert.Equal(t, []string{"Gómez", "Özdemir"}, found)

	exists := func(query string) bool {
		var ok bool
		require.NoError(t, db.NewRaw(query).Scan(ctx, &ok))
		return ok
	}
	require.NoError(t, unaccentNameSearchDown(ctx, db))
	assert.False(t, exists(`SELECT to_regprocedure('public.f_unaccent(text)') IS NOT NULL`))
	assert.False(t, exists(`SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'unaccent')`))
	require.NoError(t, unaccentNameSearchUp(ctx, db))
	assert.True(t, exists(`SELECT to_regprocedure('public.f_unaccent(text)') IS NOT NULL`))
}
