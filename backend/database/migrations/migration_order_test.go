package migrations

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// The order check (ADR 0223). Timestamp names let two branches add migrations
// without agreeing on a number, so a migration can reach a database that has
// already run later ones: bun applies whatever is missing. A fresh database
// runs the same migrations in name order. The check proves that both orders
// end in the same schema.
//
// lateMigrationsEnv names the migrations a pull request adds below the newest
// one of its base, as bun names separated by commas.
// scripts/check-migration-versions.mjs --late computes it; CI and
// scripts/test-changed.sh export it. It is empty for most pull requests, and
// the check then has nothing to replay.
// TestSchemaFingerprintSeesWhatAMigrationCanChange and the tests beside it
// keep the parts of the check working in between.
const lateMigrationsEnv = "PHX_LATE_MIGRATIONS"

// TestLateMigrationsYieldTheFreshSchema replays the whole history twice on its
// own clone, once in name order and once with the late migrations held back
// to the end, the order a deployed database gets them in. Both replays start
// from the same reset, so a difference comes from the order alone.
func TestLateMigrationsYieldTheFreshSchema(t *testing.T) {
	t.Parallel()
	late := splitNames(os.Getenv(lateMigrationsEnv))
	if len(late) == 0 {
		// Nothing runs out of name order, so every database gets the order
		// the template of this test run was built in.
		return
	}
	if differences := replayedSchemaDifferences(t, late); len(differences) > 0 {
		t.Fatalf("the schema depends on the order: a database that runs %s after the later migrations of the "+
			"base ends up different from a fresh one. %s\n%s", strings.Join(late, ", "), renameAdvice, strings.Join(differences, "\n"))
	}
}

const renameAdvice = "Rename the late migration to a current timestamp, so it runs last everywhere."

// replayedSchemaDifferences compares the schema of a replay in name order with
// one that applies late last.
func replayedSchemaDifferences(t *testing.T, late []string) []string {
	t.Helper()
	// The role migrations refuse to run without their password. The roles
	// exist on the test server, so the value is never applied; a parallel
	// test may not set it, so the caller of the test run does.
	require.NotEmpty(t, os.Getenv("PHOENIX_AUTH_PASSWORD"),
		"the replay runs the role migrations: set PHOENIX_AUTH_PASSWORD, as scripts/test-changed.sh and CI do")
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	require.NoError(t, replay(ctx, db, nil), "replay the migrations in name order")
	fresh, err := schemaFingerprint(ctx, db)
	require.NoError(t, err)

	if err := replay(ctx, db, late); err != nil {
		t.Fatalf("the migrations do not run with %s after the later migrations of the base. %s\n%v",
			strings.Join(late, ", "), renameAdvice, err)
	}
	replayed, err := schemaFingerprint(ctx, db)
	require.NoError(t, err)
	return schemaDifferences(fresh, replayed)
}

// replay resets the database and migrates it again: everything but the late
// migrations first, then the rest, as a deployed database gets them.
func replay(ctx context.Context, db *bun.DB, late []string) error {
	logger := slog.New(slog.DiscardHandler)
	if err := resetDatabase(ctx, db, logger); err != nil {
		return err
	}
	early, err := migrationsWithout(Migrations, late)
	if err != nil {
		return err
	}
	if err := migrateSet(ctx, db, early, newRunnerLog(logger)); err != nil {
		return fmt.Errorf("migrate without %v: %w", late, err)
	}
	return migrateWithLogger(ctx, db, logger)
}

// migrationsWithout copies set without the named migrations. A name the set
// does not know is an error: a stale list would otherwise prove nothing.
func migrationsWithout(set *migrate.Migrations, names []string) (*migrate.Migrations, error) {
	held := make(map[string]bool, len(names))
	for _, name := range names {
		held[name] = true
	}
	rest := migrate.NewMigrations()
	for _, migration := range set.Sorted() {
		if held[migration.Name] {
			delete(held, migration.Name)
			continue
		}
		rest.Add(migration)
	}
	for name := range held {
		return nil, fmt.Errorf("no migration is named %s", name)
	}
	return rest, nil
}

