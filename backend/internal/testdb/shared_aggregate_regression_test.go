package testdb

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests use the existing testdb self-test lifecycle, not the application
// fixture graph. Every database is a private fake-template clone. The test
// bodies deliberately work with both the original and the proposed sweep.go.
func aggregateRegressionClone(t *testing.T) (context.Context, *sql.DB, string) {
	t.Helper()
	// Fail instead of silently accepting an unexecuted DB qualification.
	require.False(t, testing.Short(), "aggregate qualification requires the DB")
	loadEnvForIntegration(t)
	require.NotEmpty(t, os.Getenv("TEST_DB_DSN"), "aggregate qualification requires TEST_DB_DSN")
	cfg := integrationConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	builds := 0
	templateCfg, err := EnsureTemplate(ctx, cfg, WithBuild(fakeBuild(&builds)), WithMigrationsHash("aggregate-regression"))
	require.NoError(t, err)
	handle, err := CreateClone(ctx, templateCfg, SanitizeRunID(""))
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, handle.Close())
		dropBareClone(t, cfg, handle.Name)
	})
	db := openSQL(handle.DSN)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })
	return ctx, db, handle.DSN
}

func aggregateRegressionExec(t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any) {
	t.Helper()
	_, err := db.ExecContext(ctx, query, args...)
	require.NoError(t, err)
}

// The oracle reads individual PostgreSQL JSON rows and hashes scalar values.
// Sorting and multiplicity are independent of the production aggregate/query
// builder and ownership resolver. Callers select expected fixture IDs directly.
func aggregateRegressionOracle(t *testing.T, ctx context.Context, db *sql.DB, rowQuery string) sharedTableState {
	t.Helper()
	rows, err := db.QueryContext(ctx, rowQuery)
	require.NoError(t, err)
	defer func() { assert.NoError(t, rows.Close()) }()
	var values []string
	for rows.Next() {
		var raw string
		require.NoError(t, rows.Scan(&raw))
		values = append(values, raw)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	digest := func(value string) string {
		var hash string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT md5($1::text)`, value).Scan(&hash))
		return hash
	}
	hashes := make([]string, 0, len(values))
	for _, value := range values {
		hashes = append(hashes, digest(value))
	}
	sort.Strings(hashes)
	return sharedTableState{Rows: int64(len(values)), Fingerprint: digest(strings.Join(hashes, ","))}
}

func aggregateRegressionBaseline(t *testing.T, ctx context.Context, db *sql.DB, table string) sharedTableState {
	t.Helper()
	var state sharedTableState
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT rows, fingerprint FROM phx_test.shared_baseline WHERE table_name = $1`, table,
	).Scan(&state.Rows, &state.Fingerprint))
	return state
}

func aggregateRegressionDelta(t *testing.T, delta TableDelta, table string, before, after sharedTableState) {
	t.Helper()
	assert.Equal(t, table, delta.Table)
	assert.Equal(t, before.Rows, delta.BaselineRows)
	assert.Equal(t, after.Rows, delta.CloneRows)
	assert.Equal(t, before.Fingerprint, delta.BaselineFingerprint)
	assert.Equal(t, after.Fingerprint, delta.CloneFingerprint)
	assert.NotEqual(t, before.Fingerprint, after.Fingerprint)
}

func TestSharedAggregateRowMutations(t *testing.T) {
	t.Parallel()
	ctx, db, dsn := aggregateRegressionClone(t)
	aggregateRegressionExec(t, ctx, db, `CREATE TABLE public.aggregate_probe (id bigint, payload text)`)
	const rowQuery = `SELECT row_to_json(r)::text FROM public.aggregate_probe AS r`
	cases := []struct {
		name string
		sql  string
		rows int64
	}{
		{"same-count replacement", `DELETE FROM public.aggregate_probe WHERE id = 1; INSERT INTO public.aggregate_probe VALUES (3, 'alpha')`, 2},
		{"update only", `UPDATE public.aggregate_probe SET payload = 'Grüße <&> "quoted"' WHERE id = 1`, 2},
		{"delete only", `DELETE FROM public.aggregate_probe WHERE id = 2`, 1},
		{"insert only", `INSERT INTO public.aggregate_probe VALUES (3, 'new')`, 3},
		{"duplicate identical row", `INSERT INTO public.aggregate_probe SELECT * FROM public.aggregate_probe WHERE id = 1`, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Sequential subtests intentionally reset the same private table.
			aggregateRegressionExec(t, ctx, db, `TRUNCATE public.aggregate_probe; INSERT INTO public.aggregate_probe VALUES (1, 'alpha'), (2, NULL)`)
			before := aggregateRegressionOracle(t, ctx, db, rowQuery)
			require.EqualValues(t, 2, before.Rows)
			require.NoError(t, SnapshotSharedBaseline(ctx, dsn))
			require.Equal(t, before, aggregateRegressionBaseline(t, ctx, db, "public.aggregate_probe"))
			aggregateRegressionExec(t, ctx, db, tc.sql)
			after := aggregateRegressionOracle(t, ctx, db, rowQuery)
			require.Equal(t, tc.rows, after.Rows)
			deltas, err := Leftovers(ctx, dsn)
			require.NoError(t, err)
			require.Len(t, deltas, 1)
			aggregateRegressionDelta(t, deltas[0], "public.aggregate_probe", before, after)
		})
	}
}

