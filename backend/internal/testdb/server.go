package testdb

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// EnsureServer makes sure the postgres server behind cfg is reachable. If it
// is not, it tries to start the long-lived local test container once
// (`docker compose --profile test up -d postgres-test` at the project root)
// and waits for readiness. A CI service container is already reachable, so
// this never touches CI setups.
func EnsureServer(ctx context.Context, cfg *Config) error {
	if pingServer(ctx, cfg) == nil {
		return nil
	}

	if err := startTestContainer(ctx); err != nil {
		return fmt.Errorf(`test database server unreachable and auto-start failed: %v

To run integration tests manually:
  1. Start test database: docker compose --profile test up -d postgres-test
  2. Ensure .env contains: TEST_DB_DSN=postgres://postgres:postgres@localhost:5433/phoenix_test?sslmode=disable`, err)
	}

	deadline := time.Now().Add(60 * time.Second)
	for {
		if err := pingServer(ctx, cfg); err == nil {
			return nil
		} else if time.Now().After(deadline) {
			return fmt.Errorf("test database server did not become ready within 60s after container start: %w", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func pingServer(ctx context.Context, cfg *Config) error {
	db := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = db.Close() }()

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return db.PingContext(pingCtx)
}

// startTestContainer starts the postgres-test compose service from the
// project root (the parent of the backend module root, where the tracked
// docker-compose.example.yml lives).
// composeProject is the compose project name of the long-lived local stack
// (the main checkout's directory name), shared by all worktrees.
const composeProject = "project-phoenix"

func startTestContainer(ctx context.Context) error {
	backend, err := backendRoot()
	if err != nil {
		return fmt.Errorf("locate backend module root: %w", err)
	}
	projectRoot := filepath.Dir(backend)
	composeFile := filepath.Join(projectRoot, "docker-compose.example.yml")
	if _, err := os.Stat(composeFile); err != nil {
		return fmt.Errorf("no docker-compose.example.yml at %s", projectRoot)
	}

	startCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	// Fixed project name: compose derives it from the directory otherwise, so
	// every worktree would start its own postgres-test on the same host port
	// and collide with the one already running. One container serves all
	// worktrees (templates are keyed by migrations hash).
	cmd := exec.CommandContext(startCtx, "docker", "compose", "-p", composeProject, "-f", composeFile, "--profile", "test", "up", "-d", "postgres-test")
	cmd.Dir = projectRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker compose -p %s -f docker-compose.example.yml --profile test up -d postgres-test: %w\n%s", composeProject, err, out)
	}
	return nil
}