func splitNames(list string) []string {
	var names []string
	for _, name := range strings.Split(list, ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func TestMigrationsWithoutHoldsBackOnlyTheNamedMigrations(t *testing.T) {
	t.Parallel()
	all := Migrations.Sorted()
	last := all[len(all)-1].Name

	rest, err := migrationsWithout(Migrations, []string{last})
	require.NoError(t, err)
	require.Len(t, rest.Sorted(), len(all)-1)
	for _, migration := range rest.Sorted() {
		require.NotEqual(t, last, migration.Name)
	}

	_, err = migrationsWithout(Migrations, []string{"19990101000000"})
	require.ErrorContains(t, err, "no migration is named 19990101000000")
}

func TestSchemaDifferencesNameEveryChangedObject(t *testing.T) {
	t.Parallel()
	fresh := map[string]string{"column a.t.x": "position=1 integer", "column a.t.y": "position=2 text", "index a.i": "CREATE INDEX"}
	replayed := map[string]string{"column a.t.x": "position=2 integer", "column a.t.y": "position=1 text", "trigger a.t.g": "CREATE TRIGGER"}

	require.Empty(t, schemaDifferences(fresh, fresh))
	require.Equal(t, []string{
		"column a.t.x:\n  fresh:    position=1 integer\n  replayed: position=2 integer",
		"column a.t.y:\n  fresh:    position=2 text\n  replayed: position=1 text",
		"index a.i: only in the fresh schema",
		"trigger a.t.g: only in the replayed schema",
	}, schemaDifferences(fresh, replayed))
}

// TestSchemaFingerprintSeesWhatAMigrationCanChange runs every fingerprint
// query against the real schema on every test run, so a query that no longer
// works fails here and not in the one pull request that needs the order check.
func TestSchemaFingerprintSeesWhatAMigrationCanChange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	before, err := schemaFingerprint(ctx, db)
	require.NoError(t, err)
	mustExec(t, db, `ALTER TABLE config.setting_values ADD COLUMN fingerprint_probe INTEGER DEFAULT 7`)
	mustExec(t, db, `CREATE INDEX fingerprint_probe_idx ON config.setting_values (fingerprint_probe)`)
	mustExec(t, db, `CREATE FUNCTION platform.fingerprint_probe() RETURNS INTEGER LANGUAGE sql AS 'SELECT 1'`)
	mustExec(t, db, `GRANT EXECUTE ON FUNCTION platform.fingerprint_probe() TO phoenix_tenant`)
	mustExec(t, db, `CREATE POLICY fingerprint_probe ON config.setting_values FOR SELECT TO phoenix_tenant USING (fingerprint_probe = 7)`)
	after, err := schemaFingerprint(ctx, db)
	require.NoError(t, err)

	changed := make(map[string]bool)
	for _, difference := range schemaDifferences(before, after) {
		changed[strings.SplitN(difference, ":", 2)[0]] = true
	}
	require.Equal(t, map[string]bool{
		"column config.setting_values.fingerprint_probe": true,
		"index config.fingerprint_probe_idx":             true,
		"function platform.fingerprint_probe()":          true,
		"policy config.setting_values.fingerprint_probe": true,
	}, changed)
	require.Contains(t, after["function platform.fingerprint_probe()"], "phoenix_tenant=X")
}

// schemaObject is one line of a schema fingerprint: what kind of object, which
// one, and its definition as PostgreSQL prints it.
type schemaObject struct {
	Kind       string `bun:"kind"`
	Identity   string `bun:"identity"`
	Definition string `bun:"definition"`
}

// userSchemas keeps the catalogs of PostgreSQL itself out of a fingerprint.
const userSchemas = `n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'`

// sortedACL prints an ACL without the order its grants were issued in, which
// two migration orders legitimately differ in.
const sortedACL = `COALESCE((SELECT string_agg(item::text, ',' ORDER BY item::text) FROM unnest(%s) AS item), '')`

// schemaFingerprintQueries read everything a migration can define, each as
// (kind, identity, definition). Rows of tables are not part of it: two orders
// may number seed rows differently, and nothing may depend on such a number.
var schemaFingerprintQueries = []string{
	// Schemas and extensions.
	`SELECT 'schema' AS kind, n.nspname::text AS identity,
		pg_get_userbyid(n.nspowner) || ' acl=' || ` + fmt.Sprintf(sortedACL, "n.nspacl") + ` AS definition
	FROM pg_namespace AS n WHERE ` + userSchemas,
	`SELECT 'extension' AS kind, extname::text AS identity, extversion::text AS definition FROM pg_extension`,
	// Tables, views, sequences and composite types: owner, row security,
	// options, grants and the query of a view.
	`SELECT 'relation' AS kind, n.nspname || '.' || c.relname AS identity,
		concat_ws(' ', c.relkind::text, pg_get_userbyid(c.relowner), 'rls=' || c.relrowsecurity::text,
			'force=' || c.relforcerowsecurity::text, 'persistence=' || c.relpersistence::text,
			'options=' || COALESCE(c.reloptions::text, ''), 'bound=' || COALESCE(pg_get_expr(c.relpartbound, c.oid), ''),
			'acl=' || ` + fmt.Sprintf(sortedACL, "c.relacl") + `,
			CASE WHEN c.relkind IN ('v', 'm') THEN 'as ' || pg_get_viewdef(c.oid) END) AS definition
	FROM pg_class AS c JOIN pg_namespace AS n ON n.oid = c.relnamespace
	WHERE c.relkind IN ('r', 'p', 'v', 'm', 'f', 'S', 'c') AND ` + userSchemas,
	// Columns with their position: two migrations that add a column to one
	// table leave them in the order they ran in.
	`SELECT 'column' AS kind, n.nspname || '.' || c.relname || '.' || a.attname AS identity,
		concat_ws(' ', 'position=' || (row_number() OVER (PARTITION BY c.oid ORDER BY a.attnum))::text,
			format_type(a.atttypid, a.atttypmod), CASE WHEN a.attnotnull THEN 'NOT NULL' END,
			'default=' || COALESCE(pg_get_expr(d.adbin, d.adrelid), ''), 'identity=' || a.attidentity::text,
			'generated=' || a.attgenerated::text, 'collation=' || COALESCE(l.collname::text, ''),
			'acl=' || ` + fmt.Sprintf(sortedACL, "a.attacl") + `) AS definition
	FROM pg_attribute AS a
	JOIN pg_class AS c ON c.oid = a.attrelid
	JOIN pg_namespace AS n ON n.oid = c.relnamespace
	LEFT JOIN pg_attrdef AS d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
	LEFT JOIN pg_collation AS l ON l.oid = a.attcollation
	WHERE a.attnum > 0 AND NOT a.attisdropped AND c.relkind IN ('r', 'p', 'v', 'm', 'f', 'c') AND ` + userSchemas,
	`SELECT 'sequence' AS kind, schemaname || '.' || sequencename AS identity,
		concat_ws(' ', data_type::text, start_value::text, min_value::text, max_value::text,
			increment_by::text, cycle::text, cache_size::text) AS definition
	FROM pg_sequences`,
	`SELECT 'constraint' AS kind,
		n.nspname || '.' || COALESCE(c.relname, t.typname) || '.' || o.conname AS identity,
		pg_get_constraintdef(o.oid) || ' validated=' || o.convalidated::text AS definition
	FROM pg_constraint AS o
	JOIN pg_namespace AS n ON n.oid = o.connamespace
	LEFT JOIN pg_class AS c ON c.oid = o.conrelid
	LEFT JOIN pg_type AS t ON t.oid = o.contypid
	WHERE ` + userSchemas,
	`SELECT 'index' AS kind, n.nspname || '.' || c.relname AS identity,
		pg_get_indexdef(i.indexrelid) || ' valid=' || i.indisvalid::text AS definition
	FROM pg_index AS i JOIN pg_class AS c ON c.oid = i.indexrelid JOIN pg_namespace AS n ON n.oid = c.relnamespace
	WHERE ` + userSchemas,
	// Functions and procedures with their bodies; an aggregate has no
	// definition PostgreSQL prints, so its signature stands for it.
	`SELECT 'function' AS kind,
		n.nspname || '.' || p.proname || '(' || pg_get_function_identity_arguments(p.oid) || ')' AS identity,
		concat_ws(' ', pg_get_userbyid(p.proowner), 'acl=' || ` + fmt.Sprintf(sortedACL, "p.proacl") + `,
			CASE WHEN p.prokind = 'a' THEN 'aggregate' ELSE pg_get_functiondef(p.oid) END) AS definition
	FROM pg_proc AS p JOIN pg_namespace AS n ON n.oid = p.pronamespace
	WHERE ` + userSchemas,
	`SELECT 'trigger' AS kind, n.nspname || '.' || c.relname || '.' || t.tgname AS identity,
		pg_get_triggerdef(t.oid) || ' enabled=' || t.tgenabled::text AS definition
	FROM pg_trigger AS t JOIN pg_class AS c ON c.oid = t.tgrelid JOIN pg_namespace AS n ON n.oid = c.relnamespace
	WHERE NOT t.tgisinternal AND ` + userSchemas,
	`SELECT 'policy' AS kind, n.nspname || '.' || c.relname || '.' || p.polname AS identity,
		concat_ws(' ', p.polcmd::text, 'permissive=' || p.polpermissive::text,
			'roles=' || COALESCE((SELECT string_agg(pg_get_userbyid(role), ',' ORDER BY pg_get_userbyid(role)) FROM unnest(p.polroles) AS role), ''),
			'using=' || COALESCE(pg_get_expr(p.polqual, p.polrelid), ''),
			'check=' || COALESCE(pg_get_expr(p.polwithcheck, p.polrelid), '')) AS definition
	FROM pg_policy AS p JOIN pg_class AS c ON c.oid = p.polrelid JOIN pg_namespace AS n ON n.oid = c.relnamespace
	WHERE ` + userSchemas,
	// Enum labels in their order, and domains.
	`SELECT 'type' AS kind, n.nspname || '.' || t.typname AS identity,
		concat_ws(' ', t.typtype::text, format_type(t.typbasetype, t.typtypmod), 'notnull=' || t.typnotnull::text,
			'default=' || COALESCE(t.typdefault, ''),
			'labels=' || COALESCE((SELECT string_agg(e.enumlabel, ',' ORDER BY e.enumsortorder) FROM pg_enum AS e WHERE e.enumtypid = t.oid), '')) AS definition
	FROM pg_type AS t JOIN pg_namespace AS n ON n.oid = t.typnamespace
	WHERE t.typtype IN ('e', 'd', 'r') AND ` + userSchemas,
	`SELECT 'default acl' AS kind,
		pg_get_userbyid(d.defaclrole) || ' in ' || COALESCE(n.nspname::text, '') || ' on ' || d.defaclobjtype::text AS identity,
		` + fmt.Sprintf(sortedACL, "d.defaclacl") + ` AS definition
	FROM pg_default_acl AS d LEFT JOIN pg_namespace AS n ON n.oid = d.defaclnamespace`,
	`SELECT 'comment' AS kind, pg_describe_object(d.classoid, d.objoid, d.objsubid) AS identity, d.description AS definition
	FROM pg_description AS d
	WHERE d.objoid >= 16384 AND d.classoid IN ('pg_class'::regclass, 'pg_proc'::regclass, 'pg_namespace'::regclass,
		'pg_constraint'::regclass, 'pg_trigger'::regclass, 'pg_type'::regclass, 'pg_policy'::regclass)`,
}

// schemaFingerprint reads the schema of db as a map from "kind identity" to
// definition.
func schemaFingerprint(ctx context.Context, db *bun.DB) (map[string]string, error) {
	fingerprint := make(map[string]string)
	for _, query := range schemaFingerprintQueries {
		var objects []schemaObject
		if err := db.NewRaw(query).Scan(ctx, &objects); err != nil {
			return nil, fmt.Errorf("read schema fingerprint: %w", err)
		}
		for _, object := range objects {
			fingerprint[object.Kind+" "+object.Identity] = object.Definition
		}
	}
	return fingerprint, nil
}

// schemaDifferences names every object that only one fingerprint has or that
// the two define differently, sorted.
func schemaDifferences(fresh, replayed map[string]string) []string {
	var differences []string
	for object, definition := range fresh {
		other, exists := replayed[object]
		switch {
		case !exists:
			differences = append(differences, object+": only in the fresh schema")
		case other != definition:
			differences = append(differences, fmt.Sprintf("%s:\n  fresh:    %s\n  replayed: %s", object, definition, other))
		}
	}
	for object := range replayed {
		if _, exists := fresh[object]; !exists {
			differences = append(differences, object+": only in the replayed schema")
		}
	}
	sort.Strings(differences)
	return differences
}
