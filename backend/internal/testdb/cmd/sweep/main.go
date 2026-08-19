// Command sweep tears down the test-database clones of one run, garbage-
// collects clones of dead runs, and enforces the leftover gate. The test
// wrapper scripts invoke it after `go test`:
//
//	PHX_TEST_RUN_ID=<id> go run ./internal/testdb/cmd/sweep
//
// The leftover gate (#2419 goal 2, "Start = Ende") compares every clone of
// this run against the start state it recorded for itself and fails the run
// when a package left rows behind in SHARED state — rows outside the tenants
// its own tests created. Rows inside a test's own tenant are not leftovers:
// nothing else can see them and they die with the clone. Tolerated pairs live
// in testdb.LeftoverAllowlist and may only shrink.
//
// PHX_TEST_LEFTOVERS=1 additionally prints the tolerated leftovers, so the
// allowlist can be worked down without guessing what is in it.
//
// The gate only sees the clones that still exist: a SECOND test run started
// while this one is going collects the clones of finished packages as dead
// (that is the generation GC's rule — no live connection means no live run),
// and their leftovers go unreported. The report is therefore a lower bound
// whenever two runs overlap; for a measurement, run the suite alone.
//
// Without TEST_DB_DSN (or with the server unreachable) the command exits 0:
// there is nothing to sweep, and the wrapper's exit trap must not mask the
// test result.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/testdb"
	"github.com/subosito/gotenv"
)

func main() {
	loadDotEnv()

	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		fmt.Println("testdb sweep: TEST_DB_DSN not set, nothing to sweep")
		return
	}
	cfg, err := testdb.NewConfig(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testdb sweep: %v\n", err)
		os.Exit(1)
	}
	// Resolve the migration-scoped template of THIS worktree so the template
	// GC spares it.
	if hash, err := testdb.MigrationsHash(); err == nil {
		cfg = cfg.ForMigrations(hash)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	verbose := os.Getenv("PHX_TEST_LEFTOVERS") == "1"

	result, err := testdb.Sweep(ctx, cfg, testdb.SweepOptions{
		RunID:           os.Getenv(testdb.RunIDEnv),
		ReportLeftovers: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "testdb sweep: %v\n", err)
		os.Exit(1)
	}

	if verbose {
		for _, l := range result.Leftovers {
			fmt.Printf("testdb sweep: leftovers in %s (%s):\n", describe(l), l.Clone)
			for _, d := range l.Tables {
				fmt.Printf("  %-60s start=%d end=%d\n", d.Table, d.BaselineRows, d.CloneRows)
			}
		}
	}
	fmt.Printf("testdb sweep: dropped %d clone(s)\n", len(result.Dropped))

	failing := testdb.UnallowedLeftovers(result.Leftovers)
	if len(failing) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "\ntestdb sweep: FAILED — %d package(s) left rows in shared state:\n\n", len(failing))
	for _, l := range failing {
		fmt.Fprintf(os.Stderr, "  %s\n", describe(l))
		for _, d := range l.Tables {
			fmt.Fprintf(os.Stderr, "    %-56s start=%d end=%d\n", d.Table, d.BaselineRows, d.CloneRows)
		}
	}
	fmt.Fprint(os.Stderr, `
A test wrote rows that no tenant of its own owns, so the next test in the same
binary runs on top of them. Fix the test, not the gate:

  - create the row under the test's own tenant (testpkg.Ctx / testpkg.Tenant),
    which is what makes it invisible to every other test, or
  - delete it again when the table is genuinely tenant-less (platform
    operators, global tokens).

testdb.LeftoverAllowlist tolerates the pairs that predate this gate; it may
only shrink.
`)
	os.Exit(1)
}

func describe(l testdb.CloneLeftovers) string {
	if l.Package == "" {
		return "(unlabeled clone)"
	}
	return l.Package
}

// loadDotEnv best-effort loads the project root .env so TEST_DB_DSN is
// available when the wrapper runs outside CI.
func loadDotEnv() {
	if root, err := testdb.ProjectRoot(); err == nil {
		_ = gotenv.Load(filepath.Join(root, ".env"))
	}
}
