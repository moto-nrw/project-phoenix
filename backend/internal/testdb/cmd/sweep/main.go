// Command sweep tears down the test-database clones of one run and garbage-
// collects clones of dead runs. The test wrapper scripts invoke it after
// `go test`:
//
//	PHX_TEST_RUN_ID=<id> go run ./internal/testdb/cmd/sweep
//
// The leftover gate (#2419 goal 2, "Start = Ende") is NOT here: it runs
// inside each test binary, right after its tests finish, so a naked
// `go test ./...` is gated too and the failure names the package that caused
// it. See internal/testdb/leftovers.go.
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

	result, err := testdb.Sweep(ctx, cfg, testdb.SweepOptions{
		RunID: os.Getenv(testdb.RunIDEnv),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "testdb sweep: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("testdb sweep: dropped %d clone(s)\n", len(result.Dropped))
}

// loadDotEnv best-effort loads the project root .env so TEST_DB_DSN is
// available when the wrapper runs outside CI.
func loadDotEnv() {
	if root, err := testdb.ProjectRoot(); err == nil {
		_ = gotenv.Load(filepath.Join(root, ".env"))
	}
}
