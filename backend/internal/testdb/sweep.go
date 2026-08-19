package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
)

// SweepOptions controls Sweep.
type SweepOptions struct {
	// RunID selects this run's clones for teardown (dropped even without the
	// generation GC's no-connection criterion — the run is over). Empty means
	// GC-only.
	RunID string
	// ReportLeftovers compares each of this run's clones against the template
	// before dropping it and reports tables with extra or missing rows.
	// Opt-in diagnosis (PHX_TEST_LEFTOVERS=1), never a gate (ADR 0004).
	ReportLeftovers bool
}

// TableDelta describes one table whose clone row count differs from the
// template's.
type TableDelta struct {
	Table        string
	TemplateRows int64
	CloneRows    int64
}

// CloneLeftovers lists the row-count deltas of one clone vs. the template.
type CloneLeftovers struct {
	Clone  string
	Tables []TableDelta
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

	unlock, err := acquireLifecycleLock(ctx, maint)
	if err != nil {
		return nil, fmt.Errorf("acquire test DB lifecycle lock: %w", err)
	}
	defer unlock()

	result := &SweepResult{}

	if opts.RunID != "" {
		runPrefix := ClonePrefix + SanitizeRunID(opts.RunID) + "_"
		own, err := listDatabasesByPrefix(ctx, maint, runPrefix)
		if err != nil {
			return nil, err
		}
		if opts.ReportLeftovers && len(own) > 0 {
			templateCounts, err := countAllTables(ctx, cfg.TemplateDSN())
			if err != nil {
				return nil, fmt.Errorf("count template rows for leftover report: %w", err)
			}
			for _, name := range own {
				deltas, err := leftoverDeltas(ctx, cfg.DatabaseDSN(name), templateCounts)
				if err != nil {
					return nil, fmt.Errorf("diagnose leftovers in %q: %w", name, err)
				}
				if len(deltas) > 0 {
					result.Leftovers = append(result.Leftovers, CloneLeftovers{Clone: name, Tables: deltas})
				}
			}
		}
		for _, name := range own {
			if err := dropDatabase(ctx, maint, name); err != nil {
				return result, fmt.Errorf("drop run clone %q: %w", name, err)
			}
			result.Dropped = append(result.Dropped, name)
		}
	}

	gcDropped, err := gcLocked(ctx, maint, "", cfg.TemplateName())
	if err != nil {
		return result, err
	}
	result.Dropped = append(result.Dropped, gcDropped...)

	return result, nil
}

func listDatabasesByPrefix(ctx context.Context, maint *sql.DB, prefix string) ([]string, error) {
	rows, err := maint.QueryContext(ctx,
		`SELECT datname FROM pg_database WHERE datname LIKE $1 ORDER BY datname`,
		prefix+"%")
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

// countAllTables returns exact row counts for every base table in dsn's
// non-system schemas. Exact counts are deliberate: this only runs behind the
// opt-in leftover diagnosis, and pg_stat estimates are unreliable right after
// a clone.
func countAllTables(ctx context.Context, dsn string) (map[string]int64, error) {
	db := openSQL(dsn)
	defer func() { _ = db.Close() }()

	rows, err := db.QueryContext(ctx, `
		SELECT n.nspname || '.' || c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY 1`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	counts := make(map[string]int64, len(tables))
	for _, t := range tables {
		var n int64
		// t comes from pg_class of the trusted test server; quote both parts.
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM `+quoteQualified(t)).Scan(&n); err != nil {
			return nil, fmt.Errorf("count %s: %w", t, err)
		}
		counts[t] = n
	}
	return counts, nil
}

func quoteQualified(schemaDotTable string) string {
	for i := 0; i < len(schemaDotTable); i++ {
		if schemaDotTable[i] == '.' {
			return quoteIdentifier(schemaDotTable[:i]) + "." + quoteIdentifier(schemaDotTable[i+1:])
		}
	}
	return quoteIdentifier(schemaDotTable)
}

func leftoverDeltas(ctx context.Context, cloneDSN string, templateCounts map[string]int64) ([]TableDelta, error) {
	cloneCounts, err := countAllTables(ctx, cloneDSN)
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
		if cloneCounts[t] != templateCounts[t] {
			deltas = append(deltas, TableDelta{Table: t, TemplateRows: templateCounts[t], CloneRows: cloneCounts[t]})
		}
	}
	return deltas, nil
}
