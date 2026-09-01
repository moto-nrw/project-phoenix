package testdb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/uptrace/bun/driver/pgdriver"
)

// EnsureServer makes sure the postgres server behind cfg is reachable. If it
// is not, it tries to start the long-lived local test container once
// (`docker compose --profile test up -d postgres-test` at the project root)
// and waits for readiness. A CI service container is already reachable, so
// this never touches CI setups.
func EnsureServer(ctx context.Context, cfg *Config) error {
	return ensureServer(ctx, cfg, func(ctx context.Context) error {
		return startTestContainer(ctx, cfg)
	}, syncLocalSuperuserPassword)
}

func ensureServer(
	ctx context.Context,
	cfg *Config,
	start func(context.Context) error,
	repairAuthentication func(context.Context, *Config) error,
) error {
	pingErr := pingServer(ctx, cfg)
	if pingErr == nil {
		return nil
	}

	if isPostgresStarting(pingErr) {
		return waitForServer(ctx, cfg)
	}
	if isPostgresAuthenticationFailure(pingErr) {
		if err := repairAuthentication(ctx, cfg); err != nil {
			return fmt.Errorf("test database authentication failed and local password synchronization failed: %v: %w", pingErr, err)
		}
		if err := pingServer(ctx, cfg); err != nil {
			return fmt.Errorf("test database authentication still fails after local password synchronization: %w", err)
		}
		return nil
	}
	if !errors.Is(pingErr, syscall.ECONNREFUSED) {
		return fmt.Errorf("test database connection failed; refusing to restart the shared server: %w", pingErr)
	}

	if err := start(ctx); err != nil {
		return fmt.Errorf(`test database server unreachable and auto-start failed: %v

To run integration tests manually:
  1. Start test database: docker compose --profile test up -d postgres-test
  2. Ensure .env contains: TEST_DB_DSN=postgres://postgres:postgres@localhost:5433/phoenix_test?sslmode=disable`, err)
	}
	return waitForServer(ctx, cfg)
}