func TestSharedAggregateTenantBand(t *testing.T) {
	t.Parallel()
	ctx, db, dsn := aggregateRegressionClone(t)
	aggregateRegressionExec(t, ctx, db, `CREATE TABLE public.scoped (id bigint, tenant_id bigint, payload text)`)
	aggregateRegressionExec(t, ctx, db, `INSERT INTO public.scoped VALUES (1, NULL, 'null'), (2, $1, 'shared'), (3, $2, 'own')`, TenantIDBase-1, TenantIDBase)
	const expectedShared = `SELECT row_to_json(r)::text FROM public.scoped AS r WHERE id IN (1, 2)`
	before := aggregateRegressionOracle(t, ctx, db, expectedShared)
	require.EqualValues(t, 2, before.Rows)
	require.NoError(t, SnapshotSharedBaseline(ctx, dsn))
	require.Equal(t, before, aggregateRegressionBaseline(t, ctx, db, "public.scoped"))
	aggregateRegressionExec(t, ctx, db, `UPDATE public.scoped SET payload = 'own changed' WHERE id = 3`)
	deltas, err := Leftovers(ctx, dsn)
	require.NoError(t, err)
	require.Empty(t, deltas, "the boundary value itself is test-owned")
	aggregateRegressionExec(t, ctx, db, `DELETE FROM public.scoped WHERE id = 3`)
	aggregateRegressionExec(t, ctx, db, `INSERT INTO public.scoped VALUES (4, $1, 'new own')`, TenantIDBase+1)
	deltas, err = Leftovers(ctx, dsn)
	require.NoError(t, err)
	require.Empty(t, deltas, "own delete/insert must not change shared state")
	aggregateRegressionExec(t, ctx, db, `UPDATE public.scoped SET payload = 'shared changed' WHERE id IN (1, 2)`)
	after := aggregateRegressionOracle(t, ctx, db, expectedShared)
	deltas, err = Leftovers(ctx, dsn)
	require.NoError(t, err)
	require.Len(t, deltas, 1)
	aggregateRegressionDelta(t, deltas[0], "public.scoped", before, after)
}

