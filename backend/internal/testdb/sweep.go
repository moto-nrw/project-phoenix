package testdb

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SweepOptions controls Sweep.
type SweepOptions struct {
	// RunID selects this run's clones for teardown (dropped even without the
	// generation GC's no-connection criterion — the run is over). Empty means
	// GC-only.
	RunID string
}

// SweepResult summarizes one Sweep run.
type SweepResult struct {
	Dropped []string
}

// Sweep tears down this run's clones and garbage-collects clones of dead
// runs. It is the wrapper's post-run companion to CreateClone; aborted runs
// without a sweep are collected by the next run's GC instead.
func Sweep(ctx context.Context, cfg *Config, opts SweepOptions) (*SweepResult, error) {
	maint := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = maint.Close() }()

	conn, unlock, err := acquireLifecycleLock(ctx, maint)
	if err != nil {
		return nil, fmt.Errorf("acquire test DB lifecycle lock: %w", err)
	}
	defer unlock()

	result := &SweepResult{}

	if opts.RunID != "" {
		runPrefix := ClonePrefix + SanitizeRunID(opts.RunID) + "_"
		own, err := listDatabasesByPrefix(ctx, conn, runPrefix)
		if err != nil {
			return nil, err
		}
		for _, name := range own {
			if err := dropDatabase(ctx, conn, name); err != nil {
				return result, fmt.Errorf("drop run clone %q: %w", name, err)
			}
			result.Dropped = append(result.Dropped, name)
		}
	}

	// The generation GC spares THIS PROCESS's run even when opts.RunID names a
	// different one. Without that, a Sweep called from inside a running suite
	// — which is what internal/testdb's own tests do — collects the clones of
	// every package that already finished, because "no live connection" is how
	// a dead run is recognised. The wrapper is unaffected: it dropped its own
	// clones explicitly a few lines above.
	gcDropped, err := gcLocked(ctx, conn, cfg.TemplateName(), ClonePrefix+RunID()+"_")
	if err != nil {
		return result, err
	}
	result.Dropped = append(result.Dropped, gcDropped...)

	templatesDropped, err := gcTemplatesLocked(ctx, conn, cfg, time.Now())
	if err != nil {
		return result, err
	}
	result.Dropped = append(result.Dropped, templatesDropped...)

	return result, nil
}

// gcTemplatesLocked drops migration-scoped templates (<base>_<hash>) that
// were not used for templateMaxIdleAge. The current run's template is spared
// because EnsureTemplate stamped it with "now" minutes ago, and a template
// being cloned from right now cannot be dropped at all: every CREATE
// DATABASE ... TEMPLATE runs under the same lifecycle lock this function
// holds. Unstamped databases are left alone — they carry no last-used
// timestamp, and EnsureTemplate rebuilds them when their hash comes back.
// Callers must hold the lifecycle lock.
func gcTemplatesLocked(ctx context.Context, maint sqlExecutor, cfg *Config, now time.Time) ([]string, error) {
	comments, err := databaseComments(ctx, maint, cfg.BaseTemplateName()+"_")
	if err != nil {
		return nil, fmt.Errorf("list test database templates: %w", err)
	}

	var stale []string
	for name, comment := range comments {
		if name == cfg.TemplateName() {
			continue
		}
		touched, ok := touchedAt(comment)
		if !ok || now.Sub(touched) < templateMaxIdleAge {
			continue
		}
		stale = append(stale, name)
	}
	sort.Strings(stale)

	var dropped []string
	for _, name := range stale {
		if err := dropDatabase(ctx, maint, name); err != nil {
			return dropped, fmt.Errorf("drop stale template %q: %w", name, err)
		}
		dropped = append(dropped, name)
	}
	return dropped, nil
}