func waitForServer(ctx context.Context, cfg *Config) error {
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

func isPostgresStarting(err error) bool {
	var pgErr pgdriver.Error
	return errors.As(err, &pgErr) && pgErr.Field('C') == "57P03"
}

func isPostgresAuthenticationFailure(err error) bool {
	var pgErr pgdriver.Error
	return errors.As(err, &pgErr) && pgErr.Field('C') == "28P01"
}

type dockerCommandRunner func(context.Context, string, io.Reader, ...string) ([]byte, error)

func syncLocalSuperuserPassword(ctx context.Context, cfg *Config) error {
	return syncLocalSuperuserPasswordWithRunner(ctx, cfg, runDockerCommand)
}

func syncLocalSuperuserPasswordWithRunner(ctx context.Context, cfg *Config, run dockerCommandRunner) error {
	username := cfg.templateURL.User.Username()
	password, hasPassword := cfg.templateURL.User.Password()
	if username != "postgres" || !hasPassword {
		return fmt.Errorf("automatic password synchronization requires the postgres user with a password")
	}
	host := cfg.templateURL.Hostname()
	if !isLoopbackHost(host) {
		return fmt.Errorf("automatic password synchronization is limited to a loopback TEST_DB_DSN host, got %q", host)
	}
	port := cfg.templateURL.Port()
	if port == "" {
		port = "5432"
	}

	backend, err := backendRoot()
	if err != nil {
		return fmt.Errorf("locate backend module root: %w", err)
	}
	projectRoot := filepath.Dir(backend)
	composeFile := filepath.Join(projectRoot, "docker-compose.example.yml")
	if _, err := os.Stat(composeFile); err != nil {
		return fmt.Errorf("no docker-compose.example.yml at %s", projectRoot)
	}

	composeProject, _, configuredPort, err := inspectTestContainer(ctx, cfg, run)
	if err != nil {
		return fmt.Errorf("locate shared postgres-test container: %w", err)
	}
	if !configuredPort {
		return fmt.Errorf("shared postgres-test container does not publish configured port %s", port)
	}
	composeArgs := []string{"compose", "-p", composeProject, "-f", composeFile}

	statement := fmt.Sprintf("ALTER ROLE %s WITH PASSWORD %s;\n", quoteIdentifier(username), quoteLiteral(password))
	execArgs := append(append([]string{}, composeArgs...),
		"exec", "-T", "postgres-test", "psql", "--no-psqlrc", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "postgres")
	if output, err := run(ctx, projectRoot, strings.NewReader(statement), execArgs...); err != nil {
		return fmt.Errorf("synchronize postgres-test superuser password: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func publishesPort(output, expected string) bool {
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		_, port, err := net.SplitHostPort(strings.TrimSpace(line))
		if err == nil && port == expected {
			return true
		}
	}
	return false
}

func runDockerCommand(ctx context.Context, dir string, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	cmd.Stdin = stdin
	return cmd.CombinedOutput()
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
// composeProjectPrefix keeps test servers separate from the application stack.
// The configured port is appended because pipeline worktrees can use different
// host ports concurrently; worktrees using the same port still share one server.
const composeProjectPrefix = "project-phoenix-testdb"
const legacyComposeProject = "project-phoenix"

func composeProjectFor(cfg *Config) string {
	port := cfg.templateURL.Port()
	if port == "" {
		port = "5432"
	}
	return composeProjectPrefix + "-" + port
}

func composeProjectCandidates(cfg *Config) []string {
	return []string{composeProjectFor(cfg), legacyComposeProject}
}

func startTestContainer(ctx context.Context, cfg *Config) error {
	startCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	return startTestContainerWithRunner(startCtx, cfg, runDockerCommand, runTestContainerUp)
}

type testContainerStarter func(context.Context, *Config, string) error

func startTestContainerWithRunner(
	ctx context.Context,
	cfg *Config,
	run dockerCommandRunner,
	start testContainerStarter,
) error {
	composeProject, running, configuredPort, err := inspectTestContainer(ctx, cfg, run)
	if err != nil {
		return fmt.Errorf("inspect shared postgres-test container: %w", err)
	}
	if running && configuredPort {
		return nil
	}
	if composeProject == "" {
		composeProject = composeProjectFor(cfg)
	}
	return start(ctx, cfg, composeProject)
}

func inspectTestContainer(ctx context.Context, cfg *Config, run dockerCommandRunner) (composeProject string, running, configuredPort bool, err error) {
	host := cfg.templateURL.Hostname()
	if !isLoopbackHost(host) {
		return "", false, false, fmt.Errorf("automatic test database startup is limited to a loopback TEST_DB_DSN host, got %q", host)
	}
	port := cfg.templateURL.Port()
	if port == "" {
		port = "5432"
	}

	backend, err := backendRoot()
	if err != nil {
		return "", false, false, fmt.Errorf("locate backend module root: %w", err)
	}
	projectRoot := filepath.Dir(backend)
	composeFile := filepath.Join(projectRoot, "docker-compose.example.yml")
	if _, err := os.Stat(composeFile); err != nil {
		return "", false, false, fmt.Errorf("no docker-compose.example.yml at %s", projectRoot)
	}

	preferredProject := composeProjectFor(cfg)
	for _, candidate := range composeProjectCandidates(cfg) {
		composeArgs := []string{"compose", "-p", candidate, "-f", composeFile}
		psArgs := append(append([]string{}, composeArgs...), "ps", "--status", "running", "--quiet", "postgres-test")
		containerID, runErr := run(ctx, projectRoot, nil, psArgs...)
		if runErr != nil {
			return "", false, false, fmt.Errorf("locate running service in compose project %s: %w", candidate, runErr)
		}
		if strings.TrimSpace(string(containerID)) == "" {
			continue
		}

		portArgs := append(append([]string{}, composeArgs...), "port", "postgres-test", "5432")
		published, portErr := run(ctx, projectRoot, nil, portArgs...)
		if portErr != nil {
			if candidate == preferredProject {
				// A concurrent compose up can expose the preferred service in `ps`
				// before its port is inspectable. Converge that same project.
				return candidate, true, false, nil
			}
			continue
		}
		if publishesPort(string(published), port) {
			return candidate, true, true, nil
		}
		if candidate == preferredProject {
			composeProject = candidate
			running = true
		}
	}
	return composeProject, running, false, nil
}

func runTestContainerUp(ctx context.Context, cfg *Config, composeProject string) error {
	cmd, err := testContainerCommandForProject(ctx, cfg, composeProject)
	if err != nil {
		return err
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker compose -p %s -f docker-compose.example.yml --profile test up -d postgres-test: %w\n%s", composeProject, err, out)
	}
	return nil
}

func testContainerCommand(ctx context.Context, cfg *Config) (*exec.Cmd, error) {
	return testContainerCommandForProject(ctx, cfg, composeProjectFor(cfg))
}

func testContainerCommandForProject(ctx context.Context, cfg *Config, composeProject string) (*exec.Cmd, error) {
	username := cfg.templateURL.User.Username()
	password, hasPassword := cfg.templateURL.User.Password()
	if username != "postgres" || !hasPassword || password == "" {
		return nil, fmt.Errorf("automatic test database startup requires the postgres user with a password")
	}
	if host := cfg.templateURL.Hostname(); !isLoopbackHost(host) {
		return nil, fmt.Errorf("automatic test database startup is limited to a loopback TEST_DB_DSN host, got %q", host)
	}
	port := cfg.templateURL.Port()
	if port == "" {
		port = "5432"
	}

	backend, err := backendRoot()
	if err != nil {
		return nil, fmt.Errorf("locate backend module root: %w", err)
	}
	projectRoot := filepath.Dir(backend)
	composeFile := filepath.Join(projectRoot, "docker-compose.example.yml")
	if _, err := os.Stat(composeFile); err != nil {
		return nil, fmt.Errorf("no docker-compose.example.yml at %s", projectRoot)
	}

	// Derive the project from the configured port. Worktrees targeting the same
	// server share it (templates are keyed by migrations hash), while pipeline
	// worktrees with distinct assigned ports cannot replace each other's service.
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", composeProject, "-f", composeFile, "--profile", "test", "up", "-d", "postgres-test")
	cmd.Dir = projectRoot
	cmd.Env = replaceCommandEnvironment(os.Environ(), "TEST_DB_PORT", port)
	cmd.Env = replaceCommandEnvironment(cmd.Env, "POSTGRES_PASSWORD", password)
	return cmd, nil
}

func replaceCommandEnvironment(environment []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}
