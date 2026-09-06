// Command bootstrap does the once-per-run part of the test-database
// lifecycle: make sure the server is reachable (starting the local container
// if needed) and the template for this worktree's migrations hash is built.
// It prints the resolved template name, which the wrapper exports as
// PHX_TEST_TEMPLATE so package binaries can skip template preparation:
//
//	PHX_TEST_TEMPLATE=$(go run ./internal/testdb/cmd/bootstrap)
//
// Package binaries still acquire a server lease and check readiness, including
// when a long compilation outlives the local server's idle timeout. A naked
// `go test ./...` resolves its own template as before.
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
	// Dispatch before loading .env: the detached watcher needs no credentials.
	if len(os.Args) > 1 {
		var err error
		switch {
		case len(os.Args) == 2 && os.Args[1] == "--idle-status":
			err = testdb.IdleStatus(context.Background())
		case len(os.Args) == 3 && os.Args[1] == "--idle-watch":
			lock := os.NewFile(3, "watcher.lock")
			if lock == nil {
				fail(fmt.Errorf("watcher requires an inherited lock"))
			}
			if _, err := lock.Stat(); err != nil {
				fail(err)
			}
			defer func() { _ = lock.Close() }()
			err = testdb.WatchIdle(context.Background(), os.Args[2], lock, 15*time.Minute, 30*time.Second)
		default:
			err = fmt.Errorf("usage: bootstrap [--idle-status]")
		}
		if err != nil {
			fail(err)
		}
		return
	}

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