// touchedAt reads the last-used timestamp out of a template comment
// ("phx-migrations-hash:<hash> touched:<unix>").
func touchedAt(comment string) (time.Time, bool) {
	if !strings.HasPrefix(comment, hashCommentPrefix) {
		return time.Time{}, false
	}
	idx := strings.Index(comment, touchedCommentKey)
	if idx < 0 {
		return time.Time{}, false
	}
	unix, err := strconv.ParseInt(comment[idx+len(touchedCommentKey):], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(unix, 0), true
}

func listDatabasesByPrefix(ctx context.Context, maint sqlExecutor, prefix string) ([]string, error) {
	rows, err := maint.QueryContext(ctx,
		`SELECT datname FROM pg_database WHERE LEFT(datname, char_length($1)) = $1 ORDER BY datname`,
		prefix)
	if err != nil {
		return nil, fmt.Errorf("list databases with prefix %q: %w", prefix, err)
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// sharedRowPredicates answers, per table, the question "is this row SHARED
// state rather than a test's own?" — as a SQL condition over the table
// aliased `t`. An empty condition means every row of the table is shared.
//
// Three shapes, in the order the resolver tries them:
//
//  1. A tenant_id column: below the test band (or absent) means shared.
//     That is the default and covers ~90% of the schema.
//  2. Tables whose own primary key IS the tenant (a school's ID is the tenant
//     ID; the fixtures create its organization under the same ID).
//  3. Tables that reach their tenant through a mapping row. auth.accounts is
//     the important one: an account carries no tenant_id at all — the link to
//     a school lives in auth.account_tenants. An account that maps to a test
//     tenant was created by a test and dies with the clone; only one that
//     maps nowhere, or nowhere test-owned, is shared state.
//
// Shape 3 is why this is a predicate and not a column name. Enumerating the
// mapping tables by hand is the price; the alternative was tolerating
// auth.accounts as a leftover in three quarters of all packages.
var tenantIdentityTables = map[string]string{
	"platform.schools":       "id",
	"platform.organizations": "id",
}

// mappedTenantTables reach their tenant through a join table.
var mappedTenantTables = map[string]struct{ mappingTable, foreignKey string }{
	"auth.accounts": {"auth.account_tenants", "account_id"},
}

func sharedRowPredicate(table string, hasTenantID bool) string {
	switch {
	case hasTenantID:
		return fmt.Sprintf("t.tenant_id IS NULL OR t.tenant_id < %d", TenantIDBase)
	case tenantIdentityTables[table] != "":
		return fmt.Sprintf("t.%s < %d", tenantIdentityTables[table], TenantIDBase)
	default:
		if m, ok := mappedTenantTables[table]; ok {
			return fmt.Sprintf(
				"NOT EXISTS (SELECT 1 FROM %s m WHERE m.%s = t.id AND m.tenant_id >= %d)",
				m.mappingTable, m.foreignKey, TenantIDBase)
		}
		return ""
	}
}

// tableSharedPredicates is tableOwnerColumns on a pool the caller already has.
func tableSharedPredicates(ctx context.Context, db sqlExecutor) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT n.nspname || '.' || c.relname,
		       EXISTS (
		           SELECT 1 FROM pg_attribute a
		           WHERE a.attrelid = c.oid AND a.attname = 'tenant_id' AND a.attnum > 0 AND NOT a.attisdropped
		       )
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', '`+baselineSchema+`')
		ORDER BY 1`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	owners := make(map[string]string)
	for rows.Next() {
		var table string
		var hasTenantID bool
		if err := rows.Scan(&table, &hasTenantID); err != nil {
			return nil, err
		}
		owners[table] = sharedRowPredicate(table, hasTenantID)
	}
	return owners, rows.Err()
}

func sharedTableStatesDB(ctx context.Context, db sqlExecutor, predicates map[string]string) (map[string]sharedTableState, error) {
	// One round trip for ~300 tables instead of ~300: the per-table counts are
	// pushed into the server as query_to_xml sub-selects. The sweep runs this
	// once per clone (~160 per suite run), so the difference is minutes.
	values, args := sharedCountValues(predicates)
	if len(args) == 0 {
		return map[string]sharedTableState{}, nil
	}

	rows, err := db.QueryContext(ctx, `
		SELECT v.tbl,
			(xpath('/row/c/text()', query_to_xml(v.q, false, true, '')))[1]::text::bigint,
			(xpath('/row/f/text()', query_to_xml(v.q, false, true, '')))[1]::text
		FROM (VALUES `+values+`) AS v(tbl, q)`, args...)
	if err != nil {
		return nil, fmt.Errorf("snapshot table rows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	states := make(map[string]sharedTableState, len(predicates))
	for rows.Next() {
		var table string
		var state sharedTableState
		if err := rows.Scan(&table, &state.Rows, &state.Fingerprint); err != nil {
			return nil, err
		}
		states[table] = state
	}
	return states, rows.Err()
}

func sharedCountValues(predicates map[string]string) (string, []any) {
	tables := make([]string, 0, len(predicates))
	for table := range predicates {
		tables = append(tables, table)
	}
	sort.Strings(tables)

	var values strings.Builder
	args := make([]any, 0, 2*len(tables))
	for i, table := range tables {
		// table and owner come from pg_catalog of the trusted test server.
		count := `SELECT count(*) AS c, md5(COALESCE(string_agg(md5(row_to_json(t)::text), ',' ORDER BY md5(row_to_json(t)::text)), '')) AS f FROM ` + quoteQualified(table) + ` AS t`
		if pred := predicates[table]; pred != "" {
			count += ` WHERE ` + pred
		}
		if i > 0 {
			values.WriteString(",")
		}
		fmt.Fprintf(&values, "($%d,$%d)", 2*i+1, 2*i+2)
		args = append(args, table, count)
	}
	return values.String(), args
}

// databaseComments returns the shared comment of every database whose name
// starts with prefix. Both stamps the lifecycle writes live in that comment:
// a template's migrations hash and a clone's package label.
func databaseComments(ctx context.Context, maint sqlExecutor, prefix string) (map[string]string, error) {
	rows, err := maint.QueryContext(ctx, `
		SELECT d.datname, COALESCE(sd.description, '')
		FROM pg_database d
		LEFT JOIN pg_shdescription sd ON sd.objoid = d.oid AND sd.classoid = 'pg_database'::regclass
		WHERE LEFT(d.datname, char_length($1)) = $1`, prefix)
	if err != nil {
		return nil, fmt.Errorf("read database comments for prefix %q: %w", prefix, err)
	}
	defer func() { _ = rows.Close() }()

	comments := make(map[string]string)
	for rows.Next() {
		var name, comment string
		if err := rows.Scan(&name, &comment); err != nil {
			return nil, err
		}
		comments[name] = comment
	}
	return comments, rows.Err()
}

func quoteQualified(schemaDotTable string) string {
	if schema, table, ok := strings.Cut(schemaDotTable, "."); ok {
		return quoteIdentifier(schema) + "." + quoteIdentifier(table)
	}
	return quoteIdentifier(schemaDotTable)
}
