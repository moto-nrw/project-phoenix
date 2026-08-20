// Command bootstrap does the once-per-run part of the test-database
// lifecycle: make sure the server is reachable (starting the local container
// if needed) and the template for this worktree's migrations hash is built.
// It prints the resolved template name, which the wrapper exports as
// PHX_TEST_TEMPLATE so the ~93 package binaries can skip both steps:
//
//	PHX_TEST_TEMPLATE=$(go run ./internal/testdb/cmd/bootstrap)
//
// Measured on the full suite, the two steps cost 2,6s summed across the
// binaries (0,7s server ping + 1,9s template check) against ~0,2s for this
// command with a warm build cache.
//
// A naked `go test ./...` never runs this and does both steps itself — that
// is the point of the env var being optional (ADR 0004).
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
	if root, err := testdb.ProjectRoot(); err == nil {
		_ = gotenv.Load(filepath.Join(root, ".env"))
	}

	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		fail(fmt.Errorf("TEST_DB_DSN not set"))
	}
	cfg, err := testdb.NewConfig(dsn)
	if err != nil {
		fail(err)
	}

	// A template rebuild runs every migration (~25s).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := testdb.EnsureServer(ctx, cfg); err != nil {
		fail(err)
	}
	templateCfg, err := testdb.EnsureTemplate(ctx, cfg)
	if err != nil {
		fail(err)
	}
	fmt.Println(templateCfg.TemplateName())
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "testdb bootstrap: %v\n", err)
	os.Exit(1)
}
