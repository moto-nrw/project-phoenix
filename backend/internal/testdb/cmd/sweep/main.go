// Command sweep tears down the test-database clones of one run and
// garbage-collects clones of dead runs. The test wrapper scripts invoke it
// after `go test`:
//
//	PHX_TEST_RUN_ID=<id> go run ./internal/testdb/cmd/sweep
//
// PHX_TEST_LEFTOVERS=1 additionally reports tables whose row counts differ
// from the template before dropping (opt-in diagnosis, never a gate; the
// suite's bootstrap fixtures — tenant/room/staff 1 — show up here until the
// PR-2 fixture sweep removes them).
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, err := testdb.Sweep(ctx, cfg, testdb.SweepOptions{
		RunID:           os.Getenv(testdb.RunIDEnv),
		ReportLeftovers: os.Getenv("PHX_TEST_LEFTOVERS") == "1",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "testdb sweep: %v\n", err)
		os.Exit(1)
	}

	for _, l := range result.Leftovers {
		fmt.Printf("testdb sweep: leftovers in %s:\n", l.Clone)
		for _, d := range l.Tables {
			fmt.Printf("  %-60s template=%d clone=%d\n", d.Table, d.TemplateRows, d.CloneRows)
		}
	}
	fmt.Printf("testdb sweep: dropped %d clone(s)\n", len(result.Dropped))
}

// loadDotEnv best-effort loads the project root .env (parent of the backend
// module root) so TEST_DB_DSN is available when the wrapper runs outside CI.
func loadDotEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			_ = gotenv.Load(filepath.Join(filepath.Dir(dir), ".env"))
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}
