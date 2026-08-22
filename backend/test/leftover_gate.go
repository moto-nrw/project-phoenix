package test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/testdb"
)

// The leftover gate (#2419 goal 2, "Start = Ende") runs HERE, in the test
// binary, not in the sweep: a naked `go test ./...` is gated exactly like a
// wrapper run, and the package that left rows behind is the one that fails.
//
// Cost: one round trip at the end of the package — countSharedRowsDB pushes
// every table's count into the server as a single query. Per test it would be
// that same round trip a few hundred times over, which is why the per-test
// variant below is opt-in and refuses to run beside parallel tests, where its
// answer would be wrong anyway.

// leftoverModeEnv selects the gate's granularity:
//
//	unset / "0"  package gate only (the default)
//	"1"          package gate, and print the leftovers the allowlist tolerates
//	"test"       additionally check after EVERY test, naming the culprit;
//	             requires -parallel 1, because a parallel neighbour's rows
//	             would be attributed to whichever test finishes next
const leftoverModeEnv = "PHX_TEST_LEFTOVERS"

// keepCloneEnv suppresses the self-drop at the end of a run, so the clone
// survives for inspection.
const keepCloneEnv = "PHX_TEST_KEEP_CLONE"

// Run executes the package's tests, then the leftover gate, and exits with the
// resulting status. Every TestMain in the backend ends in this call.
func Run(m *testing.M) {
	code := m.Run()
	// A red suite must not be relabelled as a leftover problem — and a test
	// that failed before its cleanup would be blamed for rows it does not own.
	if code == 0 {
		if msg := packageLeftovers(); msg != "" {
			fmt.Fprint(os.Stderr, msg)
			code = 1
		}
	}
	// PHX_TEST_KEEP_CLONE keeps the database around for a post-mortem (psql
	// into it, or measure what the sweep still finds).
	if os.Getenv(keepCloneEnv) == "" {
		dropPackageClone()
	}
	os.Exit(code)
}

// leftoverMode returns the configured granularity.
func leftoverMode() string {
	mode := os.Getenv(leftoverModeEnv)
	if mode == "0" {
		return ""
	}
	return mode
}

// packageLeftovers returns the gate's failure message, or "" when the package
// left shared state as it found it. A package that never opened the database
// has no clone and nothing to compare.
func packageLeftovers() string {
	if packageClone == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	deltas, err := testdb.Leftovers(ctx, packageClone.DSN)
	if err != nil {
		return fmt.Sprintf("leftover gate: %v\n", err)
	}
	pkg := testdb.CurrentPackageLabel()
	if leftoverMode() != "" {
		for _, d := range deltas {
			fmt.Printf("leftover gate: %s %-50s start=%d end=%d\n", pkg, d.Table, d.BaselineRows, d.CloneRows)
		}
	}
	failing := testdb.UnallowedTables(pkg, deltas)
	if len(failing) == 0 {
		return ""
	}
	return testdb.FormatLeftovers(pkg, failing)
}

// dropPackageClone hands the clone back at the end of the run, so a naked
// `go test ./...` cleans up after itself instead of leaving its databases to
// the next run's GC (#2419 goal 3). The wrapper's sweep then finds nothing of
// its own left to drop and does GC only.
//
// Best-effort by design: a panicking or killed binary never gets here, which
// is exactly the case the generation GC exists for. A failure to drop is
// therefore not worth failing a green run over — it is one database the next
// GC pass collects.
func dropPackageClone() {
	if packageClone == nil || packageCloneCfg == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// The keeper and the shared pool hold connections to the clone; DROP
	// DATABASE ... WITH (FORCE) would terminate them, but closing first keeps
	// the server log free of "terminating connection" noise.
	if sharedTestDB != nil {
		_ = sharedTestDB.Close()
	}
	_ = packageClone.Close()
	if err := testdb.DropClone(ctx, packageCloneCfg, packageClone.Name); err != nil {
		fmt.Fprintf(os.Stderr, "testdb: could not drop package clone %s: %v\n", packageClone.Name, err)
	}
}

// perTestLeftoverCheck is what PHX_TEST_LEFTOVERS=test buys: the same
// comparison the package gate runs, but after every test, so the failure names
// the test instead of the package. Only meaningful serially.
func perTestLeftoverCheck(tb testing.TB) {
	tb.Helper()
	if parallelism() > 1 {
		tb.Fatalf("%s=test needs -parallel 1: with up to %d tests in flight, a leftover cannot be attributed to one of them",
			leftoverModeEnv, parallelism())
	}
	tb.Cleanup(func() {
		if tb.Failed() {
			return
		}
		msg := packageLeftovers()
		if msg == "" {
			return
		}
		tb.Errorf("this test left rows in shared state:\n%s", msg)
		// Move the start line to here, so the next test is measured against
		// what this one left rather than failing for it too.
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := testdb.SnapshotSharedBaseline(ctx, packageClone.DSN); err != nil {
			tb.Errorf("re-snapshot baseline after leftover: %v", err)
		}
	})
}

// parallelism reports the binary's -test.parallel.
func parallelism() int {
	if f := flag.Lookup("test.parallel"); f != nil {
		if g, ok := f.Value.(flag.Getter); ok {
			if n, ok := g.Get().(int); ok && n > 0 {
				return n
			}
		}
	}
	return 1
}
