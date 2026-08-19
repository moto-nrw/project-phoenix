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
	// CheckLeftovers compares each of this run's clones against the start
	// state it recorded for itself and collects the tables that differ
	// OUTSIDE the test tenants (see TenantIDBase). Rows a test wrote into its
	// own tenant are not leftovers — they die with the clone. Rows it wrote
	// into shared, tenant-less state are, because the next test in the same
	// binary sees them. The caller decides what to do with the result; the
	// sweep command fails the run on it.
	CheckLeftovers bool
}

// TableDelta describes one table whose clone row count differs from the
// template's.
type TableDelta struct {
	Table        string
	BaselineRows int64
	CloneRows    int64
}

// CloneLeftovers lists the row-count deltas of one clone vs. the template.
type CloneLeftovers struct {
	Clone string
	// Package is the backend-relative package path CreateClone stamped on the
	// clone; empty for a clone created before the stamp existed.
	Package string
	Tables  []TableDelta
}

// SweepResult summarizes one Sweep run.
type SweepResult struct {
	Dropped   []string
	Leftovers []CloneLeftovers
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
		if opts.CheckLeftovers && len(own) > 0 {
			labels, err := packageLabels(ctx, conn, own)
			if err != nil {
				return nil, err
			}
			for _, name := range own {
				deltas, err := leftoverDeltas(ctx, cfg.DatabaseDSN(name))
				if err != nil {
					return nil, fmt.Errorf("diagnose leftovers in %q: %w", name, err)
				}
				if len(deltas) > 0 {
					result.Leftovers = append(result.Leftovers,
						CloneLeftovers{Clone: name, Package: labels[name], Tables: deltas})
				}
			}
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
	prefix := cfg.BaseTemplateName() + "_"
	rows, err := maint.QueryContext(ctx, `
		SELECT d.datname, COALESCE(sd.description, '')
		FROM pg_database d
		LEFT JOIN pg_shdescription sd ON sd.objoid = d.oid AND sd.classoid = 'pg_database'::regclass
		WHERE LEFT(d.datname, char_length($1)) = $1`, prefix)
	if err != nil {
		return nil, fmt.Errorf("list test database templates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var stale []string
	for rows.Next() {
		var name, comment string
		if err := rows.Scan(&name, &comment); err != nil {
			return nil, fmt.Errorf("scan template name: %w", err)
		}
		if name == cfg.TemplateName() {
			continue
		}
		touched, ok := touchedAt(comment)
		if !ok || now.Sub(touched) < templateMaxIdleAge {
			continue
		}
		stale = append(stale, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list test database templates: %w", err)
	}

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

// tenantIdentityTables are the two tables whose own primary key IS the tenant
// (a school's ID is the tenant ID, and its organization is created under the
// same ID by the test fixtures). They carry no tenant_id column, so the band
// check reads their id instead.
var tenantIdentityTables = map[string]string{
	"platform.schools":       "id",
	"platform.organizations": "id",
}

// tableOwnerColumns maps every base table in dsn's non-system schemas to the
// column that decides tenant ownership: "tenant_id" where the column exists,
// "id" for the two tenant-identity tables, "" for genuinely shared tables
// (platform operators, migration bookkeeping, …) where every row is shared
// state.
func tableOwnerColumns(ctx context.Context, dsn string) (map[string]string, error) {
	db := openSQL(dsn)
	defer func() { _ = db.Close() }()

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
		switch {
		case hasTenantID:
			owners[table] = "tenant_id"
		default:
			owners[table] = tenantIdentityTables[table]
		}
	}
	return owners, rows.Err()
}

// countSharedRows returns exact row counts per table, counting only the rows
// that do NOT belong to a test's own tenant — the shared state a test must
// leave as it found it. Exact counts are deliberate: pg_stat estimates are
// unreliable right after a clone.
func countSharedRows(ctx context.Context, dsn string, owners map[string]string) (map[string]int64, error) {
	db := openSQL(dsn)
	defer func() { _ = db.Close() }()

	// One round trip for ~300 tables instead of ~300: the per-table counts are
	// pushed into the server as query_to_xml sub-selects. The sweep runs this
	// once per clone (~160 per suite run), so the difference is minutes.
	tables := make([]string, 0, len(owners))
	for table := range owners {
		tables = append(tables, table)
	}
	sort.Strings(tables)

	var values strings.Builder
	args := make([]any, 0, 2*len(tables))
	for i, table := range tables {
		// table and owner come from pg_catalog of the trusted test server.
		count := `SELECT count(*) AS c FROM ` + quoteQualified(table)
		if owners[table] != "" {
			col := quoteIdentifier(owners[table])
			count += fmt.Sprintf(` WHERE %s IS NULL OR %s < %d`, col, col, TenantIDBase)
		}
		if i > 0 {
			values.WriteString(",")
		}
		fmt.Fprintf(&values, "($%d,$%d)", 2*i+1, 2*i+2)
		args = append(args, table, count)
	}
	if len(tables) == 0 {
		return map[string]int64{}, nil
	}

	rows, err := db.QueryContext(ctx, `
		SELECT v.tbl, (xpath('/row/c/text()', query_to_xml(v.q, false, true, '')))[1]::text::bigint
		FROM (VALUES `+values.String()+`) AS v(tbl, q)`, args...)
	if err != nil {
		return nil, fmt.Errorf("count table rows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts := make(map[string]int64, len(owners))
	for rows.Next() {
		var table string
		var n int64
		if err := rows.Scan(&table, &n); err != nil {
			return nil, err
		}
		counts[table] = n
	}
	return counts, rows.Err()
}

// packageLabels reads the package stamp off each clone.
func packageLabels(ctx context.Context, maint sqlExecutor, clones []string) (map[string]string, error) {
	labels := make(map[string]string, len(clones))
	rows, err := maint.QueryContext(ctx, `
		SELECT d.datname, COALESCE(sd.description, '')
		FROM pg_database d
		LEFT JOIN pg_shdescription sd ON sd.objoid = d.oid AND sd.classoid = 'pg_database'::regclass
		WHERE d.datname = ANY($1)`, pgArray(clones))
	if err != nil {
		return nil, fmt.Errorf("read clone package labels: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name, comment string
		if err := rows.Scan(&name, &comment); err != nil {
			return nil, err
		}
		labels[name] = PackageLabelOf(comment)
	}
	return labels, rows.Err()
}

// pgArray renders a string slice as a postgres array literal for `= ANY(...)`.
// Clone names are generated identifiers ([a-z0-9_]), so no escaping is needed
// beyond the braces.
func pgArray(values []string) string {
	return "{" + strings.Join(values, ",") + "}"
}

func quoteQualified(schemaDotTable string) string {
	for i := 0; i < len(schemaDotTable); i++ {
		if schemaDotTable[i] == '.' {
			return quoteIdentifier(schemaDotTable[:i]) + "." + quoteIdentifier(schemaDotTable[i+1:])
		}
	}
	return quoteIdentifier(schemaDotTable)
}

// leftoverDeltas compares a clone's shared-row counts against the start state
// it recorded for itself (SnapshotSharedBaseline). A clone without a snapshot
// yields no deltas: there is nothing to compare against, and a missing
// snapshot must not read as "this package left rows behind".
func leftoverDeltas(ctx context.Context, cloneDSN string) ([]TableDelta, error) {
	baseline, owners, ok, err := readSharedBaseline(ctx, cloneDSN)
	if err != nil || !ok {
		return nil, err
	}

	cloneCounts, err := countSharedRows(ctx, cloneDSN, owners)
	if err != nil {
		return nil, err
	}

	tables := make([]string, 0, len(cloneCounts))
	for t := range cloneCounts {
		tables = append(tables, t)
	}
	sort.Strings(tables)

	var deltas []TableDelta
	for _, t := range tables {
		if cloneCounts[t] != baseline[t] {
			deltas = append(deltas, TableDelta{Table: t, BaselineRows: baseline[t], CloneRows: cloneCounts[t]})
		}
	}
	return deltas, nil
}
