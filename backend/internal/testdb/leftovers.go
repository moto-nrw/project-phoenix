package testdb

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
)

// The leftover gate (#2419 goal 2, "Start = Ende") lives in the TEST PROCESS:
// every test binary compares its clone's end state against the start state it
// recorded for itself (SnapshotSharedBaseline) and fails when a package left
// rows behind in shared state. It used to sit in the sweep, which meant it
// only ran under scripts/test-backend.sh and reported after the fact; now a
// naked `go test ./...` is gated too and the failure lands on the package
// that caused it.
//
// What counts as a leftover is unchanged: SHARED rows only — rows outside the
// tenants this run's tests created (see TenantIDBase). A row a test writes
// into its own tenant is invisible to every other test and dies with the
// clone.

// TableDelta describes one table whose end state differs from the start state
// the clone recorded for itself.
type TableDelta struct {
	Table               string
	BaselineRows        int64
	CloneRows           int64
	BaselineFingerprint string
	CloneFingerprint    string
}

// Leftovers compares the clone behind cloneDSN against its own start
// snapshot. A clone without a snapshot yields no deltas: there is nothing to
// compare against, and a missing snapshot must not read as "this package left
// rows behind".
//
// One round trip for the whole schema (countSharedRowsDB pushes the per-table
// counts into the server), so this costs a package one query at exit, not one
// per table and not one per test.
func Leftovers(ctx context.Context, cloneDSN string) ([]TableDelta, error) {
	db := openSQL(cloneDSN)
	defer func() { _ = db.Close() }()

	baseline, predicates, ok, err := readSharedBaseline(ctx, db)
	if err != nil || !ok {
		return nil, err
	}

	cloneStates, err := sharedTableStatesDB(ctx, db, predicates)
	if err != nil {
		return nil, err
	}

	tables := make([]string, 0, len(cloneStates))
	for t := range cloneStates {
		tables = append(tables, t)
	}
	sort.Strings(tables)

	var deltas []TableDelta
	for _, t := range tables {
		if cloneStates[t] != baseline[t] {
			deltas = append(deltas, TableDelta{
				Table:               t,
				BaselineRows:        baseline[t].Rows,
				CloneRows:           cloneStates[t].Rows,
				BaselineFingerprint: baseline[t].Fingerprint,
				CloneFingerprint:    cloneStates[t].Fingerprint,
			})
		}
	}
	return deltas, nil
}

// CurrentPackageLabel returns the backend-relative path of the package whose
// binary is running — the key the allowlist and the failure report use. Same
// label CreateClone stamps on the clone.
func CurrentPackageLabel() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return packageLabel(wd)
}

// FormatLeftovers renders the gate's failure message for one package.
func FormatLeftovers(pkg string, deltas []TableDelta) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\nleftover gate: FAILED — %s left rows in shared state:\n\n", pkg)
	for _, d := range deltas {
		fmt.Fprintf(&b, "    %-56s start=%d end=%d", d.Table, d.BaselineRows, d.CloneRows)
		if d.BaselineFingerprint != d.CloneFingerprint {
			b.WriteString(" content changed")
		}
		b.WriteByte('\n')
	}
	b.WriteString(`
A test wrote rows that no tenant of its own owns, so the next test in the same
binary runs on top of them. Fix the test, not the gate:

  - create the row under the test's own tenant (testpkg.Ctx / testpkg.Tenant),
    which is what makes it invisible to every other test, or
  - delete it again when the table is genuinely tenant-less (platform
    operators, global tokens).

To find the test that did it, run the package serially with the per-test gate:

  PHX_TEST_LEFTOVERS=test go test -parallel 1 ./` + pkg + `

testdb.LeftoverAllowlist tolerates the pairs that predate this gate; it may
only shrink.
`)
	return b.String()
}