func TestSharedAggregateMappedOwnership(t *testing.T) {
	t.Parallel()
	ctx, db, dsn := aggregateRegressionClone(t)
	aggregateRegressionExec(t, ctx, db, `
		CREATE SCHEMA auth;
		CREATE TABLE auth.accounts (id bigint, payload text);
		CREATE TABLE auth.account_tenants (id bigint, account_id bigint, tenant_id bigint);
		CREATE TABLE auth.password_reset_tokens (id bigint, account_id bigint, payload text);
		CREATE TABLE auth.mfa_email_challenges (id bigint, account_id bigint, tenant_id bigint, payload text);
		CREATE TABLE auth.roles (id bigint, tenant_id bigint);
		CREATE TABLE auth.role_permissions (role_id bigint, payload text);
		CREATE SCHEMA platform;
		CREATE TABLE platform.schools (id bigint, payload text);
		CREATE TABLE platform.organizations (id bigint, payload text)`)
	aggregateRegressionExec(t, ctx, db, `
		INSERT INTO auth.accounts VALUES (10, 'unmapped'), (20, 'own'), (30, 'shared'), (40, 'mixed'), (50, 'null mapping');
		INSERT INTO auth.account_tenants VALUES (1, 20, $1), (2, 30, $2), (3, 40, $1), (4, 40, $2), (5, 50, NULL);
		INSERT INTO auth.password_reset_tokens VALUES (10, 10, 'unmapped'), (20, 20, 'own'), (30, 30, 'shared'), (40, 40, 'mixed'), (50, 50, 'null mapping');
		INSERT INTO auth.mfa_email_challenges VALUES (10, 10, NULL, 'shared'), (20, 20, NULL, 'mapped own'), (30, 30, $2, 'shared'), (40, 40, $2, 'mixed own'), (50, 10, $1, 'direct own');
		INSERT INTO auth.roles VALUES (10, NULL), (20, $1), (30, $2);
		INSERT INTO auth.role_permissions VALUES (10, 'null role'), (20, 'own role'), (30, 'shared role');
		INSERT INTO platform.schools VALUES ($1, 'own'), ($2, 'shared');
		INSERT INTO platform.organizations VALUES ($1, 'own'), ($2, 'shared')`, TenantIDBase, TenantIDBase-1)
	// Expected fixture IDs, not predicates copied from sharedRowPredicate.
	expected := map[string]string{
		"auth.accounts":              `SELECT row_to_json(r)::text FROM auth.accounts AS r WHERE id IN (10, 30, 50)`,
		"auth.account_tenants":       `SELECT row_to_json(r)::text FROM auth.account_tenants AS r WHERE id IN (2, 4, 5)`,
		"auth.password_reset_tokens": `SELECT row_to_json(r)::text FROM auth.password_reset_tokens AS r WHERE id IN (10, 30, 50)`,
		"auth.mfa_email_challenges":  `SELECT row_to_json(r)::text FROM auth.mfa_email_challenges AS r WHERE id IN (10, 30)`,
		"auth.roles":                 `SELECT row_to_json(r)::text FROM auth.roles AS r WHERE id IN (10, 30)`,
		"auth.role_permissions":      `SELECT row_to_json(r)::text FROM auth.role_permissions AS r WHERE role_id IN (10, 30)`,
		"platform.schools":           `SELECT row_to_json(r)::text FROM platform.schools AS r WHERE payload = 'shared'`,
		"platform.organizations":     `SELECT row_to_json(r)::text FROM platform.organizations AS r WHERE payload = 'shared'`,
	}
	before := make(map[string]sharedTableState, len(expected))
	require.NoError(t, SnapshotSharedBaseline(ctx, dsn))
	for table, query := range expected {
		before[table] = aggregateRegressionOracle(t, ctx, db, query)
		require.Positive(t, before[table].Rows, table)
		require.Equal(t, before[table], aggregateRegressionBaseline(t, ctx, db, table), table)
	}
	aggregateRegressionExec(t, ctx, db, `
		UPDATE auth.accounts SET payload = 'own changed' WHERE id IN (20, 40);
		UPDATE auth.password_reset_tokens SET payload = 'own changed' WHERE id IN (20, 40);
		UPDATE auth.mfa_email_challenges SET payload = 'own changed' WHERE id IN (20, 40, 50);
		UPDATE auth.role_permissions SET payload = 'own changed' WHERE role_id = 20;
		UPDATE platform.schools SET payload = 'own changed' WHERE id = $1;
		UPDATE platform.organizations SET payload = 'own changed' WHERE id = $1`, TenantIDBase)
	deltas, err := Leftovers(ctx, dsn)
	require.NoError(t, err)
	require.Empty(t, deltas, "mixed mappings and either MFA ownership path must stay ignored")
	aggregateRegressionExec(t, ctx, db, `
		UPDATE auth.accounts SET payload = 'changed shared' WHERE id IN (10, 30, 50);
		UPDATE auth.password_reset_tokens SET payload = 'changed shared' WHERE id IN (10, 30, 50);
		UPDATE auth.mfa_email_challenges SET payload = 'changed shared' WHERE id IN (10, 30);
		UPDATE auth.role_permissions SET payload = 'changed shared' WHERE role_id IN (10, 30)`)
	deltas, err = Leftovers(ctx, dsn)
	require.NoError(t, err)
	require.Len(t, deltas, 4)
	for _, delta := range deltas {
		query, ok := expected[delta.Table]
		require.True(t, ok, delta.Table)
		after := aggregateRegressionOracle(t, ctx, db, query)
		aggregateRegressionDelta(t, delta, delta.Table, before[delta.Table], after)
	}
}

