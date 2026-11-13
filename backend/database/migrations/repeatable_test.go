package migrations

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestEveryRepeatableFileDefinesTheFunctionItIsNamedAfter holds file name,
// registered name and SQL together: repeatable_<schema>_<name>.go registers
// <schema>.<name> and replaces exactly that function.
func TestEveryRepeatableFileDefinesTheFunctionItIsNamedAfter(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("repeatable_*_*.go")
	require.NoError(t, err)
	registered := make(map[string]bool)
	for _, name := range RepeatableNames() {
		registered[name] = true
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		require.NoError(t, err)
		match := regexp.MustCompile(`RegisterRepeatable\("([a-z_]+)\.([a-z0-9_]+)"`).FindSubmatch(source)
		require.NotNil(t, match, "%s registers no repeatable definition", file)
		schema, object := string(match[1]), string(match[2])
		require.Equal(t, "repeatable_"+schema+"_"+object+".go", file)
		require.Equal(t, 1, strings.Count(string(source), "CREATE OR REPLACE FUNCTION "),
			"%s holds one definition", file)
		require.Contains(t, string(source), "CREATE OR REPLACE FUNCTION "+schema+"."+object+"(")
		delete(registered, schema+"."+object)
	}
	require.Empty(t, registered, "every repeatable definition has its own file")
	require.NotEmpty(t, files)
}

func TestRegisterRepeatableRefusesASecondOrEmptyDefinition(t *testing.T) {
	t.Parallel()
	install := func(context.Context, bun.Tx) error { return nil }
	existing := RepeatableNames()[0]
	require.PanicsWithValue(t, "REPEATABLE DEFINITION COLLISION: "+existing+" is registered twice",
		func() { RegisterRepeatable(existing, install) })
	require.Panics(t, func() { RegisterRepeatable("platform.without_sql", nil) })
	require.Panics(t, func() { RegisterRepeatable("", install) })
}

// TestMigrateRestoresAnOverwrittenRepeatableDefinition is the case of #6391:
// something replaces a function after its definition was installed, here an
// older copy that claims every account is unused. The next migrate run, with
// no migration pending, puts the current definition back.
func TestMigrateRestoresAnOverwrittenRepeatableDefinition(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	definition := func() string {
		var sql string
		require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('auth.account_is_unused(bigint,text)'::regprocedure)`).Scan(ctx, &sql))
		return sql
	}

	current := definition()
	require.Equal(t, strings.TrimSpace(repeatableAuthAccountIsUnusedSQL), strings.TrimSpace(current),
		"a migrated database carries the repeatable definition")
	mustExec(t, db, `CREATE OR REPLACE FUNCTION auth.account_is_unused(target_id bigint, target_email text)
		RETURNS boolean LANGUAGE sql STABLE AS 'SELECT true'`)
	require.NotEqual(t, current, definition())

	logger, _ := captureLogger(t)
	require.NoError(t, migrateWithLogger(ctx, db, logger))
	require.Equal(t, current, definition())
}