func TestSharedAggregateQuotedIdentifiersAndOrdering(t *testing.T) {
	t.Parallel()
	ctx, db, dsn := aggregateRegressionClone(t)
	aggregateRegressionExec(t, ctx, db, `
		CREATE SCHEMA "audit ' odd"" schema";
		CREATE TABLE "audit ' odd"" schema"."z table" (payload text);
		CREATE TABLE "audit ' odd"" schema"."a.""table" (payload text)`)
	const aQuery = `SELECT row_to_json(r)::text FROM "audit ' odd"" schema"."a.""table" AS r`
	const zQuery = `SELECT row_to_json(r)::text FROM "audit ' odd"" schema"."z table" AS r`
	const aName = "audit ' odd\" schema.a.\"table"
	const zName = "audit ' odd\" schema.z table"
	require.NoError(t, SnapshotSharedBaseline(ctx, dsn))
	empty := aggregateRegressionOracle(t, ctx, db, aQuery)
	require.EqualValues(t, 0, empty.Rows)
	assert.Equal(t, empty, aggregateRegressionBaseline(t, ctx, db, aName))
	assert.Equal(t, empty, aggregateRegressionBaseline(t, ctx, db, zName))
	// Reverse creation/insertion order must not change the public delta order.
	aggregateRegressionExec(t, ctx, db, `
		INSERT INTO "audit ' odd"" schema"."z table" VALUES ('<&> Grüße'), (NULL);
		INSERT INTO "audit ' odd"" schema"."a.""table" VALUES ('duplicate'), ('duplicate'), (NULL)`)
	deltas, err := Leftovers(ctx, dsn)
	require.NoError(t, err)
	require.Len(t, deltas, 2)
	aState := aggregateRegressionOracle(t, ctx, db, aQuery)
	zState := aggregateRegressionOracle(t, ctx, db, zQuery)
	aggregateRegressionDelta(t, deltas[0], aName, empty, aState)
	aggregateRegressionDelta(t, deltas[1], zName, empty, zState)
	require.NoError(t, SnapshotSharedBaseline(ctx, dsn))
	aggregateRegressionExec(t, ctx, db, `
		TRUNCATE "audit ' odd"" schema"."a.""table", "audit ' odd"" schema"."z table";
		INSERT INTO "audit ' odd"" schema"."a.""table" VALUES (NULL), ('duplicate'), ('duplicate');
		INSERT INTO "audit ' odd"" schema"."z table" VALUES (NULL), ('<&> Grüße')`)
	require.Equal(t, aState, aggregateRegressionOracle(t, ctx, db, aQuery))
	require.Equal(t, zState, aggregateRegressionOracle(t, ctx, db, zQuery))
	deltas, err = Leftovers(ctx, dsn)
	require.NoError(t, err)
	require.Empty(t, deltas, "row order must not affect fingerprints; duplicate multiplicity must remain")
}

func TestSharedAggregateMissingTableFails(t *testing.T) {
	t.Parallel()
	ctx, db, dsn := aggregateRegressionClone(t)
	require.NoError(t, SnapshotSharedBaseline(ctx, dsn))
	aggregateRegressionExec(t, ctx, db, `DROP TABLE public.marker`)
	deltas, err := Leftovers(ctx, dsn)
	require.Error(t, err, "a recorded table disappearing must not become an empty table")
	assert.Nil(t, deltas)
	assert.ErrorContains(t, err, "marker")
	var state interface{ Field(byte) string }
	require.True(t, errors.As(err, &state), "preserve PostgreSQL error identity through the error chain")
	assert.Equal(t, "42P01", state.Field('C'))
}

func TestSharedAggregateEmptyInventory(t *testing.T) {
	t.Parallel()
	ctx, db, dsn := aggregateRegressionClone(t)
	aggregateRegressionExec(t, ctx, db, `DROP TABLE public.marker`)
	require.NoError(t, SnapshotSharedBaseline(ctx, dsn))
	var recorded int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM phx_test.shared_baseline`).Scan(&recorded))
	require.Zero(t, recorded)
	deltas, err := Leftovers(ctx, dsn)
	require.NoError(t, err)
	require.Empty(t, deltas)
}

func TestSharedAggregateDollarIdentifier(t *testing.T) {
	t.Parallel()
	ctx, db, dsn := aggregateRegressionClone(t)
	aggregateRegressionExec(t, ctx, db, `CREATE TABLE public."a$1" (payload text)`)
	const query = `SELECT row_to_json(r)::text FROM public."a$1" AS r`
	const name = "public.a$1"
	require.NoError(t, SnapshotSharedBaseline(ctx, dsn))
	before := aggregateRegressionOracle(t, ctx, db, query)
	assert.Equal(t, before, aggregateRegressionBaseline(t, ctx, db, name))
	aggregateRegressionExec(t, ctx, db, `INSERT INTO public."a$1" VALUES ('<&> Grüße'), (NULL), ('duplicate'), ('duplicate')`)
	deltas, err := Leftovers(ctx, dsn)
	require.NoError(t, err)
	require.Len(t, deltas, 1)
	after := aggregateRegressionOracle(t, ctx, db, query)
	aggregateRegressionDelta(t, deltas[0], name, before, after)
}
